package client

// SoundEffect enumerates all game sound effects.
// These are wired to AudioPlayer in audio.go.
type SoundEffect int

const (
	SFXFire SoundEffect = iota
	SFXExplosion
	SFXMine
	SFXHit
	SFXRoundOver
	SFXGameOver
	SFXCount // must be last — used for array sizing
)

// soundNames maps each SoundEffect to a human-readable name.
var soundNames = map[SoundEffect]string{
	SFXFire:      "Fire",
	SFXExplosion: "Explosion",
	SFXMine:      "Mine",
	SFXHit:       "Hit",
	SFXRoundOver: "RoundOver",
	SFXGameOver:  "GameOver",
}

// String returns a human-readable name for a sound effect.
func (s SoundEffect) String() string {
	if name, ok := soundNames[s]; ok {
		return name
	}
	return "Unknown"
}
