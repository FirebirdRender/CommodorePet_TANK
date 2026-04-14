package client

import (
	"encoding/json"
	"testing"
)

func TestMsgTypeConstants(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{MsgTypeInput, "input"},
		{MsgTypeCreateRoom, "create_room"},
		{MsgTypeJoinRoom, "join_room"},
		{MsgTypeReady, "ready"},
		{MsgTypePlayAgain, "play_again"},
		{MsgTypeRoomCreated, "room_created"},
		{MsgTypeJoined, "joined"},
		{MsgTypeGameStart, "game_start"},
		{MsgTypeTick, "tick"},
		{MsgTypeRoundOver, "round_over"},
		{MsgTypeGameOver, "game_over"},
		{MsgTypeError, "error"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("MsgType constant = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestErrCodeConstants(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{ErrCodeRoomFull, "room_full"},
		{ErrCodeRoomNotFound, "room_not_found"},
		{ErrCodeAlreadyInRoom, "already_in_room"},
		{ErrCodeNotReady, "not_ready"},
		{ErrCodeInvalidInput, "invalid_input"},
		{ErrCodeServerError, "server_error"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("ErrCode constant = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestWrapUnwrapInputMsg(t *testing.T) {
	orig := InputMsg{Tick: 5, Key: "up", Action: "down"}
	data, err := WrapMessage(MsgTypeInput, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeInput {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeInput)
	}
	var got InputMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestWrapUnwrapCreateRoomMsg(t *testing.T) {
	orig := CreateRoomMsg{Difficulty: 7, PlayerName: "Alice"}
	data, err := WrapMessage(MsgTypeCreateRoom, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeCreateRoom {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeCreateRoom)
	}
	var got CreateRoomMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestWrapUnwrapJoinRoomMsg(t *testing.T) {
	orig := JoinRoomMsg{RoomCode: "ABCD12", PlayerName: "Bob"}
	data, err := WrapMessage(MsgTypeJoinRoom, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeJoinRoom {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeJoinRoom)
	}
	var got JoinRoomMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestWrapUnwrapReadyMsg(t *testing.T) {
	orig := ReadyMsg{}
	data, err := WrapMessage(MsgTypeReady, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeReady {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeReady)
	}
	var got ReadyMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestWrapUnwrapPlayAgainMsg(t *testing.T) {
	orig := PlayAgainMsg{}
	data, err := WrapMessage(MsgTypePlayAgain, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypePlayAgain {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypePlayAgain)
	}
	var got PlayAgainMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestWrapUnwrapRoomCreatedMsg(t *testing.T) {
	orig := RoomCreatedMsg{RoomCode: "XYZ999"}
	data, err := WrapMessage(MsgTypeRoomCreated, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeRoomCreated {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeRoomCreated)
	}
	var got RoomCreatedMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestWrapUnwrapJoinedMsg(t *testing.T) {
	orig := JoinedMsg{RoomCode: "ABCD12", PlayerID: 1, OpponentName: "Alice"}
	data, err := WrapMessage(MsgTypeJoined, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeJoined {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeJoined)
	}
	var got JoinedMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestWrapUnwrapGameStartMsg(t *testing.T) {
	grid := [][]int{
		{1, 2, 3},
		{4, 5, 6},
	}
	orig := GameStartMsg{
		Grid: grid,
		Tanks: [2]TankState{
			{PlayerID: 0, X: 5, Y: 10, Dir: 1, Lives: 3, ShotsLeft: 5, MinesLeft: 2, Active: true},
			{PlayerID: 1, X: 35, Y: 10, Dir: 3, Lives: 3, ShotsLeft: 5, MinesLeft: 2, Active: true},
		},
		Difficulty:   5,
		YourPlayerID: 0,
	}
	data, err := WrapMessage(MsgTypeGameStart, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeGameStart {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeGameStart)
	}
	var got GameStartMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got.Difficulty != orig.Difficulty || got.YourPlayerID != orig.YourPlayerID {
		t.Errorf("difficulty/your_player_id mismatch: got %+v, want %+v", got, orig)
	}
	if len(got.Grid) != len(orig.Grid) {
		t.Errorf("grid row count mismatch: got %d, want %d", len(got.Grid), len(orig.Grid))
	}
}

func TestWrapUnwrapTickMsg(t *testing.T) {
	grid := [][]int{
		{1, 2},
		{3, 4},
	}
	orig := TickMsg{
		Tick: 100,
		Grid: grid,
		Tanks: [2]TankState{
			{PlayerID: 0, X: 5, Y: 10, Dir: 1, Lives: 3, ShotsLeft: 5, MinesLeft: 2, Active: true},
			{PlayerID: 1, X: 35, Y: 10, Dir: 3, Lives: 3, ShotsLeft: 5, MinesLeft: 2, Active: false},
		},
		Shots: []ShotState{
			{X: 10, Y: 10, Dir: 1, OwnerID: 0, Active: true},
			{X: 20, Y: 15, Dir: 3, OwnerID: 1, Active: false},
		},
		Mines: []MineState{
			{X: 15, Y: 15, OwnerID: 0, Visible: true},
			{X: 25, Y: 5, OwnerID: 1, Visible: false},
		},
		Explosions: []ExplosionState{
			{X: 10, Y: 10, Duration: 0.5, IsChainReaction: false},
		},
	}
	data, err := WrapMessage(MsgTypeTick, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeTick {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeTick)
	}
	var got TickMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got.Tick != orig.Tick {
		t.Errorf("Tick = %d, want %d", got.Tick, orig.Tick)
	}
	if len(got.Shots) != len(orig.Shots) {
		t.Errorf("Shots length = %d, want %d", len(got.Shots), len(orig.Shots))
	}
	if len(got.Mines) != len(orig.Mines) {
		t.Errorf("Mines length = %d, want %d", len(got.Mines), len(orig.Mines))
	}
	if len(got.Explosions) != len(orig.Explosions) {
		t.Errorf("Explosions length = %d, want %d", len(got.Explosions), len(orig.Explosions))
	}
}

func TestWrapUnwrapRoundOverMsg(t *testing.T) {
	orig := RoundOverMsg{Winner: 0, Wins: [2]int{3, 1}, BattlesPlayed: 4}
	data, err := WrapMessage(MsgTypeRoundOver, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeRoundOver {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeRoundOver)
	}
	var got RoundOverMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestWrapUnwrapGameOverMsg(t *testing.T) {
	orig := GameOverMsg{Winner: 1, FinalWins: [2]int{2, 5}, TotalBattles: 7}
	data, err := WrapMessage(MsgTypeGameOver, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeGameOver {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeGameOver)
	}
	var got GameOverMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestWrapUnwrapErrorMsg(t *testing.T) {
	orig := ErrorMsg{Code: ErrCodeRoomFull, Message: "room is full"}
	data, err := WrapMessage(MsgTypeError, orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	msgType, payload, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage error: %v", err)
	}
	if msgType != MsgTypeError {
		t.Errorf("msgType = %q, want %q", msgType, MsgTypeError)
	}
	var got ErrorMsg
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload unmarshal error: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestUnwrapMessageEmptyType(t *testing.T) {
	_, _, err := UnwrapMessage([]byte(`{"type":"","payload":{}}`))
	if err != errInvalidMessage {
		t.Errorf("expected errInvalidMessage, got %v", err)
	}
}

func TestUnwrapMessageInvalidJSON(t *testing.T) {
	_, _, err := UnwrapMessage([]byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestUnwrapMessageMissingType(t *testing.T) {
	_, _, err := UnwrapMessage([]byte(`{"payload":{}}`))
	if err == nil {
		t.Error("expected error for missing type field")
	}
}

func TestInputMsgJSONFormat(t *testing.T) {
	orig := InputMsg{Tick: 1, Key: "up", Action: "down"}
	want := `{"tick":1,"key":"up","action":"down"}`
	gotBytes, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	got := string(gotBytes)
	if got != want {
		t.Errorf("InputMsg JSON = %s, want %s", got, want)
	}
}

func TestInputMsgWrapOutput(t *testing.T) {
	orig := InputMsg{Tick: 1, Key: "up", Action: "down"}
	want := `{"type":"input","payload":{"tick":1,"key":"up","action":"down"}}`
	got, err := WrapMessage("input", orig)
	if err != nil {
		t.Fatalf("WrapMessage error: %v", err)
	}
	if string(got) != want {
		t.Errorf("WrapMessage output = %s, want %s", got, want)
	}
}

func TestUnwrapMessageNilPayload(t *testing.T) {
	_, _, err := UnwrapMessage(nil)
	if err == nil {
		t.Error("expected error for nil data")
	}
}
