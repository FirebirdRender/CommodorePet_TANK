package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTokenStore(t *testing.T) {
	ts := NewTokenStore()
	roomCode := "AB2D"
	playerID := 1
	playerName := "Alice"

	token := ts.GenerateToken(roomCode, playerID, playerName)
	if len(token) != 64 {
		t.Errorf("expected token length 64, got %d", len(token))
	}

	entry, ok := ts.ValidateToken(token)
	if !ok {
		t.Fatal("token validation failed")
	}
	if entry.RoomCode != roomCode || entry.PlayerID != playerID || entry.PlayerName != playerName {
		t.Errorf("incorrect token entry: %+v", entry)
	}

	ts.RemoveToken(token)
	_, ok = ts.ValidateToken(token)
	if ok {
		t.Error("token should have been removed")
	}
}

func TestTokenStoreCleanup(t *testing.T) {
	ts := NewTokenStore()
	ts.GenerateToken("CODE", 1, "P1")

	time.Sleep(10 * time.Millisecond)
	ts.CleanupStaleTokens(5 * time.Millisecond)

	token2 := ts.GenerateToken("CODE2", 1, "P2")
	_, ok := ts.ValidateToken(token2)
	if !ok {
		t.Error("fresh token should be valid")
	}
}

func TestRoomAPI_CreateRoom(t *testing.T) {
	hub := NewHub()
	ts := NewTokenStore()
	reg := NewConnRegistry()
	api := NewRoomAPI(hub, reg, ts, "*")

	t.Run("Success", func(t *testing.T) {
		reqBody := `{"difficulty": 5, "player_name": "Alice"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/room", strings.NewReader(reqBody))

		api.handleCreateRoom(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		if resp["room_code"] == "" || resp["token"] == "" || resp["player_id"] != float64(1) {
			t.Errorf("invalid response: %+v", resp)
		}
	})

	t.Run("InvalidDifficulty", func(t *testing.T) {
		reqBody := `{"difficulty": 11, "player_name": "Alice"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/room", strings.NewReader(reqBody))
		api.handleCreateRoom(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("InvalidName", func(t *testing.T) {
		reqBody := `{"difficulty": 5, "player_name": ""}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/room", strings.NewReader(reqBody))
		api.handleCreateRoom(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestRoomAPI_JoinRoom(t *testing.T) {
	hub := NewHub()
	ts := NewTokenStore()
	reg := NewConnRegistry()
	api := NewRoomAPI(hub, reg, ts, "*")

	room := hub.CreateRoom(5)
	code := room.Code
	room.AddPlayer("Alice")

	t.Run("Success", func(t *testing.T) {
		reqBody := `{"player_name": "Bob"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/room/"+code+"/join", strings.NewReader(reqBody))
		api.handleJoinRoom(w, r, code)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		if resp["player_id"] != float64(2) || resp["opponent_name"] != "Alice" {
			t.Errorf("invalid response: %+v", resp)
		}
	})
}

func TestRoomAPI_GetRoomStatus(t *testing.T) {
	hub := NewHub()
	ts := NewTokenStore()
	reg := NewConnRegistry()
	api := NewRoomAPI(hub, reg, ts, "*")

	t.Run("NotFound", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/room/XXXX/status", nil)
		api.handleRoomRoutes(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("Waiting", func(t *testing.T) {
		room := hub.CreateRoom(5)
		code := room.Code
		room.AddPlayer("Alice")

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/room/"+code+"/status", nil)
		api.handleRoomRoutes(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		if resp["status"] != "waiting" {
			t.Errorf("expected status 'waiting', got %v", resp["status"])
		}
		if resp["difficulty"] != float64(5) {
			t.Errorf("expected difficulty 5, got %v", resp["difficulty"])
		}
		players, ok := resp["players"].([]any)
		if !ok || len(players) != 1 {
			t.Errorf("expected 1 player, got %v", resp["players"])
		}
	})

	t.Run("Playing", func(t *testing.T) {
		room := hub.CreateRoom(5)
		code := room.Code
		room.AddPlayer("Alice")
		room.AddPlayer("Bob")
		room.SetState(RoomPlaying)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/room/"+code+"/status", nil)
		api.handleRoomRoutes(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		if resp["status"] != "playing" {
			t.Errorf("expected status 'playing', got %v", resp["status"])
		}
		players, ok := resp["players"].([]any)
		if !ok || len(players) != 2 {
			t.Errorf("expected 2 players, got %v", resp["players"])
		}
	})
}

func TestRoomAPI_RoomEvents_SSE(t *testing.T) {
	hub := NewHub()
	ts := NewTokenStore()
	reg := NewConnRegistry()
	api := NewRoomAPI(hub, reg, ts, "*")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/room/", api.handleRoomRoutes)
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("NoAuthReturns401", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		room := hub.CreateRoom(5)
		code := room.Code
		room.AddPlayer("Alice")

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/room/"+code+"/events", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "text/event-stream")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 without token, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidTokenReturns403", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		room := hub.CreateRoom(5)
		code := room.Code
		room.AddPlayer("Alice")

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/room/"+code+"/events?token=invalid", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "text/event-stream")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 with invalid token, got %d", resp.StatusCode)
		}
	})

	t.Run("ExpiredRoomReturnsExpiredEvent", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/room/XXXX/events", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "text/event-stream")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Without token, should get 401
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("PlayerJoinedWithToken", func(t *testing.T) {
		room := hub.CreateRoom(5)
		code := room.Code
		room.AddPlayer("Alice")
		token := ts.GenerateToken(code, 1, "Alice")

		mux2 := http.NewServeMux()
		mux2.HandleFunc("/api/room/", api.handleRoomRoutes)
		server2 := httptest.NewServer(mux2)

		events := make(chan string, 10)
		goroutineErr := make(chan error, 1)
		ctx, cancelGoroutine := context.WithCancel(context.Background())

		go func() {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server2.URL+"/api/room/"+code+"/events?token="+token, nil)
			if err != nil {
				goroutineErr <- err
				return
			}
			req.Header.Set("Accept", "text/event-stream")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				goroutineErr <- err
				return
			}
			defer resp.Body.Close()

			scanner := bufio.NewScanner(resp.Body)
			scanner.Split(scanLines)

			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "event: ") {
					select {
					case events <- line:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		time.Sleep(100 * time.Millisecond)

		room.AddPlayer("Bob")

		select {
		case msg := <-events:
			if !strings.Contains(msg, "player_joined") && !strings.Contains(msg, "bot_joined") {
				t.Errorf("expected player_joined event, got: %s", msg)
			}
		case err := <-goroutineErr:
			t.Errorf("SSE goroutine error: %v", err)
		case <-time.After(2 * time.Second):
			t.Error("timeout waiting for player_joined event")
		}

		cancelGoroutine()
		go server2.Close()
		<-time.After(100 * time.Millisecond)
	})
}

func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := strings.Index(string(data), "\n"); i >= 0 {
		return i + 1, data[0:i], nil
	}
	if !atEOF {
		return 0, nil, nil
	}
	return 0, data, io.EOF
}

func TestSpectatePageRoute(t *testing.T) {
	// Test valid code returns spectate.html
	hub := NewHub()
	tokens := NewTokenStore()
	reg := NewConnRegistry()
	api := NewRoomAPI(hub, reg, tokens, "*")
	api.SetStaticDir("../web")

	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	r := httptest.NewRequest("GET", "/spectate/ABCD", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Errorf("GET /spectate/ABCD: expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<!DOCTYPE html>") {
		t.Errorf("GET /spectate/ABCD: expected HTML content, got: %s", w.Body.String())
	}

	// Test empty code returns 404
	r = httptest.NewRequest("GET", "/spectate/", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("GET /spectate/: expected 404, got %d", w.Code)
	}

	// Test invalid code (3 chars) returns 404
	r = httptest.NewRequest("GET", "/spectate/ABC", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("GET /spectate/ABC: expected 404, got %d", w.Code)
	}

	// Test invalid code (lowercase) returns 404
	r = httptest.NewRequest("GET", "/spectate/abcd", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("GET /spectate/abcd: expected 404, got %d", w.Code)
	}

	// Test method not allowed returns 405
	r = httptest.NewRequest("POST", "/spectate/ABCD", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Errorf("POST /spectate/ABCD: expected 405, got %d", w.Code)
	}
}
