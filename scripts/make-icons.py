#!/usr/bin/env python3
"""Pixel-art app icons for the MiniDeck apps.

Each icon is drawn on a 36x36 pixel grid from simple shapes placed on whole
pixels, then finished the same way every time, so the set looks like one
family:
  - a 1-pixel ink outline around every shape (4-connected, so corners round)
  - light from the top left: a light row on top and left edges of each colour
    region, a dark row on its bottom and right edges
  - a soft one-pixel drop shadow
It is scaled up 2x with hard pixel edges (no smoothing) to Onion's 74x74
icon size, so every art pixel is a crisp 2x2 block on the Miyoo's screen.

  make-icons.py                 all icons into icons/<Folder>.png (each app's
                                repository keeps a copy as app/icon.png)
  make-icons.py --preview F     also a large preview sheet (PNG)
  make-icons.py --big N DIR     also N-times scaled copies into DIR
Standard library only.
"""
import struct, sys, zlib, os

G = 36          # art grid
SCALE = 2       # art pixel -> screen pixels
OUT = 74        # Onion icon size

INK = (27, 29, 58)
SHADOW = (0, 0, 0, 70)
# colour ramps: light, base, dark
RAMP = {
    "lav":    ((190, 194, 248), (140, 145, 222), (98, 102, 184)),
    "sky":    ((168, 228, 255), (95, 184, 240), (58, 130, 196)),
    "orange": ((255, 210, 140), (245, 167, 66), (201, 112, 42)),
    "green":  ((170, 240, 186), (92, 207, 134), (47, 154, 99)),
    "red":    ((255, 156, 142), (229, 83, 75), (168, 50, 58)),
    "yellow": ((255, 242, 160), (247, 210, 74), (201, 154, 42)),
    "white":  ((255, 255, 255), (233, 234, 242), (185, 187, 208)),
    "steel":  ((122, 127, 160), (78, 82, 116), (50, 53, 84)),
    "night":  ((60, 64, 104), (40, 43, 78), (29, 31, 60)),
    "pink":   ((255, 182, 212), (240, 107, 168), (184, 65, 122)),
}
FLAT = {"ink": INK, "glow": (120, 255, 170), "screen": (24, 60, 52)}


class Art:
    def __init__(self):
        self.px = [[None] * G for _ in range(G)]
        self.flat = [[False] * G for _ in range(G)]  # drawn flat: no shading

    def put(self, x, y, c, flat=False):
        if 0 <= x < G and 0 <= y < G:
            self.px[y][x] = c
            self.flat[y][x] = flat or c in FLAT or "." in c

    def rect(self, x0, y0, x1, y1, c, r=0, flat=False):
        """Filled rect, inclusive corners, with a pixel-art corner radius."""
        cut = {0: [], 1: [1], 2: [2, 1], 3: [3, 1, 1], 4: [4, 2, 1, 1]}[r]
        for y in range(y0, y1 + 1):
            i = min(y - y0, y1 - y)
            inset = cut[i] if i < len(cut) else 0
            for x in range(x0 + inset, x1 - inset + 1):
                self.put(x, y, c, flat)

    def disk(self, cx, cy, r, c, flat=False):
        """Filled circle centred on a pixel (cx, cy) or between pixels (x.5)."""
        for y in range(G):
            for x in range(G):
                if (x - cx) ** 2 + (y - cy) ** 2 <= r * r + r * 0.6:
                    self.put(x, y, c, flat)

    def ring(self, cx, cy, r, w, c, flat=False):
        for y in range(G):
            for x in range(G):
                d = (x - cx) ** 2 + (y - cy) ** 2
                if (r - w) ** 2 + (r - w) * 0.6 < d <= r * r + r * 0.6:
                    self.put(x, y, c, flat)

    def line(self, x0, y0, x1, y1, c, w=1, flat=False):
        """Bresenham line, w pixels wide (square brush)."""
        dx, dy = abs(x1 - x0), -abs(y1 - y0)
        sx, sy = (1 if x0 < x1 else -1), (1 if y0 < y1 else -1)
        err = dx + dy
        while True:
            for i in range(w):
                for j in range(w):
                    self.put(x0 + i - w // 2, y0 + j - w // 2, c, flat)
            if x0 == x1 and y0 == y1:
                break
            e2 = 2 * err
            if e2 >= dy:
                err += dy; x0 += sx
            if e2 <= dx:
                err += dx; y0 += sy

    def text(self, x, y, rows, c, flat=True):
        """Tiny bitmap: rows of '#'/'.'."""
        for j, row in enumerate(rows):
            for i, ch in enumerate(row):
                if ch == "#":
                    self.put(x + i, y + j, c, flat)

    # ---- finishing ----
    def finish(self):
        """Shading, outline and shadow -> RGBA grid."""
        px, flat = self.px, self.flat
        out = [[None] * G for _ in range(G)]
        same = lambda x, y, c: 0 <= x < G and 0 <= y < G and px[y][x] == c
        for y in range(G):
            for x in range(G):
                c = px[y][x]
                if c is None:
                    continue
                if c in FLAT:
                    out[y][x] = FLAT[c] + (255,)
                    continue
                name, _, tone = c.partition(".")
                lo, base, hi = RAMP[name][2], RAMP[name][1], RAMP[name][0]
                if tone:
                    col = {"hi": hi, "lo": lo, "base": base}[tone]
                elif flat[y][x]:
                    col = base
                elif not same(x, y - 1, c) or not same(x - 1, y, c):
                    col = hi
                elif not same(x, y + 1, c) or not same(x + 1, y, c):
                    col = lo
                else:
                    col = base
                out[y][x] = col + (255,)
        # ink outline around everything (4-connected)
        filled = lambda x, y: 0 <= x < G and 0 <= y < G and px[y][x] is not None
        ink = [[False] * G for _ in range(G)]
        for y in range(G):
            for x in range(G):
                if px[y][x] is None and any(filled(x + a, y + b) for a, b in ((1, 0), (-1, 0), (0, 1), (0, -1))):
                    ink[y][x] = True
        for y in range(G):
            for x in range(G):
                if ink[y][x]:
                    out[y][x] = INK + (255,)
        # drop shadow one pixel down and right of the outline
        for y in range(G - 1, 0, -1):
            for x in range(G - 1, 0, -1):
                if out[y][x] is None and out[y - 1][x - 1] is not None and out[y - 1][x - 1][3] == 255:
                    out[y][x] = SHADOW
        return out


def to_png(grid, scale, size):
    off = (size - G * scale) // 2
    rows = []
    for py in range(size):
        row = bytearray([0])
        for qx in range(size):
            gx, gy = (qx - off) // scale, (py - off) // scale
            c = grid[gy][gx] if 0 <= gx < G and 0 <= gy < G and qx >= off and py >= off else None
            row += bytes(c) if c else bytes(4)
        rows.append(bytes(row))
    return png(size, size, rows)


def png(w, h, rows):
    def chunk(t, d):
        return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d))
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 6, 0, 0, 0)) \
        + chunk(b"IDAT", zlib.compress(b"".join(rows), 9)) + chunk(b"IEND", b"")


# ---------------- the icons ----------------

def miniscale():
    a = Art()
    # three linked nodes: the MiniScale mark
    a.line(18, 8, 8, 27, "lav", 3)
    a.line(18, 8, 28, 27, "lav", 3)
    a.line(8, 27, 28, 27, "lav", 3)
    for cx, cy in ((18, 8), (8, 27), (28, 27)):
        a.disk(cx, cy, 6, "sky")
        a.disk(cx - 2, cy - 2, 1, "white", flat=True)
    return a


def minirack():
    a = Art()
    a.rect(6, 3, 29, 32, "steel", r=2)
    for i, y in enumerate((6, 15, 24)):
        a.rect(9, y, 26, y + 6, "lav", r=1)
        a.rect(12, y + 2, 20, y + 4, "night", flat=True)
        a.put(23, y + 3, ("glow", "glow", "orange.hi")[i])
        a.put(24, y + 3, "glow" if i != 2 else "orange.hi")
    a.rect(9, 33, 11, 33, "steel")   # feet
    a.rect(24, 33, 26, 33, "steel")
    return a


def minitape():
    a = Art()
    a.rect(3, 7, 32, 28, "orange", r=2)
    a.rect(7, 10, 28, 22, "white", r=1)        # label
    a.rect(9, 11, 26, 12, "red.base", flat=True)   # label stripe
    a.rect(11, 15, 24, 20, "night", r=1, flat=True)  # window
    for cx in (14, 21):
        a.disk(cx, 17.5, 2, "white")
        a.put(cx, 17, "ink"); a.put(cx, 18, "ink")
    a.rect(10, 24, 25, 28, "orange.lo", flat=True)  # bottom ridge
    for x in (13, 22):
        a.put(x, 26, "ink")
    return a


def minikey():
    a = Art()
    a.ring(10, 9, 6, 2, "yellow")              # key ring
    a.rect(9, 12, 30, 31, "lav", r=3)          # fob body
    a.rect(12, 15, 27, 22, "screen", flat=True)
    # six digits as little bars
    for i in range(6):
        x = 13 + i * 2 + (1 if i >= 3 else 0)
        a.rect(x, 17, x, 20, "glow", flat=True)
    a.rect(13, 25, 26, 28, "lav.lo", flat=True)
    a.disk(19.5, 26.5, 1.5, "sky")              # button
    return a


def tasks():
    a = Art()
    a.rect(6, 5, 29, 32, "lav", r=2)           # clipboard
    a.rect(9, 8, 26, 30, "white", r=1)         # sheet
    a.rect(13, 3, 22, 7, "steel", r=1)         # clip
    for i, y in enumerate((12, 18, 24)):
        a.rect(11, y, 14, y + 3, "night", flat=True)
        if i < 2:  # a bold tick that pokes out of its box
            for x, yy in ((11, y + 1), (12, y + 2), (13, y + 1), (14, y), (15, y - 1)):
                a.put(x, yy, "green.hi", flat=True)
            a.put(12, y + 3, "green.hi", flat=True)
        a.rect(17, y + 1, 24 - i * 2, y + 2, "steel.hi", flat=True)
    return a


def store():
    a = Art()
    a.rect(5, 15, 30, 32, "lav")               # shop front
    for i in range(6):                         # striped awning
        a.rect(4 + i * 5, 8, 8 + i * 5, 14, "red" if i % 2 == 0 else "white")
    a.rect(4, 6, 32, 7, "red.lo", flat=True)
    for i in range(6):                         # scalloped edge
        a.disk(6 + i * 5, 15, 2, "red" if i % 2 == 0 else "white")
    a.rect(8, 20, 16, 32, "sky", r=1)          # door
    a.put(14, 26, "ink")
    a.rect(19, 20, 27, 26, "sky", r=1)         # window
    a.rect(20, 23, 22, 25, "orange", flat=True)   # little box in the window
    a.rect(24, 22, 26, 25, "green", flat=True)
    return a


def guide():
    a = Art()
    a.rect(3, 9, 32, 29, "lav", r=2)           # cover
    a.rect(5, 7, 17, 27, "white", r=1)         # left page
    a.rect(18, 7, 30, 27, "white", r=1)        # right page
    a.rect(17, 7, 18, 28, "ink", flat=True)    # spine
    for y in (11, 14, 17, 20, 23):
        a.rect(7, y, 15, y, "steel.hi", flat=True)
        a.rect(20, y, 28 - (4 if y == 23 else 0), y, "steel.hi", flat=True)
    a.rect(25, 4, 27, 10, "green", flat=True)  # bookmark
    return a


def network():
    a = Art()
    a.disk(17.5, 17.5, 14, "night", flat=True)
    a.ring(17.5, 17.5, 14, 2, "lav")
    a.ring(17.5, 17.5, 9, 1, "green.lo", flat=True)
    a.ring(17.5, 17.5, 4, 1, "green.lo", flat=True)
    a.line(18, 17, 27, 8, "glow", 1, flat=True)        # sweep
    a.line(18, 18, 27, 9, "green.lo", 1, flat=True)
    for x, y in ((10, 12), (23, 22), (12, 24)):        # blips
        a.rect(x, y, x + 1, y + 1, "glow", flat=True)
    a.rect(17, 17, 18, 18, "white", flat=True)
    return a


def minipulse():
    import math
    a = Art()
    a.disk(17.5, 17.5, 14, "night", flat=True)
    a.ring(17.5, 17.5, 14, 2, "lav")
    # the scale: green, yellow, red around the top, 210° to -30°
    for deg10 in range(-300, 2101, 15):
        t = math.radians(deg10 / 10)
        c = "green" if deg10 > 1300 else "yellow" if deg10 > 500 else "red"
        for r in (9.5, 10.5):
            a.put(int(round(17.5 + r * math.cos(t))), int(round(17.5 - r * math.sin(t))), c, flat=True)
    a.line(18, 18, 25, 10, "white", 2, flat=True)       # needle, near the top of the scale
    a.disk(17.5, 17.5, 2.5, "orange")                    # hub
    a.rect(12, 25, 23, 27, "steel", flat=True)           # readout
    a.rect(14, 26, 21, 26, "glow", flat=True)
    return a


ICONS = {
    "miniscale": ("MiniScale", miniscale),
    "minirack": ("MiniRack", minirack),
    "minitape": ("MiniTape", minitape),
    "minikey": ("MiniKey", minikey),
    "tasks": ("Tasks", tasks),
    "store": ("Store", store),
    "guide": ("Guide", guide),
    "network": ("Network", network),
    "minipulse": ("MiniPulse", minipulse),
}


def sheet(grids, scale, bg):
    pad, n = 24, len(grids)
    cell = G * scale + pad
    w, h = cell * n + pad, cell + pad
    rows = []
    for y in range(h):
        row = bytearray([0])
        for x in range(w):
            c = bg
            i, cx = divmod(x - pad, cell)
            cy = y - pad
            if 0 <= i < n and 0 <= cx < G * scale and 0 <= cy < G * scale:
                p = grids[i][cy // scale][cx // scale]
                if p:
                    al = p[3] / 255
                    c = tuple(int(p[k] * al + bg[k] * (1 - al)) for k in range(3))
            row += bytes(c) + b"\xff"
        rows.append(bytes(row))
    return png(w, h, rows)


if __name__ == "__main__":
    os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    args = sys.argv[1:]
    grids = []
    for kind, (folder, fn) in ICONS.items():
        g = fn().finish()
        grids.append(g)
        os.makedirs("icons", exist_ok=True)
        open(f"icons/{folder}.png", "wb").write(to_png(g, SCALE, OUT))
        if "--big" in args:
            n, d = int(args[args.index("--big") + 1]), args[args.index("--big") + 2]
            os.makedirs(d, exist_ok=True)
            open(f"{d}/{kind}.png", "wb").write(to_png(g, n, G * n))
    if "--preview" in args:
        f = args[args.index("--preview") + 1]
        dark = sheet(grids, 6, (22, 24, 40))
        open(f, "wb").write(dark)
        print("preview", f)
