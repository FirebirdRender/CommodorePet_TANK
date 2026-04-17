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

// runBotClient connects to the server as a regular WS client,
// joins the room, and runs a minimal "dumb bot" loop that just
// sends occasional fire inputs to keep the game alive.
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

	writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
	err = conn.Write(writeCtx, websocket.MessageText, rejoinEnv)
	writeCancel()
	if err != nil {
		log.Printf("[bot] rejoin send failed for room %s: %v", roomCode, err)
		return
	}

	log.Printf("[bot] connected to room %s as player %d", roomCode, playerID)

	// Dumb bot loop: read messages, occasionally send random inputs
	// Real AI will come from cmd/bot-go; this proves the lifecycle works
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	currentTick := uint64(0)

	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send a random direction input every 500ms
			// This is the Stage A "fallback legal random move" from the AI design
			keys := []string{"up", "down", "left", "right", "fire"}
			key := keys[time.Now().UnixNano()%5]

			inputPayload, _ := json.Marshal(map[string]any{
				"tick":   currentTick,
				"key":    key,
				"action": "down",
			})
			inputEnv, _ := json.Marshal(map[string]any{
				"type":    "input",
				"payload": json.RawMessage(inputPayload),
			})

			writeCtx, writeCancel := context.WithTimeout(ctx, 2*time.Second)
			conn.Write(writeCtx, websocket.MessageText, inputEnv)
			writeCancel()

			// Send key up after a brief moment
			upPayload, _ := json.Marshal(map[string]any{
				"tick":   currentTick,
				"key":    key,
				"action": "up",
			})
			upEnv, _ := json.Marshal(map[string]any{
				"type":    "input",
				"payload": json.RawMessage(upPayload),
			})

			upCtx, upCancel := context.WithTimeout(ctx, 2*time.Second)
			conn.Write(upCtx, websocket.MessageText, upEnv)
			upCancel()

			currentTick += 30 // ~500ms at 60Hz

		default:
			// Try to read a message (non-blocking-ish)
			readCtx, readCancel := context.WithTimeout(ctx, 10*time.Millisecond)
			_, msg, err := conn.Read(readCtx)
			readCancel()
			if err == nil {
				// Parse envelope to track tick number
				var env struct {
					Type    string          `json:"type"`
					Payload json.RawMessage `json:"payload"`
				}
				if json.Unmarshal(msg, &env) == nil {
					if env.Type == "tick" || env.Type == "tick_delta" {
						var tickData struct {
							Tick uint64 `json:"tick"`
						}
						if json.Unmarshal(env.Payload, &tickData) == nil {
							currentTick = tickData.Tick
						}
					} else if env.Type == "game_over" {
						log.Printf("[bot] game over in room %s", roomCode)
						// Send play_again
						playAgainEnv, _ := json.Marshal(map[string]any{
							"type":    "play_again",
							"payload": map[string]any{},
						})
						paCtx, paCancel := context.WithTimeout(ctx, 2*time.Second)
						conn.Write(paCtx, websocket.MessageText, playAgainEnv)
						paCancel()
					} else if env.Type == "opponent_left" {
						log.Printf("[bot] opponent left room %s, exiting", roomCode)
						return
					} else if env.Type == "error" {
						log.Printf("[bot] error in room %s: %s", roomCode, string(env.Payload))
						return
					}
				}
			}
		}
	}
}
