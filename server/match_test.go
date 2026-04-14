package server

import (
	"sync"
	"testing"
	"time"

	"github.com/FirebirdRender/CommodorePet_TANK/engine"
)

type testEvents struct {
	ticks      []*TickMsg
	roundOvers []*RoundOverMsg
	gameOvers  []*GameOverMsg
	mu         sync.Mutex
}

func (e *testEvents) OnTick(_ uint64, state *TickMsg) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ticks = append(e.ticks, state)
}

func (e *testEvents) OnRoundOver(msg *RoundOverMsg) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.roundOvers = append(e.roundOvers, msg)
}

func (e *testEvents) OnGameOver(msg *GameOverMsg) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.gameOvers = append(e.gameOvers, msg)
}

func (e *testEvents) tickCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.ticks)
}

func TestNewMatchController(t *testing.T) {
	mc := NewMatchController(5, 123, nil)

	if got := mc.GetState(); got != MatchIdle {
		t.Fatalf("GetState() = %v, want %v", got, MatchIdle)
	}
	if got := mc.CurrentTick(); got != 0 {
		t.Fatalf("CurrentTick() = %d, want 0", got)
	}
}

func TestMatchControllerGameStartState(t *testing.T) {
	mc := NewMatchController(5, 123, nil)

	msg := mc.GameStartState(2)
	if msg == nil {
		t.Fatal("GameStartState() returned nil")
	}
	if msg.YourPlayerID != 2 {
		t.Fatalf("YourPlayerID = %d, want 2", msg.YourPlayerID)
	}
	if msg.Difficulty != 5 {
		t.Fatalf("Difficulty = %d, want 5", msg.Difficulty)
	}
	if len(msg.Grid) != 21 {
		t.Fatalf("grid height = %d, want 21", len(msg.Grid))
	}
	if len(msg.Grid[0]) != 40 {
		t.Fatalf("grid width = %d, want 40", len(msg.Grid[0]))
	}
	if msg.Tanks[0].PlayerID != 1 || msg.Tanks[1].PlayerID != 2 {
		t.Fatalf("unexpected tank ids: %+v", msg.Tanks)
	}
	if msg.Tanks[0].X != 2 || msg.Tanks[0].Y != 10 {
		t.Fatalf("tank1 position = (%d,%d), want (2,10)", msg.Tanks[0].X, msg.Tanks[0].Y)
	}
	if msg.Tanks[1].X != 37 || msg.Tanks[1].Y != 10 {
		t.Fatalf("tank2 position = (%d,%d), want (37,10)", msg.Tanks[1].X, msg.Tanks[1].Y)
	}
}

func TestMatchControllerStartStop(t *testing.T) {
	mc := NewMatchController(5, 123, nil)
	mc.Start()

	if got := mc.GetState(); got != MatchRunning {
		t.Fatalf("state after Start = %v, want %v", got, MatchRunning)
	}

	mc.Stop()
	if got := mc.GetState(); got != MatchStopped {
		t.Fatalf("state after Stop = %v, want %v", got, MatchStopped)
	}
}

func TestMatchControllerStopIdempotent(t *testing.T) {
	mc := NewMatchController(5, 123, nil)
	mc.Start()
	mc.Stop()
	mc.Stop()

	if got := mc.GetState(); got != MatchStopped {
		t.Fatalf("state after second Stop = %v, want %v", got, MatchStopped)
	}
}

func TestMatchControllerTickAdvances(t *testing.T) {
	events := &testEvents{}
	mc := NewMatchController(5, 123, events)
	mc.Start()

	time.Sleep(50 * time.Millisecond)
	mc.Stop()

	if got := mc.CurrentTick(); got == 0 {
		t.Fatal("CurrentTick() = 0, want > 0")
	}
	if got := events.tickCount(); got == 0 {
		t.Fatal("expected tick callbacks, got none")
	}
}

func TestMatchControllerInputApplies(t *testing.T) {
	events := &testEvents{}
	mc := NewMatchController(5, 123, events)
	mc.state = MatchRunning

	startX := mc.gc.Tanks[0].X
	mc.GetInput(1).KeyDown(engine.ActionRight)

	for range 4 {
		mc.stepTick()
	}

	if mc.gc.Tanks[0].X <= startX {
		t.Fatalf("tank1 X = %d, want > %d", mc.gc.Tanks[0].X, startX)
	}
	if events.tickCount() == 0 {
		t.Fatal("expected tick callbacks while stepping")
	}
}

func TestMatchControllerRoundOverDetection(t *testing.T) {
	events := &testEvents{}
	mc := NewMatchController(5, 123, events)
	mc.state = MatchRunning

	mc.gc.Winner = 1
	mc.gc.Wins = [2]int{1, 0}
	mc.gc.BattlesPlayed = 1
	mc.gc.Tanks[1].Lives = 2

	mc.stepTick()

	if got := mc.GetState(); got != MatchRoundOver {
		t.Fatalf("state = %v, want %v", got, MatchRoundOver)
	}
	if mc.gc.Winner != 0 {
		t.Fatalf("gc.Winner = %d, want 0 after handling", mc.gc.Winner)
	}
	if len(events.roundOvers) != 1 {
		t.Fatalf("round over callbacks = %d, want 1", len(events.roundOvers))
	}
	if events.roundOvers[0].Winner != 1 {
		t.Fatalf("round winner = %d, want 1", events.roundOvers[0].Winner)
	}
}

func TestMatchControllerGameOverDetection(t *testing.T) {
	events := &testEvents{}
	mc := NewMatchController(5, 123, events)
	mc.state = MatchRunning

	mc.gc.Winner = 1
	mc.gc.Wins = [2]int{2, 0}
	mc.gc.BattlesPlayed = 2
	mc.gc.Tanks[1].Lives = 0

	mc.stepTick()

	if got := mc.GetState(); got != MatchGameOver {
		t.Fatalf("state = %v, want %v", got, MatchGameOver)
	}
	if len(events.roundOvers) != 1 {
		t.Fatalf("round over callbacks = %d, want 1", len(events.roundOvers))
	}
	if len(events.gameOvers) != 1 {
		t.Fatalf("game over callbacks = %d, want 1", len(events.gameOvers))
	}
	if events.gameOvers[0].Winner != 1 {
		t.Fatalf("game winner = %d, want 1", events.gameOvers[0].Winner)
	}
}

func TestMatchControllerGetInputReturnsCorrectTracker(t *testing.T) {
	mc := NewMatchController(5, 123, nil)

	p1 := mc.GetInput(1)
	p2 := mc.GetInput(2)
	p3 := mc.GetInput(3)

	if p1 == nil || p2 == nil {
		t.Fatal("expected non-nil trackers for players 1 and 2")
	}
	if p1 == p2 {
		t.Fatal("expected different trackers for players 1 and 2")
	}
	if p3 != nil {
		t.Fatal("expected nil tracker for invalid player id")
	}
}
