package server

import (
	"encoding/json"
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
	api := NewRoomAPI(hub, reg, ts)

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
	api := NewRoomAPI(hub, reg, ts)

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
