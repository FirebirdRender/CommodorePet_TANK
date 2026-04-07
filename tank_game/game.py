from __future__ import annotations

import os
import random
import threading
import time
from dataclasses import dataclass
from datetime import datetime
from enum import Enum, auto

from .ai import AIAction, AIPlayer
from .board import Board, CellType
from .constants import (
    HEADLESS_WALL_BUDGET_DEFAULT_S,
    HEADLESS_WALL_BUDGET_MAX_S,
    HEADLESS_WALL_BUDGET_MIN_S,
    HEADLESS_WALL_BUDGET_STEP_S,
    difficulty_to_resources,
    get_move_delay,
    get_shot_delay,
)
from .player import DIRECTION_VECTORS, Direction, Tank
from .projectile import Mine, Shot


def _env_workers() -> int:
    """Read ``TANK_WORKERS`` env var. 0 = sequential, >1 = multiprocessing pool."""
    raw = os.environ.get("TANK_WORKERS", "0").strip()
    try:
        n = int(raw)
    except ValueError:
        return 0
    return max(0, n)


def _shots_crossed_head_on(a: Shot, b: Shot) -> bool:
    """True if two cardinal shots swapped cells in one step (discrete head-on pass-through).

    Same-cell occupancy is handled separately. This only catches the case where shots
    started adjacent on the same row/column, moved toward each other, and ended with
    their x/y order reversed — so they never shared a grid cell in the same tick.
    Requires ``_step_start_x`` / ``_step_start_y`` set on both shots before stepping.
    """
    if not (a.active and b.active):
        return False
    sa_x = getattr(a, "_step_start_x", a.x)
    sa_y = getattr(a, "_step_start_y", a.y)
    sb_x = getattr(b, "_step_start_x", b.x)
    sb_y = getattr(b, "_step_start_y", b.y)
    # Horizontal: LEFT/RIGHT on same row
    if sa_y == sb_y and a.y == b.y:
        if not (
            a.direction in (Direction.LEFT, Direction.RIGHT)
            and b.direction in (Direction.LEFT, Direction.RIGHT)
        ):
            return False
        if sa_x == sb_x:
            return False
        left, right = (a, b) if sa_x < sb_x else (b, a)
        lsx = getattr(left, "_step_start_x", left.x)
        rsx = getattr(right, "_step_start_x", right.x)
        if lsx >= rsx:
            return False
        if left.direction != Direction.RIGHT or right.direction != Direction.LEFT:
            return False
        return left.x > right.x
    # Vertical: UP/DOWN on same column
    if sa_x == sb_x and a.x == b.x:
        if not (
            a.direction in (Direction.UP, Direction.DOWN)
            and b.direction in (Direction.UP, Direction.DOWN)
        ):
            return False
        if sa_y == sb_y:
            return False
        upper, lower = (a, b) if sa_y < sb_y else (b, a)
        usy = getattr(upper, "_step_start_y", upper.y)
        lsy = getattr(lower, "_step_start_y", lower.y)
        if usy >= lsy:
            return False
        if upper.direction != Direction.DOWN or lower.direction != Direction.UP:
            return False
        return upper.y > lower.y
    return False


def _shot_shot_explosion_key(shot1: Shot, shot2: Shot, *, crossed: bool) -> tuple[int, int]:
    if not crossed:
        return (shot1.x, shot1.y)
    if shot1.y == shot2.y:
        return ((shot1.x + shot2.x) // 2, shot1.y)
    return (shot1.x, (shot1.y + shot2.y) // 2)


# Debug logging - enabled via TANK_DEBUG env var
DEBUG_LOG_FILE = "tank_debug.log"


# SHOWDOWN matrix: 10 skill levels × 10 skill levels = 100 ordered pairings per round.
SHOWDOWN_SKILL_LEVELS = 10
SHOWDOWN_MATRIX_CELLS = SHOWDOWN_SKILL_LEVELS * SHOWDOWN_SKILL_LEVELS


def build_showdown_matrix_schedule(rounds_per_pairing: int) -> list[tuple[int, int]]:
    """Ordered list of (P1 skill, P2 skill) for exhaustive matrix benchmarking."""
    out: list[tuple[int, int]] = []
    for i in range(SHOWDOWN_SKILL_LEVELS):
        for j in range(SHOWDOWN_SKILL_LEVELS):
            for _ in range(rounds_per_pairing):
                out.append((i, j))
    return out


_DEBUG = os.environ.get("TANK_DEBUG", "").strip().lower() not in ("", "0", "false", "no", "off")


def _debug_log(msg: str) -> None:
    """Write debug message to log file."""
    if not _DEBUG:
        return
    timestamp = datetime.now().strftime("%H:%M:%S.%f")[:-3]
    with open(DEBUG_LOG_FILE, "a") as f:
        f.write(f"[{timestamp}] {msg}\n")


class GameState(Enum):
    MENU = auto()
    GAME_MODE_SELECT = auto()  # NEW: Demo vs 1P vs 2P selection
    SKILL_SELECT = auto()
    SHOWDOWN_SCHEDULE_TYPE = auto()  # SHOWDOWN: random pairings vs full skill matrix
    SHOWDOWN_ITERATIONS = auto()  # Random mode: total match count
    SHOWDOWN_MATRIX_ROUNDS = auto()  # Matrix mode: rounds per (AI-i vs AI-j) pairing
    SHOWDOWN_MATRIX_CONFIRM = auto()  # Matrix mode: confirm total matches before start
    GAME_INIT = auto()
    PLAYING = auto()
    EXPLOSION = auto()
    GAME_OVER = auto()
    PLAY_AGAIN = auto()
    TOURNAMENT_END = auto()
    SHOWDOWN_RUNNING = auto()  # NEW: Running showdown matches
    SHOWDOWN_COMPLETE = auto()  # NEW: Showdown finished
    QUIT = auto()


@dataclass
class Explosion:
    x: int
    y: int
    start_time: float
    duration: float
    is_chain_reaction: bool = False  # True if triggered by another explosion (larger)


class GameController:
    def __init__(self) -> None:
        self.state: GameState = GameState.MENU
        self.game_mode: str = "2P"  # "Demo", "1P", or "2P"
        self.board = Board()
        self.tanks: dict[int, Tank] = {}
        self.shots: list[Shot] = []
        self.mines: list[Mine] = []
        self.explosions: list[Explosion] = []
        self.difficulty: int = 5
        self.difficulty_per_player: dict[int, int] = {
            1: 5,
            2: 5,
        }  # Different diff per player for Demo
        self.battles_played: int = 0
        self.wins: dict[int, int] = {1: 0, 2: 0}
        self.winner: int | None = None

        # Track held keys for 8-way movement (diagonal detection)
        self._held_keys: set[int] = set()
        self._newly_pressed_keys: set[int] = set()  # Keys pressed this frame (no repeats)

        # Movement timing
        self._last_move_time: dict[int, float] = {1: 0.0, 2: 0.0}
        self._move_delay: float = 0.5  # Default delay
        self._shot_delay: float = 0.25  # Default shot delay

        # Barrel swing state: tracks if player has swung but not moved yet
        # Key: player_id, Value: direction they swung to (or None if ready to move)
        self._swing_state: dict[int, Direction | None] = {1: None, 2: None}

        # AI for single-player or Demo mode
        self._ai: dict[int, AIPlayer] = {}  # player_id -> AIPlayer
        self._ai_last_update: float = 0.0

        # Showdown mode tracking
        self._showdown_iterations: int = 0
        self._showdown_current: int = 0
        self._showdown_results: list[dict] = []
        self._showdown_start_time: float = 0.0
        self._showdown_schedule_kind: str = "random"  # "random" | "matrix"
        self._showdown_parallel_thread: threading.Thread | None = None
        self._showdown_workers: int = 0
        self._showdown_matrix_rounds_input: int = 10  # rounds per pairing (matrix)
        self._showdown_matrix_schedule: list[tuple[int, int]] = []
        self._showdown_headless_pref: bool = False
        self._showdown_headless: bool = False
        self._headless_sim_time_s: float = 0.0
        self._headless_sim_time_cumulative: float = 0.0
        self._headless_wall_budget_s: float = HEADLESS_WALL_BUDGET_DEFAULT_S
        # Rolling ~1s sim/wall rate for headless dashboard (uses cumulative counter)
        self._headless_rate_live_wall0: float = 0.0
        self._headless_rate_live_sim0: float = 0.0
        self._headless_display_rate_live: float = 0.0
        # Wall clock at current match start; used to compute prev-match rate at match end
        self._headless_match_wall_start: float = 0.0
        self._headless_prev_match_rate: float | None = None
        # Tournament wall-clock start for matches/sec calculation
        self._headless_tournament_wall_start: float = 0.0
        # When True, _finish_showdown skips file I/O (used by multiprocessing workers)
        self._suppress_summary_file: bool = False

    def sim_time_s(self) -> float:
        """Simulation clock. In headless SHOWDOWN, advances only via headless batch steps."""
        if getattr(self, "_showdown_headless", False):
            return self._headless_sim_time_s
        import pygame

        return pygame.time.get_ticks() / 1000.0

    def abort_showdown_run(self) -> None:
        """Stop SHOWDOWN early (ESC). Does not write a tournament summary file."""
        self.state = GameState.SHOWDOWN_COMPLETE
        self.winner = None
        self._showdown_summary_file = "(cancelled — no results file written)"

    def tick_headless_sim_rate_display(self) -> None:
        """Update rolling ~1s sim/wall rate for the headless dashboard (uses cumulative counter)."""
        if not getattr(self, "_showdown_headless", False):
            return
        if self.state != GameState.SHOWDOWN_RUNNING:
            return
        now = time.perf_counter()
        sim = self._headless_sim_time_cumulative
        elapsed = now - self._headless_rate_live_wall0
        if elapsed >= 1.0:
            ds = sim - self._headless_rate_live_sim0
            if elapsed > 0:
                self._headless_display_rate_live = ds / elapsed
            self._headless_rate_live_wall0 = now
            self._headless_rate_live_sim0 = sim

    def headless_sim_rate_dashboard_lines(self, *, parallel: bool = False) -> str:
        """Live rolling rate, matches/sec, and ETA."""
        cur = self._showdown_current
        total = getattr(self, "_showdown_iterations", 0) or 1
        wall_start = self._headless_tournament_wall_start
        wall_elapsed = time.perf_counter() - wall_start if wall_start > 0 else 0.0
        mps = cur / wall_elapsed if wall_elapsed > 0.5 else 0.0
        remaining = total - cur
        eta_s = remaining / mps if mps > 0 else 0.0
        if eta_s >= 60:
            eta_txt = f"{eta_s / 60:.1f} min"
        else:
            eta_txt = f"{eta_s:.0f} s"
        if parallel:
            return f"{mps:.2f} matches/sec\nETA: {eta_txt}"
        live = self._headless_display_rate_live
        return (
            f"Sim rate (live): {live:.1f}x  |  {mps:.2f} matches/sec\n"
            f"ETA: {eta_txt}"
        )

    def init_round(self) -> None:
        import random

        self.board = Board()
        tanks, shots, mines = difficulty_to_resources(self.difficulty)

        # Set movement and shot delay based on difficulty
        # The user-selected "level" controls game speed and terrain density
        _debug_log(f"init_round: game_mode={self.game_mode}, difficulty={self.difficulty}")
        if self.game_mode in ("Demo", "SHOWDOWN"):
            # Demo/SHOWDOWN mode: use user's difficulty but 10x faster (for fast AI vs AI action)
            base_move = get_move_delay(self.difficulty)
            base_shot = get_shot_delay(self.difficulty)
            self._move_delay = base_move * 0.1  # 10x faster!
            self._shot_delay = base_shot * 0.1  # 10x faster!
            _debug_log(
                f"  -> Demo/SHOWDOWN: user diff={self.difficulty}, move_delay={self._move_delay:.3f}, shot_delay={self._shot_delay:.3f}"
            )
        else:
            self._move_delay = get_move_delay(self.difficulty)
            self._shot_delay = get_shot_delay(self.difficulty)

        self.tanks = {
            1: Tank(
                player_id=1,
                x=2,
                y=12,
                direction=Direction.RIGHT,
                lives=tanks,
                shots_left=shots,
                mines_left=mines,
                start_pos=(2, 12),
            ),
            2: Tank(
                player_id=2,
                x=self.board.width - 3,
                y=12,
                direction=Direction.LEFT,
                lives=tanks,
                shots_left=shots,
                mines_left=mines,
                start_pos=(self.board.width - 3, 12),
            ),
        }
        for tank in self.tanks.values():
            tank.occupy_board(self.board)

        # Initialize AI for single-player or Demo mode
        if self.game_mode == "1P" and self.difficulty < 10:
            enemy_start = self.tanks[1].start_pos
            ai_start = self.tanks[2].start_pos
            self._ai = {
                2: AIPlayer(
                    difficulty=self.difficulty,
                    start_pos=ai_start,
                    enemy_start_pos=enemy_start,
                    move_delay=self._move_delay,
                )
            }
            _debug_log(
                f"AI initialized: difficulty={self.difficulty}, start={ai_start}, enemy_start={enemy_start}"
            )
        elif self.game_mode in ("Demo", "SHOWDOWN"):
            if self.game_mode == "SHOWDOWN" and self.difficulty_per_player.get(1) is not None:
                diff1 = self.difficulty_per_player[1]
                diff2 = self.difficulty_per_player[2]
            else:
                diff1 = random.randint(0, 9)
                diff2 = random.randint(0, 9)
                self.difficulty_per_player[1] = diff1
                self.difficulty_per_player[2] = diff2
            _debug_log(f"Demo/SHOWDOWN: Player 1 AI diff={diff1}, Player 2 AI diff={diff2}")

            _debug_log(f"  -> move_delay={self._move_delay:.4f}")

            self._ai = {
                1: AIPlayer(
                    difficulty=diff1,
                    start_pos=self.tanks[1].start_pos,
                    enemy_start_pos=self.tanks[2].start_pos,
                    move_delay=self._move_delay,
                ),
                2: AIPlayer(
                    difficulty=diff2,
                    start_pos=self.tanks[2].start_pos,
                    enemy_start_pos=self.tanks[1].start_pos,
                    move_delay=self._move_delay,
                ),
            }
            # Debug: log AI reaction times
            _debug_log(f"  -> AI[1] reaction_time={self._ai[1].config.reaction_time:.3f}")
            _debug_log(f"  -> AI[2] reaction_time={self._ai[2].config.reaction_time:.3f}")
        else:
            self._ai = {}

        self.shots.clear()
        self.mines.clear()
        self.explosions.clear()
        self.winner = None
        self.state = GameState.PLAYING

        # Reset swing states for new round
        self._swing_state = {1: None, 2: None}

    def handle_input(self, events: list) -> None:
        import pygame

        from .constants import PLAYER1_KEYS, PLAYER2_KEYS

        self._newly_pressed_keys.clear()
        for event in events:
            if event.type == pygame.KEYDOWN:
                # Only track if not already held (filters out key repeats)
                if event.key not in self._held_keys:
                    self._newly_pressed_keys.add(event.key)
                self._held_keys.add(event.key)
            elif event.type == pygame.KEYUP:
                self._held_keys.discard(event.key)

        if self.state == GameState.MENU:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.QUIT
                    else:
                        self.state = GameState.GAME_MODE_SELECT  # NEW: go to game mode selection
            return

        if self.state == GameState.GAME_MODE_SELECT:  # NEW: Demo vs 1P vs 2P vs SHOWDOWN selection
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.MENU
                    elif event.key == pygame.K_0:
                        self.game_mode = "Demo"
                        self.state = GameState.SKILL_SELECT
                    elif event.key == pygame.K_1:
                        self.game_mode = "1P"
                        self.state = GameState.SKILL_SELECT
                    elif event.key == pygame.K_2:
                        self.game_mode = "2P"
                        self.state = GameState.SKILL_SELECT
                    elif event.key == pygame.K_3:
                        self.game_mode = "SHOWDOWN"
                        self.state = GameState.SHOWDOWN_SCHEDULE_TYPE
            return

        if self.state == GameState.SHOWDOWN_SCHEDULE_TYPE:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.GAME_MODE_SELECT
                    elif event.key == pygame.K_h:
                        self._showdown_headless_pref = not self._showdown_headless_pref
                    elif event.key in (pygame.K_1, pygame.K_r):
                        self._showdown_schedule_kind = "random"
                        if not hasattr(self, "_showdown_input"):
                            self._showdown_input = 100
                        self.state = GameState.SHOWDOWN_ITERATIONS
                    elif event.key in (pygame.K_2, pygame.K_m):
                        self._showdown_schedule_kind = "matrix"
                        self.state = GameState.SHOWDOWN_MATRIX_ROUNDS
            return

        if self.state == GameState.SHOWDOWN_MATRIX_ROUNDS:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.SHOWDOWN_SCHEDULE_TYPE
                    elif event.key == pygame.K_h:
                        self._showdown_headless_pref = not self._showdown_headless_pref
                    elif event.key == pygame.K_RETURN or event.key == pygame.K_SPACE:
                        self.state = GameState.SHOWDOWN_MATRIX_CONFIRM
                    elif event.key == pygame.K_UP:
                        self._showdown_matrix_rounds_input = min(
                            999, self._showdown_matrix_rounds_input + 1
                        )
                    elif event.key == pygame.K_DOWN:
                        self._showdown_matrix_rounds_input = max(
                            1, self._showdown_matrix_rounds_input - 1
                        )
                    elif event.key == pygame.K_RIGHT:
                        self._showdown_matrix_rounds_input = min(
                            999, self._showdown_matrix_rounds_input + 10
                        )
                    elif event.key == pygame.K_LEFT:
                        self._showdown_matrix_rounds_input = max(
                            1, self._showdown_matrix_rounds_input - 10
                        )
            return

        if self.state == GameState.SHOWDOWN_MATRIX_CONFIRM:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.SHOWDOWN_MATRIX_ROUNDS
                    elif event.key == pygame.K_h:
                        self._showdown_headless_pref = not self._showdown_headless_pref
                    elif event.key == pygame.K_RETURN or event.key == pygame.K_SPACE:
                        r = self._showdown_matrix_rounds_input
                        self._showdown_matrix_schedule = build_showdown_matrix_schedule(r)
                        self._showdown_iterations = len(self._showdown_matrix_schedule)
                        self._showdown_current = 0
                        self._showdown_results = []
                        self._showdown_headless = self._showdown_headless_pref
                        self._showdown_workers = _env_workers()
                        self._start_showdown_match()
            return

        if self.state == GameState.SHOWDOWN_ITERATIONS:
            # Random mode: total number of matches (default 100)
            if not hasattr(self, "_showdown_input"):
                self._showdown_input = 100

            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.SHOWDOWN_SCHEDULE_TYPE
                    elif event.key == pygame.K_h:
                        self._showdown_headless_pref = not self._showdown_headless_pref
                    elif event.key == pygame.K_RETURN or event.key == pygame.K_SPACE:
                        self._showdown_schedule_kind = "random"
                        self._showdown_matrix_schedule = []
                        self._showdown_iterations = self._showdown_input
                        self._showdown_current = 0
                        self._showdown_results = []
                        self._showdown_headless = self._showdown_headless_pref
                        self._showdown_workers = _env_workers()
                        self._start_showdown_match()
                    elif event.key == pygame.K_UP:
                        self._showdown_input = min(9999, self._showdown_input + 10)
                    elif event.key == pygame.K_DOWN:
                        self._showdown_input = max(1, self._showdown_input - 10)
                    elif event.key == pygame.K_RIGHT:
                        self._showdown_input = min(9999, self._showdown_input + 1)
                    elif event.key == pygame.K_LEFT:
                        self._showdown_input = max(1, self._showdown_input - 1)
                    elif event.key == pygame.K_0:
                        self._showdown_input = self._showdown_input * 10
                    elif event.key == pygame.K_1:
                        self._showdown_input = self._showdown_input * 10 + 1
                    elif event.key == pygame.K_2:
                        self._showdown_input = self._showdown_input * 10 + 2
                    elif event.key == pygame.K_3:
                        self._showdown_input = self._showdown_input * 10 + 3
                    elif event.key == pygame.K_4:
                        self._showdown_input = self._showdown_input * 10 + 4
                    elif event.key == pygame.K_5:
                        self._showdown_input = self._showdown_input * 10 + 5
                    elif event.key == pygame.K_6:
                        self._showdown_input = self._showdown_input * 10 + 6
                    elif event.key == pygame.K_7:
                        self._showdown_input = self._showdown_input * 10 + 7
                    elif event.key == pygame.K_8:
                        self._showdown_input = self._showdown_input * 10 + 8
                    elif event.key == pygame.K_9:
                        self._showdown_input = self._showdown_input * 10 + 9
            return

        if self.state == GameState.SKILL_SELECT:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.MENU
                    elif event.key in (
                        pygame.K_0,
                        pygame.K_1,
                        pygame.K_2,
                        pygame.K_3,
                        pygame.K_4,
                        pygame.K_5,
                        pygame.K_6,
                        pygame.K_7,
                        pygame.K_8,
                        pygame.K_9,
                    ):
                        # 0-9 maps to difficulty 0-9 (0 = easiest, 9 = hardest)
                        key_name = pygame.key.name(event.key)
                        self.difficulty = int(key_name[-1])  # Last char of key name
                        self.init_round()
                    elif event.key == pygame.K_UP or event.key == pygame.K_RIGHT:
                        self.difficulty = min(9, self.difficulty + 1)
                    elif event.key == pygame.K_DOWN or event.key == pygame.K_LEFT:
                        self.difficulty = max(0, self.difficulty - 1)
                    elif event.key == pygame.K_RETURN or event.key == pygame.K_SPACE:
                        self.init_round()
            return

        if self.state == GameState.PLAY_AGAIN:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.TOURNAMENT_END
                    elif event.key == pygame.K_y:
                        # Another battle
                        self.init_round()
                    elif event.key == pygame.K_n:
                        self.state = GameState.TOURNAMENT_END
            return

        if self.state == GameState.TOURNAMENT_END:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE or event.key == pygame.K_q:
                        self.state = GameState.QUIT
                    elif event.key == pygame.K_RETURN or event.key == pygame.K_SPACE:
                        # Restart from menu
                        self.state = GameState.MENU
                        self.battles_played = 0
                        self.wins = {1: 0, 2: 0}
            return

        if self.state == GameState.SHOWDOWN_COMPLETE:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE or event.key == pygame.K_q:
                        self.state = GameState.QUIT
            return

        if self.state == GameState.SHOWDOWN_RUNNING:
            # Allow ESC to cancel showdown during gameplay; headless: ↑/↓ CPU budget
            for event in events:
                if event.type != pygame.KEYDOWN:
                    continue
                if event.key == pygame.K_ESCAPE:
                    _debug_log("SHOWDOWN: ESC pressed, cancelling")
                    self.abort_showdown_run()
                    return
                if getattr(self, "_showdown_headless", False):
                    if event.key == pygame.K_UP:
                        self._headless_wall_budget_s = min(
                            HEADLESS_WALL_BUDGET_MAX_S,
                            self._headless_wall_budget_s + HEADLESS_WALL_BUDGET_STEP_S,
                        )
                    elif event.key == pygame.K_DOWN:
                        self._headless_wall_budget_s = max(
                            HEADLESS_WALL_BUDGET_MIN_S,
                            self._headless_wall_budget_s - HEADLESS_WALL_BUDGET_STEP_S,
                        )
            return  # Let update() handle showdown running

        if self.state != GameState.PLAYING:
            return

        for event in events:
            if event.type != pygame.KEYDOWN:
                continue
            if event.key == pygame.K_ESCAPE:
                self.state = GameState.QUIT
                return

            # Process input for both players
            for player_id in [1, 2]:
                tank = self.tanks[player_id]
                keys = PLAYER1_KEYS if player_id == 1 else PLAYER2_KEYS

                # Check for fire (individual keypress, not held)
                if event.key == keys["fire"] and tank.can_fire():
                    self._fire_shot(tank)
                    continue

                # Check for mine (individual keypress)
                if event.key == keys["mine"] and tank.can_place_mine():
                    cell = self.board.get_cell(tank.x, tank.y)
                    _debug_log(
                        f"MINE_CHECK: player={player_id} at ({tank.x},{tank.y}), cell={cell.type if cell else None}"
                    )
                    # Allow placement on EMPTY or on any tank (placing under yourself)
                    # Block: existing mines, walls, wreckage
                    if cell and cell.type not in {
                        CellType.MINE,
                        CellType.WALL,
                        CellType.WRECKAGE_P1,
                        CellType.WRECKAGE_P2,
                    }:
                        # Owner ID added for mine stealth tracking
                        mine = Mine(x=tank.x, y=tank.y, owner_id=player_id)
                        mine.visible_start_time = self.sim_time_s()
                        self.mines.append(mine)
                        self.board.set_cell_type(tank.x, tank.y, CellType.MINE)
                        tank.consume_mine()
                        _debug_log(
                            f"MINE_PLACED: player={player_id} at ({tank.x},{tank.y}), total mines: {len(self.mines)}"
                        )
                        # Log board state for debugging
                        _debug_log(
                            f"  Board after mine: {[(m.x, m.y, c.type) for m in self.mines if (c := self.board.get_cell(m.x, m.y))]}"
                        )
                    continue

        # Process movement based on newly pressed keys only (no repeats)
        self._process_movement()

    def _process_movement(self) -> None:
        """Process 8-way movement based on NEW keypresses only (filters repeats).

        Barrel swing mechanic:
        - First keypress in ANY direction: swing barrel, don't move
        - Second keypress in SAME direction: actually move
        - Can swing multiple times in different directions without moving
        """
        from .constants import PLAYER1_KEYS, PLAYER2_KEYS

        current_time = self.sim_time_s()

        for player_id in [1, 2]:
            tank = self.tanks[player_id]
            keys = PLAYER1_KEYS if player_id == 1 else PLAYER2_KEYS

            # Check movement delay
            if current_time - self._last_move_time[player_id] < self._move_delay:
                continue

            # Determine desired direction from currently held keys
            desired_direction = None

            # Check dedicated diagonal keys FIRST (takes priority)
            if keys["up_left"] in self._held_keys:
                desired_direction = Direction.UP_LEFT
            elif keys["up_right"] in self._held_keys:
                desired_direction = Direction.UP_RIGHT
            elif keys["down_left"] in self._held_keys:
                desired_direction = Direction.DOWN_LEFT
            elif keys["down_right"] in self._held_keys:
                desired_direction = Direction.DOWN_RIGHT
            else:
                # Determine desired direction from held cardinal keys
                dx, dy = 0, 0

                # Check horizontal
                if keys["left"] in self._held_keys and keys["right"] in self._held_keys:
                    dx = 0
                elif keys["left"] in self._held_keys:
                    dx = -1
                elif keys["right"] in self._held_keys:
                    dx = 1

                # Check vertical
                if keys["up"] in self._held_keys and keys["down"] in self._held_keys:
                    dy = 0
                elif keys["up"] in self._held_keys:
                    dy = -1
                elif keys["down"] in self._held_keys:
                    dy = 1

                # Map dx/dy to Direction enum
                if dx == -1 and dy == -1:
                    desired_direction = Direction.UP_LEFT
                elif dx == 1 and dy == -1:
                    desired_direction = Direction.UP_RIGHT
                elif dx == -1 and dy == 1:
                    desired_direction = Direction.DOWN_LEFT
                elif dx == 1 and dy == 1:
                    desired_direction = Direction.DOWN_RIGHT
                elif dx == -1:
                    desired_direction = Direction.LEFT
                elif dx == 1:
                    desired_direction = Direction.RIGHT
                elif dy == -1:
                    desired_direction = Direction.UP
                elif dy == 1:
                    desired_direction = Direction.DOWN

            # Only process if we have a valid direction AND player just pressed a direction key
            if desired_direction:
                # Check if any direction key was newly pressed this frame
                direction_keys = {
                    keys["up"],
                    keys["down"],
                    keys["left"],
                    keys["right"],
                    keys["up_left"],
                    keys["up_right"],
                    keys["down_left"],
                    keys["down_right"],
                }
                newly_pressed_direction = direction_keys & self._newly_pressed_keys

                if not newly_pressed_direction:
                    # No new direction key pressed this frame, skip
                    continue

                swung_direction = self._swing_state[player_id]

                if desired_direction == tank.direction:
                    # Pressing direction we're already facing - move immediately, no swing needed
                    if tank.attempt_move(self.board, desired_direction):
                        _debug_log(f"MOVE: player={player_id} moved to ({tank.x},{tank.y})")
                        self._last_move_time[player_id] = current_time
                        self._resolve_tank_mine_collision(tank)
                    self._swing_state[player_id] = None  # Reset state
                elif desired_direction == swung_direction:
                    # Second press in same direction we swung to - move now
                    if tank.attempt_move(self.board, desired_direction):
                        self._last_move_time[player_id] = current_time
                        self._resolve_tank_mine_collision(tank)
                    self._swing_state[player_id] = None  # Reset after move
                else:
                    # First press in a new direction - just swing, don't move
                    tank.direction = desired_direction
                    self._swing_state[player_id] = desired_direction

        # Clear newly pressed keys after processing
        self._newly_pressed_keys.clear()

    def _fire_shot(self, tank: Tank, *, direction_override: Direction | None = None) -> None:
        """Fire a shot and check for Empty Gun rule.

        Original TANK! mechanic: only ONE shot per player on screen at a time.

        ``direction_override`` (AI risky fire, FR-8): spawn shot in this direction without
        rotating the tank sprite.
        """
        # Check if this tank already has an active shot on the board
        for shot in self.shots:
            if shot.active:
                # Check if this shot belongs to this tank by looking at spawn position
                # We can infer this by checking if shot is moving AWAY from tank's position
                # Simpler: just don't allow firing if any shot exists from this general direction
                pass

        # Actually check - if we already have a shot heading in this direction, don't fire
        # Count how many shots this player has on screen
        player_shots = 0
        for shot in self.shots:
            if shot.active and shot.owner_id == tank.player_id:
                player_shots += 1

        # Original TANK! mechanic: only 1 shot per player at a time
        if player_shots > 0:
            _debug_log(
                f"SHOT_BLOCKED: player {tank.player_id} already has {player_shots} shot(s) on screen"
            )
            return

        shot_dir = direction_override if direction_override is not None else tank.direction
        shot_pos = self._shot_spawn_position(tank, shot_dir)
        if shot_pos is None:
            _debug_log(f"SHOT_BLOCKED: player {tank.player_id} no valid spawn position")
            return

        x, y = shot_pos
        self.shots.append(Shot(x=x, y=y, direction=shot_dir, owner_id=tank.player_id))
        tank.consume_shot()
        _debug_log(f"SHOT_FIRED: player={tank.player_id} pos=({x},{y}) dir={tank.direction.name}")

        # Empty Gun Rule: if shooter has no shots left, they self-destruct
        if not tank.can_fire():
            self._tank_hit(tank, (tank.x, tank.y))

    def _shot_spawn_position(
        self, tank: Tank, fire_direction: Direction | None = None
    ) -> tuple[int, int] | None:
        d = fire_direction if fire_direction is not None else tank.direction
        dx, dy = DIRECTION_VECTORS[d]
        x = tank.x + dx
        y = tank.y + dy
        if not self.board.in_bounds(x, y):
            return None
        cell = self.board.get_cell(x, y)
        if cell is None or cell.type == CellType.WALL:
            return None
        return x, y

    def _resolve_tank_mine_collision(self, tank: Tank) -> None:
        for mine in list(self.mines):
            if mine.active and mine.x == tank.x and mine.y == tank.y:
                _debug_log(f"TANK_MINE_COLLISION: player={tank.player_id} at ({tank.x},{tank.y})")
                _debug_log(f"  Mines list: {[(m.x, m.y, m.active) for m in self.mines]}")
                # Tank stepped on mine - process respawn FIRST, then explosion
                # The tank that stepped on the mine takes damage
                self._tank_hit(tank, (tank.x, tank.y))

                mine.active = False
                # Trigger 360° explosion (tanks already respawned, won't be hit again)
                self._explode_mine(mine.x, mine.y)
                _debug_log(
                    f"  -> Explosion complete, remaining mines: {[(m.x, m.y, m.active) for m in self.mines]}"
                )
                break  # Only trigger one mine per collision

    def _tank_hit(self, tank: Tank, pos: tuple[int, int]) -> None:
        _debug_log(
            f"TANK_HIT: player={tank.player_id} at ({pos[0]},{pos[1]}), lives_before={tank.lives}"
        )

        # Leave wreckage at hit location BEFORE respawning
        wreckage_type = CellType.WRECKAGE_P1 if tank.player_id == 1 else CellType.WRECKAGE_P2
        self.board.set_cell_type(pos[0], pos[1], wreckage_type)

        tank.take_damage()
        exp = Explosion(
            x=pos[0], y=pos[1], start_time=self.sim_time_s(), duration=0.5
        )
        self.explosions.append(exp)
        _debug_log(f"  -> explosion added at ({pos[0]},{pos[1]}), lives_after={tank.lives}")

        if not tank.is_alive():
            _debug_log(f"  -> player {tank.player_id} DEFEATED!")
            self._on_player_defeated(tank.player_id)
        else:
            _debug_log(f"  -> player {tank.player_id} alive, respawning both")
            # Both tanks respawn to starting positions after each kill
            self._respawn_both_tanks()

    def _respawn_both_tanks(self) -> None:
        """Respawn both tanks to their start positions after a kill."""
        tanks, shots, mines = difficulty_to_resources(self.difficulty)

        for tank in self.tanks.values():
            start_x, start_y = tank.start_pos

            # Clear the tank's *current* cell before moving coordinates. Setting x/y first
            # would make clear_from_board wipe the spawn square (often EMPTY) and leave a
            # stale TANK1/TANK2 cell where the tank used to be — blocking movement and
            # causing shots to spawn into "ghost" tanks (instant self-hits).
            tank.clear_from_board(self.board)
            tank.x = start_x
            tank.y = start_y
            tank.occupy_board(self.board)

            # RESTOCK AMMO on respawn (fresh tank = full ammo)
            tank.shots_left = shots
            tank.mines_left = mines

            # Reset swing state after respawn
            self._swing_state[tank.player_id] = None

            _debug_log(
                f"Respawned player {tank.player_id} at ({start_x},{start_y}) with {shots} shots, {mines} mines"
            )

        nav_time = self.sim_time_s()
        for ai in self._ai.values():
            ai.navigation.on_respawn(nav_time)

    def _on_player_defeated(self, player_id: int) -> None:
        self.winner = 1 if player_id == 2 else 2
        self.wins[self.winner] += 1
        self.battles_played += 1

        # Don't end game - continue with respawn
        self._respawn_both_tanks()

        # Check if either player is out of lives - only then end game
        if not self.tanks[1].is_alive() or not self.tanks[2].is_alive():
            self.state = GameState.PLAY_AGAIN
            self._show_victory_message()

    def _show_victory_message(self) -> None:
        """Generate victory message based on remaining lives."""
        winner = self.winner
        if winner is None:
            return

        winner_tank = self.tanks[winner]
        remaining_lives = winner_tank.lives

        if remaining_lives == 1:
            suffix = "WITH HIS LAST TANK!"
        elif remaining_lives == 2:
            suffix = "WITH TWO TANKS LEFT!"
        else:  # 3 lives (perfect game)
            suffix = "WITHOUT LOSING A TANK!"

        self._victory_message = f"PLAYER #{winner} WINS!\n{suffix}"

    def get_victory_message(self) -> str:
        """Return the current victory message."""
        return getattr(self, "_victory_message", "")

    def _launch_parallel_showdown(self, workers: int) -> None:
        """Start background thread running multiprocessing pool for all matches."""
        from .showdown_worker import run_showdown_parallel

        def _run() -> None:
            run_showdown_parallel(self, workers=workers)

        t = threading.Thread(target=_run, daemon=True)
        self._showdown_parallel_thread = t
        t.start()

    def _start_showdown_match(self) -> None:
        """Start a single showdown match (random pairings or matrix schedule)."""
        import random

        # Always use difficulty 9 for maximum speed (timing); per-tank skills come from below.
        self.difficulty = 9

        match_idx = self._showdown_current
        if (
            self._showdown_schedule_kind == "matrix"
            and match_idx < len(self._showdown_matrix_schedule)
        ):
            diff1, diff2 = self._showdown_matrix_schedule[match_idx]
        else:
            diff1 = random.randint(0, 9)
            diff2 = random.randint(0, 9)
            while diff2 == diff1:
                diff2 = random.randint(0, 9)

        self.difficulty_per_player[1] = diff1
        self.difficulty_per_player[2] = diff2

        if getattr(self, "_showdown_headless", False):
            self._headless_sim_time_s = 0.0
            t0 = time.perf_counter()
            self._headless_match_wall_start = t0
            if self._showdown_current == 0:
                self._headless_rate_live_wall0 = t0
                self._headless_rate_live_sim0 = 0.0
                self._headless_display_rate_live = 0.0
                self._headless_sim_time_cumulative = 0.0
                self._headless_tournament_wall_start = t0
                workers = self._showdown_workers
                if workers > 1:
                    self.state = GameState.SHOWDOWN_RUNNING
                    self._launch_parallel_showdown(workers)
                    return
        self._showdown_match_start_sim = self.sim_time_s()

        _debug_log(f"SHOWDOWN: Match {self._showdown_current + 1}/{self._showdown_iterations}")
        _debug_log(f"  -> AI[1] diff={diff1}, AI[2] diff={diff2}")
        _debug_log("  -> _move_delay will be set in init_round")

        # Initialize the round
        self.init_round()
        self._showdown_current += 1
        self.state = GameState.SHOWDOWN_RUNNING
        _debug_log(
            f"SHOWDOWN: Match started, tanks at P1=({self.tanks[1].x},{self.tanks[1].y}) P2=({self.tanks[2].x},{self.tanks[2].y})"
        )

    def _run_showdown(self) -> None:
        """Run showdown matches in sequence."""
        # Check if game is over (winner declared)
        if self.winner is not None:
            # Record match result
            match_result = {
                "match": self._showdown_current,
                "winner": self.winner,
                "ai1_diff": self.difficulty_per_player[1],
                "ai2_diff": self.difficulty_per_player[2],
                "duration": self.sim_time_s() - self._showdown_match_start_sim,
            }
            self._showdown_results.append(match_result)

            if getattr(self, "_showdown_headless", False):
                wall_elapsed = time.perf_counter() - self._headless_match_wall_start
                sim_elapsed = self._headless_sim_time_s
                if wall_elapsed > 0.0 and sim_elapsed >= 0.0:
                    self._headless_prev_match_rate = sim_elapsed / wall_elapsed

            _debug_log(f"SHOWDOWN: Match complete - Winner: P{self.winner}")

            # Clear winner so next match can run
            self.winner = None

            if self._showdown_current < self._showdown_iterations:
                # Start next match
                self._start_showdown_match()
            else:
                # Showdown complete
                self._finish_showdown()

    def _finish_showdown(self) -> None:
        """Finish showdown and output results."""
        from datetime import datetime

        # Calculate statistics
        wins_by_skill = {i: {"wins": 0, "matches": 0} for i in range(10)}
        total_duration = 0
        longest_match = 0
        shortest_match = float("inf")

        for result in self._showdown_results:
            winner = result["winner"]
            ai1 = result["ai1_diff"]
            ai2 = result["ai2_diff"]
            duration = result["duration"]

            total_duration += duration
            longest_match = max(longest_match, duration)
            shortest_match = min(shortest_match, duration)

            wins_by_skill[ai1]["matches"] += 1
            wins_by_skill[ai2]["matches"] += 1
            if winner == 1:
                wins_by_skill[ai1]["wins"] += 1
            elif winner == 2:
                wins_by_skill[ai2]["wins"] += 1

        avg_duration = total_duration / len(self._showdown_results) if self._showdown_results else 0

        summary_file = ""
        if not self._suppress_summary_file:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            summary_file = f"tank_showdown_{timestamp}.txt"

            with open(summary_file, "w") as f:
                f.write("=" * 60 + "\n")
                f.write("TANK! SHOWDOWN RESULTS\n")
                f.write("=" * 60 + "\n")
                f.write(
                    f"Headless: {'yes' if getattr(self, '_showdown_headless', False) else 'no'}\n"
                )
                if getattr(self, "_showdown_headless", False):
                    f.write("(Match durations are simulated seconds, not wall clock.)\n")
                if self._showdown_schedule_kind == "matrix":
                    r = self._showdown_matrix_rounds_input
                    f.write("Schedule: Full skill matrix (every AI-i vs AI-j, i,j in 0..9)\n")
                    f.write(
                        f"Rounds per pairing: {r}  "
                        f"({SHOWDOWN_MATRIX_CELLS} pairings × {r} = {SHOWDOWN_MATRIX_CELLS * r} matches)\n"
                    )
                else:
                    f.write("Schedule: Random pairings (distinct skills resampled each match)\n")
                f.write(f"Total Matches: {len(self._showdown_results)}\n")
                f.write(f"Longest Match: {longest_match:.1f}s\n")
                f.write(f"Shortest Match: {shortest_match:.1f}s\n")
                f.write(f"Average Match: {avg_duration:.1f}s\n")
                f.write("\n" + "=" * 60 + "\n")
                f.write("WIN/LOSS RECORD BY AI SKILL LEVEL\n")
                f.write("(Aggregate — includes both P1 and P2 assignments)\n")
                f.write("=" * 60 + "\n")

                for skill in range(10):
                    stats = wins_by_skill[skill]
                    if stats["matches"] > 0:
                        win_pct = (stats["wins"] / stats["matches"]) * 100
                        f.write(
                            f"AI-{skill}: {stats['wins']}/{stats['matches']} wins ({win_pct:.1f}%)\n"
                        )

                if self._showdown_schedule_kind == "matrix" and self._showdown_results:
                    f.write("\n" + "=" * 60 + "\n")
                    f.write("HEAD-TO-HEAD — P1 skill (row) vs P2 skill (column)\n")
                    f.write("Cell: P1 wins / games (P1 win %)\n")
                    f.write("=" * 60 + "\n")
                    h2h_games = [[0] * SHOWDOWN_SKILL_LEVELS for _ in range(SHOWDOWN_SKILL_LEVELS)]
                    h2h_p1 = [[0] * SHOWDOWN_SKILL_LEVELS for _ in range(SHOWDOWN_SKILL_LEVELS)]
                    for result in self._showdown_results:
                        rr, cc = result["ai1_diff"], result["ai2_diff"]
                        h2h_games[rr][cc] += 1
                        w = result["winner"]
                        if w == 1:
                            h2h_p1[rr][cc] += 1
                    col_w = 11
                    header = "P1\\P2".ljust(6) + "".join(f"{j:>{col_w}}" for j in range(SHOWDOWN_SKILL_LEVELS))
                    f.write(header + "\n")
                    for i in range(SHOWDOWN_SKILL_LEVELS):
                        line = f"{i:<6}"
                        for j in range(SHOWDOWN_SKILL_LEVELS):
                            g = h2h_games[i][j]
                            w = h2h_p1[i][j]
                            if g == 0:
                                cell = "---"
                            else:
                                pct = 100.0 * w / g
                                cell = f"{w}/{g} {pct:4.1f}%"
                            line += cell.rjust(col_w)
                        f.write(line + "\n")

                f.write("\n" + "=" * 60 + "\n")
                f.write("MATCH DETAILS\n")
                f.write("=" * 60 + "\n")
                for result in self._showdown_results:
                    if result.get("timeout"):
                        tr = result.get("timeout_reason", "?")
                        extra = " (exhausted retries)" if result.get("timeout_after_retries") else ""
                        f.write(
                            f"Match {result['match']}: TIMEOUT ({tr}){extra} "
                            f"AI-{result['ai1_diff']} vs AI-{result['ai2_diff']}, "
                            f"{result['duration']:.1f}s sim\n"
                        )
                    else:
                        f.write(
                            f"Match {result['match']}: P{result['winner']} won "
                            f"(AI-{result['ai1_diff']} vs AI-{result['ai2_diff']}, "
                            f"{result['duration']:.1f}s)\n"
                        )

            _debug_log(f"SHOWDOWN COMPLETE: {len(self._showdown_results)} matches")
            _debug_log(f"Results saved to {summary_file}")

        self.state = GameState.SHOWDOWN_COMPLETE
        self._showdown_summary_file = summary_file

    def update(self, dt: float) -> None:
        if self.state == GameState.SHOWDOWN_RUNNING:
            # Run game updates for showdown mode (shots, mines, explosions)
            self._update_shots()
            self._update_mines()
            self._update_explosions()

            # Run AI for showdown mode
            if self.game_mode in ("SHOWDOWN", "Demo") and self._ai:
                self._run_ai(1)
                self._run_ai(2)
            self._run_showdown()
            return

        if self.state != GameState.PLAYING:
            return
        self._update_shots()
        self._update_mines()
        self._update_explosions()

        # Run AI for 1P (player 2), Demo, or Showdown (both players)
        if self.game_mode == "1P" and self._ai:
            self._run_ai(2)
        elif self.game_mode in ("Demo", "SHOWDOWN") and self._ai:
            # Run AI for both players
            self._run_ai(1)
            self._run_ai(2)

    def _ai_try_random_escape_move(
        self,
        player_id: int,
        ai: AIPlayer,
        ai_tank: Tank,
        current_time: float,
        *,
        log_reason: str,
    ) -> bool:
        """Try one step in a shuffled random direction (unstick / evade fallback)."""
        all_dirs = [
            Direction.UP,
            Direction.DOWN,
            Direction.LEFT,
            Direction.RIGHT,
            Direction.UP_LEFT,
            Direction.UP_RIGHT,
            Direction.DOWN_LEFT,
            Direction.DOWN_RIGHT,
        ]
        tabu_ok = [
            d
            for d in all_dirs
            if not ai.navigation.is_tabu((ai_tank.x, ai_tank.y), d, current_time)
        ]
        random.shuffle(tabu_ok)
        for d in tabu_ok:
            if ai._can_move(self.board, (ai_tank.x, ai_tank.y), d):
                if ai.avoid_own_mines(self.board, (ai_tank.x, ai_tank.y), d):
                    if self._ai_attempt_move(player_id, ai, ai_tank, d, current_time):
                        self._last_move_time[player_id] = current_time
                        self._resolve_tank_mine_collision(ai_tank)
                        _debug_log(f"AI[{player_id}] random move ({log_reason})")
                        return True
        random.shuffle(all_dirs)
        for d in all_dirs:
            if ai._can_move(self.board, (ai_tank.x, ai_tank.y), d):
                if ai.avoid_own_mines(self.board, (ai_tank.x, ai_tank.y), d):
                    if self._ai_attempt_move(player_id, ai, ai_tank, d, current_time):
                        self._last_move_time[player_id] = current_time
                        self._resolve_tank_mine_collision(ai_tank)
                        _debug_log(f"AI[{player_id}] random move ({log_reason}) fallback")
                        return True
        return False

    def _ai_attempt_move(
        self,
        player_id: int,
        ai: AIPlayer,
        tank: Tank,
        direction: Direction,
        current_time: float,
    ) -> bool:
        """Apply AI move; record navigation memory on failure (PRD NAV-2)."""
        pos_before = (tank.x, tank.y)
        ok = tank.attempt_move(self.board, direction)
        if not ok:
            ai.navigation.record_failed_attempt(self.board, pos_before, direction, current_time)
        return ok

    def _run_ai(self, player_id: int) -> None:
        """Execute AI decision making for a specific player."""
        ai = self._ai.get(player_id)
        if not ai or not self.tanks.get(player_id):
            if _DEBUG:
                _debug_log(f"AI[{player_id}] SKIP: no ai or no tank")
            return

        current_time = self.sim_time_s()

        if current_time - ai._last_action_time < ai.config.reaction_time:
            return

        ai_tank = self.tanks[player_id]

        enemy_id = 2 if player_id == 1 else 1
        enemy_tank = self.tanks.get(enemy_id)
        enemy_pos = (enemy_tank.x, enemy_tank.y) if enemy_tank else None

        if _DEBUG:
            _debug_log(
                f"AI[{player_id}] update: pos=({ai_tank.x},{ai_tank.y}) enemy={enemy_pos} reaction_time={ai.config.reaction_time:.3f} last_action={ai._last_action_time:.3f}"
            )

        action = ai.update(
            current_time=current_time,
            board=self.board,
            my_pos=(ai_tank.x, ai_tank.y),
            my_direction=ai_tank.direction,
            my_shots=ai_tank.shots_left,
            my_mines=ai_tank.mines_left,
            enemy_pos=enemy_pos,
            shots_on_board=self.shots,
            player_id=player_id,
        )

        if action is None:
            return

        _debug_log(f"AI[{player_id}] Action: {action.name if hasattr(action, 'name') else action}")

        # Execute the action
        if action == AIAction.RISKY_PREPARE:
            # FR-8: turn barrel toward jitter; next decision fires like a human (no direction_override)
            if getattr(ai, "_risky_wild_state", None) == "pending_aim":
                ai_tank.direction = ai._risky_jitter_direction
                ai._risky_wild_state = "ready_to_fire"
                _debug_log(f"AI[{player_id}] risky prepare aim -> {ai_tank.direction.name}")
        elif action == AIAction.FIRE:
            if ai_tank.can_fire():
                risky = getattr(ai, "_pending_fire_risky", False)
                ai._pending_fire_risky = False
                if risky:
                    self._fire_shot(ai_tank)
                    ai_tank.direction = ai._risky_saved_direction
                    ai._risky_wild_state = "idle"
                    _debug_log(
                        f"AI[{player_id}] risky fire done, barrel restored -> {ai_tank.direction.name}"
                    )
                else:
                    self._fire_shot(ai_tank)
                _debug_log(f"AI[{player_id}] fired! risky={risky}")

        elif action == AIAction.MOVE_TOWARD_ENEMY:
            target = enemy_pos if enemy_pos else ai.enemy_start_pos
            blind = ai._combat_enemy_pos is None
            direction = ai.get_movement_toward_target(
                self.board,
                (ai_tank.x, ai_tank.y),
                target,
                blind=blind,
                current_time=current_time,
            )
            if direction and ai.avoid_own_mines(self.board, (ai_tank.x, ai_tank.y), direction):
                if self._ai_attempt_move(player_id, ai, ai_tank, direction, current_time):
                    self._last_move_time[player_id] = current_time
                    self._resolve_tank_mine_collision(ai_tank)
                    _debug_log(f"AI[{player_id}] moved toward enemy to {ai_tank.x},{ai_tank.y}")

        elif action == AIAction.MOVE_AWAY_FROM_ENEMY:
            foe_pos = enemy_pos or ai.last_known_enemy_pos
            if foe_pos:
                my_x, my_y = ai_tank.x, ai_tank.y
                dir_candidate = ai.pick_evade_direction(
                    self.board,
                    (my_x, my_y),
                    foe_pos,
                    current_time=current_time,
                )
                if dir_candidate is not None and ai.avoid_own_mines(
                    self.board, (my_x, my_y), dir_candidate
                ):
                    if self._ai_attempt_move(player_id, ai, ai_tank, dir_candidate, current_time):
                        self._last_move_time[player_id] = current_time
                        self._resolve_tank_mine_collision(ai_tank)
                        _debug_log(f"AI[{player_id}] evaded ({dir_candidate.name})")
                        return

                # Couldn't move away — actually try random directions (was log-only; caused stuck evasion)
                _debug_log(f"AI[{player_id}] couldn't evade, trying random")
                moved = self._ai_try_random_escape_move(
                    player_id, ai, ai_tank, current_time, log_reason="evade fallback"
                )
                if not moved and ai._is_evading:
                    ai._is_evading = False
                    ai._evasion_cooldown = max(ai._evasion_cooldown, 4)
                    _debug_log(f"AI[{player_id}] evasion ended: no legal step away or sideways")

            else:
                self._ai_try_random_escape_move(
                    player_id, ai, ai_tank, current_time, log_reason="evade no foe position"
                )

        elif action == AIAction.DODGE_SHOT:
            directions = [Direction.UP, Direction.DOWN, Direction.LEFT, Direction.RIGHT]
            for d in directions:
                if ai._can_move(self.board, (ai_tank.x, ai_tank.y), d):
                    if self._ai_attempt_move(player_id, ai, ai_tank, d, current_time):
                        self._last_move_time[player_id] = current_time
                        self._resolve_tank_mine_collision(ai_tank)
                        _debug_log(f"AI[{player_id}] dodged!")
                        break

        elif action == AIAction.SCAN:
            scan_dir = ai.get_scan_direction()
            ai_tank.direction = scan_dir
            _debug_log(f"AI[{player_id}] scanned: {scan_dir}")

        elif action == AIAction.RANDOM_MOVE:
            self._ai_try_random_escape_move(
                player_id, ai, ai_tank, current_time, log_reason="unstuck"
            )

        elif action == AIAction.PLACE_MINE:
            if ai_tank.can_place_mine():
                cell = self.board.get_cell(ai_tank.x, ai_tank.y)
                if cell and cell.type not in {
                    CellType.MINE,
                    CellType.WALL,
                    CellType.WRECKAGE_P1,
                    CellType.WRECKAGE_P2,
                }:
                    mine = Mine(x=ai_tank.x, y=ai_tank.y, owner_id=player_id)
                    mine.visible_start_time = current_time
                    self.mines.append(mine)
                    self.board.set_cell_type(ai_tank.x, ai_tank.y, CellType.MINE)
                    ai_tank.consume_mine()
                    ai.remember_mine(ai_tank.x, ai_tank.y)
                    _debug_log(f"AI[{player_id}] placed mine at {ai_tank.x},{ai_tank.y}")

        # Update mine memory decay
        ai.update_mine_memory()

    def _update_mines(self) -> None:
        """Update mine visibility - mines become invisible after 2 seconds."""
        current_time = self.sim_time_s()
        mine_visible_duration = 2.0  # seconds

        for mine in self.mines:
            if mine.active and mine.visible:
                if current_time - mine.visible_start_time > mine_visible_duration:
                    mine.visible = False

    def _update_shots(self) -> None:
        """Update shot positions with speed delay."""
        current_time = self.sim_time_s()
        shot_delay = self._shot_delay  # Use dedicated shot delay (0.5s to 0.05s)

        # Snapshot positions before stepping (for head-on pass-through detection)
        for shot in self.shots:
            if shot.active:
                shot._step_start_x = shot.x
                shot._step_start_y = shot.y

        # First, move all shots and check for any collisions at new positions
        for shot in list(self.shots):
            # Check if enough time has passed since last shot move
            last_move = getattr(shot, "_last_move_time", 0)
            if current_time - last_move < shot_delay:
                continue

            if not shot.active:
                if _DEBUG:
                    _debug_log(
                        f"SHOT_INACTIVE: player={shot.owner_id} at ({shot.x},{shot.y}), waiting for hit check"
                    )
                continue
            collision_pos = shot.step(self.board)

            if shot.active:
                shot._last_move_time = current_time
            else:
                if _DEBUG:
                    _debug_log(f"SHOT_HIT: player={shot.owner_id} at collision={collision_pos}")
                shot._collision_pos = collision_pos

        # Shot-shot: same cell, or head-on swap on same row/column (adjacent cells, one step)
        indices_to_remove = set()
        explosions_triggered = set()

        for i, shot1 in enumerate(self.shots):
            if not shot1.active or i in indices_to_remove:
                continue
            for j, shot2 in enumerate(self.shots):
                if j <= i or not shot2.active or j in indices_to_remove:
                    continue
                same_cell = shot1.x == shot2.x and shot1.y == shot2.y
                crossed = _shots_crossed_head_on(shot1, shot2)
                if same_cell or crossed:
                    indices_to_remove.add(i)
                    indices_to_remove.add(j)
                    key = _shot_shot_explosion_key(shot1, shot2, crossed=crossed)
                    if key not in explosions_triggered:
                        explosions_triggered.add(key)
                        if crossed:
                            _debug_log(
                                f"SHOT_SHOT_COLLISION: shots head-on crossed -> {key}, chain reaction!"
                            )
                        else:
                            _debug_log(
                                f"SHOT_SHOT_COLLISION: shots collided at {key}, chain reaction!"
                            )
                        self._explode_mine(key[0], key[1], radius=2, is_chain=True)

        # Deactivate collided shots
        for idx in indices_to_remove:
            shot = self.shots[idx]
            _debug_log(
                f"SHOT_REMOVED: player={shot.owner_id} SHOT_SHOT_COLLISION at ({shot.x},{shot.y})"
            )
            shot.active = False

        # Now process other collisions (tanks, mines)
        # Need to check BOTH active shots AND shots that just hit something (inactive but with collision_pos)
        for shot in list(self.shots):
            # Check collision position from step() FIRST (most recent hit)
            collision_pos = getattr(shot, "_collision_pos", None)
            if collision_pos:
                cx, cy = collision_pos
                shot._collision_pos = None  # Clear after checking
            else:
                # For active shots without collision, check current position
                if not shot.active:
                    continue
                cx, cy = shot.x, shot.y

            cell = self.board.get_cell(cx, cy)
            if cell is None:
                continue

            if cell.type in {CellType.TANK1, CellType.TANK2}:
                target_id = 1 if cell.type == CellType.TANK1 else 2
                _debug_log(f"SHOT_HIT_TANK: player={target_id} at ({cx},{cy})")
                self._tank_hit(self.tanks[target_id], (cx, cy))
            elif cell.type == CellType.MINE:
                for mine in self.mines:
                    if mine.x == cx and mine.y == cy and mine.active:
                        mine.active = False
                        _debug_log(
                            f"SHOT_MINE_HIT: shot hit mine at ({cx},{cy}), triggering chain reaction!"
                        )
                        self._explode_mine(mine.x, mine.y, radius=2, is_chain=True)
                        break
            # Check for splash damage from wall hits
            elif getattr(shot, "_hit_wall", False):
                # Shot hit a wall - trigger 360° splash damage (non-chain-reaction)
                _debug_log(f"SHOT_WALL_SPLASH: at ({cx},{cy})")
                self._explode_mine(cx, cy, radius=1, is_chain=False)

        # Clean up: remove shots that hit something (inactive, had collision_pos that was processed)
        self.shots = [
            shot
            for shot in self.shots
            if shot.active or getattr(shot, "_collision_pos", None) is not None
        ]
        _debug_log(f"Shot count after cleanup: {len(self.shots)}")

    def _explode_mine(self, mx: int, my: int, radius: int = 1, is_chain: bool = False) -> None:
        """Explode a mine, destroying everything in specified radius.

        Args:
            mx, my: Mine position
            radius: Explosion radius (1 = 360°, 2 = 2-cell radius for chain reactions)
            is_chain: True if triggered by another explosion (for visual distinction)
        """
        _debug_log(f"EXPLODE_MINE: pos=({mx},{my}) radius={radius} is_chain={is_chain}")

        current_time = self.sim_time_s()

        # Add explosion at mine position - chain reactions are larger
        self.explosions.append(
            Explosion(
                x=mx,
                y=my,
                start_time=current_time,
                duration=0.5 if not is_chain else 0.8,  # Longer for chain
                is_chain_reaction=is_chain,
            )
        )
        _debug_log(f"  -> Added explosion, is_chain={is_chain}")

        # Clear the mine itself
        self.board.set_cell_type(mx, my, CellType.EMPTY)
        _debug_log(f"  -> Cleared mine at ({mx},{my})")

        # Destroy everything in specified radius (including diagonals)
        for dy in range(-radius, radius + 1):
            for dx in range(-radius, radius + 1):
                nx, ny = mx + dx, my + dy
                if not self.board.in_bounds(nx, ny):
                    continue

                cell = self.board.get_cell(nx, ny)
                if cell is None:
                    continue

                _debug_log(f"  -> Check cell ({nx},{ny}): type={cell.type}")

                # Destroy walls/terrain
                if cell.type == CellType.WALL:
                    self.board.set_cell_type(nx, ny, CellType.EMPTY)
                    _debug_log(f"  -> Destroyed WALL at ({nx},{ny})")
                # Trigger other mines with MAGNIFIED 2-cell radius
                elif cell.type == CellType.MINE:
                    _debug_log(f"  -> Found MINE at ({nx},{ny}), checking active mines...")
                    for mine in self.mines:
                        _debug_log(
                            f"      Checking mine at ({mine.x},{mine.y}): active={mine.active}"
                        )
                        if mine.x == nx and mine.y == ny and mine.active:
                            mine.active = False
                            _debug_log(f"  -> Chain reaction! Triggering mine at ({nx},{ny})")
                            self._explode_mine(nx, ny, radius=2, is_chain=True)  # Chain reaction!
                            break
                # Damage tanks
                elif cell.type in {CellType.TANK1, CellType.TANK2}:
                    target_id = 1 if cell.type == CellType.TANK1 else 2
                    _debug_log(f"  -> Hit TANK{target_id} at ({nx},{ny})")
                    self._tank_hit(self.tanks[target_id], (nx, ny))
                # Destroy wreckage
                elif cell.type in {CellType.WRECKAGE_P1, CellType.WRECKAGE_P2}:
                    self.board.set_cell_type(nx, ny, CellType.EMPTY)

        inv_r = radius + 2
        for ai in self._ai.values():
            ai.navigation.invalidate_region(mx, my, inv_r)

    def _update_explosions(self) -> None:
        now = self.sim_time_s()
        self.explosions = [exp for exp in self.explosions if now - exp.start_time < exp.duration]
