"""Parallel headless SHOWDOWN via multiprocessing.

Workers run independent matches without pygame.  The main process merges results
and feeds them back to ``GameController._showdown_results``.
"""

from __future__ import annotations

import multiprocessing
import os
import random
import time
from datetime import datetime
from typing import Any

_TIMEOUT_LOG = "tank_showdown_timeouts.log"


def _env_float(name: str, default: float) -> float:
    raw = os.environ.get(name, "").strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def _env_int(name: str, default: int) -> int:
    raw = os.environ.get(name, "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def _log_timeout_line(
    *,
    match_idx: int,
    diff1: int,
    diff2: int,
    seed: int,
    reason: str,
    attempt: int,
    sim_s: float,
    steps: int,
    wall_s: float,
) -> None:
    ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    line = (
        f"[{ts}] match={match_idx + 1} AI-{diff1} vs AI-{diff2} seed={seed} "
        f"attempt={attempt} reason={reason} sim_s={sim_s:.1f} steps={steps} wall_s={wall_s:.2f}\n"
    )
    try:
        with open(_TIMEOUT_LOG, "a", encoding="utf-8") as f:
            f.write(line)
    except OSError:
        pass


def _run_single_attempt(
    args: tuple[int, int, int, int], *, attempt: int = 1
) -> dict[str, Any]:
    """Run one headless match attempt; may return ``timeout=True`` if limits exceeded."""
    match_idx, diff1, diff2, seed = args

    from .constants import (
        SHOWDOWN_WORKER_MAX_SIM_TIME_S,
        SHOWDOWN_WORKER_MAX_STEPS,
        SHOWDOWN_WORKER_MAX_WALL_S,
    )
    from .game import GameController, GameState

    max_sim_s = _env_float("TANK_SHOWDOWN_MAX_SIM_S", SHOWDOWN_WORKER_MAX_SIM_TIME_S)
    max_steps = _env_int("TANK_SHOWDOWN_MAX_STEPS", SHOWDOWN_WORKER_MAX_STEPS)
    max_wall_s = _env_float("TANK_SHOWDOWN_MAX_WALL_S", SHOWDOWN_WORKER_MAX_WALL_S)

    WORKER_SIM_STEP_S = 1.0 / 60.0

    random.seed(seed)
    c = GameController()
    c.game_mode = "SHOWDOWN"
    c._showdown_headless = True
    c._suppress_summary_file = True
    c._showdown_iterations = 1
    c._showdown_current = 0
    c._showdown_schedule_kind = "matrix"
    c._showdown_matrix_schedule = [(diff1, diff2)]
    c._showdown_results = []
    c._start_showdown_match()

    t_wall0 = time.perf_counter()
    steps = 0
    timeout_reason: str | None = None

    while c.state == GameState.SHOWDOWN_RUNNING:
        if steps >= max_steps:
            timeout_reason = "max_steps"
            break
        if c._headless_sim_time_s >= max_sim_s:
            timeout_reason = "max_sim_time"
            break
        if time.perf_counter() - t_wall0 >= max_wall_s:
            timeout_reason = "max_wall"
            break

        c._headless_sim_time_s += WORKER_SIM_STEP_S
        c._headless_sim_time_cumulative += WORKER_SIM_STEP_S
        c.update(WORKER_SIM_STEP_S)
        steps += 1

    wall_elapsed = time.perf_counter() - t_wall0

    if timeout_reason is not None:
        sim_at = c._headless_sim_time_s
        _log_timeout_line(
            match_idx=match_idx,
            diff1=diff1,
            diff2=diff2,
            seed=seed,
            reason=timeout_reason,
            attempt=attempt,
            sim_s=sim_at,
            steps=steps,
            wall_s=wall_elapsed,
        )
        return {
            "match": match_idx + 1,
            "winner": 0,
            "ai1_diff": diff1,
            "ai2_diff": diff2,
            "duration": sim_at,
            "timeout": True,
            "timeout_reason": timeout_reason,
        }

    if c._showdown_results:
        result = c._showdown_results[0]
        result["match"] = match_idx + 1
        result.setdefault("timeout", False)
        return result
    return {
        "match": match_idx + 1,
        "winner": 0,
        "ai1_diff": diff1,
        "ai2_diff": diff2,
        "duration": 0.0,
        "timeout": False,
    }


def _run_match(args: tuple[int, int, int, int]) -> dict[str, Any]:
    """Run a headless match with retries on timeout (new seed each retry).

    Args is ``(match_index, diff1, diff2, seed)`` — packed for ``Pool.map``.
    """
    from .constants import SHOWDOWN_WORKER_MAX_RETRIES

    max_retries = _env_int("TANK_SHOWDOWN_MAX_RETRIES", SHOWDOWN_WORKER_MAX_RETRIES)
    match_idx, diff1, diff2, seed = args
    current_seed = seed

    for attempt in range(1, max_retries + 1):
        r = _run_single_attempt((match_idx, diff1, diff2, current_seed), attempt=attempt)
        if not r.get("timeout"):
            if attempt > 1:
                r["retries_before_success"] = attempt - 1
            return r

        current_seed = (current_seed * 1103515245 + 12345) & 0x7FFFFFFF
        if current_seed == 0:
            current_seed = 1

    r["timeout_after_retries"] = True
    return r


def _build_schedule(
    kind: str,
    iterations: int,
    matrix_schedule: list[tuple[int, int]],
    base_seed: int,
) -> list[tuple[int, int, int, int]]:
    """Build ``(match_idx, diff1, diff2, seed)`` work items."""
    rng = random.Random(base_seed)
    items: list[tuple[int, int, int, int]] = []
    for i in range(iterations):
        if kind == "matrix" and i < len(matrix_schedule):
            d1, d2 = matrix_schedule[i]
        else:
            d1 = rng.randint(0, 9)
            d2 = rng.randint(0, 9)
            while d2 == d1:
                d2 = rng.randint(0, 9)
        seed = rng.randint(0, 2**31)
        items.append((i, d1, d2, seed))
    return items


def run_showdown_parallel(
    controller: Any,
    *,
    workers: int | None = None,
    progress_callback: Any | None = None,
) -> None:
    """Replace the sequential headless loop with a multiprocessing pool.

    Updates ``controller._showdown_results``, ``_showdown_current``, and sets
    ``state = SHOWDOWN_COMPLETE`` when done (or calls ``_finish_showdown``).
    """
    if workers is None:
        workers = max(1, (os.cpu_count() or 1))

    kind = controller._showdown_schedule_kind
    total = controller._showdown_iterations
    matrix_sched = controller._showdown_matrix_schedule
    base_seed = int(time.time() * 1000) & 0x7FFFFFFF

    items = _build_schedule(kind, total, matrix_sched, base_seed)

    controller._showdown_results = []
    controller._showdown_current = 0
    controller._headless_tournament_wall_start = time.perf_counter()

    ctx = multiprocessing.get_context("spawn")
    with ctx.Pool(processes=workers) as pool:
        for result in pool.imap_unordered(_run_match, items, chunksize=1):
            controller._showdown_results.append(result)
            controller._showdown_current = len(controller._showdown_results)
            if progress_callback is not None:
                progress_callback(controller._showdown_current, total)

    controller._showdown_current = total
    controller._finish_showdown()
