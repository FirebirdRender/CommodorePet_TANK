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
	b := make([]byte, 4)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(26))
		if err != nil {
			return ""
		}
		b[i] = byte('A' + n.Int64())
	}
	return string(b)
}
