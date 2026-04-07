"""PRD AI navigation memory: tabu edges, terrain belief, hybrid greedy / A* (strict fairness)."""

from __future__ import annotations

import heapq
import math
from collections.abc import Callable
from dataclasses import dataclass

from .board import Board, CellType
from .player import DIRECTION_VECTORS, Direction

# Stable 8-way order for tabu keys
ALL_MOVE_DIRECTIONS: tuple[Direction, ...] = (
    Direction.UP,
    Direction.DOWN,
    Direction.LEFT,
    Direction.RIGHT,
    Direction.UP_LEFT,
    Direction.UP_RIGHT,
    Direction.DOWN_LEFT,
    Direction.DOWN_RIGHT,
)


@dataclass(frozen=True, slots=True)
class NavigationConfig:
    """Linear skill scaling (difficulty 0-9).

    ``sight_radius_pct`` and ``greedy_distance_pct`` are fractions of
    ``min(map_width, map_height)``; resolved to cell counts at init.
    """

    difficulty: int
    sight_radius_pct: float
    tabu_max: int
    tabu_ttl_s: float
    decay_per_second: float
    respawn_confidence_factor: float
    search_budget: int
    greedy_distance_pct: float
    wall_block_threshold: float


# fmt: off
_NAV_TABLE: tuple[tuple[float, int, float, float, float, int, float, float], ...] = (
    #  sight_pct  tabu  tabu_ttl  decay   respawn  budget  greedy_pct  wall_thr
    (  0.08,       8,   0.45,     0.015,  0.52,     80,    0.32,       0.52),   # 0
    (  0.11,      12,   0.47,     0.019,  0.56,     86,    0.29,       0.53),   # 1
    (  0.14,      16,   0.50,     0.023,  0.59,     91,    0.26,       0.53),   # 2
    (  0.18,      20,   0.52,     0.027,  0.63,     97,    0.23,       0.54),   # 3
    (  0.22,      24,   0.55,     0.031,  0.67,    102,    0.20,       0.55),   # 4
    (  0.26,      28,   0.57,     0.035,  0.71,    108,    0.18,       0.56),   # 5
    (  0.30,      32,   0.59,     0.038,  0.74,    113,    0.15,       0.56),   # 6
    (  0.34,      36,   0.62,     0.042,  0.78,    119,    0.12,       0.57),   # 7
    (  0.38,      40,   0.64,     0.046,  0.82,    124,    0.10,       0.58),   # 8
    (  0.44,      44,   0.67,     0.050,  0.86,    130,    0.08,       0.58),   # 9
)
# fmt: on


def navigation_config_for_difficulty(difficulty: int) -> NavigationConfig:
    """Return navigation config for *difficulty* (0-9) using the explicit table."""
    d = max(0, min(9, difficulty))
    row = _NAV_TABLE[d]
    return NavigationConfig(
        difficulty=d,
        sight_radius_pct=row[0],
        tabu_max=row[1],
        tabu_ttl_s=row[2],
        decay_per_second=row[3],
        respawn_confidence_factor=row[4],
        search_budget=row[5],
        greedy_distance_pct=row[6],
        wall_block_threshold=row[7],
    )


class NavigationMemory:
    """Tabu directed edges + wall belief + observed cells; strict fairness (PRD NAV)."""

    def __init__(self, difficulty: int) -> None:
        self.cfg = navigation_config_for_difficulty(difficulty)
        from .ai import _map_cells

        self.sight_radius: int = _map_cells(self.cfg.sight_radius_pct)
        self.greedy_distance: int = _map_cells(self.cfg.greedy_distance_pct)
        self._tabu: dict[tuple[int, int, Direction], float] = {}
        self._wall_belief: dict[tuple[int, int], float] = {}
        self._observed_static: set[tuple[int, int]] = set()
        self._last_time: float = 0.0
        self._prev_decay_time: float | None = None
        self._astar_cache: tuple[tuple[int, int], tuple[int, int], tuple[int, int] | None] | None = None

    def reset_full(self) -> None:
        self._tabu.clear()
        self._wall_belief.clear()
        self._observed_static.clear()
        self._astar_cache = None

    def on_respawn(self, current_time: float) -> None:
        """NAV-LIFE-3: soften beliefs; do not clear."""
        self.tick_time(current_time)
        f = self.cfg.respawn_confidence_factor
        new_b: dict[tuple[int, int], float] = {}
        for k, v in self._wall_belief.items():
            nv = v * f
            if nv >= 0.12:
                new_b[k] = nv
        self._wall_belief = new_b
        # Shorten tabu lives
        new_t: dict[tuple[int, int, Direction], float] = {}
        for k, exp in self._tabu.items():
            if exp > current_time:
                new_t[k] = current_time + (exp - current_time) * 0.5
        self._tabu = new_t

    def invalidate_region(self, cx: int, cy: int, radius: int) -> None:
        """NAV-LIFE-5: terrain may have changed (explosion)."""
        to_del: list[tuple[int, int]] = []
        for x, y in list(self._wall_belief.keys()):
            if max(abs(x - cx), abs(y - cy)) <= radius:
                to_del.append((x, y))
        for k in to_del:
            self._wall_belief.pop(k, None)
            self._observed_static.discard(k)
        self._astar_cache = None

    def tick_time(self, current_time: float) -> None:
        """Decay tabu expiry bookkeeping + wall belief drift."""
        self._last_time = current_time
        if self._prev_decay_time is None:
            self._prev_decay_time = current_time
        dt = max(0.0, current_time - self._prev_decay_time)
        self._prev_decay_time = current_time

        # Drop expired tabu
        dead = [k for k, exp in self._tabu.items() if exp <= current_time]
        for k in dead:
            del self._tabu[k]

        # Belief decay toward 0 (forget walls)
        if dt > 0.001 and self._wall_belief:
            decay = math.exp(-dt * self.cfg.decay_per_second)
            new_b: dict[tuple[int, int], float] = {}
            for k, v in self._wall_belief.items():
                nv = v * decay
                if nv >= 0.08:
                    new_b[k] = nv
            self._wall_belief = new_b

    def observe_tick(
        self, board: Board, my_pos: tuple[int, int], _direction: Direction, _peripheral: int, current_time: float
    ) -> None:
        """Update observed terrain from peripheral LOS (same helper as combat peripheral)."""
        self.tick_time(current_time)
        self._observed_static.add(my_pos)
        r = self.sight_radius
        seen = board.get_peripheral_vision(my_pos[0], my_pos[1], r)
        for x, y, ctype in seen:
            self._observed_static.add((x, y))
            if ctype == CellType.WALL:
                self._wall_belief[(x, y)] = 1.0
            elif ctype == CellType.EMPTY:
                self._wall_belief[(x, y)] = 0.0

    def record_failed_attempt(
        self, board: Board, pos: tuple[int, int], direction: Direction, current_time: float
    ) -> None:
        """NAV-2: primary source — failed attempt_move. Tabu edge; wall belief if we hit static wall."""
        self.tick_time(current_time)
        px, py = pos
        dx, dy = DIRECTION_VECTORS[direction]
        raw_x, raw_y = px + dx, py + dy
        nx = max(1, min(raw_x, board.width - 2))
        ny = max(1, min(raw_y, board.height - 2))
        if (nx, ny) == (px, py):
            # Clamped — border bump
            self._tabu[(px, py, direction)] = current_time + self.cfg.tabu_ttl_s
            self._trim_tabu()
            return

        self._observed_static.add((nx, ny))
        self._tabu[(px, py, direction)] = current_time + self.cfg.tabu_ttl_s
        cell = board.get_cell(nx, ny)
        if cell and cell.type == CellType.WALL:
            self._wall_belief[(nx, ny)] = 1.0

        self._trim_tabu()

    def record_fallback_blocked(self, pos: tuple[int, int], direction: Direction, current_time: float) -> None:
        """Optional C: no board peek — tabu only."""
        self.tick_time(current_time)
        self._tabu[(pos[0], pos[1], direction)] = current_time + self.cfg.tabu_ttl_s
        self._trim_tabu()

    def _trim_tabu(self) -> None:
        if len(self._tabu) <= self.cfg.tabu_max:
            return
        # Drop soonest expiring
        items = sorted(self._tabu.items(), key=lambda kv: kv[1])
        for k, _ in items[: max(1, len(self._tabu) - self.cfg.tabu_max)]:
            del self._tabu[k]

    def is_tabu(self, pos: tuple[int, int], direction: Direction, current_time: float) -> bool:
        exp = self._tabu.get((pos[0], pos[1], direction))
        return exp is not None and exp > current_time

    def _static_blocks_planner(self, board: Board, x: int, y: int) -> bool:
        """Strict: unknown static cells are not read from Board for blocking (except via belief)."""
        wb = self._wall_belief.get((x, y))
        if wb is not None and wb >= self.cfg.wall_block_threshold:
            return True
        if (x, y) in self._observed_static:
            cell = board.get_cell(x, y)
            if cell and cell.type == CellType.WALL:
                return True
        return False

    def _blocked_dynamic(self, board: Board, x: int, y: int) -> bool:
        """Tanks / wreckage — always use real board (fair: everyone sees units)."""
        cell = board.get_cell(x, y)
        if cell is None:
            return True
        return cell.type in {
            CellType.TANK1,
            CellType.TANK2,
            CellType.BARREL1,
            CellType.BARREL2,
            CellType.WRECKAGE_P1,
            CellType.WRECKAGE_P2,
        }

    def _neighbors_passable_for_astar(self, board: Board, x: int, y: int) -> list[tuple[int, int, float]]:
        out: list[tuple[int, int, float]] = []
        for dx, dy in (
            (-1, 0),
            (1, 0),
            (0, -1),
            (0, 1),
            (-1, -1),
            (1, -1),
            (-1, 1),
            (1, 1),
        ):
            nx, ny = x + dx, y + dy
            if not board.in_bounds(nx, ny):
                continue
            if nx == 0 or ny == 0 or nx == board.width - 1 or ny == board.height - 1:
                continue
            if self._static_blocks_planner(board, nx, ny):
                continue
            if self._blocked_dynamic(board, nx, ny):
                continue
            step = math.sqrt(2.0) if dx != 0 and dy != 0 else 1.0
            out.append((nx, ny, step))
        return out

    def _astar_first_step(
        self,
        board: Board,
        start: tuple[int, int],
        goal: tuple[int, int],
    ) -> tuple[int, int] | None:
        """Return next cell toward goal, or None."""
        if start == goal:
            return None

        cached = self._astar_cache
        if cached is not None and cached[0] == start and cached[1] == goal:
            return cached[2]

        budget = self.cfg.search_budget

        def h(a: tuple[int, int], b: tuple[int, int]) -> float:
            return abs(a[0] - b[0]) + abs(a[1] - b[1])

        open_heap: list[tuple[float, int, tuple[int, int]]] = []
        heapq.heappush(open_heap, (h(start, goal), 0, start))
        came: dict[tuple[int, int], tuple[int, int] | None] = {start: None}
        g_score: dict[tuple[int, int], float] = {start: 0.0}
        closed: set[tuple[int, int]] = set()
        expansions = 0

        result: tuple[int, int] | None = None
        while open_heap and expansions < budget:
            _, _f, current = heapq.heappop(open_heap)
            if current in closed:
                continue
            closed.add(current)
            expansions += 1
            if current == goal:
                node = goal
                while came.get(node) is not None:
                    parent = came[node]
                    if parent == start:
                        result = node
                        break
                    node = parent
                break

            for nx, ny, step_cost in self._neighbors_passable_for_astar(board, current[0], current[1]):
                tentative = g_score[current] + step_cost
                if tentative < g_score.get((nx, ny), float("inf")):
                    g_score[(nx, ny)] = tentative
                    came[(nx, ny)] = current
                    f = tentative + h((nx, ny), goal)
                    heapq.heappush(open_heap, (f, expansions, (nx, ny)))

        self._astar_cache = (start, goal, result)
        return result

    def choose_movement_direction(
        self,
        board: Board,
        my_pos: tuple[int, int],
        target: tuple[int, int],
        current_time: float,
        *,
        blind: bool,
        try_direction_fn: Callable[[Direction], bool],
        greedy_order: list[Direction],
    ) -> Direction | None:
        """Hybrid PRD D8: A* first step when far or blind; else greedy ``greedy_order``."""
        self.tick_time(current_time)
        dist = abs(target[0] - my_pos[0]) + abs(target[1] - my_pos[1])
        use_search = blind or dist > self.greedy_distance

        if use_search:
            nxt = self._astar_first_step(board, my_pos, target)
            if nxt is not None:
                dx = nxt[0] - my_pos[0]
                dy = nxt[1] - my_pos[1]
                for d, (vx, vy) in DIRECTION_VECTORS.items():
                    if vx == dx and vy == dy:
                        if self.is_tabu(my_pos, d, current_time):
                            break
                        if try_direction_fn(d):
                            return d
                        break

        for d in greedy_order:
            if self.is_tabu(my_pos, d, current_time):
                continue
            if try_direction_fn(d):
                return d
        return None
