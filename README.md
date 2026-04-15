# TANK! — Networked Multiplayer Tank Battle

Faithful recreation of the Commodore PET TANK! game with networked multiplayer via WebSocket and WebAssembly.

## Quick Start

Prerequisites: Go 1.21+

```bash
git clone https://github.com/FirebirdRender/CommodorePet_TANK.git
cd CommodorePet_TANK
make build-all

# Start server
./bin/tank-server -addr :8080 -dir web

# Open browser
open http://localhost:8080/
```

## Development

```bash
make dev           # Build WASM and start server
make wasm-size     # Check WASM binary size
make clean         # Remove build artifacts
```

## Project Structure

- `engine/` — Deterministic game engine (board, tanks, physics, collisions)
- `server/` — WebSocket server (rooms, matchmaking, delta encoding)
- `client/` — WASM client (renderer, input, audio, CRT shader)
- `cmd/server/` — Server entry point
- `cmd/client/` — Client/WASM entry point
- `web/` — Static files (HTML, WASM binary, JS runtime)

## Server Flags

- `-addr` — listen address (default :8080)
- `-dir` — static files directory (default "web")
- `-cors` — CORS allowed origin (default "*")
- `-max-room-age` — stale room cleanup age (default 30m)

## Client URL Parameters

- `?name=YourName` — set player name (default "Player")

## Controls

Two control schemes active simultaneously — use whichever feels natural:

**PET matrix** (original Commodore PET layout):
- QWE/ASD/ZXC for 8-directional movement
- S or Space to fire
- M to deploy mine

**Numpad** (original PET Player 2 layout):
- 789/456/123 for 8-directional movement
- 5 or Space to fire
- 0 for mine

**Lobby keys**: C=create room, J=join room, 1-0=difficulty, Enter=confirm
**In-game**: ESC=back/disconnect

## Testing

```bash
go test -race ./... -count=1   # All tests with race detector
make wasm                       # Build WASM
make wasm-size                  # Check binary size (<5MB gzip)
```
