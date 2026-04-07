"""py_trees behaviour nodes for tank AI (PRD §5.2)."""

from __future__ import annotations

import random
from typing import Any

from py_trees import behaviour, common

from .ai import AIAction
from .player import Direction


class _SetsPendingAction(behaviour.Behaviour):
    """Base: write pending action on success."""

    def __init__(self, ai: Any, name: str) -> None:
        super().__init__(name=name)
        self.ai = ai

    def _set_action(self, action: AIAction) -> None:
        self.ai._pending_action = action


class OscillationUnstick(_SetsPendingAction):
    """Tight 1-wide corridors / rim evasion: A↔B ping-pong is invisible to Chase leaves while ActiveEvasion runs first."""

    def update(self) -> common.Status:
        if self.ai._oscillation_abab_detected():
            self.ai._is_evading = False
            self.ai._evade_vertical_commit = None
            self.ai._recent_positions.clear()
            self.ai.clear_decision_position_ring()
            self._set_action(AIAction.RANDOM_MOVE)
            self.feedback_message = "oscillation unstick"
            return common.Status.SUCCESS
        self.feedback_message = "no abab oscillation"
        return common.Status.FAILURE


class ActiveEvasion(_SetsPendingAction):
    """While evasion mode is active, keep moving away (PRD S1 — was pre-tree early return)."""

    def update(self) -> common.Status:
        if self.ai._is_evading:
            self._set_action(AIAction.MOVE_AWAY_FROM_ENEMY)
            self.feedback_message = "active evasion"
            return common.Status.SUCCESS
        self.feedback_message = "not evading"
        return common.Status.FAILURE


class FrustratedAdvance(_SetsPendingAction):
    """High frustration → move toward enemy spawn."""

    def update(self) -> common.Status:
        if self.ai._frustration_cycles >= self.ai.config.frustration_threshold:
            self._set_action(AIAction.MOVE_TOWARD_ENEMY)
            self.feedback_message = "frustrated advance"
            return common.Status.SUCCESS
        self.feedback_message = "not frustrated"
        return common.Status.FAILURE


class DodgeIncoming(_SetsPendingAction):
    """Probabilistic dodge when a shot is near."""

    def update(self) -> common.Status:
        board = self.ai._tick_board
        assert board is not None
        dodge = self.ai._check_dodge_shot(
            board,
            self.ai._tick_my_pos,
            self.ai._tick_shots_on_board,
        )
        if dodge:
            self._set_action(dodge)
            self.feedback_message = "dodge"
            return common.Status.SUCCESS
        self.feedback_message = "no dodge"
        return common.Status.FAILURE


class EnemyPresenceTooClose(_SetsPendingAction):
    """Too-close evasion + mine (last_known updated in _sync_combat_perception)."""

    def update(self) -> common.Status:
        ep = self.ai._combat_enemy_pos
        if not ep:
            return common.Status.FAILURE
        if self.ai._is_too_close(ep, self.ai._tick_my_pos):
            self.ai._is_evading = True
            self.ai._evasion_end_time = self.ai._tick_current_time + random.uniform(1.0, 10.0)
            self.ai._evasion_cooldown = 10
            mine_p = min(0.88, 0.5 + 0.25 * self.ai.config.aggressive_mines)
            if self.ai._tick_my_mines > 0 and random.random() < mine_p:
                self._set_action(AIAction.PLACE_MINE)
            else:
                self._set_action(AIAction.MOVE_AWAY_FROM_ENEMY)
            self.feedback_message = "too close evasion"
            return common.Status.SUCCESS
        self.feedback_message = "not too close"
        return common.Status.FAILURE


class FireIfClearLos(_SetsPendingAction):
    """Forward LOS + ammo confidence."""

    def update(self) -> common.Status:
        board = self.ai._tick_board
        assert board is not None
        ep = self.ai._combat_enemy_pos
        if not ep:
            return common.Status.FAILURE
        if self.ai._has_forward_los(board, self.ai._tick_my_pos, self.ai._tick_my_direction, ep):
            if self.ai._can_fire_confidently(self.ai._tick_my_shots):
                if self.ai.should_skip_fire("aimed"):
                    self.feedback_message = "skip fire (hesitation)"
                    return common.Status.FAILURE
                self._set_action(AIAction.FIRE)
                self.feedback_message = "fire"
                return common.Status.SUCCESS
        self.feedback_message = "no shot"
        return common.Status.FAILURE


class FireRisky(_SetsPendingAction):
    """FR-8: low skill snap shot when cone + range but ray blocked."""

    def update(self) -> common.Status:
        board = self.ai._tick_board
        assert board is not None
        ep = self.ai._combat_enemy_pos
        if self.ai._risky_wild_state == "ready_to_fire":
            if not ep:
                self.ai._risky_wild_state = "idle"
                self.feedback_message = "risky commit cancelled (no enemy)"
                return common.Status.FAILURE
            self.ai._pending_fire_risky = True
            self._set_action(AIAction.FIRE)
            self.feedback_message = "risky fire commit"
            return common.Status.SUCCESS
        if not ep:
            return common.Status.FAILURE
        if self.ai._risky_wild_state != "idle":
            self.feedback_message = "risky busy"
            return common.Status.FAILURE
        if not self.ai.risky_shot_viable(
            board, self.ai._tick_my_pos, self.ai._tick_my_direction, ep
        ):
            return common.Status.FAILURE
        if not self.ai._can_fire_risky(self.ai._tick_my_shots):
            return common.Status.FAILURE
        if self.ai.should_skip_fire("risky"):
            self.feedback_message = "skip risky fire"
            return common.Status.FAILURE
        dirs = [
            Direction.UP,
            Direction.DOWN,
            Direction.LEFT,
            Direction.RIGHT,
            Direction.UP_LEFT,
            Direction.UP_RIGHT,
            Direction.DOWN_LEFT,
            Direction.DOWN_RIGHT,
        ]
        self.ai._risky_jitter_direction = random.choice(dirs)
        self.ai._risky_saved_direction = self.ai._tick_my_direction
        self.ai._risky_wild_state = "pending_aim"
        self._set_action(AIAction.RISKY_PREPARE)
        self.feedback_message = "risky prepare aim"
        return common.Status.SUCCESS


class ChaseLastKnownOrUnstick(_SetsPendingAction):
    """Pursue last known position or random move if stuck."""

    def update(self) -> common.Status:
        if not self.ai.last_known_enemy_pos:
            return common.Status.FAILURE
        if self.ai._check_stuck(self.ai._tick_my_pos):
            self.ai._recent_positions.clear()
            self._set_action(AIAction.RANDOM_MOVE)
            self.feedback_message = "unstick chase"
            return common.Status.SUCCESS
        self.ai._update_position_history(self.ai._tick_my_pos)
        self._set_action(AIAction.MOVE_TOWARD_ENEMY)
        self.feedback_message = "chase"
        return common.Status.SUCCESS


class StuckWithoutTarget(_SetsPendingAction):
    """No last known enemy but stuck → random move."""

    def update(self) -> common.Status:
        if self.ai.last_known_enemy_pos:
            return common.Status.FAILURE
        if self.ai._check_stuck(self.ai._tick_my_pos):
            self.ai._recent_positions.clear()
            self._set_action(AIAction.RANDOM_MOVE)
            self.feedback_message = "stuck no target"
            return common.Status.SUCCESS
        return common.Status.FAILURE


class ScanOrAdvance(_SetsPendingAction):
    """FR-9: scan_frequency / difficulty bias when blind; else advance."""

    def update(self) -> common.Status:
        cfg = self.ai.config
        if self.ai._combat_enemy_pos is None:
            # Throttle SCAN so blind tanks in corners do not spin every reaction tick (visual "oscillation")
            scan_gap_s = 0.28
            if self.ai._tick_current_time - self.ai._last_scan_decision_time < scan_gap_s:
                self._set_action(AIAction.MOVE_TOWARD_ENEMY)
                self.feedback_message = "advance (scan throttle)"
                return common.Status.SUCCESS
            p_scan = min(0.95, 0.12 + cfg.scan_frequency * 0.07 + cfg.ammo_discipline * 0.025)
            if cfg.scan_when_blind or random.random() < p_scan:
                self.ai._last_scan_decision_time = self.ai._tick_current_time
                self._set_action(AIAction.SCAN)
                self.feedback_message = "scan"
                return common.Status.SUCCESS
        self._set_action(AIAction.MOVE_TOWARD_ENEMY)
        self.feedback_message = "advance spawn"
        return common.Status.SUCCESS
