## TANK! – Python/PyGame Port

Python 3 / PyGame recreation of the classic two-player **TANK!** game from Cursor Magazine #26 (Commodore PET 4016).

### Requirements

- Python 3.10+
- **Install dependencies before running** (otherwise imports such as `py_trees` will fail):

  ```bash
  pip install -e .
  ```

  That reads `pyproject.toml` and installs **pygame** and **py_trees** (required for AI). For pytest/ruff/mypy as well, use `pip install -e .[dev]`.

### Running the Game

```bash
python -m tank_game.main
```

### Testing & QA tooling

Install dev dependencies: `pip install -e .[dev]`. Headless pygame for tests uses `SDL_VIDEODRIVER=dummy` (see `tests/conftest.py`). Logic vs rendering split: `tank_game.game_loop.tick_logic` / `render_frame`. Full checklist: **`docs/PYGAME_QA_TESTING_SPEC.md`**.

**AI decisions** use **py_trees** (`tank_game/ai_tree.py`, `tank_game/ai_behaviours.py`). Set `TANK_LEGACY_AI=1` to use the legacy monolithic decision function for A/B debugging. See **`docs/PRD_py_trees_integration.md`**.

**Adding a behaviour:** subclass `_SetsPendingAction` in `ai_behaviours.py`, register the node in `build_ai_tree()` (selector order = priority). Perception/combat context is synced in `AIPlayer.update()` (`_sync_combat_perception`); use `_combat_enemy_pos` for aim-related leaves, not raw oracle position.

```bash
pytest tests
ruff check tank_game tests
mypy tank_game
```

### Project Layout

- `tank_game/` – Game code (controller, board, tanks, projectiles, rendering, audio, UI, constants).
- `tests/` – Automated tests for core logic (board, tanks, projectiles, win conditions).
- `docs/` – Design and planning documents (e.g. `PRD_py_trees_integration.md` for behavior-tree AI).
- `MECHANICS_TANK_PYGAME.md` – Detailed mechanics reference for the Python port.

### Status

- Core architecture and scaffolding are in place.
- Gameplay systems are implemented incrementally following the plan in `.cursor/plans/tank-pygame-port-plan_*.plan.md`.

