package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const writeTimeout = 5 * time.Second

type ClientConn struct {
	conn     *websocket.Conn
	hub      *Hub
	handler  *WSHandler
	playerID int

	sendCh chan []byte
	done   chan struct{}

	closeOnce sync.Once
	mu        sync.Mutex
	room      *Room
	mc        *MatchController
}

type WSHandler struct {
	Hub      *Hub
	registry *ConnRegistry
	tokens   *TokenStore
	regOnce  sync.Once
}

type ConnRegistry struct {
	mu    sync.Mutex
	rooms map[string]*RoomConnections
}

type RoomConnections struct {
	conns [2]*ClientConn
	mc    *MatchController
}

type RoomBridge struct {
	clients [2]*ClientConn
}

func NewWSHandler(hub *Hub, tokens *TokenStore) *WSHandler {
	return &WSHandler{
		Hub:      hub,
		tokens:   tokens,
		registry: NewConnRegistry(),
	}
}

func NewConnRegistry() *ConnRegistry {
	return &ConnRegistry{rooms: make(map[string]*RoomConnections)}
}

func (h *WSHandler) ensureRegistry() {
	h.regOnce.Do(func() {
		if h.registry == nil {
			h.registry = NewConnRegistry()
		}
	})
}

func (h *WSHandler) Registry() *ConnRegistry {
	h.ensureRegistry()
	return h.registry
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ensureRegistry()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}

	client := &ClientConn{
		conn:    conn,
		hub:     h.Hub,
		handler: h,
		sendCh:  make(chan []byte, 256),
		done:    make(chan struct{}),
	}

	go client.readPump()
	go client.writePump()

	<-client.done
}

func (c *ClientConn) readPump() {
	defer c.cleanupAndClose()

	ctx := context.Background()
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}

		msgType, payload, err := UnwrapMessage(data)
		if err != nil {
			c.sendError(ErrCodeInvalidInput, err.Error())
			continue
		}

		c.handleMessage(msgType, payload)
	}
}

func (c *ClientConn) writePump() {
	for {
		select {
		case data := <-c.sendCh:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := c.conn.Write(ctx, websocket.MessageText, data)
			cancel()
			if err != nil {
				c.close()
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *ClientConn) handleMessage(msgType string, payload []byte) {
	switch msgType {
	case MsgTypeCreateRoom:
		c.handleCreateRoom(payload)
	case MsgTypeJoinRoom:
		c.handleJoinRoom(payload)
	case MsgTypeReady:
		c.handleReady(payload)
	case MsgTypeInput:
		c.handleInput(payload)
	case MsgTypePlayAgain:
		c.handlePlayAgain(payload)
	case MsgTypeRejoin:
		c.handleRejoin(payload)
	default:
		c.sendError(ErrCodeInvalidInput, "unknown message type")
	}
}

func (c *ClientConn) handleRejoin(payload []byte) {
	var msg RejoinMsg
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.sendError(ErrCodeInvalidInput, "invalid rejoin message")
		c.close()
		return
	}

	entry, valid := c.handler.tokens.ValidateToken(msg.Token)
	if !valid {
		c.sendError(ErrCodeInvalidInput, "invalid or expired token")
		c.close()
		return
	}

	c.handler.tokens.RemoveToken(msg.Token)

	room := c.hub.GetRoom(entry.RoomCode)
	if room == nil {
		c.sendError(ErrCodeRoomNotFound, "room not found")
		c.close()
		return
	}

	if room.GetState() == RoomClosed {
		c.sendError(ErrCodeServerError, "room is closed")
		c.close()
		return
	}

	c.setRoomAndPlayer(room, entry.PlayerID)
	c.handler.registry.SetConn(room.Code, entry.PlayerID, c)

	mc := c.handler.registry.GetMatch(room.Code)
	if mc != nil {
		c.setMatch(mc)
		c.sendOrLog(MsgTypeGameStart, mc.GameStartState(entry.PlayerID))
	} else {
		conns := c.handler.registry.GetClients(room.Code)
		if conns[0] != nil && conns[1] != nil {
			c.startMatchForRoom(room)
		} else {
			opponentName := c.lookupOpponentName(room, entry.PlayerID)
			c.sendOrLog(MsgTypeJoined, JoinedMsg{
				RoomCode:     room.Code,
				PlayerID:     entry.PlayerID,
				OpponentName: opponentName,
			})
		}
	}
}

func (c *ClientConn) handlePlayAgain(payload []byte) {
	room := c.getRoom()
	if room == nil {
		c.sendError(ErrCodeNotReady, "not in room")
		return
	}

	state := room.GetState()
	if state != RoomGameOver && state != RoomPlaying {
		c.sendError(ErrCodeNotReady, fmt.Sprintf("cannot play again in state %d", state))
		return
	}

	playerID := c.getPlayerID()

	room.SetRematch(playerID)

	oppName := c.lookupOpponentName(room, playerID)
	c.sendOrLog(MsgTypePlayAgainAck, PlayAgainAckMsg{WaitingFor: oppName})

	if !room.BothWantRematch() {
		return
	}

	room.ResetForRematch()

	conns := c.handler.registry.GetClients(room.Code)
	if conns[0] == nil || conns[1] == nil {
		c.sendError(ErrCodeServerError, "missing players for rematch")
		return
	}

	conns[0].sendOrLog(MsgTypeRematch, RematchMsg{Difficulty: room.Difficulty})
	conns[1].sendOrLog(MsgTypeRematch, RematchMsg{Difficulty: room.Difficulty})

	c.handler.registry.RemoveMatch(room.Code)

	c.startMatchForRoom(room)
}

func (c *ClientConn) handleCreateRoom(payload []byte) {
	if c.getRoom() != nil {
		c.sendError(ErrCodeAlreadyInRoom, "already in room")
		return
	}

	var msg CreateRoomMsg
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.sendError(ErrCodeInvalidInput, err.Error())
		return
	}

	// Validate difficulty (1-10)
	if msg.Difficulty < 1 || msg.Difficulty > 10 {
		c.sendError(ErrCodeInvalidInput, "difficulty must be 1-10")
		return
	}

	// Validate player name (1-16 chars, printable ASCII)
	if len(msg.PlayerName) == 0 || len(msg.PlayerName) > 16 {
		c.sendError(ErrCodeInvalidInput, "player name must be 1-16 characters")
		return
	}
	for _, r := range msg.PlayerName {
		if r < 32 || r > 126 {
			c.sendError(ErrCodeInvalidInput, "player name must contain only printable ASCII")
			return
		}
	}

	room := c.hub.CreateRoom(msg.Difficulty)
	if room == nil {
		c.sendError(ErrCodeServerError, "failed to create room")
		return
	}

	playerID, err := room.AddPlayer(msg.PlayerName)
	if err != nil {
		c.sendError(ErrCodeServerError, err.Error())
		return
	}

	c.setRoomAndPlayer(room, playerID)
	c.handler.registry.SetConn(room.Code, playerID, c)

	if err := c.sendMsg(MsgTypeRoomCreated, RoomCreatedMsg{RoomCode: room.Code}); err != nil {
		c.sendError(ErrCodeServerError, err.Error())
		return
	}

	c.sendOrLog(MsgTypeJoined, JoinedMsg{RoomCode: room.Code, PlayerID: playerID})
}

func (c *ClientConn) handleJoinRoom(payload []byte) {
	if c.getRoom() != nil {
		c.sendError(ErrCodeAlreadyInRoom, "already in room")
		return
	}

	var msg JoinRoomMsg
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.sendError(ErrCodeInvalidInput, err.Error())
		return
	}

	// Validate room code (4 chars, A-Z2-9)
	if len(msg.RoomCode) != 4 {
		c.sendError(ErrCodeInvalidInput, "room code must be 4 characters")
		return
	}
	for _, r := range msg.RoomCode {
		if !((r >= 'A' && r <= 'Z') || (r >= '2' && r <= '9')) {
			c.sendError(ErrCodeInvalidInput, "room code must contain only A-Z2-9")
			return
		}
	}

	room := c.hub.GetRoom(msg.RoomCode)
	if room == nil {
		c.sendError(ErrCodeRoomNotFound, "room not found")
		return
	}

	playerID, err := room.AddPlayer(msg.PlayerName)
	if err != nil {
		c.sendError(ErrCodeRoomFull, "room is full")
		return
	}

	c.setRoomAndPlayer(room, playerID)
	c.handler.registry.SetConn(room.Code, playerID, c)

	opponentName := c.lookupOpponentName(room, playerID)
	c.sendOrLog(MsgTypeJoined, JoinedMsg{RoomCode: room.Code, PlayerID: playerID, OpponentName: opponentName})

	otherConn := c.handler.registry.GetConn(room.Code, otherPlayerID(playerID))
	if otherConn != nil {
		otherConn.sendOrLog(MsgTypeJoined, JoinedMsg{RoomCode: room.Code, PlayerID: otherPlayerID(playerID), OpponentName: msg.PlayerName})
	}

	if room.BothPlayersConnected() {
		c.startMatchForRoom(room)
	}
}

func (c *ClientConn) handleReady(payload []byte) {
	room := c.getRoom()
	if room == nil {
		c.sendError(ErrCodeNotReady, "not in room")
		return
	}

	// If match already started (auto-start on join), this is a no-op
	if room.GetState() == RoomPlaying || room.GetState() == RoomGameOver {
		return
	}

	if len(payload) > 0 && string(payload) != "null" {
		var msg ReadyMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			c.sendError(ErrCodeInvalidInput, err.Error())
			return
		}
	}

	if err := room.SetReady(c.getPlayerID()); err != nil {
		return
	}

	if !room.BothReady() {
		return
	}

	c.startMatchForRoom(room)
}

func (c *ClientConn) handleInput(payload []byte) {
	mc := c.getMatch()
	if mc == nil {
		c.sendError(ErrCodeNotReady, "match not started")
		return
	}

	var msg InputMsg
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.sendError(ErrCodeInvalidInput, err.Error())
		return
	}

	if msg.Action != "down" && msg.Action != "up" {
		c.sendError(ErrCodeInvalidInput, fmt.Sprintf("invalid action %q", msg.Action))
		return
	}

	tracker := mc.GetInput(c.getPlayerID())
	if tracker == nil {
		c.sendError(ErrCodeInvalidInput, "invalid player id")
		return
	}

	if err := ProcessInputMsg(&msg, tracker); err != nil {
		c.sendError(ErrCodeInvalidInput, err.Error())
	}
}

func (c *ClientConn) send(data []byte) {
	select {
	case <-c.done:
		return
	default:
	}

	select {
	case c.sendCh <- data:
	default:
	}
}

func (c *ClientConn) sendMsg(msgType string, payload any) error {
	data, err := WrapMessage(msgType, payload)
	if err != nil {
		return err
	}
	c.send(data)
	return nil
}

func (c *ClientConn) sendOrLog(msgType string, payload any) {
	if err := c.sendMsg(msgType, payload); err != nil {
		log.Printf("[room=%s player=%d] sendMsg %s failed: %v", c.roomCode(), c.getPlayerID(), msgType, err)
	}
}

func (c *ClientConn) sendError(code string, message string) {
	c.sendOrLog(MsgTypeError, ErrorMsg{Code: code, Message: message})
}

func (c *ClientConn) cleanupAndClose() {
	room := c.getRoom()
	if room != nil {
		playerID := c.getPlayerID()
		oppID := otherPlayerID(playerID)
		oppConn := c.handler.registry.GetConn(room.Code, oppID)
		if oppConn != nil {
			oppConn.sendOrLog(MsgTypeOpponentLeft, OpponentLeftMsg{Reason: "disconnect"})
		}

		empty := room.RemovePlayer(playerID)

		mc := c.getMatch()
		if mc == nil {
			mc = c.handler.registry.GetMatch(room.Code)
		}
		if mc != nil {
			mc.Stop()
		}

		c.handler.registry.RemoveConn(room.Code, playerID)
		if empty {
			c.hub.RemoveRoom(room.Code)
			c.handler.registry.RemoveRoom(room.Code)
		}
	}

	c.close()
}

func (c *ClientConn) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if err := c.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
			log.Printf("[room=%s player=%d] conn.Close failed: %v", c.roomCode(), c.getPlayerID(), err)
		}
	})
}

func (c *ClientConn) setRoomAndPlayer(room *Room, playerID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.room = room
	c.playerID = playerID
}

func (c *ClientConn) getRoom() *Room {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.room
}

func (c *ClientConn) getPlayerID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.playerID
}

func (c *ClientConn) setMatch(mc *MatchController) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mc = mc
}

func (c *ClientConn) getMatch() *MatchController {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mc
}

func (c *ClientConn) roomCode() string {
	room := c.getRoom()
	if room == nil {
		return ""
	}
	return room.Code
}

func (c *ClientConn) lookupOpponentName(room *Room, playerID int) string {
	oppID := otherPlayerID(playerID)
	if oppID == 0 {
		return ""
	}

	room.mu.Lock()
	defer room.mu.Unlock()
	opp := room.Players[oppID-1]
	if opp == nil {
		return ""
	}
	return opp.Name
}

func (rb *RoomBridge) OnTick(_ uint64, state *TickMsg) {
	data, err := WrapMessage(MsgTypeTick, state)
	if err != nil {
		return
	}
	for _, c := range rb.clients {
		if c != nil {
			c.send(data)
		}
	}
}

func (rb *RoomBridge) OnTickDelta(_ uint64, state *TickDeltaMsg) {
	data, err := WrapMessage(MsgTypeTickDelta, state)
	if err != nil {
		return
	}
	for _, c := range rb.clients {
		if c != nil {
			c.send(data)
		}
	}
}

func (rb *RoomBridge) OnRoundOver(msg *RoundOverMsg) {
	data, err := WrapMessage(MsgTypeRoundOver, msg)
	if err != nil {
		return
	}
	for _, c := range rb.clients {
		if c != nil {
			c.send(data)
		}
	}
}

func (rb *RoomBridge) OnGameOver(msg *GameOverMsg) {
	data, err := WrapMessage(MsgTypeGameOver, msg)
	if err != nil {
		return
	}
	for _, c := range rb.clients {
		if c != nil {
			c.send(data)
		}
	}
}

func (cr *ConnRegistry) SetConn(roomCode string, playerID int, conn *ClientConn) {
	if playerID < 1 || playerID > 2 {
		return
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	rc := cr.rooms[roomCode]
	if rc == nil {
		rc = &RoomConnections{}
		cr.rooms[roomCode] = rc
	}
	rc.conns[playerID-1] = conn
}

func (cr *ConnRegistry) GetConn(roomCode string, playerID int) *ClientConn {
	if playerID < 1 || playerID > 2 {
		return nil
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	rc := cr.rooms[roomCode]
	if rc == nil {
		return nil
	}
	return rc.conns[playerID-1]
}

func (cr *ConnRegistry) GetClients(roomCode string) [2]*ClientConn {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	rc := cr.rooms[roomCode]
	if rc == nil {
		return [2]*ClientConn{}
	}
	return rc.conns
}

func (cr *ConnRegistry) SetMatchIfEmpty(roomCode string, mc *MatchController) bool {
	if mc == nil {
		return false
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	rc := cr.rooms[roomCode]
	if rc == nil {
		rc = &RoomConnections{}
		cr.rooms[roomCode] = rc
	}
	if rc.mc != nil {
		return false
	}
	rc.mc = mc
	return true
}

func (cr *ConnRegistry) GetMatch(roomCode string) *MatchController {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	rc := cr.rooms[roomCode]
	if rc == nil {
		return nil
	}
	return rc.mc
}

func (cr *ConnRegistry) RemoveConn(roomCode string, playerID int) {
	if playerID < 1 || playerID > 2 {
		return
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	rc := cr.rooms[roomCode]
	if rc == nil {
		return
	}
	rc.conns[playerID-1] = nil
}

func (cr *ConnRegistry) RemoveRoom(roomCode string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	delete(cr.rooms, roomCode)
}

func (cr *ConnRegistry) RemoveAllRooms() {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.rooms = make(map[string]*RoomConnections)
}

func (cr *ConnRegistry) RemoveMatch(roomCode string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	rc := cr.rooms[roomCode]
	if rc != nil {
		rc.mc = nil
	}
}

func otherPlayerID(playerID int) int {
	switch playerID {
	case 1:
		return 2
	case 2:
		return 1
	default:
		return 0
	}
}

func (c *ClientConn) startMatchForRoom(room *Room) {
	conns := c.handler.registry.GetClients(room.Code)
	if conns[0] == nil || conns[1] == nil {
		c.sendError(ErrCodeServerError, "missing players")
		return
	}

	existing := c.handler.registry.GetMatch(room.Code)
	if existing != nil {
		return
	}

	bridge := &RoomBridge{clients: conns}
	mc := NewMatchController(room.Difficulty, time.Now().UnixNano(), bridge)
	if !c.handler.registry.SetMatchIfEmpty(room.Code, mc) {
		mc = c.handler.registry.GetMatch(room.Code)
	}
	if mc == nil {
		c.sendError(ErrCodeServerError, "failed to start match")
		return
	}

	room.SetState(RoomPlaying)
	conns[0].setMatch(mc)
	conns[1].setMatch(mc)

	conns[0].sendOrLog(MsgTypeGameStart, mc.GameStartState(1))
	conns[1].sendOrLog(MsgTypeGameStart, mc.GameStartState(2))
	mc.Start()
}
