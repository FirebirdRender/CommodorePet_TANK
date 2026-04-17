package server

import (
	"crypto/rand"
	"math/big"
	"sync"
	"time"
)

type Hub struct {
	rooms map[string]*Room
	mu    sync.Mutex
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*Room)}
}

func (h *Hub) CreateRoom(difficulty int) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	for range 100 {
		code := h.generateCode()
		if code == "" {
			return nil
		}
		if _, exists := h.rooms[code]; exists {
			continue
		}

		room := NewRoom(code, difficulty)
		h.rooms[code] = room
		return room
	}

	return nil
}

func (h *Hub) GetRoom(code string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rooms[code]
}

func (h *Hub) RemoveRoom(code string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, code)
}

func (h *Hub) CreateRoomWithBotPolicy(difficulty int, allowBot, autoFillBot bool, autoFillAfterSec int) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	for range 100 {
		code := h.generateCode()
		if code == "" {
			return nil
		}
		if _, exists := h.rooms[code]; exists {
			continue
		}

		room := NewRoomWithBotPolicy(code, difficulty, allowBot, autoFillBot, autoFillAfterSec)
		h.rooms[code] = room
		return room
	}

	return nil
}

func (h *Hub) RoomCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms)
}

func (h *Hub) CleanupStaleRooms(maxAge time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for code, room := range h.rooms {
		room.mu.Lock()
		state := room.State
		createdAt := room.CreatedAt
		room.mu.Unlock()

		if state == RoomClosed {
			delete(h.rooms, code)
			continue
		}

		if state == RoomWaiting && now.Sub(createdAt) > maxAge {
			delete(h.rooms, code)
		}
	}
}

func (h *Hub) generateCode() string {
	// Unambiguous alphabet excludes I, L, O, 0, 1 to prevent player confusion
	alph := "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	b := make([]byte, 4)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alph))))
		if err != nil {
			return ""
		}
		b[i] = alph[n.Int64()]
	}
	return string(b)
}

// Shutdown notifies all connected players of server shutdown,
// stops running matches, and waits for cleanup.
func (h *Hub) Shutdown(timeout time.Duration, registry *ConnRegistry) {
	h.mu.Lock()
	codes := make([]string, 0, len(h.rooms))
	for code := range h.rooms {
		codes = append(codes, code)
	}
	h.mu.Unlock()

	// Notify all connected clients
	for _, code := range codes {
		conns := registry.GetClients(code)
		for _, conn := range conns {
			if conn != nil {
				conn.sendOrLog(MsgTypeOpponentLeft, OpponentLeftMsg{Reason: "server_shutdown"})
			}
		}

		// Stop running matches
		mc := registry.GetMatch(code)
		if mc != nil {
			mc.Stop()
		}
	}

	// Wait for matches to clean up (with timeout)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		remaining := len(h.rooms)
		h.mu.Unlock()
		if remaining == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force cleanup remaining rooms
	h.mu.Lock()
	for code := range h.rooms {
		delete(h.rooms, code)
	}
	h.mu.Unlock()
	registry.RemoveAllRooms()
}
