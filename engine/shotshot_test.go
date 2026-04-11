package engine

import "testing"

func testShot(x, y, sx, sy int, dir Direction, owner int, active bool) *Shot {
	return &Shot{
		X:          x,
		Y:          y,
		StepStartX: sx,
		StepStartY: sy,
		Dir:        dir,
		OwnerID:    owner,
		Active:     active,
	}
}

func TestShotsCrossedHeadOn_CardinalOpposite(t *testing.T) {
	a := testShot(6, 5, 5, 5, DirRight, 1, true)
	b := testShot(5, 5, 6, 5, DirLeft, 2, true)

	if !ShotsCrossedHeadOn(a, b) {
		t.Fatal("expected head-on crossed detection")
	}
}

func TestShotsCrossedHeadOn_CardinalSameDir(t *testing.T) {
	a := testShot(6, 5, 5, 5, DirRight, 1, true)
	b := testShot(7, 5, 6, 5, DirRight, 2, true)

	if ShotsCrossedHeadOn(a, b) {
		t.Fatal("expected false for same-direction cardinal shots")
	}
}

func TestShotsCrossedHeadOn_Diagonal(t *testing.T) {
	a := testShot(6, 6, 5, 5, DirDownRight, 1, true)
	b := testShot(5, 5, 6, 6, DirUpLeft, 2, true)

	if ShotsCrossedHeadOn(a, b) {
		t.Fatal("expected false for diagonal shots")
	}
}

func TestShotsCrossedHeadOn_NotSwapped(t *testing.T) {
	a := testShot(5, 5, 5, 5, DirRight, 1, true)
	b := testShot(6, 5, 6, 5, DirLeft, 2, true)

	if ShotsCrossedHeadOn(a, b) {
		t.Fatal("expected false when cardinal opposite shots did not swap")
	}
}

func TestShotsCrossedHeadOn_Inactive(t *testing.T) {
	a := testShot(6, 5, 5, 5, DirRight, 1, false)
	b := testShot(5, 5, 6, 5, DirLeft, 2, true)

	if ShotsCrossedHeadOn(a, b) {
		t.Fatal("expected false when one shot is inactive")
	}
}

func TestShotShotExplosionKey_Crossed_Horizontal(t *testing.T) {
	s1 := testShot(6, 5, 5, 5, DirRight, 1, true)
	s2 := testShot(5, 5, 6, 5, DirLeft, 2, true)

	got := ShotShotExplosionKey(s1, s2, true)
	want := [2]int{5, 5}
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestShotShotExplosionKey_Crossed_Vertical(t *testing.T) {
	s1 := testShot(5, 6, 5, 5, DirDown, 1, true)
	s2 := testShot(5, 5, 5, 6, DirUp, 2, true)

	got := ShotShotExplosionKey(s1, s2, true)
	want := [2]int{5, 5}
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestShotShotExplosionKey_SameCell(t *testing.T) {
	s1 := testShot(8, 9, 7, 9, DirRight, 1, true)
	s2 := testShot(8, 9, 9, 9, DirLeft, 2, true)

	got := ShotShotExplosionKey(s1, s2, false)
	want := [2]int{8, 9}
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDetectShotShot_SameCell(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Shots = []*Shot{
		testShot(10, 10, 9, 10, DirRight, 1, true),
		testShot(10, 10, 11, 10, DirLeft, 2, true),
	}

	got := DetectShotShotCollisions(gc)
	if len(got) != 1 || got[0] != [2]int{0, 1} {
		t.Fatalf("got %v want [[0 1]]", got)
	}
}

func TestDetectShotShot_HeadOnSwap(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Shots = []*Shot{
		testShot(6, 5, 5, 5, DirRight, 1, true),
		testShot(5, 5, 6, 5, DirLeft, 2, true),
	}

	got := DetectShotShotCollisions(gc)
	if len(got) != 1 || got[0] != [2]int{0, 1} {
		t.Fatalf("got %v want [[0 1]]", got)
	}
}

func TestDetectShotShot_MissNearby(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.Shots = []*Shot{
		testShot(6, 5, 5, 5, DirRight, 1, true),
		testShot(8, 5, 9, 5, DirLeft, 2, true),
	}

	got := DetectShotShotCollisions(gc)
	if len(got) != 0 {
		t.Fatalf("got %v want no collisions", got)
	}
}

func TestResolveShotShot_BothDeactivated(t *testing.T) {
	gc := newTestGCClearBoard(5)
	gc.SimTime = 12.5
	s1 := testShot(6, 5, 5, 5, DirRight, 1, true)
	s2 := testShot(5, 5, 6, 5, DirLeft, 2, true)
	gc.Shots = []*Shot{s1, s2}

	ResolveShotShotCollision(gc, 0, 1)

	if s1.Active || s2.Active {
		t.Fatalf("shots should be inactive after resolve: s1=%v s2=%v", s1.Active, s2.Active)
	}
	if len(gc.Explosions) != 1 {
		t.Fatalf("explosions len: got %d want 1", len(gc.Explosions))
	}
	ex := gc.Explosions[0]
	if ex.X != 5 || ex.Y != 5 {
		t.Fatalf("explosion pos: got (%d,%d) want (5,5)", ex.X, ex.Y)
	}
	if ex.StartTime != gc.SimTime {
		t.Fatalf("explosion StartTime: got %v want %v", ex.StartTime, gc.SimTime)
	}
	if ex.Duration != ChainExplosionDuration {
		t.Fatalf("explosion Duration: got %v want %v", ex.Duration, ChainExplosionDuration)
	}
	if !ex.IsChainReaction {
		t.Fatal("explosion should be chain reaction")
	}
}
