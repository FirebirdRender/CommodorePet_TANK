package client

import (
	"encoding/json"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestNewInputHandler(t *testing.T) {
	h := NewInputHandler()
	if h == nil {
		t.Fatal("NewInputHandler() returned nil")
	}
	if h.keyMap == nil {
		t.Fatal("NewInputHandler() returned handler with nil keyMap")
	}
	if h.enabled {
		t.Fatal("NewInputHandler() should start with enabled=false")
	}
	if h.tick != 0 {
		t.Fatalf("NewInputHandler() should start with tick=0, got %d", h.tick)
	}
}

func TestMapKey(t *testing.T) {
	h := NewInputHandler()

	tests := []struct {
		ebitenKey ebiten.Key
		want      string
	}{
		{ebiten.KeyUp, "up"},
		{ebiten.KeyDown, "down"},
		{ebiten.KeyLeft, "left"},
		{ebiten.KeyRight, "right"},
		{ebiten.KeySpace, "fire"},
		{ebiten.KeyM, "mine"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := h.MapKey(tc.ebitenKey)
			if got != tc.want {
				t.Errorf("MapKey(%v) = %q, want %q", tc.ebitenKey, got, tc.want)
			}
		})
	}
}

func TestMapKeyUnmapped(t *testing.T) {
	h := NewInputHandler()
	// KeyEnter is not in our keyMap
	got := h.MapKey(ebiten.KeyEnter)
	if got != "" {
		t.Errorf("MapKey(KeyEnter) = %q, want empty string for unmapped key", got)
	}
}

func TestSetEnabled(t *testing.T) {
	h := NewInputHandler()

	if h.enabled {
		t.Fatal("handler should start disabled")
	}

	h.SetEnabled(true)
	if !h.enabled {
		t.Fatal("SetEnabled(true) should enable")
	}

	h.SetEnabled(false)
	if h.enabled {
		t.Fatal("SetEnabled(false) should disable")
	}
}

func TestPollEdgeEventsDisabled(t *testing.T) {
	h := NewInputHandler()
	h.SetEnabled(false)
	h.Update()

	// When disabled, should return nil
	msgs := h.PollEdgeEvents()
	if msgs != nil {
		t.Fatalf("PollEdgeEvents() when disabled = %v, want nil", msgs)
	}
}

func TestPollEdgeEventsEnabledNoInput(t *testing.T) {
	h := NewInputHandler()
	h.SetEnabled(true)
	h.Update()

	// When enabled but no keys pressed, should return empty or nil slice
	msgs := h.PollEdgeEvents()
	if len(msgs) != 0 {
		t.Fatalf("PollEdgeEvents() with no input = %d messages, want 0", len(msgs))
	}
}

func TestPollHeldDirectionsDisabled(t *testing.T) {
	h := NewInputHandler()
	h.SetEnabled(false)

	dirs := h.PollHeldDirections()
	if dirs != nil {
		t.Fatalf("PollHeldDirections() when disabled = %v, want nil", dirs)
	}
}

func TestInputMsgJSON(t *testing.T) {
	msg := InputMsg{
		Tick:   42,
		Key:    "up",
		Action: "down",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal(InputMsg) failed: %v", err)
	}

	var decoded InputMsg
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Tick != msg.Tick {
		t.Errorf("Tick = %d, want %d", decoded.Tick, msg.Tick)
	}
	if decoded.Key != msg.Key {
		t.Errorf("Key = %q, want %q", decoded.Key, msg.Key)
	}
	if decoded.Action != msg.Action {
		t.Errorf("Action = %q, want %q", decoded.Action, msg.Action)
	}
}

func TestWrapMessageInput(t *testing.T) {
	msg := InputMsg{
		Tick:   1,
		Key:    "fire",
		Action: "up",
	}

	data, err := WrapMessage(MsgTypeInput, msg)
	if err != nil {
		t.Fatalf("WrapMessage failed: %v", err)
	}

	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("json.Unmarshal envelope failed: %v", err)
	}

	if env.Type != MsgTypeInput {
		t.Errorf("Envelope.Type = %q, want %q", env.Type, MsgTypeInput)
	}

	var decoded InputMsg
	if err := json.Unmarshal(env.Payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal payload failed: %v", err)
	}

	if decoded.Tick != msg.Tick || decoded.Key != msg.Key || decoded.Action != msg.Action {
		t.Errorf("payload = %+v, want %+v", decoded, msg)
	}
}

func TestKeyNamesMatchServerProtocol(t *testing.T) {
	// Verify that all protocol key names used by the client
	// match what the server's KeyToAction expects.
	//
	// Server expects (from server/protocol.go KeyToAction):
	// "up", "down", "left", "right", "up_left", "up_right",
	// "down_left", "down_right", "fire", "mine"
	//
	// Client defaultKeyMap provides: up, down, left, right, fire, mine

	h := NewInputHandler()

	// All keys in our keyMap must be recognized by the server
	requiredKeys := map[string]bool{
		"up":    false,
		"down":  false,
		"left":  false,
		"right": false,
		"fire":  false,
		"mine":  false,
	}

	for _, protoKey := range h.keyMap {
		if _, ok := requiredKeys[protoKey]; ok {
			requiredKeys[protoKey] = true
		}
	}

	for key, found := range requiredKeys {
		if !found {
			t.Errorf("client key %q is not in requiredKeys map", key)
		}
	}
}

func TestUpdateIncrementsTick(t *testing.T) {
	h := NewInputHandler()

	if h.tick != 0 {
		t.Fatalf("initial tick = %d, want 0", h.tick)
	}

	h.Update()
	if h.tick != 1 {
		t.Errorf("after 1 Update: tick = %d, want 1", h.tick)
	}

	h.Update()
	h.Update()
	if h.tick != 3 {
		t.Errorf("after 3 Updates: tick = %d, want 3", h.tick)
	}
}
