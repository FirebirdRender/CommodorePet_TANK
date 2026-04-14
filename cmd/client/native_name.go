//go:build !js

package main

func getJSPlayerName() string {
	return ""
}

func getJSServerURL() string {
	return ""
}

func getJSTestMode() bool {
	return false
}
