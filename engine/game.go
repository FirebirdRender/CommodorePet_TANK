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
	Board         *Board
	Tanks         [2]*Tank
	Shots         []*Shot
	Mines         []*Mine
	Explosions    []*Explosion
	SwingDir      map[int]*Direction
	LastMoveTime  map[int]float64
	Winner        int
	Wins          [2]int
	BattlesPlayed int
	State         GameState
	RNG           *rand.Rand

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

	lastMoveTime := make(map[int]float64)
	lastMoveTime[1] = -MoveDelay(difficulty)
	lastMoveTime[2] = -MoveDelay(difficulty)

	return &GameController{
		Board:                  board,
		Tanks:                  [2]*Tank{t1, t2},
		Shots:                  []*Shot{},
		Mines:                  []*Mine{},
		Explosions:             []*Explosion{},
		SwingDir:               make(map[int]*Direction),
		LastMoveTime:           lastMoveTime,
		BarrelWreckageRegistry: []BarrelWreckageEntry{},
		BarrelHitBodies:        make(map[[2]int]bool),
		Difficulty:             difficulty,
		Winner:                 0,
		State:                  StatePlaying,
		RNG:                    rng,
	}
}

func (gc *GameController) ProcessMovement(playerID int, inputDir Direction, simTime float64) bool {
	if simTime-gc.LastMoveTime[playerID] < MoveDelay(gc.Difficulty) {
		return false
	}

	tank := gc.Tanks[playerID-1]
	swingDir := gc.SwingDir[playerID]

	if inputDir == tank.Dir {
		tank.AttemptMove(gc.Board, inputDir)
		gc.LastMoveTime[playerID] = simTime
		gc.SwingDir[playerID] = nil
		return true
	}

	if swingDir != nil && inputDir == *swingDir {
		tank.AttemptMove(gc.Board, inputDir)
		gc.LastMoveTime[playerID] = simTime
		gc.SwingDir[playerID] = nil
		return true
	}

	oldBarrelX, oldBarrelY := tank.BarrelPos()
	if gc.Board.InBounds(oldBarrelX, oldBarrelY) && gc.Board.GetCell(oldBarrelX, oldBarrelY) == CellTypeForBarrel(tank.PlayerID) {
		gc.Board.SetCellType(oldBarrelX, oldBarrelY, CellEmpty)
	}

	tank.Dir = inputDir

	newBarrelX, newBarrelY := tank.BarrelPos()
	if gc.Board.InBounds(newBarrelX, newBarrelY) && gc.Board.GetCell(newBarrelX, newBarrelY) == CellEmpty {
		gc.Board.SetCellType(newBarrelX, newBarrelY, CellTypeForBarrel(tank.PlayerID))
	}

	dir := inputDir
	gc.SwingDir[playerID] = &dir
	gc.LastMoveTime[playerID] = simTime
	return true
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

func (gc *GameController) FindSpawnPos(startX, startY int) (int, int) {
	for offset := 0; offset < gc.Board.Height; offset++ {
		dys := []int{0}
		if offset > 0 {
			dys = []int{-offset, offset}
		}

		for _, dy := range dys {
			y := startY + dy
			if y < 1 || y >= gc.Board.Height-1 {
				continue
			}
			if gc.Board.GetCell(startX, y) == CellEmpty {
				return startX, y
			}
		}
	}

	return startX, startY
}

func (gc *GameController) InitRound() {
	gc.Board = NewBoard(BoardWidth, BoardHeight, gc.Difficulty, gc.RNG)
	lives, shots, mines := DifficultyToResources(gc.Difficulty)

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

	gc.Tanks = [2]*Tank{t1, t2}
	t1.OccupyBoard(gc.Board)
	t2.OccupyBoard(gc.Board)

	gc.Shots = []*Shot{}
	gc.Mines = []*Mine{}
	gc.Explosions = []*Explosion{}
	gc.Winner = 0
	gc.State = StatePlaying
	gc.SwingDir = make(map[int]*Direction)

	gc.LastMoveTime = make(map[int]float64)
	gc.LastMoveTime[1] = -MoveDelay(gc.Difficulty)
	gc.LastMoveTime[2] = -MoveDelay(gc.Difficulty)

	gc.BarrelWreckageRegistry = []BarrelWreckageEntry{}
	gc.BarrelHitBodies = make(map[[2]int]bool)
}

func (gc *GameController) respawnBothTanks() {
	_, shots, mines := DifficultyToResources(gc.Difficulty)

	for _, tank := range gc.Tanks {
		tank.ClearFromBoard(gc.Board)
		sx, sy := gc.FindSpawnPos(tank.StartX, tank.StartY)
		tank.X, tank.Y = sx, sy
		if tank.PlayerID == 1 {
			tank.Dir = DirRight
		} else {
			tank.Dir = DirLeft
		}
		tank.OccupyBoard(gc.Board)
		tank.ShotsLeft = shots
		tank.MinesLeft = mines
		gc.SwingDir[tank.PlayerID] = nil
	}
}

func (gc *GameController) respawnSingleTank(playerID int) {
	_, shots, mines := DifficultyToResources(gc.Difficulty)
	tank := gc.Tanks[playerID-1]
	tank.ClearFromBoard(gc.Board)
	sx, sy := gc.FindSpawnPos(tank.StartX, tank.StartY)
	tank.X, tank.Y = sx, sy
	if playerID == 1 {
		tank.Dir = DirRight
	} else {
		tank.Dir = DirLeft
	}
	tank.OccupyBoard(gc.Board)
	tank.ShotsLeft = shots
	tank.MinesLeft = mines
	gc.SwingDir[playerID] = nil
}

func (gc *GameController) onPlayerDefeated(playerID int) {
	if playerID == 2 {
		gc.Winner = 1
	} else {
		gc.Winner = 2
	}
	gc.Wins[gc.Winner-1]++
	gc.BattlesPlayed++
	gc.respawnBothTanks()
}
