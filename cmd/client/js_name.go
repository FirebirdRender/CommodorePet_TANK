//go:build js

package main

import (
	"fmt"
	"syscall/js"
)

func getConfig() (serverURL, playerName string, playerID int, roomCode, token string) {
	config := js.Global().Get("window").Get("tankConfig")
	if !config.IsUndefined() && !config.IsNull() {
		serverURL = config.Get("serverUrl").String()
		playerName = config.Get("name").String()
		playerID = config.Get("pid").Int()
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
