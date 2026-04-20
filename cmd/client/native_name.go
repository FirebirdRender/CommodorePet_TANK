//go:build !js

package main

func getConfig() (serverURL, playerName string, playerID int, roomCode, token string, isSpectator bool) {
	return "ws://localhost:8080/ws", "Player", 0, "", "", false
}
