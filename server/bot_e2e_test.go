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

func setupTestServerWithBot(t *testing.T) (*httptest.Server, *WSHandler, *BotManager) {
	t.Helper()

	origEnv := os.Getenv("TANK_ENABLE_BOTS")
	os.Setenv("TANK_ENABLE_BOTS", "1")
	defer os.Setenv("TANK_ENABLE_BOTS", origEnv)

	hub := NewHub()
	tokens := NewTokenStore()
	handler := NewWSHandler(hub, tokens)

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	serverAddr := strings.TrimPrefix(ts.URL, "http://")

	botManager := NewBotManager(hub, handler, tokens, serverAddr)

	return ts, handler, botManager
}

func waitForBotJoin(t *testing.T, hub *Hub, roomCode string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		room := hub.GetRoom(roomCode)
		if room != nil {
			if room.PlayerCount() >= 2 {
				return true
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

func TestBot_VSAIRoomCreation(t *testing.T) {
	ts, handler, botManager := setupTestServerWithBot(t)
	defer ts.Close()

	roomAPI := NewRoomAPI(handler.Hub, handler.Registry(), handler.tokens)
	roomAPI.SetBotManager(botManager)

	reqBody := map[string]any{
		"difficulty":    5,
		"player_name":   "HumanPlayer",
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
	if roomCode == "" {
		t.Fatal("no room code in response")
	}

	room := handler.Hub.GetRoom(roomCode)
	if room == nil {
		t.Fatal("room not found")
	}
	if !room.AllowBot {
		t.Errorf("room AllowBot = false, want true")
	}

	if !waitForBotJoin(t, handler.Hub, roomCode, 3*time.Second) {
		t.Fatalf("bot did not join within timeout")
	}

	if room.PlayerCount() != 2 {
		t.Errorf("room player count = %d, want 2", room.PlayerCount())
	}

	foundBot := false
	for _, p := range room.Players {
		if p != nil && p.IsBot {
			foundBot = true
		}
	}
	if !foundBot {
		t.Errorf("no bot player found")
	}
}

func TestBot_BotJoinsViaWS(t *testing.T) {
	_, handler, botManager := setupTestServerWithBot(t)

	roomAPI := NewRoomAPI(handler.Hub, handler.Registry(), handler.tokens)
	roomAPI.SetBotManager(botManager)

	reqBody := map[string]any{
		"difficulty":    7,
		"player_name":   "TestUser",
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

	if !waitForBotJoin(t, handler.Hub, roomCode, 3*time.Second) {
		t.Fatalf("bot did not join within timeout")
	}

	room := handler.Hub.GetRoom(roomCode)
	if room == nil {
		t.Fatal("room not found")
	}
	if room.PlayerCount() != 2 {
		t.Errorf("room player count = %d, want 2", room.PlayerCount())
	}
}

func TestBot_DisconnectHandling(t *testing.T) {
	_, handler, botManager := setupTestServerWithBot(t)

	roomAPI := NewRoomAPI(handler.Hub, handler.Registry(), handler.tokens)
	roomAPI.SetBotManager(botManager)

	reqBody := map[string]any{
		"difficulty":    3,
		"player_name":   "PlayerOne",
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

	if !waitForBotJoin(t, handler.Hub, roomCode, 3*time.Second) {
		t.Fatalf("bot did not join within timeout")
	}

	botManager.ReleaseBot(roomCode)

	room := handler.Hub.GetRoom(roomCode)
	if room == nil {
		t.Fatal("room not found")
	}
}

func TestBot_RoomStatusIncludesBotInfo(t *testing.T) {
	ts, handler, botManager := setupTestServerWithBot(t)
	defer ts.Close()

	roomAPI := NewRoomAPI(handler.Hub, handler.Registry(), handler.tokens)
	roomAPI.SetBotManager(botManager)

	reqBody := map[string]any{
		"difficulty":          5,
		"player_name":         "TestPlayer",
		"vs_ai":               false,
		"auto_fill_bot":       false,
		"auto_fill_after_sec": 0,
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

	room := handler.Hub.GetRoom(roomCode)
	if room == nil {
		t.Fatal("room not found")
	}

	room.mu.Lock()
	allowBot := room.AllowBot
	autoFillBot := room.AutoFillBot
	autoFillAfterSec := room.AutoFillAfterSec
	room.mu.Unlock()

	if allowBot {
		t.Errorf("allow_bot = true, want false (vs_ai=false)")
	}
	if autoFillBot {
		t.Errorf("auto_fill_bot = true, want false (auto_fill_bot=false)")
	}
	if autoFillAfterSec != 0 {
		t.Errorf("auto_fill_after_sec = %v, want 0", autoFillAfterSec)
	}

	if err := room.ReserveBotSeat(); err != nil {
		t.Fatalf("reserve bot seat: %v", err)
	}

	_, err := room.AddBotPlayer("CPU-MVP", "test-bot-123", "mvp")
	if err != nil {
		t.Fatalf("add bot player: %v", err)
	}

	foundBot := false
	room.mu.Lock()
	for _, p := range room.Players {
		if p != nil && p.IsBot {
			foundBot = true
			if p.Name != "CPU-MVP" {
				t.Errorf("bot name = %v, want CPU-MVP", p.Name)
			}
		}
	}
	room.mu.Unlock()

	if !foundBot {
		t.Errorf("bot player not found in players")
	}
}

func TestBot_AutoFillTimerCancelledOnHumanJoin(t *testing.T) {
	_, handler, botManager := setupTestServerWithBot(t)

	roomAPI := NewRoomAPI(handler.Hub, handler.Registry(), handler.tokens)
	roomAPI.SetBotManager(botManager)

	reqBody := map[string]any{
		"difficulty":          4,
		"player_name":         "FirstPlayer",
		"vs_ai":               false,
		"auto_fill_bot":       true,
		"auto_fill_after_sec": 5,
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

	reqBody2 := map[string]any{
		"player_name": "SecondPlayer",
	}
	bodyBytes2, _ := json.Marshal(reqBody2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/room/"+roomCode+"/join", strings.NewReader(string(bodyBytes2)))
	w2 := httptest.NewRecorder()

	roomAPI.handleJoinRoom(w2, req2, roomCode)

	if w2.Code != http.StatusOK {
		t.Fatalf("join room failed: %d %s", w2.Code, w2.Body.String())
	}

	var resp2 map[string]any
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatal(err)
	}

	if playerID, _ := resp2["player_id"].(float64); playerID != 2 {
		t.Errorf("human player_id = %v, want 2", playerID)
	}

	room := handler.Hub.GetRoom(roomCode)
	if room == nil {
		t.Fatal("room not found")
	}

	room.mu.Lock()
	reserved := room.BotSeatReserved
	autoFillTimer := room.autoFillTimer
	room.mu.Unlock()

	if reserved {
		t.Error("bot seat still reserved after human joined")
	}

	if autoFillTimer != nil {
		t.Error("auto-fill timer still exists after human joined")
	}

	if room.Players[1] == nil || room.Players[1].IsBot {
		t.Errorf("seat 2 should be human player, but is: %v", room.Players[1])
	}
}
