package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// KeyAction maps an ebiten.Key to the protocol key name and action.
// This is sent as InputMsg to the server.
type KeyAction struct {
	Key    string // Protocol key name: "up", "down", "left", "right", etc.
	Action string // "down" or "up"
}

// InputHandler translates keyboard input to server protocol messages.
// It uses Ebitengine's inpututil for edge detection (just pressed / just released).
type InputHandler struct {
	keyMap  map[ebiten.Key]string // ebiten key → protocol key name
	tick    uint64                // incremented each Update()
	enabled bool                  // only send when in Playing phase
}

// Default key mapping matching the server's KeyToAction function.
// Arrow keys are primary, WASD are alternatives for directional movement.
var defaultKeyMap = map[ebiten.Key]string{
	ebiten.KeyUp:    "up",
	ebiten.KeyDown:  "down",
	ebiten.KeyLeft:  "left",
	ebiten.KeyRight: "right",
	ebiten.KeyW:     "up",
	ebiten.KeyS:     "down",
	ebiten.KeyA:     "left",
	ebiten.KeyD:     "right",
	ebiten.KeySpace: "fire",
	ebiten.KeyM:     "mine",
	ebiten.KeyEnter: "fire",
}

// NewInputHandler creates a new InputHandler with the default key mapping.
func NewInputHandler() *InputHandler {
	return &InputHandler{
		keyMap: defaultKeyMap,
	}
}

// SetEnabled controls whether input messages are generated.
// Only enabled during Playing phase.
func (h *InputHandler) SetEnabled(enabled bool) {
	h.enabled = enabled
}

// Update increments the tick counter. Call this once per game loop tick.
func (h *InputHandler) Update() {
	h.tick++
}

// PollEdgeEvents checks for KEYDOWN/KEYUP events since the last frame.
// Returns a slice of InputMsg to send to the server.
// Only returns messages when enabled (Playing phase).
func (h *InputHandler) PollEdgeEvents() []InputMsg {
	if !h.enabled {
		return nil
	}

	var msgs []InputMsg

	for key, protoKey := range h.keyMap {
		if inpututil.IsKeyJustPressed(key) {
			msgs = append(msgs, InputMsg{
				Tick:   h.tick,
				Key:    protoKey,
				Action: "down",
			})
		}
		if inpututil.IsKeyJustReleased(key) {
			msgs = append(msgs, InputMsg{
				Tick:   h.tick,
				Key:    protoKey,
				Action: "up",
			})
		}
	}

	return msgs
}

// PollHeldDirections checks which direction keys are currently held.
// Returns the set of protocol direction names that are held.
// Used for diagonal movement: if both "up" and "left" are held,
// the server receives both and interprets as "up_left".
func (h *InputHandler) PollHeldDirections() []string {
	if !h.enabled {
		return nil
	}

	seen := make(map[string]bool)
	var dirs []string
	for key, protoKey := range h.keyMap {
		switch protoKey {
		case "up", "down", "left", "right":
			if ebiten.IsKeyPressed(key) && !seen[protoKey] {
				seen[protoKey] = true
				dirs = append(dirs, protoKey)
			}
		}
	}

	return dirs
}

// MapKey returns the protocol key name for an ebiten key.
// Exported for testing without calling ebiten polling functions.
func (h *InputHandler) MapKey(key ebiten.Key) string {
	return h.keyMap[key]
}
