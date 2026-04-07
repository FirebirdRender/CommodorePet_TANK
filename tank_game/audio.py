from __future__ import annotations

import pygame


class SoundManager:
    def __init__(self) -> None:
        pygame.mixer.init()
        # Placeholder: in a real project these would be actual files
        self.explosion = None
        self.fire = None
        self.victory = None

    def play_explosion(self) -> None:
        if self.explosion:
            self.explosion.play()

    def play_fire(self) -> None:
        if self.fire:
            self.fire.play()

    def play_victory(self) -> None:
        if self.victory:
            self.victory.play()
