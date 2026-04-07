"""Unit tests for py_trees tank AI behaviours."""

from __future__ import annotations

import random
from unittest.mock import patch

import pytest
from tank_game.ai import AIAction, AIPlayer
from tank_game.ai_tree import log_behaviour_tree_tip
from tank_game.ai_utility import utility_snapshot
from tank_game.board import Board
from tank_game.player import Direction
from tank_game.projectile import Shot


@pytest.fixture
def board() -> Board:
    return Board()


@pytest.fixture
def ai_easy(board: Board) -> AIPlayer:
    return AIPlayer(
        difficulty=0,
        start_pos=(2, 12),
        enemy_start_pos=(37, 12),
    )


def test_reaction_time_gate_skips_subtree_when_not_ready(ai_easy: AIPlayer, board: Board) -> None:
    """PRD S1: root ``ReactionTimeGate`` does not tick the selector until reaction interval elapsed."""
    ai_easy._frustration_cycles = 100
    ai_easy._last_action_time = 0.0
    ai_easy._stage_for_tree_tick(0.05, board, (5, 5), Direction.RIGHT, 5, 1, None, [])
    ai_easy._pending_action = None
    tree = ai_easy._behaviour_tree
    assert tree is not None
    tree.tick()
    assert ai_easy._pending_action is None
    assert ai_easy._last_action_time == 0.0


def test_oscillation_unstick_before_evasion(ai_easy: AIPlayer, board: Board) -> None:
    """A↔B position ring forces RANDOM_MOVE and clears evasion (tight corridor ping-pong)."""
    ai_easy._is_evading = True
    ai_easy._last_action_time = -100.0
    ai_easy._decision_position_ring.clear()
    # Fourth sample is appended by _bind_tick_inputs during tree.tick (current cell).
    for p in [(5, 5), (6, 5), (5, 5)]:
        ai_easy._decision_position_ring.append(p)
    ai_easy._stage_for_tree_tick(1.0, board, (6, 5), Direction.RIGHT, 5, 1, (20, 12), [])
    ai_easy._pending_action = None
    tree = ai_easy._behaviour_tree
    assert tree is not None
    tree.tick()
    assert ai_easy._pending_action == AIAction.RANDOM_MOVE
    assert ai_easy._is_evading is False


def test_frustrated_advance_selects_move_toward(ai_easy: AIPlayer, board: Board) -> None:
    ai_easy._frustration_cycles = 100
    ai_easy._last_action_time = -100.0
    ai_easy._stage_for_tree_tick(0.0, board, (5, 5), Direction.RIGHT, 5, 1, None, [])
    ai_easy._pending_action = None
    tree = ai_easy._behaviour_tree
    assert tree is not None
    tree.tick()
    assert ai_easy._pending_action == AIAction.MOVE_TOWARD_ENEMY


def test_scan_or_advance_high_difficulty(board: Board) -> None:
    ai = AIPlayer(difficulty=8, start_pos=(2, 12), enemy_start_pos=(37, 12))
    ai._frustration_cycles = 0
    ai.last_known_enemy_pos = None
    ai._last_action_time = -100.0
    ai._stage_for_tree_tick(1.0, board, (10, 10), Direction.RIGHT, 5, 1, None, [])
    ai._stuck_counter = 0
    ai._pending_action = None
    assert ai._behaviour_tree is not None
    ai._behaviour_tree.tick()
    assert ai._pending_action == AIAction.SCAN


def test_scan_or_advance_throttles_scan_when_recently_blind(board: Board) -> None:
    """Corner oscillation: blind tanks were choosing SCAN every reaction tick; throttle inserts moves."""
    ai = AIPlayer(difficulty=8, start_pos=(2, 12), enemy_start_pos=(37, 12))
    ai.config.reaction_time = 0.01
    ai._frustration_cycles = 0
    ai.last_known_enemy_pos = None
    ai._last_action_time = 99.0
    ai._last_scan_decision_time = 100.0
    ai._stage_for_tree_tick(100.15, board, (10, 10), Direction.RIGHT, 5, 1, None, [])
    ai._stuck_counter = 0
    ai._pending_action = None
    assert ai._behaviour_tree is not None
    ai._behaviour_tree.tick()
    assert ai._pending_action == AIAction.MOVE_TOWARD_ENEMY


def test_get_movement_prefers_cardinal_near_edge(board: Board) -> None:
    """Outside rim: try cardinals before diagonal to reduce wall-hugging oscillation."""
    ai = AIPlayer(difficulty=5, start_pos=(2, 12), enemy_start_pos=(37, 12))
    with (
        patch.object(ai, "_can_move", return_value=True),
        patch.object(ai, "avoid_own_mines", return_value=True),
    ):
        # Short Manhattan leg stays in greedy mode (avoids hybrid A* in unit test)
        d = ai.get_movement_toward_target(
            board, (2, 2), (3, 3), blind=False, current_time=0.0
        )
    assert d == Direction.RIGHT


def test_get_movement_prefers_diagonal_in_open(board: Board) -> None:
    ai = AIPlayer(difficulty=5, start_pos=(2, 12), enemy_start_pos=(37, 12))
    with (
        patch.object(ai, "_can_move", return_value=True),
        patch.object(ai, "avoid_own_mines", return_value=True),
    ):
        d = ai.get_movement_toward_target(
            board, (15, 12), (18, 14), blind=False, current_time=0.0
        )
    assert d == Direction.DOWN_RIGHT


def test_fire_when_los_clear(board: Board) -> None:
    ai = AIPlayer(difficulty=9, start_pos=(5, 12), enemy_start_pos=(35, 12))
    ai.last_known_enemy_pos = None
    # Same row, facing right, enemy ahead in line of sight
    with (
        patch.object(ai, "_has_forward_los", return_value=True),
        patch.object(ai, "should_skip_fire", return_value=False),
    ):
        ai._last_action_time = -100.0
        # FR-10: effective min shots at skill 9 exceeds raw confidence_threshold alone
        ai._stage_for_tree_tick(1.0, board, (5, 12), Direction.RIGHT, 12, 1, (20, 12), [])
        ai._frustration_cycles = 0
        ai._pending_action = None
        assert ai._behaviour_tree is not None
        ai._behaviour_tree.tick()
    assert ai._pending_action == AIAction.FIRE


def test_dodge_incoming_when_shot_near(board: Board) -> None:
    ai = AIPlayer(difficulty=5, start_pos=(10, 10), enemy_start_pos=(30, 10))
    ai._frustration_cycles = 0
    shot = Shot(x=11, y=10, direction=Direction.RIGHT, owner_id=2)
    with patch("random.random", return_value=0.0):
        ai._last_action_time = -100.0
        ai._stage_for_tree_tick(1.0, board, (10, 10), Direction.UP, 5, 1, (20, 20), [shot])
        ai._pending_action = None
        assert ai._behaviour_tree is not None
        ai._behaviour_tree.tick()
    assert ai._pending_action == AIAction.DODGE_SHOT


def test_skill_zero_never_dodges(board: Board) -> None:
    ai = AIPlayer(difficulty=0, start_pos=(10, 10), enemy_start_pos=(30, 10))
    shot = Shot(x=11, y=10, direction=Direction.RIGHT, owner_id=2)
    ai._last_action_time = -100.0
    ai._stage_for_tree_tick(1.0, board, (10, 10), Direction.UP, 5, 1, (20, 20), [shot])
    ai._pending_action = None
    assert ai._behaviour_tree is not None
    ai._behaviour_tree.tick()
    assert ai._pending_action != AIAction.DODGE_SHOT


def test_legacy_ai_env_matches_monolith(monkeypatch: pytest.MonkeyPatch, board: Board) -> None:
    monkeypatch.setenv("TANK_LEGACY_AI", "1")
    ai = AIPlayer(difficulty=3, start_pos=(2, 12), enemy_start_pos=(37, 12))
    ai._frustration_cycles = 100
    ai._last_action_time = -100.0  # bypass reaction gate
    action = ai.update(
        current_time=1.0,
        board=board,
        my_pos=(5, 5),
        my_direction=Direction.RIGHT,
        my_shots=5,
        my_mines=1,
        enemy_pos=None,
        shots_on_board=[],
    )
    assert action == AIAction.MOVE_TOWARD_ENEMY


def test_pick_evade_rim_sticks_vertical_when_enemy_y_changes(board: Board) -> None:
    """Left-rim evasion: do not flip UP/DOWN every tick when chaser changes row (tank_debug oscillation)."""
    ai = AIPlayer(difficulty=5, start_pos=(2, 12), enemy_start_pos=(37, 12))

    def can_move(b: Board, pos: tuple[int, int], d: Direction) -> bool:
        if d == Direction.LEFT:
            return False
        if d in (Direction.UP, Direction.DOWN):
            return True
        return False

    with patch.object(ai, "_can_move", side_effect=can_move):
        d1 = ai.pick_evade_direction(board, (1, 8), (25, 8), current_time=1.0)
        assert d1 == Direction.UP
        assert ai._evade_vertical_commit == Direction.UP
        d2 = ai.pick_evade_direction(board, (1, 8), (25, 9), current_time=1.05)
        assert d2 == Direction.UP


def test_effective_min_shots_uses_confidence_threshold() -> None:
    import math

    ai = AIPlayer(difficulty=5, start_pos=(2, 12), enemy_start_pos=(37, 12))
    expected = max(1, 5 - math.ceil(ai.config.confidence_threshold))
    assert ai.effective_min_shots_for_aimed() == expected


def test_should_skip_fire_high_skill(monkeypatch: pytest.MonkeyPatch) -> None:
    ai = AIPlayer(difficulty=9, start_pos=(2, 12), enemy_start_pos=(37, 12))
    monkeypatch.setattr(random, "random", lambda: 0.0)
    assert not ai.should_skip_fire("aimed")


def test_should_skip_fire_low_skill(monkeypatch: pytest.MonkeyPatch) -> None:
    ai = AIPlayer(difficulty=0, start_pos=(2, 12), enemy_start_pos=(37, 12))
    monkeypatch.setattr(random, "random", lambda: 0.0)
    assert ai.should_skip_fire("aimed")


def test_utility_snapshot_returns_expected_keys(ai_easy: AIPlayer) -> None:
    snap = utility_snapshot(ai_easy)
    assert set(snap.keys()) == {
        "frustration_norm",
        "evasion_active",
        "dodge_skill",
        "ammo_discipline_norm",
        "reaction_norm",
    }


def test_log_behaviour_tree_tip_skips_without_tank_debug(ai_easy: AIPlayer) -> None:
    """Does not raise when TANK_DEBUG is unset."""
    tree = ai_easy._behaviour_tree
    assert tree is not None
    log_behaviour_tree_tip(tree, 1)
