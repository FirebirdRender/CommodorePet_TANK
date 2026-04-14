package server

import (
	"regexp"
	"sync"
	"testing"
	"time"
)

var roomCodeRe = regexp.MustCompile(`^[A-Z2-9]{4}$`)

func TestNewHubStartsEmpty(t *testing.T) {
	h := NewHub()
	if got := h.RoomCount(); got != 0 {
		t.Fatalf("expected empty hub, got %d rooms", got)
	}
}

func TestCreateRoomReturnsValidRoom(t *testing.T) {
	h := NewHub()
	r := h.CreateRoom(7)
	if r == nil {
		t.Fatal("expected room, got nil")
	}
	if !roomCodeRe.MatchString(r.Code) {
		t.Fatalf("expected 4-letter uppercase/2-9 code, got %q", r.Code)
	}
	if r.Difficulty != 7 {
		t.Fatalf("expected difficulty 7, got %d", r.Difficulty)
	}
}

func TestCreateRoomGeneratesUniqueCodes(t *testing.T) {
	h := NewHub()
	seen := make(map[string]struct{}, 50)

	for range 50 {
		r := h.CreateRoom(3)
		if r == nil {
			t.Fatal("expected room, got nil")
		}
		if _, exists := seen[r.Code]; exists {
			t.Fatalf("duplicate room code generated: %s", r.Code)
		}
		seen[r.Code] = struct{}{}
	}
}

func TestGetRoomReturnsCorrectRoomByCode(t *testing.T) {
	h := NewHub()
	r := h.CreateRoom(5)
	if r == nil {
		t.Fatal("expected room, got nil")
	}

	got := h.GetRoom(r.Code)
	if got != r {
		t.Fatalf("expected same room pointer, got %+v want %+v", got, r)
	}
}

func TestGetRoomReturnsNilForUnknownCode(t *testing.T) {
	h := NewHub()
	if got := h.GetRoom("ZZZZ"); got != nil {
		t.Fatalf("expected nil for unknown room code, got %+v", got)
	}
}

func TestRemoveRoomDeletesRoom(t *testing.T) {
	h := NewHub()
	r := h.CreateRoom(4)
	if r == nil {
		t.Fatal("expected room, got nil")
	}

	h.RemoveRoom(r.Code)
	if got := h.GetRoom(r.Code); got != nil {
		t.Fatalf("expected room to be removed, got %+v", got)
	}
}

func TestRoomCountReflectsAdditionsAndRemovals(t *testing.T) {
	h := NewHub()
	r1 := h.CreateRoom(1)
	r2 := h.CreateRoom(2)
	if r1 == nil || r2 == nil {
		t.Fatal("expected rooms, got nil")
	}

	if got := h.RoomCount(); got != 2 {
		t.Fatalf("expected 2 rooms, got %d", got)
	}

	h.RemoveRoom(r1.Code)
	if got := h.RoomCount(); got != 1 {
		t.Fatalf("expected 1 room after removal, got %d", got)
	}
}

func TestCleanupStaleRoomsRemovesClosedRooms(t *testing.T) {
	h := NewHub()
	r := h.CreateRoom(2)
	if r == nil {
		t.Fatal("expected room, got nil")
	}
	r.SetState(RoomClosed)

	h.CleanupStaleRooms(24 * time.Hour)
	if got := h.GetRoom(r.Code); got != nil {
		t.Fatalf("expected closed room to be removed, got %+v", got)
	}
}

func TestCleanupStaleRoomsRemovesStaleWaitingRooms(t *testing.T) {
	h := NewHub()
	r := h.CreateRoom(2)
	if r == nil {
		t.Fatal("expected room, got nil")
	}

	r.mu.Lock()
	r.CreatedAt = time.Now().Add(-2 * time.Hour)
	r.mu.Unlock()

	h.CleanupStaleRooms(30 * time.Minute)
	if got := h.GetRoom(r.Code); got != nil {
		t.Fatalf("expected stale waiting room to be removed, got %+v", got)
	}
}

func TestCleanupStaleRoomsDoesNotRemoveActiveRooms(t *testing.T) {
	h := NewHub()
	playing := h.CreateRoom(2)
	ready := h.CreateRoom(3)
	if playing == nil || ready == nil {
		t.Fatal("expected rooms, got nil")
	}

	playing.SetState(RoomPlaying)
	ready.SetState(RoomReady)

	playing.mu.Lock()
	playing.CreatedAt = time.Now().Add(-4 * time.Hour)
	playing.mu.Unlock()

	ready.mu.Lock()
	ready.CreatedAt = time.Now().Add(-4 * time.Hour)
	ready.mu.Unlock()

	h.CleanupStaleRooms(1 * time.Minute)

	if got := h.GetRoom(playing.Code); got == nil {
		t.Fatal("expected RoomPlaying room to remain")
	}
	if got := h.GetRoom(ready.Code); got == nil {
		t.Fatal("expected RoomReady room to remain")
	}
}

func TestCreateRoomConcurrent(t *testing.T) {
	h := NewHub()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	rooms := make(chan *Room, goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			rooms <- h.CreateRoom(6)
		}()
	}

	wg.Wait()
	close(rooms)

	seen := make(map[string]struct{}, goroutines)
	for r := range rooms {
		if r == nil {
			t.Fatal("expected all concurrent room creations to succeed")
		}
		if _, exists := seen[r.Code]; exists {
			t.Fatalf("duplicate code from concurrent create: %s", r.Code)
		}
		seen[r.Code] = struct{}{}
	}

	if len(seen) != goroutines {
		t.Fatalf("expected %d unique rooms, got %d", goroutines, len(seen))
	}
}

func TestRoomCodeFormatValidation(t *testing.T) {
	h := NewHub()
	for range 100 {
		code := h.generateCode()
		if !roomCodeRe.MatchString(code) {
			t.Fatalf("expected 4-char uppercase/2-9 code, got %q", code)
		}
	}
}

func TestHubShutdownNotifiesClients(t *testing.T) {
	hub := NewHub()
	registry := NewConnRegistry()

	room := hub.CreateRoom(5)
	if room == nil {
		t.Fatal("failed to create room")
	}

	hub.Shutdown(1*time.Second, registry)

	if hub.RoomCount() != 0 {
		t.Errorf("expected 0 rooms after shutdown, got %d", hub.RoomCount())
	}
}
