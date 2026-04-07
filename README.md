## TANK! — Python / PyGame port

**Version 0.6.6** — Python 3 / PyGame recreation of the classic two-player **TANK!** game from *Cursor Magazine* #26 (Commodore PET 4016). Core gameplay, UI, audio, and AI opponents are in place.

### Features

- **Two-player** local play on a shared keyboard: **left tank** — WASD + Left Ctrl (fire); **right tank** — numpad (8/4/6/2 move, 0 fire). Movement is simultaneous (not turn-based).
- **VS AI** — play against computer-controlled tanks with selectable skill and other options from the game menus.

### Requirements

- Python **3.10+**
- Install from the repo root so **pygame** and **py_trees** are available:

  ```bash
  pip install -e .
  ```

  For pytest, Ruff, mypy, and other dev tools: `pip install -e ".[dev]"`.

### Running

```bash
python -m tank_game.main
```

`main` checks for `py_trees` before loading the game and prints install hints if dependencies are missing.

### Project layout

| Path | Contents |
|------|----------|
| `tank_game/` | Game code: controller, board, players, projectiles, AI, rendering, audio, UI. |
| `tests/` | Automated tests for logic, AI, and tournament harnesses. |
| `scripts/` | Helper scripts (e.g. offline benchmarks). |
| `docs/` | Design notes, PRDs, QA spec, backlog. |
| `docs/MECHANICS_TANK_PYGAME.md` | Mechanics reference for this port. |
| `CHANGELOG.md` | Version history and release notes. |

### Documentation index

- **`CHANGELOG.md`** — what changed in each release.
- **`docs/MECHANICS_TANK_PYGAME.md`** — how the port behaves compared to the original.
- **`docs/PRD_py_trees_integration.md`**, **`docs/PRD_AI_NAVIGATION.md`** — AI behaviour and navigation (for contributors).

### License / assets

See `pyproject.toml` for package metadata. Original *TANK!* concept credits Commodore PET / *Cursor* era; this repository is the Python port’s source tree.

### REGRESSION TESTING

Details below are for **benchmarking, automated tests, and AI work** — not needed to play the game.

**SHOWDOWN** runs many AI-vs-AI matches from the menu: **random pairings** (set total match count) or **full skill matrix** (every skill 0–9 vs every skill, with configurable rounds per pairing). Matrix summaries can include head-to-head tables.

**Headless SHOWDOWN** uses a simulation clock and a **dashboard** (progress, ETA, sim/wall metrics) instead of the playfield. Work is **chunked** with a per-frame **wall-clock budget** so the window stays responsive (especially on Windows). **ESC** cancels during a batch; **↑/↓** adjust CPU budget on the dashboard.

**Parallel SHOWDOWN:** set **`TANK_WORKERS=N`** before launch to run matches in **N** worker processes (`0` = sequential). Workers skip per-match file I/O; the main process writes one summary at the end. Tunables include timeouts and retries (see table).

Decisions use a **py_trees** behaviour tree (`ai_tree`, `ai_behaviours`, `ai_decorators`) with navigation memory, peripheral vision, and skill-scaled tuning. **`TANK_LEGACY_AI=1`** selects the legacy monolithic AI for comparison.

**Environment variables**

| Variable | Role |
|----------|------|
| `TANK_LEGACY_AI` | `1` — legacy AI; unset — behaviour tree (default). |
| `TANK_DEBUG` | When enabled, extra logging and BT tip lines to `tank_debug.log`. Use `0`, `false`, `no`, or `off` to disable (the string `"0"` alone is **not** treated as off). |
| `TANK_WORKERS` | Parallel SHOWDOWN worker count (`0` = sequential). |
| `TANK_SHOWDOWN_MAX_SIM_S` / `TANK_SHOWDOWN_MAX_STEPS` / `TANK_SHOWDOWN_MAX_WALL_S` | Per-match limits for parallel workers (defaults in code; see changelog). |
| `TANK_SHOWDOWN_MAX_RETRIES` | Retries after timeout before recording failure. |
| `TANK_VSYNC` | `0` / `false` / `no` / `off` — disable vsync if the display stack limits throughput. |
| `SDL_VIDEODRIVER` | Tests often use `dummy` (see `tests/conftest.py`). |

Headless budgets, inner poll intervals, and main-loop FPS: `tank_game/constants.py`. Full behaviour and defaults: **`CHANGELOG.md`**.

**Automated checks**

Headless tests use `SDL_VIDEODRIVER=dummy` where appropriate. Logic vs rendering: `tank_game/game_loop.tick_logic` / `render_frame`.

```bash
pytest tests
ruff check tank_game tests
mypy tank_game
```

Broader QA checklist: **`docs/PYGAME_QA_TESTING_SPEC.md`**.

**AI development**

- Register new behaviour leaves in `ai_behaviours.py` and wire them in `build_ai_tree()` (selector order = priority). See **`docs/PRD_py_trees_integration.md`**.
- Combat context is synced in `AIPlayer.update()` (`_sync_combat_perception`); use `_combat_enemy_pos` for aim-related leaves where appropriate.
- Navigation: `tank_game/ai_navigation.py`, **`docs/PRD_AI_NAVIGATION.md`**.
