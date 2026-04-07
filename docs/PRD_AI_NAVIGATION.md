# PRD: AI internal map & navigation memory

**Status:** Implemented (`tank_game/ai_navigation.py`, wired in `AIPlayer` + `GameController`)  
**Product:** TANK! PyGame port (`tank_game`)  
**Related:** `docs/PRD_py_trees_integration.md`, `MECHANICS_TANK_PYGAME.md` § AI

---

## 1. Problem statement

AI tanks can **oscillate** near corners and **repeat** movement intents that physics has already shown are blocked. Decisions today are largely **myopic** (per-tick heuristics) and do not **persist** “this step failed” or “this region looks like static terrain” in a way that scales with **skill**.

---

## 2. Goals & non-goals

### Goals

- **G1:** Reduce repeated failed moves and corner **ping-pong** without granting unfair omniscience.
- **G2:** Improve navigation around **static** obstacles using **remembered** structure plus **vision**.
- **G3:** Encode **skill** as sensor quality, memory, decay, and planning limits (**linear** scaling per knob — see §6).

### Non-goals (v1 / unless added later)

- Perfect optimal play or full information about the opponent’s plan.
- Changing human input rules or shared movement physics.

---

## 3. Requestor decisions (locked)

| ID | Decision | Choice |
|----|----------|--------|
| **D1** | **MVP scope** | **Phased:** Milestone 1 = **tabu / failed-edge** memory; Milestone 2 = **sparse grid** + consumption by movement; path strategy per D8. |
| **D2** | **Map updates** | **Both** **failed moves** and **vision / LOS** (per existing perception cones where applicable). |
| **D3** | **Representation** | **Both** **directed edges** (short-term / fine-grained) **and** **cells** (longer-lived static belief). |
| **D4** | **Reset / decay / confidence** | See §4 (respawn vs new map; regional invalidate; decay). |
| **D5** | **Failed-move recording** | **`Tank.attempt_move` (or equivalent) is source of truth (B).** Optional **fallback inference (C)** only for code paths that do not use `attempt_move` — avoid double counting. |
| **D6** | **Consumers** | **All AI-chosen movement directions** (chase, evade, unstick, advance toward spawn, etc.). **Exclude** pure barrel / SCAN-only actions from “movement memory” writes (they do not imply a blocked translation). |
| **D7** | **Fairness** | **Strict:** internal state only from **observed** facts and **own failed translation attempts** — no reading hidden `Board` truth in release builds. |
| **D8** | **Path algorithm** | **Hybrid:** **search** (e.g. bounded A* / Dijkstra) when **far** or **blind** relative to target; **greedy** with penalties when **close** (thresholds TBD in implementation). |
| **D9** | **Skill curve** | **Linear** interpolation of navigation parameters across difficulty **0–9**. |
| **D10** | **Doc location** | This file: **`docs/PRD_AI_NAVIGATION.md`**. |

---

## 4. Lifecycle: reset, respawn, decay, and confidence

### 4.1 Full reset — “new game” / new terrain

A **full reset** of navigation memory runs when the **playfield is regenerated**: new **terrain layout** (new `Board` generation), i.e. the same class of event as **starting a fresh game** where the map is redrawn, tank/ammo budgets re-established from menu flow, etc.

**In current code, `GameController.init_round()` allocates a new `Board()` and new `AIPlayer` instances — treat this as a full navigation reset boundary** (each Demo entry / SHOWDOWN match start that goes through `init_round`).

**Requirement NAV-LIFE-1:** On full reset, discard navigation structures or reinitialize to empty; do not carry belief across unrelated maps.

### 4.2 Respawn — memory persists, confidence drops

**Requirement NAV-LIFE-2:** **Tank respawn** (kill → respawn **without** `init_round`, same `Board`) **must not** wipe navigation memory.

**Requirement NAV-LIFE-3:** On respawn, **do not** clear the map; instead **reduce confidence** in stored beliefs (implementation: global factor, per-cell decay, or mixed) so the AI **revalidates** areas after possible **projectile ↔ terrain** interactions (walls damaged/altered if applicable, explosions, etc.).

*Rationale:* While the tank was “out,” the world may have changed; memory should be **softer**, not absent.

### 4.3 Ongoing decay & regional invalidation

**Requirement NAV-LIFE-4:** Apply **time- or tick-based decay** so stale beliefs fade.

**Requirement NAV-LIFE-5:** On **explosions** or **known terrain change events** in a region, **invalidate or down-weight** memory for **affected cells/edges** (radius TBD), without necessarily full-map reset.

*Note:* Exact hooks depend on existing game events (mine/shot/wall interactions); list integration points during implementation.

---

## 5. Functional requirements

| ID | Requirement |
|----|-------------|
| **NAV-1** | Maintain per-AI **navigation state** separate from authoritative `Board` (belief / tabu / grid — phased). |
| **NAV-2** | **Ingest** failed translation attempts from **`attempt_move` outcome** (primary). Optional **fallback** when position unchanged after an intended move without a clean hook (documented, low frequency). |
| **NAV-3** | **Ingest** **visible** cells from **vision/LOS** rules consistent with FR-9 / perception (no extra omniscience). |
| **NAV-4** | **Emit** candidate directions or waypoints for **`get_movement_toward_target`** (and equivalent paths for evasion / random escape) so **all** AI translation choices consult memory per **D6**. |
| **NAV-5** | **Skill 0–9** linearly scales configurable knobs (sight radius, decay rate, tabu size, search budget, noise optional). |
| **NAV-6** | Milestone 1 ships **tabu**; Milestone 2 adds **grid + hybrid pathing** per **D8**. |

---

## 6. Skill model (parameters to implement)

All scalars below are **linear** in difficulty **0–9** unless noted.

| Parameter | Description |
|-----------|-------------|
| **Sight budget** | How much of the map is updated per tick / scan from LOS. |
| **Memory capacity** | Max tabu entries / grid resolution cap. |
| **Decay rate** | How fast edges/cells revert toward unknown or lower confidence. |
| **Respawn confidence multiplier** | Applied on respawn (NAV-LIFE-3). |
| **Search budget** | Max nodes expanded for A* / Dijkstra in hybrid mode. |
| **Greedy proximity threshold** | Distance (cells) below which hybrid mode uses greedy only. |

Exact formulas and defaults are **implementation tasks**; this PRD only locks **linear** scaling and **semantic** meaning.

---

## 7. Integration points (implementation guide)

| Area | Responsibility |
|------|------------------|
| **`Tank.attempt_move`** | Return or surface **success/failure** and **intended direction** so **`GameController` / `AIPlayer`** can record **blocked edge** without guessing. |
| **`AIPlayer`** | Own or reference **`NavigationMemory`** (name TBD); bind updates during tick; expose **`suggest_directions(board, goal, context)`** or augment **`get_movement_toward_target`**. |
| **Behaviour tree leaves** | No change to **priority order** required by default; movement **quality** improves via shared helper. |
| **`init_round` / new game** | Call **`navigation_memory.reset_full()`**. |
| **`_respawn_both_tanks`** | Call **`navigation_memory.on_respawn()`** (confidence drop, no full clear). |

---

## 8. Acceptance criteria

- **AC1:** After Milestone 1, metrics or logs show **reduced** repeat rate of the same **(cell, direction)** failure within a sliding window (baseline vs after — threshold set during tuning).
- **AC2:** After Milestone 2, blind / long-range paths **prefer** remembered corridors over repeatedly bumping the same wall segment.
- **AC3:** **Strict fairness (D7):** no release path uses ground-truth `Board` for cells the AI has never observed or inferred from failure.
- **AC4:** Respawn **does not** clear memory; **confidence** drops per NAV-LIFE-3; full reset only on new map / `init_round` boundary.

---

## 9. Phasing

| Milestone | Deliverable |
|-----------|-------------|
| **M1** | Tabu / directed-edge failure memory + consumption in movement helpers; hooks from `attempt_move`; respawn confidence + decay stubs; full reset on `init_round`. |
| **M2** | Sparse cell belief + vision ingestion; hybrid search vs greedy; regional invalidation hooks. |

---

## 10. Open items (for implementation tickets)

- Exact **confidence** model (scalar per cell vs per edge vs Bayesian-lite).
- **Hybrid** thresholds (distance to target, “blind” definition).
- **SHOWDOWN** / Demo: confirm each match’s `init_round` is the only full reset needed for tournament flow.
- Unit tests: pure navigation module + headless **fake time** patterns (see `tests/test_corner_headless.py` precedent).

---

## 11. Implementation notes (code)

- **`NavigationMemory`** (`tank_game/ai_navigation.py`): tabu `(x, y, Direction)` with TTL; `wall_belief` per cell; `get_peripheral_vision` for sight; bounded A* for hybrid leg; strict static blocking uses belief + observed cells only (unknown static is not read from `Board` for *lookahead* blocking—`_can_move` still enforces physics on execution).
- **`GameController._ai_attempt_move`**: on `attempt_move` failure, `record_failed_attempt` (wall type → belief).
- **`_respawn_both_tanks`**: `navigation.on_respawn(t)`; **`_explode_mine`**: `invalidate_region` for AI maps.
- **`AIPlayer`**: `observe_tick` from `_bind_tick_inputs`; `get_movement_toward_target(..., blind=..., current_time=...)`; `pick_evade_direction` (greedy + tabu).
- **Tests:** `tests/test_ai_navigation.py`, behaviour tests updated for greedy-distance short legs.

## 12. Revision history

| Date | Change |
|------|--------|
| 2026-04-06 | Initial PRD from Q1–Q10; NAV-LIFE-2/3 refined (respawn confidence, full reset on new map only); Q5 B primary, C fallback. |
| 2026-04-06 | Implementation landed: M1+M2 navigation module, game hooks, tests. |
