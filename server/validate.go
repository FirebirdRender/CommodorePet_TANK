package server

import (
	"fmt"
)

func validateDifficulty(d int) error {
	if d < 1 || d > 10 {
		return fmt.Errorf("difficulty must be between 1 and 10, got %d", d)
	}
	return nil
}

func validatePlayerName(name string) error {
	if len(name) < 1 {
		return fmt.Errorf("player name must not be empty")
	}
	if len(name) > 16 {
		return fmt.Errorf("player name must be at most 16 characters, got %d", len(name))
	}
	for _, c := range name {
		if c < 0x20 || c > 0x7E {
			return fmt.Errorf("player name contains invalid character")
		}
	}
	return nil
}

func validateRoomCode(code string) error {
	if len(code) != 4 {
		return fmt.Errorf("room code must be exactly 4 characters, got %d", len(code))
	}
	for _, c := range code {
		if !isRoomCodeChar(c) {
			return fmt.Errorf("room code contains invalid character %q", c)
		}
	}
	return nil
}

func isRoomCodeChar(c rune) bool {
	// Allow A-Z and 2-9 (unambiguous alphabet matching T8)
	return (c >= 'A' && c <= 'Z') || (c >= '2' && c <= '9')
}
