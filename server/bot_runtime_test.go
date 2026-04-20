package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestBot_RuntimeBotInputAffectsState spawns the real tank-bot subprocess against
// a real server, then asserts the bot's tank actually changes state during play
// (position changes OR shots_left decrements). This is the regression guard for
// the writePump infinite re-enqueue bug fixed in 0.9.4 — without that fix the
// bot could connect, exchange handshakes, and even appear to send inputs in
// logs, while no bytes ever reached conn.Write and the tank sat motionless.
func TestBot_RuntimeBotInputAffectsState(t *testing.T) {
	if os.Getenv("TANK_BOT_BIN") == "" {
		t.Skip("TANK_BOT_BIN not set; skipping runtime bot test")
	}

	ts, handler, botManager := setupTestServerWithBot(t)
	defer ts.Close()

	roomAPI := NewRoomAPI(handler.Hub, handler.Registry(), handler.tokens, "*")
	roomAPI.SetBotManager(botManager)

	reqBody := map[string]any{
		"difficulty":    5,
		"player_name":   "RuntimeHuman",
		"vs_ai":         true,
		"auto_fill_bot": false,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/room", strings.NewReader(string(bodyBytes)))
	w := httptest.NewRecorder()
	roomAPI.handleCreateRoom(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create room failed: %d %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	roomCode, _ := resp["room_code"].(string)
	token, _ := resp["token"].(string)
	playerIDFloat, _ := resp["player_id"].(float64)
	playerID := int(playerIDFloat)

	if !waitForBotJoin(t, handler.Hub, roomCode, 3*time.Second) {
		t.Fatalf("bot did not join within timeout")
	}

	c := newE2EClient(t, ts.URL)
	c.send("rejoin", RejoinMsg{
		RoomCode:   roomCode,
		PlayerID:   playerID,
		Token:      token,
		PlayerName: "RuntimeHuman",
	})

	gotJoined := false
	gotGameStart := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !(gotJoined && gotGameStart) {
		msgType, _, err := c.recvWithTimeout(time.Until(deadline))
		if err != nil {
			break
		}
		switch msgType {
		case "joined", "rejoin_ack":
			gotJoined = true
		case "game_start":
			gotGameStart = true
		}
	}
	if !gotJoined {
		t.Fatal("never received joined/rejoin_ack")
	}

	c.send("ready", ReadyMsg{})

	if !gotGameStart {
		deadline = time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			msgType, _, err := c.recvWithTimeout(time.Until(deadline))
			if err != nil {
				t.Fatalf("waiting for game_start: %v", err)
			}
			if msgType == "game_start" {
				gotGameStart = true
				break
			}
		}
	}
	if !gotGameStart {
		t.Fatal("never received game_start")
	}

	type tankView struct {
		X, Y, Shots int
		Active      bool
	}
	var first, last tankView
	haveFirst := false
	moved := false
	fired := false

	deadline = time.Now().Add(7 * time.Second)
	for time.Now().Before(deadline) {
		msgType, payload, err := c.recvWithTimeout(time.Until(deadline))
		if err != nil {
			break
		}
		if msgType != "tick" && msgType != "tick_delta" {
			continue
		}
		var raw struct {
			Tanks []struct {
				PlayerID  int  `json:"player_id"`
				X         int  `json:"x"`
				Y         int  `json:"y"`
				ShotsLeft int  `json:"shots_left"`
				Active    bool `json:"active"`
			} `json:"tanks"`
		}
		if err := json.Unmarshal(payload, &raw); err != nil {
			continue
		}
		var bot tankView
		botFound := false
		for _, tk := range raw.Tanks {
			if tk.PlayerID != playerID {
				bot = tankView{X: tk.X, Y: tk.Y, Shots: tk.ShotsLeft, Active: tk.Active}
				botFound = true
			}
		}
		if !botFound {
			continue
		}
		if !haveFirst {
			first = bot
			haveFirst = true
		}
		last = bot
		if bot.X != first.X || bot.Y != first.Y {
			moved = true
		}
		if bot.Shots < first.Shots {
			fired = true
		}
		if moved && fired {
			break
		}
	}

	if !haveFirst {
		t.Fatal("never observed bot tank in tick stream")
	}
	t.Logf("bot start=(%d,%d) shots=%d -> end=(%d,%d) shots=%d active=%v",
		first.X, first.Y, first.Shots, last.X, last.Y, last.Shots, last.Active)
	if !moved && !fired {
		t.Fatalf("bot never moved AND never fired during runtime window: start=(%d,%d) shots=%d end=(%d,%d) shots=%d",
			first.X, first.Y, first.Shots, last.X, last.Y, last.Shots)
	}
}
