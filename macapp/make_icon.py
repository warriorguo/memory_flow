#!/usr/bin/env python3
"""Generate the Memory Flow app icon from the brain reference image.

Pipeline: upscale + threshold the (low-res) reference to re-sharpen the line
art, recolor it onto a white rounded "squircle" tile, then emit a 1024px master
PNG. The build script turns this into the .icns iconset.
"""
import sys
from PIL import Image, ImageOps, ImageDraw, ImageFilter

SRC = sys.argv[1] if len(sys.argv) > 1 else \
    "/Users/andrew/.claude/image-cache/67531b36-2416-432e-9c84-90e736c35365/1.png"
OUT = sys.argv[2] if len(sys.argv) > 2 else \
    "/Users/andrew/Codes/github.com/warriorguo/memory_flow/macapp/icon_master.png"

CANVAS = 1024
RADIUS = 232          # rounded-square corner radius (≈ macOS squircle feel)
SUPERSAMPLE = 4       # render big, downsample once for clean edges
INK = (28, 28, 30)    # near-black brain
PAD = 0.20            # fraction of canvas kept as margin around the brain

# --- 1. Sharpen the reference line art --------------------------------------
src = Image.open(SRC).convert("L")
big = src.resize((src.width * 10, src.height * 10), Image.LANCZOS)
# Binarize so the upscaled strokes are crisp again, then soften 1px for AA.
bw = big.point(lambda p: 0 if p < 135 else 255, mode="L")
bw = bw.filter(ImageFilter.GaussianBlur(1.2))

# Crop to the brain's bounding box (content is the dark pixels).
alpha_full = ImageOps.invert(bw)            # strokes -> high alpha
bbox = alpha_full.getbbox()
alpha = alpha_full.crop(bbox)

brain = Image.new("RGBA", alpha.size, INK + (0,))
brain.putalpha(alpha)

# --- 2. White rounded-square tile (supersampled) ----------------------------
S = CANVAS * SUPERSAMPLE
tile = Image.new("RGBA", (S, S), (0, 0, 0, 0))
mask = Image.new("L", (S, S), 0)
ImageDraw.Draw(mask).rounded_rectangle([0, 0, S - 1, S - 1],
                                       radius=RADIUS * SUPERSAMPLE, fill=255)
# Soft top-down gradient (white -> faint cool gray) for a little depth.
grad = Image.new("RGBA", (S, S), (255, 255, 255, 255))
for y in range(S):
    t = y / (S - 1)
    v = int(255 - 12 * t)
    grad.paste((v, v, min(255, v + 3), 255), (0, y, S, y + 1))
tile = Image.composite(grad, tile, mask)

# Faint inner border so the white tile reads on light backgrounds.
ImageDraw.Draw(tile).rounded_rectangle(
    [3 * SUPERSAMPLE, 3 * SUPERSAMPLE, S - 1 - 3 * SUPERSAMPLE, S - 1 - 3 * SUPERSAMPLE],
    radius=(RADIUS - 3) * SUPERSAMPLE, outline=(0, 0, 0, 28), width=2 * SUPERSAMPLE)

# --- 3. Place the brain, centered, with margin ------------------------------
avail = int(S * (1 - 2 * PAD))
bw_ratio = brain.width / brain.height
if bw_ratio >= 1:
    tw = avail
    th = int(avail / bw_ratio)
else:
    th = avail
    tw = int(avail * bw_ratio)
brain_r = brain.resize((tw, th), Image.LANCZOS)
ox = (S - tw) // 2
oy = (S - th) // 2 - int(S * 0.01)   # nudge up a hair for optical centering
tile.alpha_composite(brain_r, (ox, oy))

# --- 4. Downsample to the master size ---------------------------------------
icon = tile.resize((CANVAS, CANVAS), Image.LANCZOS)
icon.save(OUT)
print("wrote", OUT, icon.size)
