from __future__ import annotations

from collections.abc import Iterable

import pygame

from .board import Board, CellType
from .constants import (
    BOARD_OFFSET_Y,
    CELL_SIZE,
    COLOR_BG,
    COLOR_GRID,
    COLOR_MINE,
    COLOR_SHOT,
    COLOR_TANK_1,
    COLOR_TANK_2,
    COLOR_WRECKAGE,
    WINDOW_WIDTH,
)
from .game import Explosion
from .player import Tank
from .projectile import Mine, Shot


def draw_board(surface: pygame.Surface, board: Board) -> None:
    surface.fill(COLOR_BG)

    # Draw dotted border (original TANK! style) - offset below status bar
    dot_spacing = CELL_SIZE // 2
    dot_color = COLOR_GRID

    # Top border (below status bar)
    border_top = BOARD_OFFSET_Y + dot_spacing // 2
    for x in range(0, WINDOW_WIDTH, dot_spacing):
        pygame.draw.circle(surface, dot_color, (x + dot_spacing // 2, border_top), 2)

    # Bottom border - at the bottom of the board area (not window bottom)
    board_bottom = BOARD_OFFSET_Y + (board.height * CELL_SIZE)
    for x in range(0, WINDOW_WIDTH, dot_spacing):
        pygame.draw.circle(
            surface, dot_color, (x + dot_spacing // 2, board_bottom - dot_spacing // 2), 2
        )

    # Left border
    for y in range(BOARD_OFFSET_Y, BOARD_OFFSET_Y + board.height * CELL_SIZE, dot_spacing):
        pygame.draw.circle(surface, dot_color, (dot_spacing // 2, y + dot_spacing // 2), 2)
    # Right border
    for y in range(BOARD_OFFSET_Y, BOARD_OFFSET_Y + board.height * CELL_SIZE, dot_spacing):
        pygame.draw.circle(
            surface, dot_color, (WINDOW_WIDTH - dot_spacing // 2, y + dot_spacing // 2), 2
        )

    # Draw interior walls (terrain) - offset by BOARD_OFFSET_Y
    for y in range(board.height):
        for x in range(board.width):
            rect = pygame.Rect(x * CELL_SIZE, BOARD_OFFSET_Y + y * CELL_SIZE, CELL_SIZE, CELL_SIZE)
            cell = board.get_cell(x, y)
            if cell and cell.type == CellType.WALL:
                pygame.draw.rect(surface, COLOR_GRID, rect)


def draw_tanks(surface: pygame.Surface, tanks: Iterable[Tank]) -> None:
    for tank in tanks:
        color = COLOR_TANK_1 if tank.player_id == 1 else COLOR_TANK_2

        # Draw tank body
        rect = pygame.Rect(
            tank.x * CELL_SIZE, BOARD_OFFSET_Y + tank.y * CELL_SIZE, CELL_SIZE, CELL_SIZE
        )
        pygame.draw.rect(surface, color, rect)

        # Draw barrel indicating direction
        from .player import Direction

        cx = tank.x * CELL_SIZE + CELL_SIZE // 2
        cy = BOARD_OFFSET_Y + tank.y * CELL_SIZE + CELL_SIZE // 2
        barrel_length = CELL_SIZE // 2

        if tank.direction == Direction.UP:
            end_pos = (cx, cy - barrel_length)
        elif tank.direction == Direction.DOWN:
            end_pos = (cx, cy + barrel_length)
        elif tank.direction == Direction.LEFT:
            end_pos = (cx - barrel_length, cy)
        elif tank.direction == Direction.RIGHT:
            end_pos = (cx + barrel_length, cy)
        elif tank.direction == Direction.UP_LEFT:
            end_pos = (cx - barrel_length // 2, cy - barrel_length // 2)
        elif tank.direction == Direction.UP_RIGHT:
            end_pos = (cx + barrel_length // 2, cy - barrel_length // 2)
        elif tank.direction == Direction.DOWN_LEFT:
            end_pos = (cx - barrel_length // 2, cy + barrel_length // 2)
        elif tank.direction == Direction.DOWN_RIGHT:
            end_pos = (cx + barrel_length // 2, cy + barrel_length // 2)
        else:
            end_pos = (cx, cy - barrel_length)

        pygame.draw.line(surface, (0, 0, 0), (cx, cy), end_pos, 3)


def draw_shots(surface: pygame.Surface, shots: Iterable[Shot]) -> None:
    for shot in shots:
        if not shot.active:
            continue
        rect = pygame.Rect(
            shot.x * CELL_SIZE + CELL_SIZE // 4,
            BOARD_OFFSET_Y + shot.y * CELL_SIZE + CELL_SIZE // 4,
            CELL_SIZE // 2,
            CELL_SIZE // 2,
        )
        pygame.draw.rect(surface, COLOR_SHOT, rect)


def draw_mines(surface: pygame.Surface, mines: Iterable[Mine]) -> None:
    for mine in mines:
        if not mine.active:
            continue
        # Only draw if mine is visible
        if not mine.visible:
            continue
        rect = pygame.Rect(
            mine.x * CELL_SIZE + CELL_SIZE // 4,
            BOARD_OFFSET_Y + mine.y * CELL_SIZE + CELL_SIZE // 4,
            CELL_SIZE // 2,
            CELL_SIZE // 2,
        )
        pygame.draw.rect(surface, COLOR_MINE, rect)


def draw_explosions(surface: pygame.Surface, explosions: Iterable[Explosion]) -> None:
    for exp in explosions:
        center = (
            exp.x * CELL_SIZE + CELL_SIZE // 2,
            BOARD_OFFSET_Y + exp.y * CELL_SIZE + CELL_SIZE // 2,
        )
        if exp.is_chain_reaction:
            # Chain reaction: larger, more intense (orange/red, bigger)
            pygame.draw.circle(surface, (255, 100, 0), center, CELL_SIZE)  # Bigger
            pygame.draw.circle(surface, (255, 200, 0), center, CELL_SIZE * 2 // 3)
        else:
            # Regular explosion: smaller (yellow/orange)
            pygame.draw.circle(surface, (255, 200, 0), center, CELL_SIZE // 2)


def draw_wreckage(surface: pygame.Surface, board: Board) -> None:
    """Draw destroyed tanks (wreckage) on the board."""
    for y in range(board.height):
        for x in range(board.width):
            cell = board.get_cell(x, y)
            if cell and cell.type in {CellType.WRECKAGE_P1, CellType.WRECKAGE_P2}:
                # Draw wreckage as a darker, damaged-looking square
                rect = pygame.Rect(
                    x * CELL_SIZE, BOARD_OFFSET_Y + y * CELL_SIZE, CELL_SIZE, CELL_SIZE
                )
                pygame.draw.rect(surface, COLOR_WRECKAGE, rect)
                # Add a cross-hatch pattern to look "wrecked"
                start_x = x * CELL_SIZE
                start_y = BOARD_OFFSET_Y + y * CELL_SIZE
                end_x = (x + 1) * CELL_SIZE
                end_y = BOARD_OFFSET_Y + (y + 1) * CELL_SIZE
                pygame.draw.line(surface, (60, 60, 60), (start_x, start_y), (end_x, end_y), 2)
                pygame.draw.line(surface, (60, 60, 60), (end_x, start_y), (start_x, end_y), 2)
