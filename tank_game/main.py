from __future__ import annotations

import os
import sys
from datetime import datetime


def _require_runtime_deps() -> None:
    """Fail fast with install instructions if dependencies from pyproject.toml are missing."""
    try:
        import py_trees  # noqa: F401
    except ModuleNotFoundError:
        sys.stderr.write(
            "Error: py_trees is not installed. The game lists it in pyproject.toml.\n"
            "From the project root, run:\n"
            "  pip install -e .\n"
            "For dev tools too:\n"
            "  pip install -e \".[dev]\"\n"
        )
        raise SystemExit(1)


_require_runtime_deps()

import pygame

from .constants import (
    HEADLESS_SHOWDOWN_MAIN_LOOP_FPS,
    MAIN_LOOP_FPS,
    WINDOW_HEIGHT,
    WINDOW_WIDTH,
)
from .constants import CELL_SIZE
from .game import GameController, GameState
from .game_loop import render_frame, tick_logic
from .crt_effect import apply_crt
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
    # set ``TANK_VSYNC=0`` to disable for benchmarking headless SHOWDOWN throughput.
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
    pygame.display.set_caption("TANK! PyGame Port")
    clock = pygame.time.Clock()

    controller = GameController()
    font = get_pet_font(CELL_SIZE)
    status = StatusDisplay(font)
    overlay = MessageOverlay(font)

    debug_log(f"Game initialized - difficulty: {controller.difficulty}")

    running = True
    headless_render_skip = 0
    HEADLESS_RENDER_INTERVAL = 4
    while running:
        headless_showdown_running = (
            getattr(controller, "_showdown_headless", False)
            and controller.state == GameState.SHOWDOWN_RUNNING
        )
        parallel_running = (
            headless_showdown_running
            and getattr(controller, "_showdown_parallel_thread", None) is not None
        )
        if headless_showdown_running and not parallel_running:
            pygame.event.pump()
        target_fps = (
            HEADLESS_SHOWDOWN_MAIN_LOOP_FPS
            if headless_showdown_running and not parallel_running
            else MAIN_LOOP_FPS
        )
        dt = clock.tick(target_fps) / 1000.0
        events = pygame.event.get()
        running = tick_logic(controller, events, dt)
        if headless_showdown_running and not parallel_running:
            headless_render_skip += 1
            if headless_render_skip >= HEADLESS_RENDER_INTERVAL:
                headless_render_skip = 0
                render_frame(screen, controller, status, overlay)
                apply_crt(screen)
                pygame.display.flip()
        else:
            headless_render_skip = 0
            render_frame(screen, controller, status, overlay)
            apply_crt(screen)
            pygame.display.flip()

    pygame.quit()
    sys.exit(0)


if __name__ == "__main__":
    main()
