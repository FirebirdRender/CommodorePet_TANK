package client

import (
	"bytes"
	"fmt"
	"image/color"

	"github.com/FirebirdRender/CommodorePet_TANK/internal/assets"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Renderer struct {
	cache    *GlyphCache
	hudFace  *text.GoTextFace
	cellFace *text.GoTextFace
}

func NewRenderer() (*Renderer, error) {
	gc, err := NewGlyphCache()
	if err != nil {
		return nil, err
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(assets.FontData))
	if err != nil {
		return nil, err
	}
	hudFace := &text.GoTextFace{
		Source: src,
		Size:   16,
	}
	cellFace := &text.GoTextFace{
		Source: src,
		Size:   float64(CellSize),
	}
	return &Renderer{
		cache:    gc,
		hudFace:  hudFace,
		cellFace: cellFace,
	}, nil
}

func difficultyToMaxShots(level int) int {
	switch {
	case level <= 1:
		return 6
	case level <= 4:
		return 8
	case level <= 7:
		return 10
	default:
		return 12
	}
}

func hudLabelRow(aiLevel int) string {
	if aiLevel > 0 {
		s := fmt.Sprintf("TANKS SHOTS MINES %d", aiLevel)
		if len(s) > 19 {
			s = s[:19]
		}
		return fmt.Sprintf("%-19s", s)
	}
	return "TANKS  SHOTS  MINES"[:19]
}

func hudValueRow(lives, shots, mines int) string {
	s := fmt.Sprintf("  %d      %d      %d", lives, shots, mines)
	if len(s) > 19 {
		s = s[:19]
	}
	return fmt.Sprintf("%-19s", s)
}

func hudStatusMessage(shotsLeft, maxShots, lives int, isWinner bool) string {
	if isWinner {
		return "THE WINNER"
	}
	if shotsLeft == 0 && maxShots > 0 {
		return "OUT OF SHOTS"
	}
	if maxShots > 0 && shotsLeft <= maxShots/5 {
		return "LOW SHOTS"
	}
	if lives == 1 {
		return "LAST TANK"
	}
	return ""
}

func (r *Renderer) Draw(screen *ebiten.Image, state *GameState) {
	screen.Fill(ColorBlack)

	if state.Phase == PhasePlaying || state.Phase == PhaseRoundOver {
		if state.Phase == PhaseRoundOver {
			r.drawRoundOverOverlay(screen, state)
		}
		r.drawGrid(screen, state)
		r.drawExplosions(screen, state)
		r.drawHUD(screen, state)
		if state.EscConfirmPending {
			r.drawEscConfirmOverlay(screen, state)
		}
	} else if state.Phase == PhaseDisconnected {
		r.drawDisconnected(screen, state)
	} else if state.Phase == PhaseGameOver {
		r.drawGrid(screen, state)
		r.drawHUD(screen, state)
		r.drawGameOverOverlay(screen, state)
	} else if state.Phase == PhaseSpectating {
		r.drawGrid(screen, state)
		r.drawExplosions(screen, state)
		r.drawHUD(screen, state)
		r.drawSpectatingLabel(screen, state)
	} else if state.Phase == PhaseWaitingReconnect {
		r.drawGrid(screen, state)
		r.drawExplosions(screen, state)
		r.drawHUD(screen, state)
		r.drawWaitingReconnectOverlay(screen, state)
	}
}

func (r *Renderer) drawRoundOverOverlay(screen *ebiten.Image, state *GameState) {
	face := &text.GoTextFace{
		Source: r.hudFace.Source,
		Size:   32,
	}

	textStr := "ROUND OVER"
	if state.Winner > 0 {
		textStr = fmt.Sprintf("PLAYER %d WINS THE ROUND!", state.Winner)
	}

	w, _ := text.Measure(textStr, face, 0)
	x := float64(BoardCols*CellSize)/2 - w/2
	y := float64(BoardRows*CellSize)/2 + HUDHeight

	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(ColorPhosphorGreen)
	text.Draw(screen, textStr, face, op)

	subFace := &text.GoTextFace{
		Source: r.hudFace.Source,
		Size:   16,
	}
	subText := "NEXT ROUND STARTING..."
	sw, _ := text.Measure(subText, subFace, 0)
	sx := float64(BoardCols*CellSize)/2 - sw/2
	sy := y + 40
	sop := &text.DrawOptions{}
	sop.GeoM.Translate(sx, sy)
	sop.ColorScale.ScaleWithColor(ColorPhosphorGreen)
	text.Draw(screen, subText, subFace, sop)
}

func (r *Renderer) drawGrid(screen *ebiten.Image, state *GameState) {
	if state.Grid == nil || len(state.Grid) == 0 {
		return
	}

	barrelPositions := make(map[[2]int]int)
	for _, bw := range state.BarrelWreckage {
		barrelPositions[[2]int{bw.X, bw.Y}] = bw.Dir
	}
	bodyPositions := make(map[[2]int]bool)
	for _, pos := range state.BarrelHitBodies {
		bodyPositions[pos] = true
	}

	for y, row := range state.Grid {
		for x, cellType := range row {
			pos := [2]int{x, y}
			var glyph *ebiten.Image

			isBorder := x == 0 || x == BoardCols-1 || y == 0 || y == BoardRows-1

			if isBorder && (cellType == CellWall || cellType == CellBorder) {
				borderDef := cellGlyphs[CellBorder]
				glyph = r.cache.Get(borderDef.Rune, borderDef.FgColor, ColorBlack, borderDef.Inverted)
			} else if cellType == CellWreckageP1 || cellType == CellWreckageP2 {
				if dir, ok := barrelPositions[pos]; ok {
					curlRune := '/'
					if dir == DirUp || dir == DirDownRight || dir == DirLeft {
						curlRune = '\\'
					}
					glyph = r.cache.Get(curlRune, ColorPhosphorGreen, ColorBlack, true)
				} else if bodyPositions[pos] {
					glyph = r.cache.Get('✕', ColorPhosphorGreen, ColorBlack, true)
				} else {
					glyph = r.cache.Get('▗', ColorPhosphorGreen, ColorBlack, false)
				}
			} else {
				def, ok := cellGlyphs[cellType]
				if !ok {
					def = cellGlyphs[CellEmpty]
				}
				glyph = r.cache.Get(def.Rune, def.FgColor, ColorBlack, def.Inverted)
			}

			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x*CellSize), float64(y*CellSize+HUDHeight))
			screen.DrawImage(glyph, op)
		}
	}

	for i := range state.Tanks {
		tank := &state.Tanks[i]
		if !tank.Active {
			continue
		}
		barrelRune, ok := barrelDirGlyphs[tank.Dir]
		if !ok {
			barrelRune = '─'
		}
		glyph := r.cache.Get(barrelRune, ColorPhosphorGreen, ColorBlack, false)
		bx, by := barrelPos(tank)
		if by >= 0 && by < BoardRows && bx >= 0 && bx < BoardCols {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(bx*CellSize), float64(by*CellSize+HUDHeight))
			screen.DrawImage(glyph, op)
		}
	}

	for _, mine := range state.Mines {
		if mine.Visible {
			glyph := r.cache.Get('●', ColorPhosphorGreen, ColorBlack, true)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(mine.X*CellSize), float64(mine.Y*CellSize+HUDHeight))
			screen.DrawImage(glyph, op)
		}
	}

	for _, shot := range state.Shots {
		if shot.Active {
			glyph := r.cache.Get('.', ColorPhosphorGreen, ColorBlack, false)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(shot.X*CellSize), float64(shot.Y*CellSize+HUDHeight))
			screen.DrawImage(glyph, op)
		}
	}
}

func barrelPos(tank *TankState) (int, int) {
	switch tank.Dir {
	case DirUp:
		return tank.X, tank.Y - 1
	case DirDown:
		return tank.X, tank.Y + 1
	case DirLeft:
		return tank.X - 1, tank.Y
	case DirRight:
		return tank.X + 1, tank.Y
	case DirUpLeft:
		return tank.X - 1, tank.Y - 1
	case DirUpRight:
		return tank.X + 1, tank.Y - 1
	case DirDownLeft:
		return tank.X - 1, tank.Y + 1
	case DirDownRight:
		return tank.X + 1, tank.Y + 1
	default:
		return tank.X, tank.Y
	}
}

func (r *Renderer) drawExplosions(screen *ebiten.Image, state *GameState) {
	for _, e := range state.Explosions {
		clr := ColorPhosphorGreen
		if e.IsChainReaction {
			clr = ColorChainGreen
		}
		glyph := r.cache.Get('●', clr, ColorBlack, true)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(e.X*CellSize), float64(e.Y*CellSize+HUDHeight))
		screen.DrawImage(glyph, op)
	}
}

func (r *Renderer) drawHUD(screen *ebiten.Image, state *GameState) {
	// 1. Fill entire HUD bar with phosphor green
	vector.FillRect(screen, 0, 0, float32(BoardCols*CellSize), float32(HUDHeight),
		ColorPhosphorGreen, false)

	// 2. Center separator: 2 columns of inverted circles (cols 19-20)
	sepGlyph := r.cache.Get('●', ColorPhosphorGreen, ColorBlack, true)
	for row := 0; row < 2; row++ {
		py := float64(row * CellSize)
		for col := HUDP1End; col < HUDP2Start; col++ {
			px := float64(col * CellSize)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(px, py)
			screen.DrawImage(sepGlyph, op)
		}
	}

	// 3. Player panels (inverted text: black glyphs on green background)
	p1 := state.Tanks[0]
	p2 := state.Tanks[1]

	p1Lives, p1Shots, p1Mines := "??", "??", "??"
	p2Lives, p2Shots, p2Mines := "??", "??", "??"
	if p1.Active || p1.Lives > 0 {
		p1Lives = fmt.Sprintf("%d", p1.Lives)
		p1Shots = fmt.Sprintf("%d", p1.ShotsLeft)
		p1Mines = fmt.Sprintf("%d", p1.MinesLeft)
	}
	if p2.Active || p2.Lives > 0 {
		p2Lives = fmt.Sprintf("%d", p2.Lives)
		p2Shots = fmt.Sprintf("%d", p2.ShotsLeft)
		p2Mines = fmt.Sprintf("%d", p2.MinesLeft)
	}

	ai1 := 0
	ai2 := 0
	if state.Difficulty > 0 {
		// In VS AI, both players see the AI difficulty label
		ai2 = state.Difficulty
	}

	p1Row0 := hudLabelRow(ai1)
	p1Row1 := hudValueRow(mustInt(p1Lives), mustInt(p1Shots), mustInt(p1Mines))
	p2Row0 := hudLabelRow(ai2)
	p2Row1 := hudValueRow(mustInt(p2Lives), mustInt(p2Shots), mustInt(p2Mines))

	r.drawHUDPanel(screen, 0, p1Row0, p1Row1)
	r.drawHUDPanel(screen, HUDP2Start, p2Row0, p2Row1)

	// 4. Per-player status messages in the top border row
	maxShots := difficultyToMaxShots(state.Difficulty)
	p1Msg := hudStatusMessage(p1.ShotsLeft, maxShots, p1.Lives, state.Winner == 1)
	p2Msg := hudStatusMessage(p2.ShotsLeft, maxShots, p2.Lives, state.Winner == 2)
	borderY := float64(HUDHeight)
	if p1Msg != "" {
		r.drawBorderMessage(screen, p1Msg, 1, borderY)
	}
	if p2Msg != "" {
		r.drawBorderMessage(screen, p2Msg, HUDP2Start, borderY)
	}
}

func (r *Renderer) drawHUDPanel(screen *ebiten.Image, startCol int, row0, row1 string) {
	for i, ch := range row0 {
		if ch == ' ' {
			continue
		}
		px := float64((startCol + i) * CellSize)
		py := float64(0)
		glyph := r.cache.Get(ch, ColorPhosphorGreen, ColorBlack, true)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(px, py)
		screen.DrawImage(glyph, op)
	}
	for i, ch := range row1 {
		if ch == ' ' {
			continue
		}
		px := float64((startCol + i) * CellSize)
		py := float64(CellSize)
		glyph := r.cache.Get(ch, ColorPhosphorGreen, ColorBlack, true)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(px, py)
		screen.DrawImage(glyph, op)
	}
}

func (r *Renderer) drawBorderMessage(screen *ebiten.Image, msg string, startCol int, y float64) {
	panelWidth := HUDP1End - 1
	centered := msg
	if len(centered) < panelWidth {
		centered = fmt.Sprintf("%*s", panelWidth, fmt.Sprintf("%-*s", panelWidth, centered))
	} else {
		centered = centered[:panelWidth]
	}
	for i, ch := range centered {
		if ch == ' ' {
			continue
		}
		px := float64((startCol + i) * CellSize)
		glyph := r.cache.Get(ch, ColorPhosphorGreen, ColorBlack, true)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(px, y)
		screen.DrawImage(glyph, op)
	}
}

func mustInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

func (r *Renderer) drawInvertedText(screen *ebiten.Image, txt string, x, y float64, face *text.GoTextFace) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(ColorBlack)
	text.Draw(screen, txt, face, op)
}

func (r *Renderer) drawInvertedTextRight(screen *ebiten.Image, txt string, rightX, y float64, face *text.GoTextFace) {
	w, _ := text.Measure(txt, face, 0)
	r.drawInvertedText(screen, txt, rightX-w, y, face)
}

func (r *Renderer) drawDisconnected(screen *ebiten.Image, state *GameState) {
	var lines []string
	if state.ErrorMsgText != "" {
		lines = []string{"DISCONNECTED", state.ErrorMsgText, "ESC - RETURN TO LOBBY"}
	} else {
		lines = []string{"DISCONNECTED", "ESC - RETURN TO LOBBY"}
	}

	face := &text.GoTextFace{
		Source: r.hudFace.Source,
		Size:   24,
	}
	for i, line := range lines {
		w, _ := text.Measure(line, face, 0)
		x := float64(BoardCols*CellSize)/2 - w/2
		y := float64(BoardRows*CellSize)/2 - float64(len(lines))*12 + float64(i)*30 + HUDHeight
		op := &text.DrawOptions{}
		op.GeoM.Translate(x, y)

		colorVal := ColorPhosphorGreen
		if i == len(lines)-1 && state.ErrorMsgText != "" {
			colorVal = color.RGBA{255, 80, 80, 255}
		}

		op.ColorScale.ScaleWithColor(colorVal)
		text.Draw(screen, line, face, op)
	}
}

func (r *Renderer) drawGameOverOverlay(screen *ebiten.Image, state *GameState) {
	face := &text.GoTextFace{
		Source: r.hudFace.Source,
		Size:   32,
	}

	// Determine win/loss based on player ID
	winnerText := fmt.Sprintf("PLAYER %d WINS!", state.Winner)
	if state.PlayerID == state.Winner {
		winnerText = "YOU WIN!"
	} else if state.PlayerID > 0 && state.Winner > 0 {
		winnerText = "YOU LOSE!"
	}

	w, _ := text.Measure(winnerText, face, 0)
	x := float64(BoardCols*CellSize)/2 - w/2
	y := float64(BoardRows*CellSize)/2 + HUDHeight - 16

	boxH := float64(80)
	boxY := y - 16
	vector.FillRect(screen, float32(x-10), float32(boxY), float32(w+20), float32(boxH),
		ColorBlack, false)
	vector.StrokeRect(screen, float32(x-10), float32(boxY), float32(w+20), float32(boxH), 2,
		ColorPhosphorGreen, false)

	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(ColorPhosphorGreen)
	text.Draw(screen, winnerText, face, op)

	promptFace := &text.GoTextFace{
		Source: r.hudFace.Source,
		Size:   16,
	}

	if state.OpponentWantsRematch {
		promptText := "WAITING FOR OPPONENT..."
		pw, _ := text.Measure(promptText, promptFace, 0)
		px := float64(BoardCols*CellSize)/2 - pw/2
		py := y + 32
		pop := &text.DrawOptions{}
		pop.GeoM.Translate(px, py)
		pop.ColorScale.ScaleWithColor(ColorPhosphorGreen)
		text.Draw(screen, promptText, promptFace, pop)
	} else {
		prompt1 := "P - PLAY AGAIN    ESC - LOBBY"
		p1w, _ := text.Measure(prompt1, promptFace, 0)
		p1x := float64(BoardCols*CellSize)/2 - p1w/2
		p1y := y + 32
		pop1 := &text.DrawOptions{}
		pop1.GeoM.Translate(p1x, p1y)
		pop1.ColorScale.ScaleWithColor(ColorPhosphorGreen)
		text.Draw(screen, prompt1, promptFace, pop1)
	}
}

func (r *Renderer) drawEscConfirmOverlay(screen *ebiten.Image, state *GameState) {
	face := &text.GoTextFace{
		Source: r.hudFace.Source,
		Size:   20,
	}

	line1 := "LEAVE GAME?"
	line2 := "ESC TO CONFIRM / ANY KEY TO STAY"

	w1, _ := text.Measure(line1, face, 0)
	w2, _ := text.Measure(line2, face, 0)
	maxW := w1
	if w2 > maxW {
		maxW = w2
	}

	centerX := float64(BoardCols*CellSize) / 2
	centerY := float64(BoardRows*CellSize)/2 + HUDHeight

	boxW := maxW + 40
	boxH := float64(70)
	boxX := centerX - boxW/2
	boxY := centerY - boxH/2

	vector.FillRect(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH),
		ColorBlack, false)
	vector.StrokeRect(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH), 2,
		color.RGBA{255, 80, 80, 255}, false)

	op1 := &text.DrawOptions{}
	op1.GeoM.Translate(centerX-w1/2, centerY-20)
	op1.ColorScale.ScaleWithColor(color.RGBA{255, 80, 80, 255})
	text.Draw(screen, line1, face, op1)

	op2 := &text.DrawOptions{}
	op2.GeoM.Translate(centerX-w2/2, centerY+10)
	op2.ColorScale.ScaleWithColor(ColorPhosphorGreen)
	text.Draw(screen, line2, face, op2)
}

func (r *Renderer) drawSpectatingLabel(screen *ebiten.Image, state *GameState) {
	face := &text.GoTextFace{
		Source: r.hudFace.Source,
		Size:   24,
	}

	textStr := "SPECTATING"
	w, _ := text.Measure(textStr, face, 0)
	x := float64(BoardCols*CellSize)/2 - w/2
	y := float64(4)

	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(ColorPhosphorGreen)
	text.Draw(screen, textStr, face, op)
}

func (r *Renderer) drawWaitingReconnectOverlay(screen *ebiten.Image, state *GameState) {
	face := &text.GoTextFace{
		Source: r.hudFace.Source,
		Size:   28,
	}

	mainText := "OPPONENT DISCONNECTED"
	subText := fmt.Sprintf("RECONNECTING... %.0fs", state.DisconnectCountdown)

	w1, _ := text.Measure(mainText, face, 0)
	subFace := &text.GoTextFace{
		Source: r.hudFace.Source,
		Size:   16,
	}
	w2, _ := text.Measure(subText, subFace, 0)
	maxW := w1
	if w2 > maxW {
		maxW = w2
	}

	centerX := float64(BoardCols*CellSize) / 2
	centerY := float64(BoardRows*CellSize)/2 + HUDHeight

	boxW := maxW + 40
	boxH := float64(80)
	boxX := centerX - boxW/2
	boxY := centerY - boxH/2

	vector.FillRect(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH),
		ColorBlack, false)
	vector.StrokeRect(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH), 2,
		color.RGBA{255, 80, 80, 255}, false)

	op1 := &text.DrawOptions{}
	op1.GeoM.Translate(centerX-w1/2, centerY-20)
	op1.ColorScale.ScaleWithColor(color.RGBA{255, 80, 80, 255})
	text.Draw(screen, mainText, face, op1)

	op2 := &text.DrawOptions{}
	op2.GeoM.Translate(centerX-w2/2, centerY+10)
	op2.ColorScale.ScaleWithColor(ColorPhosphorGreen)
	text.Draw(screen, subText, subFace, op2)
}
