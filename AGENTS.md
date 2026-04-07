## Learned User Preferences

- Prefer iterating on written PRDs and design docs before large implementation steps (for example AI architecture changes).
- Cares about AI difficulty having a visible ladder: SHOWDOWN and skill levels 0–9 should separate more clearly than flat marginal win rates near 50%.
- Favors lightweight, classical game-AI approaches (behavior trees, utility-style tuning, perception limits) that fit Python/PyGame, not LLM-based or heavy compute solutions for this project.
- Two-player mode should allow simultaneous movement (not turn-based), unlike the original serial PET flow.
- If appropriate, When you have multiple-choice questions to ask, offer possible suggestions, and use Cursor's Ask UI: the AskQuestion tool which shows an inline form you can click
- Expect verification to include running the installed game entrypoint (editable install, e.g. `pip install -e .`, then `python -m tank_game.main` or the project venv), not only pytest or headless simulation, so import and runtime errors surface before hand-off.

## Learned Workspace Facts

- `W:\CODE\TANK` is the TANK! PyGame port (Python 3.10+); package name in `pyproject.toml` is `tank-game`.
- Application code lives under `tank_game/`; automated tests under `tests/`.
- `py_trees` is integrated as the AI decision layer (`tank_game/ai_tree.py`, `ai_behaviours.py`, `ai_decorators.py`); PRD §11 closed with **FR-8** (skip-fire + risky jitter), **FR-9** (geometric 90° cone), **FR-10** (`effective_min_shots_for_aimed`), **FR-7/11** as before. S2: `TANK_DEBUG` + BT tip line in `tank_debug.log`.
- Optional dev dependencies are declared under `[project.optional-dependencies] dev` in `pyproject.toml` (install with `pip install -e ".[dev]"`): pytest, ruff, mypy, vulture, hypothesis, viztracer.
- Headless SHOWDOWN: keep the window responsive with a per-frame **wall-clock budget** (`GameController._headless_wall_budget_s`, constants `HEADLESS_WALL_BUDGET_*`), **chunked** `run_headless_showdown_batch` in `game_loop.py`, and **`pygame.event.pump()`** so Windows can repaint; **`TANK_DEBUG`** disk logging still makes each `update()` expensive.
- Headless SHOWDOWN **ESC**: cancellation must be handled **inside** the headless batch (`get_pressed`, pump, QUIT drain → `abort_showdown_run()`), not only in `handle_input`, which runs **once per frame before** the batch.
- Default **~50 ms** wall budget is a practical balance; very high budgets (75–100+ ms) hurt UI/input; CPU % may not track budget tweaks much because the main thread often **waits** on display/events rather than saturating CPU.
- Headless **dashboard**: sim time is shown as **whole seconds**; **`MessageOverlay`** caches rendered lines when the message string is unchanged.
- Parallel SHOWDOWN: `tank_game/showdown_worker.py` uses `multiprocessing.Pool` (`spawn` context on Windows); **`TANK_WORKERS`** env var controls worker count (0 = sequential, >0 = parallel). Workers must set `_suppress_summary_file = True` to avoid **Windows Defender minifilter** blocking each process in kernel space on per-match file creates.
- Headless hot-path profiling: **`_run_ai`** is ~97% of per-step CPU (A* pathfinding ~63%, peripheral vision ~21%); `CellType`/`Direction` are **`IntEnum`** (not `Enum`) for hash performance; `NavigationMemory` has an **A* result cache**; `_headless_sim_time_cumulative` tracks tournament-wide sim time (per-match `_headless_sim_time_s` resets each match boundary).
