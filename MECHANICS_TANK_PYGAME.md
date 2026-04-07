## TANK! Mechanics – Python/PyGame Port Reference

### AI (CPU) decision layer

- Tank AI uses a **py_trees** tree (`tank_game/ai_tree.py`): root **`ReactionTimeGate`** (PRD S1) then a **Selector** — active evasion → frustrated advance → dodge → too-close evasion (wrapped in **`EvasionCooldownClear`** `EternalGuard`) → aimed fire → **risky fire** (FR-8) → chase / unstick → scan or advance. **`TANK_LEGACY_AI=1`** keeps the old inline ladder without decorators.
- Per-frame world facts are bound on `AIPlayer` (`_bind_tick_inputs`); **`_sync_combat_perception`** (`ai_perception.py`) updates **last-known** from peripheral radius and **combat aim** only for targets in a **forward ~90°** cone (FR-9).
- **Skill scaling (SHOWDOWN / balance):** per-agent **reaction time** scales with that AI’s **difficulty** when tied to `move_delay`; **dodge** is disabled at skill **0**; **ammo discipline** (FR-10) adds reserve on top of **confidence_threshold** via `effective_min_shots_for_aimed()`; **risky** shots pick a random 8-way aim via **`RISKY_PREPARE`** then fire on the next reaction tick (same facing as the sprite), then restore the prior direction — low skill may **skip** a shot opportunity (`should_skip_fire`) — hesitation before firing.
- **Evasion / stuck:** The behaviour tree’s **`ActiveEvasion`** leaf keeps choosing **MOVE_AWAY_FROM_ENEMY** while `_is_evading` is true, so **stuck detection leaves** (`_check_stuck` in chase/unstick) **do not run** during that phase. **`GameController`** now implements a real **random-direction fallback** when “step away from enemy” is blocked (`_ai_try_random_escape_move`), and **clears evasion** if neither away-step nor random move is legal — fixing the previous log-only “trying random” path that left tanks motionless. Evade vector uses **`enemy_pos` or `last_known_enemy_pos`**.
- Each new round rebuilds `AIPlayer` instances — tree state resets with the controller. Set **`TANK_LEGACY_AI=1`** for the legacy decision path. Details: `docs/PRD_py_trees_integration.md`.
- **Navigation memory (PRD):** `tank_game/ai_navigation.py` — tabu failed edges, wall belief from vision + failed moves into walls, bounded A* when far or without combat LOS, greedy when close; respawn softens belief; explosions invalidate region. New `GameController` round / `init_round` still replaces `AIPlayer` (fresh memory). See **`docs/PRD_AI_NAVIGATION.md`**.

**FR-5 — state reset on new round (`GameController.init_round`):**

- New `Board`, tanks at spawn, cleared shots/mines/explosions; `winner` cleared; `GameState.PLAYING`.
- **AI:** fresh `AIPlayer` instances (new behaviour tree, no carry-over frustration, mine memory, or tick fields).
- Swing / SHOWDOWN bookkeeping updated elsewhere as needed for the active mode.

### Core Entities

- **Board**: 40×25 logical grid, bordered by solid walls; interior cells may be empty or contain tanks, shots, or mines.
- **Tank (per player)**:
  - Start positions: Player 1 near left edge, Player 2 near right edge (mirroring PET layout).
  - State: grid position, facing direction, lives, shots ammo, mines ammo.
  - Resources (baseline at medium skill): 3 lives, 25 shots, 3 mines.
- **Shot**:
  - Spawns from active tank, moves 1+ cells per tick along cardinal direction.
  - Expires on collision with wall, tank, or mine, triggering explosion.
- **Mine**:
  - Placed on the active tank’s current cell.
  - Static; detonates when hit by opponent shot or tank entering its cell.

### Turn & Input Model

- **Two-player local game**, no AI in core parity version.
- **Turn-based rhythm on top of real-time loop**:
  - Game runs as a continuous PyGame loop for rendering and input.
  - At any moment, one player is considered the **active turn owner**.
  - A turn ends when the active player performs a primary action:
    - Moving their tank one step (up/down/left/right), or
    - Firing a shot, or
    - Placing a mine (if mines remain).
  - After the action resolves (including any immediate collisions), control passes to the other player.
- **Input mapping (initial suggestion)**:
  - Player 1: `W/A/S/D` for movement, `SPACE` fire, `LEFT CTRL` place mine.
  - Player 2: arrow keys for movement, `RIGHT CTRL` fire, `RIGHT SHIFT` place mine.
  - Actual key bindings are centralized in `constants.py` to allow later remapping.

### Movement Rules

- Tanks move in **discrete grid steps**:
  - Only cardinal directions allowed (no diagonals).
  - Target cell must be within board bounds and not a wall.
- Tanks **cannot occupy the same cell**:
  - Movement into enemy tank cell is treated as a collision (tank hit).
- Tanks **cannot pass through walls or out of bounds**:
  - Invalid moves are ignored and do not consume a turn.

### Combat Rules

- **Shots**:
  - Spawn from the cell directly in front of the firing tank (based on direction).
  - Travel in a straight line at a fixed speed in cells per update.
  - On each update step:
    - If next cell is wall → shot removed, explosion at impact cell.
    - If next cell contains enemy tank → enemy tank hit, shot removed, explosion at tank cell.
    - If next cell contains mine (any player) → mine and shot removed, explosion at mine cell.
  - Only one active shot per player at a time (for simplicity and PET feel).
  - Firing consumes 1 shot ammo; if no ammo remains, firing is ignored.

- **Mines**:
  - Placed on the active tank’s current cell (if cell is empty of other mines).
  - Placing consumes 1 mine ammo.
  - A tank entering a mine’s cell (enemy or self) triggers mine explosion:
    - The entering tank is hit.
    - Mine is removed.
  - A shot entering a mine’s cell triggers explosion and removes both shot and mine.

### Damage, Lives, and Respawn

- When a tank is **hit by a shot or mine**:
  - An explosion animation is played centered on the tank’s cell.
  - The tank loses 1 life.
  - If lives remain:
    - Tank is removed from the board for the duration of the explosion.
    - After explosion completes, tank respawns at its starting cell.
    - Respawn searches for the nearest valid cell around the canonical start if the exact start is blocked.
  - If no lives remain:
    - Tank is permanently removed.
    - If the other player still has ≥1 tank, that player wins the battle.

### Difficulty & Skill Levels

- At game start, players choose a **skill level 1–10**.
- Difficulty level affects:
  - **Shot pool (`SS`)**: total shots per player (baseline `5 × level`).
  - **Mine pool (`R`)**: total mines per player (baseline `level`, capped at a reasonable max).
  - Optional modern tweaks:
    - Explosion duration or animation speed.
    - Scoring value per destroyed tank / mine.
- The Python port exposes difficulty parameters in `constants.py` and computes per-level settings via helper functions used during game initialization.

### Victory, Battles, and Tournament Flow

- **Single battle**:
  - Both players begin with full lives, shots, and mines as configured by difficulty.
  - Battle continues until one player’s lives reach 0.
  - Winner is the surviving player.
  - Victory message depends on remaining lives:
    - 1 life left: “WITH HIS LAST TANK!”
    - 2 lives left: “WITH TWO TANKS LEFT!”
    - 3 lives (no deaths): “WITHOUT LOSING A TANK!”
- **Tournament**:
  - After a battle, players are asked if they want another battle.
  - Wins per player are accumulated across battles.
  - On tournament end, final summary shows:
    - Total number of battles.
    - Player with most battle wins as overall champion.
    - Unused tanks in final tally as in the PET game (approximate messaging, staying close to original flavor text).

### Status Display & UI Semantics

- Status band shows for each player:
  - Remaining tanks (lives).
  - Remaining shots.
  - Remaining mines.
- During normal play:
  - A subtle indicator shows whose turn it is (e.g., highlighting that player’s status block).
- Temporary message overlays:
  - “HIT A MINE” when a tank is destroyed by mine.
  - “PLAYER # WINS!” and one of the victory suffixes at end of battle.
  - “ANOTHER BATTLE?” prompt with yes/no control hints.

### Clarified Edge Cases

- **Invalid movement input**:
  - If a key is pressed but results in an invalid move (wall/out-of-bounds), no action is taken and the turn does **not** advance.
- **Firing with no ammo**:
  - Input is ignored, no shot is spawned, and the turn does **not** advance.
- **Placing mine on occupied cell**:
  - Mine cannot be placed if the target cell already contains a mine; input is ignored and the turn does **not** advance.
- **Simultaneous collisions**:
  - Order of resolution per update:
    1. Move active player’s tank (if action is movement) or spawn shot/mine.
    2. Update all shots.
    3. Resolve collisions in this order for each impacted cell: tank, mine, wall.
  - This deterministic ordering approximates the original BASIC behavior while remaining simple to reason about.

