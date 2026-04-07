"""PETSCII character mapping via cbmcodecs2.

Provides ``PET_MAP`` — a dictionary of game-element names to Unicode strings
decoded from their PETSCII byte values using the ``petscii_c64en_uc`` codec
(uppercase/graphics character set).

**Important:** The PET keyboard's *shifted* letter keys produce the graphic
symbols (ball, checkered block, line segments, etc.).  In PETSCII encoding
these live at ``letter_code + 0x80`` (range 0xC0–0xDA), *not* at the bare
letter positions (0x41–0x5A) which give plain uppercase letters.  The 0x60–0x7F
range also contains graphic characters (duplicates of the shifted set).

Requires the ``cbmcodecs2`` package (registers the codec on import).
"""

from __future__ import annotations

import cbmcodecs2 as _cbmcodecs2  # noqa: F401 — side-effect: registers codecs

_CODEC = "petscii_c64en_uc"


def _pet(code: int) -> str:
    return bytes([code]).decode(_CODEC)


# ── Core visual elements (UI_RETRO_SPEC §3) ────────────────────────────────
# Codes use the SHIFTED range (0xC0+) for graphic symbols, or the 0x60-0x7F
# mirror range.  Plain 0x41-0x5A would give uppercase letters, not graphics.
PET_MAP: dict[str, str] = {
    # Border / HUD — rendered INVERTED (green fill, black glyph)
    "BORDER":    _pet(0xD1),   # Filled circle (Shift+Q) — U+25CF; inverted = green cell, black hole
    "SOLID":     _pet(0xA0),   # Solid block (Shifted Space) — HUD bar fill
    # Terrain
    "WALL":      "\u259A",     # Quadrant upper-left + lower-right — diagonal checkerboard
    # Tank composite — body rendered INVERTED (green fill, black glyph hole)
    "TANK_BODY_P1": "*",       # Asterisk; inverted = green cell, black star shape
    "TANK_BODY_P2": "#",       # Hash; inverted = green cell, black grid pattern
    "BARREL_H":  _pet(0xC0),   # Horizontal line (Shift+@) — U+2500
    "BARREL_V":  _pet(0xDD),   # Vertical line (Shift+]) — U+2502
    # Objects
    "MINE":      _pet(0x71),   # Filled circle — U+25CF
    "SHOT":      ".",          # Period — small dot projectile
    # Explosion radial spokes (PET graphic-mode line segments)
    "DIAG_NE":   _pet(0xCE),   # / diagonal stroke (Shift+N) — U+2571
    "DIAG_NW":   _pet(0xCD),   # \ diagonal stroke (Shift+M) — U+2572
    "LINE_H":    _pet(0xC0),   # Horizontal line (same as barrel) — U+2500
    "LINE_V":    _pet(0xDD),   # Vertical line (same as barrel) — U+2502
    # Wreckage
    "WRECKAGE":  _pet(0xD6),   # X / diagonal cross (Shift+V) — U+2573
    # Curled barrel wreckage — arc / hook chars for each curl direction
    "CURL_UP":    _pet(0xCD),  # \ stroke — barrel curled upward
    "CURL_DOWN":  _pet(0xCE),  # / stroke — barrel curled downward
    "CURL_LEFT":  _pet(0xCD),  # \ stroke — barrel curled left
    "CURL_RIGHT": _pet(0xCE),  # / stroke — barrel curled right
}
