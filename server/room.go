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
}

type Room struct {
	Code       string
	Difficulty int
	State      RoomState
	Players    [2]*Player
	CreatedAt  time.Time

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

func (r *Room) AddPlayer(name string) (playerID int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Players[0] == nil {
		r.Players[0] = &Player{ID: 1, Name: name, Connected: true}
		return 1, nil
	}

	if r.Players[1] == nil {
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
