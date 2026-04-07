"""SHOWDOWN full-matrix schedule builder."""

from tank_game.game import (
    SHOWDOWN_MATRIX_CELLS,
    SHOWDOWN_SKILL_LEVELS,
    build_showdown_matrix_schedule,
)


def test_matrix_schedule_length() -> None:
    for r in (1, 3, 10):
        sched = build_showdown_matrix_schedule(r)
        assert len(sched) == SHOWDOWN_MATRIX_CELLS * r


def test_matrix_schedule_order() -> None:
    sched = build_showdown_matrix_schedule(1)
    assert sched[0] == (0, 0)
    assert sched[1] == (0, 1)
    assert sched[9] == (0, 9)
    assert sched[10] == (1, 0)
    last = (SHOWDOWN_SKILL_LEVELS - 1, SHOWDOWN_SKILL_LEVELS - 1)
    assert sched[-1] == last


def test_matrix_schedule_repeats_per_pairing() -> None:
    """Each (i,j) is played N times consecutively before the next pairing."""
    sched = build_showdown_matrix_schedule(2)
    assert sched[0] == sched[1] == (0, 0)
    assert sched[2] == sched[3] == (0, 1)
