package server

import (
	"math/rand"
	"sync"
	"time"

	"github.com/FirebirdRender/CommodorePet_TANK/engine"
)

type MatchState int

const (
	MatchIdle MatchState = iota
	MatchRunning
	MatchRoundOver
	MatchGameOver
	MatchStopped
)

type MatchEvent interface {
	OnTick(tick uint64, state *TickMsg)
	OnRoundOver(msg *RoundOverMsg)
	OnGameOver(msg *GameOverMsg)
}

type MatchController struct {
	gc         *engine.GameController
	inputs     [2]*InputTracker
	tick       uint64
	state      MatchState
	events     MatchEvent
	difficulty int
	seed       int64

	roundOverAt     uint64
	roundPauseTicks uint64

	stopCh chan struct{}
	mu     sync.Mutex

	startOnce sync.Once
	stopOnce  sync.Once
}

func NewMatchController(difficulty int, seed int64, events MatchEvent) *MatchController {
	rng := rand.New(rand.NewSource(seed))
	gc := engine.NewGameController(difficulty, rng)
	gc.InitRound()

	return &MatchController{
		gc:              gc,
		inputs:          [2]*InputTracker{NewInputTracker(), NewInputTracker()},
		state:           MatchIdle,
		events:          events,
		difficulty:      difficulty,
		seed:            seed,
		roundPauseTicks: 120,
		stopCh:          make(chan struct{}),
	}
}

func (mc *MatchController) Start() {
	mc.startOnce.Do(func() {
		mc.mu.Lock()
		if mc.state == MatchIdle {
			mc.state = MatchRunning
		}
		mc.mu.Unlock()

		go mc.runLoop()
	})
}

func (mc *MatchController) Stop() {
	mc.stopOnce.Do(func() {
		close(mc.stopCh)
	})

	mc.mu.Lock()
	mc.state = MatchStopped
	mc.mu.Unlock()
}

func (mc *MatchController) GetInput(playerID int) *InputTracker {
	switch playerID {
	case 1:
		return mc.inputs[0]
	case 2:
		return mc.inputs[1]
	default:
		return nil
	}
}

func (mc *MatchController) GetState() MatchState {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.state
}

func (mc *MatchController) CurrentTick() uint64 {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.tick
}

func (mc *MatchController) GameStartState(yourPlayerID int) *GameStartMsg {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	return &GameStartMsg{
		Grid:         GridFromEngine(mc.gc.Board),
		Tanks:        [2]TankState{TankStateFromEngine(mc.gc.Tanks[0]), TankStateFromEngine(mc.gc.Tanks[1])},
		Difficulty:   mc.difficulty,
		YourPlayerID: yourPlayerID,
	}
}

func (mc *MatchController) runLoop() {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	for {
		select {
		case <-mc.stopCh:
			return
		case <-ticker.C:
			mc.stepTick()
		}
	}
}

func (mc *MatchController) stepTick() {
	var roundOverMsg *RoundOverMsg
	var gameOverMsg *GameOverMsg
	shouldBroadcast := false

	mc.mu.Lock()
	mc.tick++

	if mc.state == MatchRoundOver {
		if mc.tick-mc.roundOverAt >= mc.roundPauseTicks {
			mc.gc.InitRound()
			mc.inputs[0].Reset()
			mc.inputs[1].Reset()
			mc.state = MatchRunning
		}
		shouldBroadcast = true
		mc.mu.Unlock()
		if shouldBroadcast {
			mc.broadcastTick()
		}
		return
	}

	if mc.state != MatchRunning {
		mc.mu.Unlock()
		return
	}

	a1 := mc.inputs[0].ConsumeAction()
	a2 := mc.inputs[1].ConsumeAction()

	if a1 != engine.ActionNone {
		mc.gc.ApplyInput(1, a1)
	}
	if a2 != engine.ActionNone {
		mc.gc.ApplyInput(2, a2)
	}

	dt := 1.0 / float64(engine.FPS)
	mc.gc.Update(dt)
	shouldBroadcast = true

	if mc.gc.Winner != 0 {
		winner := mc.gc.Winner
		loserID := 1
		if winner == 1 {
			loserID = 2
		}
		loserTank := mc.gc.Tanks[loserID-1]

		roundOverMsg = &RoundOverMsg{
			Winner:        winner,
			Wins:          mc.gc.Wins,
			BattlesPlayed: mc.gc.BattlesPlayed,
		}

		if !loserTank.IsAlive() {
			mc.state = MatchGameOver
			gameOverMsg = &GameOverMsg{
				Winner:       winner,
				FinalWins:    mc.gc.Wins,
				TotalBattles: mc.gc.BattlesPlayed,
			}
		} else {
			mc.state = MatchRoundOver
			mc.roundOverAt = mc.tick
		}

		mc.gc.Winner = 0
	}

	mc.mu.Unlock()

	if shouldBroadcast {
		mc.broadcastTick()
	}
	if roundOverMsg != nil {
		mc.emitRoundOver(roundOverMsg)
	}
	if gameOverMsg != nil {
		mc.emitGameOver(gameOverMsg)
	}
}

func (mc *MatchController) broadcastTick() {
	mc.mu.Lock()
	tick := mc.tick
	msg := mc.buildTickMsgLocked()
	mc.mu.Unlock()

	mc.emitTick(tick, msg)
}

func (mc *MatchController) buildTickMsgLocked() *TickMsg {
	shots := make([]ShotState, 0, len(mc.gc.Shots))
	for _, shot := range mc.gc.Shots {
		shots = append(shots, ShotStateFromEngine(shot))
	}

	mines := make([]MineState, 0, len(mc.gc.Mines))
	for _, mine := range mc.gc.Mines {
		mines = append(mines, MineStateFromEngine(mine, mc.gc.SimTime))
	}

	explosions := make([]ExplosionState, 0, len(mc.gc.Explosions))
	for _, explosion := range mc.gc.Explosions {
		explosions = append(explosions, ExplosionStateFromEngine(explosion))
	}

	return &TickMsg{
		Tick:       mc.tick,
		Grid:       GridFromEngine(mc.gc.Board),
		Tanks:      [2]TankState{TankStateFromEngine(mc.gc.Tanks[0]), TankStateFromEngine(mc.gc.Tanks[1])},
		Shots:      shots,
		Mines:      mines,
		Explosions: explosions,
	}
}

func (mc *MatchController) emitTick(tick uint64, msg *TickMsg) {
	if mc.events != nil {
		mc.events.OnTick(tick, msg)
	}
}

func (mc *MatchController) emitRoundOver(msg *RoundOverMsg) {
	if mc.events != nil {
		mc.events.OnRoundOver(msg)
	}
}

func (mc *MatchController) emitGameOver(msg *GameOverMsg) {
	if mc.events != nil {
		mc.events.OnGameOver(msg)
	}
}
