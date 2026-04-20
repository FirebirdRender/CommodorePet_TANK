package server

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

type GlobalSSEClient struct {
	w       http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
}

type Hub struct {
	rooms            map[string]*Room
	mu               sync.Mutex
	globalSSEClients map[*GlobalSSEClient]struct{}
	globalSSEMu      sync.Mutex
	maxRooms         int
}

func NewHub(maxRooms int) *Hub {
	return &Hub{
		rooms:            make(map[string]*Room),
		globalSSEClients: make(map[*GlobalSSEClient]struct{}),
		maxRooms:         maxRooms,
	}
}

func (h *Hub) CreateRoom(difficulty int) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	// S3: Room count cap
	if h.maxRooms > 0 && len(h.rooms) >= h.maxRooms {
		return nil
	}

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

	// S3: Room count cap
	if h.maxRooms > 0 && len(h.rooms) >= h.maxRooms {
		return nil
	}

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

func (h *Hub) GetPublicRooms() []*Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	var result []*Room
	for _, room := range h.rooms {
		if room.IsPublic() && room.GetState() == RoomWaiting {
			result = append(result, room)
		}
	}
	return result
}

func (h *Hub) GetPlayingRooms() []*Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	var result []*Room
	for _, room := range h.rooms {
		if room.IsPublic() && room.GetState() == RoomPlaying {
			result = append(result, room)
		}
	}
	return result
}

func (h *Hub) GetBotMatchCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for _, room := range h.rooms {
		if room.AllowBot && !room.AutoFillBot && room.BothPlayersConnected() {
			p1 := room.Players[0]
			p2 := room.Players[1]
			if p1 != nil && p2 != nil && p1.IsBot && p2.IsBot {
				count++
			}
		}
	}
	return count
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

		// S3/S4: Clean up stale waiting and game-over rooms; cancel auto-fill timers
		if (state == RoomWaiting || state == RoomGameOver) && now.Sub(createdAt) > maxAge {
			room.CancelBotReservation()
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

func (h *Hub) AddGlobalSSEClient(c *GlobalSSEClient) {
	h.globalSSEMu.Lock()
	h.globalSSEClients[c] = struct{}{}
	h.globalSSEMu.Unlock()
}

func (h *Hub) RemoveGlobalSSEClient(c *GlobalSSEClient) {
	h.globalSSEMu.Lock()
	delete(h.globalSSEClients, c)
	h.globalSSEMu.Unlock()
	close(c.done)
}

func (h *Hub) BroadcastGlobal(event string, data any) {
	h.globalSSEMu.Lock()
	defer h.globalSSEMu.Unlock()

	payload, _ := json.Marshal(data)
	for client := range h.globalSSEClients {
		fmt.Fprintf(client.w, "event: %s\ndata: %s\n\n", event, string(payload))
		client.flusher.Flush()
	}
}

func (h *Hub) BroadcastMatchAvailable(roomCode string, difficulty int) {
	h.BroadcastGlobal("match_available", map[string]any{"room_code": roomCode, "difficulty": difficulty})
}

func (h *Hub) BroadcastMatchStarted(roomCode string) {
	h.BroadcastGlobal("match_started", map[string]any{"room_code": roomCode})
}

func (h *Hub) BroadcastMatchEnded(roomCode string) {
	h.BroadcastGlobal("match_ended", map[string]any{"room_code": roomCode})
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
