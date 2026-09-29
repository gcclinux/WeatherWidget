# Arch Linux / AUR packaging

This directory contains the `PKGBUILD` used to build WeatherWidget as a native
Arch Linux package (`.pkg.tar.zst`), and to publish it on the
[Arch User Repository (AUR)](https://aur.archlinux.org/packages/weatherwidget).

The package builds the native GTK3 UI (`cmd/weatherwidget-gtk`) directly from
the tagged GitHub release source tarball — the same binary produced by
`scripts/build-linux.sh`.

## Building and testing locally

From the repo root:

```bash
./scripts/build-archlinux.sh
```

This bumps `pkgver` in `PKGBUILD` to match the `release` file, refreshes
`sha256sums`, runs `makepkg`, and copies the resulting `.pkg.tar.zst` into
`scripts/build/`. Add `install` as an argument to install it immediately with
`pacman -U`:

```bash
./scripts/build-archlinux.sh install
```

You can also drive `makepkg` directly from this directory:

```bash
cd archlinux
updpkgsums          # refresh sha256sums after bumping pkgver
makepkg -si          # build and install
namcap PKGBUILD *.pkg.tar.zst   # optional lint before publishing
```

## Publishing / updating the AUR package

The GitHub release referenced by `source=` **must exist** (i.e. `pkgver` must
match a pushed git tag `vX.Y.Z`) before `makepkg` can fetch the source tarball.

1. Tag and push the release on GitHub (see `scripts/bump_version.sh`).
2. Update `pkgver`/`pkgrel` in `PKGBUILD` and run `updpkgsums`.
3. Regenerate `.SRCINFO` (required by the AUR):
   ```bash
   makepkg --printsrcinfo > .SRCINFO
   ```
4. Clone (first time only) the AUR git repo and copy these two files in:
   ```bash
   git clone ssh://aur@aur.archlinux.org/weatherwidget.git aur-weatherwidget
   cp PKGBUILD .SRCINFO aur-weatherwidget/
   cd aur-weatherwidget
   git add PKGBUILD .SRCINFO
   git commit -m "Update to $(pkgver)"
   git push
   ```

An AUR account and an SSH key registered with it are required to push
(https://aur.archlinux.org/register). The package name reserved must match
`pkgname` in `PKGBUILD` (`weatherwidget`).
