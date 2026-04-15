package server

const (
	// KeyframeInterval defines how often a full tick is sent (every N ticks)
	// A full tick acts as a resync point for clients that may have missed deltas
	KeyframeInterval = 60 // 1 keyframe per second at 60fps
)

// CellChange represents a single changed cell in the grid
type CellChange struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Cell int `json:"cell"`
}

// TickDeltaMsg is sent instead of TickMsg for non-keyframe ticks.
// It contains only the changes since the last tick, not the full state.
type TickDeltaMsg struct {
	Tick            uint64                `json:"tick"`
	ChangedCells    []CellChange          `json:"changed_cells"`
	Tanks           [2]TankState          `json:"tanks"`
	Shots           []ShotState           `json:"shots"`
	Mines           []MineState           `json:"mines"`
	Explosions      []ExplosionState      `json:"explosions"`
	BarrelWreckage  []BarrelWreckageState `json:"barrel_wreckage"`
	BarrelHitBodies [][2]int              `json:"barrel_hit_bodies"`
}

// DeltaTracker computes the diff between ticks and decides when to send
// full keyframes vs delta updates.
type DeltaTracker struct {
	prevGrid [][]int // Previous tick's grid state (flat copy)
	tick     uint64  // Last tick that was tracked
	keyframe bool    // Whether the last send was a keyframe
}

// NewDeltaTracker creates a new DeltaTracker with an empty previous state.
func NewDeltaTracker() *DeltaTracker {
	return &DeltaTracker{}
}

// ComputeDelta compares the current grid against the previous tick's grid
// and returns a list of changed cells.
// Returns nil if this is the first call (no previous state to compare against).
func (dt *DeltaTracker) ComputeDelta(currentGrid [][]int) []CellChange {
	if dt.prevGrid == nil {
		return nil // First tick — send as keyframe
	}

	var changes []CellChange
	for y := range currentGrid {
		for x := range currentGrid[y] {
			if currentGrid[y][x] != dt.prevGrid[y][x] {
				changes = append(changes, CellChange{
					X:    x,
					Y:    y,
					Cell: currentGrid[y][x],
				})
			}
		}
	}
	return changes
}

// UpdateState saves the current grid as the new "previous" state for next tick's diff.
// Must be called AFTER ComputeDelta and AFTER the decision to send keyframe or delta.
func (dt *DeltaTracker) UpdateState(currentGrid [][]int) {
	dt.prevGrid = copyGrid(currentGrid)
}

// Reset clears the tracker state, forcing the next tick to be a keyframe.
// Call this after game_start, round resume, or any state reset.
func (dt *DeltaTracker) Reset() {
	dt.prevGrid = nil
	dt.tick = 0
	dt.keyframe = false
}

// ShouldSendKeyframe returns true if the current tick should be sent as a full
// keyframe instead of a delta. Keyframes are sent:
// - On the first tick after Reset()
// - Every KeyframeInterval ticks
// - When the delta would be larger than a full tick (too many changes)
func (dt *DeltaTracker) ShouldSendKeyframe(tick uint64, deltaSize int) bool {
	// First tick after reset
	if dt.prevGrid == nil {
		return true
	}
	// Periodic keyframe
	if tick%KeyframeInterval == 0 {
		return true
	}
	// Delta larger than full grid (840 cells * ~3 bytes each ≈ 2520 bytes)
	// If delta has more than 840 changes, just send the full grid
	if deltaSize >= 840 {
		return true
	}
	return false
}

// copyGrid creates a deep copy of a 2D grid slice
func copyGrid(grid [][]int) [][]int {
	cp := make([][]int, len(grid))
	for y := range grid {
		cp[y] = make([]int, len(grid[y]))
		copy(cp[y], grid[y])
	}
	return cp
}
