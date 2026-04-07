from __future__ import annotations

import os
import random
from collections import deque
from dataclasses import dataclass
from enum import Enum, auto
from typing import TYPE_CHECKING, Any

from .ai_navigation import NavigationMemory
from .board import Board, CellType
from .player import DIRECTION_VECTORS, Direction

if TYPE_CHECKING:
    from py_trees.trees import BehaviourTree


class AIAction(Enum):
    """Possible actions for the AI to take."""

    MOVE_TOWARD_ENEMY = auto()
    MOVE_AWAY_FROM_ENEMY = auto()
    FIRE = auto()
    PLACE_MINE = auto()
    DODGE_SHOT = auto()
    SCAN = auto()  # Look around for enemy
    RANDOM_MOVE = auto()  # Get unstuck
    WAIT = auto()
    RISKY_PREPARE = auto()  # FR-8: turn barrel to wild direction before risky shot (human parity)


def _map_cells(fraction: float) -> int:
    """Convert a map-relative fraction to cells (floored, minimum 1)."""
    from .constants import SCREEN_HEIGHT_CELLS, SCREEN_WIDTH_CELLS

    base = min(SCREEN_WIDTH_CELLS, SCREEN_HEIGHT_CELLS)
    return max(1, int(base * fraction))


@dataclass
class AIConfig:
    """AI configuration based on difficulty level (0-9).

    Range/radius fields ending in ``_pct`` are fractions of
    ``min(map_width, map_height)``; resolve to cells via ``_map_cells``.
    """

    reaction_time: float
    peripheral_radius_pct: float
    scan_frequency: int
    mine_memory_decay: float
    confidence_threshold: float
    dodge_chance: float
    aggressive_mines: float  # 0.0 = never, 0.5 = sometimes, 1.0 = always
    ammo_discipline: int
    engage_range_pct: float
    los_range_pct: float
    risky_enabled: bool
    frustration_threshold: int
    skip_fire_base: float
    scan_when_blind: bool

    @property
    def peripheral_radius(self) -> int:
        return _map_cells(self.peripheral_radius_pct)

    @property
    def engage_range(self) -> int:
        return _map_cells(self.engage_range_pct)

    @property
    def los_range(self) -> int:
        return _map_cells(self.los_range_pct)


# fmt: off
_SKILL_TABLE: tuple[AIConfig, ...] = (
    #                react   periph  scan  mine_dec  conf   dodge  mines  disc  engage  los     risky frust  skip   scan_blind
    AIConfig(        0.200,  0.12,    0,   0.30,     0.8,   0.00,  0.00,  0,   0.16,   0.16,   True,  50,   0.30,  False),  # 0
    AIConfig(        0.170,  0.14,    1,   0.27,     1.0,   0.10,  0.05,  1,   0.20,   0.24,   True,  47,   0.27,  False),  # 1
    AIConfig(        0.145,  0.18,    2,   0.24,     1.2,   0.20,  0.10,  2,   0.24,   0.32,   True,  44,   0.23,  False),  # 2
    AIConfig(        0.120,  0.22,    3,   0.21,     1.5,   0.30,  0.20,  3,   0.28,   0.40,   True,  41,   0.20,  False),  # 3
    AIConfig(        0.100,  0.24,    4,   0.18,     1.8,   0.40,  0.30,  4,   0.32,   0.48,   True,  38,   0.17,  False),  # 4
    AIConfig(        0.080,  0.28,    5,   0.15,     2.2,   0.50,  0.45,  5,   0.36,   0.56,   True,  35,   0.13,  True),   # 5
    AIConfig(        0.065,  0.32,    6,   0.12,     2.5,   0.60,  0.60,  6,   0.44,   0.72,   False, 32,   0.10,  True),   # 6
    AIConfig(        0.050,  0.36,    7,   0.09,     2.8,   0.70,  0.75,  7,   0.56,   0.88,   False, 29,   0.07,  True),   # 7
    AIConfig(        0.038,  0.40,    8,   0.06,     3.2,   0.80,  0.88,  8,   0.72,   1.20,   False, 26,   0.03,  True),   # 8
    AIConfig(        0.025,  0.48,    9,   0.03,     3.8,   0.90,  1.00,  9,   1.60,   4.00,   False, 20,   0.00,  True),   # 9
)
# fmt: on


def get_ai_config(
    difficulty: int,
    *,
    move_delay: float | None = None,
) -> AIConfig:
    """Return the skill profile for *difficulty* (0-9).

    When *move_delay* is provided (Demo / SHOWDOWN modes), reaction_time is
    scaled relative to the table baseline so game-speed changes propagate.
    """
    d = max(0, min(9, difficulty))
    cfg = _SKILL_TABLE[d]
    if move_delay is not None:
        skill_speed = 0.55 + (d / 9.0) * 0.45
        scaled_rt = max(0.02, (move_delay / 2.0) / skill_speed)
        cfg = AIConfig(
            reaction_time=scaled_rt,
            peripheral_radius_pct=cfg.peripheral_radius_pct,
            scan_frequency=cfg.scan_frequency,
            mine_memory_decay=cfg.mine_memory_decay,
            confidence_threshold=cfg.confidence_threshold,
            dodge_chance=cfg.dodge_chance,
            aggressive_mines=cfg.aggressive_mines,
            ammo_discipline=cfg.ammo_discipline,
            engage_range_pct=cfg.engage_range_pct,
            los_range_pct=cfg.los_range_pct,
            risky_enabled=cfg.risky_enabled,
            frustration_threshold=cfg.frustration_threshold,
            skip_fire_base=cfg.skip_fire_base,
            scan_when_blind=cfg.scan_when_blind,
        )
    return cfg


from .ai_tree import log_behaviour_tree_tip  # noqa: E402 — after AIAction for import order


class AIPlayer:
    """AI opponent for single-player mode."""

    def __init__(
        self,
        difficulty: int,
        start_pos: tuple[int, int],
        enemy_start_pos: tuple[int, int],
        move_delay: float | None = None,
    ):
        self.config = get_ai_config(difficulty, move_delay=move_delay)
        self.difficulty = difficulty

        # Positions
        self.start_pos = start_pos
        self.enemy_start_pos = enemy_start_pos
        self.last_known_enemy_pos: tuple[int, int] | None = None

        # Mine memory: list of (x, y, confidence) - confidence decays
        self.mine_memory: list[tuple[int, int, float]] = []

        # State
        self._last_action_time: float = 0.0
        self._scan_direction: int = 0  # Which direction currently scanning
        self._last_scan_decision_time: float = -100.0  # throttle SCAN when blind (corner twitch)

        # Stuck detection
        self._stuck_counter: int = 0  # Consecutive cycles without making progress
        self._last_pos: tuple[int, int] | None = None
        self._recent_positions: list[tuple[int, int]] = []  # Track recently visited positions

        # Evasion cooldown - prevent getting stuck in evasion loops
        self._evasion_cooldown: int = 0  # Cycles to wait before evading again

        # Evasion mode - stay in evasion for a random duration
        self._evasion_end_time: float = 0.0  # When current evasion should end
        self._is_evading: bool = False  # Currently in evasion mode
        # When primary horizontal "away" hits the map rim, avoid flipping UP/DOWN every tick (see pick_evade_direction)
        self._evade_vertical_commit: Direction | None = None
        # One sample per reaction decision (bind_tick_inputs) — detects A↔B loops even during ActiveEvasion
        self._decision_position_ring: deque[tuple[int, int]] = deque(maxlen=12)

        # Frustration counter - if too long without seeing enemy, fire blindly
        self._last_seen_enemy_time: float = 0.0  # Last time we saw enemy
        self._frustration_cycles: int = 0  # Cycles without seeing enemy

        # Per-tick inputs for behaviour tree leaves (PRD §5.1)
        self._tick_current_time: float = 0.0
        self._tick_board: Board | None = None
        self._tick_my_pos: tuple[int, int] = (0, 0)
        self._tick_my_direction: Direction = Direction.UP
        self._tick_my_shots: int = 0
        self._tick_my_mines: int = 0
        self._tick_enemy_pos: tuple[int, int] | None = None
        self._tick_shots_on_board: list = []
        self._tick_player_id: int | None = None
        self._pending_decision_time: float = 0.0
        self._staged_update_args: Any = None  # set by _stage_for_tree_tick for ReactionTimeGate

        self._combat_enemy_pos: tuple[int, int] | None = None
        self._pending_fire_risky: bool = False
        # FR-8 risky wild shot: must face shot dir like humans — two-step aim then fire
        self._risky_wild_state: str = "idle"  # idle | pending_aim | ready_to_fire
        self._risky_jitter_direction: Direction = Direction.UP
        self._risky_saved_direction: Direction = Direction.UP

        self._pending_action: AIAction | None = None
        self._behaviour_tree: BehaviourTree | None = None
        self.navigation = NavigationMemory(difficulty)
        if not os.environ.get("TANK_LEGACY_AI"):
            from .ai_tree import build_ai_tree

            self._behaviour_tree = build_ai_tree(self)

    def _bind_tick_inputs(
        self,
        current_time: float,
        board: Board,
        my_pos: tuple[int, int],
        my_direction: Direction,
        my_shots: int,
        my_mines: int,
        enemy_pos: tuple[int, int] | None,
        shots_on_board: list,
        *,
        player_id: int | None = None,
    ) -> None:
        self._tick_current_time = current_time
        self._tick_board = board
        self._tick_my_pos = my_pos
        self._tick_my_direction = my_direction
        self._tick_my_shots = my_shots
        self._tick_my_mines = my_mines
        self._tick_enemy_pos = enemy_pos
        self._tick_shots_on_board = shots_on_board
        self._tick_player_id = player_id
        self.navigation.observe_tick(
            board,
            my_pos,
            my_direction,
            self.config.peripheral_radius,
            current_time,
        )
        self._decision_position_ring.append(my_pos)

    def _oscillation_abab_detected(self) -> bool:
        """True if last four decision positions are A,B,A,B (two-cell ping-pong)."""
        ring = self._decision_position_ring
        if len(ring) < 4:
            return False
        a, b, c, d = (ring[-4], ring[-3], ring[-2], ring[-1])
        return a == c and b == d and a != b

    def clear_decision_position_ring(self) -> None:
        """Call after a deliberate breakout (e.g. RANDOM_MOVE) to avoid immediate re-trigger."""
        self._decision_position_ring.clear()

    def _sync_combat_perception(self) -> None:
        """FR-9: track last known from cone or peripheral; combat aim only in forward cone."""
        from .ai_perception import combat_visible_enemy, should_track_enemy

        self._combat_enemy_pos = None
        raw = self._tick_enemy_pos
        if not raw:
            return
        assert self._tick_board is not None
        if should_track_enemy(
            self._tick_my_pos, self._tick_my_direction, raw, self.config.peripheral_radius
        ):
            self.last_known_enemy_pos = raw
        self._combat_enemy_pos = combat_visible_enemy(
            self._tick_my_pos, self._tick_my_direction, raw
        )

    def _evasion_bookkeeping_after_reaction_commit(self) -> None:
        """Runs once per decision frame when the reaction gate opens (PRD S1)."""
        t = self._pending_decision_time
        if self._evasion_cooldown > 0:
            self._evasion_cooldown -= 1
        if self._is_evading and t >= self._evasion_end_time:
            self._is_evading = False
            self._evasion_cooldown = 5
        if not self._is_evading:
            self._evade_vertical_commit = None

    def _stage_for_tree_tick(
        self,
        current_time: float,
        board: Board,
        my_pos: tuple[int, int],
        my_direction: Direction,
        my_shots: int,
        my_mines: int,
        enemy_pos: tuple[int, int] | None,
        shots_on_board: list,
        player_id: int | None = None,
    ) -> None:
        self._pending_decision_time = current_time
        self._staged_update_args = (
            board,
            my_pos,
            my_direction,
            my_shots,
            my_mines,
            enemy_pos,
            shots_on_board,
            player_id,
        )

    def update(
        self,
        current_time: float,
        board: Board,
        my_pos: tuple[int, int],
        my_direction: Direction,
        my_shots: int,
        my_mines: int,
        enemy_pos: tuple[int, int] | None,
        shots_on_board: list,
        *,
        player_id: int | None = None,
    ) -> AIAction | None:
        """Main AI update - called every frame. Returns action to take."""

        if os.environ.get("TANK_LEGACY_AI"):
            if current_time - self._last_action_time < self.config.reaction_time:
                return None
            self._last_action_time = current_time
            self._pending_decision_time = current_time
            self._evasion_bookkeeping_after_reaction_commit()
            if self._is_evading:
                return AIAction.MOVE_AWAY_FROM_ENEMY
            self._bind_tick_inputs(
                current_time,
                board,
                my_pos,
                my_direction,
                my_shots,
                my_mines,
                enemy_pos,
                shots_on_board,
                player_id=player_id,
            )
            if enemy_pos is None:
                self._frustration_cycles += 1
            else:
                self._frustration_cycles = 0
                self._last_seen_enemy_time = current_time
            self._sync_combat_perception()
            return self._update_legacy_decision(
                board, my_pos, my_direction, my_shots, my_mines, enemy_pos, shots_on_board
            )

        self._stage_for_tree_tick(
            current_time,
            board,
            my_pos,
            my_direction,
            my_shots,
            my_mines,
            enemy_pos,
            shots_on_board,
            player_id=player_id,
        )
        self._pending_action = None
        self._pending_fire_risky = False
        if self._behaviour_tree is None:
            from .ai_tree import build_ai_tree

            self._behaviour_tree = build_ai_tree(self)

        self._behaviour_tree.tick()
        log_behaviour_tree_tip(self._behaviour_tree, player_id)
        return self._pending_action

    def _update_legacy_decision(
        self,
        board: Board,
        my_pos: tuple[int, int],
        my_direction: Direction,
        my_shots: int,
        my_mines: int,
        enemy_pos: tuple[int, int] | None,
        shots_on_board: list,
    ) -> AIAction | None:
        """Pre–behaviour-tree decision logic (``TANK_LEGACY_AI=1`` only)."""
        if self._frustration_cycles >= self.config.frustration_threshold:
            return AIAction.MOVE_TOWARD_ENEMY

        dodge_action = self._check_dodge_shot(board, my_pos, shots_on_board)
        if dodge_action:
            return dodge_action

        if enemy_pos:
            self.last_known_enemy_pos = enemy_pos
            if self._is_too_close(enemy_pos, my_pos) and self._evasion_cooldown == 0:
                self._is_evading = True
                self._evasion_end_time = self._tick_current_time + random.uniform(1.0, 10.0)
                self._evasion_cooldown = 10
                if my_mines > 0 and random.random() < 0.5:
                    return AIAction.PLACE_MINE
                return AIAction.MOVE_AWAY_FROM_ENEMY

        if enemy_pos and self._has_forward_los(board, my_pos, my_direction, enemy_pos):
            if self._can_fire_confidently(my_shots) and not self.should_skip_fire("aimed"):
                return AIAction.FIRE

        if self.last_known_enemy_pos:
            if self._check_stuck(my_pos):
                self._recent_positions.clear()
                return AIAction.RANDOM_MOVE
            self._update_position_history(my_pos)
            return AIAction.MOVE_TOWARD_ENEMY

        if self._check_stuck(my_pos):
            self._recent_positions.clear()
            return AIAction.RANDOM_MOVE

        if self.config.scan_when_blind:
            return AIAction.SCAN
        return AIAction.MOVE_TOWARD_ENEMY

    def _check_stuck(self, my_pos: tuple[int, int]) -> bool:
        """Check if AI is stuck (not moving for a while or repeating same action/oscillating)."""
        # Check if position hasn't changed
        if self._last_pos == my_pos:
            self._stuck_counter += 1
        else:
            self._stuck_counter = 0
            self._last_pos = my_pos

        # Consider stuck after 5+ consecutive decision cycles at same position
        if self._stuck_counter >= 5:
            return True

        # Check for oscillation: if we keep bouncing between positions
        # Need at least 4 positions tracked to detect oscillation
        if len(self._recent_positions) >= 4:
            # Count how many times we've been at current position in recent history
            recent_count = self._recent_positions.count(my_pos)
            if recent_count >= 2:  # Been here 2+ times in last few cycles = oscillating
                return True

        return False

    def _update_position_history(self, my_pos: tuple[int, int]) -> None:
        """Track recently visited positions to detect oscillation."""
        self._recent_positions.append(my_pos)
        # Keep only last 8 positions
        if len(self._recent_positions) > 8:
            self._recent_positions.pop(0)

    def _check_dodge_shot(
        self, board: Board, my_pos: tuple[int, int], shots_on_board: list
    ) -> AIAction | None:
        """Check if there's a shot incoming and decide whether to dodge."""
        if self.config.dodge_chance <= 0.0:
            return None
        if not shots_on_board:
            return None

        # Random chance to dodge based on difficulty
        if random.random() > self.config.dodge_chance:
            return None

        # Find shots heading toward us
        for shot in shots_on_board:
            # Simple check: if shot is close and moving toward us
            dx = abs(shot.x - my_pos[0])
            dy = abs(shot.y - my_pos[1])
            if dx + dy <= 3:  # Shot is close
                return AIAction.DODGE_SHOT

        return None

    def _within_engagement_range(self, my_pos: tuple[int, int], target: tuple[int, int]) -> bool:
        dist_x = abs(target[0] - my_pos[0])
        dist_y = abs(target[1] - my_pos[1])
        max_range = self.config.engage_range
        return not (dist_x > max_range and dist_y > max_range)

    def risky_shot_viable(
        self,
        board: Board,
        my_pos: tuple[int, int],
        direction: Direction,
        target: tuple[int, int],
    ) -> bool:
        """FR-8: in cone and range, but no clear LOS — snap shot."""
        from .ai_perception import enemy_in_forward_cone

        if not self.config.risky_enabled:
            return False
        if not enemy_in_forward_cone(my_pos, direction, target):
            return False
        if not self._within_engagement_range(my_pos, target):
            return False
        if self._has_forward_los(board, my_pos, direction, target):
            return False
        return True

    def effective_min_shots_for_aimed(self) -> int:
        """FR-10: base reserve from ``confidence_threshold`` plus extra from ``ammo_discipline``."""
        import math

        return math.ceil(self.config.confidence_threshold) + (self.config.ammo_discipline // 3)

    def should_skip_fire(self, kind: str = "aimed") -> bool:
        """FR-8: low skill sometimes hesitates and does not take a shot this tick."""
        base = self.config.skip_fire_base
        if kind == "risky":
            p = min(0.50, base * 1.4)
        else:
            p = min(0.35, base)
        return random.random() < p

    def _can_fire_risky(self, shots_left: int) -> bool:
        thr = max(1, self.effective_min_shots_for_aimed() - 2)
        return shots_left >= thr

    def _has_forward_los(
        self, board: Board, my_pos: tuple[int, int], direction: Direction, target: tuple[int, int]
    ) -> bool:
        """Check if target is in forward line of sight within range limit."""
        dx, dy = DIRECTION_VECTORS[direction]

        dist_x = abs(target[0] - my_pos[0])
        dist_y = abs(target[1] - my_pos[1])
        max_range = self.config.los_range

        if dist_x > max_range and dist_y > max_range:
            return False

        # Raycast in facing direction
        results = board.raycast(my_pos[0], my_pos[1], dx, dy)

        for x, y, cell_type in results:
            if x == target[0] and y == target[1]:
                return True
            # Stop at anything blocking
            if cell_type not in {CellType.EMPTY, CellType.SHOT, CellType.MINE}:
                break

        return False

    def _can_fire_confidently(self, shots_left: int) -> bool:
        """Determine if AI has enough ammo to fire confidently."""
        return shots_left >= self.effective_min_shots_for_aimed()

    def _near_board_edge(self, board: Board, my_pos: tuple[int, int], margin: int = 3) -> bool:
        """True when close to outer playable rim — diagonal-first pathing tends to hug walls here."""
        x, y = my_pos
        return (
            x <= margin
            or x >= board.width - 1 - margin
            or y <= margin
            or y >= board.height - 1 - margin
        )

    def _greedy_direction_order(
        self, board: Board, my_pos: tuple[int, int], target: tuple[int, int]
    ) -> list[Direction]:
        """Preference order for greedy leg (matches prior rim / diagonal heuristic)."""
        my_x, my_y = my_pos
        target_x, target_y = target
        dx = 1 if target_x > my_x else (-1 if target_x < my_x else 0)
        dy = 1 if target_y > my_y else (-1 if target_y < my_y else 0)

        def diagonal_dir() -> Direction | None:
            if dx == 0 or dy == 0:
                return None
            if dx < 0 and dy < 0:
                return Direction.UP_LEFT
            if dx > 0 and dy < 0:
                return Direction.UP_RIGHT
            if dx < 0 and dy > 0:
                return Direction.DOWN_LEFT
            return Direction.DOWN_RIGHT

        cardinals: list[Direction] = []
        if dx != 0:
            cardinals.append(Direction.RIGHT if dx > 0 else Direction.LEFT)
        if dy != 0:
            cardinals.append(Direction.DOWN if dy > 0 else Direction.UP)

        dd = diagonal_dir()
        edge = self._near_board_edge(board, my_pos)
        ordered: list[Direction] = []
        if edge:
            ordered.extend(cardinals)
            if dd is not None:
                ordered.append(dd)
        else:
            if dd is not None:
                ordered.append(dd)
            ordered.extend(cardinals)
        rest = [
            Direction.UP,
            Direction.DOWN,
            Direction.LEFT,
            Direction.RIGHT,
            Direction.UP_LEFT,
            Direction.UP_RIGHT,
            Direction.DOWN_LEFT,
            Direction.DOWN_RIGHT,
        ]
        for d in rest:
            if d not in ordered:
                ordered.append(d)
        return ordered

    def get_movement_toward_target(
        self,
        board: Board,
        my_pos: tuple[int, int],
        target: tuple[int, int],
        *,
        blind: bool = False,
        current_time: float | None = None,
    ) -> Direction | None:
        """Calculate best direction to move toward target (navigation memory + hybrid A* / greedy)."""
        t = current_time if current_time is not None else self._tick_current_time

        def ok(d: Direction) -> bool:
            return self._can_move(board, my_pos, d) and self.avoid_own_mines(board, my_pos, d)

        greedy_order = self._greedy_direction_order(board, my_pos, target)
        return self.navigation.choose_movement_direction(
            board,
            my_pos,
            target,
            t,
            blind=blind,
            try_direction_fn=ok,
            greedy_order=greedy_order,
        )

    def _rim_blocks_horizontal_away(self, board: Board, mx: int, my: int, ex: int, ey: int) -> bool:
        """Enemy lies to one side but we cannot step further that way (outer wall)."""
        if ex > mx and mx <= 1:
            return True
        if ex < mx and mx >= board.width - 2:
            return True
        return False

    def pick_evade_direction(
        self,
        board: Board,
        my_pos: tuple[int, int],
        away_from: tuple[int, int],
        *,
        current_time: float | None = None,
    ) -> Direction | None:
        """Single evade step away from ``away_from`` (greedy + tabu; no long-range A*)."""
        t = current_time if current_time is not None else self._tick_current_time
        self.navigation.tick_time(t)
        mx, my = my_pos
        ex, ey = away_from
        dx = -1 if ex > mx else (1 if ex < mx else 0)
        dy = -1 if ey > my else (1 if ey < my else 0)

        def ok(d: Direction) -> bool:
            return self._can_move(board, my_pos, d) and self.avoid_own_mines(board, my_pos, d)

        rim_v = self._rim_blocks_horizontal_away(board, mx, my, ex, ey)
        if rim_v and self._evade_vertical_commit in (Direction.UP, Direction.DOWN):
            d0 = self._evade_vertical_commit
            if ok(d0) and not self.navigation.is_tabu(my_pos, d0, t):
                return d0
            alt = Direction.DOWN if d0 == Direction.UP else Direction.UP
            if ok(alt) and not self.navigation.is_tabu(my_pos, alt, t):
                self._evade_vertical_commit = alt
                return alt

        order: list[Direction] = []
        if dx != 0 and dy != 0:
            if dx < 0 and dy < 0:
                order.append(Direction.UP_LEFT)
            elif dx > 0 and dy < 0:
                order.append(Direction.UP_RIGHT)
            elif dx < 0 and dy > 0:
                order.append(Direction.DOWN_LEFT)
            else:
                order.append(Direction.DOWN_RIGHT)
        if dx != 0:
            order.append(Direction.RIGHT if dx > 0 else Direction.LEFT)
        if dy != 0:
            order.append(Direction.DOWN if dy > 0 else Direction.UP)
        for d in [
            Direction.UP,
            Direction.DOWN,
            Direction.LEFT,
            Direction.RIGHT,
            Direction.UP_LEFT,
            Direction.UP_RIGHT,
            Direction.DOWN_LEFT,
            Direction.DOWN_RIGHT,
        ]:
            if d not in order:
                order.append(d)

        for d in order:
            if self.navigation.is_tabu(my_pos, d, t):
                continue
            if ok(d):
                if (
                    rim_v
                    and self._evade_vertical_commit is None
                    and d in (Direction.UP, Direction.DOWN)
                ):
                    self._evade_vertical_commit = d
                return d
        return None

    def _can_move(self, board: Board, pos: tuple[int, int], direction: Direction) -> bool:
        """Check if can move in direction."""
        dx, dy = DIRECTION_VECTORS[direction]
        new_x, new_y = pos[0] + dx, pos[1] + dy

        # Let attempt_move handle clamping - just check raw bounds here
        if not board.in_bounds(new_x, new_y):
            return False

        cell = board.get_cell(new_x, new_y)
        if cell is None:
            return False

        # Can move through empty, shots, mines (but avoid own mines)
        return cell.type in {CellType.EMPTY, CellType.SHOT, CellType.MINE}

    def _is_too_close(self, enemy_pos: tuple[int, int], my_pos: tuple[int, int]) -> bool:
        """Check if enemy is too close - trigger evasion."""
        dx = abs(enemy_pos[0] - my_pos[0])
        dy = abs(enemy_pos[1] - my_pos[1])

        # If touching (0 squares) or 2-3 squares apart (face to face), ALWAYS trigger evasion
        # This prevents getting stuck in [P1][P2] or [P1][space][space][P2] patterns
        if dx <= 3 and dy == 0:
            return True
        if dx == 0 and dy <= 3:
            return True

        # Manhattan distance <= 3 for other cases
        return (dx + dy) <= 3

    def remember_mine(self, x: int, y: int) -> None:
        """Remember where we placed a mine."""
        # Add with full confidence
        self.mine_memory.append((x, y, 1.0))

    def update_mine_memory(self) -> None:
        """Update mine memory - decay confidence or forget positions."""
        if not self.mine_memory:
            return

        new_memory = []
        for x, y, confidence in self.mine_memory:
            # Decay confidence
            new_confidence = confidence - self.config.mine_memory_decay
            if new_confidence > 0:
                new_memory.append((x, y, new_confidence))

        self.mine_memory = new_memory

    def avoid_own_mines(self, board: Board, my_pos: tuple[int, int], direction: Direction) -> bool:
        """Check if direction would lead into own remembered mine."""
        if not self.mine_memory:
            return True  # No memory, safe to proceed

        dx, dy = DIRECTION_VECTORS[direction]
        new_x, new_y = my_pos[0] + dx, my_pos[1] + dy

        mine_conf_gate = 0.5 if self.config.scan_when_blind else 0.8
        for mem_x, mem_y, confidence in self.mine_memory:
            if mem_x == new_x and mem_y == new_y:
                if confidence > mine_conf_gate:
                    return False

        return True

    def get_scan_direction(self) -> Direction:
        """Get next direction to scan based on current scan state."""
        scan_dirs = [Direction.RIGHT, Direction.DOWN, Direction.LEFT, Direction.UP]
        direction = scan_dirs[self._scan_direction % 4]
        self._scan_direction += 1
        return direction
