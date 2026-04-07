from __future__ import annotations

import pygame

from .constants import COLOR_STATUS_BG, COLOR_TEXT, WINDOW_WIDTH
from .player import Tank


class StatusDisplay:
    def __init__(self, font: pygame.font.Font) -> None:
        self.font = font

    def draw(
        self,
        surface: pygame.Surface,
        player1: Tank,
        player2: Tank,
        current_player_id: int | None = None,
        ai_difficulty: dict[int, int] | None = None,  # player_id -> difficulty
    ) -> None:
        height = 40
        rect = pygame.Rect(0, 0, WINDOW_WIDTH, height)
        pygame.draw.rect(surface, COLOR_STATUS_BG, rect)

        # Player 1 status
        p1_text = f"P1 T:{player1.lives} S:{player1.shots_left} M:{player1.mines_left}"
        if ai_difficulty and 1 in ai_difficulty:
            p1_text += f" AI:{ai_difficulty[1]}"
        p1_surf = self.font.render(p1_text, True, COLOR_TEXT)

        # Player 2 status
        p2_text = f"P2 T:{player2.lives} S:{player2.shots_left} M:{player2.mines_left}"
        if ai_difficulty and 2 in ai_difficulty:
            p2_text += f" AI:{ai_difficulty[2]}"
        p2_surf = self.font.render(p2_text, True, COLOR_TEXT)

        surface.blit(p1_surf, (10, 10))
        surface.blit(p2_surf, (WINDOW_WIDTH - p2_surf.get_width() - 10, 10))

        if current_player_id is not None:
            turn_text = f"TURN: P{current_player_id}"
            turn_surf = self.font.render(turn_text, True, COLOR_TEXT)
            surface.blit(
                turn_surf,
                ((WINDOW_WIDTH - turn_surf.get_width()) // 2, 10),
            )


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
                text_surf = self.font.render(line, True, COLOR_TEXT)
                x = (w - text_surf.get_width()) // 2
                self._cached_blits.append((text_surf, (x, y)))
                y += self.font.get_height()
            self._cache_key = key
        for surf, pos in self._cached_blits:
            surface.blit(surf, pos)
