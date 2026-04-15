package server

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type TokenEntry struct {
	RoomCode   string
	PlayerID   int
	PlayerName string
	CreatedAt  time.Time
}

type TokenStore struct {
	mu     sync.Mutex
	tokens map[string]TokenEntry
}

func NewTokenStore() *TokenStore {
	return &TokenStore{
		tokens: make(map[string]TokenEntry),
	}
}

func (ts *TokenStore) GenerateToken(roomCode string, playerID int, playerName string) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed")
	}
	token := hex.EncodeToString(b)

	ts.tokens[token] = TokenEntry{
		RoomCode:   roomCode,
		PlayerID:   playerID,
		PlayerName: playerName,
		CreatedAt:  time.Now(),
	}

	return token
}

func (ts *TokenStore) ValidateToken(token string) (TokenEntry, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	entry, ok := ts.tokens[token]
	return entry, ok
}

func (ts *TokenStore) RemoveToken(token string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.tokens, token)
}

func (ts *TokenStore) CleanupStaleTokens(maxAge time.Duration) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	now := time.Now()
	for token, entry := range ts.tokens {
		if now.Sub(entry.CreatedAt) > maxAge {
			delete(ts.tokens, token)
		}
	}
}
