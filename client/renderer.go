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
	cache   *GlyphCache
	hudFace *text.GoTextFace
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
	return &Renderer{
		cache:   gc,
		hudFace: hudFace,
	}, nil
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
	} else if state.Phase == PhaseLobby || state.Phase == PhaseConnecting {
		r.drawLobbyText(screen, state)
	} else if state.Phase == PhaseGameOver {
		r.drawGrid(screen, state)
		r.drawHUD(screen, state)
		r.drawGameOverOverlay(screen, state)
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

	for y, row := range state.Grid {
		for x, cellType := range row {
			def, ok := cellGlyphs[cellType]
			if !ok {
				def = cellGlyphs[CellEmpty]
			}
			glyph := r.cache.Get(def.Rune, def.FgColor, ColorBlack, def.Inverted)
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
	vector.FillRect(screen, 0, 0, float32(BoardCols*CellSize), float32(HUDHeight),
		ColorPhosphorGreen, false)

	p1 := state.Tanks[0]
	p2 := state.Tanks[1]
	p1Lives := "??"
	p1Shots := "??"
	p1Mines := "??"
	p2Lives := "??"
	p2Shots := "??"
	p2Mines := "??"
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

	p1Name := state.PlayerName
	if p1Name == "" {
		p1Name = "P1"
	}
	p2Name := state.OpponentName
	if p2Name == "" {
		p2Name = "P2"
	}

	p1Text := fmt.Sprintf("%s: L:%s S:%s M:%s", p1Name, p1Lives, p1Shots, p1Mines)
	r.drawInvertedText(screen, p1Text, 4, 4, r.hudFace)

	p2Text := fmt.Sprintf("%s: L:%s S:%s M:%s", p2Name, p2Lives, p2Shots, p2Mines)
	r.drawInvertedTextRight(screen, p2Text, BoardCols*CellSize-4, 4, r.hudFace)

	centerX := BoardCols * CellSize / 2
	glyph := r.cache.Get('●', ColorPhosphorGreen, ColorBlack, true)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(centerX-CellSize/2), float64(4))
	screen.DrawImage(glyph, op)

	statusY := float64(22)
	if state.Winner > 0 {
		winnerText := fmt.Sprintf("P%d WINS!", state.Winner)
		r.drawInvertedText(screen, winnerText, 4, statusY, r.hudFace)
	} else if p1.ShotsLeft == 0 && p1.Active {
		r.drawInvertedText(screen, "OUT OF SHOTS", 4, statusY, r.hudFace)
	} else if p2.ShotsLeft == 0 && p2.Active {
		r.drawInvertedText(screen, "OUT OF SHOTS", 4, statusY, r.hudFace)
	}
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

func (r *Renderer) drawLobbyText(screen *ebiten.Image, state *GameState) {
	var lines []string

	// Calculate pulsing dots animation
	dots := ""
	switch (state.AnimTick / 30) % 3 {
	case 0:
		dots = "."
	case 1:
		dots = ".."
	case 2:
		dots = "..."
	}

	switch {
	case state.Phase == PhaseConnecting:
		if state.ConnectErr != "" {
			lines = []string{state.ConnectErr}
		} else {
			lines = []string{"CONNECTING" + dots}
		}

	case state.Phase == PhaseDisconnected:
		if state.ErrorMsgText != "" {
			lines = []string{"DISCONNECTED", state.ErrorMsgText, "PRESS ESC TO EXIT"}
		} else {
			lines = []string{"DISCONNECTED", "PRESS ESC TO EXIT"}
		}

	case state.LobbyMode == LobbyModeDifficulty:
		lines = []string{
			fmt.Sprintf("SELECT DIFFICULTY: %d", state.DifficultySelection),
			"",
			"UP/DOWN OR 1-9 TO CHANGE",
			"0 FOR LEVEL 10",
			"ENTER TO CONFIRM",
			"ESC TO CANCEL",
		}

	case state.LobbyMode == LobbyModeWaiting && state.RoomCode != "" && state.OpponentName == "":
		lines = []string{
			"",
			"SHARE THIS CODE:",
			"  " + state.RoomCode + "  ",
			"",
			"WAITING FOR OPPONENT" + dots,
		}

	case state.LobbyMode == LobbyModeWaiting && state.RoomCode != "":
		lines = []string{
			"ROOM: " + state.RoomCode,
			"VS " + state.OpponentName,
		}

	case state.LobbyMode == LobbyModeJoining:
		prompt := "JOIN ROOM: " + state.RoomCodeInput
		if len(state.RoomCodeInput) < 4 {
			prompt += "_"
		}
		lines = []string{prompt, "TYPE 4-LETTER CODE, PRESS ENTER"}

	case state.LobbyMode == LobbyModeCreating:
		lines = []string{"CREATING ROOM" + dots}

	default:
		lines = []string{
			"TANK!",
			"PRESS C TO CREATE ROOM",
			"PRESS J TO JOIN ROOM",
			fmt.Sprintf("YOU: %s", state.PlayerName),
		}
	}

	if state.ErrorMsgText != "" {
		lines = append(lines, "ERROR: "+state.ErrorMsgText)
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

	// Draw difficulty selection bar (only in LobbyModeDifficulty)
	if state.LobbyMode == LobbyModeDifficulty {
		// Center the bar horizontally
		barWidth := 10 * CellSize * 2
		startX := float64(BoardCols*CellSize)/2 - float64(barWidth/2)
		barY := float64(BoardRows*CellSize)/2 - float64(len(lines))*12 + float64(len(lines))*30 + HUDHeight + 20
		for i := 1; i <= 10; i++ {
			var glyph rune
			if i <= state.DifficultySelection {
				glyph = '█' // filled block
			} else {
				glyph = '░' // empty block
			}
			ch := r.cache.Get(glyph, ColorPhosphorGreen, ColorBlack, false)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(startX+float64((i-1)*CellSize*2), barY)
			screen.DrawImage(ch, op)
		}
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

	if state.LobbyMode == LobbyModeRematch {
		promptText := "WAITING FOR OPPONENT..."
		pw, _ := text.Measure(promptText, promptFace, 0)
		px := float64(BoardCols*CellSize)/2 - pw/2
		py := y + 32
		pop := &text.DrawOptions{}
		pop.GeoM.Translate(px, py)
		pop.ColorScale.ScaleWithColor(ColorPhosphorGreen)
		text.Draw(screen, promptText, promptFace, pop)
	} else {
		prompt1 := "P - PLAY AGAIN    ESC - EXIT"
		p1w, _ := text.Measure(prompt1, promptFace, 0)
		p1x := float64(BoardCols*CellSize)/2 - p1w/2
		p1y := y + 32
		pop1 := &text.DrawOptions{}
		pop1.GeoM.Translate(p1x, p1y)
		pop1.ColorScale.ScaleWithColor(ColorPhosphorGreen)
		text.Draw(screen, prompt1, promptFace, pop1)
	}
}
