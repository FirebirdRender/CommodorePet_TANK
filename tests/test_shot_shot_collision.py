"""Shot–shot interaction: same-cell and head-on adjacent swap (discrete pass-through)."""

from tank_game.game import _shots_crossed_head_on, _shot_shot_explosion_key
from tank_game.player import Direction
from tank_game.projectile import Shot


def _swap_pair() -> tuple[Shot, Shot]:
    """After step: P1 at 22,12 RIGHT from 21; P2 at 21,12 LEFT from 22 — classic pass-through."""
    s1 = Shot(x=22, y=12, direction=Direction.RIGHT, owner_id=1)
    s2 = Shot(x=21, y=12, direction=Direction.LEFT, owner_id=2)
    s1._step_start_x, s1._step_start_y = 21, 12
    s2._step_start_x, s2._step_start_y = 22, 12
    return s1, s2


def test_head_on_horizontal_cross_detected() -> None:
    s1, s2 = _swap_pair()
    assert _shots_crossed_head_on(s1, s2) is True
    assert _shot_shot_explosion_key(s1, s2, crossed=True) == (21, 12)


def test_head_on_horizontal_not_cross_when_separated() -> None:
    """Still approaching: 20,12 -> 21 and 23,12 -> 22 — order preserved."""
    s1 = Shot(x=21, y=12, direction=Direction.RIGHT, owner_id=1)
    s2 = Shot(x=22, y=12, direction=Direction.LEFT, owner_id=2)
    s1._step_start_x, s1._step_start_y = 20, 12
    s2._step_start_x, s2._step_start_y = 23, 12
    assert _shots_crossed_head_on(s1, s2) is False


def test_head_on_vertical_cross_detected() -> None:
    """Same column: upper moves DOWN, lower moves UP, swap rows in one step."""
    upper = Shot(x=5, y=12, direction=Direction.DOWN, owner_id=1)
    lower = Shot(x=5, y=11, direction=Direction.UP, owner_id=2)
    upper._step_start_x, upper._step_start_y = 5, 11
    lower._step_start_x, lower._step_start_y = 5, 12
    assert _shots_crossed_head_on(upper, lower) is True


def test_same_cell_not_marked_as_cross() -> None:
    s1 = Shot(x=22, y=12, direction=Direction.RIGHT, owner_id=1)
    s2 = Shot(x=22, y=12, direction=Direction.LEFT, owner_id=2)
    s1._step_start_x, s1._step_start_y = 21, 12
    s2._step_start_x, s2._step_start_y = 23, 12
    assert _shots_crossed_head_on(s1, s2) is False
