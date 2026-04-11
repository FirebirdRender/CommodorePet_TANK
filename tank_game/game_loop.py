"""Decoupled frame step: game logic vs rendering.

Logic (`tick_logic`) is pure of draw calls so tests can drive the controller without rendering.
Rendering (`render_frame`) isolates pygame drawing for manual QA / profiling.
"""

from __future__ import annotations

import pygame

from .game import GameController, GameState
from .graphics import (
    draw_board,
    draw_explosions,
    draw_mines,
    draw_shots,
    draw_tanks,
    draw_wreckage,
)
from .ui import MessageOverlay, StatusDisplay


def tick_logic(
    controller: GameController,
    events: list[pygame.event.Event],
    dt: float,
) -> bool:
    """Advance simulation by one frame. Returns False if the app should exit.

    No drawing; safe to call in tests with injected events and fixed dt.
    """
    for event in events:
        if event.type == pygame.QUIT:
            return False

    controller.handle_input(events)
    controller.update(dt)

    if controller.state == GameState.QUIT:
        return False
    return True


_MENU_STATES = frozenset({
    GameState.MENU,
    GameState.SKILL_SELECT,
    GameState.PLAY_AGAIN,
})

_TITLE_TEXT = (
    "TANK!   BY SHAWN MEEHAN\n"
    "             & MIKE ROWLEY\n"
    "\n"
    "    CURSOR #26  COPYRIGHT (C) 1981\n"
    "    ________________________________\n"
    "\n"
    "PATTON VS.THE DESERT FOX\n"
    "\n"
    "\n"
    "PRESS RETURN TO BEGIN"
)


def _render_menu_text(
    screen: pygame.Surface,
    controller: GameController,
    overlay: MessageOverlay,
) -> None:
    """Render menu/selection text on a clean black screen (no playfield)."""
    state = controller.state

    if state == GameState.MENU:
        overlay.set_message(_TITLE_TEXT)
    elif state == GameState.SKILL_SELECT:
        overlay.set_message(
            f"WHAT SKILL LEVEL(1-10)? {controller.difficulty}\n"
            "10 IS THE HARDEST"
        )
    elif state == GameState.PLAY_AGAIN:
        victory_msg = controller.get_victory_message()
        overlay.set_message(victory_msg + "\n\n\nANOTHER BATTLE?")

    overlay.draw(screen)


def render_frame(
    screen: pygame.Surface,
    controller: GameController,
    status: StatusDisplay,
    overlay: MessageOverlay,
) -> None:
    """All pygame draw operations for one frame (no display.flip)."""
    # Menu/selection screens: clean black background with text only (no playfield).
    if controller.state in _MENU_STATES:
        screen.fill((0, 0, 0))
        _render_menu_text(screen, controller, overlay)
        return

    draw_board(screen, controller.board)
    draw_wreckage(screen, controller.board, controller._barrel_wreckage_registry,
                  controller._barrel_hit_bodies)
    draw_mines(screen, controller.mines)
    draw_shots(screen, controller.shots)
    draw_tanks(screen, controller.tanks.values())
    draw_explosions(screen, controller.explosions)

    if controller.tanks:
        status.draw(
            screen,
            controller.tanks[1],
            controller.tanks[2],
            ai_difficulty=None,
            winner=controller.winner,
            difficulty=controller.difficulty,
        )

    overlay.clear()
