package client

import (
	"bytes"
	"image/color"
	"sync"

	"github.com/FirebirdRender/CommodorePet_TANK/internal/assets"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	ColorPhosphorGreen = color.RGBA{51, 255, 51, 255}
	ColorChainGreen    = color.RGBA{80, 255, 80, 255}
	ColorBlack         = color.RGBA{0, 0, 0, 255}
)

const CellSize = 20

const (
	BoardCols = 40
	BoardRows = 21
	HUDHeight = 40
)

const (
	CellEmpty      = 1
	CellWall       = 2
	CellTank1      = 3
	CellTank2      = 4
	CellBarrel1    = 5
	CellBarrel2    = 6
	CellShot       = 7
	CellMine       = 8
	CellWreckageP1 = 9
	CellWreckageP2 = 10
)

const (
	DirUp        = 1
	DirDown      = 2
	DirLeft      = 3
	DirRight     = 4
	DirUpLeft    = 5
	DirUpRight   = 6
	DirDownLeft  = 7
	DirDownRight = 8
)

type GlyphDef struct {
	Rune     rune
	Inverted bool
	FgColor  color.RGBA
}

var cellGlyphs = map[int]GlyphDef{
	CellEmpty:      {Rune: ' ', Inverted: false, FgColor: ColorBlack},
	CellWall:       {Rune: '▚', Inverted: false, FgColor: ColorPhosphorGreen},
	CellTank1:      {Rune: '*', Inverted: true, FgColor: ColorPhosphorGreen},
	CellTank2:      {Rune: '#', Inverted: true, FgColor: ColorPhosphorGreen},
	CellBarrel1:    {Rune: '─', Inverted: false, FgColor: ColorPhosphorGreen},
	CellBarrel2:    {Rune: '─', Inverted: false, FgColor: ColorPhosphorGreen},
	CellShot:       {Rune: '.', Inverted: false, FgColor: ColorPhosphorGreen},
	CellMine:       {Rune: '●', Inverted: true, FgColor: ColorPhosphorGreen},
	CellWreckageP1: {Rune: '✕', Inverted: true, FgColor: ColorPhosphorGreen},
	CellWreckageP2: {Rune: '✕', Inverted: true, FgColor: ColorPhosphorGreen},
}

var barrelDirGlyphs = map[int]rune{
	DirUp:        '│',
	DirDown:      '│',
	DirLeft:      '─',
	DirRight:     '─',
	DirUpLeft:    '╲',
	DirUpRight:   '╱',
	DirDownLeft:  '╱',
	DirDownRight: '╲',
}

type glyphKey struct {
	rune     rune
	fgColor  color.RGBA
	inverted bool
}

type GlyphCache struct {
	cache   map[glyphKey]*ebiten.Image
	faceSrc *text.GoTextFaceSource
	face    *text.GoTextFace
	mu      sync.Mutex
}

func NewGlyphCache() (*GlyphCache, error) {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(assets.FontData))
	if err != nil {
		return nil, err
	}
	face := &text.GoTextFace{
		Source: src,
		Size:   float64(CellSize),
	}
	gc := &GlyphCache{
		cache:   make(map[glyphKey]*ebiten.Image),
		faceSrc: src,
		face:    face,
	}
	for _, def := range cellGlyphs {
		gc.Get(def.Rune, def.FgColor, ColorBlack, def.Inverted)
	}
	for _, r := range barrelDirGlyphs {
		gc.Get(r, ColorPhosphorGreen, ColorBlack, false)
	}
	hudChars := "TANKSSHOMINETUVFLWRYDPB1234567890 ●"
	for _, r := range hudChars {
		gc.Get(r, ColorPhosphorGreen, ColorBlack, true)
	}
	return gc, nil
}

func (gc *GlyphCache) Get(r rune, fgColor, bgColor color.RGBA, inverted bool) *ebiten.Image {
	key := glyphKey{rune: r, fgColor: fgColor, inverted: inverted}
	gc.mu.Lock()
	if img, ok := gc.cache[key]; ok {
		gc.mu.Unlock()
		return img
	}
	gc.mu.Unlock()

	img := ebiten.NewImage(CellSize, CellSize)

	if inverted {
		img.Fill(fgColor)
		op := &text.DrawOptions{}
		op.GeoM.Translate(0, 0)
		op.ColorScale.ScaleWithColor(bgColor)
		text.Draw(img, string(r), gc.face, op)
	} else {
		img.Fill(bgColor)
		op := &text.DrawOptions{}
		op.GeoM.Translate(0, 0)
		op.ColorScale.ScaleWithColor(fgColor)
		text.Draw(img, string(r), gc.face, op)
	}

	gc.mu.Lock()
	gc.cache[key] = img
	gc.mu.Unlock()
	return img
}
