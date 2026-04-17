package botsdk

import (
	"fmt"
	"testing"
	"time"
)

func TestReconnectPolicyBackoff(t *testing.T) {
	p := &ReconnectPolicy{
		BaseBackoff: 1 * time.Second,
		MaxBackoff:  30 * time.Second,
		MaxRetries:  5,
	}

	tests := []struct {
		attempt int
		min     time.Duration
		max     time.Duration
	}{
		{0, 0, 2 * time.Second},   // 1s + jitter
		{1, 0, 3 * time.Second},   // 2s + jitter
		{2, 0, 6 * time.Second},   // 4s + jitter
		{3, 0, 12 * time.Second},  // 8s + jitter
		{4, 0, 24 * time.Second},  // 16s + jitter
		{5, 30 * time.Second, 30 * time.Second}, // capped at max
		{6, 0, 0}, // exceeds max retries
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			backoff := p.Backoff(tt.attempt)
			if tt.attempt > p.MaxRetries {
				if backoff != 0 {
					t.Errorf("expected 0 backoff for attempt > max, got %v", backoff)
				}
			} else {
				base := time.Duration(1<<tt.attempt) * time.Second
				maxExpected := base * 2
				if tt.attempt == 5 {
					maxExpected = 30 * time.Second
				}
				if backoff < 0 || backoff > maxExpected {
					t.Errorf("backoff %v out of expected range [0, %v]", backoff, maxExpected)
				}
			}
		})
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("ws://localhost:8080/ws", "TEST", 0, "token", "bot")

	if c.ServerURL != "ws://localhost:8080/ws" {
		t.Errorf("expected ServerURL %q, got %q", "ws://localhost:8080/ws", c.ServerURL)
	}
	if c.RoomCode != "TEST" {
		t.Errorf("expected RoomCode %q, got %q", "TEST", c.RoomCode)
	}
	if c.PlayerID != 0 {
		t.Errorf("expected PlayerID %d, got %d", 0, c.PlayerID)
	}
	if c.MaxRetries != 5 {
		t.Errorf("expected MaxRetries %d, got %d", 5, c.MaxRetries)
	}
	if c.BaseBackoff != time.Second {
		t.Errorf("expected BaseBackoff %v, got %v", time.Second, c.BaseBackoff)
	}
	if c.MaxBackoff != 30*time.Second {
		t.Errorf("expected MaxBackoff %v, got %v", 30*time.Second, c.MaxBackoff)
	}

	if cap(c.sendCh) != 16 {
		t.Errorf("expected sendCh cap 16, got %d", cap(c.sendCh))
	}
}
