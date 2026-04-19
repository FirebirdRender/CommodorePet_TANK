//go:build js

// WASM compile-time verification tests.
//
// These tests verify type signatures and build correctness under GOOS=js.
// They do NOT run as runtime tests in CI — there is no browser JS environment.
// The build tag ensures this file is excluded from native `go test` runs.
//
// To verify: make test-wasm
package client

import (
	"syscall/js"
	"testing"
)

func TestGetJSConfigReturnType(t *testing.T) {
	var _ func() (string, string, int, string, string) = getJSConfig
}

func TestRedirectLobbyCompiles(t *testing.T) {
	var _ func() = redirectLobby
}

func TestExportGameStateCompiles(t *testing.T) {
	var _ func(*Game) = (*Game).ExportGameState
}

func TestJSConfigFields(t *testing.T) {
	config := js.Global().Get("window").Get("tankConfig")
	if config.IsUndefined() || config.IsNull() {
		t.Skip("window.tankConfig not set — skipping runtime field check")
	}
	expectedFields := []string{"serverUrl", "name", "pid", "room", "token"}
	for _, field := range expectedFields {
		if config.Get(field).IsUndefined() {
			t.Errorf("tankConfig.%s is undefined", field)
		}
	}
}
