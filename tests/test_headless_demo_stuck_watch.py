"""Pygame headless Demo: run AI vs AI and detect STUCK / OSCILLATE from position telemetry.

Requires monotonic ``pygame.time.get_ticks`` — naive tight loops do not advance AI time.

Run locally with visibility::

    pytest tests/test_headless_demo_stuck_watch.py -s

Optional strict failure (CI / regression hunting)::

    set TANK_HEADLESS_FAIL_ON_WARN=1
    pytest tests/test_headless_demo_stuck_watch.py -s

Environment (defaults shown)::

    TANK_HEADLESS_STUCK_FRAMES=90      # same cell this many frames => STUCK warning
    TANK_HEADLESS_OSCILLATE_WINDOWS=8  # this many ABAB windows in one run => OSCILLATE warning
"""

from __future__ import annotations

import os
from dataclasses import dataclass

import pygame
import pytest


@dataclass(frozen=True)
class StuckWatchReport:
    frames_run: int
    ended_state: str
    max_static_streak_p1: int
    max_static_streak_p2: int
    abab_windows_p1: int
    abab_windows_p2: int


def _max_same_cell_run(positions: list[tuple[int, int]]) -> int:
    if not positions:
        return 0
    best = 1
    run = 1
    for i in range(1, len(positions)):
        if positions[i] == positions[i - 1]:
            run += 1
            best = max(best, run)
        else:
            run = 1
    return best


def _count_abab_sliding_windows(positions: list[tuple[int, int]]) -> int:
    """Count length-4 windows matching A,B,A,B with A != B (overlap allowed)."""
    if len(positions) < 4:
        return 0
    n = 0
    for i in range(len(positions) - 3):
        a, b, c, d = positions[i : i + 4]
        if a == c and b == d and a != b:
            n += 1
    return n


def _emit_warnings(report: StuckWatchReport) -> None:
    stuck_thr = int(os.environ.get("TANK_HEADLESS_STUCK_FRAMES", "90"))
    osc_thr = int(os.environ.get("TANK_HEADLESS_OSCILLATE_WINDOWS", "8"))

    if report.max_static_streak_p1 >= stuck_thr:
        print(
            f"[HEADLESS] STUCK suspected P1: max {report.max_static_streak_p1} frames "
            f"same cell (threshold {stuck_thr})"
        )
    if report.max_static_streak_p2 >= stuck_thr:
        print(
            f"[HEADLESS] STUCK suspected P2: max {report.max_static_streak_p2} frames "
            f"same cell (threshold {stuck_thr})"
        )
    if report.abab_windows_p1 >= osc_thr:
        print(
            f"[HEADLESS] OSCILLATE suspected P1: {report.abab_windows_p1} ABAB windows "
            f"(threshold {osc_thr})"
        )
    if report.abab_windows_p2 >= osc_thr:
        print(
            f"[HEADLESS] OSCILLATE suspected P2: {report.abab_windows_p2} ABAB windows "
            f"(threshold {osc_thr})"
        )


def run_headless_demo_watch(
    *,
    frames: int,
    step_ms: int,
    monkeypatch: pytest.MonkeyPatch,
) -> StuckWatchReport:
    """Advance Demo mode ``frames`` times; record tank positions each ``update``."""
    from tank_game.game import GameController, GameState

    ticks_ms = [0]

    def fake_get_ticks() -> int:
        ticks_ms[0] += step_ms
        return ticks_ms[0]

    monkeypatch.setattr(pygame.time, "get_ticks", fake_get_ticks)
    gc = GameController()
    gc.game_mode = "Demo"
    gc.init_round()
    assert gc.state == GameState.PLAYING

    p1_hist: list[tuple[int, int]] = []
    p2_hist: list[tuple[int, int]] = []

    for _ in range(frames):
        gc.update(1.0 / 60.0)
        if gc.state != GameState.PLAYING:
            break
        p1_hist.append((gc.tanks[1].x, gc.tanks[1].y))
        p2_hist.append((gc.tanks[2].x, gc.tanks[2].y))

    report = StuckWatchReport(
        frames_run=len(p1_hist),
        ended_state=gc.state.name,
        max_static_streak_p1=_max_same_cell_run(p1_hist),
        max_static_streak_p2=_max_same_cell_run(p2_hist),
        abab_windows_p1=_count_abab_sliding_windows(p1_hist),
        abab_windows_p2=_count_abab_sliding_windows(p2_hist),
    )
    _emit_warnings(report)

    if os.environ.get("TANK_HEADLESS_FAIL_ON_WARN"):
        stuck_thr = int(os.environ.get("TANK_HEADLESS_STUCK_FRAMES", "90"))
        osc_thr = int(os.environ.get("TANK_HEADLESS_OSCILLATE_WINDOWS", "8"))
        assert report.max_static_streak_p1 < stuck_thr, "P1 STUCK threshold exceeded"
        assert report.max_static_streak_p2 < stuck_thr, "P2 STUCK threshold exceeded"
        assert report.abab_windows_p1 < osc_thr, "P1 OSCILLATE threshold exceeded"
        assert report.abab_windows_p2 < osc_thr, "P2 OSCILLATE threshold exceeded"

    return report


def test_headless_demo_stuck_watch_smoke(monkeypatch: pytest.MonkeyPatch) -> None:
    """Harness runs without error; telemetry fields are populated."""
    os.environ.setdefault("SDL_VIDEODRIVER", "dummy")
    pygame.init()
    try:
        pygame.display.set_mode((320, 200))
        r = run_headless_demo_watch(frames=400, step_ms=50, monkeypatch=monkeypatch)
        assert r.frames_run >= 100
        assert r.max_static_streak_p1 >= 1
        assert r.max_static_streak_p2 >= 1
    finally:
        pygame.quit()
