# Pygame QA & Testing Architecture

This document extends the project’s testing strategy for **agentic coding**, **CI**, and **headless simulation**. It complements `docs/PRD_py_trees_integration.md` (AI behavior) by focusing on **verification infrastructure**.

| Field | Value |
|-------|--------|
| **Status** | Adopted — living document |
| **Last updated** | 2026-04-06 |

---

## 1. Overview

Integrate **automated testing**, **linting**, and **simulation-friendly** layout so an AI agent (or developer) can **build, verify, and maintain** game stability without relying on manual play for every change.

**Priority:** **Headless integration** tests for collision handling, input, and simulation stepping—before visual QA.

---

## 2. Architectural flow

```
[ Logic layer ]     →  Unit tests (pytest) / property tests (Hypothesis)
        ↓
[ Integration layer ] →  Headless pygame (`SDL_*=dummy`), event injection
        ↓
[ Rendering layer ]   →  Manual QA, VizTracer profiling
```

**Project mapping**

| Layer | Location | Tests |
|-------|----------|--------|
| Logic | `tank_game/game.py`, `board.py`, `player.py`, `projectile.py`, `ai.py`, `game_loop.tick_logic` | `tests/test_*.py` |
| Integration | Same modules under dummy SDL; `tick_logic` + injected `pygame.event.Event` | `tests/test_headless_smoke.py`, future `test_*_integration.py` |
| Rendering | `tank_game/graphics.py`, `game_loop.render_frame`, `main.main` | Manual; optional screenshot tests later |

**Decoupled loop:** `tank_game/game_loop.py` exposes:

- **`tick_logic(controller, events, dt)`** — no drawing; use for fast, deterministic tests.
- **`render_frame(...)`** — pygame draws only.

`main.main()` composes: read events → `tick_logic` → `render_frame` → `flip`.

---

## 3. Static analysis & linting stack

Configured in **`pyproject.toml`**:

| Tool | Role |
|------|------|
| **Ruff** | Lint + format (fast feedback). |
| **mypy** | Type-check; use **stricter** rules for new coordinate math (prefer **`pygame.Vector2`** with explicit types when adding physics). |
| **Vulture** | Optional pass for **dead code** in experimental mechanics (`vulture tank_game` from dev env). |

Run locally:

```bash
pip install -e ".[dev]"
ruff check tank_game tests
ruff format tank_game tests
mypy tank_game
vulture tank_game --min-confidence 80
```

---

## 4. Automated testing strategies

### 4.A Headless simulation (CI / agents)

Before importing pygame in tests, set:

- `SDL_VIDEODRIVER=dummy`
- `SDL_AUDIODRIVER=dummy`

**Project:** `tests/conftest.py` sets these by default so `pytest` is safe without a display.

### 4.B Decoupled logic

- **Simulation:** Prefer calling **`tick_logic(..., dt=...)`** with fixed `dt` instead of real-time `Clock.tick`.
- **Rendering:** Not required for logic/regression tests on `GameController` state.

### 4.C Event injection

Use **`pygame.event.Event`** + **`pygame.event.post()`** to simulate input:

```python
import pygame
pygame.event.post(pygame.event.Event(pygame.KEYDOWN, key=pygame.K_RETURN))
```

Assert on **`GameController`** / tank state after **`tick_logic`**.

### 4.D Property-based tests (optional)

**Hypothesis** (dev dependency) is for **procedural** aspects (e.g. board generation seeds, invariants). Add targeted tests under `tests/` when those systems grow.

---

## 5. Performance & stability tools

| Tool | Use |
|------|-----|
| **VizTracer** | Profile the main loop; find frames **> 16.6 ms** (`viztracer python -m tank_game.main`). |
| **Hypothesis** | Invariants over random-but-valid inputs (see §4.D). |
| **`python -m pygame.tests`** | Verify local pygame install (run manually or in CI when debugging env issues). |

---

## 6. Agent implementation checklist

- [x] Configure `pyproject.toml` with **Ruff** and **mypy** (and dev deps: **vulture**, **hypothesis**, **viztracer**).
- [x] `tests/conftest.py` for headless SDL initialization.
- [x] Smoke test: pygame **`init`** succeeds under dummy drivers (`tests/test_headless_smoke.py`).
- [x] Main loop logic accepts injectable **`dt`** via **`tick_logic`** (`tank_game/game_loop.py`).

**Follow-ups (optional)**

- [ ] Expand integration tests: menu transitions via **event.post**.
- [ ] Narrow **mypy** `strict` to modules that use **`Vector2`** / coordinate-heavy code.
- [ ] CI workflow (GitHub Actions / other) running `pytest`, `ruff`, `mypy`.

---

## 7. Revision history

| Date | Change |
|------|--------|
| 2026-04-06 | Initial spec; wired `game_loop`, `conftest`, smoke tests, `pyproject` tools |
