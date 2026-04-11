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
	EmptyGunPending        map[int]bool

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
		EmptyGunPending:        make(map[int]bool),
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

func (gc *GameController) ApplyInput(playerID int, action Action) {
	if gc.State != StatePlaying {
		return
	}

	switch action {
	case ActionFire:
		gc.FireShot(playerID)
	case ActionPlaceMine:
		gc.PlaceMineAction(playerID)
	default:
		dir, ok := ActionToDirection(action)
		if ok {
			gc.ProcessMovement(playerID, dir, gc.SimTime)
			gc.ResolveTankMineCollision(gc.Tanks[playerID-1])
		}
	}
}

func (gc *GameController) PlaceMineAction(playerID int) {
	tank := gc.Tanks[playerID-1]
	mine := PlaceMine(tank, gc.Board, gc.SimTime)
	if mine != nil {
		gc.Mines = append(gc.Mines, mine)
	}
}

func (gc *GameController) ShotSpawnPosition(tank *Tank) ([2]int, bool) {
	v := DirectionVectors[tank.Dir]
	x, y := tank.X+v[0], tank.Y+v[1]
	if !gc.Board.InBounds(x, y) {
		return [2]int{}, false
	}
	if gc.Board.GetCell(x, y) == CellWall {
		return [2]int{}, false
	}
	return [2]int{x, y}, true
}

func (gc *GameController) FireShot(playerID int) {
	tank := gc.Tanks[playerID-1]
	if !tank.CanFire() {
		return
	}

	for _, s := range gc.Shots {
		if s.Active && s.OwnerID == playerID {
			return
		}
	}

	pos, ok := gc.ShotSpawnPosition(tank)
	if !ok {
		return
	}

	v := DirectionVectors[tank.Dir]
	var maxRange int
	if v[0] != 0 && v[1] != 0 {
		minDim := gc.Board.Width
		if gc.Board.Height < minDim {
			minDim = gc.Board.Height
		}
		maxRange = int(0.75 * float64(minDim))
	} else if v[0] != 0 {
		maxRange = int(0.75 * float64(gc.Board.Width))
	} else {
		maxRange = int(0.75 * float64(gc.Board.Height))
	}

	shot := NewShot(pos[0], pos[1], tank.Dir, playerID, maxRange)
	gc.Shots = append(gc.Shots, shot)
	tank.ConsumeShot()

	if tank.ShotsLeft == 0 {
		gc.EmptyGunPending[playerID] = true
	}
}

func (gc *GameController) Update(dt float64) {
	if gc.State != StatePlaying {
		return
	}

	gc.SimTime += dt
	gc.updateShots()
	gc.updateMines()
	gc.updateExplosions()
}

func (gc *GameController) updateShots() {
	for _, shot := range gc.Shots {
		if shot.Active {
			shot.StepStartX = shot.X
			shot.StepStartY = shot.Y
		}
	}

	for _, shot := range gc.Shots {
		if gc.SimTime-shot.LastMoveTime < ShotDelay {
			continue
		}
		if !shot.Active {
			continue
		}

		collisionPos := shot.Step(gc.Board)
		if shot.Active {
			shot.LastMoveTime = gc.SimTime
		} else {
			shot.CollisionPos = collisionPos
		}
	}

	pairs := DetectShotShotCollisions(gc)
	shotShotKeys := make(map[[2]int]bool)
	for _, pair := range pairs {
		s1, s2 := gc.Shots[pair[0]], gc.Shots[pair[1]]
		crossed := ShotsCrossedHeadOn(s1, s2)
		key := ShotShotExplosionKey(s1, s2, crossed)
		s1.Active = false
		s2.Active = false
		if !shotShotKeys[key] {
			shotShotKeys[key] = true
			gc.ExplodeMine(key[0], key[1], 2, true)
		}
	}

	for _, shot := range gc.Shots {
		var cx, cy int
		if shot.CollisionPos != nil {
			cx, cy = shot.CollisionPos[0], shot.CollisionPos[1]
			shot.CollisionPos = nil
		} else {
			if !shot.Active {
				continue
			}
			cx, cy = shot.X, shot.Y
		}

		cell := gc.Board.GetCell(cx, cy)
		switch cell {
		case CellBarrel1, CellBarrel2:
			targetID := 1
			if cell == CellBarrel2 {
				targetID = 2
			}
			if targetID == shot.OwnerID {
				continue
			}
			gc.BarrelHit(gc.Tanks[targetID-1], [2]int{cx, cy})
		case CellTank1, CellTank2:
			targetID := 1
			if cell == CellTank2 {
				targetID = 2
			}
			gc.TankHit(gc.Tanks[targetID-1], [2]int{cx, cy})
		case CellMine:
			for _, mine := range gc.Mines {
				if mine.Active && mine.X == cx && mine.Y == cy {
					mine.Active = false
					gc.ExplodeMine(mine.X, mine.Y, 2, true)
					break
				}
			}
		}
	}

	active := gc.Shots[:0]
	for _, s := range gc.Shots {
		if s.Active || s.CollisionPos != nil {
			active = append(active, s)
		}
	}
	gc.Shots = active

	for pid, pending := range gc.EmptyGunPending {
		if !pending {
			continue
		}
		hasActive := false
		for _, s := range gc.Shots {
			if s.Active && s.OwnerID == pid {
				hasActive = true
				break
			}
		}
		if !hasActive {
			delete(gc.EmptyGunPending, pid)
			tank := gc.Tanks[pid-1]
			if tank.IsAlive() && gc.State == StatePlaying {
				gc.TankHit(tank, [2]int{tank.X, tank.Y})
			}
		}
	}
}

func (gc *GameController) updateMines() {
	for _, mine := range gc.Mines {
		if mine.Active {
			mine.UpdateVisibility(gc.SimTime)
		}
	}
}

func (gc *GameController) updateExplosions() {
	active := gc.Explosions[:0]
	for _, exp := range gc.Explosions {
		if gc.SimTime-exp.StartTime < exp.Duration {
			active = append(active, exp)
		}
	}
	gc.Explosions = active
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
	gc.EmptyGunPending = make(map[int]bool)
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
