package engine

import (
	"math/rand"
	"testing"
)

func newTestGC(difficulty int) *GameController {
	rng := rand.New(rand.NewSource(42))
	return NewGameController(difficulty, rng)
}

func newTestGCClearBoard(difficulty int) *GameController {
	rng := rand.New(rand.NewSource(42))
	gc := NewGameController(difficulty, rng)
	for y := 1; y < gc.Board.Height-1; y++ {
		for x := 1; x < gc.Board.Width-1; x++ {
			if gc.Board.GetCell(x, y) == CellWall {
				gc.Board.SetCellType(x, y, CellEmpty)
			}
		}
	}
	return gc
}

func TestPlaceDiceWreckage_CornerPositions(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)

	gc.PlaceDiceWreckage(10, 10, 1)

	corners := [][2]int{{9, 9}, {11, 9}, {9, 11}, {11, 11}}
	for _, p := range corners {
		if got := gc.Board.GetCell(p[0], p[1]); got != CellWreckageP1 {
			t.Fatalf("corner %v: got %v want CellWreckageP1", p, got)
		}
	}
}

func TestPlaceDiceWreckage_ReplacesWall(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	gc.Board.SetCellType(9, 9, CellWall)

	gc.PlaceDiceWreckage(10, 10, 1)

	if got := gc.Board.GetCell(9, 9); got != CellWreckageP1 {
		t.Fatalf("got %v want CellWreckageP1", got)
	}
}

func TestPlaceDiceWreckage_ReplacesEmpty(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	gc.Board.SetCellType(9, 9, CellEmpty)

	gc.PlaceDiceWreckage(10, 10, 2)

	if got := gc.Board.GetCell(9, 9); got != CellWreckageP2 {
		t.Fatalf("got %v want CellWreckageP2", got)
	}
}

func TestPlaceDiceWreckage_ReplacesMine(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	gc.Board.SetCellType(9, 9, CellMine)

	gc.PlaceDiceWreckage(10, 10, 1)

	if got := gc.Board.GetCell(9, 9); got != CellWreckageP1 {
		t.Fatalf("got %v want CellWreckageP1", got)
	}
}

func TestPlaceDiceWreckage_ReplacesShot(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	gc.Board.SetCellType(9, 9, CellShot)

	gc.PlaceDiceWreckage(10, 10, 1)

	if got := gc.Board.GetCell(9, 9); got != CellWreckageP1 {
		t.Fatalf("got %v want CellWreckageP1", got)
	}
}

func TestPlaceDiceWreckage_ReplacesAnyBarrel(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)

	gc.Board.SetCellType(9, 9, CellBarrel1)
	gc.Board.SetCellType(11, 9, CellBarrel2)

	gc.PlaceDiceWreckage(10, 10, 2)

	if got := gc.Board.GetCell(9, 9); got != CellWreckageP2 {
		t.Fatalf("barrel1 cell: got %v want CellWreckageP2", got)
	}
	if got := gc.Board.GetCell(11, 9); got != CellWreckageP2 {
		t.Fatalf("barrel2 cell: got %v want CellWreckageP2", got)
	}
}

func TestPlaceDiceWreckage_SkipsTankBody(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)

	gc.Board.SetCellType(9, 9, CellTank1)
	gc.Board.SetCellType(11, 9, CellTank2)

	gc.PlaceDiceWreckage(10, 10, 1)

	if got := gc.Board.GetCell(9, 9); got != CellTank1 {
		t.Fatalf("tank1 cell should be preserved: got %v want CellTank1", got)
	}
	if got := gc.Board.GetCell(11, 9); got != CellTank2 {
		t.Fatalf("tank2 cell should be preserved: got %v want CellTank2", got)
	}
}

func TestPlaceDiceWreckage_SkipsExistingWreckage(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)

	gc.Board.SetCellType(9, 9, CellWreckageP1)
	gc.Board.SetCellType(11, 9, CellWreckageP2)

	gc.PlaceDiceWreckage(10, 10, 2)

	if got := gc.Board.GetCell(9, 9); got != CellWreckageP1 {
		t.Fatalf("wreckage p1 should be preserved: got %v want CellWreckageP1", got)
	}
	if got := gc.Board.GetCell(11, 9); got != CellWreckageP2 {
		t.Fatalf("wreckage p2 should be preserved: got %v want CellWreckageP2", got)
	}
}

func TestPlaceDiceWreckage_SkipsOutOfBounds(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)

	gc.PlaceDiceWreckage(1, 1, 1)

	if got := gc.Board.GetCell(2, 2); got != CellWreckageP1 {
		t.Fatalf("in-bounds corner should be set: got %v want CellWreckageP1", got)
	}
}

func TestTankHit_DamageAndExplosion(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	tk := gc.Tanks[0]
	tk.X, tk.Y, tk.Dir = 10, 10, DirRight
	tk.OccupyBoard(gc.Board)
	gc.SimTime = 12.75

	gc.TankHit(tk, [2]int{10, 10})

	if tk.Lives != 2 {
		t.Fatalf("Lives: got %d want 2", tk.Lives)
	}
	if len(gc.Explosions) != 1 {
		t.Fatalf("Explosions len: got %d want 1", len(gc.Explosions))
	}
	ex := gc.Explosions[0]
	if ex.X != 10 || ex.Y != 10 {
		t.Fatalf("explosion pos: got (%d,%d) want (10,10)", ex.X, ex.Y)
	}
	if ex.StartTime != 12.75 {
		t.Fatalf("StartTime: got %v want 12.75", ex.StartTime)
	}
	if ex.Duration != ExplosionDuration {
		t.Fatalf("Duration: got %v want %v", ex.Duration, ExplosionDuration)
	}
	if ex.IsChainReaction {
		t.Fatal("IsChainReaction: got true want false")
	}
}

func TestTankHit_ClearsFromBoard(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	tk := gc.Tanks[0]
	tk.X, tk.Y, tk.Dir = 10, 10, DirRight
	tk.OccupyBoard(gc.Board)

	gc.TankHit(tk, [2]int{20, 10})

	if got := gc.Board.GetCell(10, 10); got == CellTank1 {
		t.Fatalf("body cell should be cleared: got %v", got)
	}
	if got := gc.Board.GetCell(11, 10); got == CellBarrel1 {
		t.Fatalf("barrel cell should be cleared: got %v", got)
	}
}

func TestTankHit_PlacesDiceWreckage(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	tk := gc.Tanks[0]
	tk.X, tk.Y, tk.Dir = 10, 10, DirRight
	tk.OccupyBoard(gc.Board)

	gc.TankHit(tk, [2]int{20, 10})

	corners := [][2]int{{19, 9}, {21, 9}, {19, 11}, {21, 11}}
	for _, p := range corners {
		if got := gc.Board.GetCell(p[0], p[1]); got != CellWreckageP1 {
			t.Fatalf("corner %v: got %v want CellWreckageP1", p, got)
		}
	}
}

func TestBarrelHit_BodyAndBarrelToWreckage(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	tk := gc.Tanks[0]
	tk.X, tk.Y, tk.Dir = 10, 10, DirRight
	tk.OccupyBoard(gc.Board)

	gc.BarrelHit(tk, [2]int{11, 10})

	if got := gc.Board.GetCell(10, 10); got != CellWreckageP1 {
		t.Fatalf("body cell: got %v want CellWreckageP1", got)
	}
	if got := gc.Board.GetCell(11, 10); got != CellWreckageP1 {
		t.Fatalf("barrel cell: got %v want CellWreckageP1", got)
	}
}

func TestBarrelHit_NoExplosion(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	tk := gc.Tanks[0]
	tk.X, tk.Y, tk.Dir = 10, 10, DirRight
	tk.OccupyBoard(gc.Board)

	gc.BarrelHit(tk, [2]int{11, 10})

	if len(gc.Explosions) != 0 {
		t.Fatalf("Explosions len: got %d want 0", len(gc.Explosions))
	}
}

func TestBarrelHit_DamageAndRegistry(t *testing.T) {
	gc := newTestGC(5)
	clearInterior(gc.Board)
	tk := gc.Tanks[0]
	tk.X, tk.Y, tk.Dir = 10, 10, DirRight
	tk.OccupyBoard(gc.Board)

	gc.BarrelHit(tk, [2]int{11, 10})

	if tk.Lives != 2 {
		t.Fatalf("Lives: got %d want 2", tk.Lives)
	}
	bodyPos := [2]int{10, 10}
	if !gc.BarrelHitBodies[bodyPos] {
		t.Fatalf("BarrelHitBodies missing %v", bodyPos)
	}
	if len(gc.BarrelWreckageRegistry) != 1 {
		t.Fatalf("BarrelWreckageRegistry len: got %d want 1", len(gc.BarrelWreckageRegistry))
	}
	entry := gc.BarrelWreckageRegistry[0]
	if entry.Pos != [2]int{11, 10} {
		t.Fatalf("registry pos: got %v want [11 10]", entry.Pos)
	}
	if entry.Dir != DirRight {
		t.Fatalf("registry dir: got %v want DirRight", entry.Dir)
	}
}

func TestNewGameController_InitialState(t *testing.T) {
	gc := newTestGC(5)

	if gc.Board == nil {
		t.Fatal("Board should not be nil")
	}
	if gc.Difficulty != 5 {
		t.Fatalf("Difficulty: got %d want 5", gc.Difficulty)
	}
	if gc.Tanks[0] == nil || gc.Tanks[1] == nil {
		t.Fatal("both tanks should be initialized")
	}

	t1 := gc.Tanks[0]
	if t1.PlayerID != 1 || t1.X != 2 || t1.Y != 10 || t1.Dir != DirRight {
		t.Fatalf("tank1 init mismatch: %+v", *t1)
	}
	t2 := gc.Tanks[1]
	if t2.PlayerID != 2 || t2.X != BoardWidth-3 || t2.Y != 10 || t2.Dir != DirLeft {
		t.Fatalf("tank2 init mismatch: %+v", *t2)
	}

	lives, shots, mines := DifficultyToResources(5)
	if t1.Lives != lives || t1.ShotsLeft != shots || t1.MinesLeft != mines {
		t.Fatalf("tank1 resources: got (L=%d,S=%d,M=%d) want (L=%d,S=%d,M=%d)", t1.Lives, t1.ShotsLeft, t1.MinesLeft, lives, shots, mines)
	}
	if t2.Lives != lives || t2.ShotsLeft != shots || t2.MinesLeft != mines {
		t.Fatalf("tank2 resources: got (L=%d,S=%d,M=%d) want (L=%d,S=%d,M=%d)", t2.Lives, t2.ShotsLeft, t2.MinesLeft, lives, shots, mines)
	}

	if !t1.Active || !t2.Active {
		t.Fatal("both tanks should start active")
	}

	if gc.Board.GetCell(t1.X, t1.Y) != CellTank1 {
		t.Fatalf("tank1 body not on board: got %v", gc.Board.GetCell(t1.X, t1.Y))
	}
	bx1, by1 := t1.BarrelPos()
	if gc.Board.GetCell(bx1, by1) != CellBarrel1 {
		t.Fatalf("tank1 barrel not on board: got %v", gc.Board.GetCell(bx1, by1))
	}
	if gc.Board.GetCell(t2.X, t2.Y) != CellTank2 {
		t.Fatalf("tank2 body not on board: got %v", gc.Board.GetCell(t2.X, t2.Y))
	}
	bx2, by2 := t2.BarrelPos()
	if gc.Board.GetCell(bx2, by2) != CellBarrel2 {
		t.Fatalf("tank2 barrel not on board: got %v", gc.Board.GetCell(bx2, by2))
	}

	if gc.Board.GetCell(0, 0) != CellWall || gc.Board.GetCell(BoardWidth-1, BoardHeight-1) != CellWall {
		t.Fatal("board borders should be walls")
	}
}

func TestBarrelSwing_DirectionChange(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tank := gc.Tanks[0]
	startX, startY := tank.X, tank.Y

	ok := gc.ProcessMovement(1, DirUp, 0.0)
	if !ok {
		t.Fatal("expected swing to succeed")
	}
	if tank.Dir != DirUp {
		t.Fatalf("direction: got %v want %v", tank.Dir, DirUp)
	}
	if tank.X != startX || tank.Y != startY {
		t.Fatalf("position should not change on swing: got (%d,%d) want (%d,%d)", tank.X, tank.Y, startX, startY)
	}
}

func TestBarrelSwing_SameDirection(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tank := gc.Tanks[0]
	startX, startY := tank.X, tank.Y

	ok := gc.ProcessMovement(1, DirRight, 0.0)
	if !ok {
		t.Fatal("expected move input to succeed")
	}
	if tank.X != startX+1 || tank.Y != startY {
		t.Fatalf("position: got (%d,%d) want (%d,%d)", tank.X, tank.Y, startX+1, startY)
	}
}

func TestBarrelSwing_BarrelReplacedOnBoard(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tank := gc.Tanks[0]
	oldBX, oldBY := tank.BarrelPos()

	ok := gc.ProcessMovement(1, DirUp, 0.0)
	if !ok {
		t.Fatal("expected swing to succeed")
	}

	if got := gc.Board.GetCell(oldBX, oldBY); got != CellEmpty {
		t.Fatalf("old barrel cell: got %v want CellEmpty", got)
	}
	newBX, newBY := tank.BarrelPos()
	if got := gc.Board.GetCell(newBX, newBY); got != CellBarrel1 {
		t.Fatalf("new barrel cell: got %v want CellBarrel1", got)
	}
}

func TestBarrelSwing_MoveDelayRespected(t *testing.T) {
	gc := newTestGCClearBoard(5)

	if ok := gc.ProcessMovement(1, DirRight, 0.0); !ok {
		t.Fatal("first movement at t=0 should succeed")
	}
	delay := MoveDelay(gc.Difficulty)
	if ok := gc.ProcessMovement(1, DirRight, delay-0.01); ok {
		t.Fatal("movement before delay threshold should fail")
	}
}

func TestBarrelSwing_MoveDelayAllowed(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tank := gc.Tanks[0]
	startX := tank.X

	if ok := gc.ProcessMovement(1, DirRight, 0.0); !ok {
		t.Fatal("first movement at t=0 should succeed")
	}
	delay := MoveDelay(gc.Difficulty)
	if ok := gc.ProcessMovement(1, DirRight, delay); !ok {
		t.Fatal("movement at delay threshold should succeed")
	}
	if tank.X != startX+2 {
		t.Fatalf("expected two moves to the right: got X=%d want %d", tank.X, startX+2)
	}
}

func TestBarrelSwing_SwingThenMove(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tank := gc.Tanks[0]
	startX, startY := tank.X, tank.Y

	if ok := gc.ProcessMovement(1, DirUp, 0.0); !ok {
		t.Fatal("swing should succeed")
	}
	if tank.X != startX || tank.Y != startY {
		t.Fatalf("position should not change on swing: got (%d,%d) want (%d,%d)", tank.X, tank.Y, startX, startY)
	}

	delay := MoveDelay(gc.Difficulty)
	if ok := gc.ProcessMovement(1, DirUp, delay); !ok {
		t.Fatal("move after swing with same direction should succeed")
	}
	if tank.X != startX || tank.Y != startY-1 {
		t.Fatalf("position after swing-then-move: got (%d,%d) want (%d,%d)", tank.X, tank.Y, startX, startY-1)
	}
}

func TestBarrelSwing_SwingToNewDirTwice(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tank := gc.Tanks[0]
	startX, startY := tank.X, tank.Y

	if ok := gc.ProcessMovement(1, DirUp, 0.0); !ok {
		t.Fatal("first swing should succeed")
	}
	delay := MoveDelay(gc.Difficulty)
	if ok := gc.ProcessMovement(1, DirLeft, delay); !ok {
		t.Fatal("second swing to new direction should succeed")
	}

	if tank.Dir != DirLeft {
		t.Fatalf("direction: got %v want %v", tank.Dir, DirLeft)
	}
	if tank.X != startX || tank.Y != startY {
		t.Fatalf("position should not change on two swings: got (%d,%d) want (%d,%d)", tank.X, tank.Y, startX, startY)
	}
	if gc.SwingDir[1] == nil || *gc.SwingDir[1] != DirLeft {
		t.Fatalf("swing dir: got %v want %v", gc.SwingDir[1], DirLeft)
	}
}
