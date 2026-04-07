# Lessons Learned

## 2026-04-07: `bool("0")` is `True` in Python — env var truthiness

**Context:** `TANK_DEBUG=0` was intended to disable debug logging, but
`os.environ.get("TANK_DEBUG")` returns the string `"0"`, and `bool("0")` is
`True`.  All four debug-log call sites used this pattern.  Result: 150 MB of
debug output, 24 worker processes fighting over one file, massive I/O
contention that dominated wall time.

**Rule:** Always compare env-var strings explicitly:
```python
_ENABLED = os.environ.get("MY_FLAG", "").strip().lower() not in ("", "0", "false", "no", "off")
```
Never rely on `bool(os.environ.get(...))` for "is this env var turned on?"

**Files fixed:** `game.py`, `main.py`, `ai_tree.py`, `projectile.py`.
