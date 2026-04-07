from __future__ import annotations

import os
from dataclasses import dataclass

from .board import Board, CellType
from .player import DIRECTION_VECTORS, Direction

# Mine settings
MINE_VISIBLE_DURATION: float = 2.0  # seconds visible before disappearing
MINE_LIFETIME: float = 2.0  # seconds before mine is removed entirely
MINE_HIDDEN_DISTANCE: int = 2  # tiles away from owner before becoming visible again


_DEBUG = os.environ.get("TANK_DEBUG", "").strip().lower() not in ("", "0", "false", "no", "off")


def _debug_log(msg: str) -> None:
    """Write debug message to log file."""
    if not _DEBUG:
        return
    from datetime import datetime

    timestamp = datetime.now().strftime("%H:%M:%S.%f")[:-3]
    with open(os.path.expanduser("~/.tank_debug.log"), "a") as f:
        f.write(f"[{timestamp}] {msg}\n")


@dataclass
class Shot:
    x: int
    y: int
    direction: Direction
    active: bool = True
    owner_id: int = 0  # Which player fired this shot (1 or 2)
    max_range: int = 30  # Max cells before silent removal (75% of board dim)
    _steps_taken: int = 0
    _hit_wall: bool = False  # True if last hit was a wall (for splash damage)
    _last_move_time: float = 0.0
    _collision_pos: tuple[int, int] | None = None

    def step(self, board: Board) -> tuple[int, int] | None:
        """Advance one cell; return collision position if any, else None."""
        if not self.active:
            return None

        # Range limit: silently disappear (no explosion, no damage)
        if self._steps_taken >= self.max_range:
            self.active = False
            return None

        # FIRST: Check if there's a tank/barrel at CURRENT position (for spawn-point hits)
        current_cell = board.get_cell(self.x, self.y)
        if current_cell and current_cell.type in {
            CellType.TANK1, CellType.TANK2, CellType.BARREL1, CellType.BARREL2,
        }:
            self.active = False
            return self.x, self.y

        # THEN: Move to next position
        dx, dy = DIRECTION_VECTORS[self.direction]
        next_x = self.x + dx
        next_y = self.y + dy
        if not board.in_bounds(next_x, next_y):
            self.active = False
            return self.x, self.y
        cell = board.get_cell(next_x, next_y)
        if cell is None:
            self.active = False
            return self.x, self.y

        # Wall destruction - triggers splash damage!
        if cell.type == CellType.WALL:
            self.active = False
            board.set_cell_type(next_x, next_y, CellType.EMPTY)
            self._hit_wall = True  # Flag for splash damage
            self._steps_taken += 1
            return next_x, next_y

        # Hit detection: tanks, barrels, mines, or wreckage
        if cell.type in {
            CellType.TANK1,
            CellType.TANK2,
            CellType.BARREL1,
            CellType.BARREL2,
            CellType.MINE,
            CellType.WRECKAGE_P1,
            CellType.WRECKAGE_P2,
        }:
            self.active = False
            self._steps_taken += 1
            return next_x, next_y

        # Move through empty or shot cells
        self.x, self.y = next_x, next_y
        self._steps_taken += 1
        return None


@dataclass
class Mine:
    x: int
    y: int
    owner_id: int = 0  # Which player placed this mine
    active: bool = True
    visible: bool = True
    visible_start_time: float = 0.0
    placed_time: float = 0.0  # When the mine was placed
