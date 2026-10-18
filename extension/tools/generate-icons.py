#!/usr/bin/env python3
"""Generate the extension's PNG icons from the dashboard logo geometry.

The Sorolens brand mark lives in apps/web/public/logo.svg (a cyan ellipse ring
crossed by a pulse line). Keeping that mark as source-of-truth means the
extension icons stay in sync with the web app without pulling in a rasterizer
or a build dependency: this script renders the same geometry with the Python
standard library only (zlib for PNG compression) and writes:

    src/icons/icon-16.png
    src/icons/icon-32.png
    src/icons/icon-48.png
    src/icons/icon-128.png

Usage:
    python3 tools/generate-icons.py
"""

from __future__ import annotations

import math
import struct
import zlib
from pathlib import Path

# Brand colours from apps/web (logo.svg stroke + --color-bg-page).
MARK = (6, 182, 212)  # #06B6D4

VIEWBOX = 64.0
ELLIPSE_CENTER = (32.0, 32.0)
ELLIPSE_RADII = (28.0, 16.0)
LOGO_STROKE = 3.5
PULSE_POINTS = [
    (10.0, 32.0),
    (20.0, 32.0),
    (24.0, 20.0),
    (28.0, 44.0),
    (32.0, 20.0),
    (36.0, 44.0),
    (40.0, 32.0),
    (54.0, 32.0),
]

SIZES = (16, 32, 48, 128)
SUPERSAMPLE = 4
# Small icons need a proportionally thicker stroke to stay legible.
MIN_RENDERED_STROKE_PX = 1.7

OUT_DIR = Path(__file__).resolve().parent.parent / "src" / "icons"


def stroke_width_units(px: int) -> float:
    """Stroke width in viewBox units so tiny icons stay visible."""
    minimum_units = MIN_RENDERED_STROKE_PX * VIEWBOX / px
    return max(LOGO_STROKE, minimum_units)


def ellipse_distance(x: float, y: float) -> float:
    """First-order Euclidean distance from (x, y) to the logo ellipse."""
    cx, cy = ELLIPSE_CENTER
    rx, ry = ELLIPSE_RADII
    dx = x - cx
    dy = y - cy
    k = math.hypot(dx / rx, dy / ry)
    if k < 1e-9:
        return min(rx, ry)
    # |grad k| gives the first-order distance estimate |k - 1| / |grad k|.
    gx = dx / (rx * rx * k)
    gy = dy / (ry * ry * k)
    grad = math.hypot(gx, gy)
    if grad < 1e-9:
        return min(rx, ry)
    return abs(k - 1.0) / grad


def segment_distance(px: float, py: float, ax: float, ay: float, bx: float, by: float) -> float:
    """Distance from a point to a line segment (round caps come for free)."""
    vx = bx - ax
    vy = by - ay
    length_sq = vx * vx + vy * vy
    if length_sq < 1e-12:
        return math.hypot(px - ax, py - ay)
    t = ((px - ax) * vx + (py - ay) * vy) / length_sq
    t = max(0.0, min(1.0, t))
    return math.hypot(px - (ax + t * vx), py - (ay + t * vy))


def pulse_distance(x: float, y: float) -> float:
    """Distance from (x, y) to the pulse polyline."""
    best = float("inf")
    for (ax, ay), (bx, by) in zip(PULSE_POINTS, PULSE_POINTS[1:]):
        best = min(best, segment_distance(x, y, ax, ay, bx, by))
    return best


def over(dst: tuple[float, float, float, float], rgb: tuple[int, int, int], alpha: float):
    """Straight-alpha source-over compositing."""
    sr, sg, sb = (channel / 255.0 for channel in rgb)
    da = dst[3]
    out_a = alpha + da * (1.0 - alpha)
    if out_a <= 0.0:
        return (0.0, 0.0, 0.0, 0.0)
    return (
        (sr * alpha + dst[0] * da * (1.0 - alpha)) / out_a,
        (sg * alpha + dst[1] * da * (1.0 - alpha)) / out_a,
        (sb * alpha + dst[2] * da * (1.0 - alpha)) / out_a,
        out_a,
    )


def render(px: int) -> bytearray:
    """Renders one icon at `px` and returns straight RGBA bytes (px * px * 4)."""
    width = px * SUPERSAMPLE
    scale = width / VIEWBOX
    # x/y below are in viewBox units, so the threshold is too (half the stroke).
    half_stroke = stroke_width_units(px) / 2.0

    canvas = [(0.0, 0.0, 0.0, 0.0)] * (width * width)
    for row in range(width):
        for col in range(width):
            x = (col + 0.5) / scale
            y = (row + 0.5) / scale
            alpha = 0.0
            if ellipse_distance(x, y) <= half_stroke:
                alpha = 1.0
            elif pulse_distance(x, y) <= half_stroke:
                alpha = 1.0
            if alpha > 0.0:
                canvas[row * width + col] = over(
                    canvas[row * width + col], MARK, alpha
                )

    # Box-filter down to the target size for anti-aliasing.
    out = bytearray(px * px * 4)
    samples = SUPERSAMPLE * SUPERSAMPLE
    for row in range(px):
        for col in range(px):
            r = g = b = a = 0.0
            for dy in range(SUPERSAMPLE):
                for dx in range(SUPERSAMPLE):
                    cell = canvas[
                        (row * SUPERSAMPLE + dy) * width + (col * SUPERSAMPLE + dx)
                    ]
                    r += cell[0]
                    g += cell[1]
                    b += cell[2]
                    a += cell[3]
            index = (row * px + col) * 4
            out[index] = round(r / samples * 255)
            out[index + 1] = round(g / samples * 255)
            out[index + 2] = round(b / samples * 255)
            out[index + 3] = round(a / samples * 255)
    return out


def write_png(path: Path, px: int, rgba: bytes) -> None:
    """Writes an 8-bit RGBA PNG without third-party libraries."""
    raw = bytearray()
    stride = px * 4
    for row in range(px):
        raw.append(0)  # filter type: None
        raw.extend(rgba[row * stride : (row + 1) * stride])

    def chunk(tag: bytes, payload: bytes) -> bytes:
        return (
            struct.pack(">I", len(payload))
            + tag
            + payload
            + struct.pack(">I", zlib.crc32(tag + payload) & 0xFFFFFFFF)
        )

    header = struct.pack(">IIBBBBB", px, px, 8, 6, 0, 0, 0)
    png = (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", header)
        + chunk(b"IDAT", zlib.compress(bytes(raw), 9))
        + chunk(b"IEND", b"")
    )
    path.write_bytes(png)


def main() -> int:
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    for px in SIZES:
        rgba = render(px)
        target = OUT_DIR / f"icon-{px}.png"
        write_png(target, px, rgba)
        print(f"wrote {target.relative_to(OUT_DIR.parents[2])} ({px}x{px})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
