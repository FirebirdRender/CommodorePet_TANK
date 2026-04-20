package client

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Game struct {
	state        *GameState
	renderer     *Renderer
	network      *Network
	input        *InputHandler
	audio        *AudioPlayer
	sceneManager *SceneManager

	serverURL   string
	playerName  string
	playerID    int
	roomCode    string
	token       string
	isSpectator bool
}

func NewGame(serverURL, playerName string, playerID int, roomCode, token string, isSpectator bool) *Game {
	return &Game{
		state: &GameState{
			Phase:           PhaseDisconnected,
			ExplosionTimers: make(map[[2]int]float64),
			DirtyCells:      nil,
		},
		serverURL:   serverURL,
		playerName:  playerName,
		playerID:    playerID,
		roomCode:    roomCode,
		token:       token,
		isSpectator: isSpectator,
	}
}

func (g *Game) ConnectAsync() {
	go func() {
		for attempt := 0; attempt <= 3; attempt++ {
			if attempt > 0 {
				backoff := time.Duration(1<<(attempt-1)) * time.Second
				time.Sleep(backoff)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			conn, _, err := websocket.Dial(ctx, g.serverURL, nil)
			cancel()

			if err == nil {
				g.network.mu.Lock()
				g.network.conn = conn
				conn.SetReadLimit(1 << 20)
				g.network.Connected = true
				g.network.RetryCount = 0
				g.network.mu.Unlock()

				go g.network.readLoop()
				go g.network.writeLoop()

				if g.renderer == nil {
					renderer, err := NewRenderer()
					if err != nil {
						log.Printf("failed to create renderer: %v", err)
					} else {
						g.renderer = renderer
					}
				}
				if g.input == nil {
					g.input = NewInputHandler()
				}
				if g.audio == nil {
					g.audio = NewAudioPlayer()
				}
				if err := initCRTShader(); err != nil {
					log.Printf("CRT shader init failed (non-fatal): %v", err)
				}
				initOffscreen()

				if g.isSpectator {
					g.network.Send(MsgTypeSpectate, SpectateMsg{
						RoomCode: g.roomCode,
					})
				} else {
					g.network.Send(MsgTypeRejoin, RejoinMsg{
						RoomCode:   g.roomCode,
						PlayerID:   g.playerID,
						Token:      g.token,
						PlayerName: g.playerName,
					})
				}

				g.state.Connected = true
				g.state.ConnectErr = ""

				return
			}

			if attempt < 3 {
				g.state.ConnectErr = "CONNECTING..."
			} else {
				g.state.ConnectErr = "CONNECTION FAILED. PRESS ESC TO EXIT."
			}
		}
	}()
}

func (g *Game) Update() error {
	g.state.AnimTick++

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.state.DebugLastKey = "ESC"
		g.state.DebugKeyCount++
		switch g.state.Phase {
		case PhaseDisconnected, PhaseGameOver:
			redirectLobby()
			// Return error to signal Ebiten to stop the game loop
			// This prevents the WASM program from continuing after redirect
			return fmt.Errorf("redirecting to lobby")
		case PhaseSpectating:
			if g.state.EscConfirmPending {
				redirectLobby()
				return fmt.Errorf("redirecting to lobby")
			}
			g.state.EscConfirmPending = true
		case PhasePlaying, PhaseRoundOver:
			if g.state.EscConfirmPending {
				g.state.Phase = PhaseDisconnected
				g.state.ErrorMsgText = "DISCONNECTED"
				g.state.EscConfirmPending = false
				if g.input != nil {
					g.input.SetEnabled(false)
				}
				if g.network != nil {
					g.network.Close()
				}
			} else {
				g.state.EscConfirmPending = true
			}
		}
	}

	if g.state.EscConfirmPending && g.input != nil {
		for key := range g.input.keyMap {
			if ebiten.IsKeyPressed(key) && key != ebiten.KeyEscape {
				g.state.EscConfirmPending = false
				break
			}
		}
	}

	if g.network == nil {
		g.network = &Network{
			Incoming:   make(chan []byte, 256),
			Outgoing:   make(chan []byte, 256),
			MaxRetries: 3,
			done:       make(chan struct{}),
		}
		g.ConnectAsync()
	}

	if g.network != nil {
		msgs := g.network.DrainIncoming()
		for _, data := range msgs {
			msgType, payload, err := UnwrapMessage(data)
			if err != nil {
				log.Printf("unwrap message error: %v", err)
				continue
			}
			g.handleMessage(msgType, payload)
		}

		if !g.network.Connected {
			g.state.Phase = PhaseDisconnected
		}
	}

	if g.state.Phase == PhaseWaitingReconnect {
		g.state.DisconnectCountdown -= 1.0 / 60.0
		if g.state.DisconnectCountdown <= 0 {
			redirectLobby()
			return fmt.Errorf("redirecting to lobby")
		}
	}

	if g.state.Phase == PhaseGameOver {
		if inpututil.IsKeyJustPressed(ebiten.KeyP) {
			g.network.Send(MsgTypePlayAgain, PlayAgainMsg{})
			g.state.OpponentWantsRematch = false
		}
	}

	if g.state.Phase == PhasePlaying && !g.isSpectator && g.input != nil {
		g.input.Update()
		events := g.input.PollEdgeEvents()
		for _, ev := range events {
			g.state.DebugLastKey = ev.Key
			g.state.DebugKeyCount++
			g.network.Send(MsgTypeInput, ev)
			if ev.Action == "down" {
				g.state.PredictBarrelDir(ev.Key)
			}
			if ev.Action == "up" && keyToDir(ev.Key) >= 0 {
				delete(g.state.PredictedDir, g.state.PlayerID)
			}
		}
	}

	return nil
}

func (g *Game) handleMessage(msgType string, payload []byte) {
	switch msgType {
	case MsgTypeJoined:
		var msg JoinedMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("joined unmarshal error: %v", err)
			return
		}
		g.state.ApplyJoined(msg)

	case MsgTypeGameStart:
		var msg GameStartMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("game_start unmarshal error: %v", err)
			return
		}
		g.state.ApplyGameStart(msg)
		if g.input != nil {
			g.input.SetEnabled(true)
		}

	case MsgTypeTick:
		var msg TickMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("tick unmarshal error: %v", err)
			return
		}
		g.state.ApplyTick(msg)

	case MsgTypeTickDelta:
		var msg TickDeltaMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("tick_delta unmarshal error: %v", err)
			return
		}
		g.state.ApplyTickDelta(msg)

	case MsgTypeRoundOver:
		var msg RoundOverMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("round_over unmarshal error: %v", err)
			return
		}
		g.state.ApplyRoundOver(msg)
		if g.input != nil {
			g.input.SetEnabled(false)
		}
		if g.audio != nil {
			g.audio.Play(SFXRoundOver)
		}

	case MsgTypeGameOver:
		var msg GameOverMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("game_over unmarshal error: %v", err)
			return
		}
		g.state.ApplyGameOver(msg)
		if g.input != nil {
			g.input.SetEnabled(false)
		}
		if g.audio != nil {
			g.audio.Play(SFXGameOver)
		}
		if g.isSpectator {
			g.state.Phase = PhaseWaitingReconnect
			g.state.DisconnectCountdown = 5.0
			g.state.ErrorMsgText = fmt.Sprintf("PLAYER #%d WINS - RETURNING TO LOBBY", msg.Winner)
		}

	case MsgTypeError:
		var msg ErrorMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("error unmarshal error: %v", err)
			return
		}
		g.state.ApplyError(msg)
		g.state.ErrorMsgText = msg.Message
		if msg.Code == "invalid_input" || msg.Code == "room_not_found" || msg.Code == "server_error" {
			g.state.Phase = PhaseDisconnected
		}

	case MsgTypeOpponentLeft:
		g.state.Phase = PhaseDisconnected
		g.state.ErrorMsgText = "OPPONENT LEFT THE GAME"
		if g.input != nil {
			g.input.SetEnabled(false)
		}

	case MsgTypePlayAgainAck:
		var msg PlayAgainAckMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("play_again_ack unmarshal error: %v", err)
			return
		}
		g.state.OpponentWantsRematch = true

	case MsgTypeRematch:
		var msg RematchMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("rematch unmarshal error: %v", err)
			return
		}
		g.state.ApplyRematch(msg)

	case MsgTypeSpectatorJoined:
		var msg SpectatorJoinedMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("spectator_joined unmarshal error: %v", err)
			return
		}
		g.state.Phase = PhaseSpectating
		g.state.SpectatorPlayerID = msg.PlayerID
		g.state.SpectatorCount = msg.SpectatorCount
		log.Printf("spectator joined: playerID=%d name=%s count=%d", msg.PlayerID, msg.Name, msg.SpectatorCount)

	case MsgTypeSpectatorLeft:
		var msg SpectatorLeftMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("spectator_left unmarshal error: %v", err)
			return
		}
		g.state.SpectatorCount = msg.SpectatorCount

	case MsgTypePlayerDisconnected:
		var msg PlayerDisconnectedMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("player_disconnected unmarshal error: %v", err)
			return
		}
		g.state.Phase = PhaseWaitingReconnect
		g.state.ReconnectDeadline = msg.ReconnectDeadline
		g.state.DisconnectCountdown = msg.ReconnectDeadline
		if g.input != nil {
			g.input.SetEnabled(false)
		}

	case MsgTypeReconnectQuery:
		var msg ReconnectQueryMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("reconnect_query unmarshal error: %v", err)
			return
		}
		g.state.ReconnectQuerySecondsRemaining = msg.SecondsRemaining
		go func() {
			time.Sleep(1 * time.Second)
			if g.network != nil && g.network.Connected {
				g.network.Send(MsgTypeStayConnected, StayConnectedMsg{Confirm: true})
			}
		}()

	case MsgTypeResumeMatch:
		g.state.Phase = PhasePlaying
		if g.input != nil {
			g.input.SetEnabled(true)
		}

	default:
		log.Printf("unknown message type: %s", msgType)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.renderer != nil {
		if isCRTEnabled() {
			g.renderer.Draw(offscreen, g.state)
			op := &ebiten.DrawRectShaderOptions{}
			op.Images[0] = offscreen
			screen.DrawRectShader(800, 480, crtShader, op)
		} else {
			g.renderer.Draw(screen, g.state)
		}

		var barPhase string
		switch g.state.Phase {
		case PhasePlaying:
			barPhase = "PLAY"
		case PhaseRoundOver:
			barPhase = "ROUND_OVER"
		case PhaseGameOver:
			barPhase = "GAME_OVER"
		case PhaseDisconnected:
			barPhase = "DISCONNECTED"
		case PhaseSpectating:
			barPhase = "SPECTATING"
		case PhaseWaitingReconnect:
			barPhase = "WAIT_RECONNECT"
		default:
			barPhase = "UNKNOWN"
		}
		connStr := "NO"
		if g.state.Connected {
			connStr = "YES"
		}
		lastKey := g.state.DebugLastKey
		if lastKey == "" {
			lastKey = "-"
		}
		barText := fmt.Sprintf("PHASE:%s CONN:%s KEYS:%d LAST:%s TICK:%d",
			barPhase, connStr, g.state.DebugKeyCount, lastKey, g.state.AnimTick)

		debugFace := &text.GoTextFace{
			Source: g.renderer.hudFace.Source,
			Size:   12,
		}
		debugOp := &text.DrawOptions{}
		debugOp.GeoM.Translate(4, 464)
		debugOp.ColorScale.ScaleWithColor(color.RGBA{120, 120, 120, 255})
		text.Draw(screen, barText, debugFace, debugOp)
	} else {
		screen.Fill(ColorBlack)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 800, 480
}
