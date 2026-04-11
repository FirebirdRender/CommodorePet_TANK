package engine

type Shot struct {
	X, Y         int
	Dir          Direction
	Active       bool
	OwnerID      int
	MaxRange     int
	StepsTaken   int
	HitWall      bool
	StepStartX   int
	StepStartY   int
	LastMoveTime float64
	CollisionPos *[2]int
}

func NewShot(x, y int, dir Direction, ownerID, maxRange int) *Shot {
	if maxRange <= 0 {
		maxRange = 30
	}
	return &Shot{
		X:        x,
		Y:        y,
		Dir:      dir,
		Active:   true,
		OwnerID:  ownerID,
		MaxRange: maxRange,
	}
}

func (s *Shot) isOwnCell(ct CellType) bool {
	return ct == CellTypeForTank(s.OwnerID) || ct == CellTypeForBarrel(s.OwnerID)
}

func (s *Shot) Step(b *Board) *[2]int {
	if !s.Active {
		return nil
	}

	s.StepStartX, s.StepStartY = s.X, s.Y

	if s.StepsTaken >= s.MaxRange {
		s.Active = false
		return nil
	}

	currentCell := b.GetCell(s.X, s.Y)
	if (currentCell == CellTank1 || currentCell == CellTank2 || currentCell == CellBarrel1 || currentCell == CellBarrel2) && !s.isOwnCell(currentCell) {
		s.Active = false
		return &[2]int{s.X, s.Y}
	}

	v := DirectionVectors[s.Dir]
	nx, ny := s.X+v[0], s.Y+v[1]

	if !b.InBounds(nx, ny) {
		s.Active = false
		return &[2]int{s.X, s.Y}
	}

	ct := b.GetCell(nx, ny)
	switch ct {
	case CellWall:
		s.Active = false
		b.SetCellType(nx, ny, CellEmpty)
		s.HitWall = true
		s.StepsTaken++
		return &[2]int{nx, ny}
	case CellWreckageP1, CellWreckageP2:
		s.Active = false
		b.SetCellType(nx, ny, CellEmpty)
		s.StepsTaken++
		return &[2]int{nx, ny}
	case CellMine:
		s.Active = false
		s.StepsTaken++
		return &[2]int{nx, ny}
	case CellTank1, CellTank2, CellBarrel1, CellBarrel2:
		if !s.isOwnCell(ct) {
			s.Active = false
			s.StepsTaken++
			return &[2]int{nx, ny}
		}
		s.X, s.Y = nx, ny
		s.StepsTaken++
		return nil
	case CellEmpty, CellShot:
		s.X, s.Y = nx, ny
		s.StepsTaken++
		return nil
	default:
		s.X, s.Y = nx, ny
		s.StepsTaken++
		return nil
	}
}

type Mine struct {
	X, Y             int
	OwnerID          int
	Active           bool
	Visible          bool
	VisibleStartTime float64
	PlacedTime       float64
}

func NewMine(x, y, ownerID int, placedTime float64) *Mine {
	return &Mine{
		X:                x,
		Y:                y,
		OwnerID:          ownerID,
		Active:           true,
		Visible:          true,
		VisibleStartTime: placedTime,
		PlacedTime:       placedTime,
	}
}

func (m *Mine) UpdateVisibility(simTime float64) {
	if m.Visible && (simTime-m.VisibleStartTime) >= MineVisibleDuration {
		m.Visible = false
	}
}

func PlaceMine(t *Tank, b *Board, simTime float64) *Mine {
	if !t.CanPlaceMine() {
		return nil
	}

	m := NewMine(t.X, t.Y, t.PlayerID, simTime)
	b.SetCellType(t.X, t.Y, CellMine)
	t.ConsumeMine()
	return m
}
