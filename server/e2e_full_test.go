package server

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFullMatchFlow(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	c1 := newE2EClient(t, srv.URL)
	c2 := newE2EClient(t, srv.URL)
	defer c1.closeNow()
	defer c2.closeNow()

	c1.send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 3, PlayerName: "Alice"})
	msg := c1.recvExpect(MsgTypeRoomCreated)
	var roomCreated RoomCreatedMsg
	if err := json.Unmarshal(msg, &roomCreated); err != nil {
		t.Fatalf("unmarshal room_created: %v", err)
	}
	if roomCreated.RoomCode == "" {
		t.Fatal("expected non-empty room code")
	}

	c2.send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCreated.RoomCode, PlayerName: "Bob"})
	c2.recvExpect(MsgTypeJoined)
	c1.recvExpect(MsgTypeJoined)
	c1.recvExpect(MsgTypeJoined)

	c1.send(MsgTypeReady, ReadyMsg{})
	c2.send(MsgTypeReady, ReadyMsg{})

	c1.recvExpect(MsgTypeGameStart)
	c2.recvExpect(MsgTypeGameStart)

	_, _, err := c1.recvWithTimeout(2 * time.Second)
	if err != nil {
		t.Errorf("expected tick within 2s, got error: %v", err)
	}

	c1.send(MsgTypeInput, InputMsg{Tick: 1, Key: "right", Action: "down"})

	_, _, err = c1.recvWithTimeout(2 * time.Second)
	if err != nil {
		t.Errorf("expected tick after input, got error: %v", err)
	}
}

func TestPlayAgainFlow(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	c1, c2, _, _, _ := setupStartedMatch(t, srv.URL, 5, "Alice", "Bob")
	defer c1.closeNow()
	defer c2.closeNow()

	_, _, err := c1.recvWithTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("expected tick within 3s: %v", err)
	}

	c1.send(MsgTypePlayAgain, PlayAgainMsg{})
	c2.send(MsgTypePlayAgain, PlayAgainMsg{})

	_ = c1.recvUntil(MsgTypeRematch, 20)
	_ = c2.recvUntil(MsgTypeRematch, 20)

	_ = c1.recvUntil(MsgTypeGameStart, 20)
	_ = c2.recvUntil(MsgTypeGameStart, 20)

	_, _, err = c1.recvWithTimeout(2 * time.Second)
	if err != nil {
		t.Errorf("expected tick in new match, got error: %v", err)
	}
}

func TestDisconnectNotification(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	c1, c2, _, _, _ := setupStartedMatch(t, srv.URL, 5, "Alice", "Bob")
	defer c1.closeNow()

	c2.closeNow()

	msgType, _, err := c1.recvWithTimeout(5 * time.Second)
	if err != nil {
		t.Fatalf("expected message within 5s: %v", err)
	}
	if msgType != MsgTypeOpponentLeft {
		t.Errorf("expected opponent_left, got %s", msgType)
	}
}

func TestDifficultySelection(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	c1 := newE2EClient(t, srv.URL)
	c2 := newE2EClient(t, srv.URL)
	defer c1.closeNow()
	defer c2.closeNow()

	c1.send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 7, PlayerName: "Alice"})
	msg := c1.recvExpect(MsgTypeRoomCreated)
	var roomCreated RoomCreatedMsg
	if err := json.Unmarshal(msg, &roomCreated); err != nil {
		t.Fatalf("unmarshal room_created: %v", err)
	}

	c2.send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCreated.RoomCode, PlayerName: "Bob"})
	c2.recvExpect(MsgTypeJoined)
	c1.recvExpect(MsgTypeJoined)
	c1.recvExpect(MsgTypeJoined)

	c1.send(MsgTypeReady, ReadyMsg{})
	c2.send(MsgTypeReady, ReadyMsg{})

	gs := c1.recvExpect(MsgTypeGameStart)
	var gameStart GameStartMsg
	if err := json.Unmarshal(gs, &gameStart); err != nil {
		t.Fatalf("unmarshal game_start: %v", err)
	}
	if gameStart.Difficulty != 7 {
		t.Errorf("expected difficulty 7, got %d", gameStart.Difficulty)
	}
}

func TestRoomExpiry(t *testing.T) {
	hub := NewHub()

	room := hub.CreateRoom(5)
	roomCode := room.Code
	if roomCode == "" {
		t.Fatal("failed to create room")
	}
	room.CreatedAt = time.Now().Add(-31 * time.Minute)

	hub.CleanupStaleRooms(30 * time.Minute)

	if hub.GetRoom(roomCode) != nil {
		t.Error("expected stale room to be cleaned up")
	}
}
