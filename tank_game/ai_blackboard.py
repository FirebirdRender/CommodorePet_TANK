"""Shared AI tick context (PRD §5.1).

Per-frame inputs are copied onto :class:`TankAITickContext` before the behaviour tree
ticks. Leaves read from ``AIPlayer`` tick fields (see :meth:`AIPlayer._bind_tick_inputs`).
"""

from __future__ import annotations

from dataclasses import dataclass

from .board import Board
from .player import Direction
from .projectile import Shot


@dataclass
class TankAITickContext:
    """Snapshot of world facts for one AI decision (read-only semantics for callers)."""

    current_time: float
    board: Board
    player_id: int | None  # 1 or 2 from ``GameController._run_ai`` when applicable
    my_pos: tuple[int, int]
    my_direction: Direction
    my_shots: int
    my_mines: int
    enemy_pos: tuple[int, int] | None
    shots_on_board: list[Shot]
