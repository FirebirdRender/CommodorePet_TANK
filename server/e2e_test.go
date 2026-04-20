package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type e2eClient struct {
	t       *testing.T
	conn    *websocket.Conn
	timeout time.Duration
}

func newE2EClient(t *testing.T, serverURL string) *e2eClient {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http")
	conn, _, err := websocket.Dial(ctx, wsURL+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	c := &e2eClient{t: t, conn: conn, timeout: 5 * time.Second}
	t.Cleanup(func() {
		c.closeNow()
	})
	return c
}

func (c *e2eClient) closeNow() {
	if c.conn != nil {
		c.conn.CloseNow()
	}
}

func (c *e2eClient) send(msgType string, payload any) {
	c.t.Helper()

	data, err := WrapMessage(msgType, payload)
	if err != nil {
		c.t.Fatalf("wrap message: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		c.t.Fatalf("write message: %v", err)
	}
}

func (c *e2eClient) recvWithTimeout(timeout time.Duration) (string, json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, data, err := c.conn.Read(ctx)
	if err != nil {
		return "", nil, err
	}

	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		return "", nil, err
	}

	return msgType, payload, nil
}

func (c *e2eClient) recv() (string, json.RawMessage) {
	c.t.Helper()

	msgType, payload, err := c.recvWithTimeout(c.timeout)
	if err != nil {
		c.t.Fatalf("read message: %v", err)
	}
	return msgType, payload
}

func (c *e2eClient) recvExpect(expectedType string) json.RawMessage {
	c.t.Helper()

	msgType, payload := c.recv()
	if msgType != expectedType {
		c.t.Fatalf("message type = %q, want %q", msgType, expectedType)
	}
	return payload
}

func (c *e2eClient) recvUntil(expectedType string, maxReads int) json.RawMessage {
	c.t.Helper()

	if maxReads <= 0 {
		c.t.Fatalf("maxReads must be > 0")
	}

	deadline := time.Now().Add(c.timeout)
	for i := 0; i < maxReads && time.Now().Before(deadline); i++ {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		msgType, payload, err := c.recvWithTimeout(remaining)
		if err != nil {
			c.t.Fatalf("recvUntil read failed: %v", err)
		}

		if msgType == expectedType {
			return payload
		}
	}

	c.t.Fatalf("did not receive message type %q within %s", expectedType, c.timeout)
	return nil
}

func decodeRaw[T any](t *testing.T, payload json.RawMessage) T {
	t.Helper()

	var out T
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return out
}

func requireGridShape(t *testing.T, grid [][]int, rows, cols int) {
	t.Helper()

	if len(grid) != rows {
		t.Fatalf("grid rows = %d, want %d", len(grid), rows)
	}
	for y := range grid {
		if len(grid[y]) != cols {
			t.Fatalf("grid cols at row %d = %d, want %d", y, len(grid[y]), cols)
		}
	}
}

func requireTickShape(t *testing.T, tick TickMsg) {
	t.Helper()

	if tick.Tick == 0 {
		t.Fatal("tick id = 0, want > 0")
	}
	requireGridShape(t, tick.Grid, 21, 40)
	if tick.Tanks[0].PlayerID != 1 || tick.Tanks[1].PlayerID != 2 {
		t.Fatalf("tick tank player IDs = [%d,%d], want [1,2]", tick.Tanks[0].PlayerID, tick.Tanks[1].PlayerID)
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool, name string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("condition not met: %s", name)
}

func expectTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "deadline exceeded")
}

func findTankByPlayerID(t *testing.T, tanks [2]TankState, playerID int) TankState {
	t.Helper()

	for _, tank := range tanks {
		if tank.PlayerID == playerID {
			return tank
		}
	}
	t.Fatalf("tank for player %d not found", playerID)
	return TankState{}
}

func setupStartedMatch(t *testing.T, serverURL string, difficulty int, p1Name, p2Name string) (*e2eClient, *e2eClient, string, GameStartMsg, GameStartMsg) {
	t.Helper()

	c1 := newE2EClient(t, serverURL)
	c2 := newE2EClient(t, serverURL)

	c1.send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: difficulty, PlayerName: p1Name})
	roomCreated := decodeRaw[RoomCreatedMsg](t, c1.recvExpect(MsgTypeRoomCreated))
	if roomCreated.RoomCode == "" {
		t.Fatal("expected non-empty room code")
	}

	joined1 := decodeRaw[JoinedMsg](t, c1.recvExpect(MsgTypeJoined))
	if joined1.PlayerID != 1 {
		t.Fatalf("host player_id = %d, want 1", joined1.PlayerID)
	}
	if joined1.ProtocolVersion != ProtocolVersion {
		t.Fatalf("host joined protocol_version = %q, want %q", joined1.ProtocolVersion, ProtocolVersion)
	}

	c2.send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCreated.RoomCode, PlayerName: p2Name})
	joined2 := decodeRaw[JoinedMsg](t, c2.recvExpect(MsgTypeJoined))
	if joined2.PlayerID != 2 {
		t.Fatalf("joiner player_id = %d, want 2", joined2.PlayerID)
	}
	if joined2.OpponentName != p1Name {
		t.Fatalf("joiner opponent_name = %q, want %q", joined2.OpponentName, p1Name)
	}
	if joined2.ProtocolVersion != ProtocolVersion {
		t.Fatalf("joiner joined protocol_version = %q, want %q", joined2.ProtocolVersion, ProtocolVersion)
	}

	hostJoinedAfterJoin := decodeRaw[JoinedMsg](t, c1.recvUntil(MsgTypeJoined, 30))
	if hostJoinedAfterJoin.OpponentName != p2Name {
		t.Fatalf("host opponent_name = %q, want %q", hostJoinedAfterJoin.OpponentName, p2Name)
	}

	c1.send(MsgTypeReady, ReadyMsg{})
	c2.send(MsgTypeReady, ReadyMsg{})

	start1 := decodeRaw[GameStartMsg](t, c1.recvUntil(MsgTypeGameStart, 120))
	start2 := decodeRaw[GameStartMsg](t, c2.recvUntil(MsgTypeGameStart, 120))

	return c1, c2, roomCreated.RoomCode, start1, start2
}

func waitForTickSilence(c *e2eClient, quietPeriod, overall time.Duration) {
	c.t.Helper()

	quietDeadline := time.Now().Add(quietPeriod)
	overallDeadline := time.Now().Add(overall)

	for time.Now().Before(overallDeadline) {
		msgType, _, err := c.recvWithTimeout(150 * time.Millisecond)
		if err != nil {
			if expectTimeoutErr(err) {
				if time.Now().After(quietDeadline) {
					return
				}
				continue
			}
			// Connection closed by server (room closed, game over, etc.) counts as silence
			return
		}

		if msgType == MsgTypeTick {
			quietDeadline = time.Now().Add(quietPeriod)
		}
	}

	c.t.Fatalf("ticks did not become silent within %s", overall)
}

func TestE2EFullMatchLifecycle(t *testing.T) {
	ts, _ := setupTestServer(t)
	c1, c2, _, start1, start2 := setupStartedMatch(t, ts.URL, 5, "alice", "bob")

	if start1.YourPlayerID != 1 {
		t.Fatalf("start1.YourPlayerID = %d, want 1", start1.YourPlayerID)
	}
	if start2.YourPlayerID != 2 {
		t.Fatalf("start2.YourPlayerID = %d, want 2", start2.YourPlayerID)
	}
	requireGridShape(t, start1.Grid, 21, 40)
	requireGridShape(t, start2.Grid, 21, 40)
	if start1.Tanks[0].PlayerID != 1 || start1.Tanks[1].PlayerID != 2 {
		t.Fatalf("start1 tank player IDs = [%d,%d], want [1,2]", start1.Tanks[0].PlayerID, start1.Tanks[1].PlayerID)
	}

	for i := 0; i < 5; i++ {
		tick1 := decodeRaw[TickMsg](t, c1.recvUntil(MsgTypeTick, 120))
		tick2 := decodeRaw[TickMsg](t, c2.recvUntil(MsgTypeTick, 120))
		requireTickShape(t, tick1)
		requireTickShape(t, tick2)
	}

	for i := 0; i < 8; i++ {
		c1.send(MsgTypeInput, InputMsg{Tick: uint64(i + 1), Key: "fire", Action: "down"})
		c1.send(MsgTypeInput, InputMsg{Tick: uint64(i + 1), Key: "right", Action: "down"})
	}

	sawProgress := false
	for i := 0; i < 200; i++ {
		msgType, payload := c1.recv()
		switch msgType {
		case MsgTypeTick:
			tick := decodeRaw[TickMsg](t, payload)
			requireTickShape(t, tick)
			sawProgress = true
		case MsgTypeRoundOver:
			_ = decodeRaw[RoundOverMsg](t, payload)
			return
		case MsgTypeGameOver:
			_ = decodeRaw[GameOverMsg](t, payload)
			return
		}
	}

	if !sawProgress {
		t.Fatal("expected tick or terminal progression after input")
	}
}

func TestE2EForcedGameOver(t *testing.T) {
	ts, handler := setupTestServer(t)
	c1, c2, roomCode, _, _ := setupStartedMatch(t, ts.URL, 5, "alice", "bob")

	_ = decodeRaw[TickMsg](t, c1.recvUntil(MsgTypeTick, 120))
	c2.closeNow()

	// With reconnect: player 2 disconnect causes RoomWaitingReconnect
	// Player 1 receives PlayerDisconnectedMsg, not immediate room closure
	// Wait for the PlayerDisconnectedMsg or OpponentLeftMsg
	msgType, _, err := c1.recvWithTimeout(5 * time.Second)
	if err != nil {
		t.Fatalf("expected message after disconnect: %v", err)
	}
	if msgType != MsgTypePlayerDisconnected && msgType != MsgTypeOpponentLeft {
		t.Fatalf("expected player_disconnected or opponent_left, got %s", msgType)
	}

	c1.closeNow()
	waitForCondition(t, 5*time.Second, func() bool {
		room := handler.Hub.GetRoom(roomCode)
		return room == nil || room.GetState() == RoomClosed
	}, "room closed after both disconnect")

	time.Sleep(100 * time.Millisecond)

	c3 := newE2EClient(t, ts.URL)
	c3.send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCode, PlayerName: "newbie"})
	msgType2, payload := c3.recv()
	if msgType2 != MsgTypeError {
		t.Fatalf("msg type = %q, want %q", msgType2, MsgTypeError)
	}
	errMsg := decodeRaw[ErrorMsg](t, payload)
	if errMsg.Code != ErrCodeRoomNotFound && errMsg.Code != ErrCodeRoomFull {
		t.Fatalf("error code = %q, want %q or %q", errMsg.Code, ErrCodeRoomNotFound, ErrCodeRoomFull)
	}
}

func TestE2EInputAffectsState(t *testing.T) {
	ts, _ := setupTestServer(t)
	c1, _, _, start1, _ := setupStartedMatch(t, ts.URL, 5, "alice", "bob")

	startTank := findTankByPlayerID(t, start1.Tanks, 1)

	keys := []string{"right", "down", "left", "up", "up_right", "down_right", "down_left", "up_left"}
	deadline := time.Now().Add(2 * time.Second)
	moved := false

	for _, key := range keys {
		c1.send(MsgTypeInput, InputMsg{Tick: 1, Key: key, Action: "down"})
		for i := 0; i < 20 && time.Now().Before(deadline); i++ {
			payload := c1.recvUntil(MsgTypeTick, 120)
			tick := decodeRaw[TickMsg](t, payload)
			p1 := findTankByPlayerID(t, tick.Tanks, 1)
			if p1.X != startTank.X || p1.Y != startTank.Y {
				moved = true
				break
			}
		}
		c1.send(MsgTypeInput, InputMsg{Tick: 1, Key: key, Action: "up"})
		if moved {
			break
		}
	}

	if !moved {
		t.Fatalf("player 1 tank position did not change from start (%d,%d)", startTank.X, startTank.Y)
	}
}

func TestE2EMultipleRooms(t *testing.T) {
	ts, _ := setupTestServer(t)

	a1 := newE2EClient(t, ts.URL)
	a2 := newE2EClient(t, ts.URL)
	b1 := newE2EClient(t, ts.URL)
	b2 := newE2EClient(t, ts.URL)

	a1.send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 3, PlayerName: "a1"})
	roomA := decodeRaw[RoomCreatedMsg](t, a1.recvExpect(MsgTypeRoomCreated)).RoomCode
	_ = decodeRaw[JoinedMsg](t, a1.recvExpect(MsgTypeJoined))

	b1.send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 8, PlayerName: "b1"})
	roomB := decodeRaw[RoomCreatedMsg](t, b1.recvExpect(MsgTypeRoomCreated)).RoomCode
	_ = decodeRaw[JoinedMsg](t, b1.recvExpect(MsgTypeJoined))

	a2.send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomA, PlayerName: "a2"})
	_ = decodeRaw[JoinedMsg](t, a2.recvExpect(MsgTypeJoined))
	_ = decodeRaw[JoinedMsg](t, a1.recvUntil(MsgTypeJoined, 60))

	b2.send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomB, PlayerName: "b2"})
	_ = decodeRaw[JoinedMsg](t, b2.recvExpect(MsgTypeJoined))
	_ = decodeRaw[JoinedMsg](t, b1.recvUntil(MsgTypeJoined, 60))

	a1.send(MsgTypeReady, ReadyMsg{})
	a2.send(MsgTypeReady, ReadyMsg{})
	b1.send(MsgTypeReady, ReadyMsg{})
	b2.send(MsgTypeReady, ReadyMsg{})

	aStart := decodeRaw[GameStartMsg](t, a1.recvUntil(MsgTypeGameStart, 120))
	_ = decodeRaw[GameStartMsg](t, a2.recvUntil(MsgTypeGameStart, 120))
	bStart := decodeRaw[GameStartMsg](t, b1.recvUntil(MsgTypeGameStart, 120))
	_ = decodeRaw[GameStartMsg](t, b2.recvUntil(MsgTypeGameStart, 120))

	if aStart.Difficulty != 3 {
		t.Fatalf("room A difficulty = %d, want 3", aStart.Difficulty)
	}
	if bStart.Difficulty != 8 {
		t.Fatalf("room B difficulty = %d, want 8", bStart.Difficulty)
	}

	_ = decodeRaw[TickMsg](t, a1.recvUntil(MsgTypeTick, 120))
	_ = decodeRaw[TickMsg](t, a2.recvUntil(MsgTypeTick, 120))
	_ = decodeRaw[TickMsg](t, b1.recvUntil(MsgTypeTick, 120))
	_ = decodeRaw[TickMsg](t, b2.recvUntil(MsgTypeTick, 120))

	a1.closeNow()
	a2.closeNow()

	for i := 0; i < 3; i++ {
		tickB := decodeRaw[TickMsg](t, b1.recvUntil(MsgTypeTick, 120))
		if tickB.Tick == 0 {
			t.Fatal("expected room B to continue ticking")
		}
	}
}

func TestE2ERoomCleanupAfterMatch(t *testing.T) {
	ts, handler := setupTestServer(t)
	c1 := newE2EClient(t, ts.URL)
	c2 := newE2EClient(t, ts.URL)

	c1.send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 5, PlayerName: "alice"})
	roomCode := decodeRaw[RoomCreatedMsg](t, c1.recvExpect(MsgTypeRoomCreated)).RoomCode
	_ = decodeRaw[JoinedMsg](t, c1.recvExpect(MsgTypeJoined))

	c2.send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCode, PlayerName: "bob"})
	_ = decodeRaw[JoinedMsg](t, c2.recvExpect(MsgTypeJoined))
	_ = decodeRaw[JoinedMsg](t, c1.recvUntil(MsgTypeJoined, 60))

	c1.closeNow()
	c2.closeNow()
	time.Sleep(100 * time.Millisecond)

	waitForCondition(t, 10*time.Second, func() bool {
		return handler.Hub.RoomCount() == 0
	}, fmt.Sprintf("hub room count becomes zero (room=%s)", roomCode))
}
