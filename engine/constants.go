package engine

type CellType int

const (
	CellEmpty CellType = iota + 1
	CellWall
	CellTank1
	CellTank2
	CellBarrel1
	CellBarrel2
	CellShot
	CellMine
	CellWreckageP1
	CellWreckageP2
)

type Direction int

const (
	DirUp Direction = iota + 1
	DirDown
	DirLeft
	DirRight
	DirUpLeft
	DirUpRight
	DirDownLeft
	DirDownRight
)

var DirectionVectors = map[Direction][2]int{
	DirUp:        {0, -1},
	DirDown:      {0, 1},
	DirLeft:      {-1, 0},
	DirRight:     {1, 0},
	DirUpLeft:    {-1, -1},
	DirUpRight:   {1, -1},
	DirDownLeft:  {-1, 1},
	DirDownRight: {1, 1},
}

type GameState int

const (
	StateMenu GameState = iota
	StateSkillSelect
	StateGameInit
	StatePlaying
	StateExplosion
	StateGameOver
	StatePlayAgain
	StateQuit
)

const (
	BoardWidth          = 40
	BoardHeight         = 21
	CellSize            = 20
	StatusBarHeight     = 40
	ExplosionDuration   = 0.5
	MineDetonationDelay = 0.1
	MoveDelayBase       = 0.3
	MoveDelayMin        = 0.2
	ShotDelay           = 0.1
	MineVisibleDuration = 2.0
	FPS                 = 60
)

type Action int

const (
	ActionNone Action = iota
	ActionUp
	ActionDown
	ActionLeft
	ActionRight
	ActionUpLeft
	ActionUpRight
	ActionDownLeft
	ActionDownRight
	ActionFire
	ActionPlaceMine
)

func MoveDelay(difficulty int) float64 {
	if difficulty < 0 {
		difficulty = 0
	} else if difficulty > 9 {
		difficulty = 9
	}
	return MoveDelayBase - (float64(difficulty)/9.0)*(MoveDelayBase-MoveDelayMin)
}

func GetShotDelay() float64 {
	return ShotDelay
}

func DifficultyToResources(level int) (tanks, shots, mines int) {
	tanks = 3
	switch {
	case level <= 1:
		shots = 6
	case level <= 4:
		shots = 8
	case level <= 7:
		shots = 10
	default:
		shots = 12
	}
	switch {
	case level <= 0:
		mines = 0
	case level <= 3:
		mines = 1
	case level <= 6:
		mines = 2
	default:
		mines = 3
	}
	return tanks, shots, mines
}

func ActionToDirection(a Action) (Direction, bool) {
	switch a {
	case ActionUp:
		return DirUp, true
	case ActionDown:
		return DirDown, true
	case ActionLeft:
		return DirLeft, true
	case ActionRight:
		return DirRight, true
	case ActionUpLeft:
		return DirUpLeft, true
	case ActionUpRight:
		return DirUpRight, true
	case ActionDownLeft:
		return DirDownLeft, true
	case ActionDownRight:
		return DirDownRight, true
	default:
		return 0, false
	}
}
