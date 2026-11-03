# Developing MiniDeck

MiniDeck is a set of apps for the Miyoo Mini Plus on Onion OS. Each app
has its own repository; this one holds the library they share, the tools,
and the release that bundles them. This guide covers all of them.

## The target

Everything here runs on a Miyoo Mini Plus with Onion OS 4.4: a SigmaStar
SSD202D (two ARM Cortex-A7 cores at 1.2 GHz), 128 MB of RAM of which about
100 MB is left to Linux, a 640x480 screen mounted upside down, and Onion's
BusyBox userland. There is no package manager, no certificate store, no
TUN device and no SDL that the apps can rely on. That shapes most of the
code:

- The apps are static Go binaries (`CGO_ENABLED=0`, `GOARCH=arm GOARM=7`)
  that draw to `/dev/fb0` and read `/dev/input/event0` themselves.
- HTTPS clients embed Mozilla's roots
  (`golang.org/x/crypto/x509roots/fallback`).
- Device scripts are POSIX `sh` that BusyBox's `ash` runs: no bash-isms,
  no `[[ ]]`, no arrays.
- Memory is tight. An app should stay under about 15 MB resident and free
  what it doesn't need when it leaves a screen.

## Repositories

| Repository | What it holds |
|---|---|
| [MiniDeck](https://github.com/zakijss/MiniDeck) | The library (module `github.com/zakijss/minideck`), the icons, the tools, and the all-apps release with MiniMart's catalogue. |
| [MiniScale](https://github.com/zakijss/MiniScale) | The Tailscale client: `miniscale`, `miniscale-fwd`, `miniscale-badge`, `miniscale.sh` and the Tailscale build. |
| [MiniRack](https://github.com/zakijss/MiniRack), [MiniTape](https://github.com/zakijss/MiniTape), [MiniFin](https://github.com/zakijss/MiniFin), [MiniKey](https://github.com/zakijss/MiniKey), [Tasks](https://github.com/zakijss/MiniTop), [Store](https://github.com/zakijss/MiniMart), [Guide](https://github.com/zakijss/MiniBook), [Network](https://github.com/zakijss/MiniRadar), [MiniPulse](https://github.com/zakijss/MiniPulse) | One app each. |

The library's packages:

| Package | What it does |
|---|---|
| `ui` | The UI kit: framebuffer output, buttons, the pixel fonts, panels, lists, forms, the on-screen keyboard, the status bar, themes and the moving backgrounds. |
| `fb` | Framebuffer ioctls shared by the UI kit and the overlays. |
| `secrets` | Small JSON secrets sealed with the device key, in a format `openssl enc` can open. |
| `tnet` | Reaching the tailnet through MiniScale's proxies (ports from `miniscale.conf`), server address parsing, and listing the tailnet's peers. |
| `procs` | Finding processes in `/proc`, and whether Onion is running a game. |
| `appdir` | The app's own folder (or `MINIDECK_APPDIR`). |
| `cache` | An on-card cache for server replies, images and songs. |
| `atomicfile` | Writes that survive a power cut. |
| `qr` | The QR encoder for login links. |
| `version` | The release version stamped into every binary. |

An app repository has the same shape everywhere:

| Path | What it holds |
|---|---|
| `cmd/<name>/` | The program: `main.go` holds its screens and event loop, topic files the rest. |
| `app/` | What the Onion app folder ships besides the binaries: `config.json`, `launch.sh`, `icon.png` (a copy of `icons/<Folder>.png` here), example settings. |
| `startup/` | Boot scripts installed into `.tmp_update/startup/`, if any. |
| `store.json` | The app's entry in MiniMart's catalogue; `scripts/package.sh` fills in the version, checksum and size. |
| `scripts/` | `build.sh`, `package.sh`, `deploy.sh`. |

## Tools

Docker, git and make are all you need. Go runs in the
`golang:1.27.3-bookworm` image, with module and build caches in Docker
volumes; with Go 1.27 or newer installed, add `GO=go` to any make target.
Every repository has the same targets: `make check` runs what CI runs, and
`make package VERSION=x.y.z` writes the release files into `dist/`.

To change the library and an app together, put a `go.work` beside the app's
`go.mod` (it is ignored by git):

```
go 1.27

use (
	.
	../MiniDeck
)
```

## Running apps on a computer

The UI kit has three switches that make any app run headless:

| Variable | Effect |
|---|---|
| `MINIDECK_UI_PNG=frames/f%d.png` | Write each frame to a PNG instead of the framebuffer. |
| `MINIDECK_UI_KEYS=wait,a,down,b` | Replay button presses, one every 1.5 s (`wait` presses nothing), then quit. |
| `MINIDECK_UI_NULL=1` | Draw without output (benchmarks). |

and the apps read their files from `MINIDECK_APPDIR` instead of their own
folder. `tools/mockhub` serves fake Glances, Beszel, Vantage and Subsonic
APIs, so MiniRack and MiniTape can be driven end to end:

```sh
go run github.com/zakijss/minideck/tools/mockhub@latest &
# in the MiniRack repository:
mkdir -p /tmp/minirack && cp app/settings.example.json /tmp/minirack/settings.json
MINIDECK_APPDIR=/tmp/minirack MINIDECK_SOCKS=off MINIDECK_UI_PNG=/tmp/f%d.png \
  MINIDECK_UI_KEYS=wait,a,wait,b go run ./cmd/minirack
```

Other variables used in tests and debugging:

| Variable | Used by |
|---|---|
| `MINIDECK_GALLERY=DIR` | The screen tests (below). |
| `MINIDECK_SKY=0` | Keeps the background still. |
| `MINIDECK_THEME`, `MINIDECK_ACCENT`, `MINIDECK_BACKGROUND` | Override the saved theme, accent and background by ID (`ember`, `coral`, `snow`; see `ui/theme.go` and `scene.go`), for screenshots. |
| `MINIDECK_APPEARANCE=FILE` | Where the shared look is saved, instead of `App/.minideck/appearance.json`. |
| `MINIDECK_UI_TRACE=FILE` | Append each frame's drawing and copy times to FILE, and every 10 s how smoothly the now-playing strip moved. |
| `MINIDECK_FFPLAY_RT=0` | MiniTape: start ffplay without real-time scheduling (to compare). |
| `MINIDECK_PPROF=FILE` | Write a CPU profile of the session to FILE (`go tool pprof`). |
| `MINIDECK_RUN`, `MINIDECK_DEVKEY` | Where MiniScale's runtime files and the device key are. |
| `MINIDECK_SOCKS`, `MINIDECK_HTTP_PROXY` | Override the tailnet proxies' addresses (normally read from `App/MiniScale/miniscale.conf`); `off` connects directly. |
| `MINIDECK_DIST=dist` | MiniMart's release test: installs every app from real release zips. |
| `MINIDECK_PICKS=DIR` | MiniMart's community apps test: checks each pinned download, saved in DIR as `<Folder>.zip`, against its SHA-256 and unpacks it with MiniMart's rules. |

The UI kit's text layout (Arabic shaping, bidirectional order, Fit and
Wrap) has a fuzz test too:

```sh
go test -run=x -fuzz=FuzzText -fuzztime=2m ./ui
```

The Unifont fallback (`ui/fonts/unifont.bin`) and the shaping
tables (`ui/intl_tables.go`) are generated by
`ui/mkintl.go` from the Unifont release and Unicode's data files;
its header says which versions and how to run it.

MiniBook's Markdown parser has a fuzz test in its repository (`FuzzParse`).

## Screens

Every app has screen tests that draw its main screens with fixed data:

```sh
make screens            # PNGs into build/screens/
```

Look at them after any change to a screen or to `ui`. Pixel art
breaks in ways tests don't catch: a value that lands on an odd pixel blurs
a 2x glyph, a label that grows by one character runs into its neighbour.
The UI kit's rules:

- Everything is drawn on a 2-pixel grid (`art = 2`): positions and sizes
  are even, glyphs are scaled by whole numbers.
- Text uses the faces in `ui/draw.go` (`Title`, `Heading`, `Body`,
  `Small`, `Caps`, `Big`), colours the tokens beside them (`Text`, `Muted`,
  `Faint`, `Accent`, `Good`, `Warn`, `Bad`). Don't add colours in apps.
- A screen is `StaticFrame` (background, header, footer hints, cached) plus
  panels and lists, then `Present`. `Present` copies only the rows that
  changed, so a redraw that changes little is cheap.
- Long work runs in a goroutine; the event loop waits on the input channel
  and the goroutine's results, and redraws when either arrives.

## Themes and backgrounds

Appearance (`ui/appearance.go`) is one screen shared by every app, saved
in `App/.minideck/appearance.json` for all of them. It has five settings:

| Setting | Choices | Where |
|---|---|---|
| Theme | 16 dark themes: Night (the default), Forest, Ocean, Ember, Grape, Rose, Graphite, High contrast, Dusk, Mint, Sand, Slate, Coffee, Sakura, Deep sea, Neon | `Themes` in `ui/theme.go` |
| Accent | the theme's own, or Lavender, Sky, Aqua, Mint, Lime, Gold, Orange, Coral, Pink, Violet, Silver | `AccentHues` |
| Background | 14 backdrops: Stars (the default), Snow, Rain, Fireflies, Bubbles, Petals, Code, Embers, Aurora, Clouds, Leaves, Meteor shower, Warp, Plain | `Scenes` in `ui/scene.go` |
| Brightness | Dim, Normal, Bright: the backdrop's lightness; panels and text stay as they are | `brightnesses` |
| Motion | Off, Calm, Normal, Lively: how many sprites and how fast | `motions`, `motionPace` in `ui/sky.go` |

A name the library doesn't know (from a newer version, or a damaged
file) falls back to the default for that setting. The older `"still": true`
still reads as Motion Off, and Motion Off writes it too, so apps built
before Motion keep the background still.

A theme is the Night palette with its hues
turned: one hue for the background family (backdrop, panels, keys, text)
and one for the accent family (the selection, focus, the title's shadow).
Lightness stays the same, so text keeps its contrast; a theme can also
darken its surfaces (`bgLight`). An accent turns the accent family alone,
from Night's colours. Every theme assumes light text on dark surfaces.
`TestThemeContrast` checks every text and surface pair the interface
draws, in every theme with every accent and every brightness. To add a
theme or an accent, add it to `Themes` or `AccentHues` and run the tests;
if a pair fails, change the theme's parameters, not the thresholds.

A background (`ui/scene.go`) is a function for its fixed marks
and a spawn function for its moving sprites. Sprites draw only on plain
background (through the sky's `put`), a few cells away from anything an
app drew, and must finish on their own. A spawn function starts sprites
with `k.chance(p)` under a cap of `k.room(n)`, so Motion scales them, and a
sprite's `step` moves by `dt` (more at Lively, less at Calm). The sky has
at most `maxSprites`; `TestScenesStress` runs every background for 200
simulated seconds at each Motion and checks the rules, and
`BenchmarkSceneTick` gives each one's cost a tick (keep a new one within
the range of the others). Apps read the colours (`ui.Text`, `ui.Surface`,
...) when they draw: keep nothing computed from them across frames, or
rebuild it after `Screen.Appearance` returns.

## Icons and covers

`scripts/make-icons.py` draws every app icon into `icons/`; each app
repository keeps its own copy as `app/icon.png`. `tools/cover` draws a
README cover from screenshots (`make screens` in an app, with
`MINIDECK_THEME` set for other themes):

```sh
go run ./tools/cover -title MiniTape -sub "Navidrome and Subsonic on the Miyoo Mini Plus" \
  -icon icons/MiniTape.png -o cover.png now-playing.png:"Now playing" album.png:Album
```

## Releases

An app is released from its own repository: move the "Unreleased" notes in
its `CHANGELOG.md` under the new version, commit, tag `vx.y.z` and push the
tag. Its release workflow builds the binaries, stamped with the version,
and publishes `<name>-x.y.z.zip` (laid out to unzip onto the SD card root),
`app.json` (its catalogue entry) and `SHA256SUMS`.

This repository's Bundle workflow (`tools/bundle`) then collects every app's
latest release, checks each zip against its `app.json`, and publishes
`catalog.json`, `minideck-all.zip` and `SHA256SUMS` as MiniDeck's latest
release. MiniMart's built-in "MiniDeck" source reads that `catalog.json`.
It runs every day and by hand (Actions > Bundle > Run workflow).

The library is versioned with tags like any Go module; an app picks up a
new one with `go get github.com/zakijss/minideck@vx.y.z`.

## Adding an app

1. A new repository shaped like the others; Network (a list with tabs) and
   MiniPulse (one screen with background work) are good models.
2. `cmd/<name>/main.go`: call `version.Flag("<name>")` and
   `version.Note("<name>")` first, then `ui.Open()` and an event loop.
3. `app/`: `config.json`, a `launch.sh` modelled on the others (log cap,
   CPU minimum while open), and an icon drawn in `scripts/make-icons.py`.
4. `store.json`, screen tests (`TestScreens`, writing into
   `MINIDECK_GALLERY`) and unit tests for anything that parses or computes.
5. Its repository in `tools/bundle`'s list, so the bundle and MiniMart
   pick it up.
