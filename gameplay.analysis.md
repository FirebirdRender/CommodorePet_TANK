# Technical Specification: Tank! (Commodore PET)

A comprehensive guide for recreating the 1981 "The Code Works" classic in a modern environment.

---

## 1. Core Display Logic
* **Resolution:** 40 columns x 25 rows (Character-based grid).
* **Aspect Ratio:** Standard 4:3.
* **Color Palette:** Monochrome (Green on Black or White on Black).
* **Characters (PETSCII):**
    * **Tanks:** Often a 2x2 block or a single `X` or `*`.
    * **Walls:** Solid block characters (`█`).
    * **Projectiles:** Mid-dot `·` or `+`.
    * **Mines:** `¤` (visible briefly, then turns into the background character/blank).

## 2. Game Mechanics

### A. Movement
* **Grid-Based:** Movement occurs character-by-character.
* **8-Way Directional:** Up, Down, Left, Right, and all four diagonals.
* **Collision:** Tanks cannot pass through walls or each other.

### B. Combat & Ammo
* **Shot Count:** Each life grants exactly **6 shots**.
* **The "Empty Gun" Rule:** If a player fires all 6 shots and fails to destroy the enemy, the player's own tank is automatically destroyed.
* **Destructible Terrain:** Projectiles destroy wall characters upon impact.
* **Projectiles:** Only one active projectile per player on screen at a time.

### C. Mines
* **Capacity:** 1 mine per life.
* **Stealth Mechanic:** When dropped, the mine is visible for ~1 second (or until the player moves 2 tiles away), then it becomes invisible.
* **Trigger:** Instant destruction if any player (including the one who placed it) moves onto the occupied tile.

## 3. Difficulty Levels
The game features 10 difficulty levels (1–10) which scale the following variables:
1.  **Wall Density:** Higher levels populate more of the grid with obstacles.
2.  **Ammo Scarcity:** Lower shot counts at extreme levels.
3.  **Speed:** Faster refresh rates/input polling.

## 4. Input Mapping (Modern Translation)

| Action       | Player 1 (Left) | Player 2 (Right) |
| :----------- | :-------------- | :--------------- |
| **Move** | W, A, S, D      | Arrow Keys       |
| **Diagonals**| (e.g., W+A)     | (e.g., Up+Left)  |
| **Fire** | Left Shift      | Right Shift / 0  |
| **Lay Mine** | Q               | / or .           |

## 5. Game Loop Logic (Pseudocode)

```python
# Initial State
player1_pos = [2, 12]
player2_pos = [37, 12]
p1_ammo = 6
p2_ammo = 6
active_mines = [] # Store as {pos: [x,y], visible: bool, owner: 1}

def update_loop():
    # 1. Handle Input
    # 2. Update Projectile Physics (straight line, check collision)
    # 3. Check Collision (Wall, Mine, or Enemy)
    # 4. Update Mine Visibility
    # 5. Check "No Ammo" Defeat Condition
    # 6. Render Grid
