from collections import Counter

import pygame
from tank_game.board import CellType
from tank_game.constants import HEADLESS_WALL_BUDGET_DEFAULT_S
from tank_game.game import GameController, GameState
from tank_game.game_loop import format_headless_showdown_dashboard


def test_init_round_creates_two_tanks():
    controller = GameController()
    controller.init_round()
    assert len(controller.tanks) == 2
    assert controller.state == GameState.PLAYING


def test_win_detection_simple():
    controller = GameController()
    controller.init_round()
    # Force player 2 to zero lives and trigger defeat
    controller.tanks[2].lives = 1
    controller._on_player_defeated(2)
    # Now game continues with respawn - check that winner is set and battle counted
    assert controller.winner == 1
    assert controller.battles_played == 1
    # Game should continue PLAYING (not GAME_OVER) until both are dead


def test_sim_time_s_headless_uses_internal_clock() -> None:
    controller = GameController()
    controller._showdown_headless = True
    controller._headless_sim_time_s = 42.25
    assert controller.sim_time_s() == 42.25


def test_headless_dashboard_sim_time_whole_seconds_only() -> None:
    gc = GameController()
    gc._showdown_iterations = 4
    gc._showdown_current = 2
    gc.difficulty_per_player = {1: 1, 2: 2}
    gc._headless_sim_time_s = 3.99
    gc._headless_wall_budget_s = 0.05
    text = format_headless_showdown_dashboard(gc)
    assert "Sim time (this match): 3 s" in text
    assert "3.99" not in text
    assert "Sim rate (live):" in text


def test_abort_showdown_run_sets_complete_and_message() -> None:
    gc = GameController()
    gc.state = GameState.SHOWDOWN_RUNNING
    gc.winner = 1
    gc.abort_showdown_run()
    assert gc.state == GameState.SHOWDOWN_COMPLETE
    assert gc.winner is None
    assert "cancelled" in gc._showdown_summary_file.lower()


def test_headless_wall_budget_default_and_keyboard_adjust() -> None:
    gc = GameController()
    assert gc._headless_wall_budget_s == HEADLESS_WALL_BUDGET_DEFAULT_S
    gc.state = GameState.SHOWDOWN_RUNNING
    gc._showdown_headless = True
    gc.handle_input([pygame.event.Event(pygame.KEYDOWN, key=pygame.K_UP)])
    assert gc._headless_wall_budget_s > HEADLESS_WALL_BUDGET_DEFAULT_S
    gc.handle_input([pygame.event.Event(pygame.KEYDOWN, key=pygame.K_DOWN)])
    gc.handle_input([pygame.event.Event(pygame.KEYDOWN, key=pygame.K_DOWN)])
    assert gc._headless_wall_budget_s < HEADLESS_WALL_BUDGET_DEFAULT_S


def test_respawn_clears_old_tank_cells_from_board() -> None:
    """Regression: respawn must remove TANK markers from previous positions.

    If coordinates are updated before clear_from_board, ghost tank cells remain and
    block movement; shots can spawn into a ghost cell and register as an instant hit.
    """
    controller = GameController()
    controller.init_round()
    board = controller.board
    t1, t2 = controller.tanks[1], controller.tanks[2]

    sp1 = t1.start_pos
    sp2 = t2.start_pos
    t1.clear_from_board(board)
    t1.x, t1.y = 10, 10
    t1.occupy_board(board)
    t2.clear_from_board(board)
    t2.x, t2.y = 30, 10
    t2.occupy_board(board)

    assert board.get_cell(10, 10).type == CellType.TANK1
    assert board.get_cell(30, 10).type == CellType.TANK2
    assert board.get_cell(sp1[0], sp1[1]).type == CellType.EMPTY

    controller._respawn_both_tanks()

    assert board.get_cell(10, 10).type == CellType.EMPTY
    assert board.get_cell(30, 10).type == CellType.EMPTY
    assert board.get_cell(sp1[0], sp1[1]).type == CellType.TANK1
    assert board.get_cell(sp2[0], sp2[1]).type == CellType.TANK2

    counts: Counter[CellType] = Counter()
    for y in range(board.height):
        for x in range(board.width):
            cell = board.get_cell(x, y)
            if cell is not None:
                counts[cell.type] += 1
    assert counts[CellType.TANK1] == 1
    assert counts[CellType.TANK2] == 1
