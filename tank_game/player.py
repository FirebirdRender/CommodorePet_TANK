from __future__ import annotations

from dataclasses import dataclass
from enum import IntEnum, auto

from .board import Board, CellType


class Direction(IntEnum):
    UP = auto()
    DOWN = auto()
    LEFT = auto()
    RIGHT = auto()
    UP_LEFT = auto()
    UP_RIGHT = auto()
    DOWN_LEFT = auto()
    DOWN_RIGHT = auto()


DIRECTION_VECTORS: dict[Direction, tuple[int, int]] = {
    Direction.UP: (0, -1),
    Direction.DOWN: (0, 1),
    Direction.LEFT: (-1, 0),
    Direction.RIGHT: (1, 0),
    Direction.UP_LEFT: (-1, -1),
    Direction.UP_RIGHT: (1, -1),
    Direction.DOWN_LEFT: (-1, 1),
    Direction.DOWN_RIGHT: (1, 1),
}


@dataclass
class Tank:
    player_id: int
    x: int
    y: int
    direction: Direction
    lives: int
    shots_left: int
    mines_left: int
    start_pos: tuple[int, int]

    def occupy_board(self, board: Board) -> None:
        cell_type = CellType.TANK1 if self.player_id == 1 else CellType.TANK2
        board.set_cell_type(self.x, self.y, cell_type)

    def clear_from_board(self, board: Board) -> None:
        # Check if there's a mine at this location before clearing
        cell = board.get_cell(self.x, self.y)

        # If there's a mine here, just clear the tank but leave the mine
        if cell and cell.type == CellType.MINE:
            # Tank leaves, mine stays - just change cell to MINE (not EMPTY)
            board.set_cell_type(self.x, self.y, CellType.MINE)
        else:
            board.set_cell_type(self.x, self.y, CellType.EMPTY)

    def attempt_move(self, board: Board, direction: Direction) -> bool:
        dx, dy = DIRECTION_VECTORS[direction]
        new_x = self.x + dx
        new_y = self.y + dy

        # Clamp to board bounds (leave 1 cell border on all sides)
        # Board has walls at x=0, x=width-1, y=0, y=height-1
        new_x = max(1, min(new_x, board.width - 2))
        new_y = max(1, min(new_y, board.height - 2))

        # Don't allow moving to same position (edge case when already at border)
        if new_x == self.x and new_y == self.y:
            return False

        # Check bounds after clamping
        if not board.in_bounds(new_x, new_y):
            return False
        target = board.get_cell(new_x, new_y)
        if target is None:
            return False
        # Allow movement through empty cells, shots, and mines
        # Blocked by walls, other tanks, and wreckage
        if target.type not in {CellType.EMPTY, CellType.SHOT, CellType.MINE}:
            return False

        # Only update position if ALL checks pass
        self.clear_from_board(board)
        self.x, self.y = new_x, new_y
        self.direction = direction
        self.occupy_board(board)
        return True

    def can_fire(self) -> bool:
        return self.shots_left > 0

    def consume_shot(self) -> None:
        if self.shots_left > 0:
            self.shots_left -= 1

    def can_place_mine(self) -> bool:
        return self.mines_left > 0

    def consume_mine(self) -> None:
        if self.mines_left > 0:
            self.mines_left -= 1

    def take_damage(self) -> None:
        if self.lives > 0:
            self.lives -= 1

    def is_alive(self) -> bool:
        return self.lives > 0

    def respawn(self, board: Board) -> None:
        self.clear_from_board(board)
        start_x, start_y = self.start_pos
        # Simple respawn: try start cell, then expand in a small radius
        for radius in range(0, 5):
            for dx in range(-radius, radius + 1):
                for dy in range(-radius, radius + 1):
                    x = start_x + dx
                    y = start_y + dy
                    if not board.in_bounds(x, y):
                        continue
                    cell = board.get_cell(x, y)
                    if cell and cell.type == CellType.EMPTY:
                        self.x, self.y = x, y
                        self.occupy_board(board)
                        return
