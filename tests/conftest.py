"""Pytest configuration: headless pygame for CI and agent-driven testing.

Set SDL to dummy drivers before any pygame import in test modules.
"""

from __future__ import annotations

import os

# Must run before pygame is imported anywhere
os.environ.setdefault("SDL_VIDEODRIVER", "dummy")
os.environ.setdefault("SDL_AUDIODRIVER", "dummy")
