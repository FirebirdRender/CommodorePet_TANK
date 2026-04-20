//go:build js

package main

import (
	"fmt"
	"strings"
	"syscall/js"
)

func getConfig() (serverURL, playerName string, playerID int, roomCode, token string, isSpectator bool) {
	config := js.Global().Get("window").Get("tankConfig")
	if !config.IsUndefined() && !config.IsNull() {
		serverURL = config.Get("serverUrl").String()
		playerName = config.Get("name").String()
		playerID = config.Get("pid").Int()
		roomCode = config.Get("room").String()
		token = config.Get("token").String()
		if config.Get("isSpectator").Bool() {
			isSpectator = true
			roomCode = config.Get("spectateRoom").String()
		}
	}
	if serverURL == "" {
		loc := js.Global().Get("window").Get("location")
		protocol := "ws"
		if loc.Get("protocol").String() == "https:" {
			protocol = "wss"
		}
		host := loc.Get("host").String()
		serverURL = fmt.Sprintf("%s://%s/ws", protocol, host)

		search := loc.Get("search").String()
		if strings.Contains(search, "spectate=") {
			idx := strings.Index(search, "spectate=") + len("spectate=")
			end := strings.Index(search[idx:], "&")
			if end > 0 {
				roomCode = search[idx : idx+end]
			} else {
				roomCode = search[idx:]
			}
			roomCode = strings.TrimPrefix(roomCode, "=")
			isSpectator = true
		}
	}
	if playerName == "" {
		playerName = "Player"
	}
	return serverURL, playerName, playerID, roomCode, token, isSpectator
}
