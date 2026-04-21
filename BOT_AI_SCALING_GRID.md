# TANK! Bot AI Scaling Grid

This document maps the `SkillConfig` parameters (defined in `cmd/bot-go/skill_config.go`) to each bot difficulty level (1-10), along with the empirical higher-skill win rates observed in a 500-match headless tournament.

---

## Skill Definitions

| Skill | Description | First Available |
|-------|-------------|-----------------|
| **Thinking Delay** | Ticks of artificial hesitation before evaluating the next action. Lower = faster reactions. | d=1 (54 ticks) → d=10 (0 ticks) |
| **Min Shots For Fire** | Minimum ammo reserve to allow a shot. `1` = fire freely; `2` = conserve; `0` = no limit at d=10. | d=1 (1) → d=5 (2) → d=10 (0) |
| **Detection Range** | Percentage of max projectile range used for enemy detection and firing decisions. | d=1 (50%) → d=10 (100%) |
| **Diagonal Firing** | Can fire along 45° diagonals when aligned with the enemy. All difficulties have this; it is engine-level behavior, not skill-gated. | d=1 (always) |
| **Self-Destruct Awareness** | Avoids firing the last shot unless in a guaranteed-kill position (adjacent + aimed). | d=5 |
| **Wall-Shooting Unstick** | Detects when stuck (no movement for N ticks) and fires at adjacent walls to clear a path. | d=4 |
| **Wall-Shooting Hunt** | Destroys walls blocking line-of-sight to the enemy to open firing lanes. | d=6 |
| **Protectile Detection** | Senses incoming enemy projectiles within a forward cone and dodges perpendicular. | d=1 (scalar=2) → d=10 (scalar=8) |
| **Evasion** | Active perpendicular dodging when enemy is detected in firing alignment. | d=7 |
| **Engage Range** | Close-range aggression threshold (cells). Non-zero means the bot actively pushes when close. | d=7 (4) → d=10 (3) |

---

## Parameter Grid

```
Difficulty | Delay | MinShots | Detect% | Protec | Destruct | WallUnstick | WallHunt | Evasion | Engage
-----------|-------|----------|---------|--------|----------|-------------|----------|---------|-------
    1      |  54   |    1     |   50%   |   2    |    .     |      .      |    .     |    .    |   0
    2      |  48   |    1     |   50%   |   2    |    .     |      .      |    .     |    .    |   0
    3      |  42   |    1     |   55%   |   4    |    .     |      .      |    .     |    .    |   0
    4      |  36   |    1     |   60%   |   4    |    .     |      X      |    .     |    .    |   0
    5      |  30   |    2     |   65%   |   6    |    X     |      X      |    .     |    .    |   0
    6      |  24   |    2     |   70%   |   6    |    X     |      X      |    X     |    .    |   0
    7      |  18   |    2     |   80%   |   7    |    X     |      X      |    X     |    X    |   4
    8      |  12   |    2     |   90%   |   7    |    X     |      X      |    X     |    X    |   4
    9      |   6   |    2     |   95%   |   8    |    X     |      X      |    X     |    X    |   3
   10      |   0   |    0     |  100%   |   8    |    X     |      X      |    X     |    X    |   3
```

**Legend:** `.` = disabled / `X` = enabled

---

## Empirical Win Rates (500-match tournament)

| Difficulty | Matches | Passed | Stuck (DRAW) | Higher-Skill Win Rate | Notes |
|-----------|---------|--------|-------------|----------------------|-------|
| d=1 | 50 | 47 | 3 (6%) | **63.8%** | Lots of chaos; low skills still win occasionally |
| d=2 | 50 | 42 | 8 (16%) | **71.4%** | Thinking delay reduction starts to matter |
| d=3 | 50 | 45 | 5 (10%) | **68.9%** | Detection range bump helps |
| d=4 | 50 | 44 | 6 (12%) | **70.5%** | Wall-unstick prevents spawn traps |
| d=5 | 50 | 43 | 7 (14%) | **79.1%** | Ammo conservation + destruct awareness = peak differentiation |
| d=6 | 50 | 43 | 7 (14%) | **79.1%** | Wall-hunting opens firing lanes |
| d=7 | 50 | 46 | 4 (8%) | **71.7%** | Evasion introduced; survival increases |
| d=8 | 50 | 40 | 10 (20%) | **72.5%** | Stuck rate jumps — two evasive bots kite |
| d=9 | 50 | 40 | 10 (20%) | **77.5%** | Near-max detection; fewer surprises |
| d=10 | 50 | 37 | 13 (26%) | **78.4%** | Highest stuck rate; two defensive masters |

---

## Key Findings

### 1. Peak Differentiation at d=5-6
The highest higher-skill win rates (~79%) occur at **difficulty 5 and 6**. This is where:
- Ammo conservation (`MinShotsForFire: 2`) forces disciplined play
- Self-destruct awareness prevents cheap trades
- Wall-unstick eliminates spawn bad-luck
- Wall-hunt opens lanes on dense maps
- But **evasion is NOT yet enabled** — so aggressive bots can still close and finish

### 2. Evasion Causes Stalemates at d=8-10
When **both** bots have evasion + protectile detection, they spend more effort dodging than attacking. At d=10, **26% of matches** hit the 300-second engine timeout. This is a ceiling effect: the bots are too good at not dying.

### 3. The "Skill Ladder" Is Real But Modest
Across all 500 matches, higher-skill bots won approximately **74%** of decided games. This means:
- A 2-skill advantage is meaningful but not overwhelming
- Map layout, spawn distance, and starting ammo still matter (~25% variance)
- The ladder is most visible at d=5-7; at d=1, chaos dominates

### 4. Thinking Delay Has Diminishing Returns
Thinking delay drops from 54 ticks (d=1) to 0 (d=10). The biggest jumps in win rate happen when delay drops **below 30 ticks** (d=5), where the bot can react to enemy movements in real time. Below 12 ticks (d=8+), the difference is marginal.

---

## Protectile Detection Scaling

The `ProtectileScalar` determines how aggressively a bot reacts to incoming projectiles. Higher scalar = larger safety margin.

| Difficulty | Scalar | Behavior |
|-----------|--------|----------|
| d=1-2 | 2 | Minimal dodge; mostly ignores shots unless very close |
| d=3-4 | 4 | Reacts to shots within ~4 cells of trajectory |
| d=5-6 | 6 | Solid mid-range dodge; respects crossfire |
| d=7-8 | 7 | Proactive dodge; begins lateral movement early |
| d=9-10 | 8 | Maximum dodge; treats any incoming shot as urgent |

---

## Tuning Recommendations for Test Harness

If running large tournaments (e.g., 500+ matches), consider:

1. **Reduce engine `MaxMatchTicks`** from 18000 (5 min) to ~9000 (2.5 min). At d=8-10, legitimate stalemates are common, but 5 min per match is excessive.
2. **Cap concurrency at 8-10** per test instance. The server is single-threaded for match logic; beyond 10 parallel matches, spawn/join latency increases.
3. **Log stuck matches separately** to analyze whether they correlate with specific map seeds or skill pairings (e.g., d=10 vs d=10 with equal skills).

---

*Generated from `cmd/bot-go/skill_config.go` and empirical 500-match headless tournament data.*
*Last updated: 2026-04-20 (fixed diagonal firing note — engine-level, always available)*
