package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

const (
	sampleRate = 48000
	channels   = 1
	bitDepth   = 16
)

func writeWAV(filename string, samples []int16) error {
	dataLen := len(samples) * 2
	fileSize := 36 + dataLen

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, uint32(fileSize))
	f.Write([]byte("WAVE"))

	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(1))
	binary.Write(f, binary.LittleEndian, uint16(channels))
	binary.Write(f, binary.LittleEndian, uint32(sampleRate))
	binary.Write(f, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(f, binary.LittleEndian, uint16(2))
	binary.Write(f, binary.LittleEndian, uint16(bitDepth))

	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, uint32(dataLen))

	for _, s := range samples {
		binary.Write(f, binary.LittleEndian, s)
	}

	return nil
}

func generateTone(freq float64, duration float64, volume float64) []int16 {
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]int16, numSamples)
	for i := range samples {
		t := float64(i) / float64(sampleRate)
		amplitude := volume * math.MaxInt16
		samples[i] = int16(amplitude * math.Sin(2*math.Pi*freq*t))
	}
	return samples
}

func generateNoise(duration float64, volume float64) []int16 {
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]int16, numSamples)
	seed := uint32(42)
	for i := range samples {
		seed = seed*1103515245 + 12345
		val := (seed >> 16) & 0x7FFF
		samples[i] = int16(float64(val-0x3FFF) * volume)
	}
	return samples
}

func applyDecay(samples []int16, decayRate float64) []int16 {
	for i := range samples {
		factor := math.Exp(-decayRate * float64(i) / float64(sampleRate))
		samples[i] = int16(float64(samples[i]) * factor)
	}
	return samples
}

func main() {
	outDir := filepath.Join("internal", "assets", "sounds")
	os.MkdirAll(outDir, 0755)

	sounds := map[string][]int16{
		"fire.wav":       applyDecay(generateTone(880, 0.1, 0.5), 30),
		"explosion.wav":  applyDecay(generateNoise(0.3, 0.8), 8),
		"mine.wav":       applyDecay(generateTone(220, 0.15, 0.6), 15),
		"hit.wav":        applyDecay(generateTone(440, 0.08, 0.5), 40),
		"round_over.wav": generateTone(660, 0.3, 0.5),
		"game_over.wav":  generateTone(330, 0.6, 0.6),
	}

	for name, samples := range sounds {
		path := filepath.Join(outDir, name)
		if err := writeWAV(path, samples); err != nil {
			fmt.Printf("ERROR: %v\n", err)
		} else {
			fmt.Printf("Generated: %s (%d samples)\n", path, len(samples))
		}
	}
}
