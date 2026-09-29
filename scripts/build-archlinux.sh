#!/usr/bin/env bash
#
# scripts/build-archlinux.sh — Build and package WeatherWidget for Arch Linux
#                               using the archlinux/PKGBUILD (makepkg).
#
# Usage:
#   ./scripts/build-archlinux.sh [build|install|srcinfo]
#
#   build     (default) Bump pkgver, refresh checksums, run makepkg, and copy
#             the resulting .pkg.tar.zst into scripts/build/.
#   install   Same as build, then install the package with `pacman -U`.
#   srcinfo   Only regenerate archlinux/.SRCINFO from the current PKGBUILD.
#
# The PKGBUILD fetches its source from the tagged GitHub release
# (https://github.com/gcclinux/WeatherWidget/archive/refs/tags/vX.Y.Z.tar.gz),
# so the tag for the requested version must already be pushed to GitHub.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PKG_DIR="$PROJECT_ROOT/archlinux"
BUILD_DIR="$SCRIPT_DIR/build"

APP_VERSION="$(cat "$PROJECT_ROOT/release" 2>/dev/null | tr -d '[:space:]')"
if [ -z "$APP_VERSION" ]; then
    APP_VERSION="1.0.6"
fi

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "    ERROR: '$1' not found. Install it with: pacman -S $2"
        exit 1
    fi
}

bump_pkgver() {
    echo "==> Setting pkgver=$APP_VERSION in PKGBUILD..."
    sed -i "s/^pkgver=.*/pkgver=$APP_VERSION/" "$PKG_DIR/PKGBUILD"
    if ! grep -q "^pkgrel=" "$PKG_DIR/PKGBUILD"; then
        sed -i "/^pkgver=/a pkgrel=1" "$PKG_DIR/PKGBUILD"
    fi
}

refresh_checksums() {
    echo "==> Refreshing sha256sums (downloads source tarball for v$APP_VERSION)..."
    require_cmd updpkgsums pacman-contrib
    (cd "$PKG_DIR" && updpkgsums)
}

gen_srcinfo() {
    echo "==> Generating .SRCINFO..."
    (cd "$PKG_DIR" && makepkg --printsrcinfo > .SRCINFO)
    echo "    Created: $PKG_DIR/.SRCINFO"
}

run_makepkg() {
    local extra_args=("$@")
    echo "==> Running makepkg..."
    require_cmd makepkg pacman
    (cd "$PKG_DIR" && makepkg --cleanbuild -f --syncdeps --noconfirm "${extra_args[@]}")

    mkdir -p "$BUILD_DIR"
    local built_pkg
    built_pkg="$(cd "$PKG_DIR" && ls -t weatherwidget-*.pkg.tar.zst 2>/dev/null | head -1)"
    if [ -z "$built_pkg" ]; then
        echo "    ERROR: no .pkg.tar.zst produced by makepkg."
        exit 1
    fi
    cp "$PKG_DIR/$built_pkg" "$BUILD_DIR/$built_pkg"
    echo "    Created: $BUILD_DIR/$built_pkg"
}

main() {
    local target="${1:-build}"

    case "$target" in
        srcinfo)
            gen_srcinfo
            ;;
        build)
            bump_pkgver
            refresh_checksums
            run_makepkg
            gen_srcinfo
            ;;
        install)
            bump_pkgver
            refresh_checksums
            run_makepkg --install
            gen_srcinfo
            ;;
        *)
            echo "Usage: $0 [build|install|srcinfo]"
            exit 1
            ;;
    esac

    echo ""
    echo "==> Done."
}

main "$@"
