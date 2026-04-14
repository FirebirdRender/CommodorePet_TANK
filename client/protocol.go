package client

import (
	"encoding/json"
	"errors"
)

// Message type constants (match server exactly)
const (
	MsgTypeInput      = "input"
	MsgTypeCreateRoom = "create_room"
	MsgTypeJoinRoom   = "join_room"
	MsgTypeReady      = "ready"
	MsgTypePlayAgain  = "play_again"

	MsgTypeRoomCreated  = "room_created"
	MsgTypeJoined       = "joined"
	MsgTypeGameStart    = "game_start"
	MsgTypeTick         = "tick"
	MsgTypeTickDelta    = "tick_delta"
	MsgTypeRoundOver    = "round_over"
	MsgTypeGameOver     = "game_over"
	MsgTypeError        = "error"
	MsgTypePlayAgainAck = "play_again_ack"
	MsgTypeRematch      = "rematch"
	MsgTypeOpponentLeft = "opponent_left"
)

// Error code constants
const (
	ErrCodeRoomFull      = "room_full"
	ErrCodeRoomNotFound  = "room_not_found"
	ErrCodeAlreadyInRoom = "already_in_room"
	ErrCodeNotReady      = "not_ready"
	ErrCodeInvalidInput  = "invalid_input"
	ErrCodeServerError   = "server_error"
)

// Envelope wraps all messages
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Client → Server messages
type InputMsg struct {
	Tick   uint64 `json:"tick"`
	Key    string `json:"key"`
	Action string `json:"action"`
}

type CreateRoomMsg struct {
	Difficulty int    `json:"difficulty"`
	PlayerName string `json:"player_name"`
}

type JoinRoomMsg struct {
	RoomCode   string `json:"room_code"`
	PlayerName string `json:"player_name"`
}

type ReadyMsg struct{}

type PlayAgainMsg struct{}

// Server → Client messages
type RoomCreatedMsg struct {
	RoomCode string `json:"room_code"`
}

type JoinedMsg struct {
	RoomCode     string `json:"room_code"`
	PlayerID     int    `json:"player_id"`
	OpponentName string `json:"opponent_name"`
}

type GameStartMsg struct {
	Grid         [][]int      `json:"grid"`
	Tanks        [2]TankState `json:"tanks"`
	Difficulty   int          `json:"difficulty"`
	YourPlayerID int          `json:"your_player_id"`
}

type TickMsg struct {
	Tick       uint64           `json:"tick"`
	Grid       [][]int          `json:"grid"`
	Tanks      [2]TankState     `json:"tanks"`
	Shots      []ShotState      `json:"shots"`
	Mines      []MineState      `json:"mines"`
	Explosions []ExplosionState `json:"explosions"`
}

type RoundOverMsg struct {
	Winner        int    `json:"winner"`
	Wins          [2]int `json:"wins"`
	BattlesPlayed int    `json:"battles_played"`
}

type GameOverMsg struct {
	Winner       int    `json:"winner"`
	FinalWins    [2]int `json:"final_wins"`
	TotalBattles int    `json:"total_battles"`
}

type ErrorMsg struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PlayAgainAckMsg struct {
	WaitingFor string `json:"waiting_for"`
}

type RematchMsg struct {
	Difficulty int `json:"difficulty"`
}

type OpponentLeftMsg struct {
	Reason string `json:"reason"`
}

// Entity state types
type TankState struct {
	PlayerID  int  `json:"player_id"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Dir       int  `json:"dir"`
	Lives     int  `json:"lives"`
	ShotsLeft int  `json:"shots_left"`
	MinesLeft int  `json:"mines_left"`
	Active    bool `json:"active"`
}

type ShotState struct {
	X       int  `json:"x"`
	Y       int  `json:"y"`
	Dir     int  `json:"dir"`
	OwnerID int  `json:"owner_id"`
	Active  bool `json:"active"`
}

type MineState struct {
	X       int  `json:"x"`
	Y       int  `json:"y"`
	OwnerID int  `json:"owner_id"`
	Visible bool `json:"visible"`
}

type ExplosionState struct {
	X               int     `json:"x"`
	Y               int     `json:"y"`
	Duration        float64 `json:"duration"`
	IsChainReaction bool    `json:"is_chain_reaction"`
}

type CellChange struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Cell int `json:"cell"`
}

type TickDeltaMsg struct {
	Tick         uint64           `json:"tick"`
	ChangedCells []CellChange     `json:"changed_cells"`
	Tanks        [2]TankState     `json:"tanks"`
	Shots        []ShotState      `json:"shots"`
	Mines        []MineState      `json:"mines"`
	Explosions   []ExplosionState `json:"explosions"`
}

var errInvalidMessage = errors.New("missing or empty message type")

// WrapMessage serializes a message type + payload into an Envelope JSON bytes
func WrapMessage(msgType string, payload any) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Type: msgType, Payload: payloadBytes})
}

// UnwrapMessage deserializes raw JSON bytes into (type, raw payload bytes)
func UnwrapMessage(data []byte) (msgType string, payload []byte, err error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", nil, err
	}
	if env.Type == "" {
		return "", nil, errInvalidMessage
	}
	return env.Type, env.Payload, nil
}
