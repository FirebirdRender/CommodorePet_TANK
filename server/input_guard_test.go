package server

import (
	"sync"
	"testing"
	"time"
)

func TestInputGuard_AllowsWithinLimit(t *testing.T) {
	guard := NewInputGuard()

	for i := 0; i < MaxInputsPerSecond; i++ {
		if !guard.Allow() {
			t.Fatalf("allowed input #%d should be allowed", i+1)
		}
	}
}

func TestInputGuard_BlocksOverLimit(t *testing.T) {
	guard := NewInputGuard()

	for i := 0; i < MaxInputsPerSecond; i++ {
		guard.Allow()
	}

	if guard.Allow() {
		t.Fatal("input after limit should be blocked")
	}
}

func TestInputGuard_ResetsAfterWindow(t *testing.T) {
	guard := &InputGuard{
		maxPerSecond: 5,
	}
	guard.windowStart = time.Now().Add(-2 * InputGuardWindow)
	guard.count = 5

	if !guard.Allow() {
		t.Fatal("should allow after window reset")
	}

	for i := 0; i < 4; i++ {
		if !guard.Allow() {
			t.Fatalf("should allow input #%d in new window", i+2)
		}
	}

	if guard.Allow() {
		t.Fatal("6th input in window should be blocked")
	}
}

func TestInputGuard_Concurrent(t *testing.T) {
	guard := NewInputGuard()
	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			guard.Allow()
		}()
	}

	wg.Wait()
}
