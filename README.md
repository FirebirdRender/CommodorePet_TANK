## TANK! 2P — Two-Player PyGame Port

**Version 1.0.0** — Python 3 / PyGame recreation of the classic two-player **TANK!** game from *Cursor Magazine* #26 (Commodore PET 4016). **Two-player only** — no AI, no tournament mode. Authentic **PETSCII retro** rendering with bundled PET font, monochrome phosphor-green palette, and character-cell visuals matching the original Commodore PET look.

### Features

- **Two-player** local play on a shared keyboard: **left tank** — WASD + Left Ctrl (fire); **right tank** — numpad (8/4/6/2 move, 0 fire). Simultaneous movement.
- **Skill levels 1–10** — terrain density, ammo, mines, and tank speed scale with difficulty.
- **PETSCII retro visuals** — all game elements (border, walls, tanks, shots, mines, explosions, HUD) rendered as Commodore PET character glyphs via bundled PetMe64 font. Monochrome phosphor-green palette. Optional CRT scanline effect (`TANK_CRT=1`).

### Requirements

- Python **3.10+**
- Install from the repo root:

  ```bash
  pip install -e .
  ```

  Only **pygame** and **cbmcodecs2** are required (no AI dependencies).

  For pytest, Ruff, mypy, and other dev tools: `pip install -e ".[dev]"`.

### Running

```bash
python -m tank_game.main
```

### Menu flow

**MENU** → **SKILL SELECT** (1–10) → **PLAYING** → winner shown → **ANOTHER BATTLE? Y/N** → Y restarts at same skill, N returns to MENU.

### Project layout

| Path | Contents |
|------|----------|
| `tank_game/` | Game code: controller, board, players, projectiles, rendering, audio, UI. |
| `tests/` | Automated tests for board, player, projectile, and game logic. |
| `docs/` | Design notes, mechanics reference. |
| `docs/MECHANICS_TANK_PYGAME.md` | Mechanics reference for this port. |
| `CHANGELOG.md` | Version history and release notes. |

### Environment variables

| Variable | Role |
|----------|------|
| `TANK_DEBUG` | Extra logging to `tank_debug.log`. Use `0` / `false` / `no` / `off` to disable. |
| `TANK_CRT` | `1` — enable CRT scanline overlay effect. |
| `TANK_VSYNC` | `0` / `false` / `no` / `off` — disable vsync. |
| `SDL_VIDEODRIVER` | Tests use `dummy` (see `tests/conftest.py`). |

### Testing

```bash
pytest tests
ruff check tank_game tests
mypy tank_game
```

### License / assets

See `pyproject.toml` for package metadata. Original *TANK!* concept credits Commodore PET / *Cursor* era; this repository is the Python port's source tree.
