# 1. AUTHENTIC FONT SELECTION
===============================================================================
To match the look of the original game exactly, we need the PETSCII (Commodore
Business Machines) character set. Standard monospace fonts will not have the
specific graphic blocks (semigraphic characters) used for the tank barrels
and the circular borders.

# RECOMMENDED ASSETS:
* Primary Font: "C64 Pro Mono" or "Pet Me 64" (TTF format).
* Alternative: "CBM PET" by Style (KreativeKorp).
* Character Mapping: Ensure you use the "Unshifted" (Graphics) version. 
  The "Shifted" version replaces many graphics with lowercase letters.

# 2. RENDERING PARAMETERS (PYGAME)
===============================================================================
Modern rendering often smooths edges, which ruins the 1970s hardware look.
Apply these specific settings in your PyGame loop:

# NO ANTI-ALIASING:
When rendering text surfaces, always set the 'antialias' parameter to 'False'.
> text_surface = font.render(text, False, (0, 255, 0))

# COLOR PALETTE:
* Background: (0, 0, 0)
* Foreground: (51, 255, 51) -- This provides the classic "P1 Phosphor" green.

# 3. ASSET TABLE: CHARACTER MAPPING
===============================================================================
Screenshots show specific characters that are not found in the standard
ASCII range. Use these mapping strategies:

| Game Element | Character / Description | PETSCII Code (Hex) |
| :----------- | :---------------------- | :----------------- |
| Outer Border | Circle / "Ball"         | 0x51 (Shift + Q)   |
| Wall Blocks  | Checkered / Solid Block | 0x66 or 0xA0       |
| Tank Body    | Square with Cross       | 0x58 (X)           |
| Tank Barrel  | Horizontal Line         | 0x40 (@)           |
| Mine         | Dotted / Grid Square    | 0x71               |

Using the `cbmcodecs2` library (specifically the `petscii_c64en_uc` codec), 
we can map the specific visual elements from your screenshots to their 
hexadecimal and Python-string equivalents. 

# 3A. THE USER INTERFACE (HUD)
===============================================================================
Based on "pet-screenshot-1.jpg", the top header is composed of a mix of 
alphanumeric text and block graphic separators.

* **Text Labels:** "TANKS", "SHOTS", "MINES"
  * *Method:* Standard Python strings encoded via the codec.
* **Header Background:** The solid green bar behind the labels.
  * *Character:* `0xA0` (Shifted Space / Solid Block).
  * *Python:* `b'\xa0'.decode('petscii_c64en_uc')`
* **Separator Block:** The 2x2 grid of circles between the two player HUDs.
  * *Character:* `0x51` (Shifted Q / Large Ball).
  * *Python:* `b'\x51'.decode('petscii_c64en_uc')`

# 3B. THE PLAYFIELD BORDER
===============================================================================
The iconic "dotted" or "bubble" border seen in both screenshots.

* **Character:** `0x51` (The same "Large Ball" used in the HUD separator).
  * *Visual:* A centered circular glyph. 
  * *Usage:* Repeat this character for the entire X/Y perimeter of the 
    playfield area.

# 3C. GAME OBJECTS
===============================================================================
These are the most critical for gameplay logic and visual fidelity.

# THE TANKS:
The tanks in your screenshots are composite objects (built from two chars).
* **Body:** `0x58` (Shifted X / Square with internal cross).
* **Barrel:** * Horizontal: `0x40` (The "@" character in PETSCII is a horizontal line).
  * Vertical: `0x5D` (Shifted ] / Vertical line).

# THE WALLS (OBSTRUCTIONS):
The "staircase" or "checkered" patterns in the middle.
* **Character:** `0x66` (Shifted F / Checkered pattern). 
  * *Note:* This is distinct from the solid block `0xA0`. It provides that 
    vintage texture seen in "pet-screenshot-2.jpg".

# THE MINES:
* **Character:** `0x71` (Shifted Q in lowercase/alternate set, or Row 10 
  graphics). It appears as a "grid" or "mesh" square.

# 3D. SUMMARY MAPPING TABLE
===============================================================================

| Visual Element   | PETSCII Hex | Python string equivalent (UC Codec) |
| :--------------- | :---------- | :---------------------------------- |
| Border Circle    | `0x51`      | `"Q"` (When Shifted)                |
| Solid HUD Bar    | `0xA0`      | `"\xa0"`                            |
| Checkered Wall   | `0x66`      | `"f"` (When Shifted)                |
| Tank Body        | `0x58`      | `"X"`                               |
| Tank Barrel (H)  | `0x40`      | `"@"`                               |
| Mine             | `0x71`      | `"q"` (When Shifted)                |

# 3E. IMPLEMENTATION NOTE
===============================================================================
When using `cbmcodecs2`, you can create a "Character Map" dictionary to make 
your PyGame draw calls more readable:

```python
PET_MAP = {
    'BORDER': b'\x51'.decode('petscii_c64en_uc'),
    'WALL':   b'\x66'.decode('petscii_c64en_uc'),
    'TANK':   b'\x58'.decode('petscii_c64en_uc'),
    'BARREL': b'\x40'.decode('petscii_c64en_uc'),
    'MINE':   b'\x71'.decode('petscii_c64en_uc'),
}
```

# 4. OPTIONAL POST-PROCESSING (CRT EFFECT)
===============================================================================
To get the "fuzz" seen in your second screenshot:

# SCANLINES:
Create a surface with horizontal black lines (Alpha = 60) and blit it over
the final frame.

# BLOOM EFFECT:
Render the game to a smaller surface, then scale it up to the window size 
using `pygame.transform.smoothscale`. This creates a slight "analog" blur 
characteristic of old monitors.
