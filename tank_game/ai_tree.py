"""Build the tank AI behaviour tree (PRD §5.2)."""

from __future__ import annotations

import os
from datetime import datetime
from typing import TYPE_CHECKING

from py_trees import composites, trees
from py_trees.decorators import EternalGuard

from .ai_behaviours import (
    ActiveEvasion,
    ChaseLastKnownOrUnstick,
    DodgeIncoming,
    EnemyPresenceTooClose,
    FireIfClearLos,
    FireRisky,
    FrustratedAdvance,
    OscillationUnstick,
    ScanOrAdvance,
    StuckWithoutTarget,
)
from .ai_decorators import ReactionTimeGate

if TYPE_CHECKING:
    from .ai import AIPlayer

_DEBUG_LOG_FILE = "tank_debug.log"
_DEBUG = os.environ.get("TANK_DEBUG", "").strip().lower() not in ("", "0", "false", "no", "off")


def log_behaviour_tree_tip(tree: trees.BehaviourTree, player_id: int | None) -> None:
    """Append one line with ``tree.tip()`` name + status when ``TANK_DEBUG`` is set (PRD S2)."""
    if not _DEBUG:
        return
    try:
        tip = tree.tip()
        if tip is None:
            return
        timestamp = datetime.now().strftime("%H:%M:%S.%f")[:-3]
        label = player_id if player_id is not None else "?"
        with open(_DEBUG_LOG_FILE, "a", encoding="utf-8") as f:
            f.write(f"[{timestamp}] BT tip P{label}: {tip.name} status={tip.status}\n")
    except OSError:
        pass


def build_ai_tree(ai: AIPlayer) -> trees.BehaviourTree:
    """Root: ``ReactionTimeGate`` (PRD S1) → selector matching legacy priority."""
    def evasion_cooldown_clear() -> bool:
        return ai._evasion_cooldown == 0

    too_close_guarded = EternalGuard(
        name="EvasionCooldownClear",
        condition=evasion_cooldown_clear,
        child=EnemyPresenceTooClose(ai, name="EnemyPresenceTooClose"),
    )
    inner = composites.Selector(
        "TankAI",
        memory=False,
        children=[
            OscillationUnstick(ai, name="OscillationUnstick"),
            ActiveEvasion(ai, name="ActiveEvasion"),
            FrustratedAdvance(ai, name="FrustratedAdvance"),
            DodgeIncoming(ai, name="DodgeIncoming"),
            too_close_guarded,
            FireIfClearLos(ai, name="FireIfClearLos"),
            FireRisky(ai, name="FireRisky"),
            ChaseLastKnownOrUnstick(ai, name="ChaseLastKnownOrUnstick"),
            StuckWithoutTarget(ai, name="StuckWithoutTarget"),
            ScanOrAdvance(ai, name="ScanOrAdvance"),
        ],
    )
    root = ReactionTimeGate(ai, inner)
    return trees.BehaviourTree(root)
