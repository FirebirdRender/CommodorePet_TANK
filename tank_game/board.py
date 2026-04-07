from __future__ import annotations

import random
from dataclasses import dataclass
from enum import IntEnum, auto

from .constants import SCREEN_HEIGHT_CELLS, SCREEN_WIDTH_CELLS


class CellType(IntEnum):
    EMPTY = auto()
    WALL = auto()
    TANK1 = auto()
    TANK2 = auto()
    BARREL1 = auto()  # Player 1's barrel cell
    BARREL2 = auto()  # Player 2's barrel cell
    SHOT = auto()
    MINE = auto()
    WRECKAGE_P1 = auto()  # Player 1's destroyed tank
    WRECKAGE_P2 = auto()  # Player 2's destroyed tank


@dataclass
class Cell:
    type: CellType = CellType.EMPTY


class Board:
    """Logical 40x25 grid with border walls and occupancy helpers."""

    def __init__(
        self,
        width: int = SCREEN_WIDTH_CELLS,
        height: int = SCREEN_HEIGHT_CELLS,
        difficulty: int = 5,
    ) -> None:
        self.width = width
        self.height = height
        # Terrain density scales with difficulty: level 0 ~4%, level 9 ~25%
        self._terrain_density: float = 0.04 + (difficulty / 9) * 0.21
        self._grid: list[list[Cell]] = [[Cell() for _ in range(width)] for _ in range(height)]
        self._init_borders()
        self._generate_terrain()

    def _init_borders(self) -> None:
        # Top and bottom borders
        for x in range(self.width):
            self._grid[0][x].type = CellType.WALL
            self._grid[self.height - 1][x].type = CellType.WALL
        # Left and right borders
        for y in range(self.height):
            self._grid[y][0].type = CellType.WALL
            self._grid[y][self.width - 1].type = CellType.WALL

    def _generate_terrain(self) -> None:
        """Randomly populate interior with destructible walls."""
        # Only generate terrain in the INTERIOR (not on borders)
        for y in range(1, self.height - 1):
            for x in range(1, self.width - 1):
                # Skip start positions (approximate: left side for P1, right side for P2)
                if (1 <= x <= 5 and y == 12) or (self.width - 6 <= x <= self.width - 2 and y == 12):
                    continue
                if random.random() < self._terrain_density:
                    self._grid[y][x].type = CellType.WALL

    def in_bounds(self, x: int, y: int) -> bool:
        return 0 <= x < self.width and 0 <= y < self.height

    def get_cell(self, x: int, y: int) -> Cell | None:
        if 0 <= x < self.width and 0 <= y < self.height:
            return self._grid[y][x]
        return None

    def set_cell_type(self, x: int, y: int, cell_type: CellType) -> None:
        cell = self.get_cell(x, y)
        if cell is not None:
            cell.type = cell_type

    def is_passable(self, x: int, y: int) -> bool:
        cell = self.get_cell(x, y)
        if cell is None:
            return False
        return cell.type in {CellType.EMPTY, CellType.SHOT, CellType.MINE}

    def raycast(
        self, start_x: int, start_y: int, dx: int, dy: int
    ) -> list[tuple[int, int, CellType]]:
        """Cast a ray from start position in direction (dx, dy) until blocked.

        Returns list of (x, y, cell_type) for each cell traversed.
        Stops at walls, border, or any non-empty cell.
        """
        results = []
        x, y = start_x + dx, start_y + dy

        while self.in_bounds(x, y):
            cell = self.get_cell(x, y)
            if cell is None:
                break
            results.append((x, y, cell.type))

            # Stop at any blocking cell (wall, tank, mine wreckage)
            if cell.type != CellType.EMPTY and cell.type != CellType.SHOT:
                break

            x += dx
            y += dy

        return results

    def has_line_of_sight(self, start_x: int, start_y: int, target_x: int, target_y: int) -> bool:
        """Check if there's a clear line of sight from start to target.

        Returns True if target is visible (no walls between).
        """
        if not self.in_bounds(target_x, target_y):
            return False

        # Calculate direction
        dx = 1 if target_x > start_x else (-1 if target_x < start_x else 0)
        dy = 1 if target_y > start_y else (-1 if target_y < start_y else 0)

        # If diagonal, need to check both axis-aligned paths
        if dx != 0 and dy != 0:
            # Check horizontal then vertical
            if self._has_los_axis(start_x, start_y, target_x, start_y) or self._has_los_axis(
                start_x, start_y, start_x, target_y
            ):
                return True
            # Also try diagonal path
            x, y = start_x + dx, start_y + dy
            while self.in_bounds(x, y):
                cell = self.get_cell(x, y)
                if cell and cell.type == CellType.WALL:
                    return False
                if x == target_x and y == target_y:
                    return True
                x += dx
                y += dy
            return self.in_bounds(target_x, target_y)

        return self._has_los_axis(start_x, start_y, target_x, target_y)

    def _has_los_axis(self, x0: int, y0: int, x1: int, y1: int) -> bool:
        """Check LOS along a single axis (horizontal or vertical)."""
        if x0 == x1:
            # Vertical
            step = 1 if y1 > y0 else -1
            for y in range(y0 + step, y1 + step, step):
                cell = self.get_cell(x0, y)
                if cell and cell.type == CellType.WALL:
                    return False
            return True
        else:
            # Horizontal
            step = 1 if x1 > x0 else -1
            for x in range(x0 + step, x1 + step, step):
                cell = self.get_cell(x, y0)
                if cell and cell.type == CellType.WALL:
                    return False
            return True

    def get_peripheral_vision(self, x: int, y: int, radius: int) -> list[tuple[int, int, CellType]]:
        """Get all visible cells within radius using line-of-sight.

        Returns cells that are within range AND visible (no wall blocking).
        """
        results = []

        for dy in range(-radius, radius + 1):
            for dx in range(-radius, radius + 1):
                if dx == 0 and dy == 0:
                    continue

                target_x, target_y = x + dx, y + dy

                # Must be in bounds and within radius (chebyshev)
                if not self.in_bounds(target_x, target_y):
                    continue

                # Check if visible (no wall blocking path)
                if self.has_line_of_sight(x, y, target_x, target_y):
                    cell = self.get_cell(target_x, target_y)
                    if cell:
                        results.append((target_x, target_y, cell.type))

        return results
