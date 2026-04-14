package client

import (
	"image/color"
	"testing"
)

func TestCellGlyphsCoversAllCellTypes(t *testing.T) {
	expectedCells := []int{
		CellEmpty, CellWall, CellTank1, CellTank2,
		CellBarrel1, CellBarrel2, CellShot, CellMine,
		CellWreckageP1, CellWreckageP2,
	}
	for _, cell := range expectedCells {
		def, ok := cellGlyphs[cell]
		if !ok {
			t.Errorf("cellGlyphs missing entry for cell type %d", cell)
			continue
		}
		if def.Rune == '\x00' {
			t.Errorf("cellGlyphs[%d] has zero rune", cell)
		}
	}
}

func TestBarrelDirGlyphsCoversAllDirections(t *testing.T) {
	expectedDirs := []int{
		DirUp, DirDown, DirLeft, DirRight,
		DirUpLeft, DirUpRight, DirDownLeft, DirDownRight,
	}
	for _, dir := range expectedDirs {
		r, ok := barrelDirGlyphs[dir]
		if !ok {
			t.Errorf("barrelDirGlyphs missing entry for direction %d", dir)
			continue
		}
		if r == '\x00' {
			t.Errorf("barrelDirGlyphs[%d] has zero rune", dir)
		}
	}
}

func TestBarrelPos(t *testing.T) {
	tests := []struct {
		name  string
		tank  TankState
		wantX int
		wantY int
	}{
		{"up", TankState{X: 5, Y: 5, Dir: DirUp}, 5, 4},
		{"down", TankState{X: 5, Y: 5, Dir: DirDown}, 5, 6},
		{"left", TankState{X: 5, Y: 5, Dir: DirLeft}, 4, 5},
		{"right", TankState{X: 5, Y: 5, Dir: DirRight}, 6, 5},
		{"upleft", TankState{X: 5, Y: 5, Dir: DirUpLeft}, 4, 4},
		{"upright", TankState{X: 5, Y: 5, Dir: DirUpRight}, 6, 4},
		{"downleft", TankState{X: 5, Y: 5, Dir: DirDownLeft}, 4, 6},
		{"downright", TankState{X: 5, Y: 5, Dir: DirDownRight}, 6, 6},
		{"unknown_dir", TankState{X: 5, Y: 5, Dir: 99}, 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := barrelPos(&tt.tank)
			if gotX != tt.wantX || gotY != tt.wantY {
				t.Errorf("barrelPos(%+v) = (%d, %d), want (%d, %d)",
					tt.tank, gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestBarrelPosAtOrigin(t *testing.T) {
	tank := TankState{X: 0, Y: 0, Dir: DirUp}
	gotX, gotY := barrelPos(&tank)
	if gotX != 0 || gotY != -1 {
		t.Errorf("barrelPos at origin going up = (%d, %d), want (0, -1)", gotX, gotY)
	}
}

func TestBarrelPosAtEdge(t *testing.T) {
	tank := TankState{X: BoardCols - 1, Y: BoardRows - 1, Dir: DirDownRight}
	gotX, gotY := barrelPos(&tank)
	wantX, wantY := BoardCols, BoardRows
	if gotX != wantX || gotY != wantY {
		t.Errorf("barrelPos at bottom-right going downright = (%d, %d), want (%d, %d)",
			gotX, gotY, wantX, wantY)
	}
}

func TestNewGlyphCache(t *testing.T) {
	gc, err := NewGlyphCache()
	if err != nil {
		t.Fatalf("NewGlyphCache() error: %v", err)
	}
	if gc == nil {
		t.Fatal("NewGlyphCache() returned nil cache")
	}
	if gc.cache == nil {
		t.Fatal("GlyphCache.cache map is nil")
	}
	if gc.face == nil {
		t.Fatal("GlyphCache.face is nil")
	}
	if gc.faceSrc == nil {
		t.Fatal("GlyphCache.faceSrc is nil")
	}
}

func TestGlyphCacheGetReturnsNonNil(t *testing.T) {
	gc, err := NewGlyphCache()
	if err != nil {
		t.Fatalf("NewGlyphCache() error: %v", err)
	}

	testCases := []struct {
		name     string
		r        rune
		fgColor  color.RGBA
		bgColor  color.RGBA
		inverted bool
	}{
		{"wall_normal", '▚', ColorPhosphorGreen, ColorBlack, false},
		{"tank1_inverted", '*', ColorPhosphorGreen, ColorBlack, true},
		{"shot_normal", '.', ColorPhosphorGreen, ColorBlack, false},
		{"space_normal", ' ', ColorBlack, ColorBlack, false},
		{"barrel_vertical", '│', ColorPhosphorGreen, ColorBlack, false},
		{"barrel_horizontal", '─', ColorPhosphorGreen, ColorBlack, false},
		{"mine_inverted", '●', ColorPhosphorGreen, ColorBlack, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			img := gc.Get(tc.r, tc.fgColor, tc.bgColor, tc.inverted)
			if img == nil {
				t.Errorf("Get(%q, inverted=%v) returned nil image", tc.r, tc.inverted)
			}
		})
	}
}

func TestGlyphCacheGetCachesIdenticalCalls(t *testing.T) {
	gc, err := NewGlyphCache()
	if err != nil {
		t.Fatalf("NewGlyphCache() error: %v", err)
	}

	img1 := gc.Get('▚', ColorPhosphorGreen, ColorBlack, false)
	img2 := gc.Get('▚', ColorPhosphorGreen, ColorBlack, false)
	if img1 != img2 {
		t.Error("Get() with same parameters returned different images; expected caching")
	}
}

func TestGlyphCacheGetDifferentParamsReturnDifferentImages(t *testing.T) {
	gc, err := NewGlyphCache()
	if err != nil {
		t.Fatalf("NewGlyphCache() error: %v", err)
	}

	imgNormal := gc.Get('*', ColorPhosphorGreen, ColorBlack, false)
	imgInverted := gc.Get('*', ColorPhosphorGreen, ColorBlack, true)
	if imgNormal == imgInverted {
		t.Error("Get() with different inverted flags returned same image")
	}
}

func TestNewRenderer(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}
	if r == nil {
		t.Fatal("NewRenderer() returned nil renderer")
	}
	if r.cache == nil {
		t.Fatal("Renderer.cache is nil")
	}
	if r.hudFace == nil {
		t.Fatal("Renderer.hudFace is nil")
	}
}

func TestCellTypeValues(t *testing.T) {
	for i := 1; i <= 10; i++ {
		_, ok := cellGlyphs[i]
		if !ok {
			t.Errorf("cellGlyphs missing entry for cell type %d", i)
		}
	}
}

func TestDirectionValues(t *testing.T) {
	for i := 1; i <= 8; i++ {
		_, ok := barrelDirGlyphs[i]
		if !ok {
			t.Errorf("barrelDirGlyphs missing entry for direction %d", i)
		}
	}
}

func TestGlyphCachePreWarmCoversAllCellTypes(t *testing.T) {
	gc, err := NewGlyphCache()
	if err != nil {
		t.Fatalf("NewGlyphCache() error: %v", err)
	}

	for cellType, def := range cellGlyphs {
		key := glyphKey{rune: def.Rune, fgColor: def.FgColor, inverted: def.Inverted}
		gc.mu.Lock()
		_, ok := gc.cache[key]
		gc.mu.Unlock()
		if !ok {
			t.Errorf("pre-warm cache missing cell type %d (rune %q)", cellType, def.Rune)
		}
	}
}

func TestGlyphCachePreWarmCoversAllBarrelDirections(t *testing.T) {
	gc, err := NewGlyphCache()
	if err != nil {
		t.Fatalf("NewGlyphCache() error: %v", err)
	}

	for dir, r := range barrelDirGlyphs {
		key := glyphKey{rune: r, fgColor: ColorPhosphorGreen, inverted: false}
		gc.mu.Lock()
		_, ok := gc.cache[key]
		gc.mu.Unlock()
		if !ok {
			t.Errorf("pre-warm cache missing barrel direction %d (rune %q)", dir, r)
		}
	}
}

func TestColorConstants(t *testing.T) {
	if ColorPhosphorGreen.R != 51 || ColorPhosphorGreen.G != 255 || ColorPhosphorGreen.B != 51 {
		t.Errorf("ColorPhosphorGreen = %v, want {51 255 51 255}", ColorPhosphorGreen)
	}
	if ColorChainGreen.R != 80 || ColorChainGreen.G != 255 || ColorChainGreen.B != 80 {
		t.Errorf("ColorChainGreen = %v, want {80 255 80 255}", ColorChainGreen)
	}
	if ColorBlack.R != 0 || ColorBlack.G != 0 || ColorBlack.B != 0 {
		t.Errorf("ColorBlack = %v, want {0 0 0 255}", ColorBlack)
	}
}

func TestDimensionConstants(t *testing.T) {
	if CellSize != 20 {
		t.Errorf("CellSize = %d, want 20", CellSize)
	}
	if BoardCols != 40 {
		t.Errorf("BoardCols = %d, want 40", BoardCols)
	}
	if BoardRows != 21 {
		t.Errorf("BoardRows = %d, want 21", BoardRows)
	}
	if HUDHeight != 40 {
		t.Errorf("HUDHeight = %d, want 40", HUDHeight)
	}
}
