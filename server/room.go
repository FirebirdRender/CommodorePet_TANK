package server

import (
	"fmt"
	"sync"
	"time"
)

type RoomState int

const (
	RoomWaiting RoomState = iota
	RoomReady
	RoomPlaying
	RoomGameOver
	RoomClosed
)

type Player struct {
	ID           int
	Name         string
	Ready        bool
	Connected    bool
	WantsRematch bool
	IsBot        bool
	BotID        string
	BotClass     string
}

type Room struct {
	Code       string
	Difficulty int
	State      RoomState
	Players    [2]*Player
	CreatedAt  time.Time

	AllowBot         bool
	AutoFillBot      bool
	AutoFillAfterSec int
	// BotSeatReserved is per-slot (index 0 = P1 seat, index 1 = P2 seat).
	// Bot-vs-bot rooms reserve both slots; standard vs-AI rooms reserve only [1].
	// Justification: priority 3 — exported field with non-obvious indexing semantics
	// that callers reading the struct will get wrong without explanation.
	BotSeatReserved [2]bool
	autoFillTimer   *time.Timer

	mu sync.Mutex
}

func NewRoom(code string, difficulty int) *Room {
	return &Room{
		Code:       code,
		Difficulty: difficulty,
		State:      RoomWaiting,
		CreatedAt:  time.Now(),
	}
}

func NewRoomWithBotPolicy(code string, difficulty int, allowBot, autoFillBot bool, autoFillAfterSec int) *Room {
	return &Room{
		Code:             code,
		Difficulty:       difficulty,
		State:            RoomWaiting,
		CreatedAt:        time.Now(),
		AllowBot:         allowBot,
		AutoFillBot:      autoFillBot,
		AutoFillAfterSec: autoFillAfterSec,
	}
}

func (r *Room) AddPlayer(name string) (playerID int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Players[0] == nil {
		if r.BotSeatReserved[0] {
			return 0, fmt.Errorf("seat reserved for bot")
		}
		r.Players[0] = &Player{ID: 1, Name: name, Connected: true}
		return 1, nil
	}

	if r.Players[1] == nil {
		if r.BotSeatReserved[1] {
			return 0, fmt.Errorf("seat reserved for bot")
		}
		r.Players[1] = &Player{ID: 2, Name: name, Connected: true}
		if r.State == RoomWaiting {
			r.State = RoomReady
		}
		return 2, nil
	}

	return 0, fmt.Errorf("room is full")
}

func (r *Room) RemovePlayer(playerID int) (empty bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if playerID >= 1 && playerID <= 2 {
		p := r.Players[playerID-1]
		if p != nil {
			p.Connected = false
			if p.IsBot {
				r.BotSeatReserved[playerID-1] = false
			}
		}
	}

	if r.State == RoomWaiting || r.State == RoomReady || r.State == RoomPlaying {
		r.State = RoomClosed
	}

	return r.connectedCountLocked() == 0
}

func (r *Room) SetReady(playerID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State != RoomReady {
		return fmt.Errorf("cannot set ready in state %d", r.State)
	}
	if playerID < 1 || playerID > 2 {
		return fmt.Errorf("invalid player id %d", playerID)
	}

	p := r.Players[playerID-1]
	if p == nil || !p.Connected {
		return fmt.Errorf("player %d not found", playerID)
	}

	p.Ready = true
	return nil
}

func (r *Room) SetRematch(playerID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if playerID >= 1 && playerID <= 2 && r.Players[playerID-1] != nil {
		r.Players[playerID-1].WantsRematch = true
	}
}

func (r *Room) BothWantRematch() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	p1 := r.Players[0]
	p2 := r.Players[1]
	return p1 != nil && p2 != nil && p1.Connected && p2.Connected &&
		p1.WantsRematch && p2.WantsRematch
}

func (r *Room) ResetForRematch() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Players[0] != nil {
		r.Players[0].WantsRematch = false
		r.Players[0].Ready = false
	}
	if r.Players[1] != nil {
		r.Players[1].WantsRematch = false
		r.Players[1].Ready = false
	}
	r.State = RoomReady
}

func (r *Room) BothReady() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	p1 := r.Players[0]
	p2 := r.Players[1]
	return p1 != nil && p2 != nil && p1.Connected && p2.Connected && p1.Ready && p2.Ready
}

func (r *Room) BothPlayersConnected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	p1 := r.Players[0]
	p2 := r.Players[1]
	return p1 != nil && p2 != nil && p1.Connected && p2.Connected
}

func (r *Room) PlayerCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.connectedCountLocked()
}

func (r *Room) GetPlayer(playerID int) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()

	if playerID < 1 || playerID > 2 {
		return nil
	}
	return r.Players[playerID-1]
}

func (r *Room) IsFull() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Players[0] != nil && r.Players[1] != nil
}

func (r *Room) SetState(s RoomState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.State = s
}

func (r *Room) GetState() RoomState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.State
}

func (r *Room) connectedCountLocked() int {
	count := 0
	for _, p := range r.Players {
		if p != nil && p.Connected {
			count++
		}
	}
	return count
}

func (r *Room) ReserveBotSeat() error {
	return r.ReserveBotSeatAt(1)
}

func (r *Room) ReserveBotSeatAt(slot int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if slot < 0 || slot > 1 {
		return fmt.Errorf("invalid slot %d", slot)
	}
	if r.Players[slot] != nil {
		return fmt.Errorf("seat already taken")
	}
	r.BotSeatReserved[slot] = true
	return nil
}

func (r *Room) AddBotPlayer(name, botID, botClass string) (int, error) {
	return r.AddBotPlayerAt(1, name, botID, botClass)
}

func (r *Room) AddBotPlayerAt(slot int, name, botID, botClass string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if slot < 0 || slot > 1 {
		return 0, fmt.Errorf("invalid slot %d", slot)
	}
	if !r.BotSeatReserved[slot] {
		return 0, fmt.Errorf("no bot seat reserved at slot %d", slot)
	}

	r.Players[slot] = &Player{
		ID:        slot + 1,
		Name:      name,
		Connected: true,
		IsBot:     true,
		BotID:     botID,
		BotClass:  botClass,
	}
	r.BotSeatReserved[slot] = false

	if r.State == RoomWaiting && r.Players[0] != nil && r.Players[1] != nil {
		r.State = RoomReady
	}
	return slot + 1, nil
}

func (r *Room) CancelBotReservation() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.BotSeatReserved[0] = false
	r.BotSeatReserved[1] = false
	if r.autoFillTimer != nil {
		r.autoFillTimer.Stop()
		r.autoFillTimer = nil
	}
}

func (r *Room) CancelBotReservationAt(slot int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slot >= 0 && slot <= 1 {
		r.BotSeatReserved[slot] = false
	}
}

func (r *Room) SetAutoFillTimer(t *time.Timer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.autoFillTimer = t
}
