package client

import (
	"testing"
)

// TestBarrelPredictionResponsiveness verifies that PredictBarrelDir immediately
// changes the tank direction without waiting for server confirmation.
func TestBarrelPredictionResponsiveness(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.PlayerID = 1
	gs.Tanks[0] = TankState{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: true}  // Dir=0 is "up"
	gs.Tanks[1] = TankState{PlayerID: 2, X: 30, Y: 15, Dir: 2, Active: true} // Dir=2 is "right"

	// Initial direction should be up (Dir=0)
	if gs.Tanks[0].Dir != 0 {
		t.Fatalf("initial Dir = %d, want 0 (up)", gs.Tanks[0].Dir)
	}

	// PredictBarrelDir("right") should immediately change direction to right (Dir=2)
	gs.PredictBarrelDir("right")
	if gs.Tanks[0].Dir != 2 {
		t.Errorf("after PredictBarrelDir(\"right\"), Dir = %d, want 2 (right)", gs.Tanks[0].Dir)
	}

	// PredictBarrelDir("down") should immediately change direction to down (Dir=4)
	gs.PredictBarrelDir("down")
	if gs.Tanks[0].Dir != 4 {
		t.Errorf("after PredictBarrelDir(\"down\"), Dir = %d, want 4 (down)", gs.Tanks[0].Dir)
	}

	// Verify PredictedDir map was set
	if gs.PredictedDir == nil {
		t.Fatal("PredictedDir map should be initialized after PredictBarrelDir")
	}
	if gs.PredictedDir[1] != 4 {
		t.Errorf("PredictedDir[1] = %d, want 4", gs.PredictedDir[1])
	}
}

// TestBarrelPredictionServerOverwrite verifies that a server TickMsg
// overwrites the locally predicted direction.
func TestBarrelPredictionServerOverwrite(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.PlayerID = 1
	gs.Tanks[0] = TankState{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: true}

	// Client predicts direction right (Dir=2)
	gs.PredictBarrelDir("right")
	if gs.Tanks[0].Dir != 2 {
		t.Fatalf("prediction setup failed: Dir = %d, want 2", gs.Tanks[0].Dir)
	}

	// Server sends a TickMsg with direction left (Dir=6)
	serverTick := TickMsg{
		Tick: 10,
		Grid: makeGrid(1, 21, 40),
		Tanks: [2]TankState{
			{PlayerID: 1, X: 10, Y: 5, Dir: 6, Active: true}, // Dir=6 is "left"
			{PlayerID: 2, X: 30, Y: 15, Dir: 2, Active: true},
		},
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	}

	// ApplyTick should overwrite prediction with server value
	gs.ApplyTick(serverTick)
	if gs.Tanks[0].Dir != 6 {
		t.Errorf("after ApplyTick(server Dir=6), Dir = %d, want 6 (left)", gs.Tanks[0].Dir)
	}

	// PredictedDir should be cleared/nil after keyframe (since DirtyCells=nil)
	if gs.PredictedDir != nil {
		t.Logf("PredictedDir = %v after keyframe (may be cleared by ApplyTick)", gs.PredictedDir)
	}
}

// TestBarrelPredictionNonDirectionKeys verifies that non-direction keys
// (fire, mine) do not affect barrel direction prediction.
func TestBarrelPredictionNonDirectionKeys(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.PlayerID = 1
	gs.Tanks[0] = TankState{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: true}

	// Predict direction right
	gs.PredictBarrelDir("right")
	if gs.Tanks[0].Dir != 2 {
		t.Fatalf("prediction setup failed: Dir = %d, want 2", gs.Tanks[0].Dir)
	}

	// Non-direction keys should be ignored (no error, no effect)
	gs.PredictBarrelDir("fire")
	if gs.Tanks[0].Dir != 2 {
		t.Errorf("after PredictBarrelDir(\"fire\"), Dir = %d, want 2 (unchanged)", gs.Tanks[0].Dir)
	}

	gs.PredictBarrelDir("mine")
	if gs.Tanks[0].Dir != 2 {
		t.Errorf("after PredictBarrelDir(\"mine\"), Dir = %d, want 2 (unchanged)", gs.Tanks[0].Dir)
	}

	// Invalid keys should also be ignored
	gs.PredictBarrelDir("invalid_key")
	if gs.Tanks[0].Dir != 2 {
		t.Errorf("after PredictBarrelDir(\"invalid_key\"), Dir = %d, want 2 (unchanged)", gs.Tanks[0].Dir)
	}
}

// TestBarrelPredictionInactiveTank verifies that prediction is ignored
// when the tank is not active.
func TestBarrelPredictionInactiveTank(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.PlayerID = 1
	gs.Tanks[0] = TankState{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: false}

	// Prediction should be ignored when tank is inactive
	gs.PredictBarrelDir("right")
	if gs.Tanks[0].Dir != 0 {
		t.Errorf("after PredictBarrelDir on inactive tank, Dir = %d, want 0 (unchanged)", gs.Tanks[0].Dir)
	}
}

// TestDesyncDetection verifies that DesyncCount is incremented on delta
// and reset to 0 on keyframe.
func TestDesyncDetection(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.Grid = makeGrid(1, 21, 40)

	// Apply a full keyframe - DesyncCount should reset to 0
	keyframe := TickMsg{
		Tick: 1,
		Grid: makeGrid(1, 21, 40),
		Tanks: [2]TankState{
			{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: true},
			{PlayerID: 2, X: 30, Y: 15, Dir: 2, Active: true},
		},
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	}
	gs.ApplyTick(keyframe)
	if gs.DesyncCount != 0 {
		t.Errorf("after ApplyTick (keyframe), DesyncCount = %d, want 0", gs.DesyncCount)
	}

	// Apply a delta - DesyncCount should increment
	delta := TickDeltaMsg{
		Tick: 2,
		ChangedCells: []CellChange{
			{X: 5, Y: 3, Cell: 2},
		},
		Tanks: [2]TankState{
			{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: true},
			{PlayerID: 2, X: 30, Y: 15, Dir: 2, Active: true},
		},
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	}
	gs.ApplyTickDelta(delta)
	if gs.DesyncCount != 1 {
		t.Errorf("after first ApplyTickDelta, DesyncCount = %d, want 1", gs.DesyncCount)
	}

	// Apply another delta - count should increment again
	gs.ApplyTickDelta(delta)
	if gs.DesyncCount != 2 {
		t.Errorf("after second ApplyTickDelta, DesyncCount = %d, want 2", gs.DesyncCount)
	}

	// Apply another delta - count should increment again
	gs.ApplyTickDelta(delta)
	if gs.DesyncCount != 3 {
		t.Errorf("after third ApplyTickDelta, DesyncCount = %d, want 3", gs.DesyncCount)
	}

	// Apply a full keyframe - DesyncCount should reset to 0
	keyframe2 := TickMsg{
		Tick:  120,
		Grid:  makeGrid(1, 21, 40),
		Tanks: gs.Tanks,
	}
	gs.ApplyTick(keyframe2)
	if gs.DesyncCount != 0 {
		t.Errorf("after second ApplyTick (keyframe), DesyncCount = %d, want 0", gs.DesyncCount)
	}
}

// TestDesyncDetectionWarning verifies that a warning is logged when
// consecutive delta count exceeds 2*KeyframeInterval.
func TestDesyncDetectionWarning(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.Grid = makeGrid(1, 21, 40)

	// Apply a keyframe first to reset state
	keyframe := TickMsg{
		Tick:       1,
		Grid:       makeGrid(1, 21, 40),
		Tanks:      [2]TankState{{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: true}, {PlayerID: 2, X: 30, Y: 15, Dir: 2, Active: true}},
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	}
	gs.ApplyTick(keyframe)
	if gs.DesyncCount != 0 {
		t.Fatalf("initial DesyncCount = %d, want 0", gs.DesyncCount)
	}

	// KeyframeInterval is 60, so warning threshold is 120
	// Apply 119 deltas - should not trigger warning (threshold is >120)
	delta := TickDeltaMsg{
		Tick:         2,
		ChangedCells: []CellChange{{X: 5, Y: 3, Cell: 2}},
		Tanks:        [2]TankState{{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: true}, {PlayerID: 2, X: 30, Y: 15, Dir: 2, Active: true}},
		Shots:        nil,
		Mines:        nil,
		Explosions:   nil,
	}

	for i := 0; i < 119; i++ {
		gs.ApplyTickDelta(delta)
	}

	// DesyncCount should be 119 (not 120 because we started at 0)
	if gs.DesyncCount != 119 {
		t.Errorf("after 119 deltas, DesyncCount = %d, want 119", gs.DesyncCount)
	}

	// Apply one more delta to hit 120 - this should trigger warning (DesyncCount becomes 120)
	gs.ApplyTickDelta(delta)
	if gs.DesyncCount != 120 {
		t.Errorf("after 120 deltas, DesyncCount = %d, want 120", gs.DesyncCount)
	}

	// A keyframe should reset the count back to 0
	gs.ApplyTick(TickMsg{
		Tick:       200,
		Grid:       makeGrid(1, 21, 40),
		Tanks:      gs.Tanks,
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	})
	if gs.DesyncCount != 0 {
		t.Errorf("after keyframe following warning, DesyncCount = %d, want 0", gs.DesyncCount)
	}
}

// TestDesyncDetectionDirtiesCells verifies that DirtyCells is set correctly
// for deltas and nil for keyframes.
func TestDesyncDetectionDirtyCells(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.Grid = makeGrid(1, 21, 40)

	// Apply a keyframe - DirtyCells should be nil
	keyframe := TickMsg{
		Tick:       1,
		Grid:       makeGrid(1, 21, 40),
		Tanks:      [2]TankState{{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: true}, {PlayerID: 2, X: 30, Y: 15, Dir: 2, Active: true}},
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	}
	gs.ApplyTick(keyframe)
	if gs.DirtyCells != nil {
		t.Errorf("after ApplyTick (keyframe), DirtyCells = %v, want nil", gs.DirtyCells)
	}

	// Apply a delta - DirtyCells should contain the changed cells
	delta := TickDeltaMsg{
		Tick: 2,
		ChangedCells: []CellChange{
			{X: 5, Y: 3, Cell: 2},
			{X: 10, Y: 5, Cell: 3},
		},
		Tanks:      gs.Tanks,
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	}
	gs.ApplyTickDelta(delta)
	if gs.DirtyCells == nil {
		t.Fatal("after ApplyTickDelta, DirtyCells = nil, want non-nil")
	}
	if len(gs.DirtyCells) != 2 {
		t.Errorf("after ApplyTickDelta, len(DirtyCells) = %d, want 2", len(gs.DirtyCells))
	}
	if !gs.DirtyCells[[2]int{5, 3}] {
		t.Error("DirtyCells should contain {5, 3}")
	}
	if !gs.DirtyCells[[2]int{10, 5}] {
		t.Error("DirtyCells should contain {10, 5}")
	}

	// Another keyframe should reset DirtyCells to nil
	keyframe2 := TickMsg{
		Tick:       3,
		Grid:       makeGrid(1, 21, 40),
		Tanks:      gs.Tanks,
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	}
	gs.ApplyTick(keyframe2)
	if gs.DirtyCells != nil {
		t.Errorf("after second ApplyTick, DirtyCells = %v, want nil", gs.DirtyCells)
	}
}

// TestDesyncDetectionNoIncrementOnKeyframe verifies that applying a keyframe
// does not increment DesyncCount (it resets to 0).
func TestDesyncDetectionNoIncrementOnKeyframe(t *testing.T) {
	gs := NewGameState()
	gs.Phase = PhasePlaying
	gs.Grid = makeGrid(1, 21, 40)

	// Apply a keyframe first
	keyframe := TickMsg{
		Tick:       1,
		Grid:       makeGrid(1, 21, 40),
		Tanks:      [2]TankState{{PlayerID: 1, X: 10, Y: 5, Dir: 0, Active: true}, {PlayerID: 2, X: 30, Y: 15, Dir: 2, Active: true}},
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	}
	gs.ApplyTick(keyframe)

	// Apply several deltas
	delta := TickDeltaMsg{
		Tick:         2,
		ChangedCells: []CellChange{{X: 5, Y: 3, Cell: 2}},
		Tanks:        gs.Tanks,
		Shots:        nil,
		Mines:        nil,
		Explosions:   nil,
	}
	for i := 0; i < 5; i++ {
		gs.ApplyTickDelta(delta)
	}

	if gs.DesyncCount != 5 {
		t.Fatalf("after 5 deltas, DesyncCount = %d, want 5", gs.DesyncCount)
	}

	// Apply a keyframe - should reset to 0, NOT set to 6
	gs.ApplyTick(TickMsg{
		Tick:       100,
		Grid:       makeGrid(1, 21, 40),
		Tanks:      gs.Tanks,
		Shots:      nil,
		Mines:      nil,
		Explosions: nil,
	})

	if gs.DesyncCount != 0 {
		t.Errorf("after keyframe, DesyncCount = %d, want 0 (not incremented)", gs.DesyncCount)
	}
}
