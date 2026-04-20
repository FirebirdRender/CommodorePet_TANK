package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func setupTestServer(t *testing.T) (*httptest.Server, *WSHandler) {
	t.Helper()

	hub := NewHub(0)
	tokens := NewTokenStore()
	handler := NewWSHandler(hub, tokens, nil, 0)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	return ts, handler
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func dialWS(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, rawURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})

	return conn
}

func sendMsg(t *testing.T, conn *websocket.Conn, msgType string, payload any) {
	t.Helper()

	data, err := WrapMessage(msgType, payload)
	if err != nil {
		t.Fatalf("wrap message: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write message: %v", err)
	}
}

func readMsg(t *testing.T, conn *websocket.Conn, timeout time.Duration) (string, []byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}

	return msgType, payload
}

func readUntilType(t *testing.T, conn *websocket.Conn, wanted string, timeout time.Duration) []byte {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		msgType, payload := readMsg(t, conn, remaining)
		if msgType == wanted {
			return payload
		}
	}

	t.Fatalf("did not receive message type %q within %s", wanted, timeout)
	return nil
}

func decodePayload[T any](t *testing.T, payload []byte) T {
	t.Helper()

	var out T
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return out
}

func createRoomAndGetCode(t *testing.T, conn *websocket.Conn, playerName string) string {
	t.Helper()

	sendMsg(t, conn, MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 5, PlayerName: playerName})
	_, payload := readMsg(t, conn, 5*time.Second)
	rc := decodePayload[RoomCreatedMsg](t, payload)
	_, _ = readMsg(t, conn, 5*time.Second) // joined
	return rc.RoomCode
}

func TestWSCreateRoom(t *testing.T) {
	ts, _ := setupTestServer(t)
	c1 := dialWS(t, wsURL(ts.URL))

	sendMsg(t, c1, MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 4, PlayerName: "alice"})

	msgType, payload := readMsg(t, c1, 5*time.Second)
	if msgType != MsgTypeRoomCreated {
		t.Fatalf("first msg type = %q, want %q", msgType, MsgTypeRoomCreated)
	}
	roomCreated := decodePayload[RoomCreatedMsg](t, payload)
	if roomCreated.RoomCode == "" {
		t.Fatal("expected non-empty room code")
	}

	msgType, payload = readMsg(t, c1, 5*time.Second)
	if msgType != MsgTypeJoined {
		t.Fatalf("second msg type = %q, want %q", msgType, MsgTypeJoined)
	}
	joined := decodePayload[JoinedMsg](t, payload)
	if joined.PlayerID != 1 {
		t.Fatalf("joined player_id = %d, want 1", joined.PlayerID)
	}
	if joined.RoomCode != roomCreated.RoomCode {
		t.Fatalf("joined room code = %q, want %q", joined.RoomCode, roomCreated.RoomCode)
	}
}

func TestWSJoinRoomNotifiesBothPlayers(t *testing.T) {
	ts, _ := setupTestServer(t)
	c1 := dialWS(t, wsURL(ts.URL))
	c2 := dialWS(t, wsURL(ts.URL))

	roomCode := createRoomAndGetCode(t, c1, "alice")
	sendMsg(t, c2, MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCode, PlayerName: "bob"})

	msgType, payload := readMsg(t, c2, 5*time.Second)
	if msgType != MsgTypeJoined {
		t.Fatalf("joiner msg type = %q, want %q", msgType, MsgTypeJoined)
	}
	joinerJoined := decodePayload[JoinedMsg](t, payload)
	if joinerJoined.PlayerID != 2 {
		t.Fatalf("joiner player_id = %d, want 2", joinerJoined.PlayerID)
	}
	if joinerJoined.OpponentName != "alice" {
		t.Fatalf("joiner opponent_name = %q, want alice", joinerJoined.OpponentName)
	}

	payload = readUntilType(t, c1, MsgTypeJoined, 5*time.Second)
	hostJoined := decodePayload[JoinedMsg](t, payload)
	if hostJoined.PlayerID != 1 {
		t.Fatalf("host player_id = %d, want 1", hostJoined.PlayerID)
	}
	if hostJoined.OpponentName != "bob" {
		t.Fatalf("host opponent_name = %q, want bob", hostJoined.OpponentName)
	}
}

func TestWSJoinNonexistentRoomReturnsError(t *testing.T) {
	ts, _ := setupTestServer(t)
	c := dialWS(t, wsURL(ts.URL))

	sendMsg(t, c, MsgTypeJoinRoom, JoinRoomMsg{RoomCode: "ZZZZ", PlayerName: "ghost"})
	msgType, payload := readMsg(t, c, 5*time.Second)
	if msgType != MsgTypeError {
		t.Fatalf("msg type = %q, want %q", msgType, MsgTypeError)
	}
	errMsg := decodePayload[ErrorMsg](t, payload)
	if errMsg.Code != ErrCodeRoomNotFound {
		t.Fatalf("error code = %q, want %q", errMsg.Code, ErrCodeRoomNotFound)
	}
}

func TestWSJoinFullRoomReturnsError(t *testing.T) {
	ts, _ := setupTestServer(t)
	c1 := dialWS(t, wsURL(ts.URL))
	c2 := dialWS(t, wsURL(ts.URL))
	c3 := dialWS(t, wsURL(ts.URL))

	roomCode := createRoomAndGetCode(t, c1, "alice")
	sendMsg(t, c2, MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCode, PlayerName: "bob"})
	_, _ = readMsg(t, c2, 5*time.Second)
	_ = readUntilType(t, c1, MsgTypeJoined, 5*time.Second)

	sendMsg(t, c3, MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCode, PlayerName: "charlie"})
	msgType, payload := readMsg(t, c3, 5*time.Second)
	if msgType != MsgTypeError {
		t.Fatalf("msg type = %q, want %q", msgType, MsgTypeError)
	}
	errMsg := decodePayload[ErrorMsg](t, payload)
	if errMsg.Code != ErrCodeRoomFull {
		t.Fatalf("error code = %q, want %q", errMsg.Code, ErrCodeRoomFull)
	}
}

func TestWSReadyStartsGameForBothPlayers(t *testing.T) {
	ts, _ := setupTestServer(t)
	c1 := dialWS(t, wsURL(ts.URL))
	c2 := dialWS(t, wsURL(ts.URL))

	roomCode := createRoomAndGetCode(t, c1, "alice")
	sendMsg(t, c2, MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCode, PlayerName: "bob"})
	_, _ = readMsg(t, c2, 5*time.Second)
	_ = readUntilType(t, c1, MsgTypeJoined, 5*time.Second)

	sendMsg(t, c1, MsgTypeReady, ReadyMsg{})
	sendMsg(t, c2, MsgTypeReady, ReadyMsg{})

	payload := readUntilType(t, c1, MsgTypeGameStart, 5*time.Second)
	start1 := decodePayload[GameStartMsg](t, payload)
	if start1.YourPlayerID != 1 {
		t.Fatalf("player 1 YourPlayerID = %d, want 1", start1.YourPlayerID)
	}
	if len(start1.Grid) == 0 || len(start1.Grid[0]) == 0 {
		t.Fatal("expected non-empty grid for player 1")
	}

	payload = readUntilType(t, c2, MsgTypeGameStart, 5*time.Second)
	start2 := decodePayload[GameStartMsg](t, payload)
	if start2.YourPlayerID != 2 {
		t.Fatalf("player 2 YourPlayerID = %d, want 2", start2.YourPlayerID)
	}
	if len(start2.Grid) == 0 || len(start2.Grid[0]) == 0 {
		t.Fatal("expected non-empty grid for player 2")
	}
}

func TestWSInputDuringPlayReceivesTick(t *testing.T) {
	ts, _ := setupTestServer(t)
	c1 := dialWS(t, wsURL(ts.URL))
	c2 := dialWS(t, wsURL(ts.URL))

	roomCode := createRoomAndGetCode(t, c1, "alice")
	sendMsg(t, c2, MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCode, PlayerName: "bob"})
	_, _ = readMsg(t, c2, 5*time.Second)
	_ = readUntilType(t, c1, MsgTypeJoined, 5*time.Second)

	sendMsg(t, c1, MsgTypeReady, ReadyMsg{})
	sendMsg(t, c2, MsgTypeReady, ReadyMsg{})
	_ = readUntilType(t, c1, MsgTypeGameStart, 5*time.Second)
	_ = readUntilType(t, c2, MsgTypeGameStart, 5*time.Second)

	sendMsg(t, c1, MsgTypeInput, InputMsg{Tick: 1, Key: "right", Action: "down"})
	payload := readUntilType(t, c1, MsgTypeTick, 5*time.Second)
	tick := decodePayload[TickMsg](t, payload)
	if tick.Tick == 0 {
		t.Fatalf("tick id = %d, want > 0", tick.Tick)
	}
}

func TestWSDisconnectClosesRoomState(t *testing.T) {
	ts, handler := setupTestServer(t)
	c1 := dialWS(t, wsURL(ts.URL))
	c2 := dialWS(t, wsURL(ts.URL))

	roomCode := createRoomAndGetCode(t, c1, "alice")
	sendMsg(t, c2, MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCode, PlayerName: "bob"})
	_, _ = readMsg(t, c2, 5*time.Second)
	_ = readUntilType(t, c1, MsgTypeJoined, 5*time.Second)

	sendMsg(t, c1, MsgTypeReady, ReadyMsg{})
	sendMsg(t, c2, MsgTypeReady, ReadyMsg{})
	_ = readUntilType(t, c1, MsgTypeGameStart, 5*time.Second)
	_ = readUntilType(t, c2, MsgTypeGameStart, 5*time.Second)

	_ = c2.Close(websocket.StatusNormalClosure, "bye")

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		room := handler.Hub.GetRoom(roomCode)
		if room == nil {
			return
		}
		state := room.GetState()
		if state == RoomClosed || state == RoomWaitingReconnect {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatal("expected room to be removed, closed, or in reconnect state after disconnect")
}

func TestWSInvalidMessageReturnsError(t *testing.T) {
	ts, _ := setupTestServer(t)
	c := dialWS(t, wsURL(ts.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, []byte("{")); err != nil {
		t.Fatalf("write invalid payload: %v", err)
	}

	msgType, payload := readMsg(t, c, 5*time.Second)
	if msgType != MsgTypeError {
		t.Fatalf("msg type = %q, want %q", msgType, MsgTypeError)
	}
	errMsg := decodePayload[ErrorMsg](t, payload)
	if errMsg.Code != ErrCodeInvalidInput {
		t.Fatalf("error code = %q, want %q", errMsg.Code, ErrCodeInvalidInput)
	}
}
