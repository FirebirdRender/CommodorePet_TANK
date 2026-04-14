//go:build js

package client

import (
	"fmt"
	"strings"
	"syscall/js"
)

func getJSPlayerName() string {
	search := js.Global().Get("window").Get("location").Get("search").String()
	if strings.Contains(search, "name=") {
		parts := strings.Split(search, "name=")
		if len(parts) > 1 {
			end := parts[1]
			if idx := strings.Index(end, "&"); idx >= 0 {
				end = end[:idx]
			}
			if end != "" {
				return end
			}
		}
	}
	return ""
}

func getJSServerURL() string {
	loc := js.Global().Get("window").Get("location")
	protocol := "ws"
	if loc.Get("protocol").String() == "https:" {
		protocol = "wss"
	}
	host := loc.Get("host").String()
	return fmt.Sprintf("%s://%s/ws", protocol, host)
}

func getJSTestMode() bool {
	search := js.Global().Get("window").Get("location").Get("search").String()
	return strings.Contains(search, "test=1") || strings.Contains(search, "test=true")
}

// ExportGameState registers JavaScript bridge functions for testing and debugging.
// Only active when test mode is enabled (?test=1 URL parameter).
func (g *Game) ExportGameState() {
	if !g.testMode {
		return
	}

	js.Global().Set("getGameState", js.FuncOf(func(this js.Value, args []js.Value) any {
		state := g.state
		phaseNames := map[GamePhase]string{
			PhaseConnecting:   "connecting",
			PhaseLobby:        "lobby",
			PhasePlaying:      "playing",
			PhaseRoundOver:    "roundOver",
			PhaseGameOver:     "gameOver",
			PhaseDisconnected: "disconnected",
		}
		lobbyNames := map[LobbyMode]string{
			LobbyModeStart:      "start",
			LobbyModeDifficulty: "difficulty",
			LobbyModeCreating:   "creating",
			LobbyModeJoining:    "joining",
			LobbyModeWaiting:    "waiting",
			LobbyModeRematch:    "rematch",
		}

		phaseName := "unknown"
		if n, ok := phaseNames[state.Phase]; ok {
			phaseName = n
		}
		lobbyName := "unknown"
		if n, ok := lobbyNames[state.LobbyMode]; ok {
			lobbyName = n
		}

		obj := make(map[string]any)
		obj["phase"] = phaseName
		obj["lobbyMode"] = lobbyName
		obj["playerID"] = state.PlayerID
		obj["playerName"] = g.playerName
		obj["opponentName"] = state.OpponentName
		obj["roomCode"] = state.RoomCode
		obj["connected"] = state.Connected
		obj["difficulty"] = state.DifficultySelection
		obj["error"] = state.ErrorMsgText
		obj["connectErr"] = state.ConnectErr
		obj["winner"] = state.Winner
		obj["testMode"] = g.testMode

		return js.ValueOf(obj)
	}))

	js.Global().Set("sendInput", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 2 {
			return nil
		}
		key := args[0].String()
		action := "down"
		if len(args) > 1 {
			action = args[1].String()
		}
		if g.network != nil && g.network.Connected {
			g.network.Send(MsgTypeInput, InputMsg{
				Tick:   0,
				Key:    key,
				Action: action,
			})
		}
		return nil
	}))

	js.Global().Set("sendReady", js.FuncOf(func(this js.Value, args []js.Value) any {
		if g.network != nil && g.network.Connected {
			g.network.Send(MsgTypeReady, ReadyMsg{})
		}
		return nil
	}))

	js.Global().Set("sendPlayAgain", js.FuncOf(func(this js.Value, args []js.Value) any {
		if g.network != nil && g.network.Connected {
			g.network.Send(MsgTypePlayAgain, PlayAgainMsg{})
		}
		return nil
	}))
}
