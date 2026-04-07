"""HUD and overlay rendering (PETSCII retro style).

``StatusDisplay`` renders the PET-authentic two-row HUD: solid green bar
background (``0xA0``) with ``TANKS / SHOTS / MINES`` labels+values and circle
separators (``0x51``) in the center.

``MessageOverlay`` draws centered multi-line text for menus / screens.
"""

from __future__ import annotations

import pygame

from .constants import (
    BOARD_OFFSET_Y,
    CELL_SIZE,
    COLOR_BG,
    COLOR_PET_FG,
    COLOR_TEXT,
    SCREEN_WIDTH_CELLS,
    STATUS_BAR_HEIGHT,
    WINDOW_WIDTH,
    difficulty_to_resources,
)
from .petscii_map import PET_MAP
from .petscii_render import blit_glyph, blit_inverted, get_pet_font, glyph_surface
from .player import Tank

# HUD occupies 2 character rows (each CELL_SIZE px high).
_HUD_ROWS = STATUS_BAR_HEIGHT // CELL_SIZE  # 2

# Column layout (40 columns total, matching PET):
#   P1 panel: cols 0..18  (19 chars)
#   Center:   cols 19..20 (2 chars — circle separator)
#   P2 panel: cols 21..39 (19 chars)
_CENTER_START = 19
_CENTER_END = 21  # exclusive
_P2_START = _CENTER_END


def _hud_text_row0(tank: Tank, ai_diff: int | None) -> str:
    """Row 0: ``TANKS  SHOTS  MINES``  (or + ``AI:n`` suffix)."""
    s = "TANKS  SHOTS  MINES"
    if ai_diff is not None:
        s = f"TANKS SHOTS MINES {ai_diff}"
    return s[:19].ljust(19)


def _hud_text_row1(tank: Tank) -> str:
    """Row 1: numeric values aligned under labels."""
    return f"  {tank.lives}      {tank.shots_left}      {tank.mines_left}".ljust(19)[:19]


def _player_status_message(tank: Tank, max_shots: int, is_winner: bool) -> str:
    """Per-player status message for the border message bar."""
    if is_winner:
        return "THE WINNER"
    if tank.shots_left == 0:
        return "OUT OF SHOTS"
    if max_shots > 0 and tank.shots_left <= max_shots * 0.2:
        return "LOW SHOTS"
    if tank.lives == 1:
        return "LAST TANK"
    return ""


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
        winner: int | None = None,
        difficulty: int = 5,
    ) -> None:
        solid_ch = PET_MAP["SOLID"]
        sep_ch = PET_MAP["BORDER"]
        fg = COLOR_PET_FG

        # 1. Fill entire HUD area with solid blocks (green bar)
        for row in range(_HUD_ROWS):
            py = row * CELL_SIZE
            for col in range(SCREEN_WIDTH_CELLS):
                blit_glyph(surface, solid_ch, col * CELL_SIZE, py, fg, CELL_SIZE)

        # 2. Center separator — inverted circles (dark circles on green bar)
        for row in range(_HUD_ROWS):
            py = row * CELL_SIZE
            for col in range(_CENTER_START, _CENTER_END):
                blit_inverted(surface, sep_ch, col * CELL_SIZE, py, fg, COLOR_BG, CELL_SIZE)

        # 3. Player text panels (rendered as characters "punched" onto the bar)
        ai1 = ai_difficulty.get(1) if ai_difficulty else None
        ai2 = ai_difficulty.get(2) if ai_difficulty else None

        p1_row0 = _hud_text_row0(player1, ai1)
        p1_row1 = _hud_text_row1(player1)
        p2_row0 = _hud_text_row0(player2, ai2)
        p2_row1 = _hud_text_row1(player2)

        self._draw_panel_text(surface, 0, p1_row0, p1_row1)
        self._draw_panel_text(surface, _P2_START, p2_row0, p2_row1)

        # 4. Per-player status messages in the top border row
        _, max_shots, _ = difficulty_to_resources(difficulty)
        p1_msg = _player_status_message(player1, max_shots, winner == 1)
        p2_msg = _player_status_message(player2, max_shots, winner == 2)
        border_y = BOARD_OFFSET_Y
        if p1_msg:
            self._draw_border_message(surface, p1_msg, 1, border_y)
        if p2_msg:
            self._draw_border_message(surface, p2_msg, _CENTER_END, border_y)

    def _draw_panel_text(
        self,
        surface: pygame.Surface,
        start_col: int,
        row0: str,
        row1: str,
    ) -> None:
        """Draw two rows of inverted HUD text starting at *start_col*."""
        for i, ch in enumerate(row0):
            if ch == " ":
                continue
            px = (start_col + i) * CELL_SIZE
            blit_inverted(surface, ch, px, 0, COLOR_PET_FG, COLOR_BG, CELL_SIZE)
        for i, ch in enumerate(row1):
            if ch == " ":
                continue
            px = (start_col + i) * CELL_SIZE
            py = CELL_SIZE
            blit_inverted(surface, ch, px, py, COLOR_PET_FG, COLOR_BG, CELL_SIZE)

    def _draw_border_message(
        self,
        surface: pygame.Surface,
        msg: str,
        start_col: int,
        border_y: int,
    ) -> None:
        """Render a status message into the top border row (inverted text on green)."""
        panel_width = _CENTER_START - 1  # columns available per player side
        msg = msg[:panel_width].center(panel_width)
        for i, ch in enumerate(msg):
            px = (start_col + i) * CELL_SIZE
            if ch == " ":
                continue
            blit_inverted(surface, ch, px, border_y, COLOR_PET_FG, COLOR_BG, CELL_SIZE)


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
