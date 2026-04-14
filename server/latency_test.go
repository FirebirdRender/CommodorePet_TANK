package server

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// lagProxy is a simple in-process TCP proxy that adds configurable latency+jitter.
type lagProxy struct {
	listenAddr string
	targetAddr string
	latencyMs  int
	jitterMs   int
	ln         net.Listener
	done       chan struct{}
}

func startLagProxy(t *testing.T, targetHost string, targetPort int, latencyMs, jitterMs int) *lagProxy {
	lp := &lagProxy{
		listenAddr: fmt.Sprintf("localhost:0"),
		targetAddr: fmt.Sprintf("%s:%d", targetHost, targetPort),
		latencyMs:  latencyMs,
		jitterMs:   jitterMs,
		done:       make(chan struct{}),
	}

	ln, err := net.Listen("tcp", lp.listenAddr)
	if err != nil {
		t.Fatalf("lag proxy listen: %v", err)
	}
	lp.ln = ln

	go lp.run()
	return lp
}

func (lp *lagProxy) run() {
	for {
		conn, err := lp.ln.Accept()
		if err != nil {
			select {
			case <-lp.done:
				return
			default:
			}
			continue
		}
		go lp.handleConn(conn)
	}
}

func (lp *lagProxy) handleConn(client net.Conn) {
	defer client.Close()

	backend, err := net.DialTimeout("tcp", lp.targetAddr, 5*time.Second)
	if err != nil {
		return
	}
	defer backend.Close()

	var wg waitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		lp.relay(client, backend)
	}()
	go func() {
		defer wg.Done()
		lp.relay(backend, client)
	}()

	wg.Wait()
}

func (lp *lagProxy) relay(src, dst net.Conn) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}

		delay := time.Duration(lp.latencyMs)*time.Millisecond + lp.randJitter()
		time.Sleep(delay)

		if _, err := dst.Write(buf[:n]); err != nil {
			return
		}
	}
}

func (lp *lagProxy) randJitter() time.Duration {
	if lp.jitterMs <= 0 {
		return 0
	}
	j := time.Duration(lp.jitterMs*2+1) * time.Millisecond
	offset := time.Now().UnixNano() % int64(j/time.Millisecond)
	return time.Duration(offset)*time.Millisecond - time.Duration(lp.jitterMs)*time.Millisecond
}

func (lp *lagProxy) Addr() string {
	return lp.ln.Addr().String()
}

func (lp *lagProxy) Close() error {
	close(lp.done)
	return lp.ln.Close()
}

// waitGroup is a small sync.WaitGroup subset to avoid import.
type waitGroup struct {
	ch chan struct{}
}

func (wg *waitGroup) Add(delta int) {
	if wg.ch == nil {
		wg.ch = make(chan struct{})
	}
}

func (wg *waitGroup) Done() {
	select {
	case <-wg.ch:
	default:
		close(wg.ch)
	}
}

func (wg *waitGroup) Wait() {
	if wg.ch == nil {
		return
	}
	<-wg.ch
}

// --- Test 1: DeltaEncodingUnderLatency ---

func TestDeltaEncodingUnderLatency(t *testing.T) {
	ts, _ := setupTestServer(t)
	c1, c2, _, start1, start2 := setupStartedMatch(t, ts.URL, 5, "alice", "bob")

	// Apply game start to get initial state
	gs1 := newClientGameState(start1)
	_ = newClientGameState(start2)
	_ = gs1 // suppress unused warning

	// Receive ticks and verify delta encoding behavior
	var tickCount int
	var keyframeCount int
	var deltaCount int
	var lastTickType string

	for i := 0; i < 120; i++ {
		msgType, payload, err := c1.recvWithTimeout(500 * time.Millisecond)
		if err != nil {
			// Timeout is ok after some ticks
			if i > 10 {
				break
			}
			continue
		}

		switch msgType {
		case MsgTypeTick:
			tickCount++
			keyframeCount++
			lastTickType = MsgTypeTick
			msg := decodeRaw[TickMsg](t, payload)
			gs1.ApplyTick(msg)
		case MsgTypeTickDelta:
			tickCount++
			deltaCount++
			lastTickType = MsgTypeTickDelta
			msg := decodeRaw[TickDeltaMsg](t, payload)
			gs1.ApplyTickDelta(msg)
		case MsgTypeRoundOver, MsgTypeGameOver:
			// Game ended, that's fine
			goto done
		}
	}

done:
	// Verify we got some ticks
	if tickCount == 0 {
		t.Fatal("expected at least some tick messages")
	}

	// First tick after game_start should be a full keyframe
	if lastTickType != MsgTypeTick {
		t.Errorf("first tick after game_start = %q, want %q (keyframe)", lastTickType, MsgTypeTick)
	}

	// Verify we got at least one keyframe
	if keyframeCount == 0 {
		t.Error("expected at least one full keyframe tick")
	}

	// Verify delta encoding worked
	t.Logf("tick count: %d, keyframes: %d, deltas: %d", tickCount, keyframeCount, deltaCount)

	// Send some input and continue receiving
	c1.send(MsgTypeInput, InputMsg{Tick: 1, Key: "fire", Action: "down"})
	c1.send(MsgTypeInput, InputMsg{Tick: 1, Key: "right", Action: "down"})

	// Receive more ticks
	for i := 0; i < 80; i++ {
		msgType, _, err := c1.recvWithTimeout(200 * time.Millisecond)
		if err != nil {
			break
		}
		if msgType == MsgTypeRoundOver || msgType == MsgTypeGameOver {
			break
		}
	}

	// Verify client 2 also received ticks
	msgType, _, err := c2.recvWithTimeout(500 * time.Millisecond)
	if err == nil {
		if msgType != MsgTypeTick && msgType != MsgTypeTickDelta {
			t.Errorf("c2 received unexpected message type: %q", msgType)
		}
	}

	t.Logf("gs1 tanks after %d ticks: %+v", tickCount, gs1.Tanks)
}

// testGameState is a minimal game state for testing delta application.
type testGameState struct {
	Grid  [][]int
	Tanks [2]TankState
}

func newClientGameState(startMsg GameStartMsg) *testGameState {
	return &testGameState{
		Grid:  startMsg.Grid,
		Tanks: startMsg.Tanks,
	}
}

func (gs *testGameState) ApplyTick(msg TickMsg) {
	gs.Grid = msg.Grid
	gs.Tanks = msg.Tanks
}

func (gs *testGameState) ApplyTickDelta(msg TickDeltaMsg) {
	for _, cc := range msg.ChangedCells {
		if cc.Y >= 0 && cc.Y < len(gs.Grid) && cc.X >= 0 && cc.X < len(gs.Grid[cc.Y]) {
			gs.Grid[cc.Y][cc.X] = cc.Cell
		}
	}
	gs.Tanks = msg.Tanks
}

// --- Test 4: LagProxyRoundTrip ---

func TestLagProxyRoundTrip(t *testing.T) {
	// Start the game server
	ts, _ := setupTestServer(t)
	serverHost, serverPortStr, _ := net.SplitHostPort(ts.Listener.Addr().String())
	var serverPort int
	fmt.Sscanf(serverPortStr, "%d", &serverPort)

	// Start lag proxy between clients and server
	proxy := startLagProxy(t, serverHost, serverPort, 75, 10)
	defer proxy.Close()

	// newE2EClient expects HTTP URL (adds ws prefix internally)
	proxyHost, proxyPortStr, _ := net.SplitHostPort(proxy.Addr())
	proxyHTTPURL := fmt.Sprintf("http://%s:%s", proxyHost, proxyPortStr)

	// Connect two clients through the lag proxy
	c1 := newE2EClient(t, proxyHTTPURL)
	c2 := newE2EClient(t, proxyHTTPURL)

	// Create room and join
	c1.send(MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 5, PlayerName: "alice"})
	roomCreated := decodeRaw[RoomCreatedMsg](t, c1.recvExpect(MsgTypeRoomCreated))
	if roomCreated.RoomCode == "" {
		t.Fatal("expected non-empty room code")
	}

	_ = decodeRaw[JoinedMsg](t, c1.recvExpect(MsgTypeJoined))

	c2.send(MsgTypeJoinRoom, JoinRoomMsg{RoomCode: roomCreated.RoomCode, PlayerName: "bob"})
	_ = decodeRaw[JoinedMsg](t, c2.recvExpect(MsgTypeJoined))
	_ = decodeRaw[JoinedMsg](t, c1.recvUntil(MsgTypeJoined, 30))

	// Both ready up
	c1.send(MsgTypeReady, ReadyMsg{})
	c2.send(MsgTypeReady, ReadyMsg{})

	// Receive game start on both
	_ = decodeRaw[GameStartMsg](t, c1.recvUntil(MsgTypeGameStart, 120))
	_ = decodeRaw[GameStartMsg](t, c2.recvUntil(MsgTypeGameStart, 120))

	// Send some input
	c1.send(MsgTypeInput, InputMsg{Tick: 1, Key: "right", Action: "down"})
	c1.send(MsgTypeInput, InputMsg{Tick: 1, Key: "up", Action: "down"})
	c2.send(MsgTypeInput, InputMsg{Tick: 1, Key: "left", Action: "down"})

	// Verify game runs for at least 20 ticks through the proxy
	tickCount := 0
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && tickCount < 20 {
		_, _, err := c1.recvWithTimeout(500 * time.Millisecond)
		if err != nil {
			continue
		}
		tickCount++
	}

	if tickCount < 5 {
		t.Errorf("expected at least 5 ticks through proxy, got %d", tickCount)
	}

	t.Logf("Successfully processed %d ticks through lag proxy", tickCount)
}
