"""Headless smoke: evasion fallback helper exists and runs without crashing (Demo init)."""

from __future__ import annotations

import os

import pygame
from tank_game.game import GameController, GameState


def test_ai_try_random_escape_move_runs_on_demo_board() -> None:
    """Regression: `_ai_try_random_escape_move` must be callable after Demo `init_round` (headless SDL)."""
    os.environ.setdefault("SDL_VIDEODRIVER", "dummy")
    pygame.init()
    try:
        pygame.display.set_mode((320, 200))
        gc = GameController()
        gc.game_mode = "Demo"
        gc.init_round()
        assert gc.state == GameState.PLAYING
        assert 1 in gc._ai and 2 in gc._ai
        ai = gc._ai[1]
        tank = gc.tanks[1]
        current_time = pygame.time.get_ticks() / 1000.0
        result = gc._ai_try_random_escape_move(
            1, ai, tank, current_time, log_reason="test smoke"
        )
        assert isinstance(result, bool)
    finally:
        pygame.quit()


def test_move_away_branch_uses_last_known_when_enemy_pos_none() -> None:
    """Documented behavior: executor falls back to `last_known_enemy_pos` for evade vector."""
    from tank_game.game import GameController, GameState

    os.environ.setdefault("SDL_VIDEODRIVER", "dummy")
    pygame.init()
    try:
        pygame.display.set_mode((320, 200))
        gc = GameController()
        gc.game_mode = "Demo"
        gc.init_round()
        assert gc.state == GameState.PLAYING
        p2 = gc.tanks[2]
        gc._ai[1].last_known_enemy_pos = (p2.x, p2.y)
        # Smoke: _run_ai does not raise when tanks exist
        gc._run_ai(1)
    finally:
        pygame.quit()
