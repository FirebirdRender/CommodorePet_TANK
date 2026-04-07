"""Tests for ai_perception (FR-9)."""

from __future__ import annotations

from tank_game.ai_perception import combat_visible_enemy, enemy_in_forward_cone, should_track_enemy
from tank_game.player import Direction


def test_forward_cone_sees_ahead() -> None:
    assert enemy_in_forward_cone((5, 5), Direction.RIGHT, (10, 5))
    assert not enemy_in_forward_cone((5, 5), Direction.LEFT, (10, 5))


def test_combat_visible_only_in_cone() -> None:
    assert combat_visible_enemy((5, 12), Direction.RIGHT, (20, 12)) == (20, 12)
    assert combat_visible_enemy((5, 12), Direction.LEFT, (20, 12)) is None


def test_peripheral_tracks_behind() -> None:
    assert should_track_enemy((5, 5), Direction.RIGHT, (4, 5), peripheral_radius=3)
