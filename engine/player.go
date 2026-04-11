package engine

type Tank struct {
	PlayerID  int
	X, Y      int
	Dir       Direction
	Lives     int
	ShotsLeft int
	MinesLeft int
	StartX    int
	StartY    int
	Active    bool
}

func CellTypeForTank(pid int) CellType {
	if pid == 1 {
		return CellTank1
	}
	return CellTank2
}

func CellTypeForBarrel(pid int) CellType {
	if pid == 1 {
		return CellBarrel1
	}
	return CellBarrel2
}

func (t *Tank) BarrelPos() (int, int) {
	v := DirectionVectors[t.Dir]
	return t.X + v[0], t.Y + v[1]
}

func (t *Tank) OccupyBoard(b *Board) {
	b.SetCellType(t.X, t.Y, CellTypeForTank(t.PlayerID))
	bx, by := t.BarrelPos()
	if b.InBounds(bx, by) && b.GetCell(bx, by) == CellEmpty {
		b.SetCellType(bx, by, CellTypeForBarrel(t.PlayerID))
	}
}

func (t *Tank) ClearFromBoard(b *Board) {
	bx, by := t.BarrelPos()
	if b.InBounds(bx, by) && b.GetCell(bx, by) == CellTypeForBarrel(t.PlayerID) {
		b.SetCellType(bx, by, CellEmpty)
	}
	if b.GetCell(t.X, t.Y) == CellTypeForTank(t.PlayerID) {
		b.SetCellType(t.X, t.Y, CellEmpty)
	}
}

func (t *Tank) AttemptMove(b *Board, dir Direction) bool {
	v := DirectionVectors[dir]
	nx := t.X + v[0]
	ny := t.Y + v[1]

	if nx < 1 {
		nx = 1
	} else if nx > b.Width-2 {
		nx = b.Width - 2
	}
	if ny < 1 {
		ny = 1
	} else if ny > b.Height-2 {
		ny = b.Height - 2
	}

	if nx == t.X && ny == t.Y {
		return false
	}

	target := b.GetCell(nx, ny)
	ownBarrel := CellTypeForBarrel(t.PlayerID)
	if target != CellEmpty && target != CellShot && target != CellMine && target != ownBarrel {
		return false
	}

	t.ClearFromBoard(b)
	t.X = nx
	t.Y = ny
	t.Dir = dir
	t.OccupyBoard(b)
	return true
}

func (t *Tank) CanFire() bool {
	return t.ShotsLeft > 0
}

func (t *Tank) ConsumeShot() {
	if t.ShotsLeft > 0 {
		t.ShotsLeft--
	}
}

func (t *Tank) CanPlaceMine() bool {
	return t.MinesLeft > 0
}

func (t *Tank) ConsumeMine() {
	if t.MinesLeft > 0 {
		t.MinesLeft--
	}
}

func (t *Tank) TakeDamage() {
	if t.Lives > 0 {
		t.Lives--
	}
}

func (t *Tank) IsAlive() bool {
	return t.Lives > 0
}
