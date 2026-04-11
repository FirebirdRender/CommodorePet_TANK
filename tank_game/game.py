from __future__ import annotations

import os
from dataclasses import dataclass
from datetime import datetime
from enum import Enum, auto

from .board import Board, CellType
from .constants import (
    difficulty_to_resources,
    get_move_delay,
    get_shot_delay,
)
from .player import DIRECTION_VECTORS, Direction, Tank
from .projectile import Mine, Shot


def _shots_crossed_head_on(a: Shot, b: Shot) -> bool:
    """True if two cardinal shots swapped cells in one step (discrete head-on pass-through).

    Same-cell occupancy is handled separately. This only catches the case where shots
    started adjacent on the same row/column, moved toward each other, and ended with
    their x/y order reversed — so they never shared a grid cell in the same tick.
    Requires ``_step_start_x`` / ``_step_start_y`` set on both shots before stepping.
    """
    if not (a.active and b.active):
        return False
    sa_x = getattr(a, "_step_start_x", a.x)
    sa_y = getattr(a, "_step_start_y", a.y)
    sb_x = getattr(b, "_step_start_x", b.x)
    sb_y = getattr(b, "_step_start_y", b.y)
    # Horizontal: LEFT/RIGHT on same row
    if sa_y == sb_y and a.y == b.y:
        if not (
            a.direction in (Direction.LEFT, Direction.RIGHT)
            and b.direction in (Direction.LEFT, Direction.RIGHT)
        ):
            return False
        if sa_x == sb_x:
            return False
        left, right = (a, b) if sa_x < sb_x else (b, a)
        lsx = getattr(left, "_step_start_x", left.x)
        rsx = getattr(right, "_step_start_x", right.x)
        if lsx >= rsx:
            return False
        if left.direction != Direction.RIGHT or right.direction != Direction.LEFT:
            return False
        return left.x > right.x
    # Vertical: UP/DOWN on same column
    if sa_x == sb_x and a.x == b.x:
        if not (
            a.direction in (Direction.UP, Direction.DOWN)
            and b.direction in (Direction.UP, Direction.DOWN)
        ):
            return False
        if sa_y == sb_y:
            return False
        upper, lower = (a, b) if sa_y < sb_y else (b, a)
        usy = getattr(upper, "_step_start_y", upper.y)
        lsy = getattr(lower, "_step_start_y", lower.y)
        if usy >= lsy:
            return False
        if upper.direction != Direction.DOWN or lower.direction != Direction.UP:
            return False
        return upper.y > lower.y
    return False


def _shot_shot_explosion_key(shot1: Shot, shot2: Shot, *, crossed: bool) -> tuple[int, int]:
    if not crossed:
        return (shot1.x, shot1.y)
    if shot1.y == shot2.y:
        return ((shot1.x + shot2.x) // 2, shot1.y)
    return (shot1.x, (shot1.y + shot2.y) // 2)


# Debug logging - enabled via TANK_DEBUG env var
DEBUG_LOG_FILE = "tank_debug.log"


_DEBUG = os.environ.get("TANK_DEBUG", "").strip().lower() not in ("", "0", "false", "no", "off")


def _debug_log(msg: str) -> None:
    """Write debug message to log file."""
    if not _DEBUG:
        return
    timestamp = datetime.now().strftime("%H:%M:%S.%f")[:-3]
    with open(DEBUG_LOG_FILE, "a") as f:
        f.write(f"[{timestamp}] {msg}\n")


class GameState(Enum):
    MENU = auto()
    SKILL_SELECT = auto()
    GAME_INIT = auto()
    PLAYING = auto()
    EXPLOSION = auto()
    GAME_OVER = auto()
    PLAY_AGAIN = auto()
    QUIT = auto()


@dataclass
class Explosion:
    x: int
    y: int
    start_time: float
    duration: float
    is_chain_reaction: bool = False  # True if triggered by another explosion (larger)


class GameController:
    def __init__(self) -> None:
        self.state: GameState = GameState.MENU
        self.game_mode: str = "2P"
        self.board = Board()
        self.tanks: dict[int, Tank] = {}
        self.shots: list[Shot] = []
        self.mines: list[Mine] = []
        self.explosions: list[Explosion] = []
        self._barrel_wreckage_registry: list[tuple[tuple[int, int], Direction]] = []
        self._barrel_hit_bodies: set[tuple[int, int]] = set()
        self._empty_gun_pending: set[int] = set()
        self.difficulty: int = 5
        self.battles_played: int = 0
        self.wins: dict[int, int] = {1: 0, 2: 0}
        self.winner: int | None = None

        # Track held keys for 8-way movement (diagonal detection)
        self._held_keys: set[int] = set()
        self._newly_pressed_keys: set[int] = set()  # Keys pressed this frame (no repeats)

        # Movement timing
        self._last_move_time: dict[int, float] = {1: 0.0, 2: 0.0}
        self._move_delay: float = 0.5  # Default delay
        self._shot_delay: float = 0.25  # Default shot delay

        # Barrel swing state: tracks if player has swung but not moved yet
        # Key: player_id, Value: direction they swung to (or None if ready to move)
        self._swing_state: dict[int, Direction | None] = {1: None, 2: None}

    def sim_time_s(self) -> float:
        """Simulation clock."""
        import pygame

        return pygame.time.get_ticks() / 1000.0

    def init_round(self) -> None:
        self.board = Board(difficulty=self.difficulty)
        self._barrel_wreckage_registry = []
        self._barrel_hit_bodies = set()
        self._empty_gun_pending = set()
        tanks, shots, mines = difficulty_to_resources(self.difficulty)

        # Set movement and shot delay based on difficulty
        # The user-selected "level" controls game speed and terrain density
        _debug_log(f"init_round: game_mode={self.game_mode}, difficulty={self.difficulty}")
        self._move_delay = get_move_delay(self.difficulty)
        self._shot_delay = get_shot_delay(self.difficulty)

        self.tanks = {
            1: Tank(
                player_id=1,
                x=2,
                y=10,
                direction=Direction.RIGHT,
                lives=tanks,
                shots_left=shots,
                mines_left=mines,
                start_pos=(2, 10),
            ),
            2: Tank(
                player_id=2,
                x=self.board.width - 3,
                y=10,
                direction=Direction.LEFT,
                lives=tanks,
                shots_left=shots,
                mines_left=mines,
                start_pos=(self.board.width - 3, 10),
            ),
        }
        for tank in self.tanks.values():
            tank.occupy_board(self.board)

        self.shots.clear()
        self.mines.clear()
        self.explosions.clear()
        self.winner = None
        self.state = GameState.PLAYING

        # Reset swing states for new round
        self._swing_state = {1: None, 2: None}

    def handle_input(self, events: list) -> None:
        import pygame

        from .constants import PLAYER1_KEYS, PLAYER2_KEYS

        self._newly_pressed_keys.clear()
        for event in events:
            if event.type == pygame.KEYDOWN:
                # Only track if not already held (filters out key repeats)
                if event.key not in self._held_keys:
                    self._newly_pressed_keys.add(event.key)
                self._held_keys.add(event.key)
            elif event.type == pygame.KEYUP:
                self._held_keys.discard(event.key)

        if self.state == GameState.MENU:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.QUIT
                    else:
                        self.state = GameState.SKILL_SELECT
            return

        if self.state == GameState.SKILL_SELECT:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.MENU
                    elif event.key in (
                        pygame.K_0,
                        pygame.K_1,
                        pygame.K_2,
                        pygame.K_3,
                        pygame.K_4,
                        pygame.K_5,
                        pygame.K_6,
                        pygame.K_7,
                        pygame.K_8,
                        pygame.K_9,
                    ):
                        # 0-9 maps to difficulty 0-9 (0 = easiest, 9 = hardest)
                        key_name = pygame.key.name(event.key)
                        self.difficulty = int(key_name[-1])  # Last char of key name
                        self.init_round()
                    elif event.key == pygame.K_UP or event.key == pygame.K_RIGHT:
                        self.difficulty = min(9, self.difficulty + 1)
                    elif event.key == pygame.K_DOWN or event.key == pygame.K_LEFT:
                        self.difficulty = max(0, self.difficulty - 1)
                    elif event.key == pygame.K_RETURN or event.key == pygame.K_SPACE:
                        self.init_round()
            return

        if self.state == GameState.PLAY_AGAIN:
            for event in events:
                if event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        self.state = GameState.MENU
                    elif event.key == pygame.K_y:
                        # Another battle
                        self.init_round()
                    elif event.key == pygame.K_n:
                        self.state = GameState.MENU
            return

        if self.state != GameState.PLAYING:
            return

        for event in events:
            if event.type != pygame.KEYDOWN:
                continue
            if event.key == pygame.K_ESCAPE:
                self.state = GameState.QUIT
                return

            # Process input for both players
            for player_id in [1, 2]:
                tank = self.tanks[player_id]
                keys = PLAYER1_KEYS if player_id == 1 else PLAYER2_KEYS

                # Check for fire (individual keypress, not held)
                if event.key == keys["fire"] and tank.can_fire():
                    self._fire_shot(tank)
                    continue

                # Check for mine (individual keypress)
                if event.key == keys["mine"] and tank.can_place_mine():
                    cell = self.board.get_cell(tank.x, tank.y)
                    _debug_log(
                        f"MINE_CHECK: player={player_id} at ({tank.x},{tank.y}), cell={cell.type if cell else None}"
                    )
                    # Allow placement on EMPTY or on any tank (placing under yourself)
                    # Block: existing mines, walls, wreckage
                    if cell and cell.type not in {
                        CellType.MINE,
                        CellType.WALL,
                        CellType.WRECKAGE_P1,
                        CellType.WRECKAGE_P2,
                    }:
                        # Owner ID added for mine stealth tracking
                        mine = Mine(x=tank.x, y=tank.y, owner_id=player_id)
                        mine.visible_start_time = self.sim_time_s()
                        self.mines.append(mine)
                        self.board.set_cell_type(tank.x, tank.y, CellType.MINE)
                        tank.consume_mine()
                        _debug_log(
                            f"MINE_PLACED: player={player_id} at ({tank.x},{tank.y}), total mines: {len(self.mines)}"
                        )
                        # Log board state for debugging
                        _debug_log(
                            f"  Board after mine: {[(m.x, m.y, c.type) for m in self.mines if (c := self.board.get_cell(m.x, m.y))]}"
                        )
                    continue

        # Process movement based on newly pressed keys only (no repeats)
        self._process_movement()

    def _process_movement(self) -> None:
        """Process 8-way movement based on NEW keypresses only (filters repeats).

        Barrel swing mechanic:
        - First keypress in ANY direction: swing barrel, don't move
        - Second keypress in SAME direction: actually move
        - Can swing multiple times in different directions without moving
        """
        from .constants import PLAYER1_KEYS, PLAYER2_KEYS

        current_time = self.sim_time_s()

        for player_id in [1, 2]:
            tank = self.tanks[player_id]
            keys = PLAYER1_KEYS if player_id == 1 else PLAYER2_KEYS

            # Check movement delay
            if current_time - self._last_move_time[player_id] < self._move_delay:
                continue

            # Determine desired direction from currently held keys
            desired_direction = None

            # Check dedicated diagonal keys FIRST (takes priority)
            if keys["up_left"] in self._held_keys:
                desired_direction = Direction.UP_LEFT
            elif keys["up_right"] in self._held_keys:
                desired_direction = Direction.UP_RIGHT
            elif keys["down_left"] in self._held_keys:
                desired_direction = Direction.DOWN_LEFT
            elif keys["down_right"] in self._held_keys:
                desired_direction = Direction.DOWN_RIGHT
            else:
                # Determine desired direction from held cardinal keys
                dx, dy = 0, 0

                # Check horizontal
                if keys["left"] in self._held_keys and keys["right"] in self._held_keys:
                    dx = 0
                elif keys["left"] in self._held_keys:
                    dx = -1
                elif keys["right"] in self._held_keys:
                    dx = 1

                # Check vertical
                if keys["up"] in self._held_keys and keys["down"] in self._held_keys:
                    dy = 0
                elif keys["up"] in self._held_keys:
                    dy = -1
                elif keys["down"] in self._held_keys:
                    dy = 1

                # Map dx/dy to Direction enum
                if dx == -1 and dy == -1:
                    desired_direction = Direction.UP_LEFT
                elif dx == 1 and dy == -1:
                    desired_direction = Direction.UP_RIGHT
                elif dx == -1 and dy == 1:
                    desired_direction = Direction.DOWN_LEFT
                elif dx == 1 and dy == 1:
                    desired_direction = Direction.DOWN_RIGHT
                elif dx == -1:
                    desired_direction = Direction.LEFT
                elif dx == 1:
                    desired_direction = Direction.RIGHT
                elif dy == -1:
                    desired_direction = Direction.UP
                elif dy == 1:
                    desired_direction = Direction.DOWN

            # Only process if we have a valid direction AND player just pressed a direction key
            if desired_direction:
                # Check if any direction key was newly pressed this frame
                direction_keys = {
                    keys["up"],
                    keys["down"],
                    keys["left"],
                    keys["right"],
                    keys["up_left"],
                    keys["up_right"],
                    keys["down_left"],
                    keys["down_right"],
                }
                newly_pressed_direction = direction_keys & self._newly_pressed_keys

                if not newly_pressed_direction:
                    # No new direction key pressed this frame, skip
                    continue

                swung_direction = self._swing_state[player_id]

                if desired_direction == tank.direction:
                    # Pressing direction we're already facing - move immediately, no swing needed
                    if tank.attempt_move(self.board, desired_direction):
                        _debug_log(f"MOVE: player={player_id} moved to ({tank.x},{tank.y})")
                        self._last_move_time[player_id] = current_time
                        self._resolve_tank_mine_collision(tank)
                    self._swing_state[player_id] = None  # Reset state
                elif desired_direction == swung_direction:
                    # Second press in same direction we swung to - move now
                    if tank.attempt_move(self.board, desired_direction):
                        self._last_move_time[player_id] = current_time
                        self._resolve_tank_mine_collision(tank)
                    self._swing_state[player_id] = None  # Reset after move
                else:
                    # First press in a new direction - just swing, don't move
                    tank._clear_barrel(self.board)
                    tank.direction = desired_direction
                    tank._place_barrel(self.board)
                    self._swing_state[player_id] = desired_direction

        # Clear newly pressed keys after processing
        self._newly_pressed_keys.clear()

    def _fire_shot(self, tank: Tank, *, direction_override: Direction | None = None) -> None:
        """Fire a shot and check for Empty Gun rule.

        Original TANK! mechanic: only ONE shot per player on screen at a time.

        ``direction_override`` (AI risky fire, FR-8): spawn shot in this direction without
        rotating the tank sprite.
        """
        # Check if this tank already has an active shot on the board
        for shot in self.shots:
            if shot.active:
                # Check if this shot belongs to this tank by looking at spawn position
                # We can infer this by checking if shot is moving AWAY from tank's position
                # Simpler: just don't allow firing if any shot exists from this general direction
                pass

        # Actually check - if we already have a shot heading in this direction, don't fire
        # Count how many shots this player has on screen
        player_shots = 0
        for shot in self.shots:
            if shot.active and shot.owner_id == tank.player_id:
                player_shots += 1

        # Original TANK! mechanic: only 1 shot per player at a time
        if player_shots > 0:
            _debug_log(
                f"SHOT_BLOCKED: player {tank.player_id} already has {player_shots} shot(s) on screen"
            )
            return

        shot_dir = direction_override if direction_override is not None else tank.direction
        shot_pos = self._shot_spawn_position(tank, shot_dir)
        if shot_pos is None:
            _debug_log(f"SHOT_BLOCKED: player {tank.player_id} no valid spawn position")
            return

        x, y = shot_pos
        dx, dy = DIRECTION_VECTORS[shot_dir]
        # Max range = 75% of relevant board dimension
        if dx != 0 and dy != 0:
            max_range = int(0.75 * min(self.board.width, self.board.height))
        elif dx != 0:
            max_range = int(0.75 * self.board.width)
        else:
            max_range = int(0.75 * self.board.height)
        self.shots.append(Shot(x=x, y=y, direction=shot_dir, owner_id=tank.player_id,
                               max_range=max_range))
        tank.consume_shot()
        _debug_log(f"SHOT_FIRED: player={tank.player_id} pos=({x},{y}) dir={tank.direction.name}")

        # Empty Gun Rule: deferred — wait for last projectile to resolve
        if not tank.can_fire():
            self._empty_gun_pending.add(tank.player_id)
            _debug_log(f"EMPTY_GUN_PENDING: player={tank.player_id} fired last shot, awaiting resolution")

    def _shot_spawn_position(
        self, tank: Tank, fire_direction: Direction | None = None
    ) -> tuple[int, int] | None:
        d = fire_direction if fire_direction is not None else tank.direction
        dx, dy = DIRECTION_VECTORS[d]
        x = tank.x + dx
        y = tank.y + dy
        if not self.board.in_bounds(x, y):
            return None
        cell = self.board.get_cell(x, y)
        if cell is None or cell.type == CellType.WALL:
            return None
        return x, y

    def _resolve_tank_mine_collision(self, tank: Tank) -> None:
        for mine in list(self.mines):
            if mine.active and mine.x == tank.x and mine.y == tank.y:
                _debug_log(f"TANK_MINE_COLLISION: player={tank.player_id} at ({tank.x},{tank.y})")
                _debug_log(f"  Mines list: {[(m.x, m.y, m.active) for m in self.mines]}")
                # Tank stepped on mine - process respawn FIRST, then explosion
                # The tank that stepped on the mine takes damage
                self._tank_hit(tank, (tank.x, tank.y))

                mine.active = False
                # Trigger 360° explosion (tanks already respawned, won't be hit again)
                self._explode_mine(mine.x, mine.y)
                _debug_log(
                    f"  -> Explosion complete, remaining mines: {[(m.x, m.y, m.active) for m in self.mines]}"
                )
                break  # Only trigger one mine per collision

    def _place_dice_wreckage(self, cx: int, cy: int, player_id: int) -> None:
        """Place 4-dot dice pattern (corners of a 3x3 area) as wreckage."""
        wreckage_type = CellType.WRECKAGE_P1 if player_id == 1 else CellType.WRECKAGE_P2
        for dx, dy in ((-1, -1), (1, -1), (-1, 1), (1, 1)):
            nx, ny = cx + dx, cy + dy
            if not self.board.in_bounds(nx, ny):
                continue
            cell = self.board.get_cell(nx, ny)
            if cell is None:
                continue
            if cell.type in {CellType.EMPTY, CellType.WALL, CellType.SHOT,
                             CellType.MINE, CellType.BARREL1, CellType.BARREL2}:
                self.board.set_cell_type(nx, ny, wreckage_type)

    def _tank_hit(self, tank: Tank, pos: tuple[int, int]) -> None:
        _debug_log(
            f"TANK_HIT: player={tank.player_id} at ({pos[0]},{pos[1]}), lives_before={tank.lives}"
        )

        tank.clear_from_board(self.board)
        self._place_dice_wreckage(pos[0], pos[1], tank.player_id)

        tank.take_damage()
        exp = Explosion(
            x=pos[0], y=pos[1], start_time=self.sim_time_s(), duration=0.5
        )
        self.explosions.append(exp)
        _debug_log(f"  -> explosion added at ({pos[0]},{pos[1]}), lives_after={tank.lives}")

        if not tank.is_alive():
            _debug_log(f"  -> player {tank.player_id} DEFEATED!")
            self._on_player_defeated(tank.player_id)
        else:
            _debug_log(f"  -> player {tank.player_id} alive, respawning both")
            # Both tanks respawn to starting positions after each kill
            self._respawn_both_tanks()

    def _barrel_hit(self, tank: Tank, barrel_pos: tuple[int, int]) -> None:
        """Barrel shot: no explosion, only the hit player loses a life and respawns."""
        _debug_log(
            f"BARREL_HIT: player={tank.player_id} barrel at ({barrel_pos[0]},{barrel_pos[1]}), lives_before={tank.lives}"
        )
        # Leave curled-barrel wreckage (body + barrel positions)
        wreckage_type = CellType.WRECKAGE_P1 if tank.player_id == 1 else CellType.WRECKAGE_P2
        body_pos = (tank.x, tank.y)
        barrel_dir = tank.direction
        tank.clear_from_board(self.board)
        self.board.set_cell_type(body_pos[0], body_pos[1], wreckage_type)
        self.board.set_cell_type(barrel_pos[0], barrel_pos[1], wreckage_type)
        self._barrel_hit_bodies.add(body_pos)
        self._barrel_wreckage_registry.append((barrel_pos, barrel_dir))

        tank.take_damage()
        _debug_log(f"  -> lives_after={tank.lives}")

        if not tank.is_alive():
            _debug_log(f"  -> player {tank.player_id} DEFEATED!")
            self._on_player_defeated(tank.player_id)
        else:
            self._respawn_single_tank(tank.player_id)

    def _find_spawn_pos(self, start_x: int, start_y: int) -> tuple[int, int]:
        """Find a clear cell for spawning, scanning up then down from start_pos.

        Needs room for both body AND barrel (barrel faces right for P1, left for P2).
        """
        for offset in range(0, self.board.height):
            for dy in (-offset, offset) if offset else (0,):
                y = start_y + dy
                if y < 1 or y >= self.board.height - 1:
                    continue
                cell = self.board.get_cell(start_x, y)
                if cell and cell.type == CellType.EMPTY:
                    return start_x, y
        return start_x, start_y  # fallback (shouldn't happen)

    def _respawn_single_tank(self, player_id: int) -> None:
        """Respawn only the specified player to their start position (other stays)."""
        _, shots, mines = difficulty_to_resources(self.difficulty)
        tank = self.tanks[player_id]
        tank.clear_from_board(self.board)
        sx, sy = self._find_spawn_pos(*tank.start_pos)
        tank.x = sx
        tank.y = sy
        tank.direction = Direction.RIGHT if player_id == 1 else Direction.LEFT
        tank.occupy_board(self.board)
        tank.shots_left = shots
        tank.mines_left = mines
        self._swing_state[player_id] = None
        _debug_log(
            f"Respawned player {player_id} at ({sx},{sy}) with {shots} shots, {mines} mines"
        )

    def _respawn_both_tanks(self) -> None:
        """Respawn both tanks to their start positions after a kill."""
        tanks, shots, mines = difficulty_to_resources(self.difficulty)

        for tank in self.tanks.values():
            tank.clear_from_board(self.board)
            sx, sy = self._find_spawn_pos(*tank.start_pos)
            tank.x = sx
            tank.y = sy
            tank.direction = Direction.RIGHT if tank.player_id == 1 else Direction.LEFT
            tank.occupy_board(self.board)
            tank.shots_left = shots
            tank.mines_left = mines
            self._swing_state[tank.player_id] = None

            _debug_log(
                f"Respawned player {tank.player_id} at ({sx},{sy}) with {shots} shots, {mines} mines"
            )


    def _on_player_defeated(self, player_id: int) -> None:
        self.winner = 1 if player_id == 2 else 2
        self.wins[self.winner] += 1
        self.battles_played += 1

        # Don't end game - continue with respawn
        self._respawn_both_tanks()

        # Check if either player is out of lives - only then end game
        if not self.tanks[1].is_alive() or not self.tanks[2].is_alive():
            self.state = GameState.PLAY_AGAIN
            self._show_victory_message()

    def _show_victory_message(self) -> None:
        """Generate victory message based on remaining lives."""
        winner = self.winner
        if winner is None:
            return

        winner_tank = self.tanks[winner]
        remaining_lives = winner_tank.lives

        if remaining_lives == 1:
            suffix = "WITH HIS LAST TANK!"
        elif remaining_lives == 2:
            suffix = "WITH TWO TANKS LEFT!"
        else:  # 3 lives (perfect game)
            suffix = "WITHOUT LOSING A TANK!"

        self._victory_message = f"PLAYER #{winner} WINS!\n{suffix}"

    def get_victory_message(self) -> str:
        """Return the current victory message."""
        return getattr(self, "_victory_message", "")

    def update(self, dt: float) -> None:
        if self.state != GameState.PLAYING:
            return
        self._update_shots()
        self._update_mines()
        self._update_explosions()

    def _update_mines(self) -> None:
        """Update mine visibility - mines become invisible after 2 seconds."""
        current_time = self.sim_time_s()
        mine_visible_duration = 2.0  # seconds

        for mine in self.mines:
            if mine.active and mine.visible:
                if current_time - mine.visible_start_time > mine_visible_duration:
                    mine.visible = False

    def _update_shots(self) -> None:
        """Update shot positions with speed delay."""
        current_time = self.sim_time_s()
        shot_delay = self._shot_delay  # Use dedicated shot delay (0.5s to 0.05s)

        # Snapshot positions before stepping (for head-on pass-through detection)
        for shot in self.shots:
            if shot.active:
                shot._step_start_x = shot.x
                shot._step_start_y = shot.y

        # First, move all shots and check for any collisions at new positions
        for shot in list(self.shots):
            # Check if enough time has passed since last shot move
            last_move = getattr(shot, "_last_move_time", 0)
            if current_time - last_move < shot_delay:
                continue

            if not shot.active:
                if _DEBUG:
                    _debug_log(
                        f"SHOT_INACTIVE: player={shot.owner_id} at ({shot.x},{shot.y}), waiting for hit check"
                    )
                continue
            collision_pos = shot.step(self.board)

            if shot.active:
                shot._last_move_time = current_time
            else:
                if _DEBUG:
                    _debug_log(f"SHOT_HIT: player={shot.owner_id} at collision={collision_pos}")
                shot._collision_pos = collision_pos

        # Shot-shot: same cell, or head-on swap on same row/column (adjacent cells, one step)
        indices_to_remove = set()
        explosions_triggered = set()

        for i, shot1 in enumerate(self.shots):
            if not shot1.active or i in indices_to_remove:
                continue
            for j, shot2 in enumerate(self.shots):
                if j <= i or not shot2.active or j in indices_to_remove:
                    continue
                same_cell = shot1.x == shot2.x and shot1.y == shot2.y
                crossed = _shots_crossed_head_on(shot1, shot2)
                if same_cell or crossed:
                    indices_to_remove.add(i)
                    indices_to_remove.add(j)
                    key = _shot_shot_explosion_key(shot1, shot2, crossed=crossed)
                    if key not in explosions_triggered:
                        explosions_triggered.add(key)
                        if crossed:
                            _debug_log(
                                f"SHOT_SHOT_COLLISION: shots head-on crossed -> {key}, chain reaction!"
                            )
                        else:
                            _debug_log(
                                f"SHOT_SHOT_COLLISION: shots collided at {key}, chain reaction!"
                            )
                        self._explode_mine(key[0], key[1], radius=2, is_chain=True)

        # Deactivate collided shots
        for idx in indices_to_remove:
            shot = self.shots[idx]
            _debug_log(
                f"SHOT_REMOVED: player={shot.owner_id} SHOT_SHOT_COLLISION at ({shot.x},{shot.y})"
            )
            shot.active = False

        # Now process other collisions (tanks, mines)
        # Need to check BOTH active shots AND shots that just hit something (inactive but with collision_pos)
        for shot in list(self.shots):
            # Check collision position from step() FIRST (most recent hit)
            collision_pos = getattr(shot, "_collision_pos", None)
            if collision_pos:
                cx, cy = collision_pos
                shot._collision_pos = None  # Clear after checking
            else:
                # For active shots without collision, check current position
                if not shot.active:
                    continue
                cx, cy = shot.x, shot.y

            cell = self.board.get_cell(cx, cy)
            if cell is None:
                continue

            if cell.type in {CellType.BARREL1, CellType.BARREL2}:
                target_id = 1 if cell.type == CellType.BARREL1 else 2
                if target_id == shot.owner_id:
                    continue  # can't barrel-hit yourself
                _debug_log(f"SHOT_HIT_BARREL: player={target_id} at ({cx},{cy})")
                self._barrel_hit(self.tanks[target_id], (cx, cy))
            elif cell.type in {CellType.TANK1, CellType.TANK2}:
                target_id = 1 if cell.type == CellType.TANK1 else 2
                _debug_log(f"SHOT_HIT_TANK: player={target_id} at ({cx},{cy})")
                self._tank_hit(self.tanks[target_id], (cx, cy))
            elif cell.type == CellType.MINE:
                for mine in self.mines:
                    if mine.x == cx and mine.y == cy and mine.active:
                        mine.active = False
                        _debug_log(
                            f"SHOT_MINE_HIT: shot hit mine at ({cx},{cy}), triggering chain reaction!"
                        )
                        self._explode_mine(mine.x, mine.y, radius=2, is_chain=True)
                        break
            elif getattr(shot, "_hit_wall", False):
                pass  # wall already removed in Shot.step(); no splash in original PET

        # Clean up: remove shots that hit something (inactive, had collision_pos that was processed)
        self.shots = [
            shot
            for shot in self.shots
            if shot.active or getattr(shot, "_collision_pos", None) is not None
        ]
        _debug_log(f"Shot count after cleanup: {len(self.shots)}")

        # Deferred Empty Gun Rule: self-destruct players whose last shot has resolved
        if self._empty_gun_pending:
            for pid in list(self._empty_gun_pending):
                has_active = any(s.active for s in self.shots if s.owner_id == pid)
                if not has_active:
                    self._empty_gun_pending.discard(pid)
                    tank = self.tanks.get(pid)
                    if tank and tank.is_alive() and self.state == GameState.PLAYING:
                        _debug_log(f"EMPTY_GUN_DESTRUCT: player={pid} last shot resolved, self-destruct")
                        self._tank_hit(tank, (tank.x, tank.y))

    def _explode_mine(self, mx: int, my: int, radius: int = 1, is_chain: bool = False) -> None:
        """Explode a mine, destroying everything in specified radius.

        Args:
            mx, my: Mine position
            radius: Explosion radius (1 = 360°, 2 = 2-cell radius for chain reactions)
            is_chain: True if triggered by another explosion (for visual distinction)
        """
        _debug_log(f"EXPLODE_MINE: pos=({mx},{my}) radius={radius} is_chain={is_chain}")

        current_time = self.sim_time_s()

        # Add explosion at mine position - chain reactions are larger
        self.explosions.append(
            Explosion(
                x=mx,
                y=my,
                start_time=current_time,
                duration=0.5 if not is_chain else 0.8,  # Longer for chain
                is_chain_reaction=is_chain,
            )
        )
        _debug_log(f"  -> Added explosion, is_chain={is_chain}")

        # Clear the mine itself
        self.board.set_cell_type(mx, my, CellType.EMPTY)
        _debug_log(f"  -> Cleared mine at ({mx},{my})")

        # Destroy everything in specified radius (including diagonals)
        for dy in range(-radius, radius + 1):
            for dx in range(-radius, radius + 1):
                nx, ny = mx + dx, my + dy
                if not self.board.in_bounds(nx, ny):
                    continue

                cell = self.board.get_cell(nx, ny)
                if cell is None:
                    continue

                _debug_log(f"  -> Check cell ({nx},{ny}): type={cell.type}")

                # Destroy walls/terrain
                if cell.type == CellType.WALL:
                    self.board.set_cell_type(nx, ny, CellType.EMPTY)
                    _debug_log(f"  -> Destroyed WALL at ({nx},{ny})")
                # Trigger other mines with MAGNIFIED 2-cell radius
                elif cell.type == CellType.MINE:
                    _debug_log(f"  -> Found MINE at ({nx},{ny}), checking active mines...")
                    for mine in self.mines:
                        _debug_log(
                            f"      Checking mine at ({mine.x},{mine.y}): active={mine.active}"
                        )
                        if mine.x == nx and mine.y == ny and mine.active:
                            mine.active = False
                            _debug_log(f"  -> Chain reaction! Triggering mine at ({nx},{ny})")
                            self._explode_mine(nx, ny, radius=2, is_chain=True)  # Chain reaction!
                            break
                # Damage tanks (body hit)
                elif cell.type in {CellType.TANK1, CellType.TANK2}:
                    target_id = 1 if cell.type == CellType.TANK1 else 2
                    _debug_log(f"  -> Hit TANK{target_id} at ({nx},{ny})")
                    self._tank_hit(self.tanks[target_id], (nx, ny))
                # Explosions pass through barrels without triggering barrel hits.
                # Barrel hits only come from direct projectile contact.
                elif cell.type in {CellType.BARREL1, CellType.BARREL2}:
                    pass
                # Destroy wreckage
                elif cell.type in {CellType.WRECKAGE_P1, CellType.WRECKAGE_P2}:
                    self.board.set_cell_type(nx, ny, CellType.EMPTY)

    def _update_explosions(self) -> None:
        now = self.sim_time_s()
        self.explosions = [exp for exp in self.explosions if now - exp.start_time < exp.duration]
