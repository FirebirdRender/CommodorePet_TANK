package engine

import "math/rand"

type Explosion struct {
	X, Y            int
	StartTime       float64
	Duration        float64
	IsChainReaction bool
}

type BarrelWreckageEntry struct {
	Pos [2]int
	Dir Direction
}

type GameController struct {
	Board      *Board
	Tanks      [2]*Tank
	Shots      []*Shot
	Mines      []*Mine
	Explosions []*Explosion

	BarrelWreckageRegistry []BarrelWreckageEntry
	BarrelHitBodies        map[[2]int]bool

	Difficulty int
	SimTime    float64
}

func NewGameController(difficulty int, rng *rand.Rand) *GameController {
	lives, shots, mines := DifficultyToResources(difficulty)
	board := NewBoard(BoardWidth, BoardHeight, difficulty, rng)

	t1 := &Tank{
		PlayerID:  1,
		X:         2,
		Y:         10,
		Dir:       DirRight,
		Lives:     lives,
		ShotsLeft: shots,
		MinesLeft: mines,
		StartX:    2,
		StartY:    10,
		Active:    true,
	}
	t2 := &Tank{
		PlayerID:  2,
		X:         BoardWidth - 3,
		Y:         10,
		Dir:       DirLeft,
		Lives:     lives,
		ShotsLeft: shots,
		MinesLeft: mines,
		StartX:    BoardWidth - 3,
		StartY:    10,
		Active:    true,
	}

	t1.OccupyBoard(board)
	t2.OccupyBoard(board)

	return &GameController{
		Board:                  board,
		Tanks:                  [2]*Tank{t1, t2},
		Shots:                  []*Shot{},
		Mines:                  []*Mine{},
		Explosions:             []*Explosion{},
		BarrelWreckageRegistry: []BarrelWreckageEntry{},
		BarrelHitBodies:        make(map[[2]int]bool),
		Difficulty:             difficulty,
	}
}

func (gc *GameController) PlaceDiceWreckage(cx, cy, playerID int) {
	wreckageType := CellWreckageP1
	if playerID != 1 {
		wreckageType = CellWreckageP2
	}

	corners := [][2]int{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}}
	for _, c := range corners {
		x, y := cx+c[0], cy+c[1]
		if !gc.Board.InBounds(x, y) {
			continue
		}

		ct := gc.Board.GetCell(x, y)
		switch ct {
		case CellEmpty, CellWall, CellShot, CellMine, CellBarrel1, CellBarrel2:
			gc.Board.SetCellType(x, y, wreckageType)
		}
	}
}

func (gc *GameController) TankHit(tank *Tank, hitPos [2]int) {
	tank.ClearFromBoard(gc.Board)
	gc.PlaceDiceWreckage(hitPos[0], hitPos[1], tank.PlayerID)
	tank.TakeDamage()

	gc.Explosions = append(gc.Explosions, &Explosion{
		X:               hitPos[0],
		Y:               hitPos[1],
		StartTime:       gc.SimTime,
		Duration:        ExplosionDuration,
		IsChainReaction: false,
	})

	if !tank.IsAlive() {
		gc.onPlayerDefeated(tank.PlayerID)
	}
	if tank.IsAlive() {
		gc.respawnBothTanks()
	}
}

func (gc *GameController) BarrelHit(tank *Tank, barrelPos [2]int) {
	bodyPos := [2]int{tank.X, tank.Y}
	barrelDir := tank.Dir

	wreckageType := CellWreckageP1
	if tank.PlayerID != 1 {
		wreckageType = CellWreckageP2
	}

	tank.ClearFromBoard(gc.Board)
	if gc.Board.InBounds(bodyPos[0], bodyPos[1]) {
		gc.Board.SetCellType(bodyPos[0], bodyPos[1], wreckageType)
	}
	if gc.Board.InBounds(barrelPos[0], barrelPos[1]) {
		gc.Board.SetCellType(barrelPos[0], barrelPos[1], wreckageType)
	}

	gc.BarrelHitBodies[bodyPos] = true
	gc.BarrelWreckageRegistry = append(gc.BarrelWreckageRegistry, BarrelWreckageEntry{Pos: barrelPos, Dir: barrelDir})

	tank.TakeDamage()

	if !tank.IsAlive() {
		gc.onPlayerDefeated(tank.PlayerID)
	}
	if tank.IsAlive() {
		gc.respawnSingleTank(tank.PlayerID)
	}
}

func (gc *GameController) respawnBothTanks() {
	// TODO: Task 9
}

func (gc *GameController) respawnSingleTank(playerID int) {
	// TODO: Task 9
}

func (gc *GameController) onPlayerDefeated(playerID int) {
	// TODO: Task 9
}
