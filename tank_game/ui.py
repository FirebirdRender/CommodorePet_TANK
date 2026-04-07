"""HUD and overlay rendering (PETSCII retro style).

``StatusDisplay`` renders the PET-authentic two-row HUD: solid green bar
background (``0xA0``) with ``TANKS / SHOTS / MINES`` labels+values and circle
separators (``0x51``) in the center.

``MessageOverlay`` draws centered multi-line text for menus / screens.
"""

from __future__ import annotations

import pygame

from .constants import (
    CELL_SIZE,
    COLOR_PET_FG,
    COLOR_STATUS_BG,
    COLOR_TEXT,
    SCREEN_WIDTH_CELLS,
    STATUS_BAR_HEIGHT,
    WINDOW_WIDTH,
)
from .petscii_map import PET_MAP
from .petscii_render import blit_glyph, get_pet_font, glyph_surface
from .player import Tank

# HUD occupies 2 character rows (each CELL_SIZE px high).
_HUD_ROWS = STATUS_BAR_HEIGHT // CELL_SIZE  # 2

# Column layout (40 columns total):
#   P1 panel: cols 0..17  (18 chars)
#   Center:   cols 18..21 (4 chars — circle separator)
#   P2 panel: cols 22..39 (18 chars)
_CENTER_START = 18
_CENTER_END = 22  # exclusive
_P2_START = _CENTER_END


def _hud_text_row0(tank: Tank, ai_diff: int | None) -> str:
    """Row 0: ``TANKS  SHOTS  MINES``  (or + ``AI:n`` suffix)."""
    s = "TANKS  SHOTS  MINES"
    if ai_diff is not None:
        # Trim to fit 18 cols if needed
        s = f"TANKS SHOTS MINES {ai_diff}"
    return s[:18].ljust(18)


def _hud_text_row1(tank: Tank) -> str:
    """Row 1: numeric values aligned under labels."""
    return f"  {tank.lives}      {tank.shots_left}      {tank.mines_left}".ljust(18)[:18]


class StatusDisplay:
    def __init__(self, font: pygame.font.Font) -> None:
        self.font = font

    def draw(
        self,
        surface: pygame.Surface,
        player1: Tank,
        player2: Tank,
        current_player_id: int | None = None,
        ai_difficulty: dict[int, int] | None = None,
    ) -> None:
        solid_ch = PET_MAP["SOLID"]
        sep_ch = PET_MAP["BORDER"]
        fg = COLOR_PET_FG

        # 1. Fill entire HUD area with solid blocks (green bar)
        for row in range(_HUD_ROWS):
            py = row * CELL_SIZE
            for col in range(SCREEN_WIDTH_CELLS):
                blit_glyph(surface, solid_ch, col * CELL_SIZE, py, fg, CELL_SIZE)

        # 2. Center separator — circles
        for row in range(_HUD_ROWS):
            py = row * CELL_SIZE
            for col in range(_CENTER_START, _CENTER_END):
                blit_glyph(surface, sep_ch, col * CELL_SIZE, py, fg, CELL_SIZE)

        # 3. Player text panels (rendered as characters "punched" onto the bar)
        ai1 = ai_difficulty.get(1) if ai_difficulty else None
        ai2 = ai_difficulty.get(2) if ai_difficulty else None

        p1_row0 = _hud_text_row0(player1, ai1)
        p1_row1 = _hud_text_row1(player1)
        p2_row0 = _hud_text_row0(player2, ai2)
        p2_row1 = _hud_text_row1(player2)

        self._draw_panel_text(surface, 0, p1_row0, p1_row1)
        self._draw_panel_text(surface, _P2_START, p2_row0, p2_row1)

    def _draw_panel_text(
        self,
        surface: pygame.Surface,
        start_col: int,
        row0: str,
        row1: str,
    ) -> None:
        """Draw two rows of HUD text starting at *start_col*.

        Each character cell is first cleared to black (``COLOR_STATUS_BG``)
        then the character glyph is blitted on top — matching the PET's
        normal-mode character-on-dark-background within the solid-block bar.
        """
        font = get_pet_font(CELL_SIZE)
        for i, ch in enumerate(row0):
            if ch == " ":
                continue
            px = (start_col + i) * CELL_SIZE
            pygame.draw.rect(surface, COLOR_STATUS_BG, (px, 0, CELL_SIZE, CELL_SIZE))
            blit_glyph(surface, ch, px, 0, COLOR_PET_FG, CELL_SIZE)
        for i, ch in enumerate(row1):
            if ch == " ":
                continue
            px = (start_col + i) * CELL_SIZE
            py = CELL_SIZE
            pygame.draw.rect(surface, COLOR_STATUS_BG, (px, py, CELL_SIZE, CELL_SIZE))
            blit_glyph(surface, ch, px, py, COLOR_PET_FG, CELL_SIZE)


class MessageOverlay:
    def __init__(self, font: pygame.font.Font) -> None:
        self.font = font
        self.message: str | None = None
        self._cache_key: tuple[str, int, int] | None = None
        self._cached_blits: list[tuple[pygame.Surface, tuple[int, int]]] = []

    def set_message(self, text: str) -> None:
        if text == self.message:
            return
        self.message = text
        self._cache_key = None

    def clear(self) -> None:
        self.message = None
        self._cache_key = None
        self._cached_blits = []

    def draw(self, surface: pygame.Surface) -> None:
        if not self.message:
            return
        w, h = surface.get_size()
        key = (self.message, w, h)
        if self._cache_key != key:
            lines = self.message.split("\n")
            y = (h - len(lines) * self.font.get_height()) // 2
            self._cached_blits = []
            for line in lines:
                text_surf = self.font.render(line, False, COLOR_TEXT)
                x = (w - text_surf.get_width()) // 2
                self._cached_blits.append((text_surf, (x, y)))
                y += self.font.get_height()
            self._cache_key = key
        for surf, pos in self._cached_blits:
            surface.blit(surf, pos)
