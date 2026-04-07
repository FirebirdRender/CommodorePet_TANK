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
BASE_SHOTS_PER_LEVEL: int = 5  # SS ≈ 5 × difficulty
BASE_MINES_PER_LEVEL: int = 1  # mines scale roughly with difficulty
DIFFICULTY_MIN: int = 0  # 0 = easiest
DIFFICULTY_MAX: int = 9  # 9 = hardest

# Timing (seconds)
EXPLOSION_DURATION: float = 0.5
MINE_DETONATION_DELAY: float = 0.1

# Movement delay (scales with difficulty - harder = faster)
# At skill 0: 1.0 seconds between moves
# At skill 9: 0.1 seconds between moves
MOVE_DELAY_BASE: float = 1.0
MOVE_DELAY_MIN: float = 0.1

# Shot delay (scales with difficulty - harder = faster)
# At skill 0: 0.5 seconds between cell moves
# At skill 9: 0.05 seconds between cell moves
SHOT_DELAY_BASE: float = 0.5
SHOT_DELAY_MIN: float = 0.05

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
    """Calculate movement delay based on difficulty.

    Higher difficulty = faster movement (shorter delay).
    Linear interpolation from BASE at level 0 to MIN at level 9.
    """
    level = max(DIFFICULTY_MIN, min(DIFFICULTY_MAX, difficulty))
    # Linear: level 0 -> 1.0s, level 9 -> 0.1s
    ratio = level / DIFFICULTY_MAX  # 0.0 to 1.0
    return MOVE_DELAY_BASE - ratio * (MOVE_DELAY_BASE - MOVE_DELAY_MIN)


def get_shot_delay(difficulty: int) -> float:
    """Calculate shot delay based on difficulty.

    Higher difficulty = faster shots (shorter delay).
    Linear interpolation from BASE at level 0 to MIN at level 9.
    """
    level = max(DIFFICULTY_MIN, min(DIFFICULTY_MAX, difficulty))
    # Linear: level 0 -> 0.5s, level 9 -> 0.05s
    ratio = level / DIFFICULTY_MAX  # 0.0 to 1.0
    return SHOT_DELAY_BASE - ratio * (SHOT_DELAY_BASE - SHOT_DELAY_MIN)


# Colors (RGB)
COLOR_BG = (0, 0, 0)
COLOR_GRID = (0, 80, 0)
COLOR_TANK_1 = (255, 255, 0)
COLOR_TANK_2 = (255, 0, 0)
COLOR_SHOT = (255, 255, 255)
COLOR_MINE = (160, 160, 160)
COLOR_TEXT = (0, 255, 0)
COLOR_STATUS_BG = (0, 0, 0)
COLOR_WRECKAGE = (80, 80, 80)  # Gray for destroyed tanks

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


def difficulty_to_resources(_level: int) -> tuple[int, int, int]:
    """Map difficulty level to (tanks, shots, mines) per player.

    Original TANK! mechanic: 6 shots per life, 1 mine per life.
    Difficulty affects terrain density (separate from resources).
    """
    tanks = INITIAL_TANKS
    # Original: 6 shots per life/tank
    shots = 6 * tanks
    # Original: 1 mine per life/tank
    mines = tanks
    return tanks, shots, mines
