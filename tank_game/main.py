from __future__ import annotations

import os
import sys
from datetime import datetime

import pygame

from .constants import (
    CELL_SIZE,
    MAIN_LOOP_FPS,
    WINDOW_HEIGHT,
    WINDOW_WIDTH,
)
from .crt_effect import apply_crt
from .game import GameController
from .game_loop import render_frame, tick_logic
from .petscii_render import get_pet_font
from .ui import MessageOverlay, StatusDisplay

DEBUG_LOG_FILE = "tank_debug.log"
_DEBUG = os.environ.get("TANK_DEBUG", "").strip().lower() not in ("", "0", "false", "no", "off")


def debug_log(msg: str) -> None:
    """Write debug message to log file."""
    if not _DEBUG:
        return
    timestamp = datetime.now().strftime("%H:%M:%S.%f")[:-3]
    with open(DEBUG_LOG_FILE, "a") as f:
        f.write(f"[{timestamp}] {msg}\n")


def main() -> None:
    if _DEBUG:
        open(DEBUG_LOG_FILE, "w").close()
        debug_log("=== TANK DEBUG SESSION STARTED ===")

    pygame.init()
    # Vsync on ``flip()`` can cap effective FPS (~display Hz) regardless of ``Clock.tick``;
    # set ``TANK_VSYNC=0`` to disable.
    _vsync_on = os.environ.get("TANK_VSYNC", "1").strip().lower() not in (
        "0",
        "false",
        "no",
        "off",
    )
    screen = pygame.display.set_mode(
        (WINDOW_WIDTH, WINDOW_HEIGHT),
        vsync=1 if _vsync_on else 0,
    )
    pygame.display.set_caption("TANK! 2P — PyGame Port")
    clock = pygame.time.Clock()

    controller = GameController()
    font = get_pet_font(CELL_SIZE)
    status = StatusDisplay(font)
    overlay = MessageOverlay(font)

    debug_log(f"Game initialized - difficulty: {controller.difficulty}")

    running = True
    while running:
        dt = clock.tick(MAIN_LOOP_FPS) / 1000.0
        events = pygame.event.get()
        running = tick_logic(controller, events, dt)
        render_frame(screen, controller, status, overlay)
        apply_crt(screen)
        pygame.display.flip()

    pygame.quit()
    sys.exit(0)


if __name__ == "__main__":
    main()
