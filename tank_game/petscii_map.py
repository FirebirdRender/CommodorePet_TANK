"""PETSCII character mapping via cbmcodecs2.

Provides ``PET_MAP`` — a dictionary of game-element names to Unicode strings
decoded from their PETSCII byte values using the ``petscii_c64en_uc`` codec
(uppercase/graphics character set).

Requires the ``cbmcodecs2`` package (registers the codec on import).
"""

from __future__ import annotations

import cbmcodecs2 as _cbmcodecs2  # noqa: F401 — side-effect: registers codecs

_CODEC = "petscii_c64en_uc"


def _pet(code: int) -> str:
    return bytes([code]).decode(_CODEC)


# ── Core visual elements (UI_RETRO_SPEC §3) ────────────────────────────────
PET_MAP: dict[str, str] = {
    # Border / HUD
    "BORDER":    _pet(0x51),   # Large ball / circle (Shifted Q)
    "SOLID":     _pet(0xA0),   # Solid block (Shifted Space) — HUD bar fill
    # Terrain
    "WALL":      _pet(0x66),   # Checkered / dithered block (Shifted F)
    # Tank composite
    "TANK_BODY": _pet(0x58),   # Square with internal cross (Shifted X)
    "BARREL_H":  _pet(0x40),   # Horizontal line (@ in PETSCII)
    "BARREL_V":  _pet(0x5D),   # Vertical line (Shifted ])
    # Objects
    "MINE":      _pet(0x71),   # Filled circle / mesh square (●)
    "SHOT":      _pet(0x51),   # Same ball glyph as border (small projectile)
    # Explosion radial spokes (PET graphic-mode line segments)
    "DIAG_NE":   _pet(0x4E),   # N — NE diagonal stroke
    "DIAG_NW":   _pet(0x4D),   # M — NW diagonal stroke
    "LINE_H":    _pet(0x40),   # Horizontal line (same as barrel)
    "LINE_V":    _pet(0x5D),   # Vertical line (same as barrel)
    # Wreckage
    "WRECKAGE":  _pet(0x56),   # V — cross / debris marker
}
