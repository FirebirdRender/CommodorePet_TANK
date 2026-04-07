"""Headless Demo: monotonic ``pygame.time.get_ticks`` so AI reaction gating is meaningful."""

from __future__ import annotations

import os

import pygame
import pytest
from tank_game.game import GameController, GameState


@pytest.mark.parametrize("step_ms", [50, 100])
def test_headless_demo_advancing_ticks_allows_ai_commits(
    monkeypatch: pytest.MonkeyPatch, step_ms: int
) -> None:
    """Naive tight loops barely advance ``get_ticks()``; fake ms progress proves AI can decide repeatedly."""
    os.environ.setdefault("SDL_VIDEODRIVER", "dummy")
    pygame.init()
    try:
        pygame.display.set_mode((320, 200))
        ticks_ms = [0]

        def fake_get_ticks() -> int:
            ticks_ms[0] += step_ms
            return ticks_ms[0]

        monkeypatch.setattr(pygame.time, "get_ticks", fake_get_ticks)
        gc = GameController()
        gc.game_mode = "Demo"
        gc.init_round()
        assert gc.state == GameState.PLAYING
        commits = 0
        prev = gc._ai[1]._last_action_time
        for _ in range(400):
            gc.update(1.0 / 60.0)
            cur = gc._ai[1]._last_action_time
            if cur != prev:
                commits += 1
                prev = cur
        assert commits >= 3
    finally:
        pygame.quit()
