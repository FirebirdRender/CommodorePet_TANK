"""PETSCII glyph cache and tile-blit helpers.

All game visuals that map to a PETSCII character are rendered **once** into a
:class:`pygame.Surface` (per codepoint × color × size) and then blitted from
cache.  This avoids per-frame ``font.render`` calls while keeping the authentic
character-cell look.
"""

from __future__ import annotations

import os
from functools import lru_cache
from pathlib import Path

import pygame

# ── Font path resolution ────────────────────────────────────────────────────
_BUILTIN_FONT = Path(__file__).parent / "assets" / "fonts" / "PetMe64.ttf"


def _resolve_font_path() -> str:
    env = os.environ.get("TANK_PET_FONT", "").strip()
    if env and os.path.isfile(env):
        return env
    if _BUILTIN_FONT.is_file():
        return str(_BUILTIN_FONT)
    return ""


_font_cache: dict[int, pygame.font.Font] = {}


def get_pet_font(size: int) -> pygame.font.Font:
    """Return PetMe64 font at *size* px (cached).  Falls back to system mono."""
    if size not in _font_cache:
        path = _resolve_font_path()
        if path:
            _font_cache[size] = pygame.font.Font(path, size)
        else:
            _font_cache[size] = pygame.font.SysFont("consolas", size)
    return _font_cache[size]


# ── Glyph surface cache ────────────────────────────────────────────────────

@lru_cache(maxsize=512)
def glyph_surface(
    char: str,
    color: tuple[int, int, int],
    size: int,
) -> pygame.Surface:
    """Pre-rendered glyph surface (cached by char + color + size)."""
    font = get_pet_font(size)
    return font.render(char, False, color)


def blit_glyph(
    surface: pygame.Surface,
    char: str,
    px: int,
    py: int,
    color: tuple[int, int, int],
    size: int,
) -> None:
    """Blit a cached glyph at pixel position *(px, py)*."""
    surface.blit(glyph_surface(char, color, size), (px, py))


def blit_cell(
    surface: pygame.Surface,
    char: str,
    cx: int,
    cy: int,
    color: tuple[int, int, int],
    cell_size: int,
    offset_y: int = 0,
) -> None:
    """Blit a glyph into board cell *(cx, cy)* with optional vertical offset."""
    blit_glyph(surface, char, cx * cell_size, offset_y + cy * cell_size, color, cell_size)
