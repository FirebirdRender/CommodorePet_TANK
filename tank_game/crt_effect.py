"""Optional CRT post-processing (UI_RETRO_SPEC §4).

Enable with ``TANK_CRT=1``.  Applies:
  1. **Scanlines** — semi-transparent horizontal black lines.
  2. **Bloom** — render to a smaller surface, then ``smoothscale`` up for an
     analog-CRT softness.

Both are composited *after* all game drawing, before ``display.flip()``.
"""

from __future__ import annotations

import os

import pygame

_CRT_ON: bool | None = None
_scanline_overlay: pygame.Surface | None = None
_bloom_buf: pygame.Surface | None = None

# Bloom downscale factor (2 = half-resolution intermediate).
_BLOOM_FACTOR = 2
_SCANLINE_ALPHA = 55


def crt_enabled() -> bool:
    global _CRT_ON
    if _CRT_ON is None:
        _CRT_ON = os.environ.get("TANK_CRT", "").strip().lower() in ("1", "true", "yes", "on")
    return _CRT_ON


def _ensure_scanlines(w: int, h: int) -> pygame.Surface:
    global _scanline_overlay
    if _scanline_overlay is not None and _scanline_overlay.get_size() == (w, h):
        return _scanline_overlay
    _scanline_overlay = pygame.Surface((w, h), pygame.SRCALPHA)
    _scanline_overlay.fill((0, 0, 0, 0))
    for y in range(0, h, 2):
        pygame.draw.line(_scanline_overlay, (0, 0, 0, _SCANLINE_ALPHA), (0, y), (w - 1, y))
    return _scanline_overlay


def apply_crt(screen: pygame.Surface) -> None:
    """Apply CRT post-processing to *screen* in-place (called before flip)."""
    if not crt_enabled():
        return
    w, h = screen.get_size()

    # Bloom: downscale then upscale with bilinear filtering
    global _bloom_buf
    small_w, small_h = w // _BLOOM_FACTOR, h // _BLOOM_FACTOR
    if _bloom_buf is None or _bloom_buf.get_size() != (small_w, small_h):
        _bloom_buf = pygame.Surface((small_w, small_h))
    pygame.transform.scale(screen, (small_w, small_h), _bloom_buf)
    pygame.transform.smoothscale(_bloom_buf, (w, h), screen)

    # Scanlines
    screen.blit(_ensure_scanlines(w, h), (0, 0))
