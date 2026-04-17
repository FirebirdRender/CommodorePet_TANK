package engine

import "testing"

func TestExplodeMine_DestroysWalls(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(10, 10, CellMine)
	gc.Board.SetCellType(11, 10, CellWall)

	gc.ExplodeMine(10, 10, 1, false)

	if got := gc.Board.GetCell(11, 10); got != CellEmpty {
		t.Fatalf("wall should be destroyed: got %v want CellEmpty", got)
	}
}

func TestExplodeMine_ClearsWreckage(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(10, 10, CellMine)
	gc.Board.SetCellType(9, 10, CellWreckageP1)
	gc.Board.SetCellType(11, 10, CellWreckageP2)

	gc.ExplodeMine(10, 10, 1, false)

	if got := gc.Board.GetCell(9, 10); got != CellEmpty {
		t.Fatalf("wreckage p1 should be cleared: got %v want CellEmpty", got)
	}
	if got := gc.Board.GetCell(11, 10); got != CellEmpty {
		t.Fatalf("wreckage p2 should be cleared: got %v want CellEmpty", got)
	}
}

func TestExplodeMine_DamagesTankBody(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tk := gc.Tanks[0]
	tk.ClearFromBoard(gc.Board)
	tk.X, tk.Y, tk.Dir = 12, 10, DirRight
	tk.OccupyBoard(gc.Board)
	gc.Board.SetCellType(10, 10, CellMine)

	gc.ExplodeMine(10, 10, 2, false)

	if tk.Lives != 2 {
		t.Fatalf("tank should take damage: got %d want 2", tk.Lives)
	}
}

func TestExplodeMine_PassesBarrel(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(10, 10, CellMine)
	gc.Board.SetCellType(11, 10, CellBarrel1)

	gc.ExplodeMine(10, 10, 1, false)

	if got := gc.Board.GetCell(11, 10); got != CellBarrel1 {
		t.Fatalf("barrel should be unchanged: got %v want CellBarrel1", got)
	}
}

func TestExplodeMine_ClearsShot(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(10, 10, CellMine)
	gc.Board.SetCellType(11, 10, CellShot)

	gc.ExplodeMine(10, 10, 1, false)

	if got := gc.Board.GetCell(11, 10); got != CellEmpty {
		t.Fatalf("shot should be cleared: got %v want CellEmpty", got)
	}
}

func TestExplodeMine_ChainReaction(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(10, 10, CellMine)
	gc.Board.SetCellType(11, 10, CellMine)
	gc.Board.SetCellType(13, 10, CellWall) // outside primary radius=1, inside chain radius=2 from (11,10)
	gc.Mines = []*Mine{
		{X: 10, Y: 10, Active: true},
		{X: 11, Y: 10, Active: true},
	}

	gc.ExplodeMine(10, 10, 1, false)

	if len(gc.Explosions) != 2 {
		t.Fatalf("expected two explosions (primary + chain): got %d", len(gc.Explosions))
	}
	if !gc.Explosions[1].IsChainReaction {
		t.Fatal("second explosion should be chain reaction")
	}
	if gc.Mines[1].Active {
		t.Fatal("chained mine should be deactivated")
	}
	if got := gc.Board.GetCell(13, 10); got != CellEmpty {
		t.Fatalf("chain radius should clear far wall: got %v want CellEmpty", got)
	}
}

func TestExplodeMine_ChainReactionTerminates(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(10, 10, CellMine)
	gc.Mines = []*Mine{{X: 10, Y: 10, Active: true}}

	gc.ExplodeMine(10, 10, 1, false)

	if len(gc.Explosions) != 1 {
		t.Fatalf("self mine should not recurse: got %d explosions want 1", len(gc.Explosions))
	}
	if got := gc.Board.GetCell(10, 10); got != CellEmpty {
		t.Fatalf("mine cell should be cleared: got %v want CellEmpty", got)
	}
}

func TestExplodeMine_RadiusDefault(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(10, 10, CellMine)

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			gc.Board.SetCellType(10+dx, 10+dy, CellWall)
		}
	}
	gc.Board.SetCellType(12, 10, CellWall) // outside radius=1

	gc.ExplodeMine(10, 10, 1, false)

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if got := gc.Board.GetCell(10+dx, 10+dy); got != CellEmpty {
				t.Fatalf("3x3 cell (%d,%d) should be empty: got %v", 10+dx, 10+dy, got)
			}
		}
	}
	if got := gc.Board.GetCell(12, 10); got != CellWall {
		t.Fatalf("outside radius should be unchanged: got %v want CellWall", got)
	}
}

func TestExplodeMine_ChainRadius(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Board.SetCellType(10, 10, CellMine)
	gc.Board.SetCellType(12, 10, CellWall) // 2 away, inside radius=2
	gc.Board.SetCellType(13, 10, CellWall) // 3 away, outside radius=2

	gc.ExplodeMine(10, 10, 2, true)

	if got := gc.Board.GetCell(12, 10); got != CellEmpty {
		t.Fatalf("within chain radius should be destroyed: got %v want CellEmpty", got)
	}
	if got := gc.Board.GetCell(13, 10); got != CellWall {
		t.Fatalf("outside chain radius should remain: got %v want CellWall", got)
	}
}

func TestExplodeMine_DurationNormal(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.SimTime = 7.25
	gc.Board.SetCellType(10, 10, CellMine)

	gc.ExplodeMine(10, 10, 1, false)

	if len(gc.Explosions) != 1 {
		t.Fatalf("explosion count: got %d want 1", len(gc.Explosions))
	}
	ex := gc.Explosions[0]
	if ex.Duration != ExplosionDuration {
		t.Fatalf("normal duration: got %v want %v", ex.Duration, ExplosionDuration)
	}
	if ex.StartTime != 7.25 {
		t.Fatalf("start time: got %v want 7.25", ex.StartTime)
	}
	if ex.IsChainReaction {
		t.Fatal("normal explosion should not be chain")
	}
}

func TestExplodeMine_DurationChain(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.SimTime = 9.5
	gc.Board.SetCellType(10, 10, CellMine)

	gc.ExplodeMine(10, 10, 2, true)

	if len(gc.Explosions) != 1 {
		t.Fatalf("explosion count: got %d want 1", len(gc.Explosions))
	}
	ex := gc.Explosions[0]
	if ex.Duration != ChainExplosionDuration {
		t.Fatalf("chain duration: got %v want %v", ex.Duration, ChainExplosionDuration)
	}
	if !ex.IsChainReaction {
		t.Fatal("chain explosion should be marked chain")
	}
}

func TestResolveTankMineCollision_MineUnderTank(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tk := gc.Tanks[0]
	tk.ClearFromBoard(gc.Board)
	tk.X, tk.Y, tk.Dir = 20, 10, DirRight
	tk.OccupyBoard(gc.Board)

	gc.Board.SetCellType(20, 10, CellMine)
	// Priority 3: pre-arm by setting SimTime past ArmedTime; without this the
	// new arming gate (0.9.7.1) treats the mine as inert.
	gc.SimTime = 1.0
	gc.Mines = []*Mine{{X: 20, Y: 10, Active: true, ArmedTime: 0.5}}

	gc.ResolveTankMineCollision(tk)

	if tk.Lives != 2 {
		t.Fatalf("tank should take mine damage: got %d want 2", tk.Lives)
	}
	if gc.Mines[0].Active {
		t.Fatal("mine should be deactivated")
	}
	if got := gc.Board.GetCell(20, 10); got != CellEmpty {
		t.Fatalf("mine cell should be empty after explosion: got %v want CellEmpty", got)
	}
	if len(gc.Explosions) != 2 {
		t.Fatalf("expected TankHit explosion + mine explosion: got %d", len(gc.Explosions))
	}
}

func TestResolveTankMineCollision_NoMine(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tk := gc.Tanks[0]
	startLives := tk.Lives

	gc.SimTime = 1.0
	gc.Mines = []*Mine{{X: tk.X + 1, Y: tk.Y, Active: true, ArmedTime: 0.5}}
	gc.ResolveTankMineCollision(tk)

	if tk.Lives != startLives {
		t.Fatalf("tank lives should not change: got %d want %d", tk.Lives, startLives)
	}
	if len(gc.Explosions) != 0 {
		t.Fatalf("no collision should create no explosions: got %d", len(gc.Explosions))
	}
	if !gc.Mines[0].Active {
		t.Fatal("non-colliding mine should remain active")
	}
}

func TestResolveTankMineCollision_TankHitFirst(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tk := gc.Tanks[0]
	tk.ClearFromBoard(gc.Board)
	tk.X, tk.Y, tk.Dir = 20, 10, DirRight
	tk.OccupyBoard(gc.Board)

	gc.Board.SetCellType(20, 10, CellMine)
	gc.SimTime = 1.0
	gc.Mines = []*Mine{{X: 20, Y: 10, Active: true, ArmedTime: 0.5}}

	gc.ResolveTankMineCollision(tk)

	// TankHit places dice wreckage corners first; mine explosion then clears them.
	corners := [][2]int{{19, 9}, {21, 9}, {19, 11}, {21, 11}}
	for _, p := range corners {
		if got := gc.Board.GetCell(p[0], p[1]); got != CellEmpty {
			t.Fatalf("corner %v should be cleared by post-hit explosion (hit-first order): got %v want CellEmpty", p, got)
		}
	}
}

func TestMine_NoExplodeOnLayer(t *testing.T) {
	gc := newTestGCClearBoard(5)
	tk := gc.Tanks[0]
	startLives := tk.Lives

	m := NewMine(tk.X, tk.Y, tk.PlayerID, 0)
	gc.Mines = []*Mine{m}
	gc.Board.SetCellType(tk.X, tk.Y, CellMine)

	gc.ResolveTankMineCollision(tk)

	if tk.Lives != startLives {
		t.Fatalf("freshly placed mine must not explode under layer: lives=%d want %d", tk.Lives, startLives)
	}
	if !m.Active {
		t.Fatal("inert mine should remain active (placed but not yet armed)")
	}
}

func TestMine_ArmsAfterDelayOnceLayerLeaves(t *testing.T) {
	gc := newTestGCClearBoard(5)
	owner := gc.Tanks[0]
	mineX, mineY := owner.X, owner.Y
	m := NewMine(mineX, mineY, owner.PlayerID, 0)
	gc.Mines = []*Mine{m}
	gc.Board.SetCellType(mineX, mineY, CellMine)

	owner.ClearFromBoard(gc.Board)
	owner.X, owner.Y = mineX+5, mineY
	owner.OccupyBoard(gc.Board)

	gc.Update(1.0 / FPS)
	if m.ArmedTime == 0 {
		t.Fatal("ArmedTime should be set on first tick after layer leaves")
	}
	if m.IsArmed(gc.SimTime) {
		t.Fatal("mine should still be inert before MineArmDelay elapses")
	}

	for i := 0; i < int(FPS*MineArmDelay)+5; i++ {
		gc.Update(1.0 / FPS)
	}
	if !m.IsArmed(gc.SimTime) {
		t.Fatalf("mine should be armed once SimTime passes ArmedTime: armed_at=%v sim=%v", m.ArmedTime, gc.SimTime)
	}
}

func TestMine_StaysDisarmedWhileLayerOnCell(t *testing.T) {
	gc := newTestGCClearBoard(5)
	owner := gc.Tanks[0]
	m := NewMine(owner.X, owner.Y, owner.PlayerID, 0)
	gc.Mines = []*Mine{m}
	gc.Board.SetCellType(owner.X, owner.Y, CellMine)

	for i := 0; i < 300; i++ {
		gc.Update(1.0 / FPS)
	}

	if m.ArmedTime != 0 {
		t.Fatalf("ArmedTime must remain 0 while layer stays on cell: got %v", m.ArmedTime)
	}
	if m.IsArmed(gc.SimTime) {
		t.Fatal("mine must not be armed while layer is still on the cell")
	}
}

func TestMine_InvisibleAfterArming(t *testing.T) {
	gc := newTestGCClearBoard(5)
	owner := gc.Tanks[0]
	mineX, mineY := owner.X, owner.Y
	m := NewMine(mineX, mineY, owner.PlayerID, 0)
	gc.Mines = []*Mine{m}
	gc.Board.SetCellType(mineX, mineY, CellMine)

	owner.ClearFromBoard(gc.Board)
	owner.X, owner.Y = mineX+5, mineY
	owner.OccupyBoard(gc.Board)

	for i := 0; i < int(FPS*(MineArmDelay+1.0)); i++ {
		gc.Update(1.0 / FPS)
	}

	if !m.IsArmed(gc.SimTime) {
		t.Fatalf("mine should be armed: ArmedTime=%v SimTime=%v", m.ArmedTime, gc.SimTime)
	}
	if m.Visible {
		t.Fatal("armed mine must be invisible")
	}
	if got := gc.Board.GetCell(mineX, mineY); got == CellMine {
		t.Fatalf("armed mine must not be marked as CellMine on board: got %v", got)
	}
}
