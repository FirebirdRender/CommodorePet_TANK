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
	if msgType != MsgTypeOpponentLeft && msgType != MsgTypePlayerDisconnected {
		t.Errorf("expected opponent_left or player_disconnected, got %s", msgType)
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

func TestAutoStartOnJoin(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	c1 := newE2EClient(t, srv.URL)
	c2 := newE2EClient(t, srv.URL)
	defer c1.closeNow()
	defer c2.closeNow()

	c1.send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 5, PlayerName: "Alice"})
	roomCreated := decodeRaw[RoomCreatedMsg](t, c1.recvExpect(MsgTypeRoomCreated))
	if roomCreated.RoomCode == "" {
		t.Fatal("expected non-empty room code")
	}

	_ = decodeRaw[JoinedMsg](t, c1.recvExpect(MsgTypeJoined))

	c2.send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCreated.RoomCode, PlayerName: "Bob"})
	joined2 := decodeRaw[JoinedMsg](t, c2.recvExpect(MsgTypeJoined))
	if joined2.PlayerID != 2 {
		t.Fatalf("joiner player_id = %d, want 2", joined2.PlayerID)
	}

	hostJoined := decodeRaw[JoinedMsg](t, c1.recvUntil(MsgTypeJoined, 10))
	if hostJoined.OpponentName != "Bob" {
		t.Fatalf("host opponent = %q, want 'Bob'", hostJoined.OpponentName)
	}

	gs1 := decodeRaw[GameStartMsg](t, c1.recvUntil(MsgTypeGameStart, 30))
	gs2 := decodeRaw[GameStartMsg](t, c2.recvUntil(MsgTypeGameStart, 30))

	requireGridShape(t, gs1.Grid, 21, 40)
	requireGridShape(t, gs2.Grid, 21, 40)

	if gs1.YourPlayerID != 1 {
		t.Errorf("gs1.YourPlayerID = %d, want 1", gs1.YourPlayerID)
	}
	if gs2.YourPlayerID != 2 {
		t.Errorf("gs2.YourPlayerID = %d, want 2", gs2.YourPlayerID)
	}

	msgType, _, err := c1.recvWithTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("expected tick within 3s, got error: %v", err)
	}
	if msgType != MsgTypeTick && msgType != MsgTypeTickDelta {
		t.Errorf("expected tick or tick_delta, got %s", msgType)
	}
}

func TestInputAffectsTankPosition(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	c1, c2, _, start1, _ := setupStartedMatch(t, srv.URL, 5, "Alice", "Bob")
	defer c1.closeNow()
	defer c2.closeNow()

	p1Initial := findTankByPlayerID(t, start1.Tanks, 1)

	c1.send(MsgTypeInput, InputMsg{Tick: 1, Key: "right", Action: "down"})

	maxReads := 30
	var foundMoved bool
	for i := 0; i < maxReads; i++ {
		msgType, payload, err := c1.recvWithTimeout(2 * time.Second)
		if err != nil {
			t.Fatalf("recv failed at read %d: %v", i, err)
		}

		if msgType == MsgTypeTick {
			var tick TickMsg
			if err := json.Unmarshal(payload, &tick); err != nil {
				continue
			}
			p1 := findTankByPlayerID(t, tick.Tanks, 1)
			if p1.X != p1Initial.X || p1.Y != p1Initial.Y {
				foundMoved = true
				break
			}
		} else if msgType == MsgTypeTickDelta {
			var delta TickDeltaMsg
			if err := json.Unmarshal(payload, &delta); err != nil {
				continue
			}
			p1 := findTankByPlayerID(t, delta.Tanks, 1)
			if p1.X != p1Initial.X || p1.Y != p1Initial.Y {
				foundMoved = true
				break
			}
		} else if msgType == MsgTypeRoundOver || msgType == MsgTypeGameOver {
			break
		}
	}

	if !foundMoved {
		t.Error("P1 tank never moved after right input (may need more ticks)")
	}
	_ = c2
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

func TestSpectatorJoinAndReceiveTicks(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	c1, c2, roomCode, _, _ := setupStartedMatch(t, srv.URL, 5, "Alice", "Bob")
	defer c1.closeNow()
	defer c2.closeNow()

	_ = decodeRaw[TickMsg](t, c1.recvUntil(MsgTypeTick, 120))

	spec := newE2EClient(t, srv.URL)
	defer spec.closeNow()

	spec.send(MsgTypeSpectate, SpectateMsg{RoomCode: roomCode})

	joinedMsg := decodeRaw[SpectatorJoinedMsg](t, spec.recvExpect(MsgTypeSpectatorJoined))
	if joinedMsg.SpectatorCount < 1 {
		t.Fatalf("spectator count = %d, want >= 1", joinedMsg.SpectatorCount)
	}

	// CRITICAL: spectator must receive game_start with YourPlayerID=0,
	// otherwise the WASM client treats itself as Player 1 and predicts barrel/movement
	// for the actual player (the bug fixed in v0.9.12).
	gsMsg := decodeRaw[GameStartMsg](t, spec.recvUntil(MsgTypeGameStart, 30))
	if gsMsg.YourPlayerID != 0 {
		t.Fatalf("spectator GameStart YourPlayerID = %d, want 0 (spectator marker)", gsMsg.YourPlayerID)
	}

	msgType, _, err := spec.recvWithTimeout(5 * time.Second)
	if err != nil {
		t.Fatalf("expected tick on spectator connection: %v", err)
	}
	if msgType != MsgTypeTick && msgType != MsgTypeTickDelta {
		t.Fatalf("expected tick/tick_delta on spectator after game_start, got %s", msgType)
	}

	spec.send(MsgTypeInput, InputMsg{Tick: 1, Key: "right", Action: "down"})
	errMsg := decodeRaw[ErrorMsg](t, spec.recvUntil(MsgTypeError, 30))
	if errMsg.Code != ErrCodeReadOnly {
		t.Fatalf("expected read_only error for spectator input, got code=%q message=%q", errMsg.Code, errMsg.Message)
	}
}

func TestReconnectFlow(t *testing.T) {
	srv, handler := setupTestServer(t)
	defer srv.Close()

	c1, c2, roomCode, _, _ := setupStartedMatch(t, srv.URL, 5, "Alice", "Bob")
	defer c1.closeNow()

	_ = decodeRaw[TickMsg](t, c1.recvUntil(MsgTypeTick, 120))

	c2.closeNow()

	msgType, _, err := c1.recvWithTimeout(5 * time.Second)
	if err != nil {
		t.Fatalf("expected message after P2 disconnect: %v", err)
	}
	if msgType != MsgTypePlayerDisconnected && msgType != MsgTypeOpponentLeft {
		t.Fatalf("expected player_disconnected or opponent_left, got %s", msgType)
	}

	room := handler.Hub.GetRoom(roomCode)
	if room == nil {
		t.Fatal("room should still exist during reconnect window")
	}
	if room.GetState() != RoomWaitingReconnect {
		t.Fatalf("room state = %d, want RoomWaitingReconnect(%d)", room.GetState(), RoomWaitingReconnect)
	}

	c1.closeNow()

	waitForCondition(t, 5*time.Second, func() bool {
		return handler.Hub.RoomCount() == 0 || handler.Hub.GetRoom(roomCode) == nil || handler.Hub.GetRoom(roomCode).GetState() == RoomClosed
	}, "room cleaned up after both players disconnect during reconnect")
}
