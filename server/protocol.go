package server

import (
	"encoding/json"
	"errors"

	"github.com/FirebirdRender/CommodorePet_TANK/engine"
)

const (
	MsgTypeInput      = "input"
	MsgTypeCreateRoom = "create_room"
	MsgTypeJoinRoom   = "join_room"
	MsgTypeReady      = "ready"
	MsgTypePlayAgain  = "play_again"

	MsgTypeRoomCreated = "room_created"
	MsgTypeJoined      = "joined"
	MsgTypeGameStart   = "game_start"
	MsgTypeTick        = "tick"
	MsgTypeRoundOver   = "round_over"
	MsgTypeGameOver    = "game_over"
	MsgTypeError       = "error"
)

const (
	ErrCodeRoomFull      = "room_full"
	ErrCodeRoomNotFound  = "room_not_found"
	ErrCodeAlreadyInRoom = "already_in_room"
	ErrCodeNotReady      = "not_ready"
	ErrCodeInvalidInput  = "invalid_input"
	ErrCodeServerError   = "server_error"
)

type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

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

func WrapMessage(msgType string, payload any) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	env := Envelope{
		Type:    msgType,
		Payload: payloadBytes,
	}

	return json.Marshal(env)
}

func UnwrapMessage(data []byte) (msgType string, payload []byte, err error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", nil, err
	}
	if env.Type == "" {
		return "", nil, errors.New("missing or empty message type")
	}
	return env.Type, env.Payload, nil
}

func TankStateFromEngine(t *engine.Tank) TankState {
	return TankState{
		PlayerID:  t.PlayerID,
		X:         t.X,
		Y:         t.Y,
		Dir:       int(t.Dir),
		Lives:     t.Lives,
		ShotsLeft: t.ShotsLeft,
		MinesLeft: t.MinesLeft,
		Active:    t.Active,
	}
}

func ShotStateFromEngine(s *engine.Shot) ShotState {
	return ShotState{
		X:       s.X,
		Y:       s.Y,
		Dir:     int(s.Dir),
		OwnerID: s.OwnerID,
		Active:  s.Active,
	}
}

func MineStateFromEngine(m *engine.Mine, simTime float64) MineState {
	return MineState{
		X:       m.X,
		Y:       m.Y,
		OwnerID: m.OwnerID,
		Visible: simTime-m.PlacedTime < engine.MineVisibleDuration,
	}
}

func ExplosionStateFromEngine(e *engine.Explosion) ExplosionState {
	return ExplosionState{
		X:               e.X,
		Y:               e.Y,
		Duration:        e.Duration,
		IsChainReaction: e.IsChainReaction,
	}
}

func GridFromEngine(b *engine.Board) [][]int {
	grid := make([][]int, len(b.Grid))
	for y := range b.Grid {
		grid[y] = make([]int, len(b.Grid[y]))
		for x := range b.Grid[y] {
			grid[y][x] = int(b.Grid[y][x])
		}
	}
	return grid
}

func KeyToAction(key string) (engine.Action, bool) {
	switch key {
	case "up":
		return engine.ActionUp, true
	case "down":
		return engine.ActionDown, true
	case "left":
		return engine.ActionLeft, true
	case "right":
		return engine.ActionRight, true
	case "up_left":
		return engine.ActionUpLeft, true
	case "up_right":
		return engine.ActionUpRight, true
	case "down_left":
		return engine.ActionDownLeft, true
	case "down_right":
		return engine.ActionDownRight, true
	case "fire":
		return engine.ActionFire, true
	case "mine":
		return engine.ActionPlaceMine, true
	default:
		return engine.ActionNone, false
	}
}
