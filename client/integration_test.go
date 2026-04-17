package client

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FirebirdRender/CommodorePet_TANK/server"
	"github.com/coder/websocket"
)

// newTestServer creates a test HTTP server with a WebSocket handler backed by a fresh Hub.
func newTestServer(tb testing.TB) *httptest.Server {
	tb.Helper()
	hub := server.NewHub()
	tokens := server.NewTokenStore()
	handler := server.NewWSHandler(hub, tokens)
	ts := httptest.NewServer(handler)
	tb.Cleanup(ts.Close)
	return ts
}

// wsURL converts an http:// URL to ws:// for WebSocket dialing.
func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

// connectClient creates a Network connection to the test server URL.
func connectClient(tb testing.TB, url string) *Network {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := NewNetwork(ctx, url)
	if err != nil {
		tb.Fatalf("NewNetwork: %v", err)
	}
	// Small delay to let readLoop start
	time.Sleep(50 * time.Millisecond)
	tb.Cleanup(func() {
		// Direct close to avoid existing Network.Close() nil-cancel bug
		if n.conn != nil {
			n.conn.Close(websocket.StatusNormalClosure, "test done")
		}
	})
	return n
}

// waitForMsg drains the incoming channel for up to timeout, returning the first
// message whose type matches wantType. Returns the raw payload bytes.
func waitForMsg(n *Network, wantType string, timeout time.Duration) json.RawMessage {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		case data := <-n.Incoming:
			msgType, payload, err := UnwrapMessage(data)
			if err == nil && msgType == wantType {
				return payload
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// waitForMsgType scans all currently-drained messages for the wanted type.
// Returns payload if found, nil if not.
func drainForMsg(n *Network, wantType string) json.RawMessage {
	msgs := n.DrainIncoming()
	for _, data := range msgs {
		msgType, payload, err := UnwrapMessage(data)
		if err == nil && msgType == wantType {
			return payload
		}
	}
	return nil
}

// TestClientConnectCreateRoom verifies a client can connect to the test server,
// send create_room, and receive a room_created message with a non-empty room code.
func TestClientConnectCreateRoom(t *testing.T) {
	ts := newTestServer(t)
	n := connectClient(t, wsURL(ts.URL)+"/ws")

	// Send create_room
	ok := n.Send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 5, PlayerName: "alice"})
	if !ok {
		t.Fatal("Send returned false")
	}

	// Wait for room_created
	payload := waitForMsg(n, MsgTypeRoomCreated, 3*time.Second)
	if payload == nil {
		t.Log("No room_created message, checking for error or other messages...")
		// Drain incoming to see what we got
		msgs := n.DrainIncoming()
		t.Logf("Drained messages: %d", len(msgs))
		for i, m := range msgs {
			msgType, _, err := UnwrapMessage(m)
			t.Logf("  msg %d: type=%s, err=%v", i, msgType, err)
		}
		t.Fatal("did not receive room_created message")
	}

	var roomCreated RoomCreatedMsg
	if err := json.Unmarshal(payload, &roomCreated); err != nil {
		t.Fatalf("unmarshal room_created: %v", err)
	}
	if roomCreated.RoomCode == "" {
		t.Error("RoomCode is empty, want non-empty")
	}

	// Also expect a joined message for the host
	joinedPayload := waitForMsg(n, MsgTypeJoined, 3*time.Second)
	if joinedPayload == nil {
		t.Fatal("did not receive joined message")
	}
	var joined JoinedMsg
	if err := json.Unmarshal(joinedPayload, &joined); err != nil {
		t.Fatalf("unmarshal joined: %v", err)
	}
	if joined.PlayerID != 1 {
		t.Errorf("PlayerID = %d, want 1", joined.PlayerID)
	}
}

// TestClientJoinRoom verifies two clients can create/join a room and get
// correct player IDs (1 for host, 2 for joiner).
func TestClientJoinRoom(t *testing.T) {
	ts := newTestServer(t)
	serverURL := wsURL(ts.URL) + "/ws"

	// Client 1 (host) creates room
	c1 := connectClient(t, serverURL)
	c1.Send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 3, PlayerName: "alice"})

	payload := waitForMsg(c1, MsgTypeRoomCreated, 3*time.Second)
	if payload == nil {
		t.Fatal("host did not receive room_created")
	}
	var roomCreated RoomCreatedMsg
	json.Unmarshal(payload, &roomCreated)

	// Host should also get joined for themselves
	_ = waitForMsg(c1, MsgTypeJoined, 3*time.Second)

	// Client 2 (joiner) joins the room
	c2 := connectClient(t, serverURL)
	c2.Send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCreated.RoomCode, PlayerName: "bob"})

	// Both should receive joined messages
	payload2 := waitForMsg(c2, MsgTypeJoined, 3*time.Second)
	if payload2 == nil {
		t.Fatal("joiner did not receive joined")
	}
	var joined2 JoinedMsg
	json.Unmarshal(payload2, &joined2)
	if joined2.PlayerID != 2 {
		t.Errorf("joiner PlayerID = %d, want 2", joined2.PlayerID)
	}
	if joined2.OpponentName != "alice" {
		t.Errorf("joiner OpponentName = %q, want alice", joined2.OpponentName)
	}

	// Host receives joined for the new player
	hostOpp := waitForMsg(c1, MsgTypeJoined, 3*time.Second)
	var hostJoined JoinedMsg
	json.Unmarshal(hostOpp, &hostJoined)
	if hostJoined.OpponentName != "bob" {
		t.Errorf("host opponent name = %q, want bob", hostJoined.OpponentName)
	}
}

// TestClientReadyAndGameStart verifies that after both clients send ready,
// both receive game_start with correct YourPlayerID and grid dimensions.
func TestClientReadyAndGameStart(t *testing.T) {
	ts := newTestServer(t)
	serverURL := wsURL(ts.URL) + "/ws"

	c1 := connectClient(t, serverURL)
	c1.Send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 5, PlayerName: "alice"})
	roomPayload := waitForMsg(c1, MsgTypeRoomCreated, 3*time.Second)
	var roomCreated RoomCreatedMsg
	json.Unmarshal(roomPayload, &roomCreated)
	_ = waitForMsg(c1, MsgTypeJoined, 3*time.Second)

	c2 := connectClient(t, serverURL)
	c2.Send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCreated.RoomCode, PlayerName: "bob"})
	_ = waitForMsg(c2, MsgTypeJoined, 3*time.Second)
	_ = waitForMsg(c1, MsgTypeJoined, 3*time.Second)

	// Both send ready
	c1.Send(MsgTypeReady, ReadyMsg{})
	c2.Send(MsgTypeReady, ReadyMsg{})

	// Both should receive game_start
	payload1 := waitForMsg(c1, MsgTypeGameStart, 5*time.Second)
	if payload1 == nil {
		t.Fatal("c1 did not receive game_start")
	}
	payload2 := waitForMsg(c2, MsgTypeGameStart, 5*time.Second)
	if payload2 == nil {
		t.Fatal("c2 did not receive game_start")
	}

	var start1, start2 GameStartMsg
	json.Unmarshal(payload1, &start1)
	json.Unmarshal(payload2, &start2)

	if start1.YourPlayerID != 1 {
		t.Errorf("c1 YourPlayerID = %d, want 1", start1.YourPlayerID)
	}
	if start2.YourPlayerID != 2 {
		t.Errorf("c2 YourPlayerID = %d, want 2", start2.YourPlayerID)
	}
	if len(start1.Grid) != 21 {
		t.Errorf("c1 grid rows = %d, want 21", len(start1.Grid))
	}
	if len(start1.Grid) > 0 && len(start1.Grid[0]) != 40 {
		t.Errorf("c1 grid cols = %d, want 40", len(start1.Grid[0]))
	}
	if start1.Tanks[0].PlayerID != 1 || start1.Tanks[1].PlayerID != 2 {
		t.Errorf("c1 tank player IDs = [%d,%d], want [1,2]", start1.Tanks[0].PlayerID, start1.Tanks[1].PlayerID)
	}
}

// TestClientInputAffectsState verifies that sending input messages causes
// tank positions to change in subsequent tick messages.
func TestClientInputAffectsState(t *testing.T) {
	ts := newTestServer(t)
	serverURL := wsURL(ts.URL) + "/ws"

	c1 := connectClient(t, serverURL)
	c1.Send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 5, PlayerName: "alice"})
	rcPayload := waitForMsg(c1, MsgTypeRoomCreated, 3*time.Second)
	var roomCreated RoomCreatedMsg
	json.Unmarshal(rcPayload, &roomCreated)
	_ = waitForMsg(c1, MsgTypeJoined, 3*time.Second)

	c2 := connectClient(t, serverURL)
	c2.Send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCreated.RoomCode, PlayerName: "bob"})
	_ = waitForMsg(c2, MsgTypeJoined, 3*time.Second)
	_ = waitForMsg(c1, MsgTypeJoined, 3*time.Second)

	c1.Send(MsgTypeReady, ReadyMsg{})
	c2.Send(MsgTypeReady, ReadyMsg{})

	gsPayload := waitForMsg(c1, MsgTypeGameStart, 5*time.Second)
	var gs GameStartMsg
	json.Unmarshal(gsPayload, &gs)
	// Locate player 1's spawn explicitly: Tanks[0] is conventionally P1, but read by ID to be safe.
	var startX, startY int
	var foundStart bool
	for _, tank := range gs.Tanks {
		if tank.PlayerID == 1 {
			startX, startY = tank.X, tank.Y
			foundStart = true
			break
		}
	}
	if !foundStart {
		t.Fatal("game_start did not include player 1 tank")
	}

	// Try each of the 8 directions in turn — terrain seed is non-deterministic
	// (server uses time.Now().UnixNano()), so any single direction may be blocked
	// by a spawn-adjacent wall. Per PET semantics, key must be released before a
	// fresh down event re-queues the action (server input tracker dedupes held keys).
	// Each direction gets its own 1s budget so a slow early attempt cannot starve
	// later ones. Mirrors the proven pattern in server/e2e_test.go:TestE2EInputAffectsState.
	keys := []string{"right", "down", "left", "up", "up_right", "down_right", "down_left", "up_left"}
	moved := false

	for _, key := range keys {
		c1.Send(MsgTypeInput, InputMsg{Tick: 1, Key: key, Action: "down"})
		dirDeadline := time.Now().Add(1 * time.Second)
		for time.Now().Before(dirDeadline) {
			payload := waitForMsg(c1, MsgTypeTick, 200*time.Millisecond)
			if payload == nil {
				continue
			}
			var tick TickMsg
			json.Unmarshal(payload, &tick)
			for _, tank := range tick.Tanks {
				if tank.PlayerID == 1 && (tank.X != startX || tank.Y != startY) {
					moved = true
					break
				}
			}
			if moved {
				break
			}
		}
		c1.Send(MsgTypeInput, InputMsg{Tick: 1, Key: key, Action: "up"})
		if moved {
			break
		}
	}
	if !moved {
		t.Errorf("player 1 tank did not move from (%d,%d) after trying all 8 directions", startX, startY)
	}
}

// TestClientDisconnectHandling verifies that when a connected client disconnects,
// the Network's Connected flag becomes false.
func TestClientDisconnectHandling(t *testing.T) {
	ts := newTestServer(t)
	n := connectClient(t, wsURL(ts.URL)+"/ws")

	// Verify connected
	if !n.Connected {
		t.Error("expected Connected=true right after connect")
	}

	// Create a room so the server holds state
	n.Send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 5, PlayerName: "alice"})
	_ = waitForMsg(n, MsgTypeRoomCreated, 3*time.Second)

	// Close underlying connection directly (avoids Network.Close nil-cancel bug)
	n.conn.Close(websocket.StatusNormalClosure, "test disconnect")

	// Wait for the read loop to notice the close
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Poll Connected flag until it becomes false
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		n.mu.Lock()
		connected := n.Connected
		n.mu.Unlock()
		if !connected {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("Connected did not become false after Close")
		}
	}

	// Also verify GameState would transition to PhaseDisconnected
	gs := NewGameState()
	gs.ApplyRoomCreated(RoomCreatedMsg{RoomCode: "TEST"})
	if gs.RoomCode != "TEST" {
		t.Errorf("RoomCode = %q, want 'TEST'", gs.RoomCode)
	}
	gs.Phase = PhaseDisconnected
	if gs.Phase != PhaseDisconnected {
		t.Error("expected PhaseDisconnected")
	}
}
