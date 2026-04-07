"""Decoupled frame step: game logic vs rendering.

Logic (`tick_logic`) is pure of draw calls so tests can drive the controller headless.
Rendering (`render_frame`) isolates pygame drawing for manual QA / profiling.
"""

from __future__ import annotations

import time

import pygame

from .constants import HEADLESS_INNER_POLL_INTERVAL_STEPS, HEADLESS_WALL_BUDGET_DEFAULT_S
from .game import SHOWDOWN_MATRIX_CELLS, GameController, GameState
from .graphics import (
    draw_board,
    draw_explosions,
    draw_mines,
    draw_shots,
    draw_tanks,
    draw_wreckage,
)
from .ui import MessageOverlay, StatusDisplay

# Simulated seconds advanced per headless SHOWDOWN step (smaller = finer, slower CPU).
HEADLESS_SIM_STEP_S = 1.0 / 200.0
# Default wall budget (runtime value lives on ``GameController._headless_wall_budget_s``).
# Bug-safety cap on sim steps per *batch*. If this is too low, the loop exits on step count
# before ``budget_s`` wall time elapses — then raising ↑/↓ ms/frame has almost no effect.
HEADLESS_MAX_STEPS_SAFETY = 10_000_000


def _headless_menu_hint(controller: GameController) -> str:
    on = getattr(controller, "_showdown_headless_pref", False)
    state = "ON (status dashboard, fast sim)" if on else "OFF (full board)"
    return f"Headless: {state}\nH — toggle"


def run_headless_showdown_batch(controller: GameController) -> None:
    """Advance headless SHOWDOWN by a *chunk* of sim steps, then return.

    Uses a **wall-clock budget** so each frame returns quickly for ``display.flip`` and
    keeps the OS window responsive. **Cancel (ESC)** is polled inside this loop: ``handle_input``
    runs *before* the batch, so KEYDOWN for ESC can arrive while the batch is running and
    would otherwise not be seen until the next frame's ``event.get()`` — ``pump`` +
    ``key.get_pressed`` fixes that.
    """
    if controller.state != GameState.SHOWDOWN_RUNNING:
        return
    if not getattr(controller, "_showdown_headless", False):
        return
    budget_s = float(
        getattr(controller, "_headless_wall_budget_s", HEADLESS_WALL_BUDGET_DEFAULT_S)
    )
    poll_every = max(1, int(HEADLESS_INNER_POLL_INTERVAL_STEPS))
    pygame.event.pump()
    t0 = time.perf_counter()
    steps = 0
    next_poll_at = poll_every
    while controller.state == GameState.SHOWDOWN_RUNNING and steps < HEADLESS_MAX_STEPS_SAFETY:
        if steps >= next_poll_at:
            pygame.event.pump()
            if pygame.event.get(pygame.QUIT):
                controller.state = GameState.QUIT
                return
            if pygame.key.get_pressed()[pygame.K_ESCAPE]:
                controller.abort_showdown_run()
                return
            next_poll_at = steps + poll_every
        if time.perf_counter() - t0 >= budget_s:
            break
        controller._headless_sim_time_s += HEADLESS_SIM_STEP_S
        controller._headless_sim_time_cumulative += HEADLESS_SIM_STEP_S
        controller.update(HEADLESS_SIM_STEP_S)
        steps += 1


def format_headless_showdown_dashboard(controller: GameController) -> str:
    """Multi-line status for on-screen headless progress."""
    total = getattr(controller, "_showdown_iterations", 0) or 1
    cur = getattr(controller, "_showdown_current", 0)
    parallel = getattr(controller, "_showdown_parallel_thread", None) is not None
    pct = 100.0 * min(cur, total) / total
    bar_w = 28
    filled = int(bar_w * min(cur, total) / total)
    bar = "[" + "=" * filled + "-" * (bar_w - filled) + "]"

    if parallel:
        workers = getattr(controller, "_showdown_workers", 0)
        return (
            "HEADLESS SHOWDOWN (parallel)\n\n"
            f"Match {cur} / {total}   {pct:.1f}%\n"
            f"{bar}\n\n"
            f"Workers: {workers}\n"
            f"{controller.headless_sim_rate_dashboard_lines(parallel=True)}\n\n"
            "ESC — cancel run"
        )

    d1 = controller.difficulty_per_player.get(1, 0)
    d2 = controller.difficulty_per_player.get(2, 0)
    sim = getattr(controller, "_headless_sim_time_s", 0.0)
    budget_ms = (
        float(getattr(controller, "_headless_wall_budget_s", HEADLESS_WALL_BUDGET_DEFAULT_S))
        * 1000.0
    )
    return (
        "HEADLESS SHOWDOWN\n\n"
        f"Match {cur} / {total}   {pct:.1f}%\n"
        f"{bar}\n\n"
        f"Pairing:  P1 AI-{d1}   vs   P2 AI-{d2}\n\n"
        f"Sim time (this match): {int(sim)} s\n"
        f"{controller.headless_sim_rate_dashboard_lines()}\n"
        f"CPU budget: {budget_ms:.1f} ms/frame  (↑↓)\n"
        "Higher = faster sim · Lower = smoother UI\n\n"
        "ESC — cancel run"
    )


def tick_logic(
    controller: GameController,
    events: list[pygame.event.Event],
    dt: float,
) -> bool:
    """Advance simulation by one frame. Returns False if the app should exit.

    No drawing; safe to call in headless tests with injected events and fixed dt.
    """
    for event in events:
        if event.type == pygame.QUIT:
            return False

    controller.handle_input(events)
    if controller.state == GameState.SHOWDOWN_RUNNING and getattr(
        controller, "_showdown_headless", False
    ):
        if getattr(controller, "_showdown_parallel_thread", None) is not None:
            _tick_parallel_showdown(controller)
        else:
            run_headless_showdown_batch(controller)
            controller.tick_headless_sim_rate_display()
    else:
        controller.update(dt)

    if controller.state == GameState.QUIT:
        return False
    return True


def _tick_parallel_showdown(controller: GameController) -> None:
    """Poll the background multiprocessing thread and update progress."""
    thread = controller._showdown_parallel_thread
    if thread is None:
        return
    if not thread.is_alive():
        controller._showdown_parallel_thread = None
        if controller.state == GameState.SHOWDOWN_RUNNING:
            controller.state = GameState.SHOWDOWN_COMPLETE


_MENU_STATES = frozenset({
    GameState.MENU,
    GameState.GAME_MODE_SELECT,
    GameState.SKILL_SELECT,
    GameState.SHOWDOWN_SCHEDULE_TYPE,
    GameState.SHOWDOWN_ITERATIONS,
    GameState.SHOWDOWN_MATRIX_ROUNDS,
    GameState.SHOWDOWN_MATRIX_CONFIRM,
    GameState.PLAY_AGAIN,
    GameState.TOURNAMENT_END,
    GameState.SHOWDOWN_COMPLETE,
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
    elif state == GameState.GAME_MODE_SELECT:
        overlay.set_message(
            "SELECT GAME MODE:\n\n"
            "0 - DEMO (AI vs AI)\n"
            "1 - ONE PLAYER (vs CPU)\n"
            "2 - TWO PLAYER\n"
            "3 - SHOWDOWN (AI vs AI races)"
        )
    elif state == GameState.SKILL_SELECT:
        overlay.set_message(
            f"WHAT SKILL LEVEL(1-10)? {controller.difficulty}\n"
            "10 IS THE HARDEST"
        )
    elif state == GameState.SHOWDOWN_SCHEDULE_TYPE:
        overlay.set_message(
            "SHOWDOWN SCHEDULE:\n\n"
            "1 / R - RANDOM PAIRINGS\n"
            "    PICK TOTAL MATCH COUNT NEXT.\n\n"
            "2 / M - FULL SKILL MATRIX\n"
            "    EVERY AI-I VS AI-J (I,J IN 0..9),\n"
            "    N ROUNDS PER PAIRING.\n\n"
            f"{_headless_menu_hint(controller)}\n\n"
            "ESC - BACK"
        )
    elif state == GameState.SHOWDOWN_ITERATIONS:
        iterations = controller._showdown_input if hasattr(controller, "_showdown_input") else 100
        overlay.set_message(
            f"SHOWDOWN - RANDOM PAIRINGS\n"
            f"TOTAL MATCHES: {iterations}\n\n"
            "UP/DOWN +/-10   LEFT/RIGHT +/-1\n"
            "ENTER - START\n"
            "ESC - BACK\n\n"
            f"{_headless_menu_hint(controller)}"
        )
    elif state == GameState.SHOWDOWN_MATRIX_ROUNDS:
        r = getattr(controller, "_showdown_matrix_rounds_input", 10)
        total_m = SHOWDOWN_MATRIX_CELLS * r
        overlay.set_message(
            f"MATRIX: ROUNDS PER PAIRING\n{r}\n\n"
            f"TOTAL MATCHES: {total_m}  ({SHOWDOWN_MATRIX_CELLS} PAIRINGS X {r})\n\n"
            "UP/DOWN +/-1   LEFT/RIGHT +/-10\n"
            "ENTER - REVIEW & CONFIRM\n"
            "ESC - BACK\n\n"
            f"{_headless_menu_hint(controller)}"
        )
    elif state == GameState.SHOWDOWN_MATRIX_CONFIRM:
        r = getattr(controller, "_showdown_matrix_rounds_input", 10)
        total_m = SHOWDOWN_MATRIX_CELLS * r
        overlay.set_message(
            f"START MATRIX SHOWDOWN?\n\n"
            f"{total_m} MATCHES  ({r} ROUNDS X {SHOWDOWN_MATRIX_CELLS} PAIRINGS)\n\n"
            "ENTER - BEGIN\n"
            "ESC - EDIT ROUNDS\n\n"
            f"{_headless_menu_hint(controller)}"
        )
    elif state == GameState.PLAY_AGAIN:
        victory_msg = controller.get_victory_message()
        overlay.set_message(victory_msg + "\n\n\nANOTHER BATTLE?")
    elif state == GameState.TOURNAMENT_END:
        total = controller.battles_played
        winner = None
        unused = 0
        p1w = controller.wins[1]
        p2w = controller.wins[2]
        if p1w > p2w:
            winner = 1
        elif p2w > p1w:
            winner = 2

        header = f"AFTER {total} BATTLE{'S' if total != 1 else ''}"
        lines = f"    {header}\n    {'_' * len(header)}\n\n"
        if winner is not None:
            last_winner_tank = controller.tanks.get(winner)
            if last_winner_tank is not None:
                unused = last_winner_tank.lives - 1
            lines += f"PLAYER # {winner} WON\n"
            lines += f"AND HAD {unused} UNUSED TANK{'S' if unused != 1 else ''}\n"
        else:
            lines += "IT WAS A TIE!\n"
        lines += f"\nP1 WINS: {p1w}   P2 WINS: {p2w}\n"
        lines += "\n\nREADY."
        overlay.set_message(lines)
    elif state == GameState.SHOWDOWN_COMPLETE:
        summary_file = (
            controller._showdown_summary_file
            if hasattr(controller, "_showdown_summary_file")
            else ""
        )
        overlay.set_message(
            f"SHOWDOWN COMPLETE!\n\nRESULTS SAVED TO:\n{summary_file}\n\nPRESS ESC TO QUIT"
        )

    overlay.draw(screen)


def render_frame(
    screen: pygame.Surface,
    controller: GameController,
    status: StatusDisplay,
    overlay: MessageOverlay,
) -> None:
    """All pygame draw operations for one frame (no display.flip)."""
    if controller.state == GameState.SHOWDOWN_RUNNING and getattr(
        controller, "_showdown_headless", False
    ):
        screen.fill((20, 22, 30))
        overlay.set_message(format_headless_showdown_dashboard(controller))
        overlay.draw(screen)
        return

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
        ai_diff = None
        if controller.game_mode in ("Demo", "SHOWDOWN"):
            ai_diff = controller.difficulty_per_player
        elif controller.game_mode == "1P" and controller._ai:
            ai_diff = {2: controller.difficulty}
        status.draw(
            screen,
            controller.tanks[1],
            controller.tanks[2],
            ai_difficulty=ai_diff,
            winner=controller.winner,
            difficulty=controller.difficulty,
        )

    if controller.state == GameState.SHOWDOWN_RUNNING:
        current = controller._showdown_current if hasattr(controller, "_showdown_current") else 0
        total = (
            controller._showdown_iterations if hasattr(controller, "_showdown_iterations") else 0
        )
        overlay.set_message(f"SHOWDOWN IN PROGRESS...\n\nMatch {current} of {total}")
        overlay.draw(screen)
    else:
        overlay.clear()
