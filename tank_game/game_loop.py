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

    draw_board(screen, controller.board)
    draw_wreckage(screen, controller.board)
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
        )

    if controller.state == GameState.MENU:
        overlay.set_message("PRESS ANY KEY TO START")
        overlay.draw(screen)
    elif controller.state == GameState.GAME_MODE_SELECT:
        overlay.set_message(
            "SELECT GAME MODE:\n0 - DEMO (AI vs AI)\n1 - ONE PLAYER (vs CPU)\n"
            "2 - TWO PLAYER\n3 - SHOWDOWN (AI vs AI races)"
        )
        overlay.draw(screen)
    elif controller.state == GameState.SKILL_SELECT:
        overlay.set_message(f"SELECT SKILL LEVEL (1-10): {controller.difficulty}")
        overlay.draw(screen)
    elif controller.state == GameState.SHOWDOWN_RUNNING:
        current = controller._showdown_current if hasattr(controller, "_showdown_current") else 0
        total = (
            controller._showdown_iterations if hasattr(controller, "_showdown_iterations") else 0
        )
        overlay.set_message(f"SHOWDOWN IN PROGRESS...\n\nMatch {current} of {total}")
        overlay.draw(screen)
    elif controller.state == GameState.SHOWDOWN_SCHEDULE_TYPE:
        overlay.set_message(
            "SHOWDOWN SCHEDULE:\n\n"
            "1 / R — Random pairings (existing)\n"
            "   Pick total match count next.\n\n"
            "2 / M — Full skill matrix\n"
            "   Every AI-i vs AI-j (i,j in 0..9),\n"
            "   N rounds per pairing → 100×N matches.\n\n"
            f"{_headless_menu_hint(controller)}\n\n"
            "ESC — back"
        )
        overlay.draw(screen)
    elif controller.state == GameState.SHOWDOWN_MATRIX_ROUNDS:
        r = getattr(controller, "_showdown_matrix_rounds_input", 10)
        total_m = SHOWDOWN_MATRIX_CELLS * r
        overlay.set_message(
            f"MATRIX: ROUNDS PER PAIRING\n{r}\n\n"
            f"Total matches: {total_m}  ({SHOWDOWN_MATRIX_CELLS} pairings × {r})\n\n"
            "UP/DOWN ±1   LEFT/RIGHT ±10\n"
            "ENTER — review & confirm\n"
            "ESC — back\n\n"
            f"{_headless_menu_hint(controller)}"
        )
        overlay.draw(screen)
    elif controller.state == GameState.SHOWDOWN_MATRIX_CONFIRM:
        r = getattr(controller, "_showdown_matrix_rounds_input", 10)
        total_m = SHOWDOWN_MATRIX_CELLS * r
        overlay.set_message(
            f"START MATRIX SHOWDOWN?\n\n"
            f"{total_m} matches  ({r} rounds × {SHOWDOWN_MATRIX_CELLS} pairings)\n\n"
            "ENTER — begin\n"
            "ESC — edit rounds\n\n"
            f"{_headless_menu_hint(controller)}"
        )
        overlay.draw(screen)
    elif controller.state == GameState.SHOWDOWN_ITERATIONS:
        iterations = controller._showdown_input if hasattr(controller, "_showdown_input") else 100
        overlay.set_message(
            f"SHOWDOWN — RANDOM PAIRINGS\n"
            f"Total matches: {iterations}\n\n"
            "UP/DOWN ±10   LEFT/RIGHT ±1\n"
            "ENTER — start\n"
            "ESC — back\n\n"
            f"{_headless_menu_hint(controller)}"
        )
        overlay.draw(screen)
    elif controller.state == GameState.SHOWDOWN_COMPLETE:
        summary_file = (
            controller._showdown_summary_file
            if hasattr(controller, "_showdown_summary_file")
            else ""
        )
        overlay.set_message(
            f"SHOWDOWN COMPLETE!\n\nResults saved to:\n{summary_file}\n\nPress ESC to quit"
        )
        overlay.draw(screen)
    elif controller.state == GameState.PLAY_AGAIN:
        victory_msg = controller.get_victory_message()
        overlay.set_message(victory_msg + "\n\nANOTHER BATTLE? (Y/N)")
        overlay.draw(screen)
    elif controller.state == GameState.TOURNAMENT_END:
        p1_wins = controller.wins[1]
        p2_wins = controller.wins[2]
        total = controller.battles_played
        if p1_wins > p2_wins:
            champ = "PLAYER 1"
        elif p2_wins > p1_wins:
            champ = "PLAYER 2"
        else:
            champ = "TIE"

        stats = f"AFTER {total} BATTLE{'S' if total != 1 else ''}\n"
        stats += f"{'=' * 25}\n"
        stats += f"PLAYER 1 WINS: {p1_wins}\n"
        stats += f"PLAYER 2 WINS: {p2_wins}\n"
        stats += f"\nCHAMPION: {champ}\n"
        stats += "\nPRESS ENTER TO PLAY AGAIN\n"
        stats += "PRESS ESC TO QUIT"
        overlay.set_message(stats)
        overlay.draw(screen)
    else:
        overlay.clear()
