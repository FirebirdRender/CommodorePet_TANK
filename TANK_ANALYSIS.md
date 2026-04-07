# TANK! Game Analysis & Python/PyGame Recreation Guide

## Original Game Overview

**Title:** TANK!  
**Authors:** Shaun Meehan & Mike Rowley  
**Copyright:** 1981 The Code Works  
**Platform:** Commodore PET 4016 (Commodore BASIC V4)  
**Original Publication:** Cursor Magazine #26  
**Game Type:** Two-player competitive tank battle (no AI - hardware limitations)

---

## How the Original Game Works

### Core Gameplay
This is a **two-player competitive tank battle game** designed for the Commodore PET 4016. Players control tanks on a 40×25 character screen and attempt to destroy each other's tanks using shots and mines. The original game was strictly 2-player due to the PET 4016's limited memory and processing power.

### Game State & Variables

| Component | Purpose |
|-----------|---------|
| `P(1), P(2)` | Player 1 & 2 starting positions (32801, 33718 in screen RAM) |
| `T(1), T(2)` | Current tank positions |
| `C(1), C(2)` | Tank count (lives) per player (typically 3) |
| `S(1), S(2)` | Shot count (ammunition) per player (typically 25) |
| `M(1), M(2)` | Mine count (mines) per player (typically 3) |
| `R(1), R(2)` | Character codes for tank rendering (170 & 182) |
| `D(X)` | Active mine/shot positions array |
| `WD` | Screen width (40 characters) |
| `UD` | Screen height (25 characters) |
| `CT` | Screen RAM start address (32768) |

### Screen Layout
```
Player 1 Controls          40 char width           Player 2 Controls
(Left side)                                        (Right side)
Tank 1: [®]                                        Tank 2: [¶]
Position: ~4 chars in      Playfield (obstacles)   Position: ~37 chars in

Status Line:
TANKS | SHOTS | MINES
  3   |  25   |  3    (per player)
```

### Game Mechanics

#### 1. **Player Movement**
- Players move their tanks around the playfield
- Movement appears to be tile-based or grid-based
- Collision detection prevents movement through obstacles and other tanks
- Tank sprites: Character 170 (Player 1), Character 182 (Player 2)

#### 2. **Combat System**
- **Shots**: Players fire projectile shots (limited ammo, ~25 per tank)
- **Mines**: Stationary traps that detonate when hit by enemy shots
- **Impact Detection**: Shot hits trigger explosion animation and sound

#### 3. **Destruction & Explosions**
- Tank hit displays "HIT A MINE" or similar message
- Explosion animation plays with character graphics:
  - Pattern: `M B N` (top row)
  - Pattern: `- * -` (middle row)  
  - Pattern: `N B M` (bottom row)
- Explosion sound effect via user port (location 59467/59466/59464)
- Tank respawns if player has remaining lives

#### 4. **Win Conditions**
- **Single Tank Win**: Destroy all opponent tanks → "PLAYER # WINS!"
- **Victory Messages**:
  - With 1 tank left: "WITH HIS LAST TANK!"
  - With 2 tanks left: "WITH TWO TANKS LEFT!"
  - Perfect game: "WITHOUT LOSING A TANK!"
- Tracks cumulative battles and overall winner

### Game Mechanics

#### 1. **Player Movement**
- Players move their tanks around the playfield
- Movement appears to be tile-based or grid-based
- Collision detection prevents movement through obstacles and other tanks
- Tank sprites: Character 170 (Player 1), Character 182 (Player 2)

#### 2. **Combat System**
- **Shots**: Players fire projectile shots (limited ammo, ~25 per tank)
- **Mines**: Stationary traps that detonate when hit by enemy shots
- **Impact Detection**: Shot hits trigger explosion animation and sound

#### 3. **Destruction & Explosions**
- Tank hit displays "HIT A MINE" or similar message
- Explosion animation plays with character graphics:
  - Pattern: `M B N` (top row)
  - Pattern: `- * -` (middle row)  
  - Pattern: `N B M` (bottom row)
- Explosion sound effect via user port (location 59467/59466/59464)
- Tank respawns if player has remaining lives

#### 4. **Win Conditions**
- **Single Tank Win**: Destroy all opponent tanks → "PLAYER # WINS!"
- **Victory Messages**:
  - With 1 tank left: "WITH HIS LAST TANK!"
  - With 2 tanks left: "WITH TWO TANKS LEFT!"
  - Perfect game: "WITHOUT LOSING A TANK!"
- Tracks cumulative battles and overall winner

#### 5. **Difficulty Levels**
- Player selects skill level 1-10 at game start
- Difficulty affects:
  - `SS` (shot/score value) = 5 × difficulty
  - `R` (opponent reaction/AI accuracy)
  - Mine availability

#### 6. **Audio Feedback**
- Uses Commodore PET user port for sound generation
- Explosion sounds: Frequency-swept tone patterns
- Engine/movement sounds: Pulsed tones
- Victory/defeat fanfares

#### 6. **Audio Feedback**
- Uses Commodore PET user port for sound generation
- Explosion sounds: Frequency-swept tone patterns
- Engine/movement sounds: Pulsed tones
- Victory/defeat fanfares

---

## What the Game Does (Gameplay Loop)

1. **Initialization**
   - Display title/credits
   - Prompt for skill level (1-10) - affects resource allocation, not AI
   - Create game board with boundaries
   - Place Player 1 tank (left) and Player 2 tank (right)

2. **Main Game Loop**
   - Display current status (tank count, shots, mines)
   - Await player input (movement keys: A/S/D/F/G/H/J/K/L)
   - Move tank based on input
   - Check for collisions with obstacles, mines, shots
   - If collision detected → explosion → tank destruction
   - If player tank destroyed → respawn if lives remain
   - **Both players take turns** (no simultaneous play)
   - Repeat until one player has 0 tanks

3. **Game End**
   - Display winner and victory statistics
   - Prompt for another battle
   - Track cumulative wins across multiple battles

4. **Session End**
   - Show overall tournament results
   - Display final champion

---

## Technical Implementation Details

### Memory Layout (Commodore PET)
- Screen RAM: $8000 (32768) - $87E7 (34791)
- Video ROM/Character definitions start at specific addresses
- Sound port addresses: 59464-59467 (via user port)

### Screen Rendering
- Direct screen memory (character RAM) writes using `POKE`
- Character codes for graphics:
  - Space (32): Empty area
  - `|` (124): Vertical border
  - `^`, `v`, `<`, `>`: Direction indicators
  - Custom tank glyphs: 170, 182

### Collision Detection
- Checks adjacent screen positions:
  - North: position - 40 (width)
  - South: position + 40
  - East: position + 1
  - West: position - 1
- Prevents movement into non-space characters

---

## Python/PyGame Recreation Specification

### Project Structure
```
tank_game/
├── main.py                 # Entry point
├── game.py                 # Game controller & main loop
├── player.py               # Tank class & player logic
├── board.py                # Game board/arena
├── projectile.py           # Shot & mine classes
├── graphics.py             # Sprite mgmt & rendering
├── audio.py                # Sound effects
├── ui.py                   # Menu, status display
└── constants.py            # Game constants & config
```

### Core Classes & Components

#### 1. **GameController** (main.py / game.py)
```python
class GameController:
    def __init__(self):
        self.board = Board(40, 25)
        self.player1 = Tank(player_id=1)
        self.player2 = Tank(player_id=2)
        self.shots = []           # Active shots
        self.mines = []           # Placed mines
        self.current_turn = 1
        self.game_state = "playing" # playing/game_over/menu
    
    def update(self):
        # Handle input
        # Update tank positions
        # Check collisions
        # Update projectiles
        # Check win condition
        # Render
    
    def is_collision(self, pos):
        # Check if position has obstacle/tank/mine
        pass
```

#### 2. **Tank** (player.py)
```python
class Tank:
    def __init__(self, player_id, x, y):
        self.player_id = player_id
        self.x = x
        self.y = y
        self.direction = 0  # 0-3 or N/S/E/W
        self.lives = 3
        self.shots_ammo = 25
        self.mines_ammo = 3
        self.sprite = self._get_sprite()
    
    def move(self, dx, dy):
        # Validate move, update position
        pass
    
    def fire_shot(self):
        # Create projectile, reduce ammo
        pass
    
    def place_mine(self):
        # Create mine at current position, reduce ammo
        pass
    
    def take_damage(self):
        # Trigger explosion, reduce lives
        pass
```

#### 3. **Projectile** (projectile.py)
```python
class Shot:
    def __init__(self, start_x, start_y, direction):
        self.x = start_x
        self.y = start_y
        self.direction = direction
        self.active = True
    
    def update(self):
        # Move in direction
        # Check for collisions
        # Mark inactive if hit something
        pass

class Mine:
    def __init__(self, x, y):
        self.x = x
        self.y = y
        self.active = True
        self.triggered = False
    
    def explode(self):
        # Trigger explosion animation
        pass
```

#### 4. **Board** (board.py)
```python
class Board:
    def __init__(self, width, height):
        self.width = width
        self.height = height
        self.grid = [[' ' for _ in range(width)] for _ in range(height)]
        self._generate_layout()
    
    def draw_border(self):
        # Draw arena boundaries
        pass
    
    def is_valid_position(self, x, y):
        # Check if position is within bounds
        pass
    
    def get_obstacle(self, x, y):
        # Return what's at this position
        pass
```

#### 5. **AI** (ai.py) - *FUTURE ENHANCEMENT*
```python
class AIPlayer:
    def __init__(self, difficulty=5):  # 1-10 difficulty
        self.difficulty = difficulty
        self.reaction_time = 10 - (difficulty * 0.9)
        self.fire_accuracy = 0.3 + (difficulty * 0.07)
    
    def decide_move(self, game_state):
        # Determine best action based on game state
        # Higher difficulty = better strategy
        pass
```
*Note: Original game was strictly 2-player due to PET 4016 memory/processing limitations. AI would be a modern enhancement.*

#### 6. **Graphics** (graphics.py)
```python
class SpriteManager:
    def __init__(self):
        self.tank_sprites = {}      # Tank images per direction
        self.explosion_sprite = []  # Explosion sequence
        self.mine_sprite = surface  # Mine image
    
    def draw_tank(self, surface, tank):
        # Render tank sprite at position
        pass
    
    def draw_explosion(self, surface, x, y, frame):
        # Animate explosion
        pass
```

#### 7. **Audio** (audio.py)
```python
class SoundManager:
    def __init__(self):
        self.explosion = pygame.mixer.Sound('explosion.wav')
        self.fire = pygame.mixer.Sound('fire.wav')
        self.engine_idle = pygame.mixer.Sound('engine.wav')
    
    def play_explosion(self):
        self.explosion.play()
    
    def play_fire(self):
        self.fire.play()
```

#### 8. **UI** (ui.py)
```python
class StatusDisplay:
    def draw_status(self, surface, player1, player2):
        # Draw remaining tanks, shots, mines for both players
        pass
    
    def draw_message(self, surface, message):
        # Draw centered message (HIT A MINE, PLAYER 1 WINS, etc)
        pass

class Menu:
    def skill_level_select(self):
        # Prompt for skill 1-10
        pass
```

### Key Features to Implement

#### Rendering System
- **PyGame Surface**: 800×600px or 1024×768px
- **Cell Size**: ~20px per character
- **Resolution**: Native 40×25 character grid
- **Color Scheme**:
  - Background: Black
  - Grid/Arena: Green (classic terminal)
  - Player 1 Tank: Yellow/Gold
  - Player 2 Tank: Red/Cyan
  - Shots: White
  - Mines: Gray/Purple
  - Explosions: Orange/Yellow sequence

#### Input Handling
```python
KEY_MAP = {
    pygame.K_w: 'up',       # or A key
    pygame.K_a: 'left',     # or S key
    pygame.K_s: 'down',     # or D key
    pygame.K_d: 'right',    # or F key
    pygame.K_SPACE: 'fire',
    pygame.K_m: 'mine',
}
```

#### Collision Detection Algorithm
```python
def check_collision(x, y):
    # Check if position contains:
    # - Arena boundary
    # - Enemy tank
    # - Placed mine
    # - Obstacle
    # Return collision type or None
```

#### Explosion Animation
```python
EXPLOSION_FRAMES = [
    [(x, y-1): 'M', (x, y): 'B', (x, y+1): 'N'],
    [(x, y-1): '*', (x, y): '*', (x, y+1): '*'],
    # ... more frames
]
animation_time = 0.5  # seconds
```

#### Game State Machine
```
MENU
  ↓
SKILL_SELECT
  ↓
GAME_INIT
  ↓
PLAYING → EXPLOSION → RESPAWN/GAME_OVER
  ↓
GAME_OVER (show winner)
  ↓
PLAY_AGAIN? → (loop) or QUIT
```

### Configuration Constants (constants.py)
```python
# Screen
SCREEN_WIDTH = 40
SCREEN_HEIGHT = 25
CELL_SIZE = 20

# Game
PLAYER1_START = (2, 12)
PLAYER2_START = (37, 12)
INITIAL_TANKS = 3
INITIAL_SHOTS = 25
INITIAL_MINES = 3
DIFFICULTY_RANGE = (1, 10)

# Physics
SHOT_SPEED = 3  # cells per frame
TANK_SPEED = 2  # cells per frame
EXPLOSION_DURATION = 0.5  # seconds
MINE_DETONATION_TIME = 0.1  # seconds

# Colors
COLOR_BG = (0, 0, 0)
COLOR_TANK_1 = (255, 255, 0)
COLOR_TANK_2 = (255, 0, 0)
COLOR_SHOT = (255, 255, 255)
```

### Implementation Priority

**Phase 1 (MVP)**
- [ ] PyGame initialization & main loop
- [ ] Game board (40×25 grid)
- [ ] Tank class & rendering
- [ ] Basic movement (WASD)
- [ ] Collision detection
- [ ] Single-player vs stationary target

**Phase 2 (Core Game)**
- [ ] Projectile system (shots & mines)
- [ ] Explosion animations
- [ ] Win condition detection
- [ ] Status display (lives, ammo)
- [ ] Score tracking

**Phase 3 (Polish)**
- [ ] Sound effects
- [ ] Menu system
- [ ] Skill level selection
- [ ] Multiple battle tracking

**Phase 4 (Future Enhancements)**
- [ ] AI opponent (difficulty levels) - *Original game was 2-player only*
- [ ] Obstacles/arena impediments
- [ ] Special power-ups
- [ ] Direction-based tank rendering
- [ ] Networked multiplayer (optional)

---

## Conversion Notes

### Key Differences (BASIC → Python/PyGame)

| Original (BASIC) | Python/PyGame | Notes |
|------------------|---------------|-------|
| Character RAM (POKE) | PyGame Surface | Direct pixel drawing |
| User Port 59464-59467 | pygame.mixer | Sound synthesis |
| GOSUB/RETURN | Function calls | Better control flow |
| Array indexing | Dictionary/List | More Pythonic |
| GET (blocking) | pygame.event.get() | Non-blocking input |
| Screen dimensions: 40×25 chars | Scalable resolution | Scale cell size |

### Modernizations to Consider
1. **Full-screen toggle** (Alt+Enter)
2. **Pause menu** during gameplay
3. **Replay system** (save game state)
4. **Configuration menu** (key bindings, difficulty presets)
5. **Smoother animations** (interpolation between cells)
6. **Network play** (if desired)

---

## Testing Strategy

### Unit Tests
- Tank movement validation
- Collision detection edge cases
- Projectile trajectory
- Win condition logic

### Integration Tests
- Full game flow (menu → play → end)
- AI vs player
- Multiplayer game with 0-60fps variations

### Manual Tests
- Both players can move and fire
- Explosion animations trigger correctly
- No "stuck" states
- Score tracking is accurate
- Difficulty changes behavior appropriately

---

## Additional Resources

### Commodore PET 4016 References
- Screen RAM: $8000-$87E7 (40×25 display)
- Character set: Standard ASCII + graphics
- User port: $EA60-$EA63 (sound I/O)

### PyGame Documentation
- [PyGame Getting Started](https://www.pygame.org/wiki/tutorials/GettingStarted)
- [Collision Detection](https://www.pygame.org/wiki/tutorials/Pygame/Collision%20Detection)
- [Sound/Music](https://www.pygame.org/wiki/tutorials/Pygame/Sound)

