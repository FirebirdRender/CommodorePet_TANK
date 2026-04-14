package server

import (
	"sync"
	"testing"

	"github.com/FirebirdRender/CommodorePet_TANK/engine"
)

func TestNewInputTrackerStartsEmpty(t *testing.T) {
	tracker := NewInputTracker()
	if got := tracker.ConsumeAction(); got != engine.ActionNone {
		t.Fatalf("ConsumeAction() = %v, want %v", got, engine.ActionNone)
	}
}

func TestSingleKeyDownConsumeAction(t *testing.T) {
	tracker := NewInputTracker()
	tracker.KeyDown(engine.ActionUp)

	if got := tracker.ConsumeAction(); got != engine.ActionUp {
		t.Fatalf("ConsumeAction() = %v, want %v", got, engine.ActionUp)
	}
}

func TestKeyDownKeyUpBeforeConsumeStillReturnsEdge(t *testing.T) {
	tracker := NewInputTracker()
	tracker.KeyDown(engine.ActionRight)
	tracker.KeyUp(engine.ActionRight)

	if got := tracker.ConsumeAction(); got != engine.ActionRight {
		t.Fatalf("ConsumeAction() = %v, want %v", got, engine.ActionRight)
	}
}

func TestMultipleKeyDownEdgesConsumeFIFOAndClearPending(t *testing.T) {
	tracker := NewInputTracker()
	tracker.KeyDown(engine.ActionFire)
	tracker.KeyDown(engine.ActionPlaceMine)

	if got := tracker.ConsumeAction(); got != engine.ActionFire {
		t.Fatalf("first ConsumeAction() = %v, want %v", got, engine.ActionFire)
	}

	if got := tracker.ConsumeAction(); got != engine.ActionNone {
		t.Fatalf("second ConsumeAction() = %v, want %v", got, engine.ActionNone)
	}
}

func TestHeldKeyContinuousMovement(t *testing.T) {
	tracker := NewInputTracker()
	tracker.KeyDown(engine.ActionUp)

	if got := tracker.ConsumeAction(); got != engine.ActionUp {
		t.Fatalf("first ConsumeAction() = %v, want %v", got, engine.ActionUp)
	}
	if got := tracker.ConsumeAction(); got != engine.ActionUp {
		t.Fatalf("second ConsumeAction() = %v, want %v", got, engine.ActionUp)
	}
}

func TestFireIsEdgeOnly(t *testing.T) {
	tracker := NewInputTracker()
	tracker.KeyDown(engine.ActionFire)

	if got := tracker.ConsumeAction(); got != engine.ActionFire {
		t.Fatalf("first ConsumeAction() = %v, want %v", got, engine.ActionFire)
	}
	if got := tracker.ConsumeAction(); got != engine.ActionNone {
		t.Fatalf("second ConsumeAction() = %v, want %v", got, engine.ActionNone)
	}
}

func TestMineIsEdgeOnly(t *testing.T) {
	tracker := NewInputTracker()
	tracker.KeyDown(engine.ActionPlaceMine)

	if got := tracker.ConsumeAction(); got != engine.ActionPlaceMine {
		t.Fatalf("first ConsumeAction() = %v, want %v", got, engine.ActionPlaceMine)
	}
	if got := tracker.ConsumeAction(); got != engine.ActionNone {
		t.Fatalf("second ConsumeAction() = %v, want %v", got, engine.ActionNone)
	}
}

func TestRepeatSuppression(t *testing.T) {
	tracker := NewInputTracker()
	tracker.KeyDown(engine.ActionLeft)
	tracker.KeyDown(engine.ActionLeft)

	if got := tracker.ConsumeAction(); got != engine.ActionLeft {
		t.Fatalf("first ConsumeAction() = %v, want %v", got, engine.ActionLeft)
	}
	if got := tracker.ConsumeAction(); got != engine.ActionLeft {
		t.Fatalf("second ConsumeAction() = %v, want held fallback %v", got, engine.ActionLeft)
	}
}

func TestKeyUpRemovesFromHeld(t *testing.T) {
	tracker := NewInputTracker()
	tracker.KeyDown(engine.ActionDown)
	if got := tracker.ConsumeAction(); got != engine.ActionDown {
		t.Fatalf("first ConsumeAction() = %v, want %v", got, engine.ActionDown)
	}

	tracker.KeyUp(engine.ActionDown)
	if got := tracker.ConsumeAction(); got != engine.ActionNone {
		t.Fatalf("ConsumeAction() after KeyUp = %v, want %v", got, engine.ActionNone)
	}
}

func TestResetClearsEverything(t *testing.T) {
	tracker := NewInputTracker()
	tracker.KeyDown(engine.ActionUp)
	tracker.KeyDown(engine.ActionFire)
	tracker.Reset()

	if got := tracker.ConsumeAction(); got != engine.ActionNone {
		t.Fatalf("ConsumeAction() after Reset = %v, want %v", got, engine.ActionNone)
	}

	tracker.KeyDown(engine.ActionRight)
	if got := tracker.ConsumeAction(); got != engine.ActionRight {
		t.Fatalf("ConsumeAction() after Reset and new KeyDown = %v, want %v", got, engine.ActionRight)
	}
}

func TestProcessInputMsgValid(t *testing.T) {
	tracker := NewInputTracker()

	if err := ProcessInputMsg(&InputMsg{Key: "up", Action: "down"}, tracker); err != nil {
		t.Fatalf("ProcessInputMsg(down) error = %v", err)
	}
	if got := tracker.ConsumeAction(); got != engine.ActionUp {
		t.Fatalf("ConsumeAction() after down = %v, want %v", got, engine.ActionUp)
	}

	if err := ProcessInputMsg(&InputMsg{Key: "up", Action: "up"}, tracker); err != nil {
		t.Fatalf("ProcessInputMsg(up) error = %v", err)
	}
	if got := tracker.ConsumeAction(); got != engine.ActionNone {
		t.Fatalf("ConsumeAction() after key up = %v, want %v", got, engine.ActionNone)
	}

	if err := ProcessInputMsg(&InputMsg{Key: "fire", Action: "down"}, tracker); err != nil {
		t.Fatalf("ProcessInputMsg(fire down) error = %v", err)
	}
	if got := tracker.ConsumeAction(); got != engine.ActionFire {
		t.Fatalf("ConsumeAction() after fire down = %v, want %v", got, engine.ActionFire)
	}
}

func TestProcessInputMsgUnknownKeyReturnsError(t *testing.T) {
	tracker := NewInputTracker()
	err := ProcessInputMsg(&InputMsg{Key: "teleport", Action: "down"}, tracker)
	if err == nil {
		t.Fatalf("expected error for unknown key")
	}
}

func TestProcessInputMsgUnknownActionReturnsError(t *testing.T) {
	tracker := NewInputTracker()
	err := ProcessInputMsg(&InputMsg{Key: "up", Action: "press"}, tracker)
	if err == nil {
		t.Fatalf("expected error for unknown action string")
	}
}

func TestInputTrackerConcurrentAccessRaceSafe(t *testing.T) {
	tracker := NewInputTracker()
	var wg sync.WaitGroup

	workers := 8
	iterations := 1000

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				tracker.KeyDown(engine.ActionUp)
				tracker.KeyUp(engine.ActionUp)
				tracker.KeyDown(engine.ActionFire)
				tracker.KeyUp(engine.ActionFire)
			}
		}()
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				_ = tracker.ConsumeAction()
			}
		}()
	}

	wg.Wait()

	got := tracker.ConsumeAction()
	if got < engine.ActionNone || got > engine.ActionPlaceMine {
		t.Fatalf("ConsumeAction() produced invalid action %v", got)
	}
}
