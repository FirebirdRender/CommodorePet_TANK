## TANK! — Python / PyGame port

**Version 0.6.5** — Python 3 / PyGame recreation of the classic two-player **TANK!** game from *Cursor Magazine* #26 (Commodore PET 4016). Core gameplay, UI, audio, and AI are implemented; **SHOWDOWN** tournaments (including headless runs and optional multiprocessing) support benchmarking and skill tuning.

### Features

- **Two-player** local play on a shared keyboard: **left tank** — WASD + Left Ctrl (fire); **right tank** — numpad (8/4/6/2 move, 0 fire). Movement is simultaneous (not turn-based).
- **SHOWDOWN** — run many AI-vs-AI matches: **random pairings** (set total match count) or **full skill matrix** (every skill 0–9 vs every skill, with configurable rounds per pairing). Summary output includes head-to-head tables for matrix runs.
- **Headless SHOWDOWN** — optional mode that drives matches from a **simulation clock** and shows a **dashboard** (progress, ETA, sim/wall metrics) instead of the playfield. Work is **chunked** with a per-frame **wall-clock budget** so the window stays responsive (important on Windows). **ESC** cancels during a batch; **↑/↓** adjust CPU budget on the dashboard.
- **Parallel SHOWDOWN** — set **`TANK_WORKERS=N`** before launch to run matches in **N** worker processes (0 = sequential). Workers avoid per-match file I/O overhead; the main process writes one summary at the end. Tunables include timeouts and retries (see environment variables below).
- **AI** — decisions go through a **py_trees** behaviour tree (`ai_tree`, `ai_behaviours`, `ai_decorators`) with navigation memory (A*, tabu edges, peripheral vision), perception, and skill-scaled tuning. Set **`TANK_LEGACY_AI=1`** to use the legacy monolithic AI for comparison.

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

### Environment variables (quick reference)

| Variable | Role |
|----------|------|
| `TANK_LEGACY_AI` | `1` — legacy AI; unset — behaviour tree (default). |
| `TANK_DEBUG` | When enabled, extra logging and BT tip lines to `tank_debug.log`. Use `0`, `false`, `no`, or `off` to disable (the string `"0"` alone is **not** treated as off). |
| `TANK_WORKERS` | Parallel SHOWDOWN worker count (`0` = sequential). |
| `TANK_SHOWDOWN_MAX_SIM_S` / `TANK_SHOWDOWN_MAX_STEPS` / `TANK_SHOWDOWN_MAX_WALL_S` | Per-match limits for parallel workers (defaults in code; see changelog). |
| `TANK_SHOWDOWN_MAX_RETRIES` | Retries after timeout before recording failure. |
| `TANK_VSYNC` | `0` / `false` / `no` / `off` — disable vsync if the display stack limits throughput. |
| `SDL_VIDEODRIVER` | Tests often use `dummy` (see `tests/conftest.py`). |

Constants for headless budgets, inner poll intervals, and main-loop FPS live in `tank_game/constants.py`. Full behaviour and defaults are described in **`CHANGELOG.md`** and the design docs below.

### Testing and quality

Headless tests use `SDL_VIDEODRIVER=dummy` where appropriate. Logic vs rendering split: `tank_game/game_loop.tick_logic` / `render_frame`.

```bash
pytest tests
ruff check tank_game tests
mypy tank_game
```

Broader QA checklist: **`docs/PYGAME_QA_TESTING_SPEC.md`**.

### AI development notes

- **Behaviour tree:** Register new leaves by subclassing patterns in `ai_behaviours.py` and wiring them in `build_ai_tree()` (selector order = priority). See **`docs/PRD_py_trees_integration.md`**.
- **Perception:** Combat context is synced in `AIPlayer.update()` (`_sync_combat_perception`); use `_combat_enemy_pos` for aim-related leaves where appropriate.
- **Navigation:** `tank_game/ai_navigation.py`, **`docs/PRD_AI_NAVIGATION.md`**.

### Project layout

| Path | Contents |
|------|----------|
| `tank_game/` | Game code: controller, board, players, projectiles, AI, rendering, audio, UI, headless SHOWDOWN batching, worker entry. |
| `tests/` | Automated tests (board, projectiles, AI, SHOWDOWN matrix/worker, headless harnesses). |
| `scripts/` | e.g. offline headless benchmark helper. |
| `docs/` | PRDs, QA spec, UI notes, backlog. |
| `docs/MECHANICS_TANK_PYGAME.md` | Mechanics reference for this port. |
| `CHANGELOG.md` | Version history and detailed release notes. |

### Documentation index

- **`CHANGELOG.md`** — release notes (through **0.6.5**; recent entries cover headless/parallel SHOWDOWN tuning, worker timeouts, debug env fix, stats fixes, etc.).
- **`docs/PRD_py_trees_integration.md`** — behaviour-tree AI requirements and parity notes.
- **`docs/PRD_AI_NAVIGATION.md`** — navigation memory and pathing.
- **`docs/PYGAME_QA_TESTING_SPEC.md`** — testing and tooling expectations.
- **`docs/BACKLOG_TANK_PYGAME.md`** — follow-up ideas (if present).

### License / assets

See `pyproject.toml` for package metadata. Original *TANK!* concept credits Commodore PET / *Cursor* era; this repository is the Python port’s source tree.
