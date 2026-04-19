# Wave 8: HTMX Landing Page — Task Plan

## Completion Status (updated 2025-04-19)

| Task | Description | Status | Evidence |
|------|-------------|--------|----------|
| T1 | Server HTTP API | ✅ Complete | `server/room_api.go` (362 lines): `handleCreateRoom`, `handleJoinRoom`, `handleRoomStatus`, `handleRoomEvents`; `server/room_api_test.go` (129 lines): round-trip tests for create/join/status |
| T2 | WASM Rejoin Protocol | ✅ Complete | `server/protocol.go`: `MsgTypeRejoin = "rejoin"`, `RejoinMsg` struct; `server/ws_handler.go`: rejoin handling at line 151; `server/room_api.go`: token generation and validation; `client/bridge_js.go`: `getJSConfig()` reads `window.tankConfig` |
| T3 | HTML Landing Page | ✅ Complete | `web/index.html` (103 lines): PETSCII-styled lobby; `web/app.js` (279 lines): `createRoom()`, `joinRoom()`, `pollRoomStatus()`, SSE via `EventSource`; `web/style.css` (273 lines): green-on-black terminal aesthetic; `web/game.html` (84 lines): minimal WASM page |
| T4 | WASM Client Simplification | ✅ Complete | `client/gamestate.go`: `PhaseConnecting` and all `LobbyMode*` removed — only `PhasePlaying`, `PhaseRoundOver`, `PhaseGameOver`, `PhaseDisconnected` remain; `client/game.go`: no `handleLobbyInput`; `client/renderer.go`: no `drawLobbyText`; lobby phases completely removed |
| T5 | Server Push (SSE) | ✅ Complete | `server/room_api.go`: `handleRoomEvents` (line 254) serves `text/event-stream` with `player_joined`, `game_starting`, `room_expired` events; `web/app.js`: `EventSource` for real-time updates |
| T6 | Integration Testing | ✅ Complete | `server/room_api_test.go`, `server/bot_e2e_test.go`, `server/e2e_smoke_test.go`, `server/e2e_full_test.go`; `Makefile`: `test-e2e` target; `test/e2e/src/game.spec.ts` (104 lines): 8 Playwright E2E tests |

**All WAVE8 tasks are complete.** The lobby is HTML-first, WASM loads only for gameplay.

**Date**: April 14, 2026
**Branch**: NetTank
**Status**: Complete ✅
**PRD Reference**: PROJECT.NET/PRD_NETWORK_PLAY.md §3.1, §3.2, §5; PROJECT.NET/PRD_NETWORK_PLAY_V2.md §8

## Goal

Replace the WASM-based lobby UI with a fast, native HTML/HTMX landing page that handles all pre-game interactions (name entry, room creation, difficulty selection, room joining, waiting for opponent). The WASM client only loads when a match is ready to start, eliminating the canvas-focus bug entirely.

**Acceptance Criteria**:
1. Player opens `http://localhost:8080/` → sees PETSCII-styled HTML lobby (instant load, no WASM)
2. Name entry: real HTML text input with copy/paste, mobile keyboard support
3. Create room: HTML form with difficulty slider (1-10), server creates room via HTTP POST
4. Join room: HTML text input for room code (4 chars, copy/paste works)
5. Waiting screen: SSE or HTMX poll shows "waiting for opponent" with clickable room code
6. When both players are ready → page loads WASM client which immediately receives `game_start`
7. Game-over/rematch: HTML overlay or WASM handles it (WASM already has this)
8. ESC key works natively in HTML (no focus issues)
9. WASM client only loads after room is joined and match starts
10. All existing server tests pass; new E2E tests cover the HTTP→WS handoff

## Architecture

```
[ Browser ] ---HTTP---> [ Go Server ]
     |                       |
     |  1. GET /            |  → HTML lobby page (PETSCII styled)
     |  2. POST /api/room   |  → Create room, get room code
     |  3. GET /api/room/X  |  → Check room status (poll/SSE)
     |  4. GET /game?room=X&player=Y&pid=Z  →  WASM game page
     |                       |
     |  5. WebSocket /ws     |  → Game input/output (same as now)
     v                       v
  [ WASM Client ]       [ Go Server ]
```

### Key Insight: Two-Phase Connection

**Phase 1 (HTML)**: Player creates/joins a room via HTTP. Server returns room code + player ID.
**Phase 2 (WASM)**: Page navigates to `/game?room=XXXX&player=Y&pid=1&token=JWT`. WASM client connects to `/ws` with these params, server looks up the room and player, starts/rejoins the match.

The WASM client skips the lobby entirely — it connects, sends a "rejoin" message with room+player+token, and the server sends `game_start` immediately (or `joined` if still waiting).

## What Moves To HTML

| Currently in WASM | Moves to HTML/HTMX | Stays in WASM |
|---|---|---|
| Connecting screen | ✅ Landing page | — |
| Player name input | ✅ HTML text input | — |
| "C to Create, J to Join" | ✅ HTML buttons | — |
| Difficulty selection (1-9, 0) | ✅ HTML range/dropdown | — |
| Room code display | ✅ HTML with click-to-copy | — |
| Room code entry (A-Z, 2-9) | ✅ HTML text input (4 chars) | — |
| "Waiting for opponent..." | ✅ SSE/HTMX poll | — |
| Error messages | ✅ HTML alerts | — |
| ESC/back navigation | ✅ Browser back button | — |
| Game rendering (grid, tanks) | — | ✅ Ebitengine canvas |
| Game input (QWE/ASD/ZXC) | — | ✅ Canvas focus |
| Game-over overlay | — | ✅ WASM (already works) |
| Rematch prompt | — | ✅ WASM (already works) |
| Round-over pause | — | ✅ WASM (already works) |

## What Gets Removed From WASM Client

- `PhaseConnecting` (no more WASM-side connecting)
- `LobbyModeStart`, `LobbyModeDifficulty`, `LobbyModeCreating`, `LobbyModeJoining`, `LobbyModeWaiting` (no more WASM lobby)
- `handleLobbyInput()` in `game.go`
- `drawLobbyText()` in `renderer.go` (all lobby rendering)
- Difficulty bar rendering in `renderer.go`
- Room code input via `ebiten.KeyA`→letter conversion
- Player name via `?name=` URL param (now via HTML form)
- All the lobby ESC/C/J/D/Enter key handling
- `CreateRoomMsg` and `JoinRoomMsg` handling in WASM (now via HTTP)
- `ConnectAsync()` retry logic (now handled by HTML page)
- `?test=1` bridge functions for lobby testing (replaced by HTML E2E)

## What Gets Added

### Server-Side (Go)

1. **HTTP API endpoints** in `cmd/server/main.go`:
   - `POST /api/room` — create room, returns `{"room_code": "AB2D", "player_id": 1, "player_name": "Alice"}`
   - `POST /api/room/{code}/join` — join room, returns `{"room_code": "AB2D", "player_id": 2, "player_name": "Bob", "opponent_name": "Alice"}`
   - `GET /api/room/{code}/status` — poll room status, returns `{"status": "waiting"|"full"|"playing"|"expired", "difficulty": 5, "players": [...]}`
   - `GET /game` — serve the WASM game page with room params in URL or session cookie

2. **Room API handler** in `server/room_api.go`:
   - JSON request/response for room management
   - Validates input (difficulty 1-10, player name 1-16 chars, room code format)
   - Creates room in Hub, stores player connection token
   - Join room validates code exists and has space

3. **HTML template** in `web/templates/index.html`:
   - PETSCII-styled (PetMe font, green-on-black, monospace terminal aesthetic)
   - Name entry field
   - Create Room button + difficulty selector
   - Join Room field (4-char code with validation)
   - Waiting room with SSE status updates
   - HTMX for reactive updates without full page reloads

4. **Session token** system:
   - On room create/join, server generates a random token (UUID or crypto/rand)
   - Token stored in Hub alongside player connection
   - Token passed to WASM client via URL param `/game?room=AB2D&pid=1&token=xxx`
   - WASM client sends token on WS connect; server validates token → player identity

5. **Static asset serving** updates:
   - `PetMe.ttf` served at `/fonts/PetMe.ttf`
   - `htmx.min.js` served (or from CDN with integrity hash)
   - `sse.js` or inline SSE for waiting room updates

### Client-Side (WASM)

1. **Simplified `Game` struct**: Remove `LobbyMode*`, remove `handleLobbyInput()`, remove lobby phase handling
2. **New initialization**: `NewGame()` receives `roomCode`, `playerID`, `token`, `playerName`, `serverURL` from URL params
3. **New WS connect flow**: Connect to `/ws`, send `{"type":"rejoin","room_code":"AB2D","player_id":1,"token":"xxx"}` → server validates and sends `game_start` or puts player in waiting state
4. **Game phases reduced**: `PhasePlaying`, `PhaseRoundOver`, `PhaseGameOver`, `PhaseDisconnected` only. No `PhaseConnecting` or `PhaseLobby`.
5. **Rematch stays in WASM**: Game-over overlay with P/ESC is already working

## Task Breakdown

### T1: Server HTTP API — Room Creation & Joining
**Files**: `server/room_api.go`, `server/room_api_test.go`, `cmd/server/main.go`
**Depends on**: Existing `server/hub.go`, `server/room.go`

- Add `POST /api/room` endpoint: accepts `{"difficulty": 5, "player_name": "Alice"}`, returns `{"room_code": "AB2D", "player_id": 1, "token": "uuid"}`
- Add `POST /api/room/{code}/join` endpoint: accepts `{"player_name": "Bob"}`, returns `{"room_code": "AB2D", "player_id": 2, "token": "uuid", "opponent_name": "Alice"}`
- Add `GET /api/room/{code}/status` endpoint: returns room state (`waiting`, `full`, `playing`)
- Generate crypto-random tokens for player authentication
- Store tokens in `Hub` alongside room connections
- Input validation reuses `server/validate.go`
- Unit tests for all three endpoints

**Acceptance**:
```
curl -X POST localhost:8080/api/room -d '{"difficulty":5,"player_name":"Alice"}'
→ {"room_code":"AB2D","player_id":1,"token":"550e8400-e29b-41d4-a716-446655440000"}

curl -X POST localhost:8080/api/room/AB2D/join -d '{"player_name":"Bob"}'
→ {"room_code":"AB2D","player_id":2,"token":"660e8400-...","opponent_name":"Alice"}

curl localhost:8080/api/room/AB2D/status
→ {"status":"full","difficulty":5}
```

### T2: WASM Rejoin Protocol — Token-Based Auth
**Files**: `server/ws_handler.go`, `server/protocol.go`, `server/room_api.go`
**Depends on**: T1

- Add `MsgTypeRejoin = "rejoin"` message type
- Add `RejoinMsg{RoomCode, PlayerID, Token}` struct
- In `ws_handler.go::ServeHTTP`: when client connects, first message must be `rejoin` with valid token
- Server validates token against stored tokens in Hub, looks up room and player
- If valid: set `setRoomAndPlayer`, set `setMatch` if match already started, send `game_start` or `joined`
- If invalid: send error and close connection
- Add token storage to `Hub`/`ConnRegistry` with cleanup on disconnect
- Integration test: create room via HTTP, join via HTTP, then connect WS with token, receive `game_start`

**Acceptance**: WASM client can connect with `rejoin` token and immediately enter the game. No lobby phase needed.

### T3: HTML Landing Page — PETSCII UI
**Files**: `web/index.html` (rewrite), `web/game.html` (new), `web/style.css` (new), `web/app.js` (new)
**Depends on**: T1

- Create `web/index.html` — the lobby landing page
  - Uses PetMe.ttf font (`@font-face` from `/fonts/PetMe.ttf`)
  - Green-on-black PETSCII aesthetic (CSS custom properties for colors)
  - Three states: lobby, creating (waiting), joining (enter code)
  - Name entry field (HTML `<input>` with maxlength 16)
  - Difficulty slider (1-10, default 5)
  - "CREATE ROOM" button → `POST /api/room`
  - "JOIN ROOM" button → shows room code input
  - Room code input (4 chars, A-Z/2-9, all caps, copy/paste works)
  - "JOIN" button → `POST /api/room/{code}/join`
  - After create/join: shows "ROOM: AB2D — Share this code!" with click-to-copy
  - SSE connection to `/api/room/{code}/events` for opponent-join notification
  - When opponent joins: auto-redirect to `/game?room=AB2D&pid=1&token=xxx`
- Create `web/game.html` — the WASM game page
  - Minimal HTML wrapper, just `<canvas>` + WASM bootstrap
  - Reads `room`, `pid`, `token`, `name` from URL params
  - Passes to WASM client via `window.tankConfig = {room, pid, token, name, serverUrl}`

- Create `web/app.js` — lobby logic
  - `createRoom()` — POST to `/api/room`
  - `joinRoom()` — POST to `/api/room/{code}/join`
  - `pollRoomStatus()` — GET `/api/room/{code}/status` every 2s (or SSE)
  - `redirectToGame(data)` — construct URL and `window.location = /game?...`
  - Copy room code to clipboard

- Create `web/style.css` — PETSCII styling
  - `@font-face { font-family: 'PetMe'; src: url('/fonts/PetMe.ttf'); }`
  - Body: black background, `#33ff33` green text, PetMe font
  - Inputs: styled as PET terminal (border: 2px solid #33ff33, black bg, green text)
  - Buttons: inverted style (green bg, black text) on hover
  - Room code display: large monospace, blink animation for cursor

**No HTMX dependency** — vanilla JS + SSE is simpler and avoids an npm dependency. The lobby page is simple enough (3 states: lobby, waiting, redirect) that a full HTMX dependency isn't justified.

**Acceptance**: Opening `http://localhost:8080/` shows a PETSCII-styled lobby with name input, difficulty slider, Create/Join buttons. Creating a room shows a shareable code. Joining navigates to game. No WASM loaded until game starts.

### T4: WASM Client Simplification — Remove Lobby
**Files**: `client/game.go`, `client/gamestate.go`, `client/renderer.go`, `client/bridge_js.go`, `client/bridge_native.go`, `cmd/client/main.go`, `cmd/client/js_name.go`, `web/game.html`
**Depends on**: T2, T3

- Remove from `gamestate.go`:
  - `LobbyModeStart`, `LobbyModeDifficulty`, `LobbyModeCreating`, `LobbyModeJoining`, `LobbyModeWaiting`
  - `DifficultySelection`, `RoomCodeInput`, `LobbyMode`, `ErrorMsgText`
  - All lobby-related fields from `GameState`
- Remove from `game.go`:
  - `handleLobbyInput()` entirely
  - All lobby phase handling in `Update()`
  - `testMode` bridge (replaced by HTML testing)
- Remove from `renderer.go`:
  - `drawLobbyText()` entirely
  - Difficulty bar rendering
  - Lobby-related text drawing
- Simplify `game.go:Update()`:
  - On creation: connect WS with rejoin token, send `rejoin` message
  - Wait for `game_start` (or `joined` if opponent hasn't joined yet)
  - Once in `PhasePlaying`: handle game input normally
- Update `NewGame()` signature: receives roomCode, playerID, token, playerName, serverURL (all from URL params)
- Remove `Cmd/client/js_name.go`: `getJSTestMode()`, `getJSPlayerName()`, `getJSServerURL()` (replaced by `window.tankConfig`)
- Remove `client/bridge_js.go` and `client/bridge_native.go` (lobby testing bridge no longer needed)
- Update `web/game.html`: read `window.tankConfig` and pass to WASM

**Acceptance**: WASM client goes from connection → `rejoin` → `game_start` → playing. No lobby phase. No keyboard-based room joining. Binary size should decrease.

### T5: Server Push — SSE for Room Events
**Files**: `server/room_api.go`, `cmd/server/main.go`
**Depends on**: T1

- Add `GET /api/room/{code}/events` SSE endpoint
  - Content-Type: `text/event-stream`
  - Sends events: `player_joined`, `game_starting`, `room_expired`
  - Uses `http.Flusher` for real-time push
  - Client-side EventSource in `app.js` listens for events
  - When `player_joined` event received: both players redirect to `/game?room=X&pid=Y&token=Z`
  - When `game_starting` received: redirect immediately
- Alternative (simpler): Use HTMX polling (`hx-get="/api/room/{code}/status" hx-trigger="every 2s"`) instead of SSE
  - Polling is simpler, 2s delay is acceptable for a room starting
  - Decision: **Use polling for T5, add SSE later if needed**

**Acceptance**: Player 1 creates room, sees "waiting" page. Player 2 joins room code. Player 1's page updates to show "vs Bob" and auto-redirects to game. Both players' WASM clients connect and receive `game_start`.

### T6: Integration Testing & Polish
**Files**: `server/room_api_test.go`, `test/e2e/src/lobby.spec.ts`, `test/e2e/src/game.spec.ts`, `Makefile`
**Depends on**: T3, T4, T5

- Server unit tests for `POST /api/room`, `POST /api/room/{code}/join`, `GET /api/room/{code}/status`
- Server unit tests for `rejoin` message validation
- E2E test: create room via HTTP → join via HTTP → check status → connect WS with token → receive `game_start` → play game
- E2E test: invalid token → error response
- E2E test: room full → 422 response
- E2E test: invalid room code → 404 response
- Update `Makefile`: add `test-lobby` target for lobby E2E tests
- Update README with new lobby flow
- Remove dead lobby code from WASM client (verify all paths)
- Manual browser test: full flow from lobby to game to rematch

**Acceptance**: All tests pass. Full E2E flow: lobby → create → join → game → rematch works without any canvas-focus issues. Tab away and back — no input loss.

## Dependency Graph

```
T1 (HTTP API) ──────────────────┐
                                 │
T2 (Rejoin Protocol) ────────────┤
                                 │
T3 (HTML Landing Page) ──────────┤──── T6 (Integration Testing)
                                 │
T4 (WASM Simplification) ───────┤
                                 │
T5 (SSE/Polling) ───────────────┘
```

T1, T2, T3 can be developed in parallel. T4 depends on T2 (rejoin protocol). T5 depends on T1 (room API). T6 depends on all.

## What NOT To Do

- ❌ Don't add HTMX as a Go dependency — use vanilla JS + EventSource for the lobby
- ❌ Don't use JWT/session cookies for auth — use simple random tokens stored in Hub
- ❌ Don't remove the WASM game-over/rematch flow — it works well in-canvas
- ❌ Don't add player accounts or persistence — keep it anonymous rooms
- ❌ Don't add matchmaking — room codes are the V1 mechanism
- ❌ Don't remove the WS `/ws` endpoint — the game still uses WebSocket for all gameplay
- ❌ Don't break the existing E2E tests — they use WS directly and should still pass
- ❌ Don't make the HTMX lobby depend on npm — vanilla JS only, served from Go

## Files To Create

| File | Purpose |
|------|---------|
| `server/room_api.go` | HTTP API handlers for room create/join/status |
| `server/room_api_test.go` | Unit tests for HTTP API |
| `web/index.html` | PETSCII-styled lobby landing page |
| `web/game.html` | Minimal WASM game page |
| `web/style.css` | PETSCII styling (PetMe font, green-on-black) |
| `web/app.js` | Lobby logic (create/join/poll/redirect) |
| `web/fonts/PetMe.ttf` | Font file served to browsers |

## Files To Modify

| File | Change |
|------|--------|
| `cmd/server/main.go` | Add HTTP API routes, serve `web/fonts/`, add `/api/` handlers |
| `server/ws_handler.go` | Add `rejoin` message handling, token validation |
| `server/protocol.go` | Add `MsgTypeRejoin`, `RejoinMsg` struct |
| `server/hub.go` | Add token storage, `GetRoomStatus()`, `ValidateToken()` |
| `client/game.go` | Remove lobby, add rejoin flow, simplify `Update()` |
| `client/gamestate.go` | Remove `LobbyMode*`, simplify `GameState` |
| `client/renderer.go` | Remove `drawLobbyText()`, remove lobby rendering |
| `client/bridge_js.go` | Replace with `getConfig()` that reads `window.tankConfig` |
| `cmd/client/js_name.go` | Remove `getJSTestMode`, `getJSPlayerName`, `getJSServerURL` |
| `cmd/client/native_name.go` | Remove stubs |
| `cmd/client/main.go` | Read config from `window.tankConfig` or flags |
| `test/e2e/src/game.spec.ts` | Update to use HTTP API for room setup |
| `Makefile` | Add font copy, `test-lobby` target |
| `README.md` | Update controls/flow documentation |

## Files To Remove

| File | Reason |
|------|--------|
| `client/bridge_native.go` | Replaced by `window.tankConfig` in JS |
| (lobby code in `game.go`) | All `handleLobbyInput` and lobby branching |
| (lobby code in `renderer.go`) | All `drawLobbyText` and difficulty bar |

## Estimated Effort

| Task | Effort | Parallelizable |
|------|--------|----------------|
| T1: Server HTTP API | 1 day | ✅ (with T3) |
| T2: Rejoin Protocol | 0.5 day | ✅ (with T1, T3) |
| T3: HTML Landing Page | 1.5 days | ✅ (with T1, T2) |
| T4: WASM Simplification | 1 day | ❌ (after T2) |
| T5: SSE/Polling | 0.5 day | ✅ (with T3) |
| T6: Integration Testing | 1 day | ❌ (after all) |
| **Total** | **~5.5 days** | |