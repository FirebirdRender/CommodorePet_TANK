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

func TestFindSpawnPos_EmptyBoard(t *testing.T) {
	gc := newTestGCClearBoard(5)

	x, y := gc.FindSpawnPos(20, 10)
	if x != 20 || y != 10 {
		t.Fatalf("FindSpawnPos: got (%d,%d) want (20,10)", x, y)
	}
}

func TestFindSpawnPos_BlockedCenter(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(20, 10, CellWall)

	x, y := gc.FindSpawnPos(20, 10)
	if x != 20 || y != 9 {
		t.Fatalf("FindSpawnPos blocked center: got (%d,%d) want (20,9)", x, y)
	}
}

func TestFindSpawnPos_ScanOrder(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(20, 10, CellWall)

	x, y := gc.FindSpawnPos(20, 10)
	if x != 20 || y != 9 {
		t.Fatalf("FindSpawnPos scan order: got (%d,%d) want up-first (20,9)", x, y)
	}
}

func TestFindSpawnPos_NearEdge(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(20, 1, CellWall)

	x, y := gc.FindSpawnPos(20, 1)
	if x != 20 || y != 2 {
		t.Fatalf("FindSpawnPos near edge: got (%d,%d) want (20,2)", x, y)
	}
}

func TestRespawnSingle_ResetsAmmo(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tank := gc.Tanks[0]
	tank.ShotsLeft = 0
	tank.MinesLeft = 0

	gc.respawnSingleTank(1)

	_, wantShots, wantMines := DifficultyToResources(gc.Difficulty)
	if tank.ShotsLeft != wantShots || tank.MinesLeft != wantMines {
		t.Fatalf("ammo after respawn: got (S=%d,M=%d) want (S=%d,M=%d)", tank.ShotsLeft, tank.MinesLeft, wantShots, wantMines)
	}
}

func TestRespawnSingle_NewPosition(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tank := gc.Tanks[0]
	tank.ClearFromBoard(gc.Board)
	tank.X, tank.Y, tank.Dir = 20, 15, DirUp
	tank.OccupyBoard(gc.Board)

	gc.Board.SetCellType(tank.StartX, tank.StartY, CellWall)

	gc.respawnSingleTank(1)

	if tank.X != tank.StartX || tank.Y != tank.StartY-1 {
		t.Fatalf("respawn position: got (%d,%d) want (%d,%d)", tank.X, tank.Y, tank.StartX, tank.StartY-1)
	}
}

func TestRespawnSingle_Direction(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Tanks[0].Dir = DirUp
	gc.Tanks[1].Dir = DirDown

	gc.respawnSingleTank(1)
	gc.respawnSingleTank(2)

	if gc.Tanks[0].Dir != DirRight {
		t.Fatalf("P1 direction: got %v want DirRight", gc.Tanks[0].Dir)
	}
	if gc.Tanks[1].Dir != DirLeft {
		t.Fatalf("P2 direction: got %v want DirLeft", gc.Tanks[1].Dir)
	}
}

func TestRespawnSingle_DoesNotClearMines(t *testing.T) {
	gc := newTestGCClearBoard(5)
	mine := NewMine(10, 10, 1, 0.0)
	gc.Mines = append(gc.Mines, mine)
	gc.Board.SetCellType(10, 10, CellMine)

	gc.respawnSingleTank(1)

	if len(gc.Mines) != 1 {
		t.Fatalf("mines list should be preserved: got %d want 1", len(gc.Mines))
	}
	if gc.Board.GetCell(10, 10) != CellMine {
		t.Fatalf("mine on board should be preserved: got %v want CellMine", gc.Board.GetCell(10, 10))
	}
}

func TestRespawnBoth_BothTanksReset(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Tanks[0].ShotsLeft, gc.Tanks[0].MinesLeft = 0, 0
	gc.Tanks[1].ShotsLeft, gc.Tanks[1].MinesLeft = 0, 0

	gc.Board.SetCellType(gc.Tanks[0].StartX, gc.Tanks[0].StartY, CellWall)
	gc.Board.SetCellType(gc.Tanks[1].StartX, gc.Tanks[1].StartY, CellWall)

	gc.respawnBothTanks()

	_, wantShots, wantMines := DifficultyToResources(gc.Difficulty)
	for i, tank := range gc.Tanks {
		if tank.ShotsLeft != wantShots || tank.MinesLeft != wantMines {
			t.Fatalf("tank %d ammo after respawn: got (S=%d,M=%d) want (S=%d,M=%d)", i+1, tank.ShotsLeft, tank.MinesLeft, wantShots, wantMines)
		}
		wantX, wantY := tank.StartX, tank.StartY-1
		if tank.X != wantX || tank.Y != wantY {
			t.Fatalf("tank %d position after respawn: got (%d,%d) want (%d,%d)", i+1, tank.X, tank.Y, wantX, wantY)
		}
	}
}

func TestRespawnBoth_DoesNotClearMines(t *testing.T) {
	gc := newTestGCClearBoard(5)
	mine := NewMine(10, 10, 1, 0.0)
	gc.Mines = append(gc.Mines, mine)
	gc.Board.SetCellType(10, 10, CellMine)

	gc.respawnBothTanks()

	if len(gc.Mines) != 1 {
		t.Fatalf("mines list should be preserved: got %d want 1", len(gc.Mines))
	}
	if gc.Board.GetCell(10, 10) != CellMine {
		t.Fatalf("mine on board should be preserved: got %v want CellMine", gc.Board.GetCell(10, 10))
	}
}

func TestRespawnBoth_ClearsSwing(t *testing.T) {
	gc := newTestGCClearBoard(5)
	d1 := DirUp
	d2 := DirDown
	gc.SwingDir[1] = &d1
	gc.SwingDir[2] = &d2

	gc.respawnBothTanks()

	if gc.SwingDir[1] != nil || gc.SwingDir[2] != nil {
		t.Fatalf("swing dirs should be cleared: got p1=%v p2=%v", gc.SwingDir[1], gc.SwingDir[2])
	}
}

func TestOnPlayerDefeated_WinnerSet(t *testing.T) {
	gc := newTestGCClearBoard(5)

	gc.onPlayerDefeated(2)
	if gc.Winner != 1 {
		t.Fatalf("winner after player 2 defeat: got %d want 1", gc.Winner)
	}

	gc.onPlayerDefeated(1)
	if gc.Winner != 2 {
		t.Fatalf("winner after player 1 defeat: got %d want 2", gc.Winner)
	}
}

func TestOnPlayerDefeated_WinsIncremented(t *testing.T) {
	gc := newTestGCClearBoard(5)

	gc.onPlayerDefeated(2)
	if gc.Wins[0] != 1 || gc.Wins[1] != 0 {
		t.Fatalf("wins after p2 defeat: got %v want [1 0]", gc.Wins)
	}
	if gc.BattlesPlayed != 1 {
		t.Fatalf("battles played: got %d want 1", gc.BattlesPlayed)
	}
}

func TestInitRound_TankPositions(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.InitRound()

	if gc.Tanks[0].X != 2 || gc.Tanks[0].Y != 10 {
		t.Fatalf("P1 position: got (%d,%d) want (2,10)", gc.Tanks[0].X, gc.Tanks[0].Y)
	}
	if gc.Tanks[1].X != 37 || gc.Tanks[1].Y != 10 {
		t.Fatalf("P2 position: got (%d,%d) want (37,10)", gc.Tanks[1].X, gc.Tanks[1].Y)
	}
}

func TestInitRound_TankDirections(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.InitRound()

	if gc.Tanks[0].Dir != DirRight {
		t.Fatalf("P1 direction: got %v want DirRight", gc.Tanks[0].Dir)
	}
	if gc.Tanks[1].Dir != DirLeft {
		t.Fatalf("P2 direction: got %v want DirLeft", gc.Tanks[1].Dir)
	}
}

func TestInitRound_ClearsMines(t *testing.T) {
	gc := newTestGCClearBoard(5)
	mine := NewMine(10, 10, 1, 0.0)
	gc.Mines = append(gc.Mines, mine)
	gc.Board.SetCellType(10, 10, CellMine)

	gc.InitRound()

	if len(gc.Mines) != 0 {
		t.Fatalf("mines should be cleared on init round: got %d", len(gc.Mines))
	}
}

func TestInitRound_ClearsShots(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Shots = append(gc.Shots, NewShot(10, 10, DirRight, 1, 1))

	gc.InitRound()

	if len(gc.Shots) != 0 {
		t.Fatalf("shots should be cleared on init round: got %d", len(gc.Shots))
	}
}
