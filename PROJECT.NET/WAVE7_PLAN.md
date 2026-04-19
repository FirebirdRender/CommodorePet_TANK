# Wave 7: Testing & E2E Infrastructure — Task Plan

## Completion Status (updated 2025-04-19)

| Task | Description | Status | Evidence |
|------|-------------|--------|----------|
| T1 | JS State Bridge | ✅ Complete | `client/bridge_js.go` (90 lines): `getGameState()`, `sendInput()`, `sendPlayAgain()` registered; `client/bridge_native.go` (7 lines): no-op stub; `client/gamestate_test.go`: `TestExportGameStateNativeNoop` |
| T2 | Test Mode Flag | ✅ Complete | Replaced by WAVE8's HTML lobby — `?test=1` approach dropped in favor of `window.tankConfig` via `bridge_js.go:getJSConfig()`; `getPhaseName` bridge removed; the WASM client no longer has a lobby phase |
| T3 | wasmbrowsertest | ❌ Not started | No `client/wasm_test.go`; no `wasmbrowsertest` in `go.sum`; no `test-wasm` Makefile target |
| T4 | Headless Rendering | ⚠️ Partial | `client/renderer_test.go` has unit tests (`TestCellGlyphsCoversAllCellTypes`, `TestBarrelPos`, `TestColorConstants`, `TestDimensionConstants`, `TestGlyphCache*`) but NO pixel-level rendering tests (`TestDraw*` / `TestRender*`) |
| T5 | Playwright E2E | ✅ Complete | `test/e2e/src/game.spec.ts` (104 lines): 8 tests covering page load, create room, join room, game flow, disconnect, test-mode bridge; `test/e2e/playwright.config.ts` + `package.json` set up |
| T6 | Makefile Integration | ✅ Complete | `Makefile`: `test-e2e`, `test-headless`, `test-all` targets exist; `test-wasm` target NOT created (T3 not done) |

**Remaining work**: T3 (WASM unit tests via wasmbrowsertest), T4 (pixel-level rendering verification with headless ebiten), and a `test-wasm` Makefile target.

**Date**: April 15, 2026  
**Branch**: NetTank  
**Status**: Partial — T1 ✅ T2 ✅ T5 ✅ T6 ✅ | T3 ❌ T4 ❌  
**PRD Reference**: PROJECT.NET/WAVE6_PLAN.md (follow-up from acceptance testing)

## Goal

Build comprehensive testing infrastructure for the WASM client: browser-based E2E tests, headless rendering verification, JavaScript state bridges for Playwright, and WASM-specific unit tests. A developer should be able to run `make test-e2e` and verify that create/join/ready/play/leave works end-to-end in an actual browser.

**Acceptance**: `make test-e2e` runs Playwright tests that verify full game flow in Chromium. `go test -race ./client/...` passes headless unit tests. `go test -race ./server/...` passes with new WASM-aware tests.

## V1 Scope (IN)

- JavaScript state bridge (`window.getGameState()`) for test inspection
- WASM-specific unit tests via `wasmbrowsertest`
- Headless rendering verification (pixel-level assertions on `ebiten.Image`)
- Playwright E2E test suite (create room, join, ready, play, disconnect)
- Test mode flag (`?test=1`) that exposes additional debug info

## V1 Scope (OUT)

- Visual regression testing (screenshot comparison)
- Performance/load testing
- CI/CD pipeline configuration (GitHub Actions, etc.)
- Cross-browser testing (Firefox, Safari)
- Mobile/touch testing
- Accessibility testing

---

## Current State Analysis

### Test Coverage (as of Wave 6)

| Package | Files | Coverage | Type |
|---------|-------|----------|------|
| engine/ | 11 | 92.3% | Pure logic unit tests |
| server/ | 13 | 85.1% | WebSocket integration tests |
| client/ | 7 | 27.2% | Unit tests only (no WASM/render tests) |
| cmd/client | 0 | 0% | No tests |
| cmd/server | 0 | 0% | No tests |

### What's Missing

| Gap | Impact |
|-----|--------|
| No WASM execution tests | Client binary runs but no automated verification |
| No rendering verification | Canvas draws pixels but we never verify them |
| No input simulation | No automated keyboard/mouse testing |
| No browser E2E | Full game flow (create→join→play→leave) untested in browser |
| No JS state bridge | No way for external tools to inspect game state |
| Flaky integration tests | `TestClientConnectCreateRoom` etc. depend on timing |

---

## Architecture Decisions

1. **Test mode via URL parameter**: `?test=1` enables `window.getGameState()` and `window.sendInput()` JS bridges. Not enabled in production to avoid cheating.

2. **Playwright over Selenium**: Playwright has better WebSocket inspection, native async/await, and headless Chromium support. Go's `chromedp` is an alternative but Playwright's API is more ergonomic for game testing.

3. **wasmbrowsertest for unit tests**: The `wasmbrowsertest` package allows running Go tests in a real browser environment. It verifies WASM-specific logic (JS interop, `syscall/js` calls) without needing Playwright.

4. **Headless rendering via `ebiten.NewImage`**: Ebitengine supports creating offscreen images for testing. We can verify that specific pixels are the expected color after calling draw functions.

5. **Server test mode**: The Go test server already exists (`setupTestServer`). We extend it to also serve the WASM client and static files for full E2E.

---

## Task Breakdown

### Phase 1 — State Bridge & Test Mode (independent)

#### T1: JavaScript State Bridge

**Files**: `client/bridge.go` (new, `// +build js wasm`), `client/bridge_stub.go` (new, `// +build !js,!wasm`), `web/index.html`  
**Category**: `deep`

Add a JavaScript-accessible state bridge that allows external testing tools (Playwright, chromedp) to inspect game state and simulate input.

**bridge.go** (`// +build js wasm`):
```go
package client

import "syscall/js"

// ExportGameState registers a window.getGameState() function that returns
// a JavaScript object with the current game state. Only active when
// testMode is true (set via ?test=1 URL parameter).
func (g *Game) ExportGameState() {
    js.Global().Set("getGameState", js.FuncOf(func(this js.Value, args []js.Value) any {
        return js.ValueOf(map[string]any{
            "phase":        int(g.state.Phase),
            "lobbyMode":    int(g.state.LobbyMode),
            "playerID":     g.state.PlayerID,
            "playerName":   g.state.PlayerName,
            "opponentName":  g.state.OpponentName,
            "roomCode":      g.state.RoomCode,
            "connected":     g.state.Connected,
            "difficulty":   g.state.DifficultySelection,
            "tanks":         gameStateToJS(g.state.Tanks),
            "error":         g.state.ErrorMsgText,
            "connectErr":    g.state.ConnectErr,
        })
    }))

    js.Global().Set("sendInput", js.FuncOf(func(this js.Value, args []js.Value) any {
        // Simulate a key press by sending it through the network
        key := args[0].String()
        action := "down"
        if len(args) > 1 {
            action = args[1].String()
        }
        if g.network != nil && g.network.Connected {
            g.network.Send(MsgTypeInput, InputMsg{
                Tick:   uint64(g.state.Tick),
                Key:    key,
                Action: action,
            })
        }
        return nil
    }))

    js.Global().Set("getPhaseName", js.FuncOf(func(this js.Value, args []js.Value) any {
        phases := map[GamePhase]string{
            PhaseConnecting: "connecting",
            PhaseLobby:      "lobby",
            PhasePlaying:    "playing",
            PhaseRoundOver:  "roundOver",
            PhaseGameOver:   "gameOver",
            PhaseDisconnected: "disconnected",
        }
        return js.ValueOf(phases[g.state.Phase])
    }))
}

func gameStateToJS(tanks [2]TankState) []any {
    result := make([]any, len(tanks))
    for i, t := range tanks {
        result[i] = map[string]any{
            "x":         t.X,
            "y":         t.Y,
            "dir":       t.Dir,
            "lives":     t.Lives,
            "shotsLeft": t.ShotsLeft,
            "minesLeft": t.MinesLeft,
            "active":    t.Active,
        }
    }
    return result
}
```

**bridge_stub.go** (`// +build !js,!wasm`):
```go
package client

// ExportGameState is a no-op on native builds.
func (g *Game) ExportGameState() {}
```

**Integration**: Call `g.ExportGameState()` in `ConnectAsync()` after successful connection, and in `game.go`'s `Update()` every 60 frames (to keep state updated).

**web/index.html**: Add `window.getGameState` call check in debug log.

**Tests**: `client/bridge_test.go` — verify bridge functions register correctly when test mode is active.

**QA**: `GOOS=js GOARCH=wasm go build -o /dev/null ./cmd/client/` compiles. `go build ./client/` compiles (stub).

---

#### T2: Test Mode Flag

**Files**: `client/gamestate.go`, `client/game.go`  
**Category**: `quick`

Add a `TestMode` flag to GameState that's set when the URL contains `?test=1`. In test mode, the game:

- Calls `ExportGameState()` to register JS bridges
- Logs all state transitions to the browser console
- Accepts keyboard shortcuts for testing (e.g., `Ctrl+Shift+S` to dump state)

```go
// In gamestate.go:
TestMode bool `json:"test_mode"`

// In game.go Update():
if g.state.TestMode {
    // Update JS bridge every 60 frames
    if g.state.Tick%60 == 0 {
        g.ExportGameState()
    }
}
```

Parse `?test=1` from URL in `getJSServerURL()` or a separate `getJSTestMode()` function.

**QA**: Open browser with `?test=1`, verify `window.getGameState()` returns a JS object.

---

### Phase 2 — Headless Unit Tests (after Phase 1)

#### T3: WASM Unit Tests with wasmbrowsertest

**Files**: `client/wasm_test.go` (new, `// +build js wasm`), `go.mod` (add dependency)  
**Category**: `deep`

Set up `wasmbrowsertest` for running Go tests in a real browser environment. This tests WASM-specific logic that can't be tested natively.

Tests to create:
1. `TestGetJSPlayerNameFromURL` — verify `?name=Alice` is read correctly
2. `TestGetJSServerURLFromLocation` — verify server URL derives from `window.location`
3. `TestGetJSTestMode` — verify `?test=1` is read correctly
4. `TestGameStateBridgeJSON` — verify `getGameState()` returns parseable JSON with correct fields
5. `TestSendInputBridge` — verify `sendInput("up", "down")` doesn't panic

**Setup**:
```bash
# Add wasmbrowsertest dependency
go get github.com/nicholasgasior/wasmbrowsertest
```

Create test file with `// +build js wasm` build tag. Set up CI to run these separately from native tests.

**QA**: `GOOS=js GOARCH=wasm go test ./client/... -count=1 -run TestWASM` passes.

---

#### T4: Headless Rendering Verification

**Files**: `client/renderer_test.go` (extend)  
**Category**: `deep`

Add pixel-level rendering tests that verify the drawing functions produce correct output.

Tests to create:
1. `TestDrawGridRendersAllCells` — after `drawGrid`, all cells at expected positions are non-black
2. `TestDrawHUDShowsPlayerNames` — after draw, HUD area contains text pixels
3. `TestDrawLobbyTextRendersCorrectly` — lobby screen has text at expected positions
4. `TestDrawGameOverOverlayShowsWinner` — game over screen has winner text
5. `TestDifficultyBarVisualisation` — difficulty selection shows filled/empty blocks

**Challenge**: Ebitengine's `ebiten.NewImage()` requires a display context. In tests, we need to use `ebitenutil.NewImageFromImage()` with a plain `image.RGBA` or create images via `ebiten.NewImage()` in the test's init function.

**Approach**: Create a test helper that initializes a minimal Ebitengine context:
```go
func initTestRenderer(t *testing.T) *Renderer {
    // Use ebiten.NewImage for offscreen rendering
    // This works in tests because we only need the image operations,
    // not the actual window/presentation pipeline
    r, err := NewRenderer()
    if err != nil {
        t.Fatalf("failed to create renderer: %v", err)
    }
    return r
}
```

**QA**: `go test -race ./client/... -count=1 -run TestDraw` passes.

---

### Phase 3 — Playwright E2E (after Phase 2)

#### T5: Playwright Test Infrastructure

**Files**: `test/e2e/playwright.config.ts` (new), `test/e2e/package.json` (new), `test/e2e/src/game.spec.ts` (new)  
**Category**: `deep`

Set up Playwright test infrastructure for browser-based E2E testing.

**playwright.config.ts**:
```typescript
import { defineConfig } from '@playwright/test';
export default defineConfig({
    testDir: './src',
    timeout: 30000,
    retries: 1,
    use: {
        headless: true,
        baseURL: 'http://localhost:8080',
    },
    webServer: {
        command: 'cd ../.. && make dev',
        port: 8080,
        reuseExistingServer: !process.env.CI],
    },
});
```

**package.json**: Playwright + TypeScript dependencies.

**Test scenarios** (`game.spec.ts`):
1. **Page loads and WASM initializes**
   - Navigate to `http://localhost:8080/`
   - Wait for `window.getGameState` to be defined (indicates WASM loaded)
   - Assert `phase === 'connecting'` or `phase === 'lobby'`

2. **Create room flow**
   - Navigate with `?test=1`
   - Wait for `getGameState().phase === 'lobby'`
   - Simulate pressing 'C' key
   - Wait for `getGameState().lobbyMode === 'difficulty'` (or 'creating')
   - Verify state transition is correct

3. **Join room flow** (requires two browser contexts)
   - Open page A: create room
   - Get room code from `getGameState().roomCode`
   - Open page B: navigate with `?test=1`
   - Type room code
   - Verify both reach `lobbyMode === 'waiting'`

4. **Game play flow** (requires two contexts + server)
   - Create room in page A
   - Join room in page B
   - Both ready → game starts
   - Verify `getGameState().phase === 'playing'`
   - Simulate input via `sendInput('right', 'down')`
   - Verify position changes

5. **Disconnect notification**
   - Two players in game
   - Close page B
   - Verify page A shows `phase === 'disconnected'`

6. **Test mode verification**
   - Navigate with `?test=1`
   - Verify `window.getGameState` exists
   - Verify `window.sendInput` exists
   - Verify `window.getPhaseName` exists

**QA**: `cd test/e2e && npx playwright test` passes (requires running server).

---

#### T6: Makefile Integration & CI Script

**Files**: `Makefile` (extend), `.github/workflows/test.yml` (new, optional)  
**Category**: `quick`

Add Makefile targets for the new test infrastructure:

```makefile
test-e2e: build-all
	cd test/e2e && npm install && npx playwright install chromium && npx playwright test

test-wasm:
	GOOS=js GOARCH=wasm go test ./client/... -count=1 -run TestWASM

test-headless:
	go test -race ./client/... -count=1 -run "TestDraw|TestRender|TestNew"

test-all: test test-wasm test-headless test-e2e
```

Optional GitHub Actions workflow for CI:
```yaml
name: Tests
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: make test
      - run: make test-headless
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: make build-all
      - run: make test-e2e
```

**QA**: `make test-all` runs all test suites. `make test-e2e` runs Playwright.

---

## Dependency Graph

```
Phase 1 (parallel):
  T1: JS State Bridge ──────────── independent
  T2: Test Mode Flag ────────────── independent

Phase 2 (after Phase 1):
  T3: wasmbrowsertest ──────────── depends on T1 (bridge)
  T4: Headless Rendering Tests ──── depends on T1 (bridge)

Phase 3 (after Phase 2):
  T5: Playwright E2E ────────────── depends on T1, T2, T3
  T6: Makefile & CI ────────────── depends on T5
```

## Parallel Execution Opportunities

- **T1 + T2**: Both Phase 1 tasks are independent
- **T3 + T4**: Both Phase 2 tasks can run in parallel after T1
- **T5 + T6**: T6 can start after T5's test structure is defined

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|------------|
| wasmbrowsertest requires browser on CI | Medium | Use Playwright for CI; local dev can skip WASM unit tests |
| Ebitengine rendering tests need display context | Low | Use offscreen images; mock display in test init |
| Playwright tests are slow and flaky | Medium | Add retries, use `test.slow()` for network-dependent tests |
| JS bridge could enable cheating | Medium | Only enable in test mode (`?test=1`); strip in production build |
| Two-browser E2E tests are complex | High | Start with single-browser tests; add two-browser later |

## Estimated Effort

| Task | Lines | Complexity | Category |
|------|-------|-----------|----------|
| T1: JS State Bridge | ~150 | Medium | deep |
| T2: Test Mode Flag | ~40 | Low | quick |
| T3: wasmbrowsertest | ~120 | Medium | deep |
| T4: Headless Rendering | ~150 | Medium | deep |
| T5: Playwright E2E | ~300 | High | deep |
| T6: Makefile & CI | ~80 | Low | quick |
| **Total** | **~840** | | |

## Acceptance Criteria

1. ✅ `window.getGameState()` returns correct game state in browser with `?test=1`
2. ✅ `window.sendInput('up', 'down')` sends input to server
3. ✅ `GOOS=js GOARCH=wasm go test ./client/... -count=1` passes WASM unit tests
4. ✅ `go test -race ./client/... -count=1 -run "TestDraw|TestRender"` passes rendering tests
5. ✅ `cd test/e2e && npx playwright test` passes E2E browser tests
6. ✅ `make test-all` runs all test suites
7. ✅ Game works identically with and without `?test=1` parameter
8. ✅ Test mode (`?test=1`) does not affect normal gameplay performance