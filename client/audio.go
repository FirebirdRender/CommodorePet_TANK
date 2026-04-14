package client

import (
	"bytes"
	"log"
	"sync"

	"github.com/FirebirdRender/CommodorePet_TANK/internal/assets"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

const sampleRate = 48000

var soundFiles = map[SoundEffect]string{
	SFXFire:      "fire.wav",
	SFXExplosion: "explosion.wav",
	SFXMine:      "mine.wav",
	SFXHit:       "hit.wav",
	SFXRoundOver: "round_over.wav",
	SFXGameOver:  "game_over.wav",
}

type AudioPlayer struct {
	ctx      *audio.Context
	players  [SFXCount]*audio.Player
	volume   float64
	mu       sync.Mutex
	disabled bool
}

func NewAudioPlayer() *AudioPlayer {
	ap := &AudioPlayer{
		volume: 0.7,
	}

	ap.ctx = audio.NewContext(sampleRate)

	for sfx := SoundEffect(0); sfx < SFXCount; sfx++ {
		filename, ok := soundFiles[sfx]
		if !ok {
			log.Printf("audio: no file mapping for SoundEffect %d", sfx)
			continue
		}

		data, ok := assets.SoundData[filename]
		if !ok {
			log.Printf("audio: missing sound file: %s", filename)
			continue
		}

		stream, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
		if err != nil {
			log.Printf("audio: failed to decode %s: %v", filename, err)
			continue
		}

		player, err := ap.ctx.NewPlayerF32(stream)
		if err != nil {
			log.Printf("audio: failed to create player for %s: %v", filename, err)
			continue
		}

		ap.players[sfx] = player
	}

	return ap
}

func (ap *AudioPlayer) Play(sfx SoundEffect) {
	if ap.disabled || sfx < 0 || sfx >= SFXCount {
		return
	}

	ap.mu.Lock()
	player := ap.players[sfx]
	ap.mu.Unlock()

	if player == nil {
		return
	}

	if player.IsPlaying() {
		player.Rewind()
	}
	player.SetVolume(ap.volume)
	player.Play()
}

func (ap *AudioPlayer) SetVolume(v float64) {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	ap.volume = v
}

func (ap *AudioPlayer) Close() {
	// audio.Context is garbage-collected when the game terminates.
	// No explicit cleanup needed.
}
