package engine

import "testing"

func TestCellType_Values(t *testing.T) {
	if CellEmpty != 1 || CellWall != 2 || CellTank1 != 3 || CellTank2 != 4 || CellBarrel1 != 5 || CellBarrel2 != 6 || CellShot != 7 || CellMine != 8 || CellWreckageP1 != 9 || CellWreckageP2 != 10 {
		t.Fatalf("unexpected CellType values: %v %v %v %v %v %v %v %v %v %v", CellEmpty, CellWall, CellTank1, CellTank2, CellBarrel1, CellBarrel2, CellShot, CellMine, CellWreckageP1, CellWreckageP2)
	}
}

func TestDirection_Values(t *testing.T) {
	if DirUp != 1 || DirDown != 2 || DirLeft != 3 || DirRight != 4 || DirUpLeft != 5 || DirUpRight != 6 || DirDownLeft != 7 || DirDownRight != 8 {
		t.Fatalf("unexpected Direction values: %v %v %v %v %v %v %v %v", DirUp, DirDown, DirLeft, DirRight, DirUpLeft, DirUpRight, DirDownLeft, DirDownRight)
	}
}

func TestDirectionVectors(t *testing.T) {
	cases := map[Direction][2]int{
		DirUp:        {0, -1},
		DirDown:      {0, 1},
		DirLeft:      {-1, 0},
		DirRight:     {1, 0},
		DirUpLeft:    {-1, -1},
		DirUpRight:   {1, -1},
		DirDownLeft:  {-1, 1},
		DirDownRight: {1, 1},
	}
	for dir, want := range cases {
		got, ok := DirectionVectors[dir]
		if !ok {
			t.Fatalf("missing vector for %v", dir)
		}
		if got != want {
			t.Fatalf("direction %v: got %v want %v", dir, got, want)
		}
	}
}

func TestDifficultyToResources(t *testing.T) {
	cases := []struct {
		level        int
		tanks, shots int
		mines        int
	}{
		{0, 3, 6, 0},
		{1, 3, 6, 1},
		{2, 3, 8, 1},
		{3, 3, 8, 1},
		{4, 3, 8, 2},
		{5, 3, 10, 2},
		{6, 3, 10, 2},
		{7, 3, 10, 3},
		{8, 3, 12, 3},
		{9, 3, 12, 3},
	}
	for _, tc := range cases {
		t.Run("level", func(t *testing.T) {
			tanks, shots, mines := DifficultyToResources(tc.level)
			if tanks != tc.tanks || shots != tc.shots || mines != tc.mines {
				t.Fatalf("level %d: got (%d,%d,%d) want (%d,%d,%d)", tc.level, tanks, shots, mines, tc.tanks, tc.shots, tc.mines)
			}
		})
	}
}

func TestMoveDelay(t *testing.T) {
	const eps = 1e-9
	if got := MoveDelay(0); absDiff(got, 0.3) > eps {
		t.Fatalf("level 0: got %v want 0.3", got)
	}
	if got := MoveDelay(9); absDiff(got, 0.2) > eps {
		t.Fatalf("level 9: got %v want 0.2", got)
	}
	if got := MoveDelay(4); absDiff(got, 0.3-((4.0/9.0)*0.1)) > eps {
		t.Fatalf("level 4: got %v want %v", got, 0.3-((4.0/9.0)*0.1))
	}
	if got := MoveDelay(-5); absDiff(got, 0.3) > eps {
		t.Fatalf("clamp low: got %v want 0.3", got)
	}
	if got := MoveDelay(50); absDiff(got, 0.2) > eps {
		t.Fatalf("clamp high: got %v want 0.2", got)
	}
}

func TestShotDelay(t *testing.T) {
	if got := GetShotDelay(); got != ShotDelay {
		t.Fatalf("got %v want %v", got, ShotDelay)
	}
	if ShotDelay != 0.1 {
		t.Fatalf("got %v want 0.1", ShotDelay)
	}
}

func TestBoardConstants(t *testing.T) {
	if BoardWidth != 40 || BoardHeight != 21 {
		t.Fatalf("got board %dx%d want 40x21", BoardWidth, BoardHeight)
	}
}

func TestActionToDirection(t *testing.T) {
	cases := []struct {
		a    Action
		want Direction
		ok   bool
	}{
		{ActionNone, 0, false},
		{ActionUp, DirUp, true},
		{ActionDown, DirDown, true},
		{ActionLeft, DirLeft, true},
		{ActionRight, DirRight, true},
		{ActionUpLeft, DirUpLeft, true},
		{ActionUpRight, DirUpRight, true},
		{ActionDownLeft, DirDownLeft, true},
		{ActionDownRight, DirDownRight, true},
		{ActionFire, 0, false},
		{ActionPlaceMine, 0, false},
	}
	for _, tc := range cases {
		got, ok := ActionToDirection(tc.a)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("action %v: got (%v,%v) want (%v,%v)", tc.a, got, ok, tc.want, tc.ok)
		}
	}
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
