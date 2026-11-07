# Changelog

Changes to the MiniDeck library and tools. Each app keeps its own
changelog in its repository. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.0.2] - 2026-11-07

### Fixed

- `ui`: `ScaleCrop` works out its crop in 64-bit, so a very wide picture can't overflow on the device's 32-bit int.
- `tools/bundle`: a zip entry named `..` is refused, like any other path that leaves the SD card root.
- `ui`: a list row's value on the right is cut with an ellipsis when it would run into the label, instead of being drawn across it and out of the row (a long subtitle track name in MiniFin).

## [1.0.1] - 2026-11-02

### Fixed

- The on-screen keyboard had no visible way out: B only deleted, and MENU (which cancels) wasn't shown. Now B on an empty field goes back (a held B that clears the text doesn't), and the footer shows B Back, or MENU Cancel once there is text.

## [1.0.0] - 2026-11-02

### Added

- `ui`: a pixel-art interface drawn straight to the framebuffer, with
  lists, forms, an on-screen keyboard, a status bar and a now-playing
  ticker; text in Latin, Greek, Cyrillic, Arabic, Hebrew, Chinese,
  Japanese, Korean, Thai and most other scripts; and Appearance, shared by
  every app.
- `ui` Appearance: sixteen dark themes (new: Dusk, Mint, Sand, Slate,
  Coffee, Sakura, Deep sea, Neon), an accent colour in place of the
  theme's own (eleven to pick from), fourteen animated backgrounds (new:
  Embers, Aurora, Clouds, Leaves, Meteor shower, Warp), the backdrop's
  brightness (Dim, Normal, Bright) and Motion in four steps (Off, Calm,
  Normal, Lively). Older `appearance.json` files still read, and
  `MINIDECK_ACCENT` sets the accent for screenshots.
- `ui` status bar: Wi-Fi shows its signal in three steps (from
  `/proc/net/wireless`), the battery its charge in colour and a bolt while
  charging (GPIO 59 on the Mini, `axp_test` on the Mini Plus, as Onion
  reads it), all drawn with the app icons' hard shadow.
- `ui` now-playing strip: a small moving equaliser, the title brighter
  than the artist and album, and `Screen.SetNowPlaying(title, detail)`;
  redrawing it allocates nothing.
- `tnet`: reaching the tailnet through MiniScale's proxies, with
  Mozilla's root certificates for HTTPS.
- `secrets`: small secrets sealed with the device key.
- `cache`, `atomicfile`, `appdir`, `procs`, `qr`, `fb`, `version`.
- `tools/bundle`: the all-apps zip and MiniMart's catalogue, from the
  apps' own releases.
- `tools/cover`, `tools/mockhub` and the app icons.

[Unreleased]: https://github.com/zakijss/MiniDeck/compare/v1.0.2...HEAD
[1.0.2]: https://github.com/zakijss/MiniDeck/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/zakijss/MiniDeck/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/zakijss/MiniDeck/releases/tag/v1.0.0
