"""Combat perception: forward cone, peripheral radius (PRD §5.5 FR-9)."""

from __future__ import annotations

import math

from .player import DIRECTION_VECTORS, Direction


def _facing_vector(direction: Direction) -> tuple[float, float]:
    dx, dy = DIRECTION_VECTORS[direction]
    length = math.hypot(dx, dy) or 1.0
    return (dx / length, dy / length)


def _to_enemy_unit(my_pos: tuple[int, int], enemy_pos: tuple[int, int]) -> tuple[float, float]:
    vx = enemy_pos[0] - my_pos[0]
    vy = enemy_pos[1] - my_pos[1]
    length = math.hypot(vx, vy) or 1.0
    return (vx / length, vy / length)


def enemy_in_forward_cone(
    my_pos: tuple[int, int],
    my_direction: Direction,
    enemy_pos: tuple[int, int],
    half_degrees: float = 45.0,
) -> bool:
    """True if enemy lies within ``2 * half_degrees`` forward arc (default 90° total)."""
    fx, fy = _facing_vector(my_direction)
    ex, ey = _to_enemy_unit(my_pos, enemy_pos)
    dot = max(-1.0, min(1.0, fx * ex + fy * ey))
    angle = math.degrees(math.acos(dot))
    return angle <= half_degrees + 1e-6


def manhattan(a: tuple[int, int], b: tuple[int, int]) -> int:
    return abs(a[0] - b[0]) + abs(a[1] - b[1])


def should_track_enemy(
    my_pos: tuple[int, int],
    my_direction: Direction,
    enemy_pos: tuple[int, int],
    peripheral_radius: int,
) -> bool:
    """Update last-known if enemy is in forward cone OR within peripheral radius."""
    if enemy_in_forward_cone(my_pos, my_direction, enemy_pos):
        return True
    return manhattan(my_pos, enemy_pos) <= peripheral_radius


def combat_visible_enemy(
    my_pos: tuple[int, int],
    my_direction: Direction,
    enemy_pos: tuple[int, int],
) -> tuple[int, int] | None:
    """Enemy position usable for fire / too-close combat, or None if outside forward cone."""
    if not enemy_in_forward_cone(my_pos, my_direction, enemy_pos):
        return None
    return enemy_pos
