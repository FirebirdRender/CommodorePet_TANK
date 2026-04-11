package engine

import "math/rand"

// MatchResult holds the outcome of a completed match.
type MatchResult struct {
	Winner        int // 0=no winner (timeout), 1=P1, 2=P2
	Ticks         int
	P1Wins        int
	P2Wins        int
	BattlesPlayed int
}

// InputProvider supplies player actions each tick.
type InputProvider interface {
	NextInputs(tick int, gc *GameController) map[int]Action
}

// RandomInputProvider generates random actions using a seeded RNG.
type RandomInputProvider struct {
	rng *rand.Rand
}

func NewRandomInputProvider(seed int64) *RandomInputProvider {
	return &RandomInputProvider{rng: rand.New(rand.NewSource(seed))}
}

func (r *RandomInputProvider) NextInputs(tick int, gc *GameController) map[int]Action {
	actions := map[int]Action{}
	allActions := []Action{
		ActionNone, ActionNone, ActionNone, // weight None higher so matches actually play out
		ActionUp, ActionDown, ActionLeft, ActionRight,
		ActionUpLeft, ActionUpRight, ActionDownLeft, ActionDownRight,
		ActionFire,
		ActionPlaceMine,
	}
	actions[1] = allActions[r.rng.Intn(len(allActions))]
	actions[2] = allActions[r.rng.Intn(len(allActions))]
	return actions
}

// ScriptedInput represents a single scripted action at a specific tick.
type ScriptedInput struct {
	Tick     int
	PlayerID int
	Action   Action
}

// ScriptedInputProvider replays a predetermined sequence of inputs.
type ScriptedInputProvider struct {
	inputs []ScriptedInput
}

func NewScriptedInputProvider(inputs []ScriptedInput) *ScriptedInputProvider {
	return &ScriptedInputProvider{inputs: inputs}
}

func (s *ScriptedInputProvider) NextInputs(tick int, gc *GameController) map[int]Action {
	actions := map[int]Action{}
	for _, inp := range s.inputs {
		if inp.Tick == tick {
			actions[inp.PlayerID] = inp.Action
		}
	}
	return actions
}

// RunMatch executes a complete match and returns the result.
func RunMatch(difficulty int, seed int64, maxTicks int, provider InputProvider) MatchResult {
	rng := rand.New(rand.NewSource(seed))
	gc := NewGameController(difficulty, rng)
	gc.InitRound()

	dt := 1.0 / float64(FPS)
	ticksExecuted := 0

	for tick := range maxTicks {
		if gc.State != StatePlaying {
			break
		}

		if provider != nil {
			inputs := provider.NextInputs(tick, gc)
			if action, ok := inputs[1]; ok {
				gc.ApplyInput(1, action)
			}
			if action, ok := inputs[2]; ok {
				gc.ApplyInput(2, action)
			}
		}

		gc.Update(dt)
		ticksExecuted++
	}

	return MatchResult{
		Winner:        gc.Winner,
		Ticks:         ticksExecuted,
		P1Wins:        gc.Wins[0],
		P2Wins:        gc.Wins[1],
		BattlesPlayed: gc.BattlesPlayed,
	}
}

func RunMatchDeterministic(difficulty int, seed int64, maxTicks int, provider InputProvider) MatchResult {
	r1 := RunMatch(difficulty, seed, maxTicks, provider)
	r2 := RunMatch(difficulty, seed, maxTicks, provider)
	if r1 != r2 {
		panic("non-deterministic match execution")
	}
	return r1
}
