package client

import (
	"fmt"
	"image/color"
	"testing"
)

func TestCellGlyphsCoversAllCellTypes(t *testing.T) {
	expectedCells := []int{
		CellEmpty, CellWall, CellTank1, CellTank2,
		CellBarrel1, CellBarrel2, CellShot, CellMine,
		CellWreckageP1, CellWreckageP2, CellBorder,
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
	for i := 1; i <= 11; i++ {
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
	if HUDP1End != 19 {
		t.Errorf("HUDP1End = %d, want 19", HUDP1End)
	}
	if HUDP2Start != 21 {
		t.Errorf("HUDP2Start = %d, want 21", HUDP2Start)
	}
}

func TestHUDLabelRow(t *testing.T) {
	tests := []struct {
		name    string
		aiLevel int
		want    string
	}{
		{"no AI", 0, "TANKS  SHOTS  MINES"},
		{"AI level 1", 1, "TANKS SHOTS MINES 1"},
		{"AI level 5", 5, "TANKS SHOTS MINES 5"},
		{"AI level 9", 9, "TANKS SHOTS MINES 9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hudLabelRow(tt.aiLevel)
			if len(got) != 19 {
				t.Errorf("hudLabelRow(%d) length = %d, want 19 (got %q)", tt.aiLevel, len(got), got)
			}
			if got != tt.want {
				t.Errorf("hudLabelRow(%d) = %q, want %q", tt.aiLevel, got, tt.want)
			}
		})
	}
}

func TestHUDValueRow(t *testing.T) {
	tests := []struct {
		name      string
		lives     int
		shots     int
		mines     int
		wantLen   int
		wantStart string
	}{
		{"standard", 3, 6, 0, 19, "  3      6      0"},
		{"high values", 1, 10, 3, 19, "  1      10     3"},
		{"zeros", 0, 0, 0, 19, "  0      0      0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hudValueRow(tt.lives, tt.shots, tt.mines)
			if len(got) != tt.wantLen {
				t.Errorf("hudValueRow(%d,%d,%d) length = %d, want %d", tt.lives, tt.shots, tt.mines, len(got), tt.wantLen)
			}
		})
	}
}

func TestHUDStatusMessage(t *testing.T) {
	tests := []struct {
		name      string
		shotsLeft int
		maxShots  int
		lives     int
		isWinner  bool
		want      string
	}{
		{"winner", 5, 10, 3, true, "THE WINNER"},
		{"out of shots", 0, 10, 3, false, "OUT OF SHOTS"},
		{"low shots 20pct", 2, 10, 3, false, "LOW SHOTS"},
		{"not low shots", 3, 10, 3, false, ""},
		{"last tank", 5, 10, 1, false, "LAST TANK"},
		{"normal", 5, 10, 3, false, ""},
		{"winner overrides low shots", 0, 10, 1, true, "THE WINNER"},
		{"out of shots overrides last tank", 0, 10, 1, false, "OUT OF SHOTS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hudStatusMessage(tt.shotsLeft, tt.maxShots, tt.lives, tt.isWinner)
			if got != tt.want {
				t.Errorf("hudStatusMessage(%d,%d,%d,%v) = %q, want %q",
					tt.shotsLeft, tt.maxShots, tt.lives, tt.isWinner, got, tt.want)
			}
		})
	}
}

func TestDifficultyToMaxShots(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{0, 6},
		{1, 6},
		{2, 8},
		{4, 8},
		{5, 10},
		{7, 10},
		{8, 12},
		{9, 12},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("level_%d", tt.level), func(t *testing.T) {
			got := difficultyToMaxShots(tt.level)
			if got != tt.want {
				t.Errorf("difficultyToMaxShots(%d) = %d, want %d", tt.level, got, tt.want)
			}
		})
	}
}

func TestBorderDetection(t *testing.T) {
	borderTests := []struct {
		name string
		x    int
		y    int
		want bool
	}{
		{"top-left corner", 0, 0, true},
		{"top-right corner", BoardCols - 1, 0, true},
		{"bottom-left corner", 0, BoardRows - 1, true},
		{"bottom-right corner", BoardCols - 1, BoardRows - 1, true},
		{"top edge middle", 20, 0, true},
		{"bottom edge middle", 20, BoardRows - 1, true},
		{"left edge middle", 0, 10, true},
		{"right edge middle", BoardCols - 1, 10, true},
		{"interior", 5, 5, false},
		{"interior near edge", 1, 1, false},
	}
	for _, tt := range borderTests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.x == 0 || tt.x == BoardCols-1 || tt.y == 0 || tt.y == BoardRows-1
			if got != tt.want {
				t.Errorf("border(%d,%d) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestCellBorderGlyph(t *testing.T) {
	def, ok := cellGlyphs[CellBorder]
	if !ok {
		t.Fatalf("cellGlyphs missing CellBorder entry")
	}
	if def.Rune != '●' {
		t.Errorf("CellBorder.Rune = %q, want '●'", def.Rune)
	}
	if !def.Inverted {
		t.Error("CellBorder.Inverted = false, want true")
	}
	if def.FgColor != ColorPhosphorGreen {
		t.Errorf("CellBorder.FgColor = %v, want ColorPhosphorGreen", def.FgColor)
	}
}

func TestDrawHUD_PanelLayout(t *testing.T) {
	t.Run("P1 panel range", func(t *testing.T) {
		if HUDP1End != 19 {
			t.Errorf("HUDP1End = %d, want 19", HUDP1End)
		}
	})

	t.Run("P2 panel range", func(t *testing.T) {
		if HUDP2Start != 21 {
			t.Errorf("HUDP2Start = %d, want 21", HUDP2Start)
		}
	})

	t.Run("Separator columns", func(t *testing.T) {
		sepWidth := HUDP2Start - HUDP1End
		if sepWidth != 2 {
			t.Errorf("Separator width = %d, want 2 (HUDP2Start - HUDP1End)", sepWidth)
		}
	})

	t.Run("Total width", func(t *testing.T) {
		total := HUDP1End + 2 + (BoardCols - HUDP2Start)
		if total != BoardCols {
			t.Errorf("Total width = %d, want %d (HUDP1End + 2 + (BoardCols - HUDP2Start))", total, BoardCols)
		}
	})

	t.Run("HUD label row with AI", func(t *testing.T) {
		got := hudLabelRow(5)
		if len(got) != 19 {
			t.Errorf("hudLabelRow(5) length = %d, want 19", len(got))
		}
		if got[18] != '5' {
			t.Errorf("hudLabelRow(5)[18] = %q, want '5'", got[18])
		}
	})

	t.Run("HUD value row single digits", func(t *testing.T) {
		got := hudValueRow(1, 1, 1)
		if len(got) != 19 {
			t.Errorf("hudValueRow(1,1,1) length = %d, want 19", len(got))
		}
	})
}

func TestDrawGrid_CellGlyphSelection(t *testing.T) {
	t.Run("border glyph definition", func(t *testing.T) {
		def, ok := cellGlyphs[CellBorder]
		if !ok {
			t.Fatalf("cellGlyphs missing CellBorder entry")
		}
		if def.Rune != '●' {
			t.Errorf("CellBorder.Rune = %q, want '●'", def.Rune)
		}
		if !def.Inverted {
			t.Error("CellBorder.Inverted = false, want true")
		}
		if def.FgColor != ColorPhosphorGreen {
			t.Errorf("CellBorder.FgColor = %v, want ColorPhosphorGreen", def.FgColor)
		}
	})

	t.Run("interior wall glyph definition", func(t *testing.T) {
		def, ok := cellGlyphs[CellWall]
		if !ok {
			t.Fatalf("cellGlyphs missing CellWall entry")
		}
		if def.Rune != '▚' {
			t.Errorf("CellWall.Rune = %q, want '▚'", def.Rune)
		}
		if def.Inverted {
			t.Error("CellWall.Inverted = true, want false")
		}
		if def.FgColor != ColorPhosphorGreen {
			t.Errorf("CellWall.FgColor = %v, want ColorPhosphorGreen", def.FgColor)
		}
	})

	t.Run("CellEmpty glyph definition", func(t *testing.T) {
		def, ok := cellGlyphs[CellEmpty]
		if !ok {
			t.Fatalf("cellGlyphs missing CellEmpty entry")
		}
		if def.Rune != ' ' {
			t.Errorf("CellEmpty.Rune = %q, want ' '", def.Rune)
		}
		if def.Inverted {
			t.Error("CellEmpty.Inverted = true, want false")
		}
		if def.FgColor != ColorBlack {
			t.Errorf("CellEmpty.FgColor = %v, want ColorBlack", def.FgColor)
		}
	})

	t.Run("CellBorder differs from CellWall", func(t *testing.T) {
		borderDef := cellGlyphs[CellBorder]
		wallDef := cellGlyphs[CellWall]
		if borderDef.Rune == wallDef.Rune {
			t.Error("CellBorder.Rune == CellWall.Rune, expected different runes")
		}
		if borderDef.Inverted == wallDef.Inverted {
			t.Error("CellBorder.Inverted == CellWall.Inverted, expected different inverted flags")
		}
	})

	t.Run("border detection at corners", func(t *testing.T) {
		corners := [][2]int{
			{0, 0},
			{BoardCols - 1, 0},
			{0, BoardRows - 1},
			{BoardCols - 1, BoardRows - 1},
		}
		for _, pos := range corners {
			x, y := pos[0], pos[1]
			isBorder := x == 0 || x == BoardCols-1 || y == 0 || y == BoardRows-1
			if !isBorder {
				t.Errorf("corner (%d,%d) should be border position", x, y)
			}
		}
	})

	t.Run("interior positions not border", func(t *testing.T) {
		x, y := 5, 5
		isBorder := x == 0 || x == BoardCols-1 || y == 0 || y == BoardRows-1
		if isBorder {
			t.Errorf("position (%d,%d) should NOT be border position", x, y)
		}
	})
}

func TestDrawHUD_StatusMessagePriority(t *testing.T) {
	t.Run("winner overrides all", func(t *testing.T) {
		got := hudStatusMessage(0, 10, 1, true)
		if got != "THE WINNER" {
			t.Errorf("hudStatusMessage(0,10,1,true) = %q, want %q", got, "THE WINNER")
		}
	})

	t.Run("zero shots override last tank", func(t *testing.T) {
		got := hudStatusMessage(0, 10, 1, false)
		if got != "OUT OF SHOTS" {
			t.Errorf("hudStatusMessage(0,10,1,false) = %q, want %q", got, "OUT OF SHOTS")
		}
	})

	t.Run("maxShots zero no status", func(t *testing.T) {
		got := hudStatusMessage(0, 0, 3, false)
		if got != "" {
			t.Errorf("hudStatusMessage(0,0,3,false) = %q, want %q", got, "")
		}
	})

	t.Run("low shots at 20 percent", func(t *testing.T) {
		got := hudStatusMessage(2, 10, 3, false)
		if got != "LOW SHOTS" {
			t.Errorf("hudStatusMessage(2,10,3,false) = %q, want %q", got, "LOW SHOTS")
		}
	})

	t.Run("just above threshold", func(t *testing.T) {
		got := hudStatusMessage(3, 10, 3, false)
		if got != "" {
			t.Errorf("hudStatusMessage(3,10,3,false) = %q, want %q", got, "")
		}
	})

	t.Run("last tank with enough shots", func(t *testing.T) {
		got := hudStatusMessage(5, 10, 1, false)
		if got != "LAST TANK" {
			t.Errorf("hudStatusMessage(5,10,1,false) = %q, want %q", got, "LAST TANK")
		}
	})

	t.Run("low shots priority over last tank", func(t *testing.T) {
		got := hudStatusMessage(2, 10, 1, false)
		if got != "LOW SHOTS" {
			t.Errorf("hudStatusMessage(2,10,1,false) = %q, want %q", got, "LOW SHOTS")
		}
	})
}
