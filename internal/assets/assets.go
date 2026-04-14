// Package assets provides embedded game assets for the TANK! client.
package assets

import _ "embed"

var SoundData = map[string][]byte{
	"fire.wav":       _fireWav,
	"explosion.wav":  _explosionWav,
	"mine.wav":       _mineWav,
	"hit.wav":        _hitWav,
	"round_over.wav": _roundOverWav,
	"game_over.wav":  _gameOverWav,
}

//go:embed sounds/fire.wav
var _fireWav []byte

//go:embed sounds/explosion.wav
var _explosionWav []byte

//go:embed sounds/mine.wav
var _mineWav []byte

//go:embed sounds/hit.wav
var _hitWav []byte

//go:embed sounds/round_over.wav
var _roundOverWav []byte

//go:embed sounds/game_over.wav
var _gameOverWav []byte

// FontData is the PetMe64 font binary data embedded at build time.
//
//go:embed fonts/PetMe64.ttf
var FontData []byte
