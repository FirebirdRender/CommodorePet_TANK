package botsdk

import (
	"encoding/json"
)

// ProtocolVersion is the wire-protocol version this SDK speaks. Sent on
// rejoin/join_room and compared against server's ProtocolVersion in JoinedMsg.
// Mismatch is logged but not fatal (advisory window). Bump in lock-step with
// server/protocol.go ProtocolVersion on any breaking message-shape change.
const ProtocolVersion = "1.0.0"

// Message type constants (matching server protocol.go)
const (
	MsgTypeInput     = "input"
	MsgTypeRejoin    = "rejoin"
	MsgTypePlayAgain = "play_again"

	MsgTypeJoined       = "joined"
	MsgTypeGameStart    = "game_start"
	MsgTypeTick         = "tick"
	MsgTypeTickDelta    = "tick_delta"
	MsgTypeRoundOver    = "round_over"
	MsgTypeGameOver     = "game_over"
	MsgTypeError        = "error"
	MsgTypeOpponentLeft = "opponent_left"
	MsgTypePlayAgainAck = "play_again_ack"
)

// Envelope represents a JSON message envelope.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// RejoinPayload is sent on initial WebSocket connection.
type RejoinPayload struct {
	RoomCode              string `json:"room_code"`
	PlayerID              int    `json:"player_id"`
	Token                 string `json:"token"`
	PlayerName            string `json:"player_name"`
	ClientProtocolVersion string `json:"client_protocol_version,omitempty"`
}

// InputPayload represents an input message.
type InputPayload struct {
	Tick   uint64 `json:"tick"`
	Key    string `json:"key"`
	Action string `json:"action"`
}

// GameStartPayload — full board state at match start.
type GameStartPayload struct {
	Grid         [][]int     `json:"grid"`
	Tanks        [2]TankInfo `json:"tanks"`
	Difficulty   int         `json:"difficulty"`
	YourPlayerID int         `json:"your_player_id"`
}

// TickPayload — full keyframe.
type TickPayload struct {
	Tick            uint64            `json:"tick"`
	Grid            [][]int           `json:"grid"`
	Tanks           [2]TankInfo       `json:"tanks"`
	Shots           []ShotInfo        `json:"shots"`
	Mines           []MineInfo        `json:"mines"`
	Explosions      []ExplosionInfo   `json:"explosions"`
	BarrelWreckage  []BarrelWreckInfo `json:"barrel_wreckage"`
	BarrelHitBodies [][2]int          `json:"barrel_hit_bodies"`
}

// TickDeltaPayload — incremental update.
type TickDeltaPayload struct {
	Tick            uint64            `json:"tick"`
	ChangedCells    []CellChange      `json:"changed_cells"`
	Tanks           [2]TankInfo       `json:"tanks"`
	Shots           []ShotInfo        `json:"shots"`
	Mines           []MineInfo        `json:"mines"`
	Explosions      []ExplosionInfo   `json:"explosions"`
	BarrelWreckage  []BarrelWreckInfo `json:"barrel_wreckage"`
	BarrelHitBodies [][2]int          `json:"barrel_hit_bodies"`
}

// JoinedPayload — received on successful join.
type JoinedPayload struct {
	RoomCode        string `json:"room_code"`
	PlayerID        int    `json:"player_id"`
	OpponentName    string `json:"opponent_name"`
	IsBot           bool   `json:"is_bot,omitempty"`
	BotClass        string `json:"bot_class,omitempty"`
	ProtocolVersion string `json:"protocol_version,omitempty"`
}

// RoundOverPayload — round ended, best of series.
type RoundOverPayload struct {
	Winner        int    `json:"winner"`
	Wins          [2]int `json:"wins"`
	BattlesPlayed int    `json:"battles_played"`
}

// GameOverPayload — match series ended.
type GameOverPayload struct {
	Winner       int    `json:"winner"`
	FinalWins    [2]int `json:"final_wins"`
	TotalBattles int    `json:"total_battles"`
}

// ErrorPayload — error from server.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OpponentLeftPayload — opponent disconnected.
type OpponentLeftPayload struct {
	Reason string `json:"reason"`
}

// TankInfo — tank entity data.
type TankInfo struct {
	PlayerID  int  `json:"player_id"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Dir       int  `json:"dir"`
	Lives     int  `json:"lives"`
	ShotsLeft int  `json:"shots_left"`
	MinesLeft int  `json:"mines_left"`
	Active    bool `json:"active"`
}

// ShotInfo — projectile entity data.
type ShotInfo struct {
	X       int  `json:"x"`
	Y       int  `json:"y"`
	Dir     int  `json:"dir"`
	OwnerID int  `json:"owner_id"`
	Active  bool `json:"active"`
}

// MineInfo — mine entity data.
type MineInfo struct {
	X       int  `json:"x"`
	Y       int  `json:"y"`
	OwnerID int  `json:"owner_id"`
	Visible bool `json:"visible"`
}

// ExplosionInfo — explosion entity data.
type ExplosionInfo struct {
	X               int     `json:"x"`
	Y               int     `json:"y"`
	Duration        float64 `json:"duration"`
	IsChainReaction bool    `json:"is_chain_reaction"`
}

// BarrelWreckInfo — barrel wreckage data.
type BarrelWreckInfo struct {
	X   int `json:"x"`
	Y   int `json:"y"`
	Dir int `json:"dir"`
}

// CellChange — incremental grid cell update.
type CellChange struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Cell int `json:"cell"`
}
