"""Optional utility scores for tuning and tooling (PRD P3).

These values are **not** consumed by the behaviour tree; use them for analysis,
SHOWDOWN dashboards, or future utility-based nodes.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from .ai import AIPlayer


def utility_snapshot(ai: AIPlayer) -> dict[str, float]:
    """Coarse 0..1 (or comparable) scores derived from current ``AIPlayer`` state."""
    cfg = ai.config
    frustration_norm = min(1.0, ai._frustration_cycles / 80.0)
    return {
        "frustration_norm": frustration_norm,
        "evasion_active": 1.0 if ai._is_evading else 0.0,
        "dodge_skill": cfg.dodge_chance,
        "ammo_discipline_norm": cfg.ammo_discipline / 9.0,
        "reaction_norm": min(1.0, cfg.reaction_time / 2.0),
    }
