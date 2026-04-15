package client

import (
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// crtEnabled is set once at startup from the TANK_CRT environment variable.
var crtEnabled bool

func init() {
	v := os.Getenv("TANK_CRT")
	crtEnabled = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// crtShaderSource is the Kage shader for CRT scanlines + vignette.
const crtShaderSource = `
package main

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	result := imageSrc0At(texCoord)

	// Scanlines: darken every other row
	scanline := mod(floor(texCoord.y * 460.0), 2.0)
	if scanline < 1.0 {
		result.rgb *= 0.85
	}

	// Vignette: darken edges and corners
	dist := distance(texCoord, vec2(0.5, 0.5)) * 1.4
	vignette := clamp(1.0 - dist * dist * 0.3, 0.0, 1.0)
	result.rgb *= vignette

	return result
}
`

var (
	crtShader *ebiten.Shader
	offscreen *ebiten.Image
)

// initCRTShader compiles the CRT shader. Returns an error if compilation fails.
func initCRTShader() error {
	var err error
	crtShader, err = ebiten.NewShader([]byte(crtShaderSource))
	return err
}

// initOffscreen creates the offscreen buffer for two-step CRT rendering.
func initOffscreen() {
	offscreen = ebiten.NewImage(800, 480)
}

// isCRTEnabled returns true if CRT post-processing is enabled.
func isCRTEnabled() bool {
	return crtEnabled && crtShader != nil
}
