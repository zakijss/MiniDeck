# Contributing to MiniDeck

Bug reports, ideas and pull requests are welcome, in the repository of the
app they concern, or here for the shared library and the bundle. Use the
issue forms; for security problems, see [SECURITY.md](SECURITY.md).

[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) explains how the repositories are
laid out and how to build, run and test them. In short: Docker, git and make
are enough, and `make check` runs what CI runs, in every repository.

The apps are static Go binaries that draw to the framebuffer, and the device
scripts are plain `sh` for Onion's BusyBox. Please keep it that way: a
change that needs SDL, a C toolchain or a new Go module is better raised as
an issue first. The apps target the Miyoo Mini Plus on Onion OS only;
support for other devices or firmware is out of scope.

## Pull requests

- Run `make check`. CI also runs the tests as ARMv7 binaries under qemu.
- If you change a screen (in an app, or the library's `ui`), run
  `make screens` in the app and look at the result.
  Include a before and after image in the pull request.
- Test on a real device where you can, and say in the pull request whether
  you did, on which Onion version.
- Keep a pull request to one change, and say what changed and why.
- Update the README where the change matters to someone using the apps, and
  add a line under "Unreleased" in [CHANGELOG.md](CHANGELOG.md).

Code follows the usual Go conventions (`gofmt`, `go vet`, staticcheck).
Comments say why the code does what it does; error messages on screen say
what to do next.

By contributing you agree that your work is released under the
[MIT licence](LICENSE).
