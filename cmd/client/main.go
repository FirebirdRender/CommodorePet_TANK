package main

import (
	"flag"
	"log"

	"github.com/FirebirdRender/CommodorePet_TANK/client"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	addr := flag.String("addr", "", "server WebSocket address (WASM: auto-detected from tankConfig)")
	name := flag.String("name", "", "player name (WASM: from tankConfig)")
	flag.Parse()

	serverURL, playerName, playerID, roomCode, token := getConfig()

	if *addr != "" {
		serverURL = *addr
	}
	if *name != "" {
		playerName = *name
	}

	log.Printf("TANK! connecting to %s as %s (room=%s, pid=%d)", serverURL, playerName, roomCode, playerID)

	game := client.NewGame(serverURL, playerName, playerID, roomCode, token)

	ebiten.SetWindowSize(800, 480)
	ebiten.SetWindowTitle("TANK!")
	ebiten.SetRunnableOnUnfocused(true)

	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
