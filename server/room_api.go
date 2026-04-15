package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type RoomAPI struct {
	hub      *Hub
	registry *ConnRegistry
	tokens   *TokenStore
}

func NewRoomAPI(hub *Hub, registry *ConnRegistry, tokens *TokenStore) *RoomAPI {
	return &RoomAPI{
		hub:      hub,
		registry: registry,
		tokens:   tokens,
	}
}

func (api *RoomAPI) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/room", api.handleCreateRoom)
	mux.HandleFunc("/api/room/", api.handleRoomRoutes)
}

func (api *RoomAPI) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Difficulty int    `json:"difficulty"`
		PlayerName string `json:"player_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := validateDifficulty(req.Difficulty); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validatePlayerName(req.PlayerName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	room := api.hub.CreateRoom(req.Difficulty)
	if room == nil {
		http.Error(w, "Failed to create room", http.StatusInternalServerError)
		return
	}

	playerID, err := room.AddPlayer(req.PlayerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	token := api.tokens.GenerateToken(room.Code, playerID, req.PlayerName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"room_code":   room.Code,
		"player_id":   playerID,
		"token":       token,
		"player_name": req.PlayerName,
	})
}

func (api *RoomAPI) handleRoomRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/room/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	code, action := parts[0], parts[1]

	if err := validateRoomCode(code); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch action {
	case "join":
		api.handleJoinRoom(w, r, code)
	case "status":
		api.handleRoomStatus(w, r, code)
	case "events":
		api.handleRoomEvents(w, r, code)
	default:
		http.NotFound(w, r)
	}
}

func (api *RoomAPI) handleJoinRoom(w http.ResponseWriter, r *http.Request, code string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PlayerName string `json:"player_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := validatePlayerName(req.PlayerName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	room := api.hub.GetRoom(code)
	if room == nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	if room.IsFull() {
		http.Error(w, "Room is full", http.StatusUnprocessableEntity)
		return
	}

	if room.GetState() == RoomClosed {
		http.Error(w, "Room is closed", http.StatusUnprocessableEntity)
		return
	}

	playerID, err := room.AddPlayer(req.PlayerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	token := api.tokens.GenerateToken(room.Code, playerID, req.PlayerName)

	var opponentName string
	otherID := 1
	if playerID == 1 {
		otherID = 2
	}
	if opp := room.GetPlayer(otherID); opp != nil {
		opponentName = opp.Name
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"room_code":     room.Code,
		"player_id":     playerID,
		"token":         token,
		"player_name":   req.PlayerName,
		"opponent_name": opponentName,
	})
}

func (api *RoomAPI) handleRoomStatus(w http.ResponseWriter, r *http.Request, code string) {
	room := api.hub.GetRoom(code)
	if room == nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	var status string
	switch room.GetState() {
	case RoomWaiting, RoomReady:
		status = "waiting"
	case RoomPlaying:
		status = "playing"
	case RoomGameOver:
		status = "game_over"
	case RoomClosed:
		status = "closed"
	}

	players := make([]map[string]any, 0)
	room.mu.Lock()
	for _, p := range room.Players {
		if p != nil {
			players = append(players, map[string]any{"id": p.ID, "name": p.Name})
		}
	}
	diff := room.Difficulty
	room.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":     status,
		"difficulty": diff,
		"players":    players,
	})
}

func (api *RoomAPI) handleRoomEvents(w http.ResponseWriter, r *http.Request, code string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastPlayerCount int
	var lastState RoomState

	// Initial check
	room := api.hub.GetRoom(code)
	if room != nil {
		room.mu.Lock()
		lastPlayerCount = room.connectedCountLocked()
		lastState = room.State
		room.mu.Unlock()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			return
		case <-ticker.C:
			room = api.hub.GetRoom(code)
			if room == nil {
				fmt.Fprintf(w, "event: room_expired\ndata: {}\n\n")
				flusher.Flush()
				return
			}

			room.mu.Lock()
			currCount := room.connectedCountLocked()
			currState := room.State
			roomCode := room.Code
			room.mu.Unlock()

			if currCount > lastPlayerCount && currCount == 2 {
				var oppName string
				room.mu.Lock()
				for _, p := range room.Players {
					if p != nil && p.ID != 1 {
						oppName = p.Name
					}
				}
				room.mu.Unlock()
				data, _ := json.Marshal(map[string]string{"room_code": roomCode, "opponent_name": oppName})
				fmt.Fprintf(w, "event: player_joined\ndata: %s\n\n", string(data))
				flusher.Flush()
			}

			if currState == RoomPlaying && lastState != RoomPlaying {
				data, _ := json.Marshal(map[string]string{"room_code": roomCode})
				fmt.Fprintf(w, "event: game_starting\ndata: %s\n\n", string(data))
				flusher.Flush()
			}

			lastPlayerCount = currCount
			lastState = currState
		}
	}
}
