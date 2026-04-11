## Changelog

### 2.0.0-alpha.1 - 2026-04-10 — Wave 1: Go Game Engine

Pure Go port of the Python game engine (`engine/` package). All game mechanics faithfully ported from the Python 2P codebase with 158 tests, zero float positions (pure integer grid).

- **10 Go source files** in `engine/`: `doc.go`, `constants.go`, `board.go`, `player.go`, `projectile.go`, `game.go`, `shotshot.go`, `mine_explosion.go`, `match_runner.go`, plus test files.
- **Constants & enums:** `CellType` (1-10), `Direction` (1-8), `GameState`, `Action` (0-10), timing constants — all matching Python `IntEnum` values.
- **Board:** 40×21 grid, difficulty-scaled terrain generation (4%–25% wall density), deterministic seeded RNG.
- **Tank:** 8-direction movement, barrel tracking, lives/ammo/mines resource management, spawn position memory.
- **Projectile:** Shot movement with max range (75% of dimension), mine placement with visibility timer, detonation delay.
- **Collision system:** All 8 collision types ported — shot→wall, shot→tank body, shot→barrel, shot→wreckage, shot→mine, shot→shot (head-on swap detection), mine explosion chains, tank→mine.
- **Barrel swing:** Direction change without movement, old barrel cell cleanup.
- **Respawn:** Both-tank respawn after kill, spawn position scanning (up/down), resource reset per difficulty.
- **GameController:** Full update loop orchestrating movement, shots, mines, explosions, barrel hits, win detection, round/game state transitions.
- **Headless match runner:** `RunMatch()` with `InputProvider` interface, `RandomInputProvider` for simulation, `ScriptedInputProvider` for deterministic tests.
- **158 tests** across 9 test files, all passing. Race-free (`go test -race`), vet clean.
- **12 golden integration tests** covering all collision types with deterministic scripted scenarios.

### 1.0.0 - 2026-04-10 — TANK! 2P Fork

Two-player-only fork of the TANK! PyGame port. Stripped all AI, SHOWDOWN tournament, headless runner, Demo mode, and 1P mode.

- **Package renamed** to `tank-game-2p`.
- **Removed 9 AI source files:** `ai.py`, `ai_tree.py`, `ai_behaviours.py`, `ai_decorators.py`, `ai_navigation.py`, `ai_perception.py`, `ai_utility.py`, `ai_blackboard.py`, `showdown_worker.py`.
- **Removed `py_trees` dependency** — only `pygame` and `cbmcodecs2` required.
- **GameState reduced** from 16 to 8 values: `MENU`, `SKILL_SELECT`, `COUNTDOWN`, `PLAYING`, `ROUND_OVER`, `GAME_OVER`, `PLAY_AGAIN`, `QUIT`.
- **Simplified menu flow:** MENU → SKILL SELECT (1–10) → PLAYING → winner → ANOTHER BATTLE? Y/N → restart or MENU.
- **game.py** gutted from 1695 to ~878 lines (AI init, SHOWDOWN handlers, Demo logic, 1P mode removed).
- **game_loop.py** reduced from 344 to ~117 lines (headless batch runner, dashboard, parallel thread removed).
- **main.py** reduced from 121 to ~76 lines (py_trees check, headless FPS switching, render skip removed).
- **constants.py** reduced from 159 to ~137 lines (HEADLESS/SHOWDOWN constants removed).
- **Removed 9 AI/SHOWDOWN test files;** remaining tests cover board, player, projectile, and core game logic.

### 0.9.7 - 2026-04-07

- **Fix: deferred empty-gun self-destruct.** Firing the last shot no longer immediately kills the player. The self-destruct is deferred until all of that player's in-flight projectiles resolve. If the last shot kills the opponent and ends the round, the firing player wins instead of dying first.

### 0.9.6 - 2026-04-07

- **Quarter-block wreckage dots:** Dice-pattern wreckage from tank explosions now uses `▗` (U+2597, lower-right quadrant block) instead of a full solid block, making the dots visually proportional to one cell.
- **Projectiles destroy wreckage:** Shots now clear wreckage cells on contact (like walls/terrain), matching original PET behavior where wreckage is destructible.

### 0.9.5 - 2026-04-07

- **Wreckage now full-brightness:** Dice-dot wreckage from tank explosions now uses `COLOR_PET_FG` (same phosphor green as walls/tanks/borders) and renders with inverted fill (guaranteed full-cell coverage). Previously used a dimmer green that could be nearly invisible.

### 0.9.4 - 2026-04-07

- **Fix ghost barrel on direction swing:** Barrel swing (first keypress in a new direction) now properly clears the old barrel cell and places the new one. Previously, changing direction without moving left a phantom BARREL cell on the board that could be hit by incoming shots. Also fixed the same bug in AI barrel-direction changes (risky aim, risky restore, scan).

### 0.9.3 - 2026-04-07

- **HUD inverted text:** Top-row labels and values now render as black glyphs on a green bar (inverted/reverse-video), matching the original PET look. Previously rendered green-on-black.
- **Center separator narrowed:** Circle separator between player panels is 2 columns (was 4), matching the original PET layout.
- **Panel width corrected:** Each player panel is 19 columns (was 18). Fixes "MINES" being truncated to "MINE" and improves text spacing.
- **PET-authentic board dimensions:** Board resized from 40x25 to 40x21 to match the original PET playfield (rows 2-22 of the 25-row screen). Interior playfield is now 38x19. Window height reduced from 540px to 460px.

### 0.9.2 - 2026-04-07

- **Explosion wreckage now visible:** Dice-pattern wreckage from tank explosions now correctly overwrites WALL cells (matching original PET behavior where explosions destroy surrounding terrain). Previously walls blocked wreckage placement, leaving 0-2 of 4 dots at higher difficulties.
- **Wreckage color brightened:** Changed from dim `(20,100,20)` to `(0,180,0)` so wreckage is clearly visible against the black background.
- **Barrel-hit wreckage visual:** Barrel-shot tanks now leave a visible inverse-X body + curled barrel, matching the original PET wreckage appearance.

### 0.9.0 - 2026-04-07

Retro-accuracy corrections to match the original PET TANK! (Cursor #26) mechanics.

- **Projectile max range:** Shots travel at most 75% of the board dimension in their direction, then silently disappear (no explosion). Matches the original PET shot behavior.
- **Constant shot speed:** 0.1s per cell, no longer scales with difficulty.
- **Barrel shot mechanic:** Shots can now hit a tank's barrel cell. Barrel hits: no explosion, only the hit player loses a life and respawns (other player stays). Leaves curled-barrel wreckage.
- **Difficulty-scaled terrain:** Wall density scales from ~4% (level 1) to ~25% (level 10). Previously fixed at 12%.
- **Ammo per difficulty:** Level 1-2: 6, 3-5: 8, 6-8: 10, 9-10: 12 shots. Previously fixed at 18.
- **Mines per difficulty:** Level 1: 0, 2-4: 1, 5-7: 2, 8-10: 3 mines. Previously fixed at 3.
- **Tank speed calibration:** Base 0.3s/cell (matches PET ~12s screen-cross), scales to 0.2s/cell (150%) at difficulty 10. Previously 1.0s to 0.1s.
- **Multi-cell wreckage:** Destroyed tanks leave 4-dot "dice" pattern (corners of 3x3 area) as obstacles. Degrades gracefully near borders.
- **Per-player HUD messages:** Border row shows status messages: "LOW SHOTS" (<=20% ammo), "OUT OF SHOTS", "LAST TANK", "THE WINNER".
- **Barrel cells on board:** Tank barrels now register as BARREL1/BARREL2 cells on the board, enabling barrel-specific collision detection and AI pathfinding awareness.

### 0.8.0 - 2026-04-07

PETSCII retro UI overhaul — all rendering now uses authentic Commodore PET character glyphs.

- **Font:** Bundled **PetMe64.ttf** (KreativeKorp, free license) loaded via `petscii_render.get_pet_font()`; override with `TANK_PET_FONT=/path/to/font.ttf`. All text rendered with `antialias=False`.
- **PETSCII mapping:** New `tank_game/petscii_map.py` decodes PETSCII bytes through `cbmcodecs2` (`petscii_c64en_uc` codec) into Unicode glyphs the PET font renders. `PET_MAP` covers border, wall, tank body, barrel (H/V), mine, shot, explosion spokes, and wreckage.
- **Glyph cache:** `tank_game/petscii_render.py` — `glyph_surface()` (LRU-cached per char+color+size), `blit_cell()` / `blit_glyph()` helpers for the cell grid.
- **Playfield:** `graphics.py` rewritten — border uses `0x51` (ball) around the perimeter, walls use `0x66` (checkered block), tanks are composite body+barrel glyphs, mines use `0x71`, shots use `0x51`.
- **Explosions:** Multi-cell PETSCII composite — center circle with 8-way radial spokes (diagonal N/M line chars, cardinal H/V lines). Chain/catalyst explosions use a **larger** two-ring pattern.
- **HUD:** `StatusDisplay` redesigned to PET-authentic two-row layout: solid-block green bar (`0xA0`), `TANKS SHOTS MINES` labels and numeric values, circle (`0x51`) center separators.
- **Colors:** Monochrome **P1 phosphor green** `(51, 255, 51)` for all game elements (border, walls, tanks, shots, mines, text); dimmer green for wreckage.
- **CRT effect:** Optional `TANK_CRT=1` — scanline overlay + bloom (downscale/upscale via `smoothscale`). `tank_game/crt_effect.py`.
- **Dependency:** Added `cbmcodecs2>=1.0` to `pyproject.toml`.

### 0.7.1 - 2026-04-07

- **Fix: 1P AI speed scaling.** The 0.7.0 skill-table overhaul removed `fast_mode` / `user_difficulty` but forgot to pass `move_delay` to the 1P `AIPlayer` constructor. The AI received its raw SHOWDOWN-tuned `reaction_time` (as low as 0.020s) while the human was gated by the difficulty-based `_move_delay` (0.1s-1.0s), making the AI impossibly fast at every level. Now passes `move_delay=self._move_delay` so `get_ai_config` scales reaction time proportionally to game speed.

### 0.7.0 - 2026-04-07

AI skill ladder overhaul: every parameter for skill levels 0-9 is now **unique and strictly monotonic**, stored in an explicit `_SKILL_TABLE` lookup. No more tier bands, `// 3` banding, boolean thresholds, or shared presets.

**Config architecture:**
- **`_map_cells(fraction)`** helper converts fractions of `min(map_width, map_height)` to integer cells; all range/radius parameters auto-scale if the map resizes.
- **Expanded `AIConfig`:** new fields `peripheral_radius_pct`, `engage_range_pct`, `los_range_pct` (map-relative fractions), `risky_chance` (float 0-1 replacing boolean), `frustration_threshold`, `skip_fire_base`, `scan_when_blind`.
- **`_SKILL_TABLE`**: 10 hand-tuned `AIConfig` entries with 13 strictly monotonic parameters (verified by `scripts/verify_skill_table.py`).
- **`get_ai_config()`** indexes the table directly; `fast_mode` parameter eliminated.
- **`effective_min_shots_for_aimed`**: inverted relationship — higher confidence = fewer reserves needed (stops penalizing top-skill AI).

**Inline checks removed:**
- All `self.difficulty >= N` comparisons in `AIPlayer` methods replaced with config field lookups (`engage_range`, `los_range`, `risky_chance`, `scan_when_blind`, `frustration_threshold`, `skip_fire_base`).
- `aggressive_mines` changed from `bool` to `float` (0.0-1.0) for gradual mine-laying probability.
- `risky_shot_viable` uses `risky_chance` probability instead of a binary gate.
- `avoid_own_mines` confidence gate uses `scan_when_blind` flag.

**Navigation:**
- `NavigationConfig.sight_radius` and `greedy_distance` stored as `_pct` fractions, resolved to cells once in `NavigationMemory.__init__` via `_map_cells`.
- Explicit `_NAV_TABLE` with 10 unique entries (no `// 2` banding).

**Removed:** `fast_mode` parameter from `AIPlayer.__init__`, `get_ai_config`, and `game.py` `init_round`.

**Validation (5040-match matrix):** aggregate win rate 34.7% (AI-0) → 59.6% (AI-8), AI-9 beats AI-8 at 53.6% H2H. Zero timeouts.

### 0.6.6 - 2026-04-07

- **README:** Player-facing sections first; SHOWDOWN, headless/parallel runs, env vars, pytest/Ruff/mypy, and AI contributor notes moved under **REGRESSION TESTING**.

### 0.6.5 - 2026-04-07

- **README:** Rewrote for the current codebase — version line, features (two-player controls, SHOWDOWN matrix/headless/parallel), environment variable table, AI/testing pointers, and doc index. Removed outdated “scaffolding / incremental plan” status.

### 0.6.4 - 2026-04-07

- **Parallel SHOWDOWN worker timeouts:** Each match attempt is bounded by simulated time (`TANK_SHOWDOWN_MAX_SIM_S`, default **600 s**), update steps (`TANK_SHOWDOWN_MAX_STEPS`, default **2_000_000**), and wall clock (`TANK_SHOWDOWN_MAX_WALL_S`, default **180 s**). On timeout, a line is appended to **`tank_showdown_timeouts.log`**, the match is retried with a new RNG seed up to **`TANK_SHOWDOWN_MAX_RETRIES`** (default **5**). If all retries fail, the result is recorded with `timeout` / `timeout_after_retries` and the summary file lists **`TIMEOUT (reason)`** instead of a winner.
- **Stats fix:** `_finish_showdown` no longer credits player 2 when `winner == 0`. Head-to-head matrix only increments P1 wins when `winner == 1`.
- **Test:** `test_headless_dashboard_sim_time_whole_seconds_only` updated to match the current dashboard copy (`Sim rate (live):`).

### 0.6.3 - 2026-04-07

- **Critical fix: `TANK_DEBUG=0` was truthy.** `os.environ.get("TANK_DEBUG")` returns the string `"0"`, which Python evaluates as truthy (`bool("0") is True`). All four debug-log sites (`game.py`, `main.py`, `ai_tree.py`, `projectile.py`) now use a proper falsy-string check: `"0"`, `"false"`, `"no"`, `"off"`, and empty string all disable logging. This was producing a **150 MB** `tank_debug.log` with 24 worker processes all appending concurrently (garbled interleaved output visible in the file), causing massive I/O contention on top of the per-match summary file issue.

### 0.6.2 - 2026-04-07

Parallel SHOWDOWN worker throughput overhaul. With 24 workers on a Ryzen 9 5950X, measured throughput was **1.28 matches/sec** at only **7.5% CPU** — workers were spending ~90% of wall time blocked in kernel space by Windows Defender's minifilter scanning per-match summary files.

- **Suppress per-match file writes in workers:** Added `_suppress_summary_file` flag on `GameController`; set `True` in `_run_match`. Workers no longer call `open()` / write any files — results stay in RAM and the single summary file is written once by the main process at tournament end. This was the primary bottleneck (Defender processed ~1.3 file creates/sec across all 24 workers).
- **Coarser worker sim step:** Workers now use `WORKER_SIM_STEP_S = 1/60` (was `1/200`). Reduces ticks per match by ~3.3× while preserving game outcomes (all game logic is time-gated, not tick-gated). Workers no longer import `game_loop.HEADLESS_SIM_STEP_S`.
- **Lazy pygame in workers:** Removed top-level `import pygame` from `game.py` and `constants.py`. Key dicts (`PLAYER1_KEYS`, `PLAYER2_KEYS`) now use module `__getattr__` for deferred initialization. `sim_time_s()` and `handle_input()` import pygame locally (zero-cost in the main process where it's already cached in `sys.modules`). Worker processes no longer load SDL2 DLLs at all — verified: `'pygame' not in sys.modules` after a full worker match.
- **Fast `_debug_log`:** Module-level `_DEBUG = bool(os.environ.get("TANK_DEBUG"))` replaces per-call `os.environ.get`. Hottest call sites (`_run_ai`, `_update_shots`) guarded with `if _DEBUG:` to skip f-string evaluation entirely (~64 call sites, thousands of invocations per match).

### 0.6.1 - 2026-04-07

- **Fix:** Parallel SHOWDOWN dashboard now updates in real time. Changed `imap_unordered` to `chunksize=1` so each completed match immediately increments the progress counter (was `chunksize=25`, causing ~8 s of apparent stall before the first update). Main loop renders every frame at 60 FPS in parallel mode (was skipping 3/4 frames at 240 FPS — unnecessary since parallel tick is near-zero cost). Dashboard drops the meaningless "Sim rate (live)" line in parallel mode, showing only **matches/sec** and **ETA**.

### 0.6.0 - 2026-04-07

Headless SHOWDOWN throughput overhaul. cProfile revealed AI (A* pathfinding + peripheral vision) consumed **97%** of per-step CPU; the dashboard's **~3.5× sim/wall** was a misleading artifact of the sim-clock resetting every match boundary. Actual raw `update()` speed was already **~22×**. Changes:

- **IntEnum:** `CellType` and `Direction` changed from `Enum` to `IntEnum` — eliminates **2.6M** `enum.__hash__` calls per 5 k steps (12% of total CPU).
- **AI pre-gate:** `_run_ai` checks reaction-time **before** building args / calling `ai.update()` / entering py_trees — skips ~80% of AI calls without touching the tree.
- **A\* cache:** `NavigationMemory._astar_first_step` caches `(start, goal) → result`; repeated calls with same position return instantly. Cache invalidated on explosion / respawn / reset.
- **Inline `get_cell`:** Board bounds check inlined (was separate `in_bounds()` call on 1.72M invocations).
- **Cumulative sim counter:** New `_headless_sim_time_cumulative` that **never resets** at match boundaries. Dashboard rolling rate and benchmark now use it — no more negative/misleading readings.
- **Dashboard:** Shows **matches/sec** and **ETA** instead of the old per-match sim rate.
- **Render skip:** During headless SHOWDOWN, `render_frame` + `display.flip` run only every **4th** outer frame (dashboard text changes at most ~1 Hz).
- **Multiprocessing:** Set **`TANK_WORKERS=N`** (env var) before launching to run matches across **N** processes. On a 16-core Ryzen 9 5950X: **~25 matches/sec** (16 workers) vs **~3/s** (sequential). ETA for 50 k matches drops from **~5 hours to ~33 minutes**.
- **Bug fix:** `init_round()` no longer overwrites `difficulty_per_player` with fresh random values in SHOWDOWN mode — per-pairing skills from the schedule are now preserved.

### 0.5.28 - 2026-04-06

- **Offline benchmark:** Run **batch first**, then **tick_logic**, to reduce systematic bias when both run in one process (second scenario can read lower). Docstring and footer text updated for typical **~2.3–2.7×** dummy-SDL results vs dashboard. Ruff **I001** import order on ``scripts/benchmark_headless_showdown_offline.py``.

### 0.5.27 - 2026-04-06

- **Headless SHOWDOWN throughput:** The inner sim loop no longer calls ``pygame.event.pump()`` (and QUIT / ESC checks) **on every** ``update()`` step — that was dominating CPU and capping sim rate around a few× regardless of wall budget or main-loop FPS. Polling is throttled to every **`HEADLESS_INNER_POLL_INTERVAL_STEPS` (64)** steps (tunable in `tank_game/constants.py`), with one pump at batch start.
- **Vsync:** ``pygame.display.set_mode(..., vsync=…)`` — default **on**. Set environment variable **`TANK_VSYNC=0`** (or `false` / `no` / `off`) before launch to disable vsync on ``flip()`` if the display stack was still capping headless throughput near monitor refresh rate.

### 0.5.26 - 2026-04-06

- **Headless SHOWDOWN:** Main loop uses **`HEADLESS_SHOWDOWN_MAIN_LOOP_FPS` (240)** instead of **60** so `tick_logic` / headless batches run up to ~4× more often per wall second. Other modes still use **`MAIN_LOOP_FPS` (60)**. Constants in `tank_game/constants.py`.

### 0.5.25 - 2026-04-06

- **Headless dashboard:** Restored **Sim time Δ (last window):** — value is the **previous match’s** sim s / wall s (summary when advancing to the next match). **Sim time Δ (live):** unchanged (rolling ~1 s).

### 0.5.24 - 2026-04-06

- **Remove:** Headless **AutoTune** (A key, SET/RECENTER, related constants and state).
- **Headless SHOWDOWN:** Raised **`HEADLESS_MAX_STEPS_SAFETY`** from **100_000** to **10_000_000** per batch. The old cap often stopped the inner loop **before** the wall **CPU budget** was used, so 50–250 ms budgets could look identical; the budget is now much more likely to be the real limiter. Dashboard still shows **Sim time Δ (live)** (rolling ~1 s).

### 0.5.23 - 2026-04-06

- **Headless AutoTune:** SET starts at **50 ms** (aligned with default wall budget). Each measurement window is **10 s** wall time (was 5 s); **RECENTER** interval in HOLD is **30 s** (was 60 s). Dashboard: **Sim time Δ (live)** — rolling **~1 s** sim s / wall s; **Sim time Δ (last window)** — rate from the last completed AutoTune trial (SET or RECENTER). *(Removed in 0.5.24.)*

### 0.5.22 - 2026-04-06

- **Headless SHOWDOWN — AutoTune:** Press **A** on the dashboard to toggle. **SET:** start from **20 ms** wall budget, measure **sim rate** (Δ sim time / Δ wall time) over **5 s** windows, sweep **up** then **down** in **2 ms** steps to a local maximum, then **HOLD** at that budget. **RECENTER:** every **60 s**, try **+10%** budget; keep it only if sim rate improves, else revert. Constants in `tank_game/constants.py` (`HEADLESS_AUTOTUNE_*`). AutoTune stops when SHOWDOWN ends or is cancelled.

### 0.5.21 - 2026-04-06

- **Headless SHOWDOWN dashboard:** **Sim time (this match)** shows **whole seconds** only. `MessageOverlay` now **caches** rendered lines until the message text or surface size changes, so the overlay avoids per-frame `font.render` when the dashboard string is unchanged (typically for most frames within each simulated second).

### 0.5.20 - 2026-04-06

- **Headless SHOWDOWN:** Default **wall budget** raised from **12 ms** to **50 ms** (`HEADLESS_WALL_BUDGET_DEFAULT_S`) — better default throughput while keeping UI updates and input responsive; ↑/↓ still adjusts at runtime.

### 0.5.19 - 2026-04-06

- **Fix:** **ESC** now cancels **headless SHOWDOWN** reliably. `handle_input` only runs before each headless batch, so ESC pressed during a long batch was invisible until the batch finished. The headless batch loop now calls `pygame.event.pump()`, polls `pygame.key.get_pressed()[K_ESCAPE]`, and drains `QUIT` events each iteration. Added `GameController.abort_showdown_run()` (sets `SHOWDOWN_COMPLETE` and a short cancelled summary message for the overlay).

### 0.5.18 - 2026-04-06

- **Headless SHOWDOWN:** **↑ / ↓** on the status dashboard adjusts **CPU budget** (wall time per frame, shown in ms). Stored in `GameController._headless_wall_budget_s`; defaults and limits in `tank_game/constants.py` (`HEADLESS_WALL_BUDGET_*`).

### 0.5.17 - 2026-04-06

- **Fix:** Headless SHOWDOWN chunking now uses a **wall-clock budget** per frame (`HEADLESS_WALL_BUDGET_S`, default 12 ms) instead of a fixed step count. Heavy `update()` cost (e.g. **`TANK_DEBUG`** writing every line to disk) could still freeze the UI for seconds with a large step cap. Added `pygame.event.pump()` in the headless batch and at the start of `main`’s loop when headless + running so Windows can process messages.

### 0.5.16 - 2026-04-06

- **Fix:** Headless SHOWDOWN no longer runs up to 300k sim steps in a single `tick_logic` call (that blocked the main thread: **Not Responding**, no UI refresh). Headless work is **chunked** so each frame returns to `display.flip` / the event loop (superseded by 0.5.17 time-budget + pump).

### 0.5.15 - 2026-04-06

- **SHOWDOWN — Headless mode:** On any SHOWDOWN setup screen, **H** toggles **Headless** (default off). When on, the run uses a **simulation clock** (`sim_time_s()` in `GameController`) instead of wall time; each frame advances a **chunk** of sim steps (`HEADLESS_STEPS_PER_FRAME` × **1/200 s** per step) so matches finish quickly without freezing the window. The main window shows a **dashboard** (match index, pairing, progress bar, sim time) instead of the playfield. **ESC** still cancels. Result file notes `Headless: yes/no` and that durations are simulated seconds when headless. See `run_headless_showdown_batch` / `format_headless_showdown_dashboard` in `tank_game/game_loop.py`.

### 0.5.14 - 2026-04-06

- **SHOWDOWN:** After choosing mode **3 — SHOWDOWN**, pick **schedule**: **Random pairings** (original: set total match count) or **Full skill matrix** — every AI-i vs AI-j (i,j in 0..9), with **N consecutive rounds per pairing** (100×N matches total). Confirmation screen shows total matches; **ESC** steps back. Summary file notes schedule type; matrix runs add a **P1×P2 head-to-head** table (P1 wins / games and %). Tests: `tests/test_showdown_matrix.py`.

### 0.5.13 - 2026-04-06

- **Fix:** Head-on shots on the same row/column could **pass through** each other: with one grid step per tick, two bullets in adjacent cells swap positions without ever sharing a cell, so the old “same `(x,y)`” shot–shot check missed them. Added detection for cardinal **head-on swap** using pre-step positions (`_step_start_x` / `_step_start_y`). Tests: `tests/test_shot_shot_collision.py`.

### 0.5.12 - 2026-04-06

- **Fix:** `_respawn_both_tanks` now calls `clear_from_board` **before** updating `tank.x` / `tank.y` to spawn. The previous order cleared the spawn cell (usually empty) and left **stale `TANK1`/`TANK2` cells** at old positions — invisible “walls”, and shots spawning one cell ahead could hit a ghost tank (logged as instant `SHOT_HIT` / self-kill). Regression test: `tests/test_game_logic.py::test_respawn_clears_old_tank_cells_from_board`.

### 0.5.11 - 2026-04-06

- **FR-8 risky fire (human parity):** Wild shots no longer use `direction_override` (sprite facing vs bullet direction). **`FireRisky`** now uses a two-tick flow: **`RISKY_PREPARE`** turns the tank to a random 8-way direction, next tick **`FIRE`** commits along that facing, then **`GameController._run_ai`** restores the saved barrel direction. Docs: `MECHANICS_TANK_PYGAME.md`, `docs/PRD_py_trees_integration.md` §11.

### 0.5.10 - 2026-04-06

- **Tests:** `tests/test_headless_demo_stuck_watch.py` — headless Demo harness with fake `get_ticks`, per-frame position history, **STUCK** (long same-cell run) and **OSCILLATE** (ABAB sliding-window count) warnings; optional `TANK_HEADLESS_FAIL_ON_WARN=1`.

### 0.5.9 - 2026-04-06

- **Tight-space oscillation:** Record tank position on every reaction decision; detect **A↔B↔A↔B** ping-pong. New BT leaf **`OscillationUnstick`** (runs before **ActiveEvasion**) issues **`RANDOM_MOVE`**, clears evasion / rim commit, and resets the ring — fixes long LEFT↔RIGHT / UP↻DOWN loops when chase+evade never reached stuck checks.

### 0.5.8 - 2026-04-06

- **HUD:** Restore **AI:n** skill labels in the status bar for **SHOWDOWN** (and **1P** for the CPU) — `render_frame` had only wired `difficulty_per_player` for **Demo**.

### 0.5.7 - 2026-04-06

- **Evasion oscillation (rim):** When primary horizontal “away” hits the **left/right wall**, `pick_evade_direction` now **commits** to the first successful **UP/DOWN** for the evasion session so chaser **row changes** no longer flip vertical escape every tick (fixes N/S face-to-face ping-pong in `tank_debug.log`). Cleared when evasion ends.

### 0.5.6 - 2026-04-06

- **PRD AI navigation:** `tank_game/ai_navigation.py` — `NavigationMemory` (skill-linear config, tabu edges, wall belief, peripheral vision ingest, decay, respawn soften, explosion invalidation), hybrid A* + greedy `get_movement_toward_target`, `pick_evade_direction`, `GameController._ai_attempt_move` records failed attempts; `_respawn_both_tanks` / `_explode_mine` hooks. Tests: `tests/test_ai_navigation.py`. Docs: `docs/PRD_AI_NAVIGATION.md` §11.

### 0.5.5 - 2026-04-06

- **Docs:** Added **`docs/PRD_AI_NAVIGATION.md`** — PRD for phased AI navigation memory (tabu → grid + hybrid pathing), strict fairness, `attempt_move` as source of truth with optional fallback, respawn confidence decay vs full reset on new map/`init_round`. Cross-link from **`MECHANICS_TANK_PYGAME.md`**.

### 0.5.4 - 2026-04-06

- **Corner oscillation (Demo/SHOWDOWN):** When blind to the enemy, **`ScanOrAdvance`** throttles **SCAN** decisions (~0.28s) so tanks in outside corners do not barrel-spin every reaction tick. **`get_movement_toward_target`** prefers **cardinals before diagonal** when near the playable rim to reduce diagonal wall-hugging. Tests: `tests/test_ai_behaviours.py`, `tests/test_corner_headless.py` (monotonic `get_ticks`).

### 0.5.3 - 2026-04-06

- **Evasion stuck (Demo/SHOWDOWN):** `MOVE_AWAY_FROM_ENEMY` now calls **`_ai_try_random_escape_move`** when cardinal/diagonal “away” steps are blocked (previously only logged). If still trapped, **`_is_evading`** is cleared so other behaviours can run. Evade direction uses **`enemy_pos` or `last_known_enemy_pos`**. Headless smoke: `tests/test_evasion_fallback.py`.

### 0.5.2 - 2026-04-06

- **`tank_game.main`:** Check for `py_trees` before importing the game stack; print install instructions (`pip install -e .`) and exit cleanly when dependencies are missing.
- **README:** Emphasize installing the package from the repo root so runtime deps are present.

### 0.5.1 - 2026-04-06

- **PRD §11 closed:** Documented resolutions for reaction gate, blackboard, legacy env, `py_trees` pin, FR-9 (geometric 90° cone), FR-8 (skip-fire hesitation + existing risky jitter).
- **FR-10:** `effective_min_shots_for_aimed()` = `confidence_threshold + ammo_discipline // 3`; wired into `_can_fire_confidently` and `_can_fire_risky`.
- **FR-8:** `should_skip_fire()` — low skill may skip aimed or risky shot this tick; legacy path uses skip for aimed fire when LOS would allow a shot.

### 0.5.0 - 2026-04-06

- **PRD S1:** Reaction gating implemented as a **py_trees** `ReactionTimeGate` decorator (`tank_game/ai_decorators.py`) wrapping the main selector; evasion cooldown for too-close contact uses **`EternalGuard`** around `EnemyPresenceTooClose`. **ActiveEvasion** leaf replaces the pre-tree evasion early return. Staging via `_stage_for_tree_tick` / `ReactionTimeGate` applies bind, perception, and frustration once the gate opens.

### 0.4.0 - 2026-04-06

- **PRD P3:** `tank_game/ai_utility.py` — `utility_snapshot()` for optional balance/tuning analysis (not used by the behaviour tree).
- **PRD S2:** When `TANK_DEBUG` is set, append behaviour-tree **tip** (`tree.tip()` name + status) to `tank_debug.log` after each AI tick.
- **FR-5:** Documented round-reset behaviour in `MECHANICS_TANK_PYGAME.md`.
- **`player_id`** passed from `GameController._run_ai` into `AIPlayer.update` / `_bind_tick_inputs` (blackboard contract §5.1).

### 0.3.0 - 2026-04-06

- **PRD P2 / §5.5:** Perception module `tank_game/ai_perception.py` (90° combat cone, peripheral tracking). `_sync_combat_perception` updates `last_known` / `_combat_enemy_pos` before the behaviour tree.
- **FR-11:** Demo/SHOWDOWN AI uses **per-player** `fast_mode=(difficulty >= 8)` and **per-skill** `reaction_time` when `move_delay` is set (no shared flattening).
- **FR-7:** Skill 0 never dodges; dodge curve adjusted in `get_ai_config`.
- **FR-10:** `confidence_threshold` / `ammo_discipline` — higher skill requires more reserve ammo before a disciplined shot.
- **FR-9:** `ScanOrAdvance` uses `scan_frequency` + difficulty to bias scanning when blind; combat aims only use forward-cone visibility.
- **FR-8:** `FireRisky` behaviour + `AIPlayer._pending_fire_risky`; `GameController._fire_shot(..., direction_override=)` for wild shots.
- **`aggressive_mines`** increases mine chance during too-close evasion.
- Tests: `test_ai_perception.py`; AI behaviour tests updated for perception sync.

### 0.2.0 - 2026-04-06

- **py_trees AI (PRD P1):** Tank decisions run through a `Selector` behaviour tree (`tank_game/ai_tree.py`, `tank_game/ai_behaviours.py`) with per-tick context on `AIPlayer` (`_bind_tick_inputs`). `GameController._run_ai` unchanged in effect.
- Added `tank_game/ai_blackboard.py` (`TankAITickContext` for documentation).
- `TANK_LEGACY_AI=1` restores the previous inline decision function for debugging.
- New tests: `tests/test_ai_behaviours.py`.
- Runtime dependency: `py_trees>=2.2.0`.
- Docs: PRD status updated; README + `MECHANICS_TANK_PYGAME.md` note the tree.

### 0.1.3 - 2026-04-06

- Added `docs/PYGAME_QA_TESTING_SPEC.md` (headless SDL, Ruff/mypy/Vulture/Hypothesis/VizTracer, event injection, layered testing).
- Added `tank_game/game_loop.py` with `tick_logic` / `render_frame`; `main` composes one frame with injectable `dt` via the clock.
- Added `tests/conftest.py` and `tests/test_headless_smoke.py`.
- `pyproject.toml`: dev tooling (ruff, mypy, vulture, hypothesis, viztracer) and tool sections; minor typing fixes for mypy (`ai.py`, `projectile.Shot`, `game.py` debug line).

### 0.1.2 - 2026-04-06

- Extended `docs/PRD_py_trees_integration.md` (v0.2): developer feedback on skill differentiation — dodge curve, risky shots, FOV/scan, ammo discipline, per-skill think time (§5.5, P2+).

### 0.1.1 - 2026-04-06

- Added `docs/PRD_py_trees_integration.md`: product requirements for integrating py_trees as the tank AI decision layer (blackboard, parity phases, acceptance criteria).

### 0.1.0 - 2026-03-17

- Initial Python 3 / PyGame TANK! project scaffolding.
- Added mechanics reference document for the TANK! port.
- Defined packaging metadata and development dependencies.
