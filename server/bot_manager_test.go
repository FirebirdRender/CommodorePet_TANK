package server

import (
	"os"
	"testing"
)

func TestBotManager_IsEnabledTrueWhenEnvSet(t *testing.T) {
	orig := os.Getenv("TANK_ENABLE_BOTS")
	os.Setenv("TANK_ENABLE_BOTS", "1")
	defer os.Setenv("TANK_ENABLE_BOTS", orig)

	hub := NewHub()
	tokens := NewTokenStore()
	handler := NewWSHandler(hub, tokens)
	bm := NewBotManager(hub, handler, tokens, "localhost:8080")

	if !bm.IsEnabled() {
		t.Fatal("expected bot mode to be enabled when TANK_ENABLE_BOTS=1")
	}
}

func TestBotManager_IsEnabledFalseWhenEnvNotSet(t *testing.T) {
	orig := os.Getenv("TANK_ENABLE_BOTS")
	os.Unsetenv("TANK_ENABLE_BOTS")
	defer os.Setenv("TANK_ENABLE_BOTS", orig)

	hub := NewHub()
	tokens := NewTokenStore()
	handler := NewWSHandler(hub, tokens)
	bm := NewBotManager(hub, handler, tokens, "localhost:8080")

	if bm.IsEnabled() {
		t.Fatal("expected bot mode to be disabled when TANK_ENABLE_BOTS is not set")
	}
}

func TestBotManager_AssignBotToNonExistentRoom(t *testing.T) {
	hub := NewHub()
	tokens := NewTokenStore()
	handler := NewWSHandler(hub, tokens)
	bm := NewBotManager(hub, handler, tokens, "localhost:8080")

	err := bm.AssignBotToRoom("NONEXISTENT", "test")
	if err == nil {
		t.Fatal("expected error when assigning bot to non-existent room")
	}
}

func TestBotManager_ReleaseBotNonExistent(t *testing.T) {
	hub := NewHub()
	tokens := NewTokenStore()
	handler := NewWSHandler(hub, tokens)
	bm := NewBotManager(hub, handler, tokens, "localhost:8080")

	bm.ReleaseBot("NONEXISTENT")
}

func TestBotManager_HealthSnapshot(t *testing.T) {
	hub := NewHub()
	tokens := NewTokenStore()
	handler := NewWSHandler(hub, tokens)
	bm := NewBotManager(hub, handler, tokens, "localhost:8080")

	snapshot := bm.HealthSnapshot()
	if snapshot["enabled"] != "false" {
		t.Fatalf("expected enabled=false when TANK_ENABLE_BOTS not set, got %s", snapshot["enabled"])
	}
}
