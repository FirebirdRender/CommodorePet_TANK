from __future__ import annotations

# Logical board dimensions (Commodore PET: 40×25 character grid)
SCREEN_WIDTH_CELLS: int = 40
SCREEN_HEIGHT_CELLS: int = 25

# Pixel scaling
CELL_SIZE: int = 20

# Status bar / UI offset (must be defined before WINDOW_HEIGHT)
STATUS_BAR_HEIGHT: int = 40  # Pixels reserved for status bar at top
BOARD_OFFSET_Y: int = STATUS_BAR_HEIGHT  # Game board starts below status bar

# Window dimensions
WINDOW_WIDTH: int = SCREEN_WIDTH_CELLS * CELL_SIZE
WINDOW_HEIGHT: int = BOARD_OFFSET_Y + (SCREEN_HEIGHT_CELLS * CELL_SIZE)

# Resources
INITIAL_TANKS: int = 3
DIFFICULTY_MIN: int = 0  # 0 = easiest  (UI shows 1-10, internally 0-9)
DIFFICULTY_MAX: int = 9  # 9 = hardest

# Timing (seconds)
EXPLOSION_DURATION: float = 0.5
MINE_DETONATION_DELAY: float = 0.1

# Movement delay: calibrated to original PET timing (~12s to cross 40 cells at fastest keypress).
# Base = 0.3s/cell (100%); scales to 0.2s/cell (150%) at difficulty 9.
MOVE_DELAY_BASE: float = 0.3
MOVE_DELAY_MIN: float = 0.2

# Shot delay: constant speed regardless of difficulty.
# ~0.1s per cell → 3s to cross 75% of 40-cell width, ~2s for height.
SHOT_DELAY: float = 0.1

# Headless SHOWDOWN: wall time per frame for the sim batch (runtime ↑/↓ on dashboard).
# Default 50 ms balances sim throughput vs UI/event responsiveness on typical displays.
HEADLESS_WALL_BUDGET_DEFAULT_S: float = 0.050
HEADLESS_WALL_BUDGET_MIN_S: float = 0.001
HEADLESS_WALL_BUDGET_MAX_S: float = 0.250
HEADLESS_WALL_BUDGET_STEP_S: float = 0.002

# Inside ``run_headless_showdown_batch``, poll SDL / ESC only every N sim steps — **not** every
# step. Per-step ``pygame.event.pump()`` dominates CPU and caps sim rate (~few×) regardless of
# wall budget or main-loop FPS.
HEADLESS_INNER_POLL_INTERVAL_STEPS: int = 64

# Main loop: normal play caps at 60 FPS. Headless SHOWDOWN raises this so ``tick_logic`` /
# ``run_headless_showdown_batch`` run more often per wall second (otherwise ~60 Hz caps sim rate).
MAIN_LOOP_FPS: int = 60
# Pygame: ``Clock.tick(0)`` does not cap framerate (no delay between frames).
HEADLESS_SHOWDOWN_MAIN_LOOP_FPS: int = 240

# Parallel SHOWDOWN worker: abort a single match attempt if any limit is hit (then retry with a
# new seed). Tuned so normal matches (~5–30 s sim) finish easily; stuck / oscillating games exit.
SHOWDOWN_WORKER_MAX_SIM_TIME_S: float = 600.0  # simulated seconds per attempt
SHOWDOWN_WORKER_MAX_STEPS: int = 2_000_000  # hard cap on update() iterations per attempt
SHOWDOWN_WORKER_MAX_WALL_S: float = 180.0  # wall-clock seconds per attempt (safety)
SHOWDOWN_WORKER_MAX_RETRIES: int = 5  # per match; re-run with new seed after timeout


def get_move_delay(difficulty: int) -> float:
    """Movement delay: 0.3s at level 0 down to 0.2s at level 9 (linear)."""
    level = max(DIFFICULTY_MIN, min(DIFFICULTY_MAX, difficulty))
    ratio = level / DIFFICULTY_MAX  # 0.0 to 1.0
    return MOVE_DELAY_BASE - ratio * (MOVE_DELAY_BASE - MOVE_DELAY_MIN)


def get_shot_delay(_difficulty: int) -> float:
    """Shot delay is constant (not affected by difficulty)."""
    return SHOT_DELAY


# Colors (RGB) — PET phosphor-green monochrome palette (UI_RETRO_SPEC §2)
COLOR_BG = (0, 0, 0)
COLOR_PET_FG: tuple[int, int, int] = (51, 255, 51)  # P1 phosphor green
COLOR_GRID = COLOR_PET_FG
COLOR_TANK_1 = COLOR_PET_FG
COLOR_TANK_2 = COLOR_PET_FG
COLOR_SHOT = COLOR_PET_FG
COLOR_MINE = COLOR_PET_FG
COLOR_TEXT = COLOR_PET_FG
COLOR_STATUS_BG = (0, 0, 0)
COLOR_WRECKAGE = (20, 100, 20)  # Dimmer green for destroyed tanks
COLOR_EXPLOSION = COLOR_PET_FG
COLOR_EXPLOSION_CHAIN = (80, 255, 80)  # Slightly brighter for catalyst blasts

# Key bindings (per player) — built lazily to avoid importing pygame/SDL in
# headless worker processes.  Access via module attribute (e.g.
# ``from .constants import PLAYER1_KEYS``) which triggers ``__getattr__``.
_PLAYER_KEYS_CACHE: dict[str, dict[str, int]] | None = None


def _build_player_keys() -> dict[str, dict[str, int]]:
    import pygame

    return {
        "PLAYER1_KEYS": {
            "up": pygame.K_w,
            "down": pygame.K_x,
            "left": pygame.K_a,
            "right": pygame.K_d,
            "up_left": pygame.K_q,
            "up_right": pygame.K_e,
            "down_left": pygame.K_z,
            "down_right": pygame.K_c,
            "fire": pygame.K_s,
            "mine": pygame.K_t,
        },
        "PLAYER2_KEYS": {
            "up": pygame.K_KP8,
            "down": pygame.K_KP2,
            "left": pygame.K_KP4,
            "right": pygame.K_KP6,
            "up_left": pygame.K_KP7,
            "up_right": pygame.K_KP9,
            "down_left": pygame.K_KP1,
            "down_right": pygame.K_KP3,
            "fire": pygame.K_KP5,
            "mine": pygame.K_KP0,
        },
    }


def __getattr__(name: str) -> object:
    global _PLAYER_KEYS_CACHE
    if name in ("PLAYER1_KEYS", "PLAYER2_KEYS"):
        if _PLAYER_KEYS_CACHE is None:
            _PLAYER_KEYS_CACHE = _build_player_keys()
        return _PLAYER_KEYS_CACHE[name]
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")


def difficulty_to_resources(level: int) -> tuple[int, int, int]:
    """Map difficulty level (0-9) to (tanks, shots, mines) per player.

    Tables derived from original PET TANK! (Cursor #26).
    UI displays 1-10; internal level = UI - 1 (0-9).
    """
    tanks = INITIAL_TANKS
    # Shots: 0-1→6, 2-4→8, 5-7→10, 8-9→12
    if level <= 1:
        shots = 6
    elif level <= 4:
        shots = 8
    elif level <= 7:
        shots = 10
    else:
        shots = 12
    # Mines: 0→0, 1-3→1, 4-6→2, 7-9→3
    if level <= 0:
        mines = 0
    elif level <= 3:
        mines = 1
    elif level <= 6:
        mines = 2
    else:
        mines = 3
    return tanks, shots, mines
