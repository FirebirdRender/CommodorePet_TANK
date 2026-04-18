package server

import (
	"fmt"
	"os"
	"sync"

	"github.com/FirebirdRender/CommodorePet_TANK/engine"
)

var inputDebugEnabled = os.Getenv("TANK_INPUT_DEBUG") == "1"

type InputTracker struct {
	held     map[engine.Action]bool
	pending  []engine.Action
	mu       sync.Mutex
	playerID int
}

func NewInputTracker() *InputTracker {
	return &InputTracker{
		held: make(map[engine.Action]bool),
	}
}

func (t *InputTracker) SetPlayerID(id int) {
	t.mu.Lock()
	t.playerID = id
	t.mu.Unlock()
}

func (t *InputTracker) KeyDown(action engine.Action) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.held[action] {
		if inputDebugEnabled && action == engine.ActionFire {
			fmt.Fprintf(os.Stderr, "INPUT p=%d KeyDown(Fire) DROPPED:already_held\n", t.playerID)
		}
		return
	}
	t.held[action] = true
	t.pending = append(t.pending, action)
	if inputDebugEnabled && action == engine.ActionFire {
		fmt.Fprintf(os.Stderr, "INPUT p=%d KeyDown(Fire) QUEUED pending_len=%d\n", t.playerID, len(t.pending))
	}
}

func (t *InputTracker) KeyUp(action engine.Action) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.held, action)
	if inputDebugEnabled && action == engine.ActionFire {
		fmt.Fprintf(os.Stderr, "INPUT p=%d KeyUp(Fire)\n", t.playerID)
	}
}

func (t *InputTracker) ConsumeAction() engine.Action {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.pending) > 0 {
		action := t.pending[0]
		dropped := len(t.pending) - 1
		t.pending = nil
		if inputDebugEnabled && (action == engine.ActionFire || dropped > 0) {
			fmt.Fprintf(os.Stderr, "INPUT p=%d Consume=%d dropped_others=%d\n", t.playerID, action, dropped)
		}
		return action
	}

	return engine.ActionNone
}

func (t *InputTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.held = make(map[engine.Action]bool)
	t.pending = nil
}

func ProcessInputMsg(msg *InputMsg, tracker *InputTracker) error {
	if msg == nil {
		return fmt.Errorf("input message is nil")
	}
	if tracker == nil {
		return fmt.Errorf("input tracker is nil")
	}

	action, ok := KeyToAction(msg.Key)
	if !ok {
		return fmt.Errorf("unknown input key: %q", msg.Key)
	}

	switch msg.Action {
	case "down":
		tracker.KeyDown(action)
	case "up":
		tracker.KeyUp(action)
	default:
		return fmt.Errorf("unknown input action: %q", msg.Action)
	}

	return nil
}
