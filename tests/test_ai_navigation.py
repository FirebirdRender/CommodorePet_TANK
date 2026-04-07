"""Unit tests for ``tank_game.ai_navigation`` (PRD AI_NAVIGATION)."""

from __future__ import annotations

from tank_game.ai_navigation import NavigationMemory, navigation_config_for_difficulty
from tank_game.board import Board, CellType
from tank_game.player import Direction


def test_navigation_config_linear_difficulty() -> None:
    lo = navigation_config_for_difficulty(0)
    hi = navigation_config_for_difficulty(9)
    assert lo.sight_radius < hi.sight_radius
    assert lo.search_budget < hi.search_budget


def test_record_failed_marks_wall_and_tabu() -> None:
    board = Board()
    nav = NavigationMemory(5)
    # Force a wall at (3,3) interior
    board.set_cell_type(3, 3, CellType.WALL)
    pos = (2, 3)
    t = 10.0
    nav.record_failed_attempt(board, pos, Direction.RIGHT, t)
    assert nav.is_tabu(pos, Direction.RIGHT, t + 0.01)
    assert nav._wall_belief.get((3, 3), 0) >= 0.99


def test_on_respawn_softens_belief() -> None:
    nav = NavigationMemory(5)
    nav._wall_belief[(5, 5)] = 1.0
    nav.on_respawn(100.0)
    assert nav._wall_belief[(5, 5)] < 1.0


def test_invalidate_region_drops_walls() -> None:
    nav = NavigationMemory(3)
    nav._wall_belief[(10, 10)] = 1.0
    nav.invalidate_region(10, 10, 2)
    assert (10, 10) not in nav._wall_belief


def test_astar_returns_first_step_on_empty_board() -> None:
    board = Board()
    nav = NavigationMemory(9)
    start = (5, 12)
    goal = (20, 12)
    nxt = nav._astar_first_step(board, start, goal)
    assert nxt is not None
    assert nxt[0] >= start[0]


def test_choose_respects_tabu() -> None:
    board = Board()
    nav = NavigationMemory(5)
    pos = (10, 12)
    t = 1.0
    nav._tabu[(pos[0], pos[1], Direction.RIGHT)] = t + 5.0

    def ok(d: Direction) -> bool:
        return True

    order = [Direction.RIGHT, Direction.LEFT]
    d = nav.choose_movement_direction(
        board, pos, (30, 12), t, blind=False, try_direction_fn=ok, greedy_order=order
    )
    assert d == Direction.LEFT
