## TANK! 2P — Two-Player PyGame Port

**Version 2.0.0-alpha.2** — Python 3 / PyGame recreation of the classic two-player **TANK!** game from *Cursor Magazine* #26 (Commodore PET 4016). **Two-player only** — no AI, no tournament mode. Authentic **PETSCII retro** rendering with bundled PET font, monochrome phosphor-green palette, and character-cell visuals matching the original Commodore PET look.

### Features

- **Two-player** local play on a shared keyboard: **left tank** — numpad-style with S as the center (fire); **right tank** — numpad (all 8 directions move, 5 fire). Simultaneous movement.
- **Skill levels 1–10** — terrain density, ammo, mines, and tank speed scale with difficulty.
- **PETSCII retro visuals** — all game elements (border, walls, tanks, shots, mines, explosions, HUD) rendered as Commodore PET character glyphs via bundled PetMe64 font. Monochrome phosphor-green palette. Optional CRT scanline effect (`TANK_CRT=1`).

### Requirements

- Python **3.10+**
- Install from the repo root:

  ```bash
  pip install -e .
  ```

  Only **pygame** and **cbmcodecs2** are required (no AI dependencies).

  For pytest, Ruff, mypy, and other dev tools: `pip install -e ".[dev]"`.

### Running

```bash
python -m tank_game.main
```

### Menu flow

**MENU** → **SKILL SELECT** (1–10) → **PLAYING** → winner shown → **ANOTHER BATTLE? Y/N** → Y restarts at same skill, N returns to MENU.

### Project layout

| Path | Contents |
|------|----------|
| `tank_game/` | Game code: controller, board, players, projectiles, rendering, audio, UI. |
| `tests/` | Automated tests for board, player, projectile, and game logic. |
| `docs/` | Design notes, mechanics reference. |
| `docs/MECHANICS_TANK_PYGAME.md` | Mechanics reference for this port. |
| `CHANGELOG.md` | Version history and release notes. |

### Environment variables

| Variable | Role |
|----------|------|
| `TANK_DEBUG` | Extra logging to `tank_debug.log`. Use `0` / `false` / `no` / `off` to disable. |
| `TANK_CRT` | `1` — enable CRT scanline overlay effect. |
| `TANK_VSYNC` | `0` / `false` / `no` / `off` — disable vsync. |
| `SDL_VIDEODRIVER` | Tests use `dummy` (see `tests/conftest.py`). |

### Testing

```bash
pytest tests
ruff check tank_game tests
mypy tank_game
```

### License / assets

See `pyproject.toml` for package metadata. Original *TANK!* concept credits Commodore PET / *Cursor* era; this repository is the Python port's source tree.

---

## NetTank — Network Play (In Development)

**Branch: `NetTank`** — Multiplayer network play built on a pure Go game engine.

### Go Engine (`engine/`)

Wave 1 ported the entire Python game engine to Go as a pure logic package (no rendering, no I/O). The engine is the authoritative game state for networked play.

| File | Contents |
|------|----------|
| `engine/constants.go` | CellType, Direction, GameState, Action enums; timing constants |
| `engine/board.go` | Board struct, terrain generation, cell operations |
| `engine/player.go` | Tank struct, movement, barrel tracking, resources |
| `engine/projectile.go` | Shot and Mine structs, movement, detonation |
| `engine/game.go` | GameController — full update loop, all collision handling |
| `engine/shotshot.go` | Shot-shot collision detection (including head-on swap) |
| `engine/mine_explosion.go` | Mine explosion and chain reaction logic |
| `engine/match_runner.go` | Headless match runner with InputProvider interface |

**158 tests** across 9 test files. Run with:

```bash
go test ./engine/... -count=1
```

### Go Server (`server/`)

Wave 2 adds an authoritative WebSocket server for networked 2P play. Uses `coder/websocket` v1.8.12 for both server and future WASM client.

| File | Contents |
|------|----------|
| `server/protocol.go` | JSON message types (Envelope, InputMsg, TickMsg, etc.), engine→wire converters, KeyToAction |
| `server/room.go` | Room struct with player management, lifecycle states |
| `server/hub.go` | Hub with crypto/rand room code generation, stale room cleanup |
| `server/input.go` | InputTracker — KEYDOWN/KEYUP edge→Action translation, repeat suppression |
| `server/match.go` | MatchController — 60Hz game loop wrapping GameController, round/game lifecycle |
| `server/ws_handler.go` | WebSocket handler, ClientConn, RoomBridge, ConnRegistry |
| `cmd/server/main.go` | Server binary with graceful shutdown and periodic cleanup |

**58 tests** including 5 E2E integration tests. Run with:

```bash
go test -race ./server/... -count=1
```

Start the server:

```bash
go run ./cmd/server/ -addr :8080
```

### Planned Waves

| Wave | Scope | Status |
|------|-------|--------|
| Wave 1 | Go game engine | ✅ Complete |
| Wave 2 | WebSocket protocol + Go server | ✅ Complete |
| Wave 3 | WASM client + Ebitengine rendering | Planned |
| Wave 4 | Latency compensation | Planned |
| Wave 5 | Lobby + matchmaking | Planned |
