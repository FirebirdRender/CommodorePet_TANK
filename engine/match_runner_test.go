package engine

import "testing"

func TestRunMatch_Completes(t *testing.T) {
	provider := NewRandomInputProvider(100)
	result := RunMatch(5, 42, 10000, provider)

	if result.BattlesPlayed < 0 {
		t.Fatalf("BattlesPlayed: got %d, want >= 0", result.BattlesPlayed)
	}
	if result.Ticks < 0 || result.Ticks > 10000 {
		t.Fatalf("Ticks out of range: got %d, want [0, 10000]", result.Ticks)
	}
}

func TestRunMatch_Deterministic(t *testing.T) {
	r1 := RunMatch(5, 42, 5000, NewRandomInputProvider(100))
	r2 := RunMatch(5, 42, 5000, NewRandomInputProvider(100))

	if r1 != r2 {
		t.Fatalf("expected deterministic results, got r1=%+v r2=%+v", r1, r2)
	}
}

func TestRunMatch_ScriptedInputs(t *testing.T) {
	provider := NewScriptedInputProvider([]ScriptedInput{
		{Tick: 0, PlayerID: 1, Action: ActionFire},
		{Tick: 0, PlayerID: 2, Action: ActionFire},
		{Tick: 1, PlayerID: 1, Action: ActionFire},
		{Tick: 1, PlayerID: 2, Action: ActionFire},
	})

	result := RunMatch(5, 42, 120, provider)

	if result.Ticks < 0 || result.Ticks > 120 {
		t.Fatalf("Ticks out of range: got %d, want [0, 120]", result.Ticks)
	}
	if result.Winner < 0 || result.Winner > 2 {
		t.Fatalf("Winner out of range: got %d, want [0, 2]", result.Winner)
	}
	if result.BattlesPlayed < 0 {
		t.Fatalf("BattlesPlayed: got %d, want >= 0", result.BattlesPlayed)
	}
}

func TestRunMatch_ZeroTicks(t *testing.T) {
	result := RunMatch(5, 42, 0, NewRandomInputProvider(100))

	if result.Ticks != 0 {
		t.Fatalf("Ticks: got %d want 0", result.Ticks)
	}
	if result.BattlesPlayed != 0 {
		t.Fatalf("BattlesPlayed: got %d want 0", result.BattlesPlayed)
	}
	if result.Winner != 0 {
		t.Fatalf("Winner: got %d want 0", result.Winner)
	}
}

func TestRandomInputProvider_Deterministic(t *testing.T) {
	p1 := NewRandomInputProvider(123)
	p2 := NewRandomInputProvider(123)

	for tick := range 100 {
		a1 := p1.NextInputs(tick, nil)
		a2 := p2.NextInputs(tick, nil)

		if a1[1] != a2[1] || a1[2] != a2[2] {
			t.Fatalf("tick %d: mismatched actions p1=%v/%v p2=%v/%v", tick, a1[1], a1[2], a2[1], a2[2])
		}
	}
}

func TestScriptedInputProvider_OnlyEmitsMatchingTick(t *testing.T) {
	provider := NewScriptedInputProvider([]ScriptedInput{
		{Tick: 3, PlayerID: 1, Action: ActionRight},
		{Tick: 3, PlayerID: 2, Action: ActionLeft},
	})

	if got := provider.NextInputs(2, nil); len(got) != 0 {
		t.Fatalf("tick 2: expected no actions, got %+v", got)
	}

	got := provider.NextInputs(3, nil)
	if got[1] != ActionRight || got[2] != ActionLeft {
		t.Fatalf("tick 3: unexpected actions: %+v", got)
	}
}
