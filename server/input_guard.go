package server

import (
	"sync"
	"time"
)

const (
	MaxInputsPerSecond = 120 // 2x the tick rate (60Hz) — generous but bounded
	InputGuardWindow   = time.Second
)

type InputGuard struct {
	mu           sync.Mutex
	count        int
	windowStart  time.Time
	maxPerSecond int
}

func NewInputGuard() *InputGuard {
	return &InputGuard{
		maxPerSecond: MaxInputsPerSecond,
	}
}

func (g *InputGuard) Allow() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	if now.Sub(g.windowStart) >= InputGuardWindow {
		g.count = 0
		g.windowStart = now
	}

	g.count++
	return g.count <= g.maxPerSecond
}

func (g *InputGuard) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.count = 0
	g.windowStart = time.Time{}
}
