## Learned User Preferences

- Prefer iterating on written PRDs and design docs before large implementation steps (for example AI architecture changes).
- Cares about AI difficulty having a visible ladder: SHOWDOWN and skill levels 0–9 should separate more clearly than flat marginal win rates near 50%.
- Favors lightweight, classical game-AI approaches (behavior trees, utility-style tuning, perception limits) that fit Python/PyGame, not LLM-based or heavy compute solutions for this project.
- Two-player mode should allow simultaneous movement (not turn-based), unlike the original serial PET flow.
- If appropriate, When you have multiple-choice questions to ask, offer possible suggestions, and use Cursor's Ask UI: the AskQuestion tool which shows an inline form you can click
- Expect verification to include running the installed game entrypoint (editable install, e.g. `pip install -e .`, then `python -m tank_game.main` or the project venv), not only pytest or headless simulation, so import and runtime errors surface before hand-off.
- On the `retro` branch, original PET program behavior and screenshots are the authoritative reference for visual and gameplay decisions.

## Learned Workspace Facts

- `W:\CODE\TANK` is the TANK! PyGame port (Python 3.10+); package name in `pyproject.toml` is `tank-game`.
- Application code lives under `tank_game/`; automated tests under `tests/`.
- `py_trees` is integrated as the AI decision layer (`tank_game/ai_tree.py`, `ai_behaviours.py`, `ai_decorators.py`); PRD §11 closed with **FR-8** (skip-fire + risky jitter), **FR-9** (geometric 90° cone), **FR-10** (`effective_min_shots_for_aimed`), **FR-7/11** as before. S2: `TANK_DEBUG` + BT tip line in `tank_debug.log`.
- Optional dev dependencies are declared under `[project.optional-dependencies] dev` in `pyproject.toml` (install with `pip install -e ".[dev]"`): pytest, ruff, mypy, vulture, hypothesis, viztracer.
- Headless SHOWDOWN: per-frame **wall-clock budget** (`_headless_wall_budget_s`, ~50 ms default); cancellation handled **inside** the headless batch (`get_pressed`, pump, QUIT drain → `abort_showdown_run()`), not in `handle_input`; very high budgets (75–100+ ms) hurt UI/input; `TANK_DEBUG` disk logging makes each `update()` expensive.
- Headless **dashboard**: sim time is shown as **whole seconds**; **`MessageOverlay`** caches rendered lines when the message string is unchanged.
- Parallel SHOWDOWN: `tank_game/showdown_worker.py` uses `multiprocessing.Pool` (`spawn` context on Windows); **`TANK_WORKERS`** env var controls worker count (0 = sequential, >0 = parallel). Workers must set `_suppress_summary_file = True` to avoid **Windows Defender minifilter** blocking each process in kernel space on per-match file creates.
- Headless hot-path profiling: **`_run_ai`** is ~97% of per-step CPU (A* pathfinding ~63%, peripheral vision ~21%); `CellType`/`Direction` are **`IntEnum`** (not `Enum`) for hash performance; `NavigationMemory` has an **A* result cache**; `_headless_sim_time_cumulative` tracks tournament-wide sim time (per-match `_headless_sim_time_s` resets each match boundary).
- GitHub remote: `https://github.com/FirebirdRender/CommodorePet_TANK.git` (branch `main`); `tasks/` is gitignored.
- Active **`retro`** branch for PETSCII retro-UI overhaul; spec is `docs/UI_RETRO_SPEC.md`; adds `cbmcodecs2`; rendering uses cached per-cell glyph blits (`petscii_render.py`, `petscii_map.py`); P1 body = inverted `*`, P2 body = inverted `#`; menu/selection screens render on clean black background (no playfield) via `_MENU_STATES` in `game_loop.py`; post-match summary exits on Space/Return/ESC.
- Retro corrections (active on `retro` branch): difficulty-scaled terrain density (4%–25%), constant projectile speed (~0.1s/cell) with 75% max range, barrel-curl mechanic (single-player respawn, no explosion), per-difficulty mine/ammo tables, tank speed calibration (0.3s base → 0.2s at level 10), multi-cell dice wreckage, per-player HUD status messages ("LOW SHOTS", "OUT OF SHOTS", "LAST TANK", "THE WINNER").
- Collision rules (retro branch): barrel hits only from direct projectile contact (not explosion splash); projectile→terrain removes wall only (no splash); own barrel doesn't block own tank movement or own shots; respawn uses `_find_spawn_pos()` to avoid wreckage.
