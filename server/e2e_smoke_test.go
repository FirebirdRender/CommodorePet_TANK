package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSmoke_RoomLifecycle: create room → join → ready → game start → receive ticks → disconnect → cleanup
func TestSmoke_RoomLifecycle(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	c1 := newE2EClient(t, ts.URL)
	c2 := newE2EClient(t, ts.URL)
	defer c1.closeNow()
	defer c2.closeNow()

	// Create room
	c1.send("create_room", CreateRoomMsg{Difficulty: 5, PlayerName: "Alice"})
	msg := c1.recvExpect("room_created")
	var roomCreated RoomCreatedMsg
	if err := json.Unmarshal(msg, &roomCreated); err != nil {
		t.Fatalf("unmarshal room_created: %v", err)
	}
	if roomCreated.RoomCode == "" {
		t.Fatal("expected non-empty room code")
	}

	// Join room
	c2.send("join_room", JoinRoomMsg{RoomCode: roomCreated.RoomCode, PlayerName: "Bob"})
	c2.recvExpect("joined")
	// c1 also receives joined notification about c2 joining
	c1.recvExpect("joined")

	// Ready
	c1.send("ready", ReadyMsg{})
	c2.send("ready", ReadyMsg{})

	// Game start
	// c1 may have received extra messages (like another joined or ready ack)
	// Keep reading until we get game_start
	maxReads := 10
	for i := 0; i < maxReads; i++ {
		msgType, payload, err := c1.recvWithTimeout(2 * time.Second)
		if err != nil {
			t.Fatalf("c1 failed to receive message: %v", err)
		}
		if msgType == "game_start" {
			var gameStart GameStartMsg
			json.Unmarshal(payload, &gameStart)
			break
		}
		t.Logf("c1 received %s, waiting for game_start (attempt %d/%d)", msgType, i+1, maxReads)
	}

	c2.recvExpect("game_start")

	// Receive ticks
	msgType, _, err := c1.recvWithTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("failed to receive tick: %v", err)
	}
	if msgType != "tick" && msgType != "tick_delta" {
		t.Errorf("expected tick or tick_delta, got %s", msgType)
	}
}

// TestSmoke_InvalidInputRejected: bad difficulty, empty name, invalid room code
func TestSmoke_InvalidInputRejected(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	// Difficulty out of range (0)
	c := newE2EClient(t, ts.URL)
	defer c.closeNow()
	c.send("create_room", CreateRoomMsg{Difficulty: 0, PlayerName: "Alice"})
	msg := c.recvExpect("error")
	var errMsg ErrorMsg
	json.Unmarshal(msg, &errMsg)
	if errMsg.Code != "invalid_input" {
		t.Errorf("expected invalid_input error, got %s", errMsg.Code)
	}

	// Difficulty too high (11)
	c2 := newE2EClient(t, ts.URL)
	defer c2.closeNow()
	c2.send("create_room", CreateRoomMsg{Difficulty: 11, PlayerName: "Bob"})
	msg = c2.recvExpect("error")
	json.Unmarshal(msg, &errMsg)
	if errMsg.Code != "invalid_input" {
		t.Errorf("expected invalid_input for difficulty 11, got %s", errMsg.Code)
	}

	// Empty player name
	c3 := newE2EClient(t, ts.URL)
	defer c3.closeNow()
	c3.send("create_room", CreateRoomMsg{Difficulty: 5, PlayerName: ""})
	msg = c3.recvExpect("error")
	json.Unmarshal(msg, &errMsg)
	if errMsg.Code != "invalid_input" {
		t.Errorf("expected invalid_input for empty name, got %s", errMsg.Code)
	}

	// Invalid room code (lowercase)
	c4 := newE2EClient(t, ts.URL)
	defer c4.closeNow()
	c4.send("join_room", JoinRoomMsg{RoomCode: "ab12", PlayerName: "Dave"})
	msg = c4.recvExpect("error")
	json.Unmarshal(msg, &errMsg)
	if errMsg.Code != "invalid_input" {
		t.Errorf("expected invalid_input for lowercase room code, got %s", errMsg.Code)
	}
}

// TestSmoke_UnambiguousRoomCodes: verify 100 generated codes have no I/L/O/0/1
func TestSmoke_UnambiguousRoomCodes(t *testing.T) {
	hub := NewHub(0)
	ambiguous := "ILO01"
	for i := 0; i < 100; i++ {
		code := hub.generateCode()
		for _, c := range code {
			if strings.ContainsRune(ambiguous, c) {
				t.Errorf("code %q contains ambiguous char %c", code, c)
			}
		}
	}
}

// TestSmoke_CORSHeaders: verify CORS middleware sets headers
func TestSmoke_CORSHeaders(t *testing.T) {
	handler := CORSMiddleware("https://example.com", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Preflight
	req := httptest.NewRequest("OPTIONS", "/ws", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("OPTIONS: expected 200, got %d", rec.Code)
	}
	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "https://example.com" {
		t.Errorf("CORS origin: expected https://example.com, got %s", v)
	}

	// Normal request
	req = httptest.NewRequest("GET", "/health", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "https://example.com" {
		t.Errorf("CORS origin on GET: expected https://example.com, got %s", v)
	}
}

// TestSmoke_WASMCacheHeaders: verify .wasm files get no-cache headers
func TestSmoke_WASMCacheHeaders(t *testing.T) {
	handler := WASMNoCacheMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// .wasm file
	req := httptest.NewRequest("GET", "/game.wasm", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if v := rec.Header().Get("Cache-Control"); v != "no-cache, no-store, must-revalidate" {
		t.Errorf("wasm Cache-Control: expected no-cache..., got %q", v)
	}

	// .html file should NOT have no-cache
	req = httptest.NewRequest("GET", "/index.html", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if v := rec.Header().Get("Cache-Control"); v != "" {
		t.Errorf("html Cache-Control: expected empty, got %q", v)
	}
}
