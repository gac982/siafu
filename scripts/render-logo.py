#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Siafu contributors
"""Render the Siafu logo as ANSI half-block art.

Requires ImageMagick (`magick`) and Python 3. The output is embedded in the
TUI, so it only needs to run when the logo changes:

    python3 scripts/render-logo.py \
        --crop 1125x1451+461+304 --width 44 --rows 28 --no-snap \
        --out internal/tui/assets/logo.ans
    python3 scripts/render-logo.py \
        --crop 420x480+810+480 --width 24 --rows 12 --no-snap \
        --out internal/tui/assets/mark.ans

Pixels connected to the border and close to the background become transparent,
so the art blends with the terminal instead of drawing a dark rectangle. By
default colors snap to a small brand palette, which keeps small renditions
crisp; `--no-snap` keeps the original truecolor for larger renderings.
"""

from __future__ import annotations

import argparse
import subprocess
from collections import deque

RESET = "\x1b[0m"
BG = (14, 17, 22)

# Brand palette plus two shades that keep the ant's edges and face readable at
# small sizes. The first entry is the background and becomes transparent.
PALETTE = [
    (14, 17, 22),     # Ink
    (122, 42, 16),    # dark ember (edges)
    (252, 76, 2),     # Ember
    (184, 174, 161),  # bone shadow
    (230, 225, 214),  # Bone
    (122, 131, 148),  # Ash
]


def is_background(rgb: tuple[int, int, int]) -> bool:
    return all(abs(c - b) <= 26 for c, b in zip(rgb, BG))


def snap(rgb: tuple[int, int, int]) -> tuple[int, int, int]:
    return min(PALETTE, key=lambda p: sum((a - b) ** 2 for a, b in zip(rgb, p)))


def background_mask(pixels: list[tuple[int, int, int]], width: int, height: int) -> list[bool]:
    """Flood-fill the background from the image borders.

    Only the outer background is removed: dark interior details such as the
    eyes and outlines stay opaque.
    """
    mask = [False] * (width * height)
    queue: deque[int] = deque()

    def seed(index: int) -> None:
        if not mask[index] and is_background(pixels[index]):
            mask[index] = True
            queue.append(index)

    for x in range(width):
        seed(x)
        seed((height - 1) * width + x)
    for y in range(height):
        seed(y * width)
        seed(y * width + width - 1)

    while queue:
        index = queue.popleft()
        x, y = index % width, index // width
        if x > 0:
            seed(index - 1)
        if x + 1 < width:
            seed(index + 1)
        if y > 0:
            seed(index - width)
        if y + 1 < height:
            seed(index + width)
    return mask


def render(src: str, crop: str, width: int, rows: int, snap_palette: bool) -> str:
    height = rows * 2
    args = ["magick", src]
    if crop:
        args += ["-crop", crop, "+repage"]
    args += ["-resize", f"{width}x{height}!", "-depth", "8", "RGB:-"]
    data = subprocess.check_output(args)
    if len(data) != width * height * 3:
        raise SystemExit(f"unexpected pixel data: {len(data)} bytes")

    if snap_palette:
        pixels = [
            snap((data[i], data[i + 1], data[i + 2]))
            for i in range(0, len(data), 3)
        ]
    else:
        pixels = [
            (data[i], data[i + 1], data[i + 2]) for i in range(0, len(data), 3)
        ]
    transparent = background_mask(pixels, width, height)

    lines = []
    for row in range(rows):
        parts = []
        for col in range(width):
            top = row * 2 * width + col
            bottom = (row * 2 + 1) * width + col
            top_rgb = pixels[top]
            bottom_rgb = pixels[bottom]
            top_bg = transparent[top]
            bottom_bg = transparent[bottom]

            if top_bg and bottom_bg:
                parts.append(" ")
            elif top_bg:
                r, g, b = bottom_rgb
                parts.append(f"\x1b[38;2;{r};{g};{b}m\u2584")
            elif bottom_bg:
                r, g, b = top_rgb
                parts.append(f"\x1b[38;2;{r};{g};{b}m\u2580")
            elif top_rgb == bottom_rgb:
                r, g, b = top_rgb
                parts.append(f"\x1b[38;2;{r};{g};{b}m\u2580")
            else:
                tr, tg, tb = top_rgb
                br, bg, bb = bottom_rgb
                parts.append(
                    f"\x1b[38;2;{tr};{tg};{tb}m\x1b[48;2;{br};{bg};{bb}m\u2580"
                )
        lines.append("".join(parts) + RESET)
    return "\n".join(lines) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--src", default="assets/logo.jpeg")
    parser.add_argument("--crop", default="")
    parser.add_argument("--width", type=int, required=True)
    parser.add_argument("--rows", type=int, required=True)
    parser.add_argument("--no-snap", action="store_true")
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    art = render(
        args.src,
        args.crop,
        args.width,
        args.rows,
        not args.no_snap,
    )
    with open(args.out, "w", encoding="utf-8") as handle:
        handle.write(art)


if __name__ == "__main__":
    main()
