# PRD: py_trees Integration for TANK! AI

| Field | Value |
|-------|--------|
| **Status** | **Complete** — P1 + P2 + P3; S1 + S2; §5.5 FR-7–FR-11 + §11 design decisions locked (see §11) |
| **Version** | 0.5.1 |
| **Last updated** | 2026-04-06 |
| **Owner** | TANK! PyGame port |
| **Depends on** | `tank_game.ai.AIPlayer`, `tank_game.game.GameController._run_ai` |

---

## 1. Executive summary

Integrate **[py_trees](https://github.com/splintered-reality/py_trees)** as the **decision layer** for tank AI: replace the monolithic priority ladder in `AIPlayer.update()` with **composable behavior-tree nodes** while preserving existing **physics, shooting, and game rules** in `GameController`. Goals: clearer structure, easier per-difficulty tuning, testable behaviors, and a path toward better **SHOWDOWN** skill separation—without LLMs or heavy compute.

---

## 2. Problem statement

### 2.1 Current state

- `AIPlayer.update()` implements a long **fixed if/else priority chain** (dodge → evasion → fire → chase → stuck → scan → default).
- `AIConfig` exposes several **unused or partially used** fields; difficulty scaling is **split** between `get_ai_config`, direct `self.difficulty` checks, and **game-mode overrides** (`fast_mode`, shared `move_delay`).
- Hard to **unit-test** individual behaviors; regressions require playtesting or large SHOWDOWN batches.

### 2.2 Desired state

- **Explicit** behavior hierarchy visible as a tree (and optionally **ASCII/dot** output for debugging).
- **Shared blackboard** for world facts and decisions, separating **sense** (what we know) from **act** (what `GameController` executes).
- **Incremental migration**: legacy path remains available until parity is proven.

---

## 3. Goals & non-goals

### 3.1 Goals (must have)

| ID | Goal |
|----|------|
| G1 | **Parity**: Default tree reproduces current AI behavior within agreed tolerance (see §8). |
| G2 | **Integration**: `GameController._run_ai` still applies actions via existing `_fire_shot`, `attempt_move`, mine placement, etc. |
| G3 | **Dependency**: Add `py_trees` as a normal **runtime** dependency in `pyproject.toml`. |
| G4 | **Tests**: New unit tests for critical **behaviors** (dodge branch, fire branch, stuck branch) using mocked blackboard + board. |
| G5 | **Difficulty**: `difficulty` (0–9) and existing `AIConfig` **inputs** remain the primary tuning knobs; tree shape may branch on difficulty only where documented. |

### 3.2 Goals (should have)

| ID | Goal |
|----|------|
| S1 | **Decorators** for reaction gating and dodge/evasion cooldowns (replacing ad-hoc counters where practical). |
| S2 | **Logging hook**: optional debug line per tick: selected leaf name + status (behind `TANK_DEBUG` or similar). |
| S3 | **Documentation**: Short “how to add a behavior node” section in README or `MECHANICS_TANK_PYGAME.md`. |

**Should-have status:** S3 **done** (README + mechanics). S1 **done** — `ReactionTimeGate` (`ai_decorators.py`) wraps the main selector; `EternalGuard` gates `EnemyPresenceTooClose` on evasion cooldown; `ActiveEvasion` leaf covers sustained evasion moves. S2 **done** (`TANK_DEBUG` logs action + risky flag in `_run_ai`, plus **behaviour-tree tip** line via `tree.tip()` in `tank_debug.log`).

### 3.3 Goals — skill differentiation (P2+; developer feedback)

These **change gameplay** vs the current AI. They ship **after** P1 structural parity (or behind a **skill-v2** flag) so SHOWDOWN baselines stay interpretable.

| ID | Goal |
|----|------|
| D1 | **Dodge**: Probability of attempting dodge scales with skill; **skill 0 never attempts** dodge (`DODGE_SHOT`). |
| D2 | **Shoot**: Lower skill takes **riskier / less accurate** shots (implementation: e.g. intentional miss bias, wider effective cone, or fire under worse LOS — TBD). |
| D3 | **Scan & visibility**: Narrow **perception** to **forward-facing ~90°** for acquiring/tracking targets; **stop, rotate-in-place scan** to search; **higher** skill → **higher** propensity to scan when the enemy is **not** currently known. |
| D4 | **Ammo discipline**: **Zero** baseline discipline at skill **0**; **increases** monotonically with skill (conservative ammo use or stricter fire rules at high skill — exact curve TBD). |
| D5 | **Thinking speed**: **`reaction_time` / “think” cadence** scales **per AI** from that agent’s **skill level**, not a **shared** value for both tanks (fixes equal `fast_mode` / shared `move_delay` flattening SHOWDOWN). |

### 3.4 Non-goals (out of scope for this PRD)

- **No** ML / LLM integration.
- **No** mandatory change to rendering, audio, or board generation in phase 1.
- **No** requirement to remove **all** legacy `AIPlayer` helpers (`_has_forward_los`, etc.)—they may be **called from leaves** initially.
- **No** commitment to JSON/XML-authored trees in v1 (Python-built trees only unless we explicitly add later).

---

## 4. Stakeholders & users

| Role | Interest |
|------|----------|
| **Player** | Indistinguishable or better gameplay vs current AI; fair difficulty curve. |
| **Developer / maintainer** | Safer changes, clearer bugs, faster iteration on SHOWDOWN balance. |
| **Future contributor** | Obvious place to add behaviors without editing a 400-line function. |

---

## 5. Functional requirements

### 5.1 Blackboard contract

A **single blackboard namespace** (e.g. `py_trees.blackboard.Client` namespaced keys or a dedicated `TankAIBlackboard` dataclass synced each tick) must be populated **before** `tree.tick()`:

| Key / field | Source | Notes |
|-------------|--------|--------|
| `current_time` | `pygame` time | Seconds, same as today |
| `player_id` | `_run_ai` | 1 or 2 |
| `my_pos`, `my_direction` | `Tank` | |
| `my_shots`, `my_mines` | `Tank` | |
| `enemy_pos` | Enemy tank or `None` | Same semantics as today |
| `shots_on_board` | `GameController.shots` | Read-only reference or copy |
| `board` | `Board` | Read-only for raycast / passability |
| `config` | `AIConfig` | From `get_ai_config` |
| `difficulty` | `int` | 0–9 |
| `enemy_start_pos` | AI init | For frustration / advance |
| **Output** | | |
| `pending_action` | `AIAction \| None` | What legacy code expects; `None` = no decision this tick (reaction gate) |

**FR-1:** If `pending_action` is `None`, `_run_ai` performs **no** action (same as current early return from `update()`).

**FR-2:** If `pending_action` is set, `_run_ai` executes the **same** `if/elif` dispatch as today (`FIRE`, `MOVE_TOWARD_ENEMY`, etc.).

### 5.2 Behavior tree topology (v1 — parity)

**FR-3:** Root is a **Selector** whose children are ordered to match **current** priority:

1. **ReactionTimeGate** (decorator): if `current_time - last_decision_time < reaction_time` → FAILURE and subtree not ticked (`tank_game/ai_decorators.py`).
2. **DodgeIncoming** — maps to `_check_dodge_shot` logic.
3. **EvasionSequence** — `_is_evading` / too-close evasion + optional mine.
4. **FireIfLos** — forward LOS + `_can_fire_confidently`.
5. **ChaseOrUnstick** — last known enemy, stuck → random move.
6. **ScanOrAdvance** — difficulty ≥ 6 → SCAN else move toward spawn.

**FR-4:** **Frustration** behavior (cycles without enemy) must be represented as a **condition + action** node pair or subtree, equivalent to current `frustration_threshold` logic.

### 5.3 State that lives on `AIPlayer` vs tree

| State | Location | Rationale |
|-------|----------|-----------|
| `last_known_enemy_pos`, `mine_memory`, `_scan_direction` | `AIPlayer` or blackboard | Persistent across ticks |
| `_is_evading`, `_evasion_end_time`, `_evasion_cooldown` | Prefer **explicit nodes/decorators** over scattered ints | Easier to reset between rounds |
| `_last_action_time` | Keep on `AIPlayer` or decorator | Aligns with reaction gate |

**FR-5:** `init_round` / new match must **reset** tree-internal and AI state consistently (document list).

### 5.4 Game modes

**FR-6:** **1P / Demo / SHOWDOWN** must use the **same** tree **unless** we document a **mode flag** on the blackboard that switches subtree (e.g. only in phase 2).

### 5.5 Skill differentiation (P2+ — developer requirements)

These refine **behavior** and **SHOWDOWN** ladder quality; they are **not** required for P1 parity.

| FR | Requirement | Notes |
|----|-------------|--------|
| **FR-7 Dodge** | At **difficulty 0**, the dodge branch is **unreachable** (probability **0** or node omitted). For **difficulty ≥ 1**, probability of **considering** dodge scales with skill (e.g. map to `dodge_chance` curve — must be **monotonic** non-decreasing where possible). | Aligns with `DodgeIncoming` leaf + config. |
| **FR-8 Shoot accuracy / risk** | Low skill **accepts** higher-risk shots: e.g. fire without ideal alignment, add angular/spread error in execution layer, or lower “effective accuracy” before pulling trigger. High skill **restricts** fire to better opportunities (pairs with FR-9). | May require **`GameController._fire_shot`** or tank facing to accept an optional **noise** term; tree sets intent, executor applies error. |
| **FR-9 Scan & field of view** | **Target acquisition** outside a **forward ~90°** arc is **not** treated as “known line-of-sight” for shooting/chase (enemy may be **unknown** for targeting even if geometrically visible — perception model). **SCAN** = **no translation** for that tick; **rotate** through directions (existing `get_scan_direction` cycle or extended). **Frequency / priority** of scan when `enemy_pos` is **unknown** increases with **difficulty**. | Implements real use of **`scan_frequency`** / **`peripheral_radius`** semantics; may replace oracle `enemy_pos` for combat decisions. |
| **FR-10 Ammo discipline** | Define **`ammo_discipline`** (or equivalent) with value **0** at skill **0** and **strictly increasing** discipline with skill (e.g. higher minimum reserve before committing to a shot, or stricter “good shot” gate). | Revisit current `confidence_threshold` formula — today it **increases** aggression at high skill; this requirement may **invert or replace** that curve for “discipline” semantics. |
| **FR-11 Per-agent think time** | **`reaction_time`** (and any **fast_mode** analogue) is derived **per `AIPlayer`** from **`difficulty`** (and mode), **not** identical for both agents because of shared `move_delay` or single `fast_mode` flag. | **GameController** / `get_ai_config` must pass **per-player** config into each tree instance. |

**Design note:** FR-7–FR-11 imply a **perception blackboard**: `enemy_known_for_combat: bool`, `last_seen_direction`, optional **FOV cone** checks in a small helper module (`ai_perception.py`) used by leaves.

---

## 6. Non-functional requirements

| ID | Requirement |
|----|----------------|
| N1 | **Performance**: One `tree.tick()` per AI per frame budget **&lt; 1 ms** typical on target hardware (same class as current Python AI). |
| N2 | **Dependency**: Pin **`py_trees`** to a **minimum version** compatible with Python 3.10+; document in `pyproject.toml`. |
| N3 | **Failure handling**: Node failures must not crash the game loop; root should always resolve to a safe fallback (e.g. advance toward spawn). |

---

## 7. Architecture overview

```
GameController._run_ai
    │
    ├─ Stage tick args on AIPlayer → ai.tree.tick()
    ├─ ReactionTimeGate (py_trees decorator) → bind + perception + frustration → Selector
    └─ Read AIPlayer._pending_action → existing action execution
```

- **Leaves** either return SUCCESS and set `pending_action`, or FAILURE so the Selector tries the next sibling.
- **RUNNING** reserved for future multi-tick behaviors (optional in v1).

---

## 8. Acceptance criteria & parity

| Check | Method |
|-------|--------|
| **Unit** | Tests for individual nodes with fixed blackboard + mock `Board` / shots |
| **Regression** | Run SHOWDOWN (e.g. 500+ matches) comparing win-rate **distribution** vs baseline commit; **no large unexplained drift** (threshold TBD, e.g. ±2% per skill bucket) |
| **Manual** | 1P / 2P / Demo spot-check: no obvious stuck loops, firing, mines |

**Parity caveat:** Exact RNG alignment across runs may differ if tick order changes; acceptance should allow **statistical** parity, not bit-identical logs.

---

## 9. Phased delivery

| Phase | Deliverable | Exit |
|-------|-------------|------|
| **P0** | Spike: 3-node tree + blackboard + one leaf calling existing helper | Runs in dev branch |
| **P1** | Full parity tree + remove monolithic `update()` body | Tests green + SHOWDOWN check |
| **P2** | **Done:** §5.5 — `ai_perception`, per-agent `fast_mode` + `reaction_time` (FR-11), dodge@0 (FR-7), ammo discipline + `ammo_discipline` field (FR-10), FOV + `scan_frequency` scan bias (FR-9), risky fire via **`RISKY_PREPARE`** + aim restore (FR-8, human-parity — no `direction_override`); `aggressive_mines` wired | See `CHANGELOG` 0.3.0 / 0.5.11 |
| **P3** | **Done:** `tank_game/ai_utility.py` — `utility_snapshot(ai)` (read-only coarse scores for tooling; not wired into the tree) | See `CHANGELOG` 0.4.0 |

---

## 10. Risks & mitigations

| Risk | Mitigation |
|------|------------|
| py_trees **status API** learning curve | Spike + read upstream docs; keep wrapper API thin |
| **Over-engineering** | P0/P1 strictly parity-first; **§5.5** gameplay changes only in P2+ |
| **Blackboard drift** | Single module defining keys (`tank_game/ai_blackboard.py`) |
| **SHOWDOWN still flat** | Address **FR-11** + **§5.5** in P2; measure with conditional win matrices |
| **Inverted ammo curves** | FR-10 may conflict with legacy `confidence_threshold` — document migration in P2 |

---

## 11. Open questions — **resolved** (2026-04-06)

| # | Topic | Decision |
|---|--------|----------|
| 1 | Reaction gate | **`ReactionTimeGate`** decorator (`tank_game/ai_decorators.py`) wrapping the main selector — not `Duration`/`Timer`. |
| 2 | Blackboard | **Manual**: tick fields on `AIPlayer` + `_stage_for_tree_tick` / `_bind_tick_inputs`. **`TankAITickContext`** documents the contract. *Optional future:* migrate read/write to **`py_trees.blackboard.Client`** if we need cross-agent sharing or tooling. |
| 3 | Legacy flag | Keep **`TANK_LEGACY_AI=1`** for the pre-tree decision path. |
| 4 | `py_trees` version | **`py_trees>=2.2.0`** in `pyproject.toml` (minimum compatible with Python 3.10+). |
| 5 | **FR-8** risky / low skill | **Primary:** occasional **skip-fire** (hesitation) via `AIPlayer.should_skip_fire("aimed" \| "risky")` in `FireIfClearLos` / `FireRisky`. **Also:** when a risky shot commits, **`RISKY_PREPARE`** aims to a random 8-way direction, next tick **`FIRE`** uses that facing (no override), then barrel direction is restored — same visibility rules as a human turning to shoot. |
| 6 | **FR-9** FOV | **Literal ~90° geometric cone** (`enemy_in_forward_cone` dot/acos in `ai_perception.py`). Not the 4-direction-only approximation. |
| 7 | **FR-10** discipline | **Two inputs, one effective bar:** `effective_min_shots_for_aimed()` = `confidence_threshold + (ammo_discipline // 3)`; used by `_can_fire_confidently` and `_can_fire_risky`. |

---

## 12. Documentation & versioning

- Update **README** “Development / Architecture” with one paragraph + link to this PRD.
- **CHANGELOG** entry when implementation lands.
- **SemVer**: Implementation likely **minor** (new dependency + feature); doc-only PRD may be **patch**.

---

## Appendix A: Mapping current `AIPlayer` methods → candidate nodes

| Region | Current helper | Candidate node name |
|--------|----------------|---------------------|
| Dodge | `_check_dodge_shot` | `DodgeIncoming` |
| LOS | `_has_forward_los`, `_can_fire_confidently` | `FireIfClearLos` |
| Move | `get_movement_toward_target`, `avoid_own_mines` | Used by action executor (unchanged) or `MoveTowardTarget` leaf |
| Stuck | `_check_stuck`, `_update_position_history` | `UnstickRandom` |
| Scan | `get_scan_direction` | `ScanBarrel` |
| Mines | `remember_mine`, `update_mine_memory`, `avoid_own_mines` | Side effects remain in executor + `update_mine_memory` after tick |

---

## Appendix B: References

- [py_trees documentation](https://py-trees.readthedocs.io/)
- Internal: `tank_game/ai.py`, `tank_game/game.py` (`_run_ai`)
- **P2 wired:** `peripheral_radius`, `scan_frequency`, and `aggressive_mines` are used in perception / evasion / scan behaviour (see `ai_perception.py`, `get_ai_config`, `ai_behaviours.py`).
- **Testing / QA:** `docs/PYGAME_QA_TESTING_SPEC.md` — headless SDL, Ruff/mypy, `tick_logic` vs `render_frame`, event injection

---

## Revision history

| Version | Date | Changes |
|---------|------|---------|
| 0.5.1 | 2026-04-06 | §11 resolved; FR-8 skip-fire; FR-10 effective min shots; PRD table |
| 0.5 | 2026-04-06 | **S1** — `ReactionTimeGate`, `EternalGuard` on too-close evasion, `ActiveEvasion` leaf; staging + `CHANGELOG` 0.5.0 |
| 0.4 | 2026-04-06 | P3 `ai_utility`; S2 BT tip logging; FR-5 reset list in mechanics; blackboard `player_id` on tick |
| 0.3 | 2026-04-06 | Mark **P2 complete** (§5.5); perception + risky fire + per-agent think |
| 0.2 | 2026-04-06 | P1 **implemented**: `ai_tree`, `ai_behaviours`, blackboard tick context, `TANK_LEGACY_AI`, tests |
| 0.1 | 2026-04-06 | Added §3.3 / §5.5 developer feedback; P2 scope; open questions |
| 0.0 | 2026-04-06 | Initial draft |
