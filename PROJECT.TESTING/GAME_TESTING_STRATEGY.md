# 2D Web Game Testing Strategy
# Stack: Go + Ebitengine + WASM

> This document defines the testing architecture for a 2D web-based game
> built with Go, Ebitengine, and compiled to WebAssembly. It is intended
> as a reference and directive for agentic coding tools implementing or
> scaffolding the test infrastructure.

---

## Guiding Principle

> Treat Ebitengine as the shell. The game itself must be a deterministic,
> pure Go state machine that can be driven entirely from tests — without
> a browser, canvas, or real frame loop.

The single most important architectural decision for testability is:

- **Game logic MUST NOT depend on Ebitengine, the browser, or real time.**
- **Rendering MUST only READ state — never write or decide.**
- **Input MUST be abstracted so tests can inject it directly.**

---

## Package Architecture

The codebase MUST be structured to separate concerns as follows:

```
/internal
  /gamecore         # Pure game logic — no engine dependencies
    state.go        # Full GameState type definition
    update.go       # Update(state, inputs) -> GameState (pure function)
    collision.go    # Collision rules and helpers
    physics.go      # Movement, velocity, gravity rules
    combat.go       # Damage, invulnerability, health logic
    progression.go  # Score, level advancement, win/loss conditions
    ai.go           # Enemy AI decision logic
    save.go         # Save/load serialization (no IO, just encoding)

  /uiflow           # Menu and scene state machines — no engine dependencies
    scenes.go       # Scene enum and transition rules
    menus.go        # Menu state: selected index, visible items, modal state
    navigation.go   # Input -> menu state transitions

  /platform         # Thin engine adapters — Ebitengine and browser glue
    ebiten_game.go  # Implements ebiten.Game interface; delegates to gamecore
    input_adapter.go# Maps ebiten input to internal Action types
    audio.go        # Audio playback adapter
    storage.go      # Browser localStorage / file save adapter
    wasm.go         # WASM-specific init and browser API calls

  /testharness      # Shared test utilities — used by integration tests
    harness.go      # TestGameHarness: step, press, release, assert helpers
    scenario.go     # ScriptedScenario runner: frame-indexed input sequences
    fixtures.go     # Common test states: default level, empty inventory, etc.
    fake_input.go   # FakeInputSource implementing the InputSource interface
    fake_clock.go   # FakeClock for deterministic tick-based time

/tests
  /integration      # Headless simulation tests — plain `go test`, no browser
    menu_flow_test.go
    scene_transition_test.go
    gameplay_smoke_test.go
    collision_test.go
    combat_test.go
    save_load_test.go
    checkpoint_test.go

  /e2e              # Browser + WASM tests — requires built artifact + browser
    /playwright
      game.spec.ts        # Smoke: page loads, canvas visible, game starts
      menu_nav.spec.ts    # Keyboard nav through menus
      gameplay_smoke.spec.ts # One minimal gameplay path
```

---

## Input Abstraction (Required)

Define an `InputSource` interface in `/internal/gamecore` or `/internal/platform`:

```go
type Action int

const (
    ActionUp Action = iota
    ActionDown
    ActionLeft
    ActionRight
    ActionConfirm
    ActionCancel
    ActionPause
    ActionAttack
    ActionJump
)

type InputSource interface {
    IsPressed(a Action) bool
    IsJustPressed(a Action) bool
    IsJustReleased(a Action) bool
}
```

- The real `platform/input_adapter.go` implements this using `ebiten/v2/inpututil`.
- The test `testharness/fake_input.go` implements this using frame-indexed maps.
- `Update(state GameState, input InputSource) GameState` takes the interface, never ebiten directly.

---

## Determinism Requirements

All game logic MUST be deterministic and tick-based:

- Use a `TickClock` interface, not `time.Now()` or `time.Since()`.
- Use a seeded, injectable `rand.Source` — never `math/rand` global state.
- All Update calls take a fixed `deltaT` (e.g., 1 tick = 1/60s) — no wall clock.
- All scripted test scenarios are expressed as frame-indexed input sequences.

---

## Test Harness API (Implement in `/internal/testharness`)

The harness must expose at minimum:

```go
// NewHarness creates a fresh game state with a seeded RNG and fake clock.
func NewHarness(seed int64) *TestGameHarness

// Step advances the game by n ticks with current inputs applied.
func (h *TestGameHarness) Step(n int)

// Press marks an action as held from the next Step onward.
func (h *TestGameHarness) Press(a Action)

// Release removes a held action.
func (h *TestGameHarness) Release(a Action)

// Tap presses and releases an action in the same tick.
func (h *TestGameHarness) Tap(a Action)

// State returns the current GameState snapshot.
func (h *TestGameHarness) State() GameState

// RunScenario executes a scripted frame sequence and returns the final state.
func (h *TestGameHarness) RunScenario(s Scenario) GameState
```

A `Scenario` is a list of `(frame int, action FrameAction)` pairs that the harness
replays deterministically.

---

## Test Layers

### Layer 1 — Unit Tests
**Location:** Alongside source files as `*_test.go` in `/internal/gamecore` and `/internal/uiflow`
**Runner:** `go test ./internal/...`
**Coverage target:** All pure logic functions

Must cover:
- [ ] Entity movement rules and boundary clamping
- [ ] Collision detection helpers (AABB, tile overlap, etc.)
- [ ] Damage calculation, invulnerability frame windows
- [ ] Jump arc: start, peak, land state transitions
- [ ] Animation state progression logic
- [ ] Menu selection wraparound (top/bottom)
- [ ] Input rebinding and conflict detection
- [ ] Save/load encoding round-trips (marshal → unmarshal → assert equal)
- [ ] Score and level progression rules
- [ ] Enemy AI decision table outputs
- [ ] RNG seeding produces reproducible sequences

---

### Layer 2 — Integration / Headless Simulation Tests
**Location:** `/tests/integration/`
**Runner:** `go test ./tests/integration/...`
**Tooling:** Uses `/internal/testharness` harness only — no browser, no ebiten rendering
**Coverage target:** All meaningful user journeys through state

Must cover:

#### Menu and Navigation
- [ ] Title screen is initial scene on game start
- [ ] Confirm on title screen transitions to Main Menu
- [ ] Down arrow moves selection index down; wraps at bottom
- [ ] Up arrow moves selection index up; wraps at top
- [ ] Confirm on "New Game" transitions to gameplay scene
- [ ] Confirm on "Options" opens options menu
- [ ] Cancel/Escape in options returns to main menu
- [ ] Confirm on "Quit" triggers quit state

#### Scene Transitions
- [ ] Title → Main Menu → Gameplay → Pause → Gameplay → Game Over → Main Menu
- [ ] Level complete triggers next-level scene transition
- [ ] Death transitions to respawn or game over based on lives remaining

#### Gameplay Elements
- [ ] Player moves right for N ticks → reaches expected X position
- [ ] Player cannot pass through solid tile collision boundary
- [ ] Jump starts, arcs, and lands in expected number of ticks
- [ ] Pickup collected → inventory/score/state updated correctly
- [ ] Enemy contact → HP reduced by correct amount
- [ ] Invulnerability frames prevent repeated damage within window
- [ ] 3 enemy hits → enemy defeat and removal from state
- [ ] Player HP reaches 0 → death state triggered
- [ ] Checkpoint reached → checkpoint state recorded
- [ ] Death after checkpoint → respawn at checkpoint position

#### Save and Load
- [ ] Save game serializes all required state fields
- [ ] Load game restores position, inventory, score, scene
- [ ] Corrupt/missing save handled gracefully without panic

---

### Layer 3 — End-to-End Browser Tests
**Location:** `/tests/e2e/playwright/`
**Runner:** Playwright with headless Chromium (and optionally Firefox)
**Prerequisite:** WASM artifact built and served locally
**Coverage target:** Deployment correctness and browser integration only

Must cover:
- [ ] Page loads without JS errors in console
- [ ] Canvas element is present and non-zero size
- [ ] WASM module loads and initializes within timeout
- [ ] Game reaches title screen (detect via DOM attribute or canvas hash)
- [ ] Keyboard Enter key reaches game and triggers state change
- [ ] Arrow keys navigate menu selection
- [ ] New game starts successfully
- [ ] Player character responds to movement input on canvas
- [ ] Pause and resume works
- [ ] Game over screen appears after scripted death sequence

Keep this suite small. Prioritize startup, input wiring, and one full
smoke path. Do not replicate logic already covered in integration tests.

---

### Layer 4 — Visual / Golden Tests (Optional)
**Location:** `/tests/golden/`
**When to use:** Only for stable, high-value screens where layout regressions are costly

Prefer **state-model assertions** over image snapshots wherever possible:
- assert `menu.SelectedIndex == 2` not "Options text is at pixel 240,180"

Use image snapshots only for:
- [ ] Title screen layout
- [ ] HUD layout at known state (full HP, 3 lives, score 0)
- [ ] Game over screen
- [ ] One representative gameplay frame (stable tilemap, no animation)

When using image snapshots:
- Store golden images in `/tests/golden/fixtures/`
- Use a tolerance threshold to avoid platform font-rendering flakiness
- Gate image snapshot tests behind a build tag: `//go:build golden`

---

## What NOT to Do

The following patterns will make the game untestable and MUST be avoided:

- [ ] DO NOT store game decisions inside `Draw()` methods
- [ ] DO NOT read `ebiten.IsKeyPressed` directly in game logic
- [ ] DO NOT use `time.Now()` or `time.Since()` in update logic
- [ ] DO NOT use `math/rand` global state without a seeded source
- [ ] DO NOT use package-level mutable singletons for game state
- [ ] DO NOT mix scene transition logic with rendering code
- [ ] DO NOT make `Update()` depend on the real frame rate without abstraction

---

## CI Pipeline Definition

### Fast Pipeline — runs on every push and PR
```
go vet ./...
go test -race ./internal/...
go test -race ./tests/integration/...
staticcheck ./...
```

### Slow Pipeline — runs on merge to main or release branch
```
GOOS=js GOARCH=wasm go build -o web/game.wasm ./cmd/game
npx playwright test --project=chromium
npx playwright test --project=firefox
go test -tags golden ./tests/golden/...   # if golden tests are enabled
```

---

## Acceptance Checklist for Scaffolding

Before considering the test infrastructure complete, verify:

- [ ] `go test ./internal/...` passes with zero engine dependencies
- [ ] `go test ./tests/integration/...` passes headlessly with no browser
- [ ] All menu navigation paths are covered by integration tests
- [ ] At least one full gameplay scenario (start → move → pickup → death → respawn) is scripted
- [ ] Save/load round-trip is covered
- [ ] WASM build succeeds with `GOOS=js GOARCH=wasm go build`
- [ ] Playwright smoke suite passes in headless Chromium
- [ ] CI pipelines are defined and passing
- [ ] No test imports `github.com/hajimehoshi/ebiten/v2` directly

---

## Summary

| Layer        | Location                   | Runner                  | Speed    | Target Coverage                     |
|--------------|----------------------------|-------------------------|----------|-------------------------------------|
| Unit         | `/internal/*/_test.go`     | `go test`               | Fast     | All pure logic functions            |
| Integration  | `/tests/integration/`      | `go test` + harness     | Fast     | All navigation and gameplay flows   |
| E2E          | `/tests/e2e/playwright/`   | Playwright + WASM build | Slow     | Boot, input wiring, smoke path      |
| Golden       | `/tests/golden/`           | `go test -tags golden`  | Slow     | Stable UI screen layout snapshots   |

**Ratio target:** ~65% unit · ~30% integration · ~5% E2E/golden