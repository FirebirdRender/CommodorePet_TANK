package botsdk

import (
	"math/rand"
	"time"
)

// ReconnectPolicy defines the reconnect strategy.
type ReconnectPolicy struct {
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

// DefaultReconnectPolicy returns a sensible default reconnect policy.
func DefaultReconnectPolicy() *ReconnectPolicy {
	return &ReconnectPolicy{
		MaxRetries:  5,
		BaseBackoff: time.Second,
		MaxBackoff:  30 * time.Second,
	}
}

// Backoff calculates the backoff duration for a given attempt.
func (p *ReconnectPolicy) Backoff(attempt int) time.Duration {
	if attempt > p.MaxRetries {
		return 0
	}

	// Exponential backoff: base * 2^attempt
	backoff := p.BaseBackoff * time.Duration(1<<attempt)

	// Add jitter: ±25% of backoff
	jitter := time.Duration(float64(backoff) * 0.25 * (rand.Float64() - 0.5))
	backoff += jitter

	// Cap at max backoff
	if backoff > p.MaxBackoff {
		backoff = p.MaxBackoff
	}

	return backoff
}
