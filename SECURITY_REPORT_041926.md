# NetTank Branch — Internet Exposure Security Analysis

**Date:** 2026-04-19  
**Branch:** `NetTank` (commit `76d95a67`)  
**Scope:** Exposing the server on TCP port 4016 as an internet-facing service  
**Analyst:** Copilot security review

---

## Architecture Overview

The branch is a Go HTTP server (`cmd/server/main.go`) with these network-reachable surfaces:

| Path | Protocol | Purpose |
|---|---|---|
| `GET /ws` | WebSocket | Game communication (in-flight game state, inputs) |
| `POST /api/room` | HTTP | Create a room (optionally spawns bot subprocess) |
| `POST /api/room/{code}/join` | HTTP | Join an existing room |
| `GET /api/room/{code}/status` | HTTP | Poll room state |
| `GET /api/room/{code}/events` | HTTP SSE | Long-poll room events |
| `GET /health` | HTTP | Health probe |
| `GET /` | HTTP | Static file server (WASM, JS, HTML) |
| `GET /debug/pprof/...` | HTTP | Go pprof (conditional, separate addr) |

**Dependencies:** `github.com/coder/websocket v1.8.12`, `github.com/hajimehoshi/ebiten/v2 v2.9.9`

---

## 🔴 BLOCKERS — Must Fix Before Any Public Exposure

### B1. WebSocket upgrade disables origin verification
**File:** `server/ws_handler.go`, `ServeHTTP`
```go
conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
```
`coder/websocket` by default checks that the `Origin` header matches the `Host` header (same-origin policy for WS), protecting against Cross-Site WebSocket Hijacking (CSWSH). `InsecureSkipVerify: true` disables this check entirely. Any page on any origin (e.g., `evil.com`) can establish a WebSocket to the game server from a visitor's browser and act as a player, creating rooms, sending inputs, and receiving game ticks. This is particularly dangerous because game tokens issued via the HTTP API (`/api/room` → `token` response field) arrive in the browser's JavaScript context and a malicious page can steal them.

**Fix:** Replace with `OriginPatterns: []string{"yourdomain.com"}` to allow only your origin, or implement explicit token authentication in the WebSocket handshake (before the first protocol message).

---

### B2. CORS defaults to wildcard `*`; SSE hardcodes `*` and ignores the flag
**Files:** `cmd/server/main.go` (flag default), `server/room_api.go` `handleRoomEvents`

```go
// cmd/server/main.go
cors := flag.String("cors", "*", "CORS allowed origin")
```
```go
// server/room_api.go — handleRoomEvents
w.Header().Set("Access-Control-Allow-Origin", "*")
```

The CORS default of `*` permits any cross-origin site to call `POST /api/room` and `POST /api/room/{code}/join` and read the JSON responses. The SSE endpoint hardcodes `*` unconditionally, so even if the operator sets `-cors https://yourdomain.com` on the command line, SSE remains open to all origins. Combined with B1 (WS origin skip), a malicious page can create a room via CORS-allowed `POST /api/room`, receive the token, and then open a WebSocket to play.

**Fix:** Change the CORS default to the actual deployed origin; add an `OriginAllowed` check in `handleRoomEvents` using the same flag value passed to the middleware.

---

### B3. No HTTP request body size limit — memory amplification via oversized JSON
**File:** `server/room_api.go`, `handleCreateRoom` and `handleJoinRoom`

```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
```

Neither endpoint wraps `r.Body` with `http.MaxBytesReader`. An attacker can send a multi-gigabyte body; `json.Decoder` will buffer and parse it, exhausting server RAM before any field validation triggers.

**Fix:** Wrap each body with `r.Body = http.MaxBytesReader(w, r.Body, 4096)` before decoding.

---

### B4. Unconditional `net/http/pprof` import — heap/goroutine dumps reachable
**File:** `cmd/server/main.go`

```go
_ "net/http/pprof"
```

This blank import auto-registers all pprof endpoints (`/debug/pprof/heap`, `/debug/pprof/goroutine`, `/debug/pprof/symbol`, etc.) on `http.DefaultServeMux`. The main server mux is `http.NewServeMux()`, so pprof is **not** on the main port. However, if `-pprof-addr` is set, it calls `http.ListenAndServe(*pprofAddr, nil)` — i.e., it serves from `http.DefaultServeMux` with zero authentication. Pprof heap dumps contain: in-memory rejoin tokens (plaintext 64-hex-char strings from `TokenStore`), player names, room codes, and full goroutine stacks with local variables.

**Fix:** Remove the blank import (eliminate the risk entirely) or guard the pprof mux with at least HTTP Basic Auth over TLS.

---

### B5. No limit on concurrent WebSocket connections — goroutine/FD exhaustion DoS
**File:** `server/ws_handler.go`, `ServeHTTP`

Every accepted WebSocket connection immediately spawns two goroutines (`readPump`, `writePump`) and allocates a 256-element `sendCh` byte-slice channel:
```go
client := &ClientConn{
    sendCh: make(chan []byte, 256),
    ...
}
go client.readPump()
go client.writePump()
```

There is no per-IP connection limit, no global connection cap, and no admission control. A single attacker machine can open tens of thousands of connections, exhausting OS file descriptors and Go scheduler goroutines. Each connection costs ≈ 2 goroutines (~8 KB stack each) + channel buffer. 50,000 connections ≈ ~800 MB stack + FD exhaustion on default Linux limits.

**Fix:** Add a `semaphore.NewWeighted` or `atomic.Int64` counter; reject connections above a configured cap with `websocket.StatusTryAgainLater`.

---

### B6. No rate limiting on HTTP API endpoints — room namespace / bot DoS
**Files:** `server/room_api.go`, `server/bot_manager.go`

`POST /api/room` has no per-IP or global rate limiting. The Hub can hold at most ~923,521 rooms (31-char alphabet, 4 chars). An attacker can spam room creation until the namespace is full, after which `CreateRoom` returns `nil` and all subsequent `POST /api/room` calls return 500. When `TANK_ENABLE_BOTS=1`, each `POST /api/room` with `vs_ai=true` triggers `AssignBotToRoom` → `exec.Command(bm.botBinary, ...)` in `bot_manager.go`. Unlimited bot subprocess spawning is reachable with no rate gate, enabling a fork-bomb-style DoS.

**Fix:** Apply per-IP rate limiting middleware (token bucket or fixed window) to all `/api/room` mutations; cap total Hub room count; limit concurrent bot subprocess count.

---

### B7. Bot subprocess command path controllable via environment variable without validation
**File:** `server/bot_manager.go`, `resolveBotBinary`

```go
if env := os.Getenv("TANK_BOT_BIN"); env != "" {
    return env
}
```

If an attacker can influence the server's environment (e.g., shared hosting, CI/CD injection, or misconfigured container), they can point `TANK_BOT_BIN` to any binary on the server. `exec.Command(bm.botBinary, ...)` would then execute it with arguments that include `roomCode`, `token`, and `playerID`. The `roomCode` comes from `hub.generateCode()` (safe 31-char alphabet), and `token` is cryptographically random — so argument injection through those fields is not possible. But the arbitrary binary path is a privilege escalation vector in the deployment environment.

**Fix:** At startup, validate that the resolved binary path is within a set of allowed directories; reject relative paths outside the application's working directory.

---

## 🟡 STRONGLY RECOMMENDED

### S1. WebSocket `readPump` reads with `context.Background()` — no idle read timeout
**File:** `server/ws_handler.go`, `readPump`

```go
ctx := context.Background()
for {
    _, data, err := c.conn.Read(ctx)
```

A connected client that never sends a message holds the goroutines open indefinitely. Legitimate HTTP-level `ReadTimeout: 15s` does not apply after the WebSocket upgrade. The `writePump` does enforce a 5-second write deadline per message, but the receive side has no timeout. An attacker can hold thousands of "silent" WebSocket connections open after the initial handshake.

**Fix:** Use a ticker-based ping or set a context deadline per read, resetting on each successful read.

---

### S2. SSE stream has no authentication — room event snooping by any external party
**File:** `server/room_api.go`, `handleRoomEvents`

Any unauthenticated client can subscribe to `GET /api/room/{code}/events` for any room code and learn: when a second player joins, whether a bot joined, the opponent's display name, and when the game starts. Room codes (4 chars, 31-char alphabet) have ~923K combinations. A systematic scan of all codes reveals all active rooms. Combined with B2, this works cross-origin.

**Fix:** Require the rejoin token (generated at room create/join) as a query parameter or header to authenticate SSE access.

---

### S3. Hub has no room count cap — unlimited memory growth
**File:** `server/hub.go`, `CreateRoom`

```go
h.rooms[code] = room
```

The `rooms` map grows without bound. Each `Room` holds players, match controllers with full board state, and delta tracker state. Room cleanup (`CleanupStaleRooms`) only runs every 60 seconds for `RoomWaiting` rooms older than `maxRoomAge`. Rooms in `RoomPlaying` or `RoomGameOver` state are never cleaned up by the ticker — only on player disconnect.

**Fix:** Add a configurable maximum room count checked in `CreateRoom` before allocating.

---

### S4. AutoFillAfterSec timer is not cancelled on room expiry — goroutine/bot leak
**Files:** `server/room_api.go` `handleCreateRoom`, `server/hub.go` `CleanupStaleRooms`

```go
room.SetAutoFillTimer(time.AfterFunc(time.Duration(req.AutoFillAfterSec)*time.Second, func() {
    api.botManager.AssignBotToRoom(room.Code, "mvp")
}))
```

`CleanupStaleRooms` does `delete(h.rooms, code)` but never calls `room.CancelBotReservation()`. If the room expires while the timer is still pending, the timer goroutine remains alive until it fires. With a large `auto_fill_after_sec` value (no maximum validated), many rooms can each hold a live timer goroutine for an extended period.

**Fix:** Call `room.CancelBotReservation()` inside `CleanupStaleRooms` before deleting the room entry; validate `auto_fill_after_sec` to a reasonable maximum (e.g., ≤ 300 seconds).

---

### S5. No maximum game/match duration — bot or idle match runs forever
**File:** `server/match.go`, `runLoop`

```go
ticker := time.NewTicker(time.Second / 60)
for {
    select {
    case <-mc.stopCh:
        return
    case <-ticker.C:
        mc.stepTick()
```

A match ticks at 60 Hz indefinitely until a winner is determined or `Stop()` is called. A bot or malicious client that never moves and never fires can keep a match running forever, holding a match ticker goroutine, an engine `GameController`, and a `DeltaTracker`.

**Fix:** Add a `maxMatchTicks` limit (e.g., 10 minutes × 60 fps = 36,000 ticks) after which the match is force-stopped and both clients notified of a draw.

---

### S6. Missing HTTP security headers on all responses
**File:** `server/middleware.go` (only provides CORS and WASM cache headers)

No middleware sets:
- `X-Content-Type-Options: nosniff` — MIME-sniffing attacks on served WASM/JS
- `X-Frame-Options: DENY` — prevents embedding in attacker iframes
- `Referrer-Policy: no-referrer` — prevents room codes leaking via `Referer` header

**Fix:** Add a security headers middleware wrapping the entire mux.

---

### S7. Room code brute-force enumeration via unauthenticated status endpoint
**File:** `server/room_api.go`, `handleRoomStatus`

`GET /api/room/{code}/status` returns distinct 404 vs. 200 with no rate limiting. With ~923,521 total possible room codes, an attacker can enumerate all active rooms in minutes. Room status leaks player names and bot configuration.

**Fix:** Per-IP rate limiting on the status and events endpoints; consider returning 404 for all non-joinable rooms to avoid distinguishing between "wrong code" and "room full" states.

---

### S8. lagproxy must never reach production
**File:** `cmd/lagproxy/main.go`

The lag proxy binds to `:8081` with no authentication, no TLS, and no origin validation. It is a developer testing tool and must not be included in any production deployment package.

**Fix:** Document explicitly that `lagproxy` is dev-only; exclude it from production build targets; ensure port 8081 is blocked at the firewall in any production environment.

---

## 🟢 NICE-TO-HAVES

### N1. No TLS — all traffic (tokens, names, game state) travels in cleartext
The server only speaks plain HTTP/WS. Tokens issued at `POST /api/room` travel over plain WebSocket (`ws://`) during the rejoin handshake. On a public internet path, these are trivially sniffed.

**Recommendation:** Terminate TLS at the server or via a reverse proxy (nginx, Caddy). Enforce `wss://` by checking the Origin scheme.

---

### N2. No structured / request-correlation logging
`log.Printf` throughout produces flat text with no request ID, no remote IP, and no severity level. When investigating abuse, there is no way to correlate a series of events from one actor.

**Recommendation:** Adopt `log/slog` (Go 1.21 stdlib) with a middleware that injects `request_id` and `remote_addr` into the log context.

---

### N3. No explicit WebSocket message size limit
The `coder/websocket` library enforces a default read limit (32 KB). There is no explicit `conn.SetReadLimit(N)` call in `readPump`. Explicitly setting a tight limit (e.g., 1 KB, since input messages are short) communicates intent and protects against library default changes across upgrades.

---

### N4. `WriteTimeout: 300s` is excessive for non-SSE HTTP endpoints
**File:** `cmd/server/main.go`

```go
WriteTimeout: 300 * time.Second,
```

This is set to accommodate long SSE streams but applies to all HTTP responses including the health endpoint and API calls.

**Recommendation:** Use `http.TimeoutHandler` to enforce tighter response deadlines on API routes, while keeping the liberal timeout only for SSE/WebSocket upgrade paths.

---

### N5. No `govulncheck` integration in CI
No automated dependency vulnerability audit is present in the `Makefile` or CI configuration.

**Recommendation:** Add `govulncheck ./...` to CI and the `make test` target.

---

## Prioritized Summary

| # | Severity | Finding | File / Location |
|---|---|---|---|
| B1 | 🔴 Blocker | `InsecureSkipVerify: true` — CSWSH, any origin can hijack WS | `server/ws_handler.go:ServeHTTP` |
| B2 | 🔴 Blocker | CORS wildcard default + SSE hardcodes `*` ignoring the flag | `cmd/server/main.go`, `server/room_api.go:handleRoomEvents` |
| B3 | 🔴 Blocker | No request body size limit — RAM exhaustion via oversized JSON | `server/room_api.go:handleCreateRoom,handleJoinRoom` |
| B4 | 🔴 Blocker | Unconditional pprof import; unauth pprof endpoint leaks tokens | `cmd/server/main.go` |
| B5 | 🔴 Blocker | No WS connection limit — goroutine/FD exhaustion DoS | `server/ws_handler.go:ServeHTTP` |
| B6 | 🔴 Blocker | No API rate limiting; unlimited bot subprocess spawning | `server/room_api.go`, `server/bot_manager.go` |
| B7 | 🔴 Blocker | Bot binary path from env var — arbitrary exec on env control | `server/bot_manager.go:resolveBotBinary` |
| S1 | 🟡 Strong | No idle read timeout on WS — silent connection hold | `server/ws_handler.go:readPump` |
| S2 | 🟡 Strong | SSE events unauthenticated — room enumeration & snooping | `server/room_api.go:handleRoomEvents` |
| S3 | 🟡 Strong | No room count cap — unbounded memory growth | `server/hub.go:CreateRoom` |
| S4 | 🟡 Strong | AutoFill timer not cancelled on room eviction — goroutine leak | `server/room_api.go`, `server/hub.go:CleanupStaleRooms` |
| S5 | 🟡 Strong | No max match duration — forever-tick goroutine | `server/match.go:runLoop` |
| S6 | 🟡 Strong | Missing security headers (CSP, XCTO, XFO) | `server/middleware.go` |
| S7 | 🟡 Strong | Room code enumeration via unauthenticated status endpoint | `server/room_api.go:handleRoomStatus` |
| S8 | 🟡 Strong | lagproxy must never be deployed to production | `cmd/lagproxy/main.go` |
| N1 | 🟢 Nice | No TLS — cleartext tokens, names, game state | `cmd/server/main.go` |
| N2 | 🟢 Nice | No structured / correlated logging | throughout |
| N3 | 🟢 Nice | No explicit WebSocket message size limit | `server/ws_handler.go:readPump` |
| N4 | 🟢 Nice | `WriteTimeout: 300s` excessive for non-SSE routes | `cmd/server/main.go` |
| N5 | 🟢 Nice | No `govulncheck` in CI | `Makefile` |

---

**Bottom line:** The server has a solid structural foundation — `crypto/rand` tokens, input validation, mutex-protected shared state, graceful shutdown — but is not safe for direct public internet exposure in its current form. The two most urgent issues are the WebSocket origin bypass (**B1**) and wildcard CORS + hardcoded SSE `*` (**B2**), which together allow any website to silently act as a game client on behalf of visitors. Rate limiting (**B6**) and connection caps (**B5**) must also be in place before the bot-spawning path (`TANK_ENABLE_BOTS=1`) is reachable from the internet.
