"""Headless pygame sanity checks (see docs/PYGAME_QA_TESTING_SPEC.md)."""

from __future__ import annotations


def test_pygame_init_headless() -> None:
    import pygame

    pygame.init()
    assert pygame.get_init()
    pygame.quit()
    assert not pygame.get_init()


def test_tick_logic_accepts_fixed_dt() -> None:
    """Logic layer runs without display; dt is injectable for deterministic tests."""
    import pygame
    from tank_game.game import GameController
    from tank_game.game_loop import tick_logic

    pygame.init()
    try:
        pygame.display.set_mode((320, 200))
        controller = GameController()
        events: list[pygame.event.Event] = []
        assert tick_logic(controller, events, dt=1.0 / 60.0)
    finally:
        pygame.quit()
