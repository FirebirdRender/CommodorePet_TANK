package engine

import (
	"math/rand"
	"testing"
)

func TestNewBoard_Dimensions(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	b := NewBoard(40, 21, 5, rng)
	if b.Width != 40 {
		t.Fatalf("Width: got %d want 40", b.Width)
	}
	if b.Height != 21 {
		t.Fatalf("Height: got %d want 21", b.Height)
	}
	if len(b.Grid) != 21 {
		t.Fatalf("Grid rows: got %d want 21", len(b.Grid))
	}
	for y, row := range b.Grid {
		if len(row) != 40 {
			t.Fatalf("Grid[%d] cols: got %d want 40", y, len(row))
		}
	}
}

func TestNewBoard_Borders(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	b := NewBoard(40, 21, 5, rng)
	for x := 0; x < 40; x++ {
		if b.Grid[0][x] != CellWall {
			t.Fatalf("top border [0][%d] = %v, want CellWall", x, b.Grid[0][x])
		}
		if b.Grid[20][x] != CellWall {
			t.Fatalf("bottom border [20][%d] = %v, want CellWall", x, b.Grid[20][x])
		}
	}
	for y := 0; y < 21; y++ {
		if b.Grid[y][0] != CellWall {
			t.Fatalf("left border [%d][0] = %v, want CellWall", y, b.Grid[y][0])
		}
		if b.Grid[y][39] != CellWall {
			t.Fatalf("right border [%d][39] = %v, want CellWall", y, b.Grid[y][39])
		}
	}
}

func TestNewBoard_SpawnZonesCleared(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	b := NewBoard(40, 21, 9, rng)
	for x := 1; x <= 5; x++ {
		if b.Grid[12][x] == CellWall {
			t.Fatalf("spawn zone left [12][%d] should not be CellWall", x)
		}
	}
	for x := 34; x <= 38; x++ {
		if b.Grid[12][x] == CellWall {
			t.Fatalf("spawn zone right [12][%d] should not be CellWall", x)
		}
	}
}

func TestNewBoard_SpawnNeighborsClearAcrossSeeds(t *testing.T) {
	for seed := int64(0); seed < 200; seed++ {
		b := NewBoard(BoardWidth, BoardHeight, 9, rand.New(rand.NewSource(seed)))
		for _, sp := range [2][2]int{{SpawnX1, SpawnY1}, {SpawnX2, SpawnY2}} {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					x, y := sp[0]+dx, sp[1]+dy
					if b.Grid[y][x] == CellWall {
						t.Fatalf("seed %d spawn (%d,%d) neighbor (%d,%d) is CellWall — tank would be wall-locked", seed, sp[0], sp[1], x, y)
					}
				}
			}
		}
	}
}

func TestGenerateTerrain_Deterministic(t *testing.T) {
	b1 := NewBoard(40, 21, 5, rand.New(rand.NewSource(42)))
	b2 := NewBoard(40, 21, 5, rand.New(rand.NewSource(42)))
	for y := 0; y < 21; y++ {
		for x := 0; x < 40; x++ {
			if b1.Grid[y][x] != b2.Grid[y][x] {
				t.Fatalf("mismatch at [%d][%d]: %v vs %v", y, x, b1.Grid[y][x], b2.Grid[y][x])
			}
		}
	}
}

func TestGenerateTerrain_DensityScaling(t *testing.T) {
	b0 := NewBoard(40, 21, 0, rand.New(rand.NewSource(42)))
	b9 := NewBoard(40, 21, 9, rand.New(rand.NewSource(42)))

	count := func(b *Board) int {
		n := 0
		for y := 1; y < b.Height-1; y++ {
			for x := 1; x < b.Width-1; x++ {
				if b.Grid[y][x] == CellWall {
					n++
				}
			}
		}
		return n
	}

	c0 := count(b0)
	c9 := count(b9)

	if c9 <= c0 {
		t.Fatalf("diff-9 walls (%d) should exceed diff-0 walls (%d)", c9, c0)
	}

	interior := 38 * 19
	target0 := int(float64(interior) * 0.04)
	target9 := int(float64(interior) * 0.25)

	if absDiffInt(c0, target0) > 15 {
		t.Fatalf("diff-0 wall count %d too far from expected ~%d (±15)", c0, target0)
	}
	if absDiffInt(c9, target9) > 40 {
		t.Fatalf("diff-9 wall count %d too far from expected ~%d (±40)", c9, target9)
	}
}

func TestInBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	b := NewBoard(40, 21, 0, rng)
	cases := []struct {
		x, y int
		want bool
	}{
		{0, 0, true},
		{39, 20, true},
		{-1, 0, false},
		{40, 0, false},
		{0, -1, false},
		{0, 21, false},
	}
	for _, tc := range cases {
		if got := b.InBounds(tc.x, tc.y); got != tc.want {
			t.Fatalf("InBounds(%d,%d): got %v want %v", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestGetCell_SetCellType(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	b := NewBoard(40, 21, 0, rng)
	b.SetCellType(5, 5, CellWall)
	if got := b.GetCell(5, 5); got != CellWall {
		t.Fatalf("after SetCellType CellWall: got %v want CellWall", got)
	}
	b.SetCellType(5, 5, CellEmpty)
	if got := b.GetCell(5, 5); got != CellEmpty {
		t.Fatalf("after SetCellType CellEmpty: got %v want CellEmpty", got)
	}
}

func TestIsPassable(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	b := NewBoard(40, 21, 0, rng)

	passable := []CellType{CellEmpty, CellShot, CellMine}
	notPassable := []CellType{CellWall, CellTank1, CellTank2, CellBarrel1, CellBarrel2, CellWreckageP1, CellWreckageP2}

	x, y := 5, 5
	for _, ct := range passable {
		b.SetCellType(x, y, ct)
		if !b.IsPassable(x, y) {
			t.Fatalf("IsPassable: %v should be passable", ct)
		}
	}
	for _, ct := range notPassable {
		b.SetCellType(x, y, ct)
		if b.IsPassable(x, y) {
			t.Fatalf("IsPassable: %v should NOT be passable", ct)
		}
	}
}

func absDiffInt(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
