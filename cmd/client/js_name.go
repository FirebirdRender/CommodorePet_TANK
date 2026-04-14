//go:build js

package main

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

// getJSServerURL derives the WebSocket URL from the page's location.
// This ensures the WASM client connects to the same host that served the page,
// rather than hardcoding localhost (which fails for remote/production deployments).
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
