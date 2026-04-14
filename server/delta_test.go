package server

import (
	"strconv"
	"testing"
)

func makeTestGrid(fill int) [][]int {
	grid := make([][]int, 21)
	for y := range grid {
		grid[y] = make([]int, 40)
		for x := range grid[y] {
			grid[y][x] = fill
		}
	}
	return grid
}

func TestNewDeltaTracker(t *testing.T) {
	dt := NewDeltaTracker()
	if dt == nil {
		t.Fatal("NewDeltaTracker returned nil")
	}
	if dt.prevGrid != nil {
		t.Errorf("expected nil prevGrid, got %v", dt.prevGrid)
	}
}

func TestComputeDeltaNilPrevious(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	delta := dt.ComputeDelta(grid)
	if delta != nil {
		t.Errorf("expected nil delta on first call, got %v", delta)
	}
}

func TestComputeDeltaNoChanges(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	dt.UpdateState(grid)
	delta := dt.ComputeDelta(grid)
	if len(delta) != 0 {
		t.Errorf("expected empty delta after identical grid, got %d changes", len(delta))
	}
}

func TestComputeDeltaSingleChange(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	dt.UpdateState(grid)
	grid[5][10] = 2
	delta := dt.ComputeDelta(grid)
	if len(delta) != 1 {
		t.Fatalf("expected 1 change, got %d", len(delta))
	}
	if delta[0].X != 10 || delta[0].Y != 5 || delta[0].Cell != 2 {
		t.Errorf("unexpected CellChange: X=%d, Y=%d, Cell=%d", delta[0].X, delta[0].Y, delta[0].Cell)
	}
}

func TestComputeDeltaMultipleChanges(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	dt.UpdateState(grid)
	grid[0][0] = 2
	grid[0][1] = 3
	grid[10][20] = 4
	grid[20][39] = 5
	grid[5][15] = 6
	delta := dt.ComputeDelta(grid)
	if len(delta) != 5 {
		t.Fatalf("expected 5 changes, got %d", len(delta))
	}
	// grid[y][x] = val → key "x,y" → val
	expected := map[string]int{
		"0,0":   2,
		"1,0":   3,
		"20,10": 4,
		"39,20": 5,
		"15,5":  6,
	}
	for _, c := range delta {
		key := strconv.Itoa(c.X) + "," + strconv.Itoa(c.Y)
		if v, ok := expected[key]; !ok || v != c.Cell {
			t.Errorf("unexpected change at (%d,%d): expected cell=%d, got %d", c.X, c.Y, v, c.Cell)
		}
	}
}

func TestComputeDeltaEntireRowChange(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	dt.UpdateState(grid)
	for x := range grid[10] {
		grid[10][x] = 2
	}
	delta := dt.ComputeDelta(grid)
	if len(delta) != 40 {
		t.Fatalf("expected 40 changes for entire row, got %d", len(delta))
	}
	for _, c := range delta {
		if c.Y != 10 {
			t.Errorf("expected Y=10 for row change, got Y=%d", c.Y)
		}
	}
}

func TestUpdateStatePreservesGrid(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	grid[5][10] = 5
	grid[15][25] = 7
	dt.UpdateState(grid)
	for y := range dt.prevGrid {
		for x := range dt.prevGrid[y] {
			if dt.prevGrid[y][x] != grid[y][x] {
				t.Errorf("prevGrid[%d][%d] = %d, expected %d", y, x, dt.prevGrid[y][x], grid[y][x])
			}
		}
	}
}

func TestShouldSendKeyframeFirstTick(t *testing.T) {
	dt := NewDeltaTracker()
	if !dt.ShouldSendKeyframe(1, 0) {
		t.Error("first tick after reset should be keyframe")
	}
}

func TestShouldSendKeyframePeriodic(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	dt.UpdateState(grid)
	for _, tick := range []uint64{60, 120, 180} {
		if !dt.ShouldSendKeyframe(tick, 10) {
			t.Errorf("tick %d should be keyframe", tick)
		}
	}
}

func TestShouldSendKeyframeLargeDelta(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	dt.UpdateState(grid)
	if !dt.ShouldSendKeyframe(61, 840) {
		t.Error("delta with >=840 changes should force keyframe")
	}
}

func TestShouldSendDeltaForSmallChanges(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	dt.UpdateState(grid)
	for deltaSize := 1; deltaSize <= 10; deltaSize++ {
		if dt.ShouldSendKeyframe(61, deltaSize) {
			t.Errorf("small delta (%d changes) should not be keyframe", deltaSize)
		}
	}
}

func TestShouldSendDeltaBetweenKeyframes(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	dt.UpdateState(grid)
	for tick := uint64(1); tick < 60; tick++ {
		if dt.ShouldSendKeyframe(tick, 10) {
			t.Errorf("tick %d should NOT be keyframe (between keyframes)", tick)
		}
	}
	for tick := uint64(61); tick < 120; tick++ {
		if dt.ShouldSendKeyframe(tick, 10) {
			t.Errorf("tick %d should NOT be keyframe (between keyframes)", tick)
		}
	}
}

func TestResetForcesKeyframe(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	dt.UpdateState(grid)
	if dt.ShouldSendKeyframe(61, 10) {
		t.Error("tick 61 should NOT be keyframe before reset")
	}
	dt.Reset()
	if !dt.ShouldSendKeyframe(62, 10) {
		t.Error("tick 62 should be keyframe after reset")
	}
}

func TestGameStartScenario(t *testing.T) {
	dt := NewDeltaTracker()
	grid := makeTestGrid(1)
	grid[10][20] = 5
	if !dt.ShouldSendKeyframe(1, 0) {
		t.Error("first tick should be keyframe")
	}
	dt.UpdateState(grid)
	grid[10][20] = 6
	if dt.ShouldSendKeyframe(2, 1) {
		t.Error("tick 2 with 1 change should NOT be keyframe")
	}
	dt.UpdateState(grid)
	grid[10][20] = 7
	if dt.ShouldSendKeyframe(3, 1) {
		t.Error("tick 3 with 1 change should NOT be keyframe")
	}
}

func TestRoundTripDeltaApplication(t *testing.T) {
	dt := NewDeltaTracker()
	src := makeTestGrid(1)
	src[5][10] = 5
	src[15][25] = 7
	dt.UpdateState(src)
	delta := dt.ComputeDelta(src)
	if delta != nil {
		t.Error("no delta expected for identical grids")
	}
	modified := makeTestGrid(1)
	modified[5][10] = 5
	modified[15][25] = 7
	newDelta := dt.ComputeDelta(modified)
	if len(newDelta) != 0 {
		t.Errorf("expected no changes, got %d", len(newDelta))
	}
}
