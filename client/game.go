package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// GamePhase constants are defined in gamestate.go

type Game struct {
	state    *GameState
	renderer *Renderer
	network  *Network
	input    *InputHandler
	audio    *AudioPlayer

	serverURL  string
	playerName string
	testMode   bool // enabled via ?test=1 URL parameter for E2E testing
}

func NewGame(serverURL, playerName string, testMode bool) *Game {
	return &Game{
		state: &GameState{
			Phase:               PhaseConnecting,
			DifficultySelection: 5,
			PlayerName:          playerName,
			ExplosionTimers:     make(map[[2]int]float64),
			DirtyCells:          nil,
		},
		serverURL:  serverURL,
		playerName: playerName,
		testMode:   testMode,
	}
}

func (g *Game) ConnectAsync() {
	go func() {
		for attempt := 0; attempt <= 3; attempt++ {
			if attempt > 0 {
				backoff := time.Duration(1<<(attempt-1)) * time.Second
				time.Sleep(backoff)
			}

			g.network.RetryCount = attempt

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

				// Initialize renderer, input, and audio after successful connection
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

				g.state.Connected = true
				g.state.Phase = PhaseLobby
				g.state.ConnectErr = ""

				if g.testMode {
					g.ExportGameState()
				}

				return
			}

			if attempt < 3 {
				g.state.ConnectErr = fmt.Sprintf("Connecting... (attempt %d/4)", attempt+1)
			} else {
				g.state.ConnectErr = "CONNECTION FAILED. PRESS ESC TO EXIT."
			}
		}
	}()
}

func (g *Game) Update() error {
	g.state.AnimTick++

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	if g.network == nil && g.state.Phase == PhaseConnecting {
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

	if g.state.Phase == PhaseLobby {
		g.handleLobbyInput()
	}

	if g.state.Phase == PhaseGameOver {
		if inpututil.IsKeyJustPressed(ebiten.KeyP) {
			g.network.Send(MsgTypePlayAgain, PlayAgainMsg{})
			g.state.LobbyMode = LobbyModeRematch
			g.state.OpponentWantsRematch = false
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.state.Phase = PhaseLobby
			g.state.LobbyMode = LobbyModeStart
			g.state.RoomCode = ""
			g.state.OpponentName = ""
			g.state.ErrorMsgText = ""
			g.state.Winner = 0
			g.network.Close()
		}
	}

	if g.state.LobbyMode == LobbyModeRematch {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.state.Phase = PhaseLobby
			g.state.LobbyMode = LobbyModeStart
			g.state.RoomCode = ""
			g.state.OpponentName = ""
			g.state.ErrorMsgText = ""
			g.state.Winner = 0
		}
	}

	if g.state.Phase == PhasePlaying && g.input != nil {
		g.input.Update()
		events := g.input.PollEdgeEvents()
		for _, ev := range events {
			g.network.Send(MsgTypeInput, ev)
			if ev.Action == "down" {
				g.state.PredictBarrelDir(ev.Key)
			}
			if ev.Action == "up" {
				delete(g.state.PredictedDir, g.state.PlayerID)
			}
		}
		dirs := g.input.PollHeldDirections()
		for _, dir := range dirs {
			g.network.Send(MsgTypeInput, InputMsg{
				Tick:   g.input.tick,
				Key:    dir,
				Action: "down",
			})
		}
	}

	if g.testMode && g.state.AnimTick%60 == 0 {
		g.ExportGameState()
	}

	return nil
}

func (g *Game) handleMessage(msgType string, payload []byte) {
	switch msgType {
	case MsgTypeRoomCreated:
		var msg RoomCreatedMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("room_created unmarshal error: %v", err)
			return
		}
		g.state.ApplyRoomCreated(msg)
		g.state.LobbyMode = LobbyModeWaiting

	case MsgTypeJoined:
		var msg JoinedMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("joined unmarshal error: %v", err)
			return
		}
		g.state.ApplyJoined(msg)
		g.state.LobbyMode = LobbyModeWaiting

	case MsgTypeGameStart:
		var msg GameStartMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("game_start unmarshal error: %v", err)
			return
		}
		g.state.ApplyGameStart(msg)
		g.input.SetEnabled(true)
		g.network.Send(MsgTypeReady, ReadyMsg{})

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

	case MsgTypeError:
		var msg ErrorMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("error unmarshal error: %v", err)
			return
		}
		g.state.ApplyError(msg)
		g.state.ErrorMsgText = msg.Message

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

	default:
		log.Printf("unknown message type: %s", msgType)
	}
}

func (g *Game) handleLobbyInput() {
	switch g.state.LobbyMode {
	case LobbyModeStart:
		if inpututil.IsKeyJustPressed(ebiten.KeyC) {
			g.state.LobbyMode = LobbyModeDifficulty
			g.state.DifficultySelection = 5
		} else if inpututil.IsKeyJustPressed(ebiten.KeyJ) {
			g.state.LobbyMode = LobbyModeJoining
			g.state.RoomCodeInput = ""
		}

	case LobbyModeDifficulty:
		for i := 1; i <= 9; i++ {
			key := ebiten.Key(int(ebiten.Key1) - 1 + i)
			if inpututil.IsKeyJustPressed(key) {
				g.state.DifficultySelection = i
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.Key0) {
			g.state.DifficultySelection = 10
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			if g.state.DifficultySelection < 10 {
				g.state.DifficultySelection++
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
			if g.state.DifficultySelection > 1 {
				g.state.DifficultySelection--
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.state.LobbyMode = LobbyModeCreating
			g.network.Send(MsgTypeCreateRoom, CreateRoomMsg{
				Difficulty: g.state.DifficultySelection,
				PlayerName: g.playerName,
			})
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.state.LobbyMode = LobbyModeStart
		}

	case LobbyModeJoining:
		for key := ebiten.KeyA; key <= ebiten.KeyZ; key++ {
			if inpututil.IsKeyJustPressed(key) {
				if len(g.state.RoomCodeInput) < 4 {
					g.state.RoomCodeInput += string(rune('A' + (key - ebiten.KeyA)))
				}
			}
		}
		for key := ebiten.Key0; key <= ebiten.Key9; key++ {
			if inpututil.IsKeyJustPressed(key) {
				if len(g.state.RoomCodeInput) < 4 {
					g.state.RoomCodeInput += string(rune('0' + (key - ebiten.Key0)))
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			if len(g.state.RoomCodeInput) > 0 {
				g.state.RoomCodeInput = g.state.RoomCodeInput[:len(g.state.RoomCodeInput)-1]
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && len(g.state.RoomCodeInput) == 4 {
			g.network.Send(MsgTypeJoinRoom, JoinRoomMsg{
				RoomCode:   g.state.RoomCodeInput,
				PlayerName: g.playerName,
			})
			g.state.LobbyMode = LobbyModeWaiting
		}
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.renderer != nil {
		if isCRTEnabled() {
			g.renderer.Draw(offscreen, g.state)
			op := &ebiten.DrawRectShaderOptions{}
			op.Images[0] = offscreen
			screen.DrawRectShader(800, 460, crtShader, op)
		} else {
			g.renderer.Draw(screen, g.state)
		}
	} else {
		screen.Fill(ColorBlack)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 800, 460
}
