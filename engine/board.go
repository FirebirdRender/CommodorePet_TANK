package engine

import "math/rand"

type Board struct {
	Width      int
	Height     int
	Difficulty int
	Grid       [][]CellType
}

func NewBoard(width, height, difficulty int, rng *rand.Rand) *Board {
	grid := make([][]CellType, height)
	for y := range grid {
		grid[y] = make([]CellType, width)
		for x := range grid[y] {
			grid[y][x] = CellEmpty
		}
	}
	b := &Board{Width: width, Height: height, Difficulty: difficulty, Grid: grid}
	b.initBorders()
	b.generateTerrain(rng)
	return b
}

func (b *Board) initBorders() {
	for x := 0; x < b.Width; x++ {
		b.Grid[0][x] = CellWall
		b.Grid[b.Height-1][x] = CellWall
	}
	for y := 0; y < b.Height; y++ {
		b.Grid[y][0] = CellWall
		b.Grid[y][b.Width-1] = CellWall
	}
}

func (b *Board) generateTerrain(rng *rand.Rand) {
	density := 0.04 + (float64(b.Difficulty)/9.0)*0.21
	for y := 1; y <= b.Height-2; y++ {
		for x := 1; x <= b.Width-2; x++ {
			if y == 12 && ((x >= 1 && x <= 5) || (x >= b.Width-6 && x <= b.Width-2)) {
				continue
			}
			if b.isSpawnReserved(x, y) {
				continue
			}
			if rng.Float64() < density {
				b.Grid[y][x] = CellWall
			}
		}
	}
}

// isSpawnReserved guards each tank's spawn cell plus all 8 neighbors against
// terrain walls, guaranteeing at least one legal opening direction at match
// start. Without this, dense-difficulty seeds randomly produce wall-locked
// spawns that strand the tank — repro'd as flake in TestE2EInputAffectsState.
func (b *Board) isSpawnReserved(x, y int) bool {
	for _, sp := range [2][2]int{{SpawnX1, SpawnY1}, {SpawnX2, SpawnY2}} {
		dx := x - sp[0]
		dy := y - sp[1]
		if dx >= -1 && dx <= 1 && dy >= -1 && dy <= 1 {
			return true
		}
	}
	return false
}

func (b *Board) InBounds(x, y int) bool {
	return x >= 0 && x < b.Width && y >= 0 && y < b.Height
}

func (b *Board) GetCell(x, y int) CellType {
	return b.Grid[y][x]
}

func (b *Board) SetCellType(x, y int, ct CellType) {
	b.Grid[y][x] = ct
}

func (b *Board) IsPassable(x, y int) bool {
	ct := b.Grid[y][x]
	return ct == CellEmpty || ct == CellShot || ct == CellMine
}
