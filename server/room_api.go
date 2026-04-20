package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type RoomAPI struct {
	hub        *Hub
	registry   *ConnRegistry
	tokens     *TokenStore
	botManager *BotManager
	staticDir  string
	cors       string
}

func NewRoomAPI(hub *Hub, registry *ConnRegistry, tokens *TokenStore, cors string) *RoomAPI {
	return &RoomAPI{
		hub:       hub,
		registry:  registry,
		tokens:    tokens,
		staticDir: "web",
		cors:      cors,
	}
}

func (api *RoomAPI) SetBotManager(bm *BotManager) {
	api.botManager = bm
}

// SetStaticDir overrides the directory used to serve static HTML pages
// (e.g. /spectate/{code} -> {staticDir}/spectate.html). Defaults to "web".
func (api *RoomAPI) SetStaticDir(dir string) {
	if dir != "" {
		api.staticDir = dir
	}
}

func (api *RoomAPI) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/room", api.handleCreateRoom)
	mux.HandleFunc("/api/room/", api.handleRoomRoutes)
	mux.HandleFunc("/api/rooms", api.handleListRooms)
	mux.HandleFunc("/api/events", api.handleGlobalEvents)
	mux.HandleFunc("/spectate/", api.handleSpectatePage)
}

func (api *RoomAPI) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// B3: Request body size limit (4KB)
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req struct {
		Difficulty       int    `json:"difficulty"`
		PlayerName       string `json:"player_name"`
		VsAI             bool   `json:"vs_ai,omitempty"`
		AutoFillBot      bool   `json:"auto_fill_bot,omitempty"`
		AutoFillAfterSec int    `json:"auto_fill_after_sec,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
		}
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

	var room *Room
	if req.VsAI || req.AutoFillBot {
		room = api.hub.CreateRoomWithBotPolicy(req.Difficulty, req.VsAI, req.AutoFillBot, req.AutoFillAfterSec)
	} else {
		room = api.hub.CreateRoom(req.Difficulty)
	}
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

	botAssigned := false
	if req.VsAI && api.botManager != nil && api.botManager.IsEnabled() {
		if err := api.botManager.AssignBotToRoom(room.Code, "mvp"); err == nil {
			botAssigned = true
		} else {
			log.Printf("[bot] failed to assign bot to room %s: %v", room.Code, err)
		}
	} else if req.AutoFillBot && req.AutoFillAfterSec > 0 && api.botManager != nil && api.botManager.IsEnabled() {
		room.SetAutoFillTimer(time.AfterFunc(time.Duration(req.AutoFillAfterSec)*time.Second, func() {
			if err := api.botManager.AssignBotToRoom(room.Code, "mvp"); err != nil {
				log.Printf("[bot] auto-fill timer triggered but failed to assign bot to room %s: %v", room.Code, err)
			}
		}))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"room_code":     room.Code,
		"player_id":     playerID,
		"token":         token,
		"player_name":   req.PlayerName,
		"vs_ai":         req.VsAI,
		"auto_fill_bot": req.AutoFillBot,
		"bot_assigned":  botAssigned,
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

	// B3: Request body size limit (4KB)
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req struct {
		PlayerName string `json:"player_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
		}
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

	if playerID == 2 && room.AutoFillBot && api.botManager != nil {
		room.CancelBotReservation()
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
			player := map[string]any{"id": p.ID, "name": p.Name}
			if p.IsBot {
				player["is_bot"] = true
				if p.BotClass != "" {
					player["bot_class"] = p.BotClass
				}
			}
			players = append(players, player)
		}
	}
	diff := room.Difficulty
	allowBot := room.AllowBot
	autoFillBot := room.AutoFillBot
	autoFillAfterSec := room.AutoFillAfterSec
	room.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":              status,
		"difficulty":          diff,
		"players":             players,
		"allow_bot":           allowBot,
		"auto_fill_bot":       autoFillBot,
		"auto_fill_after_sec": autoFillAfterSec,
	})
}

func (api *RoomAPI) handleRoomEvents(w http.ResponseWriter, r *http.Request, code string) {
	// S2: Require auth token for SSE
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Auth-Token")
	}
	if token == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	entry, valid := api.tokens.ValidateToken(token)
	if !valid {
		http.Error(w, "Invalid or expired token", http.StatusForbidden)
		return
	}
	if entry.RoomCode != code {
		http.Error(w, "Token doesn't belong to this room", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", api.cors)

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

	// Initial check — if the room already has 2 players at stream-open time
	// (common in VS AI mode, where the bot is added synchronously during
	// POST /api/room before the client opens the SSE stream), emit the
	// appropriate join event immediately. Otherwise the edge-triggered
	// detection below will never fire because there is no 1→2 transition.
	room := api.hub.GetRoom(code)
	if room != nil {
		room.mu.Lock()
		initialCount := room.connectedCountLocked()
		lastState = room.State
		roomCode := room.Code
		var oppName string
		var isBot bool
		if initialCount == 2 {
			for _, p := range room.Players {
				if p != nil && p.ID != 1 {
					oppName = p.Name
					isBot = p.IsBot
				}
			}
		}
		room.mu.Unlock()

		if initialCount == 2 {
			if isBot {
				data, _ := json.Marshal(map[string]interface{}{"room_code": roomCode, "opponent_name": oppName, "is_bot": true})
				fmt.Fprintf(w, "event: bot_joined\ndata: %s\n\n", string(data))
			} else {
				data, _ := json.Marshal(map[string]string{"room_code": roomCode, "opponent_name": oppName})
				fmt.Fprintf(w, "event: player_joined\ndata: %s\n\n", string(data))
			}
			flusher.Flush()
		}
		lastPlayerCount = initialCount
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
				var isBot bool
				room.mu.Lock()
				for _, p := range room.Players {
					if p != nil && p.ID != 1 {
						oppName = p.Name
						isBot = p.IsBot
					}
				}
				room.mu.Unlock()

				if isBot {
					data, _ := json.Marshal(map[string]interface{}{"room_code": roomCode, "opponent_name": oppName, "is_bot": true})
					fmt.Fprintf(w, "event: bot_joined\ndata: %s\n\n", string(data))
				} else {
					data, _ := json.Marshal(map[string]string{"room_code": roomCode, "opponent_name": oppName})
					fmt.Fprintf(w, "event: player_joined\ndata: %s\n\n", string(data))
				}
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

func (api *RoomAPI) handleListRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filter := r.URL.Query().Get("filter")

	var rooms []*Room
	switch filter {
	case "playing":
		rooms = api.hub.GetPlayingRooms()
	case "waiting", "":
		rooms = api.hub.GetPublicRooms()
	default:
		http.Error(w, "Invalid filter", http.StatusBadRequest)
		return
	}

	type playerSummary struct {
		Name  string `json:"name"`
		IsBot bool   `json:"is_bot"`
	}
	type roomSummary struct {
		Code       string          `json:"code"`
		Difficulty int             `json:"difficulty"`
		State      string          `json:"state"`
		Players    []playerSummary `json:"players"`
	}

	out := make([]roomSummary, 0, len(rooms))
	for _, room := range rooms {
		state := room.GetState()
		var stateStr string
		switch state {
		case RoomWaiting:
			stateStr = "waiting"
		case RoomReady:
			stateStr = "ready"
		case RoomPlaying:
			stateStr = "playing"
		case RoomGameOver:
			stateStr = "game_over"
		case RoomWaitingReconnect:
			stateStr = "waiting_reconnect"
		default:
			stateStr = "unknown"
		}

		players := make([]playerSummary, 0, 2)
		room.mu.Lock()
		for _, p := range room.Players {
			if p == nil {
				continue
			}
			players = append(players, playerSummary{Name: p.Name, IsBot: p.IsBot})
		}
		room.mu.Unlock()

		out = append(out, roomSummary{
			Code:       room.Code,
			Difficulty: room.Difficulty,
			State:      stateStr,
			Players:    players,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (api *RoomAPI) handleGlobalEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	client := &GlobalSSEClient{
		w:       w,
		flusher: flusher,
		done:    make(chan struct{}),
	}
	api.hub.AddGlobalSSEClient(client)
	defer func() {
		// RemoveGlobalSSEClient closes done; guard against double-close on ctx cancel.
		defer func() { _ = recover() }()
		api.hub.RemoveGlobalSSEClient(client)
	}()

	// Initial keepalive comment so EventSource transitions to OPEN immediately.
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.done:
			return
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (api *RoomAPI) handleSpectatePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/spectate/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	if err := validateRoomCode(path); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, api.staticDir+"/spectate.html")
}

func getCwd() string {
	dir, _ := os.Getwd()
	return dir
}
