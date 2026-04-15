//go:build js

package client

import (
	"fmt"
	"syscall/js"
)

func getJSConfig() (serverURL, playerName string, playerID int, roomCode, token string) {
	config := js.Global().Get("window").Get("tankConfig")
	if !config.IsUndefined() && !config.IsNull() {
		serverURL = config.Get("serverUrl").String()
		playerName = config.Get("name").String()
		playerID = int(config.Get("pid").Int())
		roomCode = config.Get("room").String()
		token = config.Get("token").String()
	}
	if serverURL == "" {
		loc := js.Global().Get("window").Get("location")
		protocol := "ws"
		if loc.Get("protocol").String() == "https:" {
			protocol = "wss"
		}
		host := loc.Get("host").String()
		serverURL = fmt.Sprintf("%s://%s/ws", protocol, host)
	}
	if playerName == "" {
		playerName = "Player"
	}
	return serverURL, playerName, playerID, roomCode, token
}

func redirectLobby() {
	js.Global().Get("window").Get("location").Set("href", "/")
}

func (g *Game) ExportGameState() {
	js.Global().Set("getGameState", js.FuncOf(func(this js.Value, args []js.Value) any {
		state := g.state
		phaseNames := map[GamePhase]string{
			PhasePlaying:      "playing",
			PhaseRoundOver:    "roundOver",
			PhaseGameOver:     "gameOver",
			PhaseDisconnected: "disconnected",
		}

		phaseName := "unknown"
		if n, ok := phaseNames[state.Phase]; ok {
			phaseName = n
		}

		obj := make(map[string]any)
		obj["phase"] = phaseName
		obj["playerID"] = state.PlayerID
		obj["playerName"] = g.playerName
		obj["opponentName"] = state.OpponentName
		obj["roomCode"] = state.RoomCode
		obj["connected"] = state.Connected
		obj["winner"] = state.Winner

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

	js.Global().Set("sendPlayAgain", js.FuncOf(func(this js.Value, args []js.Value) any {
		if g.network != nil && g.network.Connected {
			g.network.Send(MsgTypePlayAgain, PlayAgainMsg{})
		}
		return nil
	}))
}
