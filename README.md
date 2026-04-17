# TANK! — Networked Multiplayer Tank Battle

Faithful recreation of the Commodore PET TANK! game with networked multiplayer via WebSocket and WebAssembly.

## Version

Current: **0.9.2** (Wave 9 - Bot API & VS AI Mode)

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

## How to Play

1. Open `http://localhost:8080/` in a browser
2. Enter your name
3. **Create Room** — set difficulty (1-10) and get a 4-character room code to share
4. **Join Room** — enter a room code shared by your opponent
5. Once both players have joined, the WASM game client loads automatically

## Development

```bash
make dev           # Build WASM and start server
make wasm-size     # Check WASM binary size
make clean         # Remove build artifacts
```

## Project Structure

- `engine/` — Deterministic game engine (board, tanks, physics, collisions)
- `server/` — HTTP API + WebSocket server (rooms, matchmaking, delta encoding, bot manager)
- `client/` — WASM client (renderer, input, audio, CRT shader)
- `cmd/server/` — Server entry point
- `cmd/client/` — Client/WASM entry point
- `cmd/bot-go/` — Reference AI bot (Stage A state machine)
- `bot-sdk-go/` — Go SDK for building custom bots
- `web/` — Static files (HTML lobby, WASM binary, JS, CSS, fonts)
- `docs/BOT_API.md` — External developer guide for bot protocol

## Architecture

The game uses a two-phase connection model:

1. **HTML Lobby** (`/`) — Players create/join rooms via HTTP API, using a PETSCII-styled web interface with SSE for real-time updates
2. **WASM Game** (`/game`) — Once both players are ready, the browser redirects to the WASM game client which connects via WebSocket with a rejoin token

This eliminates the canvas-focus bug that plagues keyboard input in WASM-rendered lobby UIs.

## HTTP API

| Method | Endpoint | Description |
|--------|-----------|-------------|
| POST | `/api/room` | Create a new room (supports `vs_ai`, `auto_fill_bot`, `auto_fill_after_sec`) |
| POST | `/api/room/{code}/join` | Join an existing room |
| GET | `/api/room/{code}/status` | Poll room status (includes `is_bot`, `bot_class` for bot players) |
| GET | `/api/room/{code}/events` | SSE stream for room events (`player_joined`, `bot_joined`, `game_starting`) |
| GET | `/ws` | WebSocket for game communication |

## Server Flags

- `-addr` — listen address (default :8080)
- `-dir` — static files directory (default "web")
- `-cors` — CORS allowed origin (default "*")
- `-max-room-age` — stale room cleanup age (default 30m)

## Bot Mode

Set `TANK_ENABLE_BOTS=1` environment variable to enable bot opponents. When enabled:

- **VS AI**: Click "VS AI" in the lobby to play against an immediate CPU opponent
- **Auto-fill**: Create a room with `auto_fill_bot=true` and a bot joins after a configurable timeout if no human does
- Bots connect via the same WebSocket protocol as humans — third-party bots can use the `bot-sdk-go` package

**Runtime requirement**: The server spawns `bin/tank-bot` (or `bin\tank-bot.exe` on Windows) as a subprocess for each bot opponent. Run `make bot` (or `build.bat → bot`) to build it. The server resolves the binary in this order:

1. `TANK_BOT_BIN` environment variable (absolute path)
2. Same directory as the server executable
3. `./bin/tank-bot` (current working directory)

See `docs/BOT_API.md` for the full bot developer guide.

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

**In-game**: ESC to disconnect

## Testing

```bash
go test -race ./... -count=1   # All tests with race detector
make wasm                       # Build WASM
make wasm-size                  # Check binary size (<5MB gzip)
```