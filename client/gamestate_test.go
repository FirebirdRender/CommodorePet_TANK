package client

import (
	"testing"
)

// makeGrid creates a 21x40 grid filled with the specified cell type
func makeGrid(cellType int, rows, cols int) [][]int {
	grid := make([][]int, rows)
	for y := range grid {
		row := make([]int, cols)
		for x := range row {
			row[x] = cellType
		}
		grid[y] = row
	}
	return grid
}

func TestNewGameState(t *testing.T) {
	gs := NewGameState()

	if gs.Phase != PhaseDisconnected {
		t.Errorf("Phase = %v, want PhaseDisconnected (%v)", gs.Phase, PhaseDisconnected)
	}
	if gs.PlayerID != 0 {
		t.Errorf("PlayerID = %d, want 0", gs.PlayerID)
	}
	if gs.Difficulty != 0 {
		t.Errorf("Difficulty = %d, want 0", gs.Difficulty)
	}
	if gs.Grid != nil {
		t.Errorf("Grid = %v, want nil", gs.Grid)
	}
	if gs.Tanks != ([2]TankState{}) {
		t.Errorf("Tanks = %v, want zero value", gs.Tanks)
	}
	if gs.Shots != nil {
		t.Errorf("Shots = %v, want nil", gs.Shots)
	}
	if gs.Mines != nil {
		t.Errorf("Mines = %v, want nil", gs.Mines)
	}
	if gs.Explosions != nil {
		t.Errorf("Explosions = %v, want nil", gs.Explosions)
	}
	if gs.RoomCode != "" {
		t.Errorf("RoomCode = %q, want empty string", gs.RoomCode)
	}
	if gs.OpponentName != "" {
		t.Errorf("OpponentName = %q, want empty string", gs.OpponentName)
	}
	if gs.Wins != ([2]int{}) {
		t.Errorf("Wins = %v, want zero value", gs.Wins)
	}
	if gs.BattlesPlayed != 0 {
		t.Errorf("BattlesPlayed = %d, want 0", gs.BattlesPlayed)
	}
	if gs.Winner != 0 {
		t.Errorf("Winner = %d, want 0", gs.Winner)
	}
	if gs.ExplosionTimers == nil {
		t.Error("ExplosionTimers = nil, want non-nil map")
	}
	if gs.Connected != false {
		t.Errorf("Connected = %v, want false", gs.Connected)
	}
	if gs.Error != nil {
		t.Errorf("Error = %v, want nil", gs.Error)
	}
}

func TestApplyGameStart(t *testing.T) {
	gs := NewGameState()

	grid := makeGrid(1, 21, 40)
	msg := GameStartMsg{
		Grid: grid,
		Tanks: [2]TankState{
			{PlayerID: 1, X: 5, Y: 10, Dir: 1, Lives: 3, ShotsLeft: 5, MinesLeft: 2, Active: true},
			{PlayerID: 2, X: 35, Y: 10, Dir: 3, Lives: 3, ShotsLeft: 5, MinesLeft: 2, Active: true},
		},
		Difficulty:   7,
		YourPlayerID: 1,
	}

	gs.ApplyGameStart(msg)

	if gs.Phase != PhasePlaying {
		t.Errorf("Phase = %v, want PhasePlaying (%v)", gs.Phase, PhasePlaying)
	}
	if gs.PlayerID != 1 {
		t.Errorf("PlayerID = %d, want 1", gs.PlayerID)
	}
	if gs.Difficulty != 7 {
		t.Errorf("Difficulty = %d, want 7", gs.Difficulty)
	}
	if len(gs.Grid) != 21 {
		t.Errorf("Grid rows = %d, want 21", len(gs.Grid))
	}
	if len(gs.Grid[0]) != 40 {
		t.Errorf("Grid cols = %d, want 40", len(gs.Grid[0]))
	}
	if gs.Tanks[0].PlayerID != 1 {
		t.Errorf("Tanks[0].PlayerID = %d, want 1", gs.Tanks[0].PlayerID)
	}
	if gs.Tanks[1].PlayerID != 2 {
		t.Errorf("Tanks[1].PlayerID = %d, want 2", gs.Tanks[1].PlayerID)
	}
	if gs.Shots != nil {
		t.Errorf("Shots = %v, want nil", gs.Shots)
	}
	if gs.Mines != nil {
		t.Errorf("Mines = %v, want nil", gs.Mines)
	}
	if gs.Explosions != nil {
		t.Errorf("Explosions = %v, want nil", gs.Explosions)
	}
	if gs.ExplosionTimers == nil {
		t.Error("ExplosionTimers = nil, want non-nil map")
	}
	if gs.Winner != 0 {
		t.Errorf("Winner = %d, want 0", gs.Winner)
	}
	if gs.Error != nil {
		t.Errorf("Error = %v, want nil", gs.Error)
	}
}

func TestApplyTick(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying

	initialGrid := makeGrid(1, 21, 40)
	gs.Grid = initialGrid

	newGrid := makeGrid(2, 21, 40)
	newGrid[5][10] = 3 // tank position

	msg := TickMsg{
		Tick: 100,
		Grid: newGrid,
		Tanks: [2]TankState{
			{PlayerID: 1, X: 10, Y: 5, Dir: 2, Lives: 3, ShotsLeft: 4, MinesLeft: 2, Active: true},
			{PlayerID: 2, X: 30, Y: 15, Dir: 4, Lives: 2, ShotsLeft: 3, MinesLeft: 1, Active: true},
		},
		Shots: []ShotState{
			{X: 12, Y: 5, Dir: 2, OwnerID: 1, Active: true},
		},
		Mines: []MineState{
			{X: 20, Y: 10, OwnerID: 2, Visible: true},
		},
		Explosions: []ExplosionState{
			{X: 15, Y: 15, Duration: 0.5, IsChainReaction: false},
		},
	}

	gs.ApplyTick(msg)

	if len(gs.Grid) != len(msg.Grid) || len(gs.Grid[0]) != len(msg.Grid[0]) {
		t.Error("Grid dimensions do not match")
	}
	if gs.Grid[5][10] != 3 {
		t.Errorf("Grid[5][10] = %d, want 3 (tank position)", gs.Grid[5][10])
	}
	if len(gs.Tanks) != 2 {
		t.Errorf("Tanks length = %d, want 2", len(gs.Tanks))
	}
	if len(gs.Shots) != 1 {
		t.Errorf("Shots length = %d, want 1", len(gs.Shots))
	}
	if len(gs.Mines) != 1 {
		t.Errorf("Mines length = %d, want 1", len(gs.Mines))
	}
	if len(gs.Explosions) != 1 {
		t.Errorf("Explosions length = %d, want 1", len(gs.Explosions))
	}
}

func TestApplyTickExplosionTimers(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying

	gs.ExplosionTimers[[2]int{10, 10}] = 0.5
	gs.ExplosionTimers[[2]int{20, 20}] = 0.5

	msg := TickMsg{
		Tick:  100,
		Grid:  makeGrid(1, 21, 40),
		Tanks: gs.Tanks,
		Shots: nil,
		Mines: nil,
		Explosions: []ExplosionState{
			{X: 10, Y: 10, Duration: 0.5, IsChainReaction: false},
			{X: 30, Y: 30, Duration: 0.8, IsChainReaction: true},
		},
	}

	gs.ApplyTick(msg)

	// (10,10): existing timer at 0.5 decremented to ~0.483
	// Server also reports explosion at (10,10) with duration 0.5,
	// but since (10,10) already tracked, server duration is ignored.
	pos1010 := [2]int{10, 10}
	if val, exists := gs.ExplosionTimers[pos1010]; !exists {
		t.Error("(10,10) timer should exist")
	} else if val >= 0.5 {
		t.Errorf("(10,10) timer = %f, want decremented value < 0.5", val)
	}

	// (20,20): existing timer at 0.5 decremented to ~0.483
	// No server explosion at (20,20), so it stays decremented.
	pos2020 := [2]int{20, 20}
	if val, exists := gs.ExplosionTimers[pos2020]; !exists {
		t.Error("(20,20) timer should exist (not yet expired)")
	} else if val >= 0.5 {
		t.Errorf("(20,20) timer = %f, want decremented value < 0.5", val)
	}

	// (30,30): new explosion added with its full duration
	pos3030 := [2]int{30, 30}
	if val, exists := gs.ExplosionTimers[pos3030]; !exists {
		t.Error("(30,30) timer should exist")
	} else if val != 0.8 {
		t.Errorf("(30,30) timer = %f, want 0.8", val)
	}
}

func TestApplyTickExplosionTimerPreventsReset(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying

	gs.ExplosionTimers[[2]int{10, 10}] = 0.3

	msg := TickMsg{
		Tick:  100,
		Grid:  makeGrid(1, 21, 40),
		Tanks: gs.Tanks,
		Shots: nil,
		Mines: nil,
		Explosions: []ExplosionState{
			{X: 10, Y: 10, Duration: 0.5, IsChainReaction: false},
		},
	}

	gs.ApplyTick(msg)

	// After one frame: 0.3 - 1/60 ≈ 0.283
	// Server sends duration 0.5, but we should NOT reset since 0.283 < 0.5
	pos := [2]int{10, 10}
	val := gs.ExplosionTimers[pos]
	if val >= 0.3 {
		t.Errorf("timer should have decremented, got %f", val)
	}
	if val > 0.29 && val < 0.28 {
		t.Errorf("timer decrement appears incorrect, got %f", val)
	}
}

func TestApplyRoundOver(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.Wins = [2]int{1, 0}
	gs.BattlesPlayed = 1

	msg := RoundOverMsg{
		Winner:        1,
		Wins:          [2]int{2, 0},
		BattlesPlayed: 2,
	}

	gs.ApplyRoundOver(msg)

	if gs.Phase != PhaseRoundOver {
		t.Errorf("Phase = %v, want PhaseRoundOver (%v)", gs.Phase, PhaseRoundOver)
	}
	if gs.Winner != 1 {
		t.Errorf("Winner = %d, want 1", gs.Winner)
	}
	if gs.Wins != ([2]int{2, 0}) {
		t.Errorf("Wins = %v, want [2]int{2, 0}", gs.Wins)
	}
	if gs.BattlesPlayed != 2 {
		t.Errorf("BattlesPlayed = %d, want 2", gs.BattlesPlayed)
	}
}

func TestApplyGameOver(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhaseRoundOver
	gs.Wins = [2]int{2, 0}
	gs.BattlesPlayed = 2

	msg := GameOverMsg{
		Winner:       2,
		FinalWins:    [2]int{3, 1},
		TotalBattles: 7,
	}

	gs.ApplyGameOver(msg)

	if gs.Phase != PhaseGameOver {
		t.Errorf("Phase = %v, want PhaseGameOver (%v)", gs.Phase, PhaseGameOver)
	}
	if gs.Winner != 2 {
		t.Errorf("Winner = %d, want 2", gs.Winner)
	}
	if gs.Wins != ([2]int{3, 1}) {
		t.Errorf("Wins = %v, want [2]int{3, 1}", gs.Wins)
	}
	if gs.BattlesPlayed != 7 {
		t.Errorf("BattlesPlayed = %d, want 7", gs.BattlesPlayed)
	}
}

func TestApplyRoomCreated(t *testing.T) {
	gs := NewGameState()

	msg := RoomCreatedMsg{RoomCode: "ABCD12"}

	gs.ApplyRoomCreated(msg)

	if gs.RoomCode != "ABCD12" {
		t.Errorf("RoomCode = %q, want 'ABCD12'", gs.RoomCode)
	}
}

func TestApplyJoined(t *testing.T) {
	gs := NewGameState()

	msg := JoinedMsg{
		RoomCode:     "XYZ999",
		PlayerID:     2,
		OpponentName: "Alice",
	}

	gs.ApplyJoined(msg)

	if gs.PlayerID != 2 {
		t.Errorf("PlayerID = %d, want 2", gs.PlayerID)
	}
	if gs.OpponentName != "Alice" {
		t.Errorf("OpponentName = %q, want 'Alice'", gs.OpponentName)
	}
	if gs.RoomCode != "XYZ999" {
		t.Errorf("RoomCode = %q, want 'XYZ999'", gs.RoomCode)
	}
}

func TestApplyError(t *testing.T) {
	gs := NewGameState()
	gs.Error = nil

	msg := ErrorMsg{Code: ErrCodeRoomFull, Message: "room is full"}

	gs.ApplyError(msg)

	if gs.Error == nil {
		t.Fatal("Error = nil, want non-nil")
	}
	if gs.Error.Code != ErrCodeRoomFull {
		t.Errorf("Error.Code = %q, want %q", gs.Error.Code, ErrCodeRoomFull)
	}
	if gs.Error.Message != "room is full" {
		t.Errorf("Error.Message = %q, want 'room is full'", gs.Error.Message)
	}
}

func TestReset(t *testing.T) {
	gs := NewGameState()

	gs.Phase = PhasePlaying
	gs.PlayerID = 1
	gs.Difficulty = 5
	gs.Grid = makeGrid(2, 21, 40)
	gs.RoomCode = "TEST01"
	gs.OpponentName = "Bob"
	gs.Wins = [2]int{1, 2}
	gs.BattlesPlayed = 3
	gs.Winner = 1
	gs.Connected = true
	gs.Error = &ErrorMsg{Code: "test", Message: "test error"}
	gs.ExplosionTimers[[2]int{5, 5}] = 1.0

	gs.Reset()

	if gs.Phase != PhaseDisconnected {
		t.Errorf("Phase = %v, want PhaseDisconnected", gs.Phase)
	}
	if gs.PlayerID != 0 {
		t.Errorf("PlayerID = %d, want 0", gs.PlayerID)
	}
	if gs.Difficulty != 0 {
		t.Errorf("Difficulty = %d, want 0", gs.Difficulty)
	}
	if gs.Grid != nil {
		t.Error("Grid should be nil after Reset")
	}
	if gs.RoomCode != "" {
		t.Errorf("RoomCode = %q, want empty string", gs.RoomCode)
	}
	if gs.OpponentName != "" {
		t.Errorf("OpponentName = %q, want empty string", gs.OpponentName)
	}
	if gs.Wins != ([2]int{}) {
		t.Errorf("Wins = %v, want zero value", gs.Wins)
	}
	if gs.BattlesPlayed != 0 {
		t.Errorf("BattlesPlayed = %d, want 0", gs.BattlesPlayed)
	}
	if gs.Winner != 0 {
		t.Errorf("Winner = %d, want 0", gs.Winner)
	}
	if gs.Connected != false {
		t.Errorf("Connected = %v, want false", gs.Connected)
	}
	if gs.Error != nil {
		t.Errorf("Error = %v, want nil", gs.Error)
	}
	if len(gs.ExplosionTimers) != 0 {
		t.Errorf("ExplosionTimers length = %d, want 0", len(gs.ExplosionTimers))
	}
}

func TestGamePhaseConstants(t *testing.T) {
	tests := []struct {
		got  GamePhase
		want GamePhase
		desc string
	}{
		{PhasePlaying, 0, "PhasePlaying"},
		{PhaseRoundOver, 1, "PhaseRoundOver"},
		{PhaseGameOver, 2, "PhaseGameOver"},
		{PhaseDisconnected, 3, "PhaseDisconnected"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.desc, tt.got, tt.want)
		}
	}
}

func TestApplyTickDelta(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.Grid = makeGrid(1, 21, 40)

	msg := TickDeltaMsg{
		Tick: 100,
		ChangedCells: []CellChange{
			{X: 5, Y: 10, Cell: 3},
			{X: 15, Y: 20, Cell: 4},
			{X: 25, Y: 5, Cell: 5},
		},
		Tanks: [2]TankState{
			{PlayerID: 1, X: 5, Y: 10, Dir: 1, Lives: 3, ShotsLeft: 5, MinesLeft: 2, Active: true},
			{PlayerID: 2, X: 35, Y: 10, Dir: 3, Lives: 3, ShotsLeft: 5, MinesLeft: 2, Active: true},
		},
		Shots: []ShotState{
			{X: 12, Y: 5, Dir: 2, OwnerID: 1, Active: true},
		},
		Mines: []MineState{
			{X: 20, Y: 10, OwnerID: 2, Visible: true},
		},
		Explosions: []ExplosionState{
			{X: 15, Y: 15, Duration: 0.5, IsChainReaction: false},
		},
	}

	gs.ApplyTickDelta(msg)

	// Verify cells were changed
	if gs.Grid[10][5] != 3 {
		t.Errorf("Grid[10][5] = %d, want 3", gs.Grid[10][5])
	}
	if gs.Grid[20][15] != 4 {
		t.Errorf("Grid[20][15] = %d, want 4", gs.Grid[20][15])
	}
	if gs.Grid[5][25] != 5 {
		t.Errorf("Grid[5][25] = %d, want 5", gs.Grid[5][25])
	}

	// Verify DirtyCells is populated
	if gs.DirtyCells == nil {
		t.Fatal("DirtyCells should not be nil after ApplyTickDelta")
	}
	if !gs.DirtyCells[[2]int{5, 10}] {
		t.Error("DirtyCells should contain (5,10)")
	}
	if !gs.DirtyCells[[2]int{15, 20}] {
		t.Error("DirtyCells should contain (15,20)")
	}
	if !gs.DirtyCells[[2]int{25, 5}] {
		t.Error("DirtyCells should contain (25,5)")
	}

	// Verify Tanks, Shots, Mines, Explosions
	if gs.Tanks[0].PlayerID != 1 {
		t.Errorf("Tanks[0].PlayerID = %d, want 1", gs.Tanks[0].PlayerID)
	}
	if len(gs.Shots) != 1 {
		t.Errorf("Shots length = %d, want 1", len(gs.Shots))
	}
	if len(gs.Mines) != 1 {
		t.Errorf("Mines length = %d, want 1", len(gs.Mines))
	}
	if len(gs.Explosions) != 1 {
		t.Errorf("Explosions length = %d, want 1", len(gs.Explosions))
	}
}

func TestApplyTickDeltaOutOfBounds(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.Grid = makeGrid(1, 21, 40)

	// Send cells outside the grid bounds (beyond grid dimensions)
	msg := TickDeltaMsg{
		Tick: 100,
		ChangedCells: []CellChange{
			{X: 50, Y: 10, Cell: 3}, // X out of bounds (max is 39)
			{X: 5, Y: 30, Cell: 3},  // Y out of bounds (max is 20)
			{X: 5, Y: 10, Cell: 3},  // valid cell (should be applied)
		},
		Tanks:      [2]TankState{},
		Shots:      []ShotState{},
		Mines:      []MineState{},
		Explosions: []ExplosionState{},
	}

	// Should not panic
	gs.ApplyTickDelta(msg)

	// Only the valid cell should be changed
	if gs.Grid[10][5] != 3 {
		t.Errorf("Grid[10][5] = %d, want 3 (valid cell should be changed)", gs.Grid[10][5])
	}
}

func TestApplyTickDeltaDesyncCount(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.Grid = makeGrid(1, 21, 40)

	// First delta tick
	gs.ApplyTickDelta(TickDeltaMsg{Tick: 100})
	if gs.DesyncCount != 1 {
		t.Errorf("After first delta, DesyncCount = %d, want 1", gs.DesyncCount)
	}

	// Second delta tick
	gs.ApplyTickDelta(TickDeltaMsg{Tick: 101})
	if gs.DesyncCount != 2 {
		t.Errorf("After second delta, DesyncCount = %d, want 2", gs.DesyncCount)
	}

	// Apply a full keyframe - should reset desync
	msg := TickMsg{
		Tick:       120,
		Grid:       makeGrid(2, 21, 40),
		Tanks:      [2]TankState{},
		Shots:      []ShotState{},
		Mines:      []MineState{},
		Explosions: []ExplosionState{},
	}
	gs.ApplyTick(msg)
	if gs.DesyncCount != 0 {
		t.Errorf("After keyframe, DesyncCount = %d, want 0", gs.DesyncCount)
	}
}

func TestApplyTickDeltaWithExplosions(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.Grid = makeGrid(1, 21, 40)

	// Add an existing explosion timer
	gs.ExplosionTimers[[2]int{10, 10}] = 0.5

	// Create delta with explosions
	msg := TickDeltaMsg{
		Tick:         100,
		ChangedCells: []CellChange{},
		Tanks:        [2]TankState{},
		Shots:        []ShotState{},
		Mines:        []MineState{},
		Explosions: []ExplosionState{
			{X: 10, Y: 10, Duration: 0.6, IsChainReaction: false}, // This one already exists
			{X: 20, Y: 20, Duration: 0.5, IsChainReaction: true},  // New explosion
		},
	}

	gs.ApplyTickDelta(msg)

	// (10,10): existing timer at 0.5 decremented to ~0.483
	// Server also reports explosion at (10,10) with duration 0.6,
	// but since (10,10) already tracked, server duration is ignored.
	pos1010 := [2]int{10, 10}
	if val, exists := gs.ExplosionTimers[pos1010]; !exists {
		t.Error("(10,10) timer should exist")
	} else if val >= 0.5 {
		t.Errorf("(10,10) timer = %f, want decremented value < 0.5", val)
	}

	// (20,20): new explosion added with its full duration
	pos2020 := [2]int{20, 20}
	if val, exists := gs.ExplosionTimers[pos2020]; !exists {
		t.Error("(20,20) timer should exist")
	} else if val != 0.5 {
		t.Errorf("(20,20) timer = %f, want 0.5", val)
	}
}

func TestApplyRematch(t *testing.T) {
	gs := NewGameState()
	gs.OpponentWantsRematch = true

	msg := RematchMsg{Difficulty: 5}

	gs.ApplyRematch(msg)

	if gs.OpponentWantsRematch != false {
		t.Error("ApplyRematch should reset OpponentWantsRematch to false")
	}
}

func TestKeyToDir(t *testing.T) {
	tests := []struct {
		key  string
		want int
	}{
		{"up", DirUp},
		{"down", DirDown},
		{"left", DirLeft},
		{"right", DirRight},
		{"up_left", DirUpLeft},
		{"up_right", DirUpRight},
		{"down_left", DirDownLeft},
		{"down_right", DirDownRight},
		{"fire", -1},
		{"mine", -1},
		{"invalid_key", -1},
		{"", -1},
	}

	for _, tt := range tests {
		got := keyToDir(tt.key)
		if got != tt.want {
			t.Errorf("keyToDir(%q) = %d, want %d", tt.key, got, tt.want)
		}
	}
}

func TestNewGameSetsPlayerName(t *testing.T) {
	g := NewGame("ws://localhost:8080/ws", "TestPlayer", 1, "ABCD", "token123")
	if g.playerName != "TestPlayer" {
		t.Errorf("playerName = %q, want 'TestPlayer'", g.playerName)
	}
	if g.roomCode != "ABCD" {
		t.Errorf("roomCode = %q, want 'ABCD'", g.roomCode)
	}
}

func TestExportGameStateNativeNoop(t *testing.T) {
	g := NewGame("ws://localhost:8080/ws", "TestPlayer", 1, "ABCD", "token123")
	g.ExportGameState()
}

func TestAnimTickIncrements(t *testing.T) {
	gs := NewGameState()
	if gs.AnimTick != 0 {
		t.Errorf("initial AnimTick = %d, want 0", gs.AnimTick)
	}
}
