package engine

import (
	"math/rand"
	"testing"
)

func newTestBoard() *Board {
	return NewBoard(40, 21, 0, rand.New(rand.NewSource(0)))
}

func clearInterior(b *Board) {
	for y := 1; y <= b.Height-2; y++ {
		for x := 1; x <= b.Width-2; x++ {
			b.SetCellType(x, y, CellEmpty)
		}
	}
}

func newTank(pid, x, y int, dir Direction) *Tank {
	return &Tank{
		PlayerID:  pid,
		X:         x,
		Y:         y,
		Dir:       dir,
		Lives:     3,
		ShotsLeft: 5,
		MinesLeft: 2,
		Active:    true,
	}
}

func TestBarrelPos_AllDirections(t *testing.T) {
	cases := []struct {
		dir    Direction
		wantBX int
		wantBY int
	}{
		{DirUp, 10, 9},
		{DirDown, 10, 11},
		{DirLeft, 9, 10},
		{DirRight, 11, 10},
		{DirUpLeft, 9, 9},
		{DirUpRight, 11, 9},
		{DirDownLeft, 9, 11},
		{DirDownRight, 11, 11},
	}

	for _, c := range cases {
		tk := newTank(1, 10, 10, c.dir)
		bx, by := tk.BarrelPos()
		if bx != c.wantBX || by != c.wantBY {
			t.Errorf("dir=%v: got barrel (%d,%d), want (%d,%d)", c.dir, bx, by, c.wantBX, c.wantBY)
		}
	}
}

func TestOccupyBoard(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	if b.GetCell(10, 10) != CellTank1 {
		t.Errorf("body cell: got %v, want CellTank1", b.GetCell(10, 10))
	}
	if b.GetCell(11, 10) != CellBarrel1 {
		t.Errorf("barrel cell: got %v, want CellBarrel1", b.GetCell(11, 10))
	}
}

func TestOccupyBoard_BarrelNotPlacedOnWall(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	b.SetCellType(11, 10, CellWall)
	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	if b.GetCell(10, 10) != CellTank1 {
		t.Errorf("body cell: got %v, want CellTank1", b.GetCell(10, 10))
	}
	if b.GetCell(11, 10) != CellWall {
		t.Errorf("wall cell should not be overwritten: got %v, want CellWall", b.GetCell(11, 10))
	}
}

func TestClearFromBoard(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)
	tk.ClearFromBoard(b)

	if b.GetCell(10, 10) != CellEmpty {
		t.Errorf("body cell after clear: got %v, want CellEmpty", b.GetCell(10, 10))
	}
	if b.GetCell(11, 10) != CellEmpty {
		t.Errorf("barrel cell after clear: got %v, want CellEmpty", b.GetCell(11, 10))
	}
}

func TestClearFromBoard_DoesNotClearEnemyBarrel(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	b.SetCellType(11, 10, CellBarrel2)
	tk.ClearFromBoard(b)

	if b.GetCell(10, 10) != CellEmpty {
		t.Errorf("body cell after clear: got %v, want CellEmpty", b.GetCell(10, 10))
	}
	if b.GetCell(11, 10) != CellBarrel2 {
		t.Errorf("enemy barrel cell should not be cleared: got %v, want CellBarrel2", b.GetCell(11, 10))
	}
}

func TestAttemptMove_ValidMove(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	ok := tk.AttemptMove(b, DirRight)
	if !ok {
		t.Fatal("expected move to succeed")
	}
	if tk.X != 11 || tk.Y != 10 {
		t.Errorf("position: got (%d,%d), want (11,10)", tk.X, tk.Y)
	}
	if tk.Dir != DirRight {
		t.Errorf("direction: got %v, want DirRight", tk.Dir)
	}
	if b.GetCell(11, 10) != CellTank1 {
		t.Errorf("new body cell: got %v, want CellTank1", b.GetCell(11, 10))
	}
}

func TestAttemptMove_BlockedByWall(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	b.SetCellType(11, 10, CellWall)
	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	ok := tk.AttemptMove(b, DirRight)
	if ok {
		t.Fatal("expected move to fail (blocked by wall)")
	}
	if tk.X != 10 || tk.Y != 10 {
		t.Errorf("position should be unchanged: got (%d,%d)", tk.X, tk.Y)
	}
}

func TestAttemptMove_BlockedByTank(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	b.SetCellType(11, 10, CellTank2)
	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	ok := tk.AttemptMove(b, DirRight)
	if ok {
		t.Fatal("expected move to fail (blocked by enemy tank)")
	}
	if tk.X != 10 || tk.Y != 10 {
		t.Errorf("position should be unchanged: got (%d,%d)", tk.X, tk.Y)
	}
}

func TestAttemptMove_BlockedByEnemyBarrel(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	b.SetCellType(11, 10, CellBarrel2)
	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	ok := tk.AttemptMove(b, DirRight)
	if ok {
		t.Fatal("expected move to fail (blocked by enemy barrel)")
	}
	if tk.X != 10 || tk.Y != 10 {
		t.Errorf("position should be unchanged: got (%d,%d)", tk.X, tk.Y)
	}
}

func TestAttemptMove_PassThroughOwnBarrel(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	ok := tk.AttemptMove(b, DirRight)
	if !ok {
		t.Fatal("expected move through own barrel to succeed")
	}
	if tk.X != 11 || tk.Y != 10 {
		t.Errorf("position: got (%d,%d), want (11,10)", tk.X, tk.Y)
	}
}

func TestAttemptMove_PassThroughMine(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	b.SetCellType(11, 10, CellMine)
	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	ok := tk.AttemptMove(b, DirRight)
	if !ok {
		t.Fatal("expected move through mine to succeed")
	}
	if tk.X != 11 || tk.Y != 10 {
		t.Errorf("position: got (%d,%d), want (11,10)", tk.X, tk.Y)
	}
}

func TestAttemptMove_PassThroughShot(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	b.SetCellType(11, 10, CellShot)
	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	ok := tk.AttemptMove(b, DirRight)
	if !ok {
		t.Fatal("expected move through shot to succeed")
	}
	if tk.X != 11 || tk.Y != 10 {
		t.Errorf("position: got (%d,%d), want (11,10)", tk.X, tk.Y)
	}
}

func TestAttemptMove_BoundaryClamping(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	tk := newTank(1, 1, 10, DirLeft)
	tk.OccupyBoard(b)

	ok := tk.AttemptMove(b, DirLeft)
	if ok {
		t.Fatal("expected move at boundary to fail (clamped to same cell)")
	}
	if tk.X != 1 || tk.Y != 10 {
		t.Errorf("position should be unchanged: got (%d,%d)", tk.X, tk.Y)
	}
}

func TestAttemptMove_DirectionUpdates(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	tk := newTank(1, 10, 10, DirRight)
	tk.OccupyBoard(b)

	ok := tk.AttemptMove(b, DirUp)
	if !ok {
		t.Fatal("expected move to succeed")
	}
	if tk.Dir != DirUp {
		t.Errorf("direction: got %v, want DirUp", tk.Dir)
	}
}

func TestCanFire_ShotsLeft(t *testing.T) {
	tk := newTank(1, 10, 10, DirRight)
	tk.ShotsLeft = 5
	if !tk.CanFire() {
		t.Error("CanFire should be true when ShotsLeft=5")
	}
	tk.ShotsLeft = 0
	if tk.CanFire() {
		t.Error("CanFire should be false when ShotsLeft=0")
	}
}

func TestConsumeShot(t *testing.T) {
	tk := newTank(1, 10, 10, DirRight)
	tk.ShotsLeft = 3
	tk.ConsumeShot()
	if tk.ShotsLeft != 2 {
		t.Errorf("ShotsLeft: got %d, want 2", tk.ShotsLeft)
	}

	tk.ShotsLeft = 0
	tk.ConsumeShot()
	if tk.ShotsLeft != 0 {
		t.Errorf("ShotsLeft should not underflow: got %d, want 0", tk.ShotsLeft)
	}
}

func TestTakeDamage_LivesDecrement(t *testing.T) {
	tk := newTank(1, 10, 10, DirRight)
	tk.Lives = 3

	tk.TakeDamage()
	if tk.Lives != 2 {
		t.Errorf("Lives: got %d, want 2", tk.Lives)
	}
	if !tk.IsAlive() {
		t.Error("IsAlive should be true when Lives=2")
	}

	tk.TakeDamage()
	tk.TakeDamage()
	if tk.Lives != 0 {
		t.Errorf("Lives: got %d, want 0", tk.Lives)
	}
	if tk.IsAlive() {
		t.Error("IsAlive should be false when Lives=0")
	}
}

func TestCanPlaceMine_MinesLeft(t *testing.T) {
	tk := newTank(1, 10, 10, DirRight)
	tk.MinesLeft = 2
	if !tk.CanPlaceMine() {
		t.Error("CanPlaceMine should be true when MinesLeft=2")
	}
	tk.MinesLeft = 0
	if tk.CanPlaceMine() {
		t.Error("CanPlaceMine should be false when MinesLeft=0")
	}
}

func TestConsumeMine(t *testing.T) {
	tk := newTank(1, 10, 10, DirRight)
	tk.MinesLeft = 2
	tk.ConsumeMine()
	if tk.MinesLeft != 1 {
		t.Errorf("MinesLeft: got %d, want 1", tk.MinesLeft)
	}

	tk.MinesLeft = 0
	tk.ConsumeMine()
	if tk.MinesLeft != 0 {
		t.Errorf("MinesLeft should not underflow: got %d, want 0", tk.MinesLeft)
	}
}
