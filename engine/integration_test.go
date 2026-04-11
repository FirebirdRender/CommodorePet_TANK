package engine

import (
	"math/rand"
	"testing"
)

func setupGame(t *testing.T, difficulty int, seed int64) *GameController {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	gc := NewGameController(difficulty, rng)
	gc.InitRound()
	clearInterior(gc.Board)
	for _, tank := range gc.Tanks {
		tank.OccupyBoard(gc.Board)
	}
	return gc
}

func stepTicks(gc *GameController, n int) {
	dt := 1.0 / float64(FPS)
	for i := 0; i < n; i++ {
		gc.Update(dt)
	}
}

func runUntil(gc *GameController, maxTicks int, cond func() bool) bool {
	for i := 0; i < maxTicks; i++ {
		if cond() {
			return true
		}
		stepTicks(gc, 1)
	}
	return cond()
}

func setTankPose(gc *GameController, playerID, x, y int, dir Direction) {
	tk := gc.Tanks[playerID-1]
	tk.ClearFromBoard(gc.Board)
	tk.X = x
	tk.Y = y
	tk.Dir = dir
	tk.OccupyBoard(gc.Board)
}

func findExplosion(explosions []*Explosion, x, y int) *Explosion {
	for _, ex := range explosions {
		if ex.X == x && ex.Y == y {
			return ex
		}
	}
	return nil
}

func TestGolden_ShotWall(t *testing.T) {
	gc := setupGame(t, 0, 42)
	gc.Board.SetCellType(5, 10, CellWall)

	gc.ApplyInput(1, ActionFire)
	ok := runUntil(gc, 200, func() bool {
		return gc.Board.GetCell(5, 10) == CellEmpty && len(gc.Shots) == 0
	})
	if !ok {
		t.Fatal("shot-wall interaction did not resolve within tick budget")
	}

	if got := gc.Board.GetCell(5, 10); got != CellEmpty {
		t.Fatalf("wall should be destroyed by shot: got %v want CellEmpty", got)
	}
	if len(gc.Shots) != 0 {
		t.Fatalf("shot should be deactivated and removed: got %d active entries", len(gc.Shots))
	}
}

func TestGolden_ShotEnemyTankBody(t *testing.T) {
	gc := setupGame(t, 0, 42)
	setTankPose(gc, 2, 12, 10, DirUp)
	startLives := gc.Tanks[1].Lives

	gc.ApplyInput(1, ActionFire)
	ok := runUntil(gc, 300, func() bool {
		return gc.Tanks[1].Lives == startLives-1
	})
	if !ok {
		t.Fatal("shot-body collision did not resolve within tick budget")
	}

	if got, want := gc.Tanks[1].Lives, startLives-1; got != want {
		t.Fatalf("P2 should lose one life on body hit: got %d want %d", got, want)
	}
	if gc.Tanks[0].X != gc.Tanks[0].StartX || gc.Tanks[0].Y != gc.Tanks[0].StartY {
		t.Fatalf("both-tank respawn should reset P1 to start, got (%d,%d)", gc.Tanks[0].X, gc.Tanks[0].Y)
	}
	if gc.Tanks[1].X != gc.Tanks[1].StartX || gc.Tanks[1].Y != gc.Tanks[1].StartY {
		t.Fatalf("both-tank respawn should reset P2 to start, got (%d,%d)", gc.Tanks[1].X, gc.Tanks[1].Y)
	}
}

func TestGolden_ShotEnemyBarrel(t *testing.T) {
	gc := setupGame(t, 0, 42)
	setTankPose(gc, 2, 12, 10, DirLeft)
	startLives := gc.Tanks[1].Lives

	gc.ApplyInput(1, ActionFire)
	ok := runUntil(gc, 300, func() bool {
		return gc.Tanks[1].Lives == startLives-1
	})
	if !ok {
		t.Fatal("shot-barrel collision did not resolve within tick budget")
	}

	if got, want := gc.Tanks[1].Lives, startLives-1; got != want {
		t.Fatalf("P2 should lose one life on barrel hit: got %d want %d", got, want)
	}
	if gc.Tanks[1].X != gc.Tanks[1].StartX || gc.Tanks[1].Y != gc.Tanks[1].StartY {
		t.Fatalf("barrel-hit victim should respawn at start, got (%d,%d)", gc.Tanks[1].X, gc.Tanks[1].Y)
	}
	if gc.Tanks[0].X != 2 || gc.Tanks[0].Y != 10 {
		t.Fatalf("non-victim tank should remain in place, got (%d,%d)", gc.Tanks[0].X, gc.Tanks[0].Y)
	}
}

func TestGolden_ShotMine(t *testing.T) {
	gc := setupGame(t, 5, 42)
	setTankPose(gc, 2, 9, 10, DirLeft)

	gc.ApplyInput(2, ActionPlaceMine)
	if len(gc.Mines) != 1 {
		t.Fatalf("expected one mine after placement, got %d", len(gc.Mines))
	}
	m := gc.Mines[0]
	setTankPose(gc, 2, 20, 10, DirLeft)

	gc.ApplyInput(1, ActionFire)
	ok := runUntil(gc, 300, func() bool { return !m.Active })
	if !ok {
		t.Fatal("shot-mine collision did not resolve within tick budget")
	}

	if m.Active {
		t.Fatal("mine should deactivate when hit by shot")
	}
	ex := findExplosion(gc.Explosions, m.X, m.Y)
	if ex == nil {
		t.Fatalf("expected explosion at mine position (%d,%d)", m.X, m.Y)
	}
	if !ex.IsChainReaction {
		t.Fatal("shot-triggered mine explosion should be chain reaction")
	}
	if ex.Duration != ChainExplosionDuration {
		t.Fatalf("chain explosion duration mismatch: got %v want %v", ex.Duration, ChainExplosionDuration)
	}
}

func TestGolden_ShotShotSameCell(t *testing.T) {
	gc := setupGame(t, 0, 42)
	setTankPose(gc, 2, 8, 10, DirLeft)

	gc.ApplyInput(1, ActionFire)
	gc.ApplyInput(2, ActionFire)
	ok := runUntil(gc, 200, func() bool { return len(gc.Shots) == 0 })
	if !ok {
		t.Fatal("same-cell shot-shot collision did not resolve within tick budget")
	}

	if len(gc.Shots) != 0 {
		t.Fatalf("same-cell shot collision should remove both shots, got %d", len(gc.Shots))
	}
	ex := findExplosion(gc.Explosions, 5, 10)
	if ex == nil {
		t.Fatal("expected chain explosion at same-cell collision point (5,10)")
	}
	if !ex.IsChainReaction {
		t.Fatal("shot-shot collision must create chain reaction explosion")
	}
}

func TestGolden_ShotShotHeadOn(t *testing.T) {
	gc := setupGame(t, 0, 42)
	setTankPose(gc, 2, 9, 10, DirLeft)

	gc.ApplyInput(1, ActionFire)
	gc.ApplyInput(2, ActionFire)
	ok := runUntil(gc, 200, func() bool { return len(gc.Shots) == 0 })
	if !ok {
		t.Fatal("head-on shot-shot collision did not resolve within tick budget")
	}

	if len(gc.Shots) != 0 {
		t.Fatalf("head-on shot crossing should remove both shots, got %d", len(gc.Shots))
	}
	ex := findExplosion(gc.Explosions, 5, 10)
	if ex == nil {
		t.Fatal("expected head-on chain explosion at midpoint (5,10)")
	}
	if !ex.IsChainReaction {
		t.Fatal("head-on shot collision must be chain reaction")
	}
}

func TestGolden_MineTankStepOn(t *testing.T) {
	gc := setupGame(t, 5, 42)
	setTankPose(gc, 1, 10, 10, DirUp)
	setTankPose(gc, 2, 12, 10, DirLeft)

	gc.ApplyInput(1, ActionPlaceMine)
	if len(gc.Mines) != 1 {
		t.Fatalf("expected one mine after placement, got %d", len(gc.Mines))
	}
	m := gc.Mines[0]
	startLives := gc.Tanks[1].Lives

	gc.ApplyInput(2, ActionLeft)
	stepTicks(gc, 20)
	gc.ApplyInput(2, ActionLeft)
	ok := runUntil(gc, 120, func() bool { return gc.Tanks[1].Lives == startLives-1 })
	if !ok {
		t.Fatal("tank-mine collision did not resolve within tick budget")
	}

	if got, want := gc.Tanks[1].Lives, startLives-1; got != want {
		t.Fatalf("tank stepping on mine should lose one life: got %d want %d", got, want)
	}
	if m.Active {
		t.Fatal("mine should deactivate after tank collision")
	}
	ex := findExplosion(gc.Explosions, 10, 10)
	if ex == nil {
		t.Fatal("expected explosion at mine location after step-on")
	}
	if ex.IsChainReaction {
		t.Fatal("direct tank step-on mine explosion should not be chain reaction")
	}
}

func TestGolden_MineChainReaction(t *testing.T) {
	gc := setupGame(t, 5, 42)
	setTankPose(gc, 1, 8, 10, DirRight)
	setTankPose(gc, 2, 30, 10, DirLeft)

	m1 := NewMine(10, 10, 1, gc.SimTime)
	m2 := NewMine(11, 10, 2, gc.SimTime)
	gc.Mines = []*Mine{m1, m2}
	gc.Board.SetCellType(10, 10, CellMine)
	gc.Board.SetCellType(11, 10, CellMine)

	gc.ApplyInput(1, ActionFire)
	ok := runUntil(gc, 150, func() bool { return !m1.Active && !m2.Active })
	if !ok {
		t.Fatal("mine chain reaction did not resolve within tick budget")
	}

	if m1.Active || m2.Active {
		t.Fatalf("both mines should be inactive after chain reaction: m1=%v m2=%v", m1.Active, m2.Active)
	}
	if len(gc.Explosions) < 2 {
		t.Fatalf("expected at least two explosions in chain reaction, got %d", len(gc.Explosions))
	}
	ex2 := findExplosion(gc.Explosions, 11, 10)
	if ex2 == nil {
		t.Fatal("expected secondary chain explosion at second mine position")
	}
	if !ex2.IsChainReaction {
		t.Fatal("secondary mine explosion must be marked chain reaction")
	}
}

func TestGolden_EmptyGunSelfDestruct(t *testing.T) {
	gc := setupGame(t, 0, 42)
	t1 := gc.Tanks[0]
	t1.ShotsLeft = 1
	gc.Board.SetCellType(4, 10, CellWall)

	gc.ApplyInput(1, ActionFire)
	ok := runUntil(gc, 200, func() bool { return t1.Lives == 2 })
	if !ok {
		t.Fatal("empty-gun self-destruct did not resolve within tick budget")
	}

	if got, want := t1.Lives, 2; got != want {
		t.Fatalf("empty-gun self-destruct should remove one life: got %d want %d", got, want)
	}
	if _, exists := gc.EmptyGunPending[1]; exists {
		t.Fatal("empty-gun pending flag should clear after self-destruct")
	}
}

func TestGolden_BarrelSwingThenFire(t *testing.T) {
	gc := setupGame(t, 0, 42)
	t1 := gc.Tanks[0]
	startX, startY := t1.X, t1.Y

	gc.ApplyInput(1, ActionUp)
	if t1.Dir != DirUp {
		t.Fatalf("barrel swing should update direction to up: got %v", t1.Dir)
	}
	if t1.X != startX || t1.Y != startY {
		t.Fatalf("swing should not move tank, got (%d,%d)", t1.X, t1.Y)
	}

	gc.ApplyInput(1, ActionFire)
	if len(gc.Shots) != 1 {
		t.Fatalf("expected one shot after firing, got %d", len(gc.Shots))
	}
	shot := gc.Shots[0]
	if shot.Dir != DirUp {
		t.Fatalf("shot direction should match swung barrel direction: got %v want %v", shot.Dir, DirUp)
	}
	if shot.X != 2 || shot.Y != 9 {
		t.Fatalf("shot should spawn at new barrel position (2,9), got (%d,%d)", shot.X, shot.Y)
	}
}

func TestGolden_RespawnAfterDefeat(t *testing.T) {
	gc := setupGame(t, 0, 42)
	setTankPose(gc, 2, 12, 10, DirUp)
	gc.Tanks[1].Lives = 1

	gc.ApplyInput(1, ActionFire)
	ok := runUntil(gc, 300, func() bool { return gc.BattlesPlayed == 1 })
	if !ok {
		t.Fatal("defeat/respawn flow did not resolve within tick budget")
	}

	if got, want := gc.Winner, 1; got != want {
		t.Fatalf("winner mismatch after P2 defeat: got %d want %d", got, want)
	}
	if got, want := gc.Wins[0], 1; got != want {
		t.Fatalf("P1 wins mismatch after P2 defeat: got %d want %d", got, want)
	}
	if got, want := gc.BattlesPlayed, 1; got != want {
		t.Fatalf("battles played mismatch: got %d want %d", got, want)
	}
}

func TestGolden_FullGameToCompletion(t *testing.T) {
	result := RunMatch(5, 42, 50000, NewRandomInputProvider(100))

	if result.BattlesPlayed < 1 {
		t.Fatalf("expected at least one completed battle in full match run, got %d", result.BattlesPlayed)
	}
	if result.P1Wins+result.P2Wins != result.BattlesPlayed {
		t.Fatalf("wins should sum to battles: result=%+v", result)
	}
}
