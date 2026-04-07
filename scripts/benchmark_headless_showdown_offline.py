#!/usr/bin/env python3
"""Offline headless SHOWDOWN throughput (dummy video, no ``display.flip()``).

Run from repo root::

  python scripts/benchmark_headless_showdown_offline.py

Uses ``SDL_VIDEODRIVER=dummy`` — no real window.

Uses **one** ``pygame.init()`` for both scenarios (no SDL quit between) so the second
scenario is not penalized by cold-start costs.

Measures sim s per wall s for:

1. **tick_logic** — same path as ``main`` minus rendering/flip.
2. **batch only** — ``run_headless_showdown_batch`` in a tight loop (no ``handle_input``).

**Expectation:** With dummy video there is no GPU ``flip`` — sim/wall should reflect CPU
work in ``update()`` / the headless batch loop, typically **~2.3–2.7×** on this workload.
That is often **below** the on-screen dashboard (~3.2–3.7×): the live number can differ
(rolling window, burst, real SDL). **Scenario order** in one process can bias the second
run (thermal/OS); this script runs **batch first**, then ``tick_logic``, to reduce that.
"""

from __future__ import annotations

import argparse
import os
import random
import time

os.environ.setdefault("SDL_VIDEODRIVER", "dummy")
os.environ.setdefault("SDL_AUDIODRIVER", "dummy")

import pygame
from tank_game.constants import WINDOW_HEIGHT, WINDOW_WIDTH
from tank_game.game import GameController, GameState
from tank_game.game_loop import run_headless_showdown_batch, tick_logic

DEFAULT_WALL_SECONDS = 5.0
DEFAULT_DT = 1.0 / 240.0


def _fresh_showdown_controller() -> GameController:
    random.seed(42)
    c = GameController()
    c.game_mode = "SHOWDOWN"
    c._showdown_headless = True
    c._showdown_iterations = 50_000
    c._showdown_current = 0
    c._showdown_schedule_kind = "random"
    c._showdown_matrix_schedule = []
    c._showdown_results = []
    c._start_showdown_match()
    if c.state != GameState.SHOWDOWN_RUNNING:
        raise RuntimeError(f"expected SHOWDOWN_RUNNING, got {c.state}")
    return c


def bench_tick_logic(wall_s: float, dt: float) -> tuple[float, float, int]:
    c = _fresh_showdown_controller()
    sim0 = c._headless_sim_time_cumulative
    t0 = time.perf_counter()
    frames = 0
    while time.perf_counter() - t0 < wall_s:
        if not tick_logic(c, [], dt):
            break
        frames += 1
        if c.state != GameState.SHOWDOWN_RUNNING:
            break
    wall = time.perf_counter() - t0
    sim = c._headless_sim_time_cumulative - sim0
    return sim, wall, frames


def bench_batch_only(wall_s: float) -> tuple[float, float, int]:
    c = _fresh_showdown_controller()
    sim0 = c._headless_sim_time_cumulative
    t0 = time.perf_counter()
    batches = 0
    while time.perf_counter() - t0 < wall_s:
        run_headless_showdown_batch(c)
        batches += 1
        if c.state != GameState.SHOWDOWN_RUNNING:
            break
    wall = time.perf_counter() - t0
    sim = c._headless_sim_time_cumulative - sim0
    return sim, wall, batches


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--wall",
        type=float,
        default=DEFAULT_WALL_SECONDS,
        help="Wall seconds per scenario (default %(default)s)",
    )
    args = ap.parse_args()
    wall_s = float(args.wall)
    dt = DEFAULT_DT

    pygame.init()
    pygame.display.set_mode((WINDOW_WIDTH, WINDOW_HEIGHT))
    try:
        print("=== Headless SHOWDOWN offline benchmark ===")
        print(f"SDL_VIDEODRIVER={os.environ.get('SDL_VIDEODRIVER')!r}")
        print(f"window: {WINDOW_WIDTH}x{WINDOW_HEIGHT}, wall={wall_s}s, dt={dt:.6f}s")
        print("(single pygame session for both scenarios)")
        print()

        s1, w1, b1 = bench_batch_only(wall_s)
        r1 = s1 / w1 if w1 > 0 else 0.0
        print("[1] run_headless_showdown_batch only (tight loop)")
        print(f"    sim_s={s1:.4f}  wall_s={w1:.4f}  ratio={r1:.3f}x")
        print(f"    batches={b1}  ({b1/w1:.1f} Hz)")
        print()

        s2, w2, f2 = bench_tick_logic(wall_s, dt)
        r2 = s2 / w2 if w2 > 0 else 0.0
        print("[2] tick_logic (no render, no flip)")
        print(f"    sim_s={s2:.4f}  wall_s={w2:.4f}  ratio={r2:.3f}x")
        print(f"    calls={f2}  ({f2/w2:.1f} Hz effective)")
        print()

        if r1 > 0 and r2 > 0:
            rel = abs(r2 - r1) / r1
            print(f"Ratio difference: {rel*100:.1f}% (tick_logic vs batch).")
            if rel < 0.12:
                print(
                    "-> Throughput matches: handle_input overhead is small; cost is in "
                    "run_headless_showdown_batch / update()."
                )
            print()
            print(
                "Dummy video: no GPU flip. If both are ~2.3-2.7x, display is unlikely the "
                "sole cap vs a ~3.5x dashboard reading (metric/timing may differ)."
            )
    finally:
        pygame.quit()


if __name__ == "__main__":
    main()
