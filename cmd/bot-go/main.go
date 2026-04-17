package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	botsdk "github.com/FirebirdRender/CommodorePet_TANK/bot-sdk-go"
)

func main() {
	// Parse command-line flags
	serverURL := flag.String("server", "ws://localhost:8080/ws", "WebSocket server URL")
	roomCode := flag.String("room", "", "Room code (auto-create if not provided)")
	playerName := flag.String("name", "CPU", "Bot player name")
	token := flag.String("token", "", "Auth token (auto-create if not provided)")
	playerID := flag.Int("player-id", 1, "Player ID (1 or 2)")
	flag.Parse()

	// If no room/token provided, create one via HTTP API
	var rcode string
	var pid int
	var tok string

	if *roomCode == "" || *token == "" {
		rc, p, t, err := createRoomViaAPI(*serverURL, *playerName)
		if err != nil {
			log.Fatalf("Failed to create room: %v", err)
		}
		rcode = rc
		pid = p
		tok = t
		log.Printf("Created room: %s, player_id: %d, token: %s", rcode, pid, tok)
	} else {
		rcode = *roomCode
		pid = *playerID
		tok = *token
	}

	// Connect using bot-sdk-go client
	client := botsdk.NewClient(*serverURL, rcode, pid, tok, *playerName)

	var botState *BotState

	client.OnGameStart = func(msg *botsdk.GameStartPayload) {
		log.Printf("Game start - difficulty: %d, size: %dx%d, your_player_id=%d", msg.Difficulty, len(msg.Grid[0]), len(msg.Grid), msg.YourPlayerID)
		botState = NewBotState(msg.YourPlayerID, msg.Difficulty, len(msg.Grid[0]), len(msg.Grid))
		botState.LoadKeyframe(msg.Grid)
	}

	sendActions := func(tick uint64, actions []InputAction) {
		for _, a := range actions {
			if err := client.SendInput(tick, a.Key, a.Action); err != nil {
				log.Printf("Send input error (%s/%s): %v", a.Key, a.Action, err)
			}
		}
	}

	client.OnTick = func(msg *botsdk.TickPayload) {
		if botState == nil {
			return
		}
		botState.LoadKeyframe(msg.Grid)
		sendActions(msg.Tick, botState.Decide(msg.Tanks))
	}

	client.OnTickDelta = func(msg *botsdk.TickDeltaPayload) {
		if botState == nil {
			return
		}
		botState.ApplyDelta(msg.ChangedCells)
		sendActions(msg.Tick, botState.Decide(msg.Tanks))
	}

	client.OnRoundOver = func(msg *botsdk.RoundOverPayload) {
		log.Printf("Round over - winner: %d (P1: %d wins, P2: %d wins)", msg.Winner, msg.Wins[0], msg.Wins[1])
	}

	client.OnGameOver = func(msg *botsdk.GameOverPayload) {
		log.Printf("Game over - winner: %d (final: P1 %d, P2 %d)", msg.Winner, msg.FinalWins[0], msg.FinalWins[1])
		// Send play_again message
		if err := client.SendPlayAgain(); err != nil {
			log.Printf("Send play_again error: %v", err)
		}
	}

	client.OnOpponentLeft = func(reason string) {
		log.Printf("Opponent left: %s", reason)
		os.Exit(0)
	}

	client.OnError = func(code string, message string) {
		log.Printf("Error %s: %s", code, message)
		os.Exit(1)
	}

	// Run the client loop
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Connect error: %v", err)
	}
	log.Printf("Bot connected to room %s as player %d", rcode, pid)
	if err := client.Run(ctx); err != nil {
		log.Printf("Run error: %v", err)
	}
}

// createRoomViaAPI creates a room via the HTTP API
func createRoomViaAPI(serverURL, playerName string) (roomCode string, playerID int, token string, err error) {
	// Parse server URL to get base HTTP URL
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", 0, "", fmt.Errorf("parse server URL: %w", err)
	}

	// Change ws:// to http://
	var httpScheme string
	if u.Scheme == "ws" {
		httpScheme = "http"
	} else if u.Scheme == "wss" {
		httpScheme = "https"
	} else {
		httpScheme = u.Scheme
	}

	httpURL := fmt.Sprintf("%s://%s/api/room", httpScheme, u.Host)

	// POST to /api/room with vs_ai=true
	body := fmt.Sprintf(`{"difficulty":5,"player_name":"%s","vs_ai":true}`, playerName)

	req, err := http.NewRequest("POST", httpURL, strings.NewReader(body))
	if err != nil {
		return "", 0, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", 0, "", fmt.Errorf("API error: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		RoomCode string `json:"room_code"`
		PlayerID int    `json:"player_id"`
		Token    string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, "", fmt.Errorf("decode response: %w", err)
	}

	return result.RoomCode, result.PlayerID, result.Token, nil
}
