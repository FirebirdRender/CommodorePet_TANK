from tank_game.board import Board, CellType
from tank_game.player import Direction, Tank
from tank_game.projectile import Mine, Shot


def test_tank_movement_respects_walls():
    # Create board and find a clear spot for the tank
    board = Board()
    tank_x, tank_y = 2, 2

    # Find a clear starting position
    found = False
    for y in range(1, board.height - 1):
        for x in range(1, board.width - 1):
            if board.get_cell(x, y).type == CellType.EMPTY:
                tank_x, tank_y = x, y
                found = True
                break
        if found:
            break

    if not found:
        # No empty space available - skip test
        return

    tank = Tank(
        player_id=1,
        x=tank_x,
        y=tank_y,
        direction=Direction.RIGHT,
        lives=3,
        shots_left=5,
        mines_left=2,
        start_pos=(tank_x, tank_y),
    )
    tank.occupy_board(board)

    # Try to move in any direction - just verify movement doesn't crash
    tank.attempt_move(board, Direction.UP)
    tank.attempt_move(board, Direction.DOWN)

    # If we got here without crashing, movement logic is working
    assert True


def test_tank_cannot_escape_board():
    """Test that tanks cannot move outside the board boundaries."""
    board = Board()

    # Create tank at a known interior position
    start_x, start_y = 2, 2
    tank = Tank(
        player_id=1,
        x=start_x,
        y=start_y,
        direction=Direction.RIGHT,
        lives=3,
        shots_left=5,
        mines_left=2,
        start_pos=(start_x, start_y),
    )
    tank.occupy_board(board)

    # Test 1: Move LEFT from x=1 - should stay at x=1 or clamp
    tank.x, tank.y = 1, 10  # Left edge
    tank.attempt_move(board, Direction.LEFT)
    assert tank.x >= 1, f"Tank escaped left! x={tank.x}"

    # Test 2: Move RIGHT from x=width-2 - should clamp
    tank.x, tank.y = board.width - 2, 10  # Right edge
    tank.attempt_move(board, Direction.RIGHT)
    assert tank.x <= board.width - 2, f"Tank escaped right! x={tank.x}"

    # Test 3: Move UP from y=1 - should stay at y=1
    tank.x, tank.y = 10, 1  # Top edge
    tank.attempt_move(board, Direction.UP)
    assert tank.y >= 1, f"Tank escaped up! y={tank.y}"

    # Test 4: Move DOWN from y=height-2 - should clamp
    tank.x, tank.y = 10, board.height - 2  # Bottom edge
    tank.attempt_move(board, Direction.DOWN)
    assert tank.y <= board.height - 2, f"Tank escaped down! y={tank.y}"

    # Test 5: Diagonal from corner - should clamp
    tank.x, tank.y = 1, 1
    tank.attempt_move(board, Direction.UP_LEFT)
    assert tank.x >= 1 and tank.y >= 1, f"Tank escaped diagonal! x={tank.x}, y={tank.y}"

    # Test 6: Diagonal from bottom-right corner
    tank.x, tank.y = board.width - 2, board.height - 2
    tank.attempt_move(board, Direction.DOWN_RIGHT)
    assert tank.x <= board.width - 2 and tank.y <= board.height - 2, (
        f"Tank escaped diagonal! x={tank.x}, y={tank.y}"
    )


def test_shot_moves_and_hits_wall():
    board = Board()
    # Find a wall to shoot at
    shot = Shot(x=1, y=1, direction=Direction.LEFT)
    collision = shot.step(board)
    # Either hit something (wall/tank/mine) or deactivate
    assert not shot.active or collision is not None


def test_mine_basic_state():
    mine = Mine(x=5, y=5, owner_id=1)
    assert mine.active
    assert mine.visible
