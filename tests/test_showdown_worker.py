"""Parallel SHOWDOWN worker (timeouts / retries)."""

from tank_game.showdown_worker import _run_match


def test_run_match_times_out_when_max_steps_too_low(monkeypatch) -> None:
    monkeypatch.setenv("TANK_SHOWDOWN_MAX_STEPS", "5")
    monkeypatch.setenv("TANK_SHOWDOWN_MAX_RETRIES", "1")
    r = _run_match((0, 5, 3, 42))
    assert r.get("timeout") is True
    assert r.get("timeout_reason") == "max_steps"
    assert r.get("timeout_after_retries") is True


def test_run_match_completes_normally(monkeypatch) -> None:
    monkeypatch.delenv("TANK_SHOWDOWN_MAX_STEPS", raising=False)
    r = _run_match((0, 5, 3, 42))
    assert r.get("timeout") is not True
    assert r["winner"] in (1, 2)
