package server

import (
	"fmt"
	"sync"

	"github.com/FirebirdRender/CommodorePet_TANK/engine"
)

type InputTracker struct {
	held    map[engine.Action]bool
	pending []engine.Action
	mu      sync.Mutex
}

func NewInputTracker() *InputTracker {
	return &InputTracker{
		held: make(map[engine.Action]bool),
	}
}

func (t *InputTracker) KeyDown(action engine.Action) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.held[action] {
		return
	}
	t.held[action] = true
	t.pending = append(t.pending, action)
}

func (t *InputTracker) KeyUp(action engine.Action) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.held, action)
}

func (t *InputTracker) ConsumeAction() engine.Action {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.pending) > 0 {
		action := t.pending[0]
		t.pending = nil
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
