//go:build !js

package main

func getConfig() (serverURL, playerName string, playerID int, roomCode, token string) {
	return "ws://localhost:8080/ws", "Player", 0, "", ""
}
