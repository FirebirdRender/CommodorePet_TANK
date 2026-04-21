## Changelog

### 1.4.3 - 2026-04-20 — Bot AI: linearize skill ladder with diagonal gating

**Diagonal firing gated to d=4+** (was engine-level, available to all).
Empirical 1000-match tournament verified the intended behavior: low-skill bots (d=1-3) are more tactically limited (only cardinal shots), making the mid-tier difficulty ramp more meaningful.

- Win-rate shift after gating:
  - d=1: 63.8% → **76.8%** (low-skill higher-skill advantage amplified)
  - d=2: 71.4% → **82.6%**
  - d=3: 68.9% → 64.5%
  - d=5: 79.1% → 67.0% (peak flattened; mid-tier closer to linear)
  - d=10: 78.4% → **81.2%**

Files: `cmd/bot-go/skill_config.go`, `cmd/bot-go/ai.go`, `BOT_AI_SCALING_GRID.md`

### 1.4.2 - 2026-04-20 — Bot AI: gated diagonal firing + skilltest harness

**Diagonal firing now skill-gated (d=4+).** Previously all difficulties could fire along 45-degree diagonals — this was engine-level behavior, not an earned skill. Now lower-skill bots (d=1-3) are restricted to cardinal row/column shots only, which steepens the mid-tier skill ladder.

- `SkillConfig.EnableDiagonalFire` field added; `false` for d=1-3, `true` for d=4-10
- `cmd/bot-go/ai.go::canFireWithLOS` diagonal alignment check guarded on `bs.skillConfig.EnableDiagonalFire`
- `BOT_AI_SCALING_GRID.md` updated with `DiagFire` column and d=4 unlock note

**Skill-vs-skill test harness (`cmd/bot-skilltest/`).** Automated headless tournament runner that validates higher-skill bots beat lower-skill bots.

- Bot-skilltest binary created: concurrent match spawning, configurable match count + concurrency
- Per-match timeout (default 300s) → bots killed, result recorded as DRAW
- CSV + JSON output with per-difficulty summaries (passed/failed/draws + higher-skill win rate)
- Server auto-room disabled during harness runs to eliminate phantom bot interference
- Stderr pipe continuously drained to prevent server I/O deadlocks

**Build:** `make skilltest` target added; included in `make build-all`.

### 1.4.1 - 2026-04-20 — Fix VS AI room status regression from S7

S7's room status enumeration protection was too aggressive — it blocked
authenticated lobby polls for full/playing rooms, breaking VS AI mode
where the bot joins immediately and the room becomes "full".

Fix: `handleRoomStatus` now accepts `?token=` or `X-Auth-Token` header.
Unauthenticated requests still get generic 404 for non-joinable rooms.
Authenticated requests see full room status (needed for lobby poll → game redirect).

Also updated `web/app.js` to send token on SSE and status poll requests.

### 1.4.0 - 2026-04-20 — Security hardening Phase 4 (AF1, AF2, AF3, AF4)

Phase 4 — Additional findings from security audit:

- **AF1: Spectate token crypto/rand** — Deleted weak `generateRoomCode()` (time-based); `GenerateSpectateToken` now uses `crypto/rand` with 16-byte hex; `AddSpectator` ID also uses `crypto/rand` (8-byte hex)
- **AF2: CI/CD pipeline** — `.github/workflows/test.yaml` added; runs `go test -race`, `govulncheck`, and builds server+bot on push to main/NetTank/develop
- **AF3: Python deps** — No-op; `pyproject.toml` removed in v1.0.0 Python codebase cleanup
- **AF4: lagproxy documentation** — S8 `//go:build dev` tag applied in Phase 3

All 7 blockers, 8 strongly recommended items, and 5 nice-to-haves from the security audit are now addressed.

### 1.3.0 - 2026-04-20 — Security hardening Phase 3

Phase 3 of the security mitigation plan (B7, S8, N4, N5):

- **B7: Bot binary path validation** — `TANK_BOT_BIN` must be absolute; `resolveBotBinary` validates no path traversal via `filepath.EvalSymlinks` + allow-list; startup fails if invalid
- **S8: lagproxy build tag** — `cmd/lagproxy/main.go` now has `//go:build dev` tag; excluded from production builds
- **N4: API timeout** — Server-level `WriteTimeout: 300s` caps API responses; SSE/WS have their own timeouts; documented in middleware chain comment
- **N5: govulncheck Makefile target** — `make govulncheck` installs and runs `govulncheck ./...`

### 1.2.0 - 2026-04-20 — Security hardening Phase 2

Phase 2 of the security mitigation plan (B4, B5, B6, S3, S4, S5, S7):

- **B4: pprof import removed** — `_ "net/http/pprof"` gone from main.go; `-pprof-addr` flag replaced with `-debug-addr` (dev-only, no auth)
- **B5: WebSocket connection cap** — `-max-conns` flag (default 1000); atomic counter rejects with 503 when limit reached
- **B6: HTTP rate limiting** — Per-IP `golang.org/x/time/rate` middleware; `-rate-requests` (5/s) and `-rate-burst` (10) flags; bot subprocess cap (`-max-bots`, default 20); `ExtractIP` helper respects `X-Forwarded-For`
- **S3: Hub room count cap** — `-max-rooms` flag (default 500); `CreateRoom`/`CreateRoomWithBotPolicy` return nil when cap reached; `CleanupStaleRooms` now also evicts `RoomGameOver` rooms past maxAge
- **S4: AutoFill timer cancellation** — `CleanupStaleRooms` calls `CancelBotReservation()` before deleting; `auto_fill_after_sec` validated ≤ 300
- **S5: Max match duration** — `MaxMatchTicks = 36000` (10 min @ 60 Hz); `MatchController.stepTick` force-stops and emits `GameOverMsg{Winner: 0}` on timeout
- **S7: Room code enumeration protection** — `handleRoomStatus` returns generic 404 for missing, full, playing, game-over, and closed rooms

New dependency: `golang.org/x/time v0.7.0` (rate limiter)
`NewHub` signature changed: now takes `maxRooms int`
`NewBotManager` signature changed: now takes `maxBots int`

### 1.1.0 - 2026-04-20 — Security hardening Phase 1

Phase 1 of the security mitigation plan (B1, B2, B3, N3, S1, S2, S6):

- **B1: WebSocket origin verification** — `InsecureSkipVerify: true` replaced with configurable `OriginPatterns`; `-allowed-origins` flag accepts comma-separated host patterns (empty = allow all, dev only)
- **B2: CORS default changed** — Default CORS origin changed from `*` to `https://localhost:8080`; SSE `Access-Control-Allow-Origin` now uses the configured CORS value instead of hardcoded `*`; `X-Auth-Token` added to allowed headers
- **B3: HTTP request body size limit** — `MaxBytesReader(4KB)` added to `handleCreateRoom` and `handleJoinRoom`; oversized requests return 413
- **N3: WebSocket message size limit** — `SetReadLimit(4096)` added to `readPump` matching the HTTP body limit
- **S1: WebSocket read timeout** — 90s context timeout per read in `readPump`; idle connections are closed
- **S2: SSE authentication** — `/api/room/{code}/events` now requires a valid token via `?token=` or `X-Auth-Token` header; validates room code match; returns 401/403 on failure
- **S6: Security headers** — `SecurityHeadersMiddleware` adds `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Permissions-Policy`
- **B5 (partial): Connection cap plumbing** — `maxConns` and `atomic.Int64` counter added to `WSHandler`; `-max-conns` flag (default 1000); connections exceeding cap receive 503
- **B4 (partial): pprof import removed** — `_ "net/http/pprof"` removed from `main.go`; replaced with `-debug-addr` flag for dev-only pprof on separate port

Server flags added: `-allowed-origins`, `-max-conns`, `-debug-addr`
`NewWSHandler` signature changed: now takes `originPatterns []string, maxConns int64`
`NewRoomAPI` signature changed: now takes `cors string`

### 1.0.0 - 2026-04-20 — Go/WASM finalization: Python codebase removed

**Breaking change:** The Python/PyGame codebase (`tank_game/`, `tests/`, `pyproject.toml`) has been fully removed. The project is now exclusively Go/WASM. All Python dependencies, build files, test frameworks, and documentation have been cleaned up.

**Removed (git-tracked):**
- `tank_game/` — entire Python/PyGame source tree (14 `.py` files, font assets, `__pycache__`)
- `tests/` — Python pytest suite (5 test files)
- `pyproject.toml` — Python project definition (`tank-game-2p`)
- `gameplay.analysis.md` — Python-era technical spec
- `SECURITY_REPORT_041926.md` — root-level duplicate (kept in `docs/`)
- `scripts/benchmark_headless_showdown_offline.py` — Python headless benchmark
- `docs/PRD_AI_NAVIGATION.md`, `docs/PRD_py_trees_integration.md` — Python AI PRDs (replaced by Go bot AI)
- `docs/PYGAME_QA_TESTING_SPEC.md` — PyGame QA spec (replaced by `go test`)
- `docs/UI_RETRO_SPEC.md` — Python PETSCII retro spec (replaced by Go renderer)
- `docs/FONTS/PetMe.ttf`, `docs/IMAGES/` — Python-only font copy and PyGame screenshots
- `docs/tank-*.png`, `docs/tanks-*.png` — PyGame gameplay screenshots

**Removed (local-only):**
- `.venv/` (208 MB Python virtualenv)
- `tank_game_2p.egg-info/`, `.pytest_cache/`, `.ruff_cache/`
- Build artifacts: `tank-bot-1.00`, `tank-server-1.00`, `lagproxy`, `client_bin`, `server_bin`, `client.test`, `coverage.out`, `bot-load`, `cmd/server/server`
- Dev screenshots (8 PNG verification images)

**Fixed:**
- `Makefile`: `copy-font` target no longer references deleted `tank_game/assets/fonts/` — font is embedded via `//go:embed` in `internal/assets/`
- `Makefile`: `copy-web-fonts` target now copies from `internal/assets/fonts/PetMe64.ttf` instead of deleted `docs/FONTS/PetMe.ttf`

**Verification:** `make clean && make build-all` passes. `go test -race ./... -count=1` passes. `go vet ./...` clean.

**Bumped:** `AppVersion = "1.0.0"`, `VERSION = "1.0.0"`.

### 0.9.13 - 2026-04-19 — Spectator UI overhaul + bot debug logging + match list SSE

**Spectator page (web/spectate.html) rewritten and tuned:**

- **HUD header fix.** Restored the separator cell layout to match the WASM client exactly — inverted `●` glyphs in cols 19-20, row 0-1; black glyphs on phosphor-green background; player panels use inverted rendering (black glyph, green bg) matching the original PET TANK! look.
- **Status messages ported.** `THE WINNER`, `OUT OF SHOTS`, `LOW SHOTS`, `LAST TANK` now render as inverted glyphs in the playfield border row, exactly as the WASM client does.
- **Canvas scaling.** Added `resizeCanvas()` with 90%-viewport scaling (no cap — scales up on large screens, down on small), centered in the viewport with `image-rendering: pixelated`.
- **Per-bot random skill in HUD separator.** The two center-top separator cells now show each bot's skill level (0-9) as non-inverted digits. P1 skill at col 19, row 0; P2 skill at col 20, row 0. Bots with skill 0 display `●` (hidden, like a human player).

**Bot debug logging (`cmd/bot-go/main.go`):**

- New `-botdebug` flag (default off). When set, enables verbose per-tick bot decision logging (`[t270 dl n=270] me=(29,10) act=true ...`).
- Disabled by default — eliminates continuous console spam from always-on bot matches.

**Match list SSE wiring (`server/room_api.go`):**

- Added `handleListRooms` (GET `/api/rooms?filter=playing|waiting`) returning JSON array of active rooms with code, difficulty, state, and player info.
- Added `handleGlobalEvents` (GET `/api/events`) SSE stream for real-time lobby updates on match start/end.
- Both wired into `RoomAPI.RegisterRoutes`.

**Backend per-bot skill (`server/`):**

- `Player.Skill` field added (0 for humans, 0-9 for bots).
- `AddBotPlayerAt` now accepts `skill int` parameter.
- `bot_manager.go` assigns `rand.IntN(10)` skill to each bot at spawn.
- `MatchController` gains `player1Skill`/`player2Skill` + `SetPlayerSkills()`, populated by `startMatchForRoom` from `room.Players[].Skill`.
- `TickMsg` gains `Player1Skill`/`Player2Skill` fields, sent every tick.

**Verification:** `go test -race ./server -count=1` passes (71s). Playwright confirmed HUD separator cells rendering non-inverted digits for live bot skills.

**Bumped:** `AppVersion = "0.9.13"`.

**Bug fixes.** Five bugs reported against 0.9.11.

**Bug 1 — Server hangs after SIGINT (`cmd/server/main.go`)**:

- Root cause: signal handler closed WS conns + bot subprocs but never called `srv.Shutdown()`, leaving `srv.Serve(ln)` blocked forever.
- Added `context` import and `srv.Shutdown(ctx)` with 10s timeout after `hub.Shutdown()`.

**Bug 2 — Spectator input affecting Player 1 tank (`server/ws_handler.go`, `client/gamestate.go`, `client/bridge_js.go`, `client/game.go`)**:

- Root cause: `handleSpectate` sent `GameStartState(1)` to spectators, so the WASM client's `ApplyGameStart` set `Phase = PhasePlaying` and `PlayerID = 1`, making the spectator's client believe it was Player 1. Local barrel-direction prediction then ran on Player 1's tank in the spectator's view.
- `handleSpectate` now sends `GameStartState(0)` — `YourPlayerID == 0` is the spectator marker.
- `ApplyGameStart` now branches on `YourPlayerID`: `0` → `PhaseSpectating`, non-zero → `PhasePlaying`.
- Defense-in-depth: `window.sendInput` (bridge_js.go) early-returns when `g.isSpectator`; `Update()` input pump gated by `&& !g.isSpectator`.

**Bug 2 — Tanks appear frozen after spectator joins (`server/ws_handler.go`)**:

- Root cause: `RoomBridge.OnTickSpectator` iterated `rb.clients` (player conns only), filtering for `c.isSpectator == true`. Spectators are stored in `Room.spectatorConns`, not `rb.clients`, so spectators received zero ticks.
- Added `room *Room` field to `RoomBridge`; spectator broadcast methods now call `rb.room.BroadcastToSpectators()`.

**Bug 3 — Spectator stuck on game screen after match ends (`server/ws_handler.go`, `client/game.go`)**:

- Root cause: `RoomBridge.OnGameOver` only iterated `rb.clients` — spectators never received `game_over`, leaving the spectator client frozen in `PhasePlaying`.
- Server: `OnGameOver` now also calls `rb.room.BroadcastToSpectators(MsgTypeGameOver, ...)`.
- Client: when `MsgTypeGameOver` arrives in spectator mode, switches to `PhaseWaitingReconnect` with a 5s `DisconnectCountdown` and a "PLAYER #N WINS - RETURNING TO LOBBY" message; existing countdown logic auto-redirects to lobby.

**Bug 4 — ESC behavior in spectator mode (`client/game.go`)**:

- ESC in `PhaseSpectating` now requires confirmation: first ESC sets `EscConfirmPending`, second ESC returns to lobby.
- Per user requirement: "the only KEY INPUT that should be taken is ESC, which should ask for a confirmation, then leave spectating back to the menu".

**Regression guard (`server/e2e_full_test.go`)**:

- `TestSpectatorJoinAndReceiveTicks` now asserts `gsMsg.YourPlayerID == 0` with comment tying the assertion to the v0.9.12 fix.

**Bumped:** `AppVersion = "0.9.12"`. Protocol unchanged (no new message types, no shape changes).

### 0.9.11 - 2026-04-19 — Test infrastructure completion (WAVE7 T3/T4, WAVE8 T6 gaps)

**Testing.** Fills the remaining test infrastructure gaps identified in the WAVE7 and WAVE8 plan reviews.

**WASM compile verification (`Makefile`, `client/wasm_test.go`)**:

- Added `test-wasm` Makefile target: builds and vets the client package under `GOOS=js GOARCH=wasm`
- Created `client/wasm_test.go` with `//go:build js` tag: compile-time type assertions for `getJSConfig`, `redirectLobby`, `ExportGameState`; runtime `TestJSConfigFields` skips gracefully if `window.tankConfig` isn't set
- `make test-wasm` now exits 0 with "WASM compile check passed" message
- Target is deliberately NOT included in `test-all` (cross-compilation is slow)

**Renderer logic tests (`client/renderer_test.go`)**:

- `TestDrawHUD_PanelLayout` (6 sub-tests): P1 panel range (cols 0-18), P2 panel range (cols 21-39), separator columns (19-20), total width verification, label row with AI level, value row formatting
- `TestDrawGrid_CellGlyphSelection` (6 sub-tests): CellBorder glyph definition (● inverted), CellWall glyph (▚ non-inverted), CellEmpty (space, black), CellBorder vs CellWall differentiation, border detection at corners, interior positions
- `TestDrawHUD_StatusMessagePriority` (7 sub-tests): winner overrides all, zero shots override last tank, maxShots=0 no status, low shots at 20%, just above threshold, last tank with enough shots, low shots priority over last tank

**Room status API tests (`server/room_api_test.go`)**:

- `TestRoomAPI_GetRoomStatus` (3 sub-tests): NotFound → 404, Waiting → 200 with status:"waiting"+difficulty+players, Playing → 200 with status:"playing"+2 players
- Uses `api.handleRoomRoutes(w, r)` for full URL dispatch chain validation

**SSE integration tests (`server/room_api_test.go`)**:

- `TestRoomAPI_RoomEvents_SSE` (2 sub-tests): NotFound → receives `room_expired` event, PlayerJoined → receives `player_joined` event after second player joins
- Uses `httptest.NewServer` (not `NewRecorder`) for proper SSE streaming
- Uses `context.WithTimeout`/`context.WithCancel` to prevent goroutine hangs
- Added imports: `bufio`, `context` to existing test file

**WAVE7/WAVE8 plan docs updated**: Both `PROJECT.NET/WAVE7_PLAN.md` and `PROJECT.NET/WAVE8_PLAN.md` now include completion status tables mapping each task to evidence.

**Bumped:** `AppVersion = "0.9.11"`. Protocol unchanged.

### 0.9.10 - 2026-04-18 — PET-authentic HUD and border rendering

**Visual fidelity fix.** Replaces the single-line text HUD and checkerboard border with PET-authentic two-row header and bubble border matching original PET screenshots.

**HUD (`client/renderer.go`)**:

- Two-row layout: label row ("TANKS  SHOTS  MINES") over value row (numeric stats), matching the original PET header
- 2-column inverted circle separator between P1/P2 panels (cols 19–20)
- Inverted text rendering (black glyphs on green bar) via glyph cache, aligned to cell grid
- Per-player status messages ("LOW SHOTS", "OUT OF SHOTS", "LAST TANK", "THE WINNER") rendered in the playfield border row, not in the HUD area
- Priority system: THE WINNER > OUT OF SHOTS > LOW SHOTS > LAST TANK

**Border (`client/renderer.go`)**:

- Playfield perimeter now renders as inverted circle glyphs (`●` with `Inverted: true`) — green cell with black circle cutout, creating the "bubble" border seen on original PET
- Interior walls remain checkerboard (`▚`, non-inverted) — unchanged
- Position-based detection: `x==0 || x==39 || y==0 || y==20` (no protocol change)

**Glyph cache (`client/glyph.go`)**:

- Added `CellBorder = 11` (rendering-only constant, not a protocol cell type)
- Added `CellBorder` glyph definition: `{Rune: '●', Inverted: true}` for bubble border
- Added `HUDP1End = 19`, `HUDP2Start = 21` layout constants
- Added `cellFace` (20px) to `Renderer` for grid-aligned HUD text
- Expanded `hudChars` pre-warming for full label character set

**Helpers (`client/renderer.go`)**:

- `difficultyToMaxShots()`: mirrors engine `DifficultyToResources` shot table per difficulty level
- `hudLabelRow()`: formats "TANKS  SHOTS  MINES" (or AI variant) padded to 19 chars
- `hudValueRow()`: formats numeric values aligned under labels
- `hudStatusMessage()`: returns priority-ordered status string
- `drawHUDPanel()`: renders inverted text character-by-character at cell boundaries
- `drawBorderMessage()`: renders status messages in the border row centered per panel

**Tests (`client/renderer_test.go`)**:

- `TestHUDLabelRow`: 4 cases (no AI, AI levels 1/5/9)
- `TestHUDValueRow`: 3 cases (standard, high values, zeros)
- `TestHUDStatusMessage`: 8 cases covering all priority branches
- `TestDifficultyToMaxShots`: 8 cases mirroring engine test
- `TestBorderDetection`: 10 cases for corner/edge/interior positions
- `TestCellBorderGlyph`: verifies CellBorder glyph definition
- Updated `TestCellGlyphsCoversAllCellTypes` and `TestCellTypeValues` for CellBorder

**Bumped:** `AppVersion = "0.9.10"`. Protocol unchanged.

### 0.9.9 - 2026-04-18 — Phase 6 G-3: Bot-vs-bot load harness + Phase 6 close

**Closes Phase 6 gap G-3.** Adds `cmd/bot-load`, a subprocess-based load harness that drives 100+ real bot-vs-bot matches over the network against a live `tank-server`, validates per-match outcomes, and verifies the server has no goroutine leaks across the tournament. Implementation follows the Oracle-confirmed full design (subprocess + real network) with random independent skill levels per bot tracked as a 10×10 skill-vs-skill matrix.

**New harness binary** (`cmd/bot-load/main.go`):

- Spawns one `tank-server` process (`-addr :0 -pprof-addr 127.0.0.1:6160` with `TANK_ENABLE_BOTS=1`); parses the resolved port from the `listening on :PORT` log line on **stderr** (Go's `log.Printf` defaults to stderr; harness routes stdout pass-through).
- Detects pprof bind-collision early (`[pprof] server error` line) to fail fast if a stale server holds port 6160.
- Creates rooms via HTTP (`POST /api/room` then `POST /api/room/{code}/join`); harvests both player tokens.
- Spawns two `bot-go` subprocesses per match with `-skill`, `-token`, `-exit-after-gameover`, `-summary-file` flags. Each subprocess's stdout/stderr is drained into a `cappedBuffer` (64KB) to prevent pipe deadlock — Oracle-flagged risk.
- **Stratified scheduler**: 10×10 skill grid, deterministic seed (`-seed`, default time-based), shuffled per tournament. Sample-without-replacement for `<100` matches; cycling for `>100`.
- **Concurrency**: `sync.WaitGroup` + buffered semaphore (`-concurrency`, default 4). Server-death watchdog via background `cmd.Process.Wait()` → abort channel.
- **Per-match validation**: bot exit codes, summary file presence on both sides, winner agreement between p1/p2 reports, valid winner ∈ {1,2}. Per-match wall budget (`-match-timeout`, default 120s) enforced via `context.WithTimeout`; both bots SIGKILLed on overrun.
- **Goroutine leak detection**: baseline snapshot via `/debug/pprof/goroutine?debug=1` after server start (with retry/backoff for the async pprof bind), final snapshot after a 5s settle window. Configurable threshold (`-leak-threshold`, default 5).
- **JSON report** (`-report`): seed, per-pair aggregates (Played/P1Wins/P2Wins/Failures/AvgTicks/AvgWallSec), full match detail array, leak status. Final stdout summary always printed.
- Process exits non-zero if any match fails or the leak threshold is exceeded.

**Bot-side support** (`cmd/bot-go/main.go`, `cmd/bot-go/ai.go`):

- New flags: `-skill <0-9>`, `-exit-after-gameover` (clean exit on terminal state vs. lobby return), `-summary-file <path>` (writes JSON `matchSummary` on GameOver).
- New `BotState` counters: `FireCount`, `MoveCount`, `MineCount` incremented at the three action sites; emitted in summary alongside `Winner`, `MyPlayerID`, `Difficulty`, `FinalWins`, `DroppedInputs`, `Ticks`.

**Server cleanup** (`cmd/server/main.go`):

- Removed leftover `_ = strings.HasPrefix` workaround line and the now-unused `strings` import (compile-clean post-G-3 plumbing).

**Verification:**

- Full `go test -race ./... -count=1`: PASS.
- `go build ./cmd/bot-load ./cmd/bot-go ./cmd/server`: clean.
- **Smoke run** (10 matches @ concurrency 4): 9/10 PASS, 1 FAIL (skill 5 vs 9 stalemate timeout). Goroutine delta=1 (PASS, threshold=5). Wall time 180s.
- **Full tournament** (100 matches @ concurrency 4, seed=1776527240002274203): **94/100 PASS, 6 FAIL** — all 6 failures are bot-vs-bot stalemate timeouts at the 120s match cap (both bots alive, full ammo, not engaging). Affected pairs: (3,8), (0,4), (1,0), (5,8), (6,5), (0,2) — no skill-level pattern. **Goroutine leak target met: baseline=7, final=8, delta=1, threshold=5 → PASS.** Wall time 1679.5s (~28 min).

**Known limitation (Phase 6 → Phase 7 carry-over).** The reference bot's engagement logic produces stalemates in approximately 6% of bot-vs-bot matches when both bots end up at long-distance non-aligned positions and neither bot's heuristic chooses to close. This is **not** a harness/network/leak defect — both bots run normally with full ammo and are killed by the harness's `match-timeout`. Symptom signature (from `bot_debug.log` tail): `pend=move/...|none` repeatedly, `sh=10 mi=2`, distant XY coordinates with no row/column alignment. Tracking as a Phase 7 AI improvement (deterministic engagement closer or duel timer with forced approach).

**Bumped:** `AppVersion = "0.9.9"`. `ProtocolVersion` unchanged at `1.0.0` (G-3 is harness/test infrastructure + bot CLI flags; wire format untouched).

**Phase 6 status: CLOSED.** All three Phase 6 gaps (G-1 anti-cheat ack-window, G-2 per-slot bot seat reservation, G-3 bot-vs-bot load harness with leak verification) shipped. Tagging `v1.0.0-phase6`.

### 0.9.8.1 - 2026-04-17 — Hotfix: bot fire LOS regression from 0.9.7.3

**Bugfix.** The 0.9.7.3 spawn-block guard in `cmd/bot-go/ai.go::canFireWithLOS` used `bs.isOpen(myBarrelX, myBarrelY)` to validate the bot's own barrel cell, but `isOpen` only returns true for `cellEmpty (=1)`. The engine encodes the bot's own barrel cell in keyframes as `CellBarrel1 (=5)` or `CellBarrel2 (=6)` (see `engine/player.go:24-26`, `engine/game.go:279`), so `isOpen` was **always false** for the bot's own barrel. The point-blank exception only saved the trivial case where the enemy stood literally on the barrel cell. **Result:** the bot rejected every legitimate fire opportunity in real playtests — confirmed by user log showing P2 at (37,10) aligned with P1 at (2,10) on row 10 with 6 shots and full clear LOS, never firing across ~2280 ticks.

**Engine ground truth** (`engine/game.go:160 ShotSpawnPosition`): rejects ONLY `CellWall`. Barrel cells, tank cells, mines, wreckage, and shots are all valid spawn cells.

**Fix.** Replaced the broken `isOpen`-based guard with a new `canSpawnShot(x,y) bool` helper that mirrors `ShotSpawnPosition` exactly: returns `grid[y][x] != cellWall`, with OOB safety. Added `cellWall = 2` constant. Updated `canFireWithLOS` docstring to document the engine-correctness contract and the 0.9.7.3 failure mode so this regression cannot recur silently.

**Verification:** Full `go test -race ./... -count=1` PASSES. `TestBot_VsBot_FullMatch` PASS in 38.36s. User playtest log (post-fix) shows 4 fire events at ticks 597/921/1137/1677 — bot correctly fires when aligned and tracks the foe.

**Bumped:** `AppVersion = "0.9.8.1"`. `ProtocolVersion` unchanged at `1.0.0` (bot-only fix; wire format untouched).

### 0.9.8 - 2026-04-17 — Phase 6 G-2: Bot API extended for bot-vs-bot (per-slot seat reservation)

**Closes Phase 6 gap G-2.** The Bot API previously assumed a fixed P2 seat for the bot, which precluded bot-vs-bot rooms (both seats need to be reservable). This release extends `Room` and `BotManager` to support per-slot seat reservation while preserving the existing single-bot vs-AI flow as the default.

**Approach:** Option 1 (Oracle-confirmed) — extend `Room`/`BotManager` to support seat 0 rather than introduce a parallel API. Backward-compatible: existing callers using `ReserveBotSeat()` / `AddBotPlayer()` continue to work and target slot 1 (P2 seat).

**Implementation:**

- `server/room.go`:
  - `BotSeatReserved` is now `[2]bool` (per-slot), not a single bool. Index 0 = P1 seat, index 1 = P2 seat.
  - New `ReserveBotSeatAt(slot int) error` — slot-aware reservation. `ReserveBotSeat()` is preserved as a thin wrapper calling `ReserveBotSeatAt(1)`.
  - New `AddBotPlayerAt(slot int, name, botID, botClass string) (int, error)` — slot-aware bot insertion. `AddBotPlayer(...)` is preserved as a wrapper calling `AddBotPlayerAt(1, ...)`. Returns `slot+1` as the player ID.
  - New `CancelBotReservationAt(slot int)` — slot-aware cancellation. `CancelBotReservation()` clears both slots.
  - `AddPlayer` now refuses slot 0 if `BotSeatReserved[0]` is set (previously only enforced for slot 1).
  - `RemovePlayer` clears `BotSeatReserved[playerID-1]` (previously only cleared the single global flag).
  - State transition to `RoomReady` now requires both seats filled (was: any seat filled with `BotSeatReserved` true).
- `server/bot_manager.go`: extended to drive bot processes for either slot — argv carries the assigned `playerID`, lifecycle hooks update the correct slot index.
- `server/room_test.go`: updated existing tests to use the new `[2]bool` semantics + added coverage for slot-0 reservation, dual-bot reservation, and slot-aware cancellation.

**Test results:** Full `go test -race ./... -count=1` passes (server suite 107s with race detector). `TestBot_VsBot_FullMatch` passes 5/5 deterministically.

**Bumped:** `AppVersion = "0.9.8"`. `ProtocolVersion` unchanged at `1.0.0` (G-2 changes are server-internal; wire format untouched).

### 0.9.7.3 - 2026-04-17 — Bot AI: livelock + 8-direction + diagonal fire + spawn-block guard

**Bugfix.** `TestBot_VsBot_FullMatch` was flaking due to four independent defects in the bot AI that compounded under specific spawn geometries. After this release the test passes 5/5 deterministically.

**Defect 1 — Symmetric phase-lock livelock.** Mirror-image spawns with identical seeds and difficulty caused both bots to tick in lockstep through `rotate→gate→rotate`, never reaching a fire opportunity. **Fix:** P2 gets a one-shot `+1` tick offset on its first `commitMove` (`firstActionDone` flag in `BotState`). After the first decision, ack timing + grid asymmetry keep them desynced naturally.

**Defect 2 — Rotation-only ack burned the gate.** When a `move` input was consumed by the engine as a turn (not a translation — barrel-curl first input), the bot still waited the full `thinkingDelayTicks` before its next decision, idling ~28 of 30 gate ticks. **Fix:** `checkAck` now resets `nextActionTick=tick` when the ack is rotation-only, letting `startNewAction` either fire (if newly aligned) or commit the second input (the actual move) on the very next tick.

**Defect 3 — 8-direction movement not exploited.** Bot only considered the 4 cardinal directions for movement and LOS, despite the engine supporting `DirUpLeft..DirDownRight` with `DirectionVectors {+/-1, +/-1}`. **Fix (Option A, Oracle-confirmed):**

- `dirToward` now returns one of the 4 diagonal direction constants when `dx != 0 && dy != 0 && abs(dx) == abs(dy)`.
- `dirToKey` maps the 4 new direction constants to `up_left`/`up_right`/`down_left`/`down_right`.
- `canFireWithLOS` adds a 45° diagonal alignment branch that walks one cell diagonally per step, matching engine `Shot.Step` semantics exactly.

**Defect 4 — `FireShot` spawn-block livelock.** When the cell directly in front of the bot was a `CellWall`, `canFireWithLOS` returned true (own-barrel cell unconditionally treated as open) but the engine's `ShotSpawnPosition` silently rejected the shot. `ShotsLeft` never decremented, no `pendingFire` ack ever fired, and the bot infinite-looped trying to fire (578× `skip:spawn_blocked` vs 1× `ok:fired` in the captured failing run). **Fix:** `canFireWithLOS` now rejects when the barrel cell is non-open, with a **point-blank exception** for when the barrel cell is occupied by the enemy tank itself (engine `ShotSpawnPosition` only rejects `CellWall`, not tanks/barrels — adjacent enemy is a valid fire target).

**Watchdog.** Added a 3× `thinkingDelayTicks` watchdog in `Decide` that force-clears `nextActionTick` if no action is pending but the gate is still locked far past any legitimate thinking delay. Defense-in-depth against future regressions.

**Debug instrumentation (env-gated, defaults off — zero perf cost when disabled):**

- `TANK_BOT_VERBOSE=1` — bot per-tick decision log (`cmd/bot-go/main.go`).
- `TANK_FIRE_DEBUG=1` — engine logs all four `FireShot` outcomes: `ok:fired`, `skip:cannot_fire`, `skip:active_shot_exists`, `skip:spawn_blocked` (`engine/debug.go` + `engine/game.go`).
- `TANK_INPUT_DEBUG=1` — server logs Fire keypress lifecycle: `KeyDown(Fire) DROPPED/QUEUED`, `KeyUp(Fire)`, `Consume` (`server/input.go`).

Player IDs are now wired through `InputState.SetPlayerID` from `NewMatchController` so the input-debug logs correctly label P1 vs P2.

**Bumped:** `AppVersion = "0.9.7.3"`. `ProtocolVersion` unchanged at `1.0.0`.

### 0.9.7.2 - 2026-04-17 — Server version banner

**Feature.** The server now logs its version on the startup line so operators can identify the running build at a glance, both in the console and in log files.

**Before:** `TANK! server listening on :8080`
**After:**  `TANK! server (v0.9.7.2) listening on :8080`

**Implementation:**

- New `server/version.go` exporting `AppVersion = "0.9.7.2"`. This is the human-facing app version (separate from `ProtocolVersion`, which is the wire-format version).
- `cmd/server/main.go:92` now formats `"TANK! server (v%s) listening on %s"` with `server.AppVersion`.

**Bump policy:** `AppVersion` follows the existing changelog cadence — bumped on every release. `ProtocolVersion` stays at `1.0.0` and only changes per the `docs/BOT_API.md` §10 policy.

### 0.9.7.1 - 2026-04-17 — Mine arming delay (2-second fuse)

**Bugfix.** Laying a mine immediately exploded under the layer because `ResolveTankMineCollision` only gated on `mine.Active`, and `NewMine` set `Active=true` at placement. There was no arming delay at all — the visible-duration timer (`MineVisibleDuration`) was a separate concern that only controlled board-cell rendering, not detonation.

**New semantics (Strict):**

- A mine is **inert** at placement.
- The arming timer starts the **first** tick the layer's tank leaves the mine cell.
- After `MineArmDelay = 2.0` seconds, the mine becomes **armed** and **invisible** (`Visible=false`, board cell cleared from `CellMine`).
- Once armed, the mine **never re-disarms** — even if the layer returns to the cell, it detonates.
- Chain reactions (mine adjacent to another exploding mine) still ignore the arm gate (intentional — explosion damage propagates regardless of arm state).

**Implementation:**

- `engine/constants.go` — added `MineArmDelay = 2.0`.
- `engine/projectile.go` — `Mine.ArmedTime float64` field (0 = not yet armed) + `IsArmed(simTime) bool` method.
- `engine/mine_explosion.go` — `ResolveTankMineCollision` now gated on `mine.IsArmed(gc.SimTime)`.
- `engine/game.go` — `updateMines` sets `ArmedTime = SimTime + MineArmDelay` the first tick the owner is off the cell; once armed, forces `Visible=false` and clears `CellMine` from the board.

**Tests added (`engine/mine_explosion_test.go`):**

- `TestMine_NoExplodeOnLayer` — fresh mine + tank stands on it → no explosion, mine still active.
- `TestMine_ArmsAfterDelayOnceLayerLeaves` — owner steps off, simulate forward → ArmedTime set on first off-tick, mine armed after `MineArmDelay`.
- `TestMine_StaysDisarmedWhileLayerOnCell` — owner stays on for 5s → ArmedTime still 0, never armed.
- `TestMine_InvisibleAfterArming` — once armed, board cell is `CellEmpty`, `mine.Visible=false`.

**Tests updated:** `TestResolveTankMineCollision_*` (3 tests) and `TestApplyInput_MoveTriggersMineCheck` now pre-arm via explicit `ArmedTime` + `gc.SimTime`. `TestGolden_MineTankStepOn` now steps P1 off the mine cell and waits 130 ticks (>2s @ 60 FPS) before P2 walks onto it.

### 0.9.7 - 2026-04-17 — Phase 6 G-1: Protocol versioning on the wire

**Closes Phase 6 gap G-1.** External bot SDK consumers had no way to detect a protocol drift between their version and the live server — silent breakage was possible across server upgrades. This release stamps a `protocol_version` field on every server-issued `joined`/`rejoin_ack` payload and accepts an optional `client_protocol_version` on `join_room`/`rejoin` payloads.

**Policy:** mismatch is **logged, not rejected**. This is the forward-compatible advisory window. Strict rejection is reserved for a future MAJOR bump and will be announced one MINOR release in advance per the existing §10 policy in `docs/BOT_API.md`.

**Wire format:**

```json
// server → client
{"type":"joined","payload":{"room_code":"AB2D","player_id":1,"protocol_version":"1.0.0"}}

// bot → server
{"type":"rejoin","payload":{"room_code":"AB2D","player_id":2,"token":"...","player_name":"my-bot","client_protocol_version":"1.0.0"}}
```

**Implementation:**
- New `ProtocolVersion = "1.0.0"` const in `server/protocol.go` and `bot-sdk-go/protocol.go` (lock-step bump rule documented inline).
- `JoinedMsg`, `RejoinAckMsg` gained `ProtocolVersion string` (`omitempty`).
- `JoinRoomMsg`, `RejoinMsg` gained `ClientProtocolVersion string` (`omitempty`).
- All 4 `JoinedMsg{}` send sites in `ws_handler.go` (lines 201, 300, 343, 347) now stamp the version.
- `handleRejoin` and `handleJoinRoom` log `client=q server=q (accepting)` on mismatch, never reject.
- `bot-sdk-go/client.go` sends `ClientProtocolVersion: ProtocolVersion` automatically on every rejoin.

**Tests added (`server/protocol_test.go`):**
- `TestProtocolVersion_Constant` — non-empty invariant
- `TestJoinedMsg_StampsProtocolVersion` — server stamp round-trip
- `TestJoinRoomMsg_AcceptsClientProtocolVersion` — client field round-trip
- `TestRejoinMsg_AcceptsClientProtocolVersion` — client field round-trip
- `TestRejoinAckMsg_StampsProtocolVersion` — server stamp round-trip
- `setupStartedMatch` (server e2e helper) now asserts both `joined1.ProtocolVersion` and `joined2.ProtocolVersion` match `ProtocolVersion`, so every existing e2e test (~10+) implicitly verifies the live-server stamp.

**Verification:** full `go test -race ./... -count=1` green.

**Docs:** `docs/BOT_API.md` §10 expanded with "Wire-format version negotiation" subsection.

**Phase 6 status:** G-1 closed. Next: G-2 (bot-vs-bot E2E — `TestBot_VsBot_FullMatch` with `-race -count=10`).

### 0.9.6.1 - 2026-04-17 — Engine: spawn wall-lock fix (Phase 6 baseline cleanup)

**Pre-existing flake discovered while preparing Phase 6 G-1 work.** `TestE2EInputAffectsState` (server) and `TestClientInputAffectsState` (client) both intermittently failed (~22% of runs at difficulty 5) with "tank did not move from (X,Y) after trying all 8 directions". Stress test confirmed the failure rate; investigation traced it to a real engine bug, not a test-code bug.

**Root cause: spawn cells unprotected from random terrain.** `engine/board.go generateTerrain` reserved a single row at `y == 12` for `x ∈ [1..5]` and `[Width-6..Width-2]` — but tanks actually spawn at `(2, 10)` and `(BoardWidth-3, 10)` per `engine/game.go InitRound`. With ~14% wall density at difficulty 5, ~25% at difficulty 9, the 8 neighbors of a spawn cell could all be CellWall by chance, leaving the tank fully surrounded at match start with no legal move.

**Fix:** added `SpawnX1, SpawnY1, SpawnX2, SpawnY2` constants in `engine/constants.go` (mirroring the literals already used in `InitRound`). `generateTerrain` now consults a new `isSpawnReserved(x, y)` helper that skips wall placement for the spawn cell and all 8 neighbors of each tank's start position. Existing `y == 12` strip protection is kept for backward compatibility with the original PET-style spawn corridor.

**Verification:**
- `TestE2EInputAffectsState`: 50/50 PASS post-fix (was 11/50 FAIL pre-fix)
- `TestClientInputAffectsState`: 50/50 PASS post-fix (was ~5/10 FAIL pre-fix)
- New `TestNewBoard_SpawnNeighborsClearAcrossSeeds` enumerates seeds 0..199 at difficulty 9, asserts no neighbor of any spawn is CellWall — locks in the regression boundary.
- Existing `TestGenerateTerrain_DensityScaling` still within ±40 tolerance (spawn protection removes ≤16 cells out of 722 interior).
- Full `go test -race ./... -count=1` green.

**Files changed:** `engine/constants.go`, `engine/board.go`, `engine/board_test.go`, `client/integration_test.go` (8-direction defensive harness retained for resilience against future density tuning), `README.md`, `CHANGELOG.md`.

**Phase 6 status:** baseline now reproducibly green. Next: G-1 (protocol versioning).

### 0.9.6 - 2026-04-17 — Bot AI: own-barrel LOS exception (fire-when-aligned fix)

**Playtest feedback on 0.9.5: thinking delay at lvl 1 "feels about right", but the bot still marches across the entire screen along the same row as the player without firing — only firing once it ends up directly above/below.** Match log showed bot at Y=9, foe at (10,9), `pend=idle`, `sh=6` unchanged across ~1200 ticks of perfect same-row alignment. The 0.9.5 fire-interrupt only addressed the "pending move blocks fire" failure mode; this is a deeper LOS bug that prevents fire from ever being eligible in the first place.

**Root cause: own barrel blocks own LOS.** `canFireWithLOS` walked cells strictly between bot and enemy via `bs.isOpen(x, y)`, which only returns true for `cellEmpty`. The bot's barrel is **always** one cell in its facing direction (engine `tank.BarrelPos() = tank.X + DirectionVectors[Dir]`). When the bot faces left toward an aligned enemy, its own barrel sits at `(myX-1, myY)` — directly in the LOS path on the very first loop iteration → `isOpen` returns false → LOS check fails forever, regardless of how clean the actual line of fire is. Same hazard at the enemy end: enemy's barrel facing the bot occupies `(enemyX+1, enemyY)` which is also in the strict-between range.

**Engine confirms barrel cells are NOT projectile-blocking.** `engine/game.go:160 ShotSpawnPosition` spawns the projectile **AT** the barrel cell `(tank.X+v[0], tank.Y+v[1])` — the projectile originates AT the bot's own barrel, so own barrel is never in the projectile's path. And `engine/game.go:275-283` treats a projectile striking the enemy barrel as a kill (`case CellBarrel1, CellBarrel2: BarrelHit → wreckage + tank kill`). Both barrel cells must therefore count as fire-through for LOS purposes.

**Fix:** `cmd/bot-go/ai.go` adds a `barrelPos(t *botsdk.TankInfo) (int, int)` helper mirroring engine `DirectionVectors`, plus an `openForLOS(x, y)` closure inside `canFireWithLOS` that returns true if the cell matches own barrel or enemy barrel, falling through to `bs.isOpen` otherwise. Both the row-aligned and column-aligned LOS loops now use `openForLOS` instead of `bs.isOpen`. Sentinel `(-1,-1)` for unknown direction safely never matches a real grid cell.

**Files changed:** `cmd/bot-go/ai.go`, `README.md`, `CHANGELOG.md`.

**Test coverage:** Existing `TestBot_RuntimeBotInputAffectsState` continues to pass (clean build, no regression in movement). Fire-when-aligned behavior is verified by manual playtest — the writePump regression guard does not exercise alignment-range LOS.

### 0.9.5 - 2026-04-17 — Bot AI: fire-interrupt + thinking delay + clean game-end

**Playtest feedback on 0.9.4: bot moves and tracks the player, but (1) never fires even on clean shots, (2) is too fast at low difficulty, (3) errors out at game end with `unknown message type: play_again_ack`.** Three independent fixes:

- **Fire-interrupt preempts pending move (root cause of "bot never fires").** In 0.9.4 the bot stayed locked in `pendingMove` for the full `MoveDelay` window — at difficulty 1 that's ~33 ticks per cell, far longer than the typical alignment window in which both tanks share a row or column. Fire-eligibility was only evaluated in `startNewAction`, which only runs in the 1-tick gap between pending=idle and the next `commitMove` — so alignment opportunities were systematically missed. Match logs confirmed `me=(18,8) foe=(9,8)` aligned at t1290 with `pend=move/down@1287`, no shots fired across 2880 ticks (`sh=6` unchanged). Fix: `Decide` now checks fire-eligibility **before** the pending-state guard. If a move is pending AND the bot has LOS AND `my.Dir` already points at the enemy AND shots remain, it clears the pending move and releases the held movement key. Next tick `startNewAction` fires.
- **Scaling thinking delay (1.0s @ lvl 1 → 0.0s @ lvl 10).** New `thinkingDelayTicks(difficulty)` returns `(10 - difficulty) * 6` ticks at 60 Hz: lvl 1 = 60 ticks (1.0s), lvl 5 = 30 ticks (0.5s), lvl 9 = 6 ticks (0.1s), lvl 10 = 0. Per user request "Delay both before action AND between move re-commits", the gate `if tick < bs.nextActionTick { return nil }` is applied at the top of both `commitMove` (after the held-key short-circuit but before pending creation) and `startNewAction`'s fire-down / mine-down paths. `nextActionTick = tick + thinkingDelayTicks(difficulty)` is set after every commit (move-down, fire-down, mine-down) but **not** after release-up events — pending-resolution traffic must flow at engine cadence to keep the closed-loop SM honest. Fire-interrupt is also ungated, so the bot can still react instantly to alignment windows even at low difficulty (the delay throttles initiative, not reflexes).
- **`play_again_ack` no-op in SDK.** `bot-sdk-go/client.go` returned `unknown message type: play_again_ack` when the server sent the post-game ack, killing the read loop with noisy EOF logs. Bots don't participate in rematch flow (room is torn down), so the SDK now handles `MsgTypePlayAgainAck` as a no-op with a justified comment explaining why the empty case must remain.

**Files changed:** `cmd/bot-go/ai.go`, `bot-sdk-go/client.go`, `bot-sdk-go/protocol.go`, `README.md`, `CHANGELOG.md`.

**Test coverage:** Existing `TestBot_RuntimeBotInputAffectsState` continues to pass (movement still works); thinking delay does not regress the difficulty-5 movement assertion (bot moves 37,10 → 26,11 in 7s, slower than 0.9.4's 24,10 due to the new 0.5s/cell delay, but well within the test's "must move" tolerance).

### 0.9.4 - 2026-04-17 — Bot AI: closed-loop SM + writePump fix + grid-aware movement

**Bug Fixes — bot connected and looked alive in 0.9.3 logs but never actually delivered input to the server, then once that was fixed, got stuck on its own barrel and oscillated.** Six independent bugs across the SDK and AI:

- **`writePump` re-enqueued instead of writing (root cause).** `bot-sdk-go/client.go` had a `writePump` goroutine that read from `sendCh` and then called `sendRaw(msg)` — which itself enqueues into `sendCh`. The result was an infinite re-enqueue loop on the same channel; **no bot input ever reached the websocket** in 0.9.3. The match logs showed the AI "deciding" but the server saw no `KeyDown`/`KeyUp` traffic. Fix: `writePump` now writes directly via `conn.Write(ctx, websocket.MessageText, msg)`, with `ErrSendBufferFull` returned from `sendRaw` when the buffered channel is full (non-blocking send so a slow socket can never block the AI tick).
- **Closed-loop state machine.** `cmd/bot-go/ai.go` `Decide` was previously open-loop: it emitted `{key, down}` every tick the bot wanted to advance, with no concept of whether the previous action had landed. Per the original PET semantics ("press/release key, wait to see their desired/reported position/status from the server change, and only choose a new action if the previous one was successful"), `Decide` now tracks a `pending` action (`pendingMove` / `pendingFire` / `pendingMine`) with the pre-action state snapshot (`beforeX/Y/Dir/Shots/Mines`) and an `emittedTick`. Each tick first calls `checkAck` against the latest `TankInfo` — only when the server-reported state actually changes (or the per-action timeout fires) does the bot pick a new action. `pendingMove` acks on **moved OR rotated**, since the engine treats a perpendicular keypress as a free in-place rotation (`engine/game.go:101-130 ProcessMovement`).
- **LOS-aware fire.** `canFireAligned` now walks the grid between bot and enemy via `hasLOS`, refusing to fire through walls / barrels / mines / wreckage. Combined with strict same-row / same-column alignment, this stops the bot wasting ammo at unreachable targets.
- **Face-before-fire.** When LOS is clear but `my.Dir != desiredFireDir`, the bot emits a single rotation tap (a perpendicular keypress that the engine resolves as in-place rotation, no step) **before** firing. This eliminates "fire in wrong direction" wasted shots that 0.9.3 exhibited.
- **Grid-aware movement with own-barrel exception.** `moveTowardEnemy` now checks the destination cell via `isWall` and falls back to a perpendicular axis when blocked. Critical engine fact: **own barrel never blocks own tank movement** — the engine vacates the barrel cell on rotation before stepping. The previous draft of `isWall` treated the own-barrel cell as solid, so the bot was permanently stuck at spawn (37,10) with `CellBarrel2=6` at (36,10). Fix: `isWall` skips `cellBarrel1` if `MyID==1`, `cellBarrel2` if `MyID==2`. After the fix the runtime test shows steady ~1.85 cells/sec at difficulty 5 (37,10 → 24,10 in 7s).
- **2-tick desired-direction hysteresis + `commitMove` chokepoint.** Without hysteresis the bot flip-flopped left/right when the enemy crossed its row/column boundary, generating new `pending` entries that timed out. `desiredStableTicks < 2` now blocks committing a new direction unless that key is already held. The `commitMove(tick, my, key)` chokepoint also early-returns if `bs.heldMove == key`, preventing the previous bug where re-emitting the same `{key, down}` every tick reset `pending.beforeX/Dir` and the ack window never fired.

**Diagnostics:**
- `cmd/bot-go/main.go` tracks `tickCount` and `droppedInputs` (incremented on `ErrSendBufferFull` from `sendRaw`), logged on disconnect.
- `bot-sdk-go/client.go` exports `ErrSendBufferFull` for SDK consumers.

**Test coverage:**
- New `server/bot_runtime_test.go` `TestBot_RuntimeBotInputAffectsState` spawns the real `bin/tank-bot` subprocess against an in-process server and asserts that the bot's `TankInfo.X/Y` actually changes. This is the missing integration test that would have caught both the `writePump` bug in 0.9.3 and the own-barrel-stuck bug introduced mid-0.9.4. Set `TANK_BOT_BIN=/path/to/tank-bot` to point at a custom binary.

**Files changed:** `bot-sdk-go/client.go`, `cmd/bot-go/ai.go`, `cmd/bot-go/main.go`, `server/bot_runtime_test.go` (new), `README.md`, `CHANGELOG.md`.

### 0.9.3 - 2026-04-17 — Bot AI: single-key-per-tick + cell-enum + alignment fixes

**Bug Fixes — bot connected, fired harmlessly, never moved, lost 0-3 every match.** Three independent input-contract bugs in `cmd/bot-go/ai.go`:

- **Cell enum was 1-indexed but `isWall` treated 0 as empty.** `engine/constants.go` declares `CellEmpty CellType = iota + 1`, so empty floor is `1`, not `0`. The bot's `isWall` used `grid[ny][nx] > 0` which marked **every** cell (including empty floor) as a wall, so `moveTowardEnemy` always concluded it was boxed in and fell through to a no-op. Fix: `const cellEmpty = 1` and `bs.grid[ny][nx] != cellEmpty`.
- **Server `InputTracker.ConsumeAction` is one-shot destructive.** It returns the pending action then clears it to `nil` (`server/input.go:44-48`); the `held` map written by `KeyDown`/`KeyUp` is never read. The match loop calls `ConsumeAction` once per 60 Hz tick → one `ApplyInput` → one cell move (gated by `MoveDelay ≈ 0.244s` at difficulty 5). The previous bot pressed a movement key once and waited for an "up" event that the server didn't need — so movement stopped after one cell. Fix: re-emit `{key: dir, action: "down"}` **every** tick the bot wants to advance; never send movement `up` events.
- **Single-key-per-tick rule (original PET semantics).** The original Commodore PET TANK! had keyboard repeat off and accepted only one key at a time. The bot was emitting `[{down down} {fire down} {fire up}]` simultaneously, violating this. Fix: `Decide` now returns exactly one logical input per tick — fire (if strictly same-row or same-column with shots remaining) > drop mine (if dist<3, p=0.05) > move toward enemy (re-pressed every tick).
- **`canFireAligned` was too loose.** Original `abs(rowDiff) <= 1 || abs(colDiff) <= 1` triggered fire on diagonals, wasting shots. Tightened to strict `my.Y == enemy.Y || my.X == enemy.X`.

**Code cleanup:** Removed `currentMoveKey` field and `releaseMoveIfAny` method (no longer needed under the re-emit-every-tick model). Stripped verbose first-5-tick / 60Hz-sampled diagnostic logging from `cmd/bot-go/main.go` now that the protocol is correct.

### 0.9.2 - 2026-04-17 — VS AI hotfixes (subprocess bots + SSE race)

**Bug Fixes:**
- **VS AI never started match (SSE race)**: In VS AI mode, both seats fill synchronously inside `POST /api/room` (human via `AddPlayer`, bot via `AssignBotToRoom` → `AddBotPlayer`). The browser's SSE stream opened *after* the room was already full, so the edge-triggered `currCount > lastPlayerCount` check in `handleRoomEvents` never fired and the client sat on "WAITING FOR CPU OPPONENT...". Fix: emit `bot_joined` (or `player_joined`) immediately at SSE stream open whenever the room is already full at connect time. (`server/room_api.go`)
- **Bot connected but never moved**: The previous in-process `runBotClient` only sent `rejoin` and then blocked on `<-done` with no AI loop, so the bot was a sitting duck. Fix: replaced with subprocess model (see below).
- **`-addr :8080` form rejected by bot URL builder**: `cmd/server/main.go` only prepended `localhost` when the addr lacked a `:`, but `:8080` contains `:` already. Now also prepends when the addr starts with `:`.

**Architectural Change — Bot Subprocess Model:**
- `server/bot_manager.go` rewritten: `AssignBotToRoom` now spawns `bin/tank-bot` as an OS subprocess via `exec.Command` instead of dialing the WebSocket in-process. The bot binary already contains the full Stage A AI from `cmd/bot-go` + `bot-sdk-go`, so the in-server bot now actually plays.
- 3-tier bot binary resolution: (1) `TANK_BOT_BIN` env override, (2) same directory as the server executable (`os.Executable()`), (3) `./bin/tank-bot` fallback. Windows `.exe` suffix handled.
- `ReleaseBot` kills the subprocess if it's still alive; a background goroutine waits on `cmd.Wait()` to release the seat when the bot exits on its own.
- Bot stdout/stderr piped to server stdout/stderr for visibility.
- `HealthSnapshot()` now reports the resolved `bot_binary` path.
- Removes the `coder/websocket` import from `server/bot_manager.go` (no longer dials WS).

**Runtime Requirement:**
- When `TANK_ENABLE_BOTS=1`, the server now requires `bin/tank-bot` (or `bin\tank-bot.exe` on Windows) to exist on disk. `make bot` / `build.bat → bot` builds it. Override the path with `TANK_BOT_BIN=/path/to/tank-bot`.

### 0.9.1 - 2026-04-17 — Windows build.bat + bot build target

- **`build.bat`**: Windows batch script mirroring Makefile targets — `wasm`, `server`, `bot`, `build-all`, `dev`, `test`, `test-headless`, `test-e2e`, `test-all`, `clean`. Interactive menu for convenience.
- **`bot` build target**: Added to both Makefile (`bin/tank-bot`) and build.bat (`bin\tank-bot.exe`). Included in `build-all`.
- **`.gitignore`**: Added `bot-go` / `bot-go.exe` to prevent committed binaries.
- **Cleanup**: Removed accidentally committed `bot-go` binary (9MB) from repo root.
- **Fix**: `build.bat` subroutines now use `exit /b` instead of `goto menu` so `build-all` actually chains all three builds (previously `goto menu` inside a `call` prevented return).
- **Fix**: Both Makefile and build.bat now `mkdir bin` before writing executables into it (`go build` does not auto-create the output directory).

### 0.9.0 - 2026-04-17 — Wave 9: Bot API & VS AI Mode

**New:**
- **Bot API**: Third-party bots connect via the same WebSocket protocol as human players. Full protocol documented in `docs/BOT_API.md` — envelope format, message types, timing semantics, error codes, golden message fixtures, versioning policy, and Go SDK quickstart.
- **VS AI Mode**: "VS AI" button in lobby creates a room with an immediate CPU opponent. Server assigns a bot via `BotManager` which dials the local WS endpoint as a regular client. SSE emits `bot_joined` event when a bot joins.
- **Auto-fill Bot**: Create a room with `auto_fill_bot=true` and `auto_fill_after_sec=N` — if no human joins within N seconds, a bot automatically fills seat 2. Human joining before the timer cancels the auto-fill.
- **Go Bot SDK** (`bot-sdk-go/`): Standalone client package with WS dial, envelope helpers, reconnect policy (exponential backoff + jitter), typed callbacks for all message types, input sender with edge semantics, and `play_again` lifecycle support.
- **Reference Bot** (`cmd/bot-go/`): Stage A MVP AI — fires when aligned, moves toward enemy with obstacle avoidance, close-range evasion and mines, fallback random movement. Auto-creates room via HTTP API or joins existing room. Handles `game_over` (sends `play_again`) and `opponent_left` (graceful exit).
- **Bot Manager** (`server/bot_manager.go`): Orchestrates bot connections — `AssignBotToRoom`, `ReleaseBot`, `HealthSnapshot`. Feature-flagged via `TANK_ENABLE_BOTS=1`. Panic recovery in bot goroutines prevents server crashes.
- **Input Flood Guard** (`server/input_guard.go`): Rate limiter caps input messages at 120/sec per connection (2× tick rate). Excess inputs are silently dropped.

**Server Changes:**
- `server/room.go`: `Player` gains `IsBot`, `BotID`, `BotClass` fields. `Room` gains `AllowBot`, `AutoFillBot`, `AutoFillAfterSec`, `BotSeatReserved`, auto-fill timer. New methods: `ReserveBotSeat()`, `AddBotPlayer()`, `CancelBotReservation()`, `SetAutoFillTimer()`, `NewRoomWithBotPolicy()`.
- `server/protocol.go`: `CreateRoomMsg` gains `VsAI`, `AutoFillBot`, `AutoFillAfterSec`. `JoinedMsg` and `RejoinAckMsg` gain `IsBot`, `BotClass` (omitempty).
- `server/room_api.go`: Creates rooms with bot policy, assigns bots for VS AI, sets auto-fill timer, cancels timer on human join, SSE `bot_joined` event, room status includes `is_bot`/`bot_class` for bot players and `allow_bot`/`auto_fill_bot`/`auto_fill_after_sec` policy fields.
- `server/room_api.go`: New `SetBotManager()` for dependency injection of `BotManager` into `RoomAPI`.
- `server/hub.go`: `CreateRoomWithBotPolicy()` constructor.
- `server/ws_handler.go`: `InputGuard` on `ClientConn`, checked in `handleInput()`.
- `cmd/server/main.go`: Creates `BotManager`, wires into `RoomAPI`, logs feature flag status.

**Lobby Changes:**
- `web/index.html`: "VS AI" button with amber PETSCII styling, "AI MODE" label under difficulty slider.
- `web/app.js`: `vsAI` state, VS AI button handler, `bot_joined` and `auto_fill_started` SSE event handlers, "WAITING FOR CPU OPPONENT..." waiting message.

**Tests:**
- 7 new unit tests for bot identity, room policy, seat claiming, auto-fill, removal, and status API (`server/room_test.go`).
- 5 new integration tests for VS AI room creation, bot join, disconnect handling, room status with bot info, and auto-fill timer cancellation (`server/bot_e2e_test.go`).
- 5 new `BotManager` tests for enabled/disabled state, room assignment, release, and health (`server/bot_manager_test.go`).
- 4 new `InputGuard` tests for rate limiting, window reset, and concurrency (`server/input_guard_test.go`).
- 3 SDK tests for message wrapping, unwrap, and reconnect policy (`bot-sdk-go/client_test.go`).

### 0.8.0 - 2026-04-15 — Wave 8: HTMX Lobby & Focus Fixes

**New:**
- **HTML Lobby Page:** Replaced WASM-only lobby with HTMX-based HTML page (`web/index.html`). Room creation, joining, and status display all work server-side via HTTP API.
- **ESC Confirmation Dialog:** Pressing ESC during gameplay shows a confirmation dialog: "LEAVE GAME? ESC TO CONFIRM / ANY KEY TO STAY". Second ESC confirms exit. Any other key dismisses.
- **Debug Status Bar:** On-screen status bar (`PHASE: PLAY CONN: YES KEYS: 42 LAST: right TICK: 1234`) shows game state, connection status, key count, last key, and anim tick for debugging.
- **Focus Handling:** Added `canvas.focus()` before WASM init, visibilitychange handler to restore focus, and "CLICK HERE TO PLAY" button for keyboard recovery.

**Bug Fixes:**
- **Circular Dependency (Match Start):** WASM client only sent `ready` after receiving `game_start`, but server only sent `game_start` after receiving `ready` — deadlock. Match now auto-starts when both players join.
- **Difficulty Slider on Waiting Screen:** Difficulty selector moved to lobby screen. Waiting screen shows static difficulty (e.g., "DIFFICULTY: 5").
- **SSE WriteTimeout Too Short:** Increased from 60s to 300s to accommodate SSE connections lasting up to 5 minutes.
- **Barrel Direction Flash:** `keyToDir()` now returns correct Direction constants instead of magic numbers. Fixed barrel prediction to clear on direction key releases only (not fire/mine).
- **Room Code Copy Button:** Added `document.execCommand('copy')` fallback for non-HTTPS contexts.
- **Window Focus:** Added `SetRunnableOnUnfocused(true)` and focus handlers to restore input after tab switch.
- **Mineral Visibility:** Mines now go invisible 2s after owner tank moves away, visibility timer resets when tank steps back onto mine, cell becomes `EMPTY` when invisible (so shots can destroy it).
- **Barrel Wreckage Rendering:** Barrel-hit cells now render with directional curl glyphs (`/` or `\`) matching PETSCII `CURL_UP`/`CURL_DOWN`. Body-hit cells render inverted X. Dice wreckage renders dot.
- **Game Over ESC:** Added `PhaseGameOver` to ESC handler — redirects to lobby.

**Breaking Changes:**
- **Lobby Removed:** Removed `PhaseConnecting`, `PhaseLobby`, `LobbyMode*` state. Game starts immediately after WebSocket connection. WASM client simplified (~200 lines).
- **Input Changes:** Removed `PollHeldDirections()` — each keypress = one movement (no continuous movement while holding a key).
- **Network Phase:** Removed explicit `ready` message handling. Match starts automatically on both-players-join.

### Wave 7.1: Auto-Start Bug Fix & Headless E2E (April 14, 2026)

Server
- **Bug Fix: Auto-start match when both players join.** The server previously required explicit `ready` messages from both clients before sending `game_start`, but the WASM client only sent `ready` after receiving `game_start` — creating a deadlock where both screens showed "ROOM: XXXX - VS Player" indefinitely. Now the match starts automatically when P2 joins, making the `ready` handshake optional/idempotent.
- **`startMatchForRoom()`**: Extracted shared match-start logic from `handleReady` and `handlePlayAgain` into a reusable method. Both join and rematch paths use it.
- **`BothPlayersConnected()`**: New `Room` method to check if both player slots are filled with connected players.
- **`handleReady` is idempotent**: If a match already started (room state `RoomPlaying`), `handleReady` silently returns — no error, no duplicate match.

Tests
- **`TestAutoStartOnJoin`**: Headless E2E test verifying that create room → join room → match auto-starts without `ready` messages. Validates game state, grid dimensions, player IDs, and tick reception.
- **`TestInputAffectsTankPosition`**: Headless E2E test that creates a match, sends movement input, and verifies the tank position changes in tick messages.

### Wave 7: Testing & E2E Infrastructure (April 14, 2026)

Client
- **JS State Bridge**: `window.getGameState()`, `window.sendInput()`, `window.sendReady()`, `window.sendPlayAgain()` — JavaScript bridge functions for external testing tools (Playwright). Only active when `?test=1` URL parameter is present.
- **Test Mode**: `Game.testMode` field set from `?test=1` URL parameter. Enables JS bridge functions and periodic state export every 60 frames.
- **Build Tags**: `bridge_js.go` (`//go:build js`) for WASM, `bridge_native.go` (`//go:build !js`) for native — conditional compilation for JS interop.
- **Native Stubs**: `getJSTestMode()` returns `false` on native builds, truthy on WASM from URL params.
- **AnimTick**: Now correctly increments every `Update()` frame (was declared but never incremented).

E2E Tests
- **Playwright Infrastructure**: `test/e2e/` directory with Playwright config, TypeScript, and 7 test scenarios:
  - Page loads and WASM initializes
  - Create room flow
  - Difficulty selection
  - Test mode bridge functions exist with correct fields
  - Join room input accepts letters
  - Without `?test=1`, bridge functions are absent (security check)

Makefile
- **`test`**: `go test -race ./... -count=1` (all Go tests)
- **`test-headless`**: Subset of headless-safe Go tests (no display context required)
- **`test-e2e`**: Builds all, then runs Playwright in Chromium
- **`test-all`**: Runs all three test suites

### Wave 6: Hardening & Production Readiness (April 15, 2026)

Server
- **Error Handling**: Replaced 13 discarded send errors with `sendOrLog()` helper for proper logging
- **CORS Middleware**: Added configurable CORS headers for browser WASM clients (`-cors` flag, default `*`)
- **Input Validation**: Server validates difficulty (1-10), player name (1-16 chars, printable ASCII), room codes (4 chars, A-Z2-9)
- **Graceful Shutdown**: Server notifies clients on SIGTERM, drains running matches (30s timeout), then stops HTTP (15s)
- **Room Codes**: Unambiguous alphabet — excludes I, L, O, 0, 1 to prevent player confusion
- **WASM Cache-Busting**: No-cache headers for `.wasm` and `.js` files prevent stale binaries
- **CLI Flags**: Added `-cors`, `-max-room-age`, `-write-timeout` flags to server

Client
- **Crash Resilience**: Panic recovery in message handler, unknown message type logging, nil-cancel guard
- **Grid Dimension Guards**: `ApplyTick` and `ApplyGameStart` validate grid dimensions before applying
- **Dimension Mismatch Logging**: Tick/delta messages with wrong grid size are logged and skipped

Testing
- **Validation Tests**: `server/validate_test.go` — difficulty, player name, room code validation
- **Middleware Tests**: `server/middleware_test.go` — CORS and WASM no-cache headers
- **Hub Shutdown Tests**: `server/hub_test.go` — graceful shutdown with client notification
- **Client Tests**: `client/gamestate_test.go` — delta application, desync, explosions, bounds checking
- **Client Network Tests**: `client/network_test.go` — message wrapping, unwrapping, close edge cases
- **Smoke Tests**: `server/e2e_smoke_test.go` — full lifecycle, input validation, room codes, CORS, cache headers

Documentation
- **README.md**: Quick start, project structure, server flags, client URL params, controls, testing
- **docs/DEPLOY.md**: VPS deployment, Nginx config, TLS setup, production tips

### 2.0.0-alpha.3 - 2026-04-13 — Wave 3: WASM Game Client

Go+WebAssembly game client using Ebitengine v2. Two WS clients can play a full match through the server in a browser.

- **Protocol client (`client/protocol.go`):** JSON envelope with type discriminator, 12 message types (5 client→server, 7 server→client), wire entity state types (TankState, ShotState, MineState, ExplosionState), WrapMessage/UnwrapMessage helpers, KeyToAction mapper, error code constants. Round-trip JSON tests.
- **Network layer (`client/network.go`):** WebSocket client using `coder/websocket` v1.8.12, read/write goroutines with buffered channels (256 msgs), non-blocking Send with drop semantics, DrainIncoming for game loop integration, 1MB read limit for game state messages, graceful Close.
- **Game state (`client/gamestate.go`):** GamePhase state machine (Connecting→Lobby→Playing→RoundOver→GameOver→Disconnected), Apply* methods for all server messages, explosion timer management with per-frame 1/60s decrement, LobbyMode sub-states (Start→Creating/Joining→Waiting).
- **Input handler (`client/input.go`):** Ebitengine inpututil-based KEYDOWN/KEYUP edge detection, key→protocol mapping (arrows, space=fire, M=mine), tick counter for InputMsg synchronization, enabled/disabled by game phase.
- **Renderer (`client/renderer.go`, `client/glyph.go`):** 4-pass rendering (grid cells→tank barrels→mines→shots), direction-aware barrel glyphs (8 directions), PhosphorGreen/ChainGreen/Black palette, go:embed PetMe64.ttf font, text/v2 for HUD rendering (inverted text on green bars), lobby/connecting/game-over scene drawing.
- **Glyph cache (`client/glyph.go`):** Pre-rendered glyph images (cell type→rune mapping), mutex-protected cache, inverted text support (green background + black glyph for tank/mine characters), barrel direction rune lookup.
- **Game loop (`client/game.go`):** Ebitengine Game interface (Update/Draw/Layout), message dispatch by type, lobby keyboard handling (C=create, J=join, digits for room code, Enter=confirm), game-over→Enter→lobby flow, ESC=quit.
- **Scaffold (`cmd/client/main.go`, `web/index.html`, `Makefile`):** Native dev target (`make dev`), WASM build target (`make wasm`), HTTP serve target (`make serve`), go:embed font asset pipeline.
- **Integration tests (`client/integration_test.go`):** 5 E2E tests — connect+create room, join room, ready+game_start, input affects state, disconnect handling. All using `httptest` + real WebSocket server.
- **Total: 2935 lines of Go, 64 client tests, 158 engine tests, 58 server tests = 280 tests all passing with `-race`.** WASM binary builds clean.

### 2.0.0-alpha.2 - 2026-04-13 — Wave 2: WebSocket Protocol & Authoritative Server

Go WebSocket server for networked 2P play. Full protocol, room lifecycle, 60Hz authoritative game loop, and E2E integration tests.

- **Protocol (`server/protocol.go`):** JSON envelope with type discriminator, 12 message types (5 client→server, 7 server→client), wire entity state types (TankState, ShotState, MineState, ExplosionState), engine→wire converters, KeyToAction mapper, error code constants. Round-trip JSON tests.
- **Room (`server/room.go`):** Room struct with mutex-protected state machine (Waiting→Ready→Playing→GameOver→Closed), player slot management (ID 1/2), ready tracking, disconnect=forfeit semantics.
- **Hub (`server/hub.go`):** Hub with crypto/rand 4-letter room code generation, concurrent room lookup/creation, stale room cleanup (Closed rooms and abandoned Waiting rooms).
- **Input (`server/input.go`):** InputTracker with KEYDOWN/KEYUP edge detection, repeat suppression, FIFO pending queue, held-key fallback (directions only — fire/mine are edge-triggered), ProcessInputMsg wire→engine bridge.
- **MatchController (`server/match.go`):** 60Hz game loop (fixed dt=1/60s), goroutine-safe Start/Stop (sync.Once), round-over detection via engine.Winner, game-over detection via tank.Lives==0, inter-round pause (2s), engine state→TickMsg snapshot broadcast, MatchEvent callback interface for decoupled notification.
- **WebSocket handler (`server/ws_handler.go`):** HTTP→WS upgrade, ClientConn with read/write pumps, message dispatch (create_room, join_room, ready, input, play_again), RoomBridge implementing MatchEvent for dual-client broadcast, ConnRegistry for per-room player→connection mapping, idempotent disconnect cleanup.
- **Server binary (`cmd/server/main.go`):** Graceful shutdown (SIGINT/SIGTERM), periodic stale room cleanup (60s interval, 30min max age), health endpoint, configurable listen address.
- **E2E tests (`server/e2e_test.go`):** Full lifecycle (create→join→ready→ticks), forced disconnect (room teardown), input affects state (position change verification), multiple rooms (isolation), room cleanup after disconnect.
- **58 server tests** total (protocol: 8, room: 9, hub: 7, input: 8, match: 9, ws_handler: 8, e2e: 5, room bridge: 2). All passing with `-race` flag.

### 2.0.0-alpha.1 - 2026-04-10 — Wave 1: Go Game Engine

Pure Go port of the Python game engine (`engine/` package). All game mechanics faithfully ported from the Python 2P codebase with 158 tests, zero float positions (pure integer grid).

- **10 Go source files** in `engine/`: `doc.go`, `constants.go`, `board.go`, `player.go`, `projectile.go`, `game.go`, `shotshot.go`, `mine_explosion.go`, `match_runner.go`, plus test files.
- **Constants & enums:** `CellType` (1-10), `Direction` (1-8), `GameState`, `Action` (0-10), timing constants — all matching Python `IntEnum` values.
- **Board:** 40×21 grid, difficulty-scaled terrain generation (4%–25% wall density), deterministic seeded RNG.
- **Tank:** 8-direction movement, barrel tracking, lives/ammo/mines resource management, spawn position memory.
- **Projectile:** Shot movement with max range (75% of dimension), mine placement with visibility timer, detonation delay.
- **Collision system:** All 8 collision types ported — shot→wall, shot→tank body, shot→barrel, shot→wreckage, shot→mine, shot→shot (head-on swap detection), mine explosion chains, tank→mine.
- **Barrel swing:** Direction change without movement, old barrel cell cleanup.
- **Respawn:** Both-tank respawn after kill, spawn position scanning (up/down), resource reset per difficulty.
- **GameController:** Full update loop orchestrating movement, shots, mines, explosions, barrel hits, win detection, round/game state transitions.
- **Headless match runner:** `RunMatch()` with `InputProvider` interface, `RandomInputProvider` for simulation, `ScriptedInputProvider` for deterministic tests.
- **158 tests** across 9 test files, all passing. Race-free (`go test -race`), vet clean.
- **12 golden integration tests** covering all collision types with deterministic scripted scenarios.

### 1.0.0 - 2026-04-10 — TANK! 2P Fork

Two-player-only fork of the TANK! PyGame port. Stripped all AI, SHOWDOWN tournament, headless runner, Demo mode, and 1P mode.

- **Package renamed** to `tank-game-2p`.
- **Removed 9 AI source files:** `ai.py`, `ai_tree.py`, `ai_behaviours.py`, `ai_decorators.py`, `ai_navigation.py`, `ai_perception.py`, `ai_utility.py`, `ai_blackboard.py`, `showdown_worker.py`.
- **Removed `py_trees` dependency** — only `pygame` and `cbmcodecs2` required.
- **GameState reduced** from 16 to 8 values: `MENU`, `SKILL_SELECT`, `COUNTDOWN`, `PLAYING`, `ROUND_OVER`, `GAME_OVER`, `PLAY_AGAIN`, `QUIT`.
- **Simplified menu flow:** MENU → SKILL SELECT (1–10) → PLAYING → winner → ANOTHER BATTLE? Y/N → restart or MENU.
- **game.py** gutted from 1695 to ~878 lines (AI init, SHOWDOWN handlers, Demo logic, 1P mode removed).
- **game_loop.py** reduced from 344 to ~117 lines (headless batch runner, dashboard, parallel thread removed).
- **main.py** reduced from 121 to ~76 lines (py_trees check, headless FPS switching, render skip removed).
- **constants.py** reduced from 159 to ~137 lines (HEADLESS/SHOWDOWN constants removed).
- **Removed 9 AI/SHOWDOWN test files;** remaining tests cover board, player, projectile, and core game logic.

### 0.9.7 - 2026-04-07

- **Fix: deferred empty-gun self-destruct.** Firing the last shot no longer immediately kills the player. The self-destruct is deferred until all of that player's in-flight projectiles resolve. If the last shot kills the opponent and ends the round, the firing player wins instead of dying first.

### 0.9.6 - 2026-04-07

- **Quarter-block wreckage dots:** Dice-pattern wreckage from tank explosions now uses `▗` (U+2597, lower-right quadrant block) instead of a full solid block, making the dots visually proportional to one cell.
- **Projectiles destroy wreckage:** Shots now clear wreckage cells on contact (like walls/terrain), matching original PET behavior where wreckage is destructible.

### 0.9.5 - 2026-04-07

- **Wreckage now full-brightness:** Dice-dot wreckage from tank explosions now uses `COLOR_PET_FG` (same phosphor green as walls/tanks/borders) and renders with inverted fill (guaranteed full-cell coverage). Previously used a dimmer green that could be nearly invisible.

### 0.9.4 - 2026-04-07

- **Fix ghost barrel on direction swing:** Barrel swing (first keypress in a new direction) now properly clears the old barrel cell and places the new one. Previously, changing direction without moving left a phantom BARREL cell on the board that could be hit by incoming shots. Also fixed the same bug in AI barrel-direction changes (risky aim, risky restore, scan).

### 0.9.3 - 2026-04-07

- **HUD inverted text:** Top-row labels and values now render as black glyphs on a green bar (inverted/reverse-video), matching the original PET look. Previously rendered green-on-black.
- **Center separator narrowed:** Circle separator between player panels is 2 columns (was 4), matching the original PET layout.
- **Panel width corrected:** Each player panel is 19 columns (was 18). Fixes "MINES" being truncated to "MINE" and improves text spacing.
- **PET-authentic board dimensions:** Board resized from 40x25 to 40x21 to match the original PET playfield (rows 2-22 of the 25-row screen). Interior playfield is now 38x19. Window height reduced from 540px to 460px.

### 0.9.2 - 2026-04-07

- **Explosion wreckage now visible:** Dice-pattern wreckage from tank explosions now correctly overwrites WALL cells (matching original PET behavior where explosions destroy surrounding terrain). Previously walls blocked wreckage placement, leaving 0-2 of 4 dots at higher difficulties.
- **Wreckage color brightened:** Changed from dim `(20,100,20)` to `(0,180,0)` so wreckage is clearly visible against the black background.
- **Barrel-hit wreckage visual:** Barrel-shot tanks now leave a visible inverse-X body + curled barrel, matching the original PET wreckage appearance.

### 0.9.0 - 2026-04-07

Retro-accuracy corrections to match the original PET TANK! (Cursor #26) mechanics.

- **Projectile max range:** Shots travel at most 75% of the board dimension in their direction, then silently disappear (no explosion). Matches the original PET shot behavior.
- **Constant shot speed:** 0.1s per cell, no longer scales with difficulty.
- **Barrel shot mechanic:** Shots can now hit a tank's barrel cell. Barrel hits: no explosion, only the hit player loses a life and respawns (other player stays). Leaves curled-barrel wreckage.
- **Difficulty-scaled terrain:** Wall density scales from ~4% (level 1) to ~25% (level 10). Previously fixed at 12%.
- **Ammo per difficulty:** Level 1-2: 6, 3-5: 8, 6-8: 10, 9-10: 12 shots. Previously fixed at 18.
- **Mines per difficulty:** Level 1: 0, 2-4: 1, 5-7: 2, 8-10: 3 mines. Previously fixed at 3.
- **Tank speed calibration:** Base 0.3s/cell (matches PET ~12s screen-cross), scales to 0.2s/cell (150%) at difficulty 10. Previously 1.0s to 0.1s.
- **Multi-cell wreckage:** Destroyed tanks leave 4-dot "dice" pattern (corners of 3x3 area) as obstacles. Degrades gracefully near borders.
- **Per-player HUD messages:** Border row shows status messages: "LOW SHOTS" (<=20% ammo), "OUT OF SHOTS", "LAST TANK", "THE WINNER".
- **Barrel cells on board:** Tank barrels now register as BARREL1/BARREL2 cells on the board, enabling barrel-specific collision detection and AI pathfinding awareness.

### 0.8.0 - 2026-04-07

PETSCII retro UI overhaul — all rendering now uses authentic Commodore PET character glyphs.

- **Font:** Bundled **PetMe64.ttf** (KreativeKorp, free license) loaded via `petscii_render.get_pet_font()`; override with `TANK_PET_FONT=/path/to/font.ttf`. All text rendered with `antialias=False`.
- **PETSCII mapping:** New `tank_game/petscii_map.py` decodes PETSCII bytes through `cbmcodecs2` (`petscii_c64en_uc` codec) into Unicode glyphs the PET font renders. `PET_MAP` covers border, wall, tank body, barrel (H/V), mine, shot, explosion spokes, and wreckage.
- **Glyph cache:** `tank_game/petscii_render.py` — `glyph_surface()` (LRU-cached per char+color+size), `blit_cell()` / `blit_glyph()` helpers for the cell grid.
- **Playfield:** `graphics.py` rewritten — border uses `0x51` (ball) around the perimeter, walls use `0x66` (checkered block), tanks are composite body+barrel glyphs, mines use `0x71`, shots use `0x51`.
- **Explosions:** Multi-cell PETSCII composite — center circle with 8-way radial spokes (diagonal N/M line chars, cardinal H/V lines). Chain/catalyst explosions use a **larger** two-ring pattern.
- **HUD:** `StatusDisplay` redesigned to PET-authentic two-row layout: solid-block green bar (`0xA0`), `TANKS SHOTS MINES` labels and numeric values, circle (`0x51`) center separators.
- **Colors:** Monochrome **P1 phosphor green** `(51, 255, 51)` for all game elements (border, walls, tanks, shots, mines, text); dimmer green for wreckage.
- **CRT effect:** Optional `TANK_CRT=1` — scanline overlay + bloom (downscale/upscale via `smoothscale`). `tank_game/crt_effect.py`.
- **Dependency:** Added `cbmcodecs2>=1.0` to `pyproject.toml`.

### 0.7.1 - 2026-04-07

- **Fix: 1P AI speed scaling.** The 0.7.0 skill-table overhaul removed `fast_mode` / `user_difficulty` but forgot to pass `move_delay` to the 1P `AIPlayer` constructor. The AI received its raw SHOWDOWN-tuned `reaction_time` (as low as 0.020s) while the human was gated by the difficulty-based `_move_delay` (0.1s-1.0s), making the AI impossibly fast at every level. Now passes `move_delay=self._move_delay` so `get_ai_config` scales reaction time proportionally to game speed.

### 0.7.0 - 2026-04-07

AI skill ladder overhaul: every parameter for skill levels 0-9 is now **unique and strictly monotonic**, stored in an explicit `_SKILL_TABLE` lookup. No more tier bands, `// 3` banding, boolean thresholds, or shared presets.

**Config architecture:**
- **`_map_cells(fraction)`** helper converts fractions of `min(map_width, map_height)` to integer cells; all range/radius parameters auto-scale if the map resizes.
- **Expanded `AIConfig`:** new fields `peripheral_radius_pct`, `engage_range_pct`, `los_range_pct` (map-relative fractions), `risky_chance` (float 0-1 replacing boolean), `frustration_threshold`, `skip_fire_base`, `scan_when_blind`.
- **`_SKILL_TABLE`**: 10 hand-tuned `AIConfig` entries with 13 strictly monotonic parameters (verified by `scripts/verify_skill_table.py`).
- **`get_ai_config()`** indexes the table directly; `fast_mode` parameter eliminated.
- **`effective_min_shots_for_aimed`**: inverted relationship — higher confidence = fewer reserves needed (stops penalizing top-skill AI).

**Inline checks removed:**
- All `self.difficulty >= N` comparisons in `AIPlayer` methods replaced with config field lookups (`engage_range`, `los_range`, `risky_chance`, `scan_when_blind`, `frustration_threshold`, `skip_fire_base`).
- `aggressive_mines` changed from `bool` to `float` (0.0-1.0) for gradual mine-laying probability.
- `risky_shot_viable` uses `risky_chance` probability instead of a binary gate.
- `avoid_own_mines` confidence gate uses `scan_when_blind` flag.

**Navigation:**
- `NavigationConfig.sight_radius` and `greedy_distance` stored as `_pct` fractions, resolved to cells once in `NavigationMemory.__init__` via `_map_cells`.
- Explicit `_NAV_TABLE` with 10 unique entries (no `// 2` banding).

**Removed:** `fast_mode` parameter from `AIPlayer.__init__`, `get_ai_config`, and `game.py` `init_round`.

**Validation (5040-match matrix):** aggregate win rate 34.7% (AI-0) → 59.6% (AI-8), AI-9 beats AI-8 at 53.6% H2H. Zero timeouts.

### 0.6.6 - 2026-04-07

- **README:** Player-facing sections first; SHOWDOWN, headless/parallel runs, env vars, pytest/Ruff/mypy, and AI contributor notes moved under **REGRESSION TESTING**.

### 0.6.5 - 2026-04-07

- **README:** Rewrote for the current codebase — version line, features (two-player controls, SHOWDOWN matrix/headless/parallel), environment variable table, AI/testing pointers, and doc index. Removed outdated “scaffolding / incremental plan” status.

### 0.6.4 - 2026-04-07

- **Parallel SHOWDOWN worker timeouts:** Each match attempt is bounded by simulated time (`TANK_SHOWDOWN_MAX_SIM_S`, default **600 s**), update steps (`TANK_SHOWDOWN_MAX_STEPS`, default **2_000_000**), and wall clock (`TANK_SHOWDOWN_MAX_WALL_S`, default **180 s**). On timeout, a line is appended to **`tank_showdown_timeouts.log`**, the match is retried with a new RNG seed up to **`TANK_SHOWDOWN_MAX_RETRIES`** (default **5**). If all retries fail, the result is recorded with `timeout` / `timeout_after_retries` and the summary file lists **`TIMEOUT (reason)`** instead of a winner.
- **Stats fix:** `_finish_showdown` no longer credits player 2 when `winner == 0`. Head-to-head matrix only increments P1 wins when `winner == 1`.
- **Test:** `test_headless_dashboard_sim_time_whole_seconds_only` updated to match the current dashboard copy (`Sim rate (live):`).

### 0.6.3 - 2026-04-07

- **Critical fix: `TANK_DEBUG=0` was truthy.** `os.environ.get("TANK_DEBUG")` returns the string `"0"`, which Python evaluates as truthy (`bool("0") is True`). All four debug-log sites (`game.py`, `main.py`, `ai_tree.py`, `projectile.py`) now use a proper falsy-string check: `"0"`, `"false"`, `"no"`, `"off"`, and empty string all disable logging. This was producing a **150 MB** `tank_debug.log` with 24 worker processes all appending concurrently (garbled interleaved output visible in the file), causing massive I/O contention on top of the per-match summary file issue.

### 0.6.2 - 2026-04-07

Parallel SHOWDOWN worker throughput overhaul. With 24 workers on a Ryzen 9 5950X, measured throughput was **1.28 matches/sec** at only **7.5% CPU** — workers were spending ~90% of wall time blocked in kernel space by Windows Defender's minifilter scanning per-match summary files.

- **Suppress per-match file writes in workers:** Added `_suppress_summary_file` flag on `GameController`; set `True` in `_run_match`. Workers no longer call `open()` / write any files — results stay in RAM and the single summary file is written once by the main process at tournament end. This was the primary bottleneck (Defender processed ~1.3 file creates/sec across all 24 workers).
- **Coarser worker sim step:** Workers now use `WORKER_SIM_STEP_S = 1/60` (was `1/200`). Reduces ticks per match by ~3.3× while preserving game outcomes (all game logic is time-gated, not tick-gated). Workers no longer import `game_loop.HEADLESS_SIM_STEP_S`.
- **Lazy pygame in workers:** Removed top-level `import pygame` from `game.py` and `constants.py`. Key dicts (`PLAYER1_KEYS`, `PLAYER2_KEYS`) now use module `__getattr__` for deferred initialization. `sim_time_s()` and `handle_input()` import pygame locally (zero-cost in the main process where it's already cached in `sys.modules`). Worker processes no longer load SDL2 DLLs at all — verified: `'pygame' not in sys.modules` after a full worker match.
- **Fast `_debug_log`:** Module-level `_DEBUG = bool(os.environ.get("TANK_DEBUG"))` replaces per-call `os.environ.get`. Hottest call sites (`_run_ai`, `_update_shots`) guarded with `if _DEBUG:` to skip f-string evaluation entirely (~64 call sites, thousands of invocations per match).

### 0.6.1 - 2026-04-07

- **Fix:** Parallel SHOWDOWN dashboard now updates in real time. Changed `imap_unordered` to `chunksize=1` so each completed match immediately increments the progress counter (was `chunksize=25`, causing ~8 s of apparent stall before the first update). Main loop renders every frame at 60 FPS in parallel mode (was skipping 3/4 frames at 240 FPS — unnecessary since parallel tick is near-zero cost). Dashboard drops the meaningless "Sim rate (live)" line in parallel mode, showing only **matches/sec** and **ETA**.

### 0.6.0 - 2026-04-07

Headless SHOWDOWN throughput overhaul. cProfile revealed AI (A* pathfinding + peripheral vision) consumed **97%** of per-step CPU; the dashboard's **~3.5× sim/wall** was a misleading artifact of the sim-clock resetting every match boundary. Actual raw `update()` speed was already **~22×**. Changes:

- **IntEnum:** `CellType` and `Direction` changed from `Enum` to `IntEnum` — eliminates **2.6M** `enum.__hash__` calls per 5 k steps (12% of total CPU).
- **AI pre-gate:** `_run_ai` checks reaction-time **before** building args / calling `ai.update()` / entering py_trees — skips ~80% of AI calls without touching the tree.
- **A\* cache:** `NavigationMemory._astar_first_step` caches `(start, goal) → result`; repeated calls with same position return instantly. Cache invalidated on explosion / respawn / reset.
- **Inline `get_cell`:** Board bounds check inlined (was separate `in_bounds()` call on 1.72M invocations).
- **Cumulative sim counter:** New `_headless_sim_time_cumulative` that **never resets** at match boundaries. Dashboard rolling rate and benchmark now use it — no more negative/misleading readings.
- **Dashboard:** Shows **matches/sec** and **ETA** instead of the old per-match sim rate.
- **Render skip:** During headless SHOWDOWN, `render_frame` + `display.flip` run only every **4th** outer frame (dashboard text changes at most ~1 Hz).
- **Multiprocessing:** Set **`TANK_WORKERS=N`** (env var) before launching to run matches across **N** processes. On a 16-core Ryzen 9 5950X: **~25 matches/sec** (16 workers) vs **~3/s** (sequential). ETA for 50 k matches drops from **~5 hours to ~33 minutes**.
- **Bug fix:** `init_round()` no longer overwrites `difficulty_per_player` with fresh random values in SHOWDOWN mode — per-pairing skills from the schedule are now preserved.

### 0.5.28 - 2026-04-06

- **Offline benchmark:** Run **batch first**, then **tick_logic**, to reduce systematic bias when both run in one process (second scenario can read lower). Docstring and footer text updated for typical **~2.3–2.7×** dummy-SDL results vs dashboard. Ruff **I001** import order on ``scripts/benchmark_headless_showdown_offline.py``.

### 0.5.27 - 2026-04-06

- **Headless SHOWDOWN throughput:** The inner sim loop no longer calls ``pygame.event.pump()`` (and QUIT / ESC checks) **on every** ``update()`` step — that was dominating CPU and capping sim rate around a few× regardless of wall budget or main-loop FPS. Polling is throttled to every **`HEADLESS_INNER_POLL_INTERVAL_STEPS` (64)** steps (tunable in `tank_game/constants.py`), with one pump at batch start.
- **Vsync:** ``pygame.display.set_mode(..., vsync=…)`` — default **on**. Set environment variable **`TANK_VSYNC=0`** (or `false` / `no` / `off`) before launch to disable vsync on ``flip()`` if the display stack was still capping headless throughput near monitor refresh rate.

### 0.5.26 - 2026-04-06

- **Headless SHOWDOWN:** Main loop uses **`HEADLESS_SHOWDOWN_MAIN_LOOP_FPS` (240)** instead of **60** so `tick_logic` / headless batches run up to ~4× more often per wall second. Other modes still use **`MAIN_LOOP_FPS` (60)**. Constants in `tank_game/constants.py`.

### 0.5.25 - 2026-04-06

- **Headless dashboard:** Restored **Sim time Δ (last window):** — value is the **previous match’s** sim s / wall s (summary when advancing to the next match). **Sim time Δ (live):** unchanged (rolling ~1 s).

### 0.5.24 - 2026-04-06

- **Remove:** Headless **AutoTune** (A key, SET/RECENTER, related constants and state).
- **Headless SHOWDOWN:** Raised **`HEADLESS_MAX_STEPS_SAFETY`** from **100_000** to **10_000_000** per batch. The old cap often stopped the inner loop **before** the wall **CPU budget** was used, so 50–250 ms budgets could look identical; the budget is now much more likely to be the real limiter. Dashboard still shows **Sim time Δ (live)** (rolling ~1 s).

### 0.5.23 - 2026-04-06

- **Headless AutoTune:** SET starts at **50 ms** (aligned with default wall budget). Each measurement window is **10 s** wall time (was 5 s); **RECENTER** interval in HOLD is **30 s** (was 60 s). Dashboard: **Sim time Δ (live)** — rolling **~1 s** sim s / wall s; **Sim time Δ (last window)** — rate from the last completed AutoTune trial (SET or RECENTER). *(Removed in 0.5.24.)*

### 0.5.22 - 2026-04-06

- **Headless SHOWDOWN — AutoTune:** Press **A** on the dashboard to toggle. **SET:** start from **20 ms** wall budget, measure **sim rate** (Δ sim time / Δ wall time) over **5 s** windows, sweep **up** then **down** in **2 ms** steps to a local maximum, then **HOLD** at that budget. **RECENTER:** every **60 s**, try **+10%** budget; keep it only if sim rate improves, else revert. Constants in `tank_game/constants.py` (`HEADLESS_AUTOTUNE_*`). AutoTune stops when SHOWDOWN ends or is cancelled.

### 0.5.21 - 2026-04-06

- **Headless SHOWDOWN dashboard:** **Sim time (this match)** shows **whole seconds** only. `MessageOverlay` now **caches** rendered lines until the message text or surface size changes, so the overlay avoids per-frame `font.render` when the dashboard string is unchanged (typically for most frames within each simulated second).

### 0.5.20 - 2026-04-06

- **Headless SHOWDOWN:** Default **wall budget** raised from **12 ms** to **50 ms** (`HEADLESS_WALL_BUDGET_DEFAULT_S`) — better default throughput while keeping UI updates and input responsive; ↑/↓ still adjusts at runtime.

### 0.5.19 - 2026-04-06

- **Fix:** **ESC** now cancels **headless SHOWDOWN** reliably. `handle_input` only runs before each headless batch, so ESC pressed during a long batch was invisible until the batch finished. The headless batch loop now calls `pygame.event.pump()`, polls `pygame.key.get_pressed()[K_ESCAPE]`, and drains `QUIT` events each iteration. Added `GameController.abort_showdown_run()` (sets `SHOWDOWN_COMPLETE` and a short cancelled summary message for the overlay).

### 0.5.18 - 2026-04-06

- **Headless SHOWDOWN:** **↑ / ↓** on the status dashboard adjusts **CPU budget** (wall time per frame, shown in ms). Stored in `GameController._headless_wall_budget_s`; defaults and limits in `tank_game/constants.py` (`HEADLESS_WALL_BUDGET_*`).

### 0.5.17 - 2026-04-06

- **Fix:** Headless SHOWDOWN chunking now uses a **wall-clock budget** per frame (`HEADLESS_WALL_BUDGET_S`, default 12 ms) instead of a fixed step count. Heavy `update()` cost (e.g. **`TANK_DEBUG`** writing every line to disk) could still freeze the UI for seconds with a large step cap. Added `pygame.event.pump()` in the headless batch and at the start of `main`’s loop when headless + running so Windows can process messages.

### 0.5.16 - 2026-04-06

- **Fix:** Headless SHOWDOWN no longer runs up to 300k sim steps in a single `tick_logic` call (that blocked the main thread: **Not Responding**, no UI refresh). Headless work is **chunked** so each frame returns to `display.flip` / the event loop (superseded by 0.5.17 time-budget + pump).

### 0.5.15 - 2026-04-06

- **SHOWDOWN — Headless mode:** On any SHOWDOWN setup screen, **H** toggles **Headless** (default off). When on, the run uses a **simulation clock** (`sim_time_s()` in `GameController`) instead of wall time; each frame advances a **chunk** of sim steps (`HEADLESS_STEPS_PER_FRAME` × **1/200 s** per step) so matches finish quickly without freezing the window. The main window shows a **dashboard** (match index, pairing, progress bar, sim time) instead of the playfield. **ESC** still cancels. Result file notes `Headless: yes/no` and that durations are simulated seconds when headless. See `run_headless_showdown_batch` / `format_headless_showdown_dashboard` in `tank_game/game_loop.py`.

### 0.5.14 - 2026-04-06

- **SHOWDOWN:** After choosing mode **3 — SHOWDOWN**, pick **schedule**: **Random pairings** (original: set total match count) or **Full skill matrix** — every AI-i vs AI-j (i,j in 0..9), with **N consecutive rounds per pairing** (100×N matches total). Confirmation screen shows total matches; **ESC** steps back. Summary file notes schedule type; matrix runs add a **P1×P2 head-to-head** table (P1 wins / games and %). Tests: `tests/test_showdown_matrix.py`.

### 0.5.13 - 2026-04-06

- **Fix:** Head-on shots on the same row/column could **pass through** each other: with one grid step per tick, two bullets in adjacent cells swap positions without ever sharing a cell, so the old “same `(x,y)`” shot–shot check missed them. Added detection for cardinal **head-on swap** using pre-step positions (`_step_start_x` / `_step_start_y`). Tests: `tests/test_shot_shot_collision.py`.

### 0.5.12 - 2026-04-06

- **Fix:** `_respawn_both_tanks` now calls `clear_from_board` **before** updating `tank.x` / `tank.y` to spawn. The previous order cleared the spawn cell (usually empty) and left **stale `TANK1`/`TANK2` cells** at old positions — invisible “walls”, and shots spawning one cell ahead could hit a ghost tank (logged as instant `SHOT_HIT` / self-kill). Regression test: `tests/test_game_logic.py::test_respawn_clears_old_tank_cells_from_board`.

### 0.5.11 - 2026-04-06

- **FR-8 risky fire (human parity):** Wild shots no longer use `direction_override` (sprite facing vs bullet direction). **`FireRisky`** now uses a two-tick flow: **`RISKY_PREPARE`** turns the tank to a random 8-way direction, next tick **`FIRE`** commits along that facing, then **`GameController._run_ai`** restores the saved barrel direction. Docs: `MECHANICS_TANK_PYGAME.md`, `docs/PRD_py_trees_integration.md` §11.

### 0.5.10 - 2026-04-06

- **Tests:** `tests/test_headless_demo_stuck_watch.py` — headless Demo harness with fake `get_ticks`, per-frame position history, **STUCK** (long same-cell run) and **OSCILLATE** (ABAB sliding-window count) warnings; optional `TANK_HEADLESS_FAIL_ON_WARN=1`.

### 0.5.9 - 2026-04-06

- **Tight-space oscillation:** Record tank position on every reaction decision; detect **A↔B↔A↔B** ping-pong. New BT leaf **`OscillationUnstick`** (runs before **ActiveEvasion**) issues **`RANDOM_MOVE`**, clears evasion / rim commit, and resets the ring — fixes long LEFT↔RIGHT / UP↻DOWN loops when chase+evade never reached stuck checks.

### 0.5.8 - 2026-04-06

- **HUD:** Restore **AI:n** skill labels in the status bar for **SHOWDOWN** (and **1P** for the CPU) — `render_frame` had only wired `difficulty_per_player` for **Demo**.

### 0.5.7 - 2026-04-06

- **Evasion oscillation (rim):** When primary horizontal “away” hits the **left/right wall**, `pick_evade_direction` now **commits** to the first successful **UP/DOWN** for the evasion session so chaser **row changes** no longer flip vertical escape every tick (fixes N/S face-to-face ping-pong in `tank_debug.log`). Cleared when evasion ends.

### 0.5.6 - 2026-04-06

- **PRD AI navigation:** `tank_game/ai_navigation.py` — `NavigationMemory` (skill-linear config, tabu edges, wall belief, peripheral vision ingest, decay, respawn soften, explosion invalidation), hybrid A* + greedy `get_movement_toward_target`, `pick_evade_direction`, `GameController._ai_attempt_move` records failed attempts; `_respawn_both_tanks` / `_explode_mine` hooks. Tests: `tests/test_ai_navigation.py`. Docs: `docs/PRD_AI_NAVIGATION.md` §11.

### 0.5.5 - 2026-04-06

- **Docs:** Added **`docs/PRD_AI_NAVIGATION.md`** — PRD for phased AI navigation memory (tabu → grid + hybrid pathing), strict fairness, `attempt_move` as source of truth with optional fallback, respawn confidence decay vs full reset on new map/`init_round`. Cross-link from **`MECHANICS_TANK_PYGAME.md`**.

### 0.5.4 - 2026-04-06

- **Corner oscillation (Demo/SHOWDOWN):** When blind to the enemy, **`ScanOrAdvance`** throttles **SCAN** decisions (~0.28s) so tanks in outside corners do not barrel-spin every reaction tick. **`get_movement_toward_target`** prefers **cardinals before diagonal** when near the playable rim to reduce diagonal wall-hugging. Tests: `tests/test_ai_behaviours.py`, `tests/test_corner_headless.py` (monotonic `get_ticks`).

### 0.5.3 - 2026-04-06

- **Evasion stuck (Demo/SHOWDOWN):** `MOVE_AWAY_FROM_ENEMY` now calls **`_ai_try_random_escape_move`** when cardinal/diagonal “away” steps are blocked (previously only logged). If still trapped, **`_is_evading`** is cleared so other behaviours can run. Evade direction uses **`enemy_pos` or `last_known_enemy_pos`**. Headless smoke: `tests/test_evasion_fallback.py`.

### 0.5.2 - 2026-04-06

- **`tank_game.main`:** Check for `py_trees` before importing the game stack; print install instructions (`pip install -e .`) and exit cleanly when dependencies are missing.
- **README:** Emphasize installing the package from the repo root so runtime deps are present.

### 0.5.1 - 2026-04-06

- **PRD §11 closed:** Documented resolutions for reaction gate, blackboard, legacy env, `py_trees` pin, FR-9 (geometric 90° cone), FR-8 (skip-fire hesitation + existing risky jitter).
- **FR-10:** `effective_min_shots_for_aimed()` = `confidence_threshold + ammo_discipline // 3`; wired into `_can_fire_confidently` and `_can_fire_risky`.
- **FR-8:** `should_skip_fire()` — low skill may skip aimed or risky shot this tick; legacy path uses skip for aimed fire when LOS would allow a shot.

### 0.5.0 - 2026-04-06

- **PRD S1:** Reaction gating implemented as a **py_trees** `ReactionTimeGate` decorator (`tank_game/ai_decorators.py`) wrapping the main selector; evasion cooldown for too-close contact uses **`EternalGuard`** around `EnemyPresenceTooClose`. **ActiveEvasion** leaf replaces the pre-tree evasion early return. Staging via `_stage_for_tree_tick` / `ReactionTimeGate` applies bind, perception, and frustration once the gate opens.

### 0.4.0 - 2026-04-06

- **PRD P3:** `tank_game/ai_utility.py` — `utility_snapshot()` for optional balance/tuning analysis (not used by the behaviour tree).
- **PRD S2:** When `TANK_DEBUG` is set, append behaviour-tree **tip** (`tree.tip()` name + status) to `tank_debug.log` after each AI tick.
- **FR-5:** Documented round-reset behaviour in `MECHANICS_TANK_PYGAME.md`.
- **`player_id`** passed from `GameController._run_ai` into `AIPlayer.update` / `_bind_tick_inputs` (blackboard contract §5.1).

### 0.3.0 - 2026-04-06

- **PRD P2 / §5.5:** Perception module `tank_game/ai_perception.py` (90° combat cone, peripheral tracking). `_sync_combat_perception` updates `last_known` / `_combat_enemy_pos` before the behaviour tree.
- **FR-11:** Demo/SHOWDOWN AI uses **per-player** `fast_mode=(difficulty >= 8)` and **per-skill** `reaction_time` when `move_delay` is set (no shared flattening).
- **FR-7:** Skill 0 never dodges; dodge curve adjusted in `get_ai_config`.
- **FR-10:** `confidence_threshold` / `ammo_discipline` — higher skill requires more reserve ammo before a disciplined shot.
- **FR-9:** `ScanOrAdvance` uses `scan_frequency` + difficulty to bias scanning when blind; combat aims only use forward-cone visibility.
- **FR-8:** `FireRisky` behaviour + `AIPlayer._pending_fire_risky`; `GameController._fire_shot(..., direction_override=)` for wild shots.
- **`aggressive_mines`** increases mine chance during too-close evasion.
- Tests: `test_ai_perception.py`; AI behaviour tests updated for perception sync.

### 0.2.0 - 2026-04-06

- **py_trees AI (PRD P1):** Tank decisions run through a `Selector` behaviour tree (`tank_game/ai_tree.py`, `tank_game/ai_behaviours.py`) with per-tick context on `AIPlayer` (`_bind_tick_inputs`). `GameController._run_ai` unchanged in effect.
- Added `tank_game/ai_blackboard.py` (`TankAITickContext` for documentation).
- `TANK_LEGACY_AI=1` restores the previous inline decision function for debugging.
- New tests: `tests/test_ai_behaviours.py`.
- Runtime dependency: `py_trees>=2.2.0`.
- Docs: PRD status updated; README + `MECHANICS_TANK_PYGAME.md` note the tree.

### 0.1.3 - 2026-04-06

- Added `docs/PYGAME_QA_TESTING_SPEC.md` (headless SDL, Ruff/mypy/Vulture/Hypothesis/VizTracer, event injection, layered testing).
- Added `tank_game/game_loop.py` with `tick_logic` / `render_frame`; `main` composes one frame with injectable `dt` via the clock.
- Added `tests/conftest.py` and `tests/test_headless_smoke.py`.
- `pyproject.toml`: dev tooling (ruff, mypy, vulture, hypothesis, viztracer) and tool sections; minor typing fixes for mypy (`ai.py`, `projectile.Shot`, `game.py` debug line).

### 0.1.2 - 2026-04-06

- Extended `docs/PRD_py_trees_integration.md` (v0.2): developer feedback on skill differentiation — dodge curve, risky shots, FOV/scan, ammo discipline, per-skill think time (§5.5, P2+).

### 0.1.1 - 2026-04-06

- Added `docs/PRD_py_trees_integration.md`: product requirements for integrating py_trees as the tank AI decision layer (blackboard, parity phases, acceptance criteria).

### 0.1.0 - 2026-03-17

- Initial Python 3 / PyGame TANK! project scaffolding.
- Added mechanics reference document for the TANK! port.
- Defined packaging metadata and development dependencies.
