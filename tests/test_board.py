from tank_game.board import Board, CellType


def test_board_borders_are_walls():
    board = Board()
    for x in range(board.width):
        assert board.get_cell(x, 0).type == CellType.WALL
        assert board.get_cell(x, board.height - 1).type == CellType.WALL
    for y in range(board.height):
        assert board.get_cell(0, y).type == CellType.WALL
        assert board.get_cell(board.width - 1, y).type == CellType.WALL


def test_in_bounds_and_passable():
    board = Board()
    assert board.in_bounds(1, 1)
    assert not board.in_bounds(-1, -1)
    # Find a passable cell - board now has random terrain
    passable_found = False
    for y in range(1, board.height - 1):
        for x in range(1, board.width - 1):
            if board.is_passable(x, y):
                passable_found = True
                break
        if passable_found:
            break
    assert passable_found, "Should have at least one passable interior cell"
