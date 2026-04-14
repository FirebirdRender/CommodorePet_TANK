package main

import (
	"flag"
	"log"

	"github.com/FirebirdRender/CommodorePet_TANK/client"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	addr := flag.String("addr", "", "server WebSocket address (WASM: auto-detected from page URL)")
	name := flag.String("name", "", "player name (default: Player or URL ?name=)")
	flag.Parse()

	playerName := *name
	if playerName == "" {
		playerName = getJSPlayerName()
	}
	if playerName == "" {
		playerName = "Player"
	}

	serverURL := *addr
	if serverURL == "" {
		serverURL = getJSServerURL()
	}
	if serverURL == "" {
		serverURL = "ws://localhost:8080/ws"
	}

	testMode := getJSTestMode()

	log.Printf("TANK! connecting to %s as %s", serverURL, playerName)

	game := client.NewGame(serverURL, playerName, testMode)

	ebiten.SetWindowSize(800, 460)
	ebiten.SetWindowTitle("TANK!")

	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
