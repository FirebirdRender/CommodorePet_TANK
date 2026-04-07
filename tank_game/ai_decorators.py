"""py_trees decorators for reaction gating (PRD S1)."""

from __future__ import annotations

from collections.abc import Iterator
from typing import Any

from py_trees import behaviour, common
from py_trees.decorators import Decorator


class ReactionTimeGate(Decorator):
    """Blocks the subtree until the reaction interval has elapsed; commits ``_last_action_time`` on pass.

    On success, runs evasion bookkeeping (same as the previous pre-tree block), applies staged
    tick inputs, perception sync, and frustration tracking, then ticks the child selector.
    """

    def __init__(self, ai: Any, child: behaviour.Behaviour):
        super().__init__(name="ReactionTimeGate", child=child)
        self.ai = ai

    def tick(self) -> Iterator[behaviour.Behaviour]:
        self.logger.debug(f"{self.__class__.__name__}.tick()")
        if self.status != common.Status.RUNNING:
            self.initialise()
        ai = self.ai
        staged = ai._staged_update_args
        if staged is None:
            self.stop(common.Status.FAILURE)
            self.status = common.Status.FAILURE
            yield self
            return
        if ai._pending_decision_time - ai._last_action_time < ai.config.reaction_time:
            self.stop(common.Status.FAILURE)
            self.status = common.Status.FAILURE
            yield self
            return

        ai._last_action_time = ai._pending_decision_time
        ai._evasion_bookkeeping_after_reaction_commit()

        (
            board,
            my_pos,
            my_direction,
            my_shots,
            my_mines,
            enemy_pos,
            shots_on_board,
            player_id,
        ) = staged
        ai._bind_tick_inputs(
            ai._pending_decision_time,
            board,
            my_pos,
            my_direction,
            my_shots,
            my_mines,
            enemy_pos,
            shots_on_board,
            player_id=player_id,
        )
        ai._sync_combat_perception()
        if enemy_pos is None:
            ai._frustration_cycles += 1
        else:
            ai._frustration_cycles = 0
            ai._last_seen_enemy_time = ai._pending_decision_time

        yield from super().tick()

    def update(self) -> common.Status:
        return self.decorated.status
