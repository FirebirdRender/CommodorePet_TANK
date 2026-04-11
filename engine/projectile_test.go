package engine

import "testing"

func TestShot_StepIntoEmpty(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	s := NewShot(10, 10, DirRight, 1, 30)
	collision := s.Step(b)

	if collision != nil {
		t.Fatalf("collision: got %v want nil", *collision)
	}
	if s.X != 11 || s.Y != 10 {
		t.Fatalf("position: got (%d,%d) want (11,10)", s.X, s.Y)
	}
	if s.StepsTaken != 1 {
		t.Fatalf("StepsTaken: got %d want 1", s.StepsTaken)
	}
	if !s.Active {
		t.Fatal("shot should remain active")
	}
}

func TestShot_StepIntoWall(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	b.SetCellType(11, 10, CellWall)

	s := NewShot(10, 10, DirRight, 1, 30)
	collision := s.Step(b)

	if collision == nil {
		t.Fatal("expected collision with wall")
	}
	if *collision != [2]int{11, 10} {
		t.Fatalf("collision: got %v want [11 10]", *collision)
	}
	if s.Active {
		t.Fatal("shot should deactivate on wall hit")
	}
	if b.GetCell(11, 10) != CellEmpty {
		t.Fatalf("wall cell should be cleared: got %v want CellEmpty", b.GetCell(11, 10))
	}
	if !s.HitWall {
		t.Fatal("HitWall should be true")
	}
	if s.StepsTaken != 1 {
		t.Fatalf("StepsTaken: got %d want 1", s.StepsTaken)
	}
}

func TestShot_StepIntoWreckage(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	b.SetCellType(11, 10, CellWreckageP1)

	s := NewShot(10, 10, DirRight, 2, 30)
	collision := s.Step(b)

	if collision == nil {
		t.Fatal("expected collision with wreckage")
	}
	if *collision != [2]int{11, 10} {
		t.Fatalf("collision: got %v want [11 10]", *collision)
	}
	if s.Active {
		t.Fatal("shot should deactivate on wreckage hit")
	}
	if b.GetCell(11, 10) != CellEmpty {
		t.Fatalf("wreckage cell should be cleared: got %v want CellEmpty", b.GetCell(11, 10))
	}
	if s.StepsTaken != 1 {
		t.Fatalf("StepsTaken: got %d want 1", s.StepsTaken)
	}
}

func TestShot_StepIntoMine(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	b.SetCellType(11, 10, CellMine)

	s := NewShot(10, 10, DirRight, 1, 30)
	collision := s.Step(b)

	if collision == nil {
		t.Fatal("expected collision with mine")
	}
	if *collision != [2]int{11, 10} {
		t.Fatalf("collision: got %v want [11 10]", *collision)
	}
	if s.Active {
		t.Fatal("shot should deactivate on mine hit")
	}
	if b.GetCell(11, 10) != CellMine {
		t.Fatalf("mine cell should remain unchanged: got %v want CellMine", b.GetCell(11, 10))
	}
	if s.StepsTaken != 1 {
		t.Fatalf("StepsTaken: got %d want 1", s.StepsTaken)
	}
}

func TestShot_StepIntoEnemyTank(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	b.SetCellType(11, 10, CellTank2)

	s := NewShot(10, 10, DirRight, 1, 30)
	collision := s.Step(b)

	if collision == nil {
		t.Fatal("expected collision with enemy tank")
	}
	if *collision != [2]int{11, 10} {
		t.Fatalf("collision: got %v want [11 10]", *collision)
	}
	if s.Active {
		t.Fatal("shot should deactivate on enemy tank")
	}
	if b.GetCell(11, 10) != CellTank2 {
		t.Fatalf("enemy tank cell should remain unchanged: got %v want CellTank2", b.GetCell(11, 10))
	}
	if s.StepsTaken != 1 {
		t.Fatalf("StepsTaken: got %d want 1", s.StepsTaken)
	}
}

func TestShot_StepIntoEnemyBarrel(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	b.SetCellType(11, 10, CellBarrel2)

	s := NewShot(10, 10, DirRight, 1, 30)
	collision := s.Step(b)

	if collision == nil {
		t.Fatal("expected collision with enemy barrel")
	}
	if *collision != [2]int{11, 10} {
		t.Fatalf("collision: got %v want [11 10]", *collision)
	}
	if s.Active {
		t.Fatal("shot should deactivate on enemy barrel")
	}
	if s.StepsTaken != 1 {
		t.Fatalf("StepsTaken: got %d want 1", s.StepsTaken)
	}
}

func TestShot_PassThroughOwnTank(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	b.SetCellType(11, 10, CellTank1)

	s := NewShot(10, 10, DirRight, 1, 30)
	collision := s.Step(b)

	if collision != nil {
		t.Fatalf("collision: got %v want nil", *collision)
	}
	if s.X != 11 || s.Y != 10 {
		t.Fatalf("position: got (%d,%d) want (11,10)", s.X, s.Y)
	}
	if !s.Active {
		t.Fatal("shot should remain active through own tank")
	}
	if s.StepsTaken != 1 {
		t.Fatalf("StepsTaken: got %d want 1", s.StepsTaken)
	}
}

func TestShot_PassThroughOwnBarrel(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	b.SetCellType(11, 10, CellBarrel1)

	s := NewShot(10, 10, DirRight, 1, 30)
	collision := s.Step(b)

	if collision != nil {
		t.Fatalf("collision: got %v want nil", *collision)
	}
	if s.X != 11 || s.Y != 10 {
		t.Fatalf("position: got (%d,%d) want (11,10)", s.X, s.Y)
	}
	if !s.Active {
		t.Fatal("shot should remain active through own barrel")
	}
	if s.StepsTaken != 1 {
		t.Fatalf("StepsTaken: got %d want 1", s.StepsTaken)
	}
}

func TestShot_RangeLimit(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	b.SetCellType(12, 10, CellWall)

	s := NewShot(10, 10, DirRight, 1, 1)
	if collision := s.Step(b); collision != nil {
		t.Fatalf("step 1 collision: got %v want nil", *collision)
	}

	if s.X != 11 || s.Y != 10 {
		t.Fatalf("after step 1 position: got (%d,%d) want (11,10)", s.X, s.Y)
	}
	if s.StepsTaken != 1 {
		t.Fatalf("after step 1 StepsTaken: got %d want 1", s.StepsTaken)
	}

	collision := s.Step(b)
	if collision != nil {
		t.Fatalf("step 2 collision: got %v want nil", *collision)
	}
	if s.Active {
		t.Fatal("shot should deactivate at range limit")
	}
	if b.GetCell(12, 10) != CellWall {
		t.Fatalf("range limit should not modify cells: got %v want CellWall", b.GetCell(12, 10))
	}
}

func TestShot_OutOfBounds(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	s := NewShot(38, 10, DirRight, 1, 30)
	s.X = 39
	collision := s.Step(b)

	if collision == nil {
		t.Fatal("expected collision when stepping out of bounds")
	}
	if *collision != [2]int{39, 10} {
		t.Fatalf("collision: got %v want [39 10]", *collision)
	}
	if s.Active {
		t.Fatal("shot should deactivate when leaving bounds")
	}
}

func TestShot_SnapshotPositions(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)

	s := NewShot(10, 10, DirRight, 1, 30)
	_ = s.Step(b)

	if s.StepStartX != 10 || s.StepStartY != 10 {
		t.Fatalf("snapshot: got (%d,%d) want (10,10)", s.StepStartX, s.StepStartY)
	}
}

func TestShot_CurrentCellEnemyHit(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	b.SetCellType(10, 10, CellTank2)

	s := NewShot(10, 10, DirRight, 1, 30)
	collision := s.Step(b)

	if collision == nil {
		t.Fatal("expected collision at current enemy cell")
	}
	if *collision != [2]int{10, 10} {
		t.Fatalf("collision: got %v want [10 10]", *collision)
	}
	if s.Active {
		t.Fatal("shot should deactivate on current-cell enemy hit")
	}
	if s.StepsTaken != 0 {
		t.Fatalf("StepsTaken: got %d want 0", s.StepsTaken)
	}
}

func TestMine_NewMine(t *testing.T) {
	m := NewMine(5, 6, 1, 12.5)

	if m.X != 5 || m.Y != 6 {
		t.Fatalf("position: got (%d,%d) want (5,6)", m.X, m.Y)
	}
	if m.OwnerID != 1 {
		t.Fatalf("OwnerID: got %d want 1", m.OwnerID)
	}
	if !m.Active {
		t.Fatal("Active: got false want true")
	}
	if !m.Visible {
		t.Fatal("Visible: got false want true")
	}
	if m.VisibleStartTime != 12.5 {
		t.Fatalf("VisibleStartTime: got %v want 12.5", m.VisibleStartTime)
	}
	if m.PlacedTime != 12.5 {
		t.Fatalf("PlacedTime: got %v want 12.5", m.PlacedTime)
	}
}

func TestMine_VisibilityTimeout(t *testing.T) {
	m := NewMine(5, 6, 1, 10.0)

	m.UpdateVisibility(11.9)
	if !m.Visible {
		t.Fatal("mine should still be visible at t=11.9")
	}

	m.UpdateVisibility(12.0)
	if m.Visible {
		t.Fatal("mine should be hidden at t=12.0")
	}
}

func TestMine_VisibilityBeforeTimeout(t *testing.T) {
	m := NewMine(5, 6, 1, 10.0)

	m.UpdateVisibility(10.5)
	if !m.Visible {
		t.Fatal("mine should remain visible before timeout")
	}
}

func TestPlaceMine_Success(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	tk := newTank(1, 10, 10, DirUp)
	tk.MinesLeft = 2

	m := PlaceMine(tk, b, 7.0)
	if m == nil {
		t.Fatal("expected mine placement to succeed")
	}
	if m.X != 10 || m.Y != 10 {
		t.Fatalf("mine position: got (%d,%d) want (10,10)", m.X, m.Y)
	}
	if m.OwnerID != 1 {
		t.Fatalf("OwnerID: got %d want 1", m.OwnerID)
	}
	if b.GetCell(10, 10) != CellMine {
		t.Fatalf("board cell: got %v want CellMine", b.GetCell(10, 10))
	}
	if tk.MinesLeft != 1 {
		t.Fatalf("MinesLeft: got %d want 1", tk.MinesLeft)
	}
}

func TestPlaceMine_NoMinesLeft(t *testing.T) {
	b := newTestBoard()
	clearInterior(b)
	tk := newTank(2, 15, 10, DirLeft)
	tk.MinesLeft = 0
	b.SetCellType(15, 10, CellTank2)

	m := PlaceMine(tk, b, 3.0)
	if m != nil {
		t.Fatalf("expected nil mine, got %+v", *m)
	}
	if b.GetCell(15, 10) != CellTank2 {
		t.Fatalf("board cell should remain unchanged: got %v want CellTank2", b.GetCell(15, 10))
	}
	if tk.MinesLeft != 0 {
		t.Fatalf("MinesLeft: got %d want 0", tk.MinesLeft)
	}
}
