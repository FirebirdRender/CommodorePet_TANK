package server

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"

	"github.com/FirebirdRender/CommodorePet_TANK/engine"
)

func roundTripPayload[T any](t *testing.T, msgType string, in T) T {
	t.Helper()

	wrapped, err := WrapMessage(msgType, in)
	if err != nil {
		t.Fatalf("WrapMessage() error = %v", err)
	}

	gotType, payload, err := UnwrapMessage(wrapped)
	if err != nil {
		t.Fatalf("UnwrapMessage() error = %v", err)
	}
	if gotType != msgType {
		t.Fatalf("message type mismatch: got %q, want %q", gotType, msgType)
	}

	var out T
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("json.Unmarshal(payload) error = %v", err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("payload mismatch: got %+v, want %+v", out, in)
	}

	return out
}

func TestRoundTripAllMessageTypes(t *testing.T) {
	ready := roundTripPayload(t, MsgTypeReady, ReadyMsg{})
	if !reflect.DeepEqual(ready, ReadyMsg{}) {
		t.Fatalf("ready payload mismatch: got %+v", ready)
	}

	playAgain := roundTripPayload(t, MsgTypePlayAgain, PlayAgainMsg{})
	if !reflect.DeepEqual(playAgain, PlayAgainMsg{}) {
		t.Fatalf("play_again payload mismatch: got %+v", playAgain)
	}

	roundTripPayload(t, MsgTypeInput, InputMsg{Tick: 99, Key: "up_left", Action: "down"})
	roundTripPayload(t, MsgTypeCreateRoom, CreateRoomMsg{Difficulty: 7, PlayerName: "alice"})
	roundTripPayload(t, MsgTypeJoinRoom, JoinRoomMsg{RoomCode: "ABCD", PlayerName: "bob"})
	roundTripPayload(t, MsgTypeRoomCreated, RoomCreatedMsg{RoomCode: "ABCD"})
	roundTripPayload(t, MsgTypeJoined, JoinedMsg{RoomCode: "ABCD", PlayerID: 2, OpponentName: "alice"})

	gs := GameStartMsg{
		Grid: [][]int{{1, 2}, {3, 4}},
		Tanks: [2]TankState{
			{PlayerID: 1, X: 2, Y: 10, Dir: int(engine.DirRight), Lives: 3, ShotsLeft: 8, MinesLeft: 2, Active: true},
			{PlayerID: 2, X: 37, Y: 10, Dir: int(engine.DirLeft), Lives: 3, ShotsLeft: 8, MinesLeft: 2, Active: true},
		},
		Difficulty:   5,
		YourPlayerID: 1,
	}
	roundTripPayload(t, MsgTypeGameStart, gs)

	tick := TickMsg{
		Tick: 1234,
		Grid: [][]int{{1, 2, 3}, {4, 5, 6}},
		Tanks: [2]TankState{
			{PlayerID: 1, X: 3, Y: 10, Dir: int(engine.DirUp), Lives: 2, ShotsLeft: 4, MinesLeft: 1, Active: true},
			{PlayerID: 2, X: 35, Y: 10, Dir: int(engine.DirDown), Lives: 1, ShotsLeft: 0, MinesLeft: 0, Active: false},
		},
		Shots: []ShotState{
			{X: 4, Y: 10, Dir: int(engine.DirRight), OwnerID: 1, Active: true},
		},
		Mines: []MineState{
			{X: 6, Y: 8, OwnerID: 2, Visible: true},
		},
		Explosions: []ExplosionState{
			{X: 7, Y: 8, Duration: 0.5, IsChainReaction: true},
		},
	}
	roundTripPayload(t, MsgTypeTick, tick)

	roundTripPayload(t, MsgTypeRoundOver, RoundOverMsg{Winner: 1, Wins: [2]int{2, 1}, BattlesPlayed: 3})
	roundTripPayload(t, MsgTypeGameOver, GameOverMsg{Winner: 1, FinalWins: [2]int{5, 3}, TotalBattles: 8})
	roundTripPayload(t, MsgTypeError, ErrorMsg{Code: ErrCodeRoomFull, Message: "room is full"})
}

func TestGridFromEngineSmallBoard(t *testing.T) {
	b := &engine.Board{
		Width:  3,
		Height: 2,
		Grid: [][]engine.CellType{
			{engine.CellEmpty, engine.CellWall, engine.CellTank1},
			{engine.CellMine, engine.CellShot, engine.CellBarrel2},
		},
	}

	got := GridFromEngine(b)
	want := [][]int{
		{int(engine.CellEmpty), int(engine.CellWall), int(engine.CellTank1)},
		{int(engine.CellMine), int(engine.CellShot), int(engine.CellBarrel2)},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GridFromEngine() mismatch: got %+v, want %+v", got, want)
	}
}

func TestEngineStateConverters(t *testing.T) {
	tank := &engine.Tank{PlayerID: 1, X: 5, Y: 6, Dir: engine.DirDownRight, Lives: 3, ShotsLeft: 7, MinesLeft: 2, Active: true}
	shot := &engine.Shot{X: 8, Y: 9, Dir: engine.DirUpLeft, OwnerID: 2, Active: true}
	mine := &engine.Mine{X: 4, Y: 5, OwnerID: 1, Active: true, Visible: true, PlacedTime: 10.0}
	explosion := &engine.Explosion{X: 12, Y: 13, Duration: 0.5, IsChainReaction: true}

	gotTank := TankStateFromEngine(tank)
	if gotTank != (TankState{PlayerID: 1, X: 5, Y: 6, Dir: int(engine.DirDownRight), Lives: 3, ShotsLeft: 7, MinesLeft: 2, Active: true}) {
		t.Fatalf("TankStateFromEngine() mismatch: %+v", gotTank)
	}

	gotShot := ShotStateFromEngine(shot)
	if gotShot != (ShotState{X: 8, Y: 9, Dir: int(engine.DirUpLeft), OwnerID: 2, Active: true}) {
		t.Fatalf("ShotStateFromEngine() mismatch: %+v", gotShot)
	}

	visibleMine := MineStateFromEngine(mine)
	if !visibleMine.Visible {
		t.Fatalf("MineStateFromEngine() visibility expected true, got false")
	}

	mine.Visible = false
	hiddenMine := MineStateFromEngine(mine)
	if hiddenMine.Visible {
		t.Fatalf("MineStateFromEngine() visibility expected false, got true")
	}

	gotExplosion := ExplosionStateFromEngine(explosion)
	if gotExplosion != (ExplosionState{X: 12, Y: 13, Duration: 0.5, IsChainReaction: true}) {
		t.Fatalf("ExplosionStateFromEngine() mismatch: %+v", gotExplosion)
	}
}

func TestKeyToAction(t *testing.T) {
	tests := map[string]engine.Action{
		"up":         engine.ActionUp,
		"down":       engine.ActionDown,
		"left":       engine.ActionLeft,
		"right":      engine.ActionRight,
		"up_left":    engine.ActionUpLeft,
		"up_right":   engine.ActionUpRight,
		"down_left":  engine.ActionDownLeft,
		"down_right": engine.ActionDownRight,
		"fire":       engine.ActionFire,
		"mine":       engine.ActionPlaceMine,
	}

	for key, want := range tests {
		got, ok := KeyToAction(key)
		if !ok {
			t.Fatalf("KeyToAction(%q) expected ok=true", key)
		}
		if got != want {
			t.Fatalf("KeyToAction(%q) = %v, want %v", key, got, want)
		}
	}

	got, ok := KeyToAction("teleport")
	if ok {
		t.Fatalf("KeyToAction(unknown) expected ok=false")
	}
	if got != engine.ActionNone {
		t.Fatalf("KeyToAction(unknown) action = %v, want %v", got, engine.ActionNone)
	}
}

func TestWrapMessageNilPayload(t *testing.T) {
	wrapped, err := WrapMessage(MsgTypeReady, nil)
	if err != nil {
		t.Fatalf("WrapMessage(nil) error = %v", err)
	}

	msgType, payload, err := UnwrapMessage(wrapped)
	if err != nil {
		t.Fatalf("UnwrapMessage() error = %v", err)
	}
	if msgType != MsgTypeReady {
		t.Fatalf("msgType = %q, want %q", msgType, MsgTypeReady)
	}
	if string(payload) != "null" {
		t.Fatalf("payload = %q, want %q", string(payload), "null")
	}
}

func TestUnwrapMessageErrors(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		if _, _, err := UnwrapMessage([]byte("{")); err == nil {
			t.Fatalf("expected error for invalid JSON")
		}
	})

	t.Run("missing type", func(t *testing.T) {
		data := []byte(`{"payload":{"x":1}}`)
		if _, _, err := UnwrapMessage(data); err == nil {
			t.Fatalf("expected error for missing type")
		}
	})

	t.Run("empty type", func(t *testing.T) {
		data := []byte(`{"type":"","payload":{}}`)
		if _, _, err := UnwrapMessage(data); err == nil {
			t.Fatalf("expected error for empty type")
		}
	})
}

func TestUnwrapMessageUnknownType(t *testing.T) {
	wrapped, err := WrapMessage("unknown_type", map[string]int{"v": 1})
	if err != nil {
		t.Fatalf("WrapMessage() error = %v", err)
	}

	msgType, payload, err := UnwrapMessage(wrapped)
	if err != nil {
		t.Fatalf("UnwrapMessage() error = %v", err)
	}
	if msgType != "unknown_type" {
		t.Fatalf("msgType = %q, want %q", msgType, "unknown_type")
	}

	var decoded map[string]int
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(payload) error = %v", err)
	}
	if decoded["v"] != 1 {
		t.Fatalf("decoded payload mismatch: %+v", decoded)
	}
}

func TestTickGridDimensions21x40(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	b := engine.NewBoard(engine.BoardWidth, engine.BoardHeight, 5, rng)

	grid := GridFromEngine(b)
	if len(grid) != engine.BoardHeight {
		t.Fatalf("grid rows = %d, want %d", len(grid), engine.BoardHeight)
	}
	for i, row := range grid {
		if len(row) != engine.BoardWidth {
			t.Fatalf("grid cols at row %d = %d, want %d", i, len(row), engine.BoardWidth)
		}
	}

	tick := TickMsg{Tick: 1, Grid: grid}
	out := roundTripPayload(t, MsgTypeTick, tick)
	if len(out.Grid) != engine.BoardHeight {
		t.Fatalf("round-trip grid rows = %d, want %d", len(out.Grid), engine.BoardHeight)
	}
	for i, row := range out.Grid {
		if len(row) != engine.BoardWidth {
			t.Fatalf("round-trip grid cols at row %d = %d, want %d", i, len(row), engine.BoardWidth)
		}
	}
}

func TestProtocolVersion_Constant(t *testing.T) {
	if ProtocolVersion == "" {
		t.Fatalf("ProtocolVersion must not be empty")
	}
}

func TestJoinedMsg_StampsProtocolVersion(t *testing.T) {
	in := JoinedMsg{RoomCode: "ABCD", PlayerID: 1, ProtocolVersion: ProtocolVersion}
	wrapped, err := WrapMessage(MsgTypeJoined, in)
	if err != nil {
		t.Fatalf("WrapMessage: %v", err)
	}
	_, payload, err := UnwrapMessage(wrapped)
	if err != nil {
		t.Fatalf("UnwrapMessage: %v", err)
	}
	var out JoinedMsg
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.ProtocolVersion != ProtocolVersion {
		t.Fatalf("ProtocolVersion = %q, want %q", out.ProtocolVersion, ProtocolVersion)
	}
}

func TestJoinRoomMsg_AcceptsClientProtocolVersion(t *testing.T) {
	in := JoinRoomMsg{RoomCode: "ABCD", PlayerName: "bob", ClientProtocolVersion: "0.9.0"}
	out := roundTripPayload(t, MsgTypeJoinRoom, in)
	if out.ClientProtocolVersion != "0.9.0" {
		t.Fatalf("ClientProtocolVersion = %q, want %q", out.ClientProtocolVersion, "0.9.0")
	}
}

func TestRejoinMsg_AcceptsClientProtocolVersion(t *testing.T) {
	in := RejoinMsg{RoomCode: "ABCD", PlayerID: 1, Token: "xyz", PlayerName: "bob", ClientProtocolVersion: ProtocolVersion}
	out := roundTripPayload(t, MsgTypeRejoin, in)
	if out.ClientProtocolVersion != ProtocolVersion {
		t.Fatalf("ClientProtocolVersion round-trip lost: got %q", out.ClientProtocolVersion)
	}
}

func TestRejoinAckMsg_StampsProtocolVersion(t *testing.T) {
	in := RejoinAckMsg{RoomCode: "ABCD", PlayerID: 2, ProtocolVersion: ProtocolVersion}
	out := roundTripPayload(t, "rejoin_ack", in)
	if out.ProtocolVersion != ProtocolVersion {
		t.Fatalf("ProtocolVersion lost on round-trip: got %q", out.ProtocolVersion)
	}
}
