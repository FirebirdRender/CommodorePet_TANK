package client

import (
	"testing"
)

func TestNetworkCloseIdempotent(t *testing.T) {
	n := &Network{}
	// Close on unconnected network should not panic
	n.Close()
	// Double close should not panic
	n.Close()
}

func TestNetworkCloseWithCancel(t *testing.T) {
	// Create network with cancel function set but no connection
	n := &Network{
		cancel: func() {},
	}
	n.Close()
	n.Close() //should not panic on second close
}

func TestApplyTickDimensionMismatch(t *testing.T) {
	gs := &GameState{
		Grid: make([][]int, 21),
	}
	for i := range gs.Grid {
		gs.Grid[i] = make([]int, 40)
	}
	msg := TickMsg{
		Grid: make([][]int, 10), // different dimension
	}
	gs.ApplyTick(msg)
	// Should log error and return without modifying grid
	if len(gs.Grid) != 21 {
		t.Error("grid dimension was modified despite mismatch")
	}
}

func TestApplyGameStartEmptyGrid(t *testing.T) {
	gs := &GameState{
		Phase: PhasePlaying,
		Grid:  make([][]int, 21),
	}
	for i := range gs.Grid {
		gs.Grid[i] = make([]int, 40)
	}
	msg := GameStartMsg{
		Grid: [][]int{},
	}
	gs.ApplyGameStart(msg)
	if gs.Phase != PhasePlaying {
		t.Error("game phase should remain unchanged for empty grid")
	}
}

func TestNetworkNewNetwork(t *testing.T) {
	n := &Network{}
	if n.Connected {
		t.Error("New Network.Connected should be false when uninitialized")
	}
	if n.MaxRetries != 0 {
		t.Errorf("New Network.MaxRetries = %d, want 0 when uninitialized", n.MaxRetries)
	}
}

func TestNetworkConnectFailsOnBadURL(t *testing.T) {
	n := &Network{MaxRetries: 3}
	err := n.ConnectWithRetry("not-a-url")
	if err == nil {
		t.Error("ConnectWithRetry should return error for invalid URL")
	}
}
