"""PETSCII-based rendering for the TANK! playfield.

All game-object drawing uses cached glyph blits from :mod:`petscii_render`
instead of ``pygame.draw`` primitives, matching the original Commodore PET
look (UI_RETRO_SPEC).
"""

from __future__ import annotations

from collections.abc import Iterable

from .board import Board, CellType
from .constants import (
    BOARD_OFFSET_Y,
    CELL_SIZE,
    COLOR_BG,
    COLOR_EXPLOSION,
    COLOR_EXPLOSION_CHAIN,
    COLOR_GRID,
    COLOR_MINE,
    COLOR_SHOT,
    COLOR_TANK_1,
    COLOR_TANK_2,
    COLOR_WRECKAGE,
    SCREEN_HEIGHT_CELLS,
    SCREEN_WIDTH_CELLS,
)
from .game import Explosion
from .petscii_map import PET_MAP
from .petscii_render import blit_cell, blit_glyph
from .player import Direction, Tank
from .projectile import Mine, Shot

# ── Barrel glyph per direction ──────────────────────────────────────────────
# Cardinals use horizontal or vertical line; diagonals use NE/NW stroke chars.
_BARREL_CHAR: dict[Direction, str] = {
    Direction.UP:         PET_MAP["LINE_V"],
    Direction.DOWN:       PET_MAP["LINE_V"],
    Direction.LEFT:       PET_MAP["BARREL_H"],
    Direction.RIGHT:      PET_MAP["BARREL_H"],
    Direction.UP_LEFT:    PET_MAP["DIAG_NW"],
    Direction.UP_RIGHT:   PET_MAP["DIAG_NE"],
    Direction.DOWN_LEFT:  PET_MAP["DIAG_NE"],
    Direction.DOWN_RIGHT: PET_MAP["DIAG_NW"],
}

_BARREL_OFFSET: dict[Direction, tuple[int, int]] = {
    Direction.UP:         ( 0, -1),
    Direction.DOWN:       ( 0,  1),
    Direction.LEFT:       (-1,  0),
    Direction.RIGHT:      ( 1,  0),
    Direction.UP_LEFT:    (-1, -1),
    Direction.UP_RIGHT:   ( 1, -1),
    Direction.DOWN_LEFT:  (-1,  1),
    Direction.DOWN_RIGHT: ( 1,  1),
}

# ── Explosion spoke patterns ────────────────────────────────────────────────
# (dx, dy, glyph) — relative to the explosion center cell.
_EXPLOSION_SPOKES_SMALL: list[tuple[int, int, str]] = [
    ( 0, -1, PET_MAP["LINE_V"]),    # N
    ( 0,  1, PET_MAP["LINE_V"]),    # S
    (-1,  0, PET_MAP["LINE_H"]),    # W
    ( 1,  0, PET_MAP["LINE_H"]),    # E
    (-1, -1, PET_MAP["DIAG_NW"]),   # NW
    ( 1, -1, PET_MAP["DIAG_NE"]),   # NE
    (-1,  1, PET_MAP["DIAG_NE"]),   # SW
    ( 1,  1, PET_MAP["DIAG_NW"]),   # SE
]

_EXPLOSION_SPOKES_LARGE: list[tuple[int, int, str]] = (
    _EXPLOSION_SPOKES_SMALL
    + [
        # Second ring of rays (extends each spoke one cell further)
        ( 0, -2, PET_MAP["LINE_V"]),
        ( 0,  2, PET_MAP["LINE_V"]),
        (-2,  0, PET_MAP["LINE_H"]),
        ( 2,  0, PET_MAP["LINE_H"]),
        (-2, -2, PET_MAP["DIAG_NW"]),
        ( 2, -2, PET_MAP["DIAG_NE"]),
        (-2,  2, PET_MAP["DIAG_NE"]),
        ( 2,  2, PET_MAP["DIAG_NW"]),
    ]
)


# ── Public draw functions ───────────────────────────────────────────────────

def draw_board(surface: pygame.Surface, board: Board) -> None:
    surface.fill(COLOR_BG)

    # Border: PETSCII ball character around the perimeter (row 0, row H-1, col 0, col W-1)
    border_ch = PET_MAP["BORDER"]
    wall_ch = PET_MAP["WALL"]

    for y in range(board.height):
        for x in range(board.width):
            cell = board.get_cell(x, y)
            is_border = x == 0 or x == board.width - 1 or y == 0 or y == board.height - 1
            if is_border:
                blit_cell(
                    surface, border_ch, x, y, COLOR_GRID, CELL_SIZE, BOARD_OFFSET_Y,
                    inverted=True, bg=COLOR_BG,
                )
            elif cell and cell.type == CellType.WALL:
                blit_cell(surface, wall_ch, x, y, COLOR_GRID, CELL_SIZE, BOARD_OFFSET_Y)


def draw_tanks(surface: pygame.Surface, tanks: Iterable[Tank]) -> None:
    for tank in tanks:
        color = COLOR_TANK_1 if tank.player_id == 1 else COLOR_TANK_2
        body_ch = PET_MAP["TANK_BODY_P1"] if tank.player_id == 1 else PET_MAP["TANK_BODY_P2"]
        blit_cell(
            surface, body_ch, tank.x, tank.y, color, CELL_SIZE, BOARD_OFFSET_Y,
            inverted=True, bg=COLOR_BG,
        )

        barrel_ch = _BARREL_CHAR.get(tank.direction, PET_MAP["LINE_V"])
        dx, dy = _BARREL_OFFSET.get(tank.direction, (0, -1))
        bx, by = tank.x + dx, tank.y + dy
        if 0 <= bx < SCREEN_WIDTH_CELLS and 0 <= by < SCREEN_HEIGHT_CELLS:
            blit_cell(surface, barrel_ch, bx, by, color, CELL_SIZE, BOARD_OFFSET_Y)


def draw_shots(surface: pygame.Surface, shots: Iterable[Shot]) -> None:
    shot_ch = PET_MAP["SHOT"]
    for shot in shots:
        if not shot.active:
            continue
        blit_cell(surface, shot_ch, shot.x, shot.y, COLOR_SHOT, CELL_SIZE, BOARD_OFFSET_Y)


def draw_mines(surface: pygame.Surface, mines: Iterable[Mine]) -> None:
    mine_ch = PET_MAP["MINE"]
    for mine in mines:
        if not mine.active or not mine.visible:
            continue
        blit_cell(surface, mine_ch, mine.x, mine.y, COLOR_MINE, CELL_SIZE, BOARD_OFFSET_Y)


def draw_explosions(surface: pygame.Surface, explosions: Iterable[Explosion]) -> None:
    center_ch = PET_MAP["BORDER"]

    for exp in explosions:
        is_big = getattr(exp, "is_chain_reaction", False)
        color = COLOR_EXPLOSION_CHAIN if is_big else COLOR_EXPLOSION
        spokes = _EXPLOSION_SPOKES_LARGE if is_big else _EXPLOSION_SPOKES_SMALL

        blit_cell(
            surface, center_ch, exp.x, exp.y, color, CELL_SIZE, BOARD_OFFSET_Y,
            inverted=True, bg=COLOR_BG,
        )

        for dx, dy, ch in spokes:
            sx, sy = exp.x + dx, exp.y + dy
            if 0 <= sx < SCREEN_WIDTH_CELLS and 0 <= sy < SCREEN_HEIGHT_CELLS:
                blit_cell(surface, ch, sx, sy, color, CELL_SIZE, BOARD_OFFSET_Y)


def draw_wreckage(surface: pygame.Surface, board: Board) -> None:
    wreck_ch = PET_MAP["WRECKAGE"]
    for y in range(board.height):
        for x in range(board.width):
            cell = board.get_cell(x, y)
            if cell and cell.type in {CellType.WRECKAGE_P1, CellType.WRECKAGE_P2}:
                blit_cell(surface, wreck_ch, x, y, COLOR_WRECKAGE, CELL_SIZE, BOARD_OFFSET_Y)
