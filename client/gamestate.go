package client

import "log"

// GamePhase represents the current phase of the game
type GamePhase int

const (
	PhaseConnecting GamePhase = iota
	PhaseLobby
	PhasePlaying
	PhaseRoundOver
	PhaseGameOver
	PhaseDisconnected
)

// LobbyMode represents the current lobby sub-state
type LobbyMode int

const (
	LobbyModeStart      LobbyMode = iota
	LobbyModeDifficulty           // selecting difficulty before creating room
	LobbyModeCreating
	LobbyModeJoining
	LobbyModeWaiting
	LobbyModeRematch // waiting for opponent to accept rematch
)

// KeyframeInterval is the number of ticks between full keyframes (must match server)
const KeyframeInterval = 60

// GameState holds all client-side game state
type GameState struct {
	Phase        GamePhase
	PlayerID     int // 1 or 2 — set by game_start
	PlayerName   string
	Difficulty   int
	Grid         [][]int // 21 rows x 40 cols (grid[y][x]), values 1-10
	Tanks        [2]TankState
	Shots        []ShotState
	Mines        []MineState
	Explosions   []ExplosionState
	RoomCode     string
	OpponentName string

	// Round/game tracking
	Wins          [2]int
	BattlesPlayed int
	Winner        int // 0 = no winner yet, 1 or 2 = round/match winner

	// Animation tracking (not from server)
	AnimTick        int                // incremented each Update()
	ExplosionTimers map[[2]int]float64 // (x,y) → remaining duration in seconds

	// DirtyCells tracks grid cells that changed in the last tick.
	// nil means full redraw needed (keyframe or game start).
	// Non-nil contains only the changed cells (for future retained-mode rendering).
	DirtyCells map[[2]int]bool

	// DesyncCount tracks consecutive delta ticks since last keyframe.
	// Reset to 0 on keyframe, incremented on delta. Logs warning if > KeyframeInterval * 2.
	DesyncCount int

	// Network status
	Connected bool
	Error     *ErrorMsg // latest error from server, nil if no error

	// Lobby UI state
	LobbyMode            LobbyMode
	DifficultySelection  int // selected difficulty (1-10) before creating room
	RoomCodeInput        string
	ErrorMsgText         string
	ConnectErr           string // connection error message for display
	OpponentWantsRematch bool

	// Barrel prediction: playerID → predicted direction (only for local player)
	PredictedDir map[int]int
}

func NewGameState() *GameState {
	return &GameState{
		Phase:               PhaseConnecting,
		DifficultySelection: 5,
		ExplosionTimers:     make(map[[2]int]float64),
		DirtyCells:          nil,
	}
}

// ApplyGameStart processes a game_start message
func (gs *GameState) ApplyGameStart(msg GameStartMsg) {
	// Validate grid dimensions before any state modification
	if len(msg.Grid) == 0 || len(msg.Grid[0]) == 0 {
		log.Printf("game_start has empty grid")
		return
	}
	// Only check dimensions if existing grid is initialized (non-empty)
	if len(gs.Grid) > 0 && (len(msg.Grid) != len(gs.Grid) || len(msg.Grid[0]) != len(gs.Grid[0])) {
		log.Printf("game_start grid dimension mismatch: expected %d rows x %d cols, got %d rows x %d cols",
			len(gs.Grid), len(gs.Grid[0]), len(msg.Grid), len(msg.Grid[0]))
	}

	gs.Phase = PhasePlaying
	gs.PlayerID = msg.YourPlayerID
	gs.Difficulty = msg.Difficulty
	gs.Grid = msg.Grid
	gs.Tanks = msg.Tanks
	gs.Shots = nil
	gs.Mines = nil
	gs.Explosions = nil
	gs.ExplosionTimers = make(map[[2]int]float64)
	gs.DirtyCells = nil
	gs.DesyncCount = 0
	gs.Winner = 0
	gs.Error = nil
	gs.OpponentWantsRematch = false
}

// ApplyTick processes a tick message from the server
func (gs *GameState) ApplyTick(msg TickMsg) {
	// Validate grid dimensions before assignment
	if len(gs.Grid) > 0 && len(msg.Grid) != len(gs.Grid) {
		log.Printf("tick grid dimension mismatch: expected %d rows, got %d", len(gs.Grid), len(msg.Grid))
		return
	}
	gs.Grid = msg.Grid
	gs.Tanks = msg.Tanks
	gs.Shots = msg.Shots
	gs.Mines = msg.Mines

	newTimers := make(map[[2]int]float64, len(gs.ExplosionTimers)+len(msg.Explosions))
	for pos, remaining := range gs.ExplosionTimers {
		remaining -= 1.0 / 60.0 // one frame at 60fps
		if remaining > 0 {
			newTimers[pos] = remaining
		}
	}
	// Add new explosions from server only if not already being tracked
	for _, e := range msg.Explosions {
		key := [2]int{e.X, e.Y}
		if _, exists := newTimers[key]; !exists {
			newTimers[key] = e.Duration
		}
	}
	gs.Explosions = msg.Explosions
	gs.ExplosionTimers = newTimers
	// Full keyframe: reset dirty tracking and desync counter
	gs.DirtyCells = nil
	gs.DesyncCount = 0
}

func (gs *GameState) ApplyTickDelta(msg TickDeltaMsg) {
	for _, cc := range msg.ChangedCells {
		if cc.Y >= 0 && cc.Y < len(gs.Grid) && cc.X >= 0 && cc.X < len(gs.Grid[cc.Y]) {
			gs.Grid[cc.Y][cc.X] = cc.Cell
		}
	}
	gs.Tanks = msg.Tanks
	gs.Shots = msg.Shots
	gs.Mines = msg.Mines

	newTimers := make(map[[2]int]float64, len(gs.ExplosionTimers)+len(msg.Explosions))
	for pos, remaining := range gs.ExplosionTimers {
		remaining -= 1.0 / 60.0
		if remaining > 0 {
			newTimers[pos] = remaining
		}
	}
	for _, e := range msg.Explosions {
		key := [2]int{e.X, e.Y}
		if _, exists := newTimers[key]; !exists {
			newTimers[key] = e.Duration
		}
	}
	gs.Explosions = msg.Explosions
	gs.ExplosionTimers = newTimers

	gs.DirtyCells = make(map[[2]int]bool, len(msg.ChangedCells))
	for _, cc := range msg.ChangedCells {
		gs.DirtyCells[[2]int{cc.X, cc.Y}] = true
	}

	gs.DesyncCount++
	if gs.DesyncCount > KeyframeInterval*2 {
		log.Printf("desync warning: no keyframe for %d ticks (delta only)", gs.DesyncCount)
	}
}

// ApplyRoundOver processes a round_over message
func (gs *GameState) ApplyRoundOver(msg RoundOverMsg) {
	gs.Phase = PhaseRoundOver
	gs.Winner = msg.Winner
	gs.Wins = msg.Wins
	gs.BattlesPlayed = msg.BattlesPlayed
}

// ApplyGameOver processes a game_over message
func (gs *GameState) ApplyGameOver(msg GameOverMsg) {
	gs.Phase = PhaseGameOver
	gs.Winner = msg.Winner
	gs.Wins = msg.FinalWins
	gs.BattlesPlayed = msg.TotalBattles
}

// ApplyRoomCreated stores the room code
func (gs *GameState) ApplyRoomCreated(msg RoomCreatedMsg) {
	gs.RoomCode = msg.RoomCode
}

// ApplyJoined stores player ID and opponent name
func (gs *GameState) ApplyJoined(msg JoinedMsg) {
	gs.PlayerID = msg.PlayerID
	gs.OpponentName = msg.OpponentName
	gs.RoomCode = msg.RoomCode
}

// ApplyError stores the latest error from the server
func (gs *GameState) ApplyError(msg ErrorMsg) {
	gs.Error = &msg
}

// Reset clears the game state for a new connection
func (gs *GameState) Reset() {
	*gs = *NewGameState()
}

func (gs *GameState) ApplyRematch(msg RematchMsg) {
	gs.OpponentWantsRematch = false
}

func keyToDir(key string) int {
	switch key {
	case "up":
		return 0
	case "up_right":
		return 1
	case "right":
		return 2
	case "down_right":
		return 3
	case "down":
		return 4
	case "down_left":
		return 5
	case "left":
		return 6
	case "up_left":
		return 7
	default:
		return -1
	}
}

func (gs *GameState) PredictBarrelDir(key string) {
	if gs.PlayerID < 1 || gs.PlayerID > 2 {
		return
	}
	dir := keyToDir(key)
	if dir < 0 {
		return
	}
	idx := gs.PlayerID - 1
	if gs.Tanks[idx].Active {
		gs.Tanks[idx].Dir = dir
		if gs.PredictedDir == nil {
			gs.PredictedDir = make(map[int]int)
		}
		gs.PredictedDir[gs.PlayerID] = dir
	}
}
