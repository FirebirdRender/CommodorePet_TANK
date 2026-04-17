package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type BotAssignment struct {
	RoomCode string
	BotID    string
	BotClass string
	Done     chan struct{}
}

type BotManager struct {
	hub        *Hub
	handler    *WSHandler
	tokens     *TokenStore
	serverAddr string
	enabled    bool

	mu     sync.Mutex
	active map[string]*BotAssignment // roomCode -> assignment
}

func NewBotManager(hub *Hub, handler *WSHandler, tokens *TokenStore, serverAddr string) *BotManager {
	enabled := os.Getenv("TANK_ENABLE_BOTS") == "1"
	return &BotManager{
		hub:        hub,
		handler:    handler,
		tokens:     tokens,
		serverAddr: serverAddr,
		enabled:    enabled,
		active:     make(map[string]*BotAssignment),
	}
}

func (bm *BotManager) IsEnabled() bool {
	return bm.enabled
}

// AssignBotToRoom reserves the bot seat and starts a goroutine that:
// 1. Dials WS to the server
// 2. Sends rejoin message for the given room code
// 3. Enters a minimal keep-alive loop (tick consumption + empty input)
// The actual AI logic comes from cmd/bot-go, but for M1 we need a "dumb bot"
// that just joins and keeps the match alive.
func (bm *BotManager) AssignBotToRoom(roomCode string, botClass string) error {
	if !bm.enabled {
		return fmt.Errorf("bot mode not available")
	}

	room := bm.hub.GetRoom(roomCode)
	if room == nil {
		return fmt.Errorf("room not found: %s", roomCode)
	}

	// Reserve seat 2 for bot
	if err := room.ReserveBotSeat(); err != nil {
		return fmt.Errorf("reserve bot seat: %w", err)
	}

	// Add bot player to room
	botID := fmt.Sprintf("bot-%s-%d", botClass, time.Now().UnixNano())
	botName := "CPU" // PET-style name
	if botClass != "" {
		botName = fmt.Sprintf("CPU-%s", botClass)
	}

	playerID, err := room.AddBotPlayer(botName, botID, botClass)
	if err != nil {
		room.CancelBotReservation()
		return fmt.Errorf("add bot player: %w", err)
	}

	// Generate token for bot
	token := bm.tokens.GenerateToken(roomCode, playerID, botName)

	done := make(chan struct{})
	assignment := &BotAssignment{
		RoomCode: roomCode,
		BotID:    botID,
		BotClass: botClass,
		Done:     done,
	}

	bm.mu.Lock()
	bm.active[roomCode] = assignment
	bm.mu.Unlock()

	log.Printf("[bot] assigned bot %s (%s) to room %s as player %d", botID, botName, roomCode, playerID)

	// Start bot goroutine — dials WS as a regular client
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[bot] panic in bot client for room %s: %v", roomCode, r)
				bm.ReleaseBot(roomCode)
			}
		}()
		bm.runBotClient(roomCode, playerID, token, botName, done)
	}()

	return nil
}

func (bm *BotManager) ReleaseBot(roomCode string) {
	bm.mu.Lock()
	assignment, ok := bm.active[roomCode]
	if ok {
		close(assignment.Done)
		delete(bm.active, roomCode)
	}
	bm.mu.Unlock()

	if ok {
		log.Printf("[bot] released bot from room %s", roomCode)
	}
}

func (bm *BotManager) HealthSnapshot() map[string]string {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	result := map[string]string{
		"enabled":     fmt.Sprintf("%v", bm.enabled),
		"active_bots": fmt.Sprintf("%d", len(bm.active)),
	}
	return result
}

// runBotClient connects to the server as a regular WS client and sends a rejoin
// message. The server's ClientConn pumps handle all further reads/writes.
// This proves the WS-based bot architecture works end-to-end.
func (bm *BotManager) runBotClient(roomCode string, playerID int, token, botName string, done chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[bot] recovered panic in runBotClient for room %s: %v", roomCode, r)
		}
	}()
	defer bm.ReleaseBot(roomCode)

	wsURL := fmt.Sprintf("ws://%s/ws", bm.serverAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		log.Printf("[bot] WS dial failed for room %s: %v", roomCode, err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "bot done")

	// Send rejoin message to join the room
	rejoinPayload, _ := json.Marshal(map[string]any{
		"room_code":   roomCode,
		"player_id":   playerID,
		"token":       token,
		"player_name": botName,
	})
	rejoinEnv, _ := json.Marshal(map[string]any{
		"type":    "rejoin",
		"payload": json.RawMessage(rejoinPayload),
	})

	// Write the rejoin message
	err = conn.Write(ctx, websocket.MessageText, rejoinEnv)
	if err != nil {
		log.Printf("[bot] rejoin send failed for room %s: %v", roomCode, err)
		return
	}

	log.Printf("[bot] connected to room %s as player %d", roomCode, playerID)
	log.Printf("[bot] waiting for server to process rejoin and start match...")

	// Wait for the server to process and start the match.
	// The server's ClientConn pumps handle all further communication.
	// We just need to keep this goroutine alive so the connection doesn't close.
	<-done

	log.Printf("[bot] bot exiting for room %s", roomCode)
}
