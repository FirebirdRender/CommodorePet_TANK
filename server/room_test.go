package server

import (
	"sync"
	"testing"
	"time"
)

func TestNewRoomDefaults(t *testing.T) {
	before := time.Now()
	r := NewRoom("ABCD", 7)
	after := time.Now()

	if r.Code != "ABCD" {
		t.Fatalf("expected code ABCD, got %q", r.Code)
	}
	if r.Difficulty != 7 {
		t.Fatalf("expected difficulty 7, got %d", r.Difficulty)
	}
	if r.State != RoomWaiting {
		t.Fatalf("expected state RoomWaiting, got %v", r.State)
	}
	if r.Players[0] != nil || r.Players[1] != nil {
		t.Fatalf("expected empty players, got %+v", r.Players)
	}
	if r.CreatedAt.Before(before) || r.CreatedAt.After(after) {
		t.Fatalf("createdAt out of expected range: %v", r.CreatedAt)
	}
}

func TestAddPlayerAssignsSlotsInOrder(t *testing.T) {
	r := NewRoom("ROOM", 3)

	id1, err := r.AddPlayer("Alice")
	if err != nil {
		t.Fatalf("add first player failed: %v", err)
	}
	id2, err := r.AddPlayer("Bob")
	if err != nil {
		t.Fatalf("add second player failed: %v", err)
	}

	if id1 != 1 || id2 != 2 {
		t.Fatalf("expected player ids 1 then 2, got %d then %d", id1, id2)
	}
	if r.GetPlayer(1) == nil || r.GetPlayer(1).Name != "Alice" {
		t.Fatalf("expected player 1 to be Alice")
	}
	if r.GetPlayer(2) == nil || r.GetPlayer(2).Name != "Bob" {
		t.Fatalf("expected player 2 to be Bob")
	}
}

func TestAddPlayerReturnsErrorWhenFull(t *testing.T) {
	r := NewRoom("ROOM", 3)
	_, _ = r.AddPlayer("Alice")
	_, _ = r.AddPlayer("Bob")

	if _, err := r.AddPlayer("Charlie"); err == nil {
		t.Fatal("expected error adding third player to full room")
	}
}

func TestAddPlayerTransitionsToReadyOnSecondJoin(t *testing.T) {
	r := NewRoom("ROOM", 3)

	_, _ = r.AddPlayer("Alice")
	if r.GetState() != RoomWaiting {
		t.Fatalf("expected RoomWaiting after first player, got %v", r.GetState())
	}

	_, _ = r.AddPlayer("Bob")
	if r.GetState() != RoomReady {
		t.Fatalf("expected RoomReady after second player, got %v", r.GetState())
	}
}

func TestRemovePlayerMarksDisconnected(t *testing.T) {
	r := NewRoom("ROOM", 3)
	_, _ = r.AddPlayer("Alice")

	_ = r.RemovePlayer(1)
	p := r.GetPlayer(1)
	if p == nil {
		t.Fatal("expected player 1 to exist")
	}
	if p.Connected {
		t.Fatal("expected player 1 to be disconnected")
	}
}

func TestRemovePlayerReturnsEmptyWhenAllPlayersGone(t *testing.T) {
	r := NewRoom("ROOM", 3)
	_, _ = r.AddPlayer("Alice")
	_, _ = r.AddPlayer("Bob")

	empty := r.RemovePlayer(1)
	if empty {
		t.Fatal("expected empty=false after removing one of two connected players")
	}

	empty = r.RemovePlayer(2)
	if !empty {
		t.Fatal("expected empty=true after removing both connected players")
	}
}

func TestRemovePlayerTransitionsToClosedFromWaiting(t *testing.T) {
	r := NewRoom("ROOM", 3)
	_, _ = r.AddPlayer("Alice")

	_ = r.RemovePlayer(1)
	if r.GetState() != RoomClosed {
		t.Fatalf("expected RoomClosed, got %v", r.GetState())
	}
}

func TestRemovePlayerTransitionsToClosedFromPlaying(t *testing.T) {
	r := NewRoom("ROOM", 3)
	_, _ = r.AddPlayer("Alice")
	_, _ = r.AddPlayer("Bob")
	r.SetState(RoomPlaying)

	_ = r.RemovePlayer(2)
	if r.GetState() != RoomClosed {
		t.Fatalf("expected RoomClosed from RoomPlaying disconnect, got %v", r.GetState())
	}
}

func TestSetReadyMarksPlayerReadyAndInvalidIDErrors(t *testing.T) {
	r := NewRoom("ROOM", 3)
	_, _ = r.AddPlayer("Alice")
	_, _ = r.AddPlayer("Bob")

	if err := r.SetReady(1); err != nil {
		t.Fatalf("expected SetReady for player 1 to succeed, got %v", err)
	}
	if !r.GetPlayer(1).Ready {
		t.Fatal("expected player 1 to be ready")
	}

	if err := r.SetReady(99); err == nil {
		t.Fatal("expected SetReady to fail for invalid player id")
	}
}

func TestBothReady(t *testing.T) {
	r := NewRoom("ROOM", 3)
	_, _ = r.AddPlayer("Alice")
	_, _ = r.AddPlayer("Bob")

	if r.BothReady() {
		t.Fatal("expected BothReady false initially")
	}

	_ = r.SetReady(1)
	if r.BothReady() {
		t.Fatal("expected BothReady false with one ready")
	}

	_ = r.SetReady(2)
	if !r.BothReady() {
		t.Fatal("expected BothReady true with both ready")
	}
}

func TestPlayerCountReflectsConnectedPlayers(t *testing.T) {
	r := NewRoom("ROOM", 3)

	if got := r.PlayerCount(); got != 0 {
		t.Fatalf("expected 0 connected players, got %d", got)
	}

	_, _ = r.AddPlayer("Alice")
	if got := r.PlayerCount(); got != 1 {
		t.Fatalf("expected 1 connected player, got %d", got)
	}

	_, _ = r.AddPlayer("Bob")
	if got := r.PlayerCount(); got != 2 {
		t.Fatalf("expected 2 connected players, got %d", got)
	}

	_ = r.RemovePlayer(1)
	if got := r.PlayerCount(); got != 1 {
		t.Fatalf("expected 1 connected player after disconnect, got %d", got)
	}
}

func TestIsFullReflectsRoomOccupancy(t *testing.T) {
	r := NewRoom("ROOM", 3)
	if r.IsFull() {
		t.Fatal("expected room not full initially")
	}

	_, _ = r.AddPlayer("Alice")
	if r.IsFull() {
		t.Fatal("expected room not full with one player")
	}

	_, _ = r.AddPlayer("Bob")
	if !r.IsFull() {
		t.Fatal("expected room full with two players")
	}
}

func TestAddPlayerConcurrentSafety(t *testing.T) {
	r := NewRoom("ROOM", 3)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	var mu sync.Mutex
	successIDs := make([]int, 0, 2)
	errs := 0

	for range goroutines {
		name := "P"
		go func(playerName string) {
			defer wg.Done()
			id, err := r.AddPlayer(playerName)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs++
				return
			}
			successIDs = append(successIDs, id)
		}(name)
	}

	wg.Wait()

	if len(successIDs) != 2 {
		t.Fatalf("expected exactly 2 successful joins, got %d", len(successIDs))
	}
	if errs != goroutines-2 {
		t.Fatalf("expected %d errors, got %d", goroutines-2, errs)
	}

	seen := map[int]int{}
	for _, id := range successIDs {
		seen[id]++
	}
	if seen[1] != 1 || seen[2] != 1 {
		t.Fatalf("expected one success for id=1 and one for id=2, got %+v", seen)
	}
}

func TestGetPlayerNilForEmptySlot(t *testing.T) {
	r := NewRoom("ROOM", 3)
	if got := r.GetPlayer(1); got != nil {
		t.Fatalf("expected nil for empty slot 1, got %+v", got)
	}
	if got := r.GetPlayer(2); got != nil {
		t.Fatalf("expected nil for empty slot 2, got %+v", got)
	}
}

func TestReserveBotSeat(t *testing.T) {
	r := NewRoom("ROOM", 3)

	err := r.ReserveBotSeat()
	if err != nil {
		t.Fatalf("expected ReserveBotSeat to succeed, got %v", err)
	}
	if !r.BotSeatReserved {
		t.Fatal("expected BotSeatReserved to be true")
	}

	r.Players[1] = &Player{ID: 2, Name: "Bob", Connected: true}
	err = r.ReserveBotSeat()
	if err == nil {
		t.Fatal("expected ReserveBotSeat to fail when seat is taken")
	}
}

func TestAddBotPlayer(t *testing.T) {
	r := NewRoom("ROOM", 3)

	_, err := r.AddBotPlayer("Bot1", "bot-001", "mvp")
	if err == nil {
		t.Fatal("expected AddBotPlayer to fail without reservation")
	}

	r.BotSeatReserved = true
	id, err := r.AddBotPlayer("Bot1", "bot-001", "mvp")
	if err != nil {
		t.Fatalf("expected AddBotPlayer to succeed after reservation, got %v", err)
	}
	if id != 2 {
		t.Fatalf("expected player id 2, got %d", id)
	}

	p := r.GetPlayer(2)
	if p == nil {
		t.Fatal("expected player 2 to exist")
	}
	if !p.IsBot {
		t.Fatal("expected player 2 to be a bot")
	}
	if p.BotID != "bot-001" {
		t.Fatalf("expected BotID bot-001, got %q", p.BotID)
	}
	if p.BotClass != "mvp" {
		t.Fatalf("expected BotClass mvp, got %q", p.BotClass)
	}
	if p.Name != "Bot1" {
		t.Fatalf("expected Name Bot1, got %q", p.Name)
	}

	if r.GetState() != RoomReady {
		t.Fatalf("expected RoomReady after bot joins, got %v", r.GetState())
	}
	if r.BotSeatReserved {
		t.Fatal("expected BotSeatReserved to be cleared after AddBotPlayer")
	}
}

func TestAddPlayerBotSeatReserved(t *testing.T) {
	r := NewRoom("ROOM", 3)

	r.BotSeatReserved = true

	id1, err := r.AddPlayer("Alice")
	if err != nil {
		t.Fatalf("expected AddPlayer seat 1 to succeed, got %v", err)
	}
	if id1 != 1 {
		t.Fatalf("expected player id 1, got %d", id1)
	}

	_, err = r.AddPlayer("Bob")
	if err == nil {
		t.Fatal("expected AddPlayer seat 2 to fail when bot seat is reserved")
	}
}

func TestCancelBotReservation(t *testing.T) {
	r := NewRoom("ROOM", 3)

	r.ReserveBotSeat()
	if !r.BotSeatReserved {
		t.Fatal("expected BotSeatReserved to be true after ReserveBotSeat")
	}

	r.CancelBotReservation()
	if r.BotSeatReserved {
		t.Fatal("expected BotSeatReserved to be false after CancelBotReservation")
	}

	id, err := r.AddPlayer("Bob")
	if err != nil {
		t.Fatalf("expected AddPlayer to succeed after cancel, got %v", err)
	}
	if id != 1 {
		t.Fatalf("expected player id 1 (first available seat), got %d", id)
	}
}

func TestNewRoomWithBotPolicy(t *testing.T) {
	r := NewRoomWithBotPolicy("ABCD", 5, true, true, 30)
	if r.Code != "ABCD" {
		t.Fatalf("expected code ABCD, got %q", r.Code)
	}
	if r.Difficulty != 5 {
		t.Fatalf("expected difficulty 5, got %d", r.Difficulty)
	}
	if !r.AllowBot {
		t.Fatal("expected AllowBot to be true")
	}
	if !r.AutoFillBot {
		t.Fatal("expected AutoFillBot to be true")
	}
	if r.AutoFillAfterSec != 30 {
		t.Fatalf("expected AutoFillAfterSec 30, got %d", r.AutoFillAfterSec)
	}
	if r.State != RoomWaiting {
		t.Fatalf("expected state RoomWaiting, got %v", r.State)
	}
}

func TestRemoveBotPlayerClearsReservation(t *testing.T) {
	r := NewRoomWithBotPolicy("ROOM", 3, true, false, 0)
	r.BotSeatReserved = true

	r.Players[1] = &Player{ID: 2, Name: "Bot1", Connected: true, IsBot: true, BotID: "bot-001"}
	r.RemovePlayer(2)

	if r.BotSeatReserved {
		t.Fatal("expected BotSeatReserved to be cleared when bot player removed")
	}
}

func TestRoomWithBotPolicy_StatusAPI(t *testing.T) {
	r := NewRoomWithBotPolicy("ABCD", 5, true, true, 30)
	r.Players[0] = &Player{ID: 1, Name: "Alice", Connected: true}

	if !r.AllowBot {
		t.Fatal("expected AllowBot to be true")
	}
	if !r.AutoFillBot {
		t.Fatal("expected AutoFillBot to be true")
	}
	if r.AutoFillAfterSec != 30 {
		t.Fatalf("expected AutoFillAfterSec 30, got %d", r.AutoFillAfterSec)
	}

	r.Players[1] = &Player{ID: 2, Name: "Bot1", Connected: true, IsBot: true, BotID: "bot-001", BotClass: "mvp"}

	p1 := r.GetPlayer(1)
	p2 := r.GetPlayer(2)
	if p1 == nil || p2 == nil {
		t.Fatal("expected both players to exist")
	}
	if !p2.IsBot {
		t.Fatal("expected player 2 to be a bot")
	}
}
