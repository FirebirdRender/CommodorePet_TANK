package server

import "testing"

func TestValidateDifficulty(t *testing.T) {
	tests := []struct {
		input int
		valid bool
	}{
		{0, false},
		{1, true},
		{5, true},
		{10, true},
		{11, false},
		{-1, false},
		{100, false},
	}
	for _, tt := range tests {
		err := validateDifficulty(tt.input)
		if (err == nil) != tt.valid {
			t.Errorf("validateDifficulty(%d): valid=%v, got err=%v", tt.input, tt.valid, err)
		}
	}
}

func TestValidatePlayerName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"Alice", true},
		{"B", true},
		{"", false},
		{"A name", true}, // 8 chars, valid
		{"1234567890123456", true},
		{"12345678901234567", false},
		{"name\twith\ttabs", false}, // control chars
		{"namewith🎉", false},        // non-ASCII
	}
	for _, tt := range tests {
		err := validatePlayerName(tt.name)
		if (err == nil) != tt.valid {
			t.Errorf("validatePlayerName(%q): valid=%v, got err=%v", tt.name, tt.valid, err)
		}
	}
}

func TestValidateRoomCode(t *testing.T) {
	tests := []struct {
		code  string
		valid bool
	}{
		{"ABCD", true},
		{"A2B9", true},
		{"abc", false},
		{"ABCD1", false},
		{"ABC", false},
		{"AB29", true},
		{"abCD", false},
		{"AB D", false},
	}
	for _, tt := range tests {
		err := validateRoomCode(tt.code)
		if (err == nil) != tt.valid {
			t.Errorf("validateRoomCode(%q): valid=%v, got err=%v", tt.code, tt.valid, err)
		}
	}
}
