package main

import (
	"context"
	"encoding/json"
	"errors"
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
	serverURL := flag.String("server", "ws://localhost:8080/ws", "WebSocket server URL")
	roomCode := flag.String("room", "", "Room code (auto-create if not provided)")
	playerName := flag.String("name", "CPU", "Bot player name")
	token := flag.String("token", "", "Auth token (auto-create if not provided)")
	playerID := flag.Int("player-id", 1, "Player ID (1 or 2)")
	flag.Parse()

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

	client := botsdk.NewClient(*serverURL, rcode, pid, tok, *playerName)

	var botState *BotState
	var tickCount uint64
	var droppedInputs uint64

	client.OnGameStart = func(msg *botsdk.GameStartPayload) {
		log.Printf("Game start - difficulty: %d, size: %dx%d, your_player_id=%d",
			msg.Difficulty, len(msg.Grid[0]), len(msg.Grid), msg.YourPlayerID)
		botState = NewBotState(msg.YourPlayerID, msg.Difficulty, len(msg.Grid[0]), len(msg.Grid))
		botState.LoadKeyframe(msg.Grid)
		tickCount = 0
		droppedInputs = 0
	}

	shouldLog := func(n uint64) bool {
		return n < 10 || n%30 == 0
	}

	logTick := func(source string, tick uint64, tanks [2]botsdk.TankInfo, actions []InputAction) {
		if botState == nil || !shouldLog(tickCount) {
			return
		}
		var me, foe botsdk.TankInfo
		for _, t := range tanks {
			if t.PlayerID == botState.MyID {
				me = t
			} else {
				foe = t
			}
		}
		actStr := "none"
		if len(actions) > 0 {
			parts := make([]string, 0, len(actions))
			for _, a := range actions {
				parts = append(parts, a.Key+"/"+a.Action)
			}
			actStr = strings.Join(parts, ",")
		}
		log.Printf("[t%d %s n=%d] me=(%d,%d) act=%v sh=%d mi=%d foe=(%d,%d) act=%v pend=%s | %s",
			tick, source, tickCount,
			me.X, me.Y, me.Active, me.ShotsLeft, me.MinesLeft,
			foe.X, foe.Y, foe.Active,
			botState.PendingDesc(),
			actStr)
	}

	sendActions := func(tick uint64, actions []InputAction) {
		for _, a := range actions {
			if err := client.SendInput(tick, a.Key, a.Action); err != nil {
				if errors.Is(err, botsdk.ErrSendBufferFull) {
					droppedInputs++
					if droppedInputs%10 == 1 {
						log.Printf("Send buffer full tick=%d (%s/%s) dropped_total=%d", tick, a.Key, a.Action, droppedInputs)
					}
					continue
				}
				log.Printf("Send input error tick=%d (%s/%s): %v", tick, a.Key, a.Action, err)
			}
		}
	}

	client.OnTick = func(msg *botsdk.TickPayload) {
		if botState == nil {
			return
		}
		tickCount++
		botState.LoadKeyframe(msg.Grid)
		actions := botState.Decide(msg.Tick, msg.Tanks)
		logTick("kf", msg.Tick, msg.Tanks, actions)
		sendActions(msg.Tick, actions)
	}

	client.OnTickDelta = func(msg *botsdk.TickDeltaPayload) {
		if botState == nil {
			return
		}
		tickCount++
		botState.ApplyDelta(msg.ChangedCells)
		actions := botState.Decide(msg.Tick, msg.Tanks)
		logTick("dl", msg.Tick, msg.Tanks, actions)
		sendActions(msg.Tick, actions)
	}

	client.OnRoundOver = func(msg *botsdk.RoundOverPayload) {
		log.Printf("Round over - winner: %d (P1: %d wins, P2: %d wins)", msg.Winner, msg.Wins[0], msg.Wins[1])
	}

	client.OnGameOver = func(msg *botsdk.GameOverPayload) {
		log.Printf("Game over - winner: %d (final: P1 %d, P2 %d) dropped_inputs=%d",
			msg.Winner, msg.FinalWins[0], msg.FinalWins[1], droppedInputs)
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

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Connect error: %v", err)
	}
	log.Printf("Bot connected to room %s as player %d", rcode, pid)
	if err := client.Run(ctx); err != nil {
		log.Printf("Run error: %v", err)
	}
}

func createRoomViaAPI(serverURL, playerName string) (roomCode string, playerID int, token string, err error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", 0, "", fmt.Errorf("parse server URL: %w", err)
	}

	var httpScheme string
	if u.Scheme == "ws" {
		httpScheme = "http"
	} else if u.Scheme == "wss" {
		httpScheme = "https"
	} else {
		httpScheme = u.Scheme
	}

	httpURL := fmt.Sprintf("%s://%s/api/room", httpScheme, u.Host)
	body := fmt.Sprintf(`{"difficulty":5,"player_name":"%s","vs_ai":true}`, playerName)

	req, err := http.NewRequest("POST", httpURL, strings.NewReader(body))
	if err != nil {
		return "", 0, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, "", fmt.Errorf("API error: %d", resp.StatusCode)
	}

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
