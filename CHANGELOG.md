# Changelog

Changes to the MiniDeck library and tools. Each app keeps its own
changelog in its repository. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.3.0] - 2026-11-19

### Added

- `sign`: checks minisign signatures, prehashed (`ED`, streamed, any size) and legacy (`Ed`, up to 16 MB), including the key id and the signed trusted comment. `sign.ReleaseKey` is the owner's release key.
- `cmd/minisig`: a small static checker for the device and shell scripts (`minisig -p KEY -m FILE -x SIG`), built by `make minisig` and attached to the Bundle release as `minisig-armv7`.

## [1.2.0] - 2026-11-19

### Added

- `ui`: L2 and R2 go to a list's first and last row in every `List`.
- `ui`: `Battery()` returns the charge, whether it is charging and whether it is low, as the status bar shows them, so an app can hold off a big download.
- `ui`: a `folder` icon, yellow on a tile.
- `ui`: below 15% and not charging, the status bar's battery percentage turns red with the bar.

### Changed

- `ui`: holding Up or Down in a list or a form stops at the first or last row; a fresh press still goes round to the other end.

### Fixed

- `ui`: a held A no longer answers the question it opened (Remove in a form), and a held button no longer dismisses a notice before it can be read: `Confirm` and `Notice` wait for a fresh press.
- `ui`: a list whose items shrank under the selection (or whose `Sel` was set out of range, such as -1) drew nothing or crashed; it now keeps the selection on a row and fills the space.
- `ui`: a long error message or title no longer runs the alert card off the screen; it is cut with an ellipsis.
- `ui`: the scaled pictures kept for redrawing (cover art) are capped at 6 MB, so a long listening session can't use up the app's memory.
- `atomicfile`: two goroutines saving the same file at once could rename half-written data into place or fail; saves of one file now take turns. Each save also flushes the folder, so a power cut just after it keeps the new file.
- `tnet`: an IPv6 address without a port (`http://[::1]/`) was sent to the tailnet as if it were a device name, and Tailscale's own IPv6 addresses (fd7a:115c:a1e0::/48) weren't.
- `tnet`: a link-local address with its interface named (`http://[fe80::1%25eth0]/`) went through the tailnet proxy as if it were a device name. Link-local addresses now always stay direct.

## [1.1.0] - 2026-11-12

### Added

- tnet: while MiniScale joins a WireGuard or NetBird network, apps also send the addresses and names it lists in /tmp/miniscale/routes through its proxy. The tailnet rules are unchanged, and this device's own services always stay direct.

## [1.0.4] - 2026-11-12

### Changed

- Dependencies updated: golang.org/x/image 0.48.0, x/text 0.44.0, newer root certificates (x509roots/fallback); Go 1.27.4.

## [1.0.3] - 2026-11-08

### Fixed

- `ui`: grey and red small text is lighter, so it reads at 4.5:1 in every theme, accent and brightness.
- `ui`: secondary text on a selected row is darker, so it reads at 4.5:1 on the lavender.

### Changed

- `ui`: a failed server check says "Couldn't connect", and Remove warns that it can't be undone.
- `tnet`, `ui`: an empty tailnet says to type the address instead, under "Couldn't list devices".

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

[Unreleased]: https://github.com/zakijss/MiniDeck/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/zakijss/MiniDeck/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/zakijss/MiniDeck/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/zakijss/MiniDeck/compare/v1.0.4...v1.1.0
[1.0.4]: https://github.com/zakijss/MiniDeck/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/zakijss/MiniDeck/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/zakijss/MiniDeck/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/zakijss/MiniDeck/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/zakijss/MiniDeck/releases/tag/v1.0.0
