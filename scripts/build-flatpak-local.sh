#!/usr/bin/env bash
#
# scripts/build-flatpak-local.sh — Build WeatherWidget as a local Flatpak bundle.
#
# Produces a single-file `.flatpak` bundle (installable with
# `flatpak install <file>`) and copies it into scripts/build/ next to the
# other Linux artifacts produced by scripts/build-linux.sh.
#
# This script is ADDITIVE and NON-MUTATING: it only *reads* the existing
# Flathub packaging files under flatpak/ (the manifest, build-flatpak.sh, the
# generated go.mod.yml / modules.txt, the desktop/metainfo files). It never
# edits, regenerates, or overwrites any of them, and it keeps all of its own
# build scratch space in a temporary directory so the flatpak/ tree and the
# automated Flathub build are left untouched.
#
# Prerequisites (must already be present — the script will NOT create them):
#   - flatpak + flatpak-builder installed on the host,
#   - the GNOME runtime/SDK + Go SDK extension referenced by the manifest,
#   - flatpak/shared-modules/ checked out (git submodule/clone, per
#     flatpak/README.md — required by the manifest for local builds),
#   - flatpak/go.mod.yml + flatpak/modules.txt (generated offline Go sources).
#
# If a prerequisite is missing the script stops with a clear message telling
# you which existing tool/command to run — it does not mutate the repo for you.
#
# Usage:
#   ./scripts/build-flatpak-local.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FLATPAK_DIR="$PROJECT_ROOT/flatpak"
BUILD_DIR="$SCRIPT_DIR/build"

APP_ID="uk.co.easysmartapps.WeatherWidget"
MANIFEST="$FLATPAK_DIR/${APP_ID}.yml"

APP_VERSION="$(cat "$PROJECT_ROOT/release" 2>/dev/null | tr -d '[:space:]')"
[ -z "$APP_VERSION" ] && APP_VERSION="dev"

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  DEB_ARCH="amd64" ;;
    aarch64) DEB_ARCH="arm64" ;;
    *)       DEB_ARCH="$ARCH" ;;
esac

# Keep ALL flatpak-builder scratch state in a temp dir outside flatpak/ so we
# never add build-dir/, repo/, or .flatpak-builder/ into the existing tree.
# Cleaned up on exit.
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/weatherwidget-flatpak.XXXXXX")"

cleanup() { rm -rf "$WORK_DIR"; }
trap cleanup EXIT

BUILDER_STATE_DIR="$WORK_DIR/.flatpak-builder"
BUILDER_BUILD_DIR="$WORK_DIR/build-dir"
BUILDER_REPO_DIR="$WORK_DIR/repo"

BUNDLE_OUTPUT="$BUILD_DIR/${APP_ID}_${APP_VERSION}_${DEB_ARCH}.flatpak"

# ---------------------------------------------------------------------------
# Prerequisite checks (read-only — never create or modify anything)
# ---------------------------------------------------------------------------
require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: '$1' is not installed. $2" >&2
        exit 1
    fi
}

check_prereqs() {
    require_cmd flatpak "Install it (e.g. 'sudo apt install flatpak' or 'sudo dnf install flatpak')."
    require_cmd flatpak-builder "Install it (e.g. 'sudo apt install flatpak-builder' or 'sudo dnf install flatpak-builder')."

    if [ ! -f "$MANIFEST" ]; then
        echo "ERROR: manifest not found at $MANIFEST" >&2
        exit 1
    fi

    # Local build dependency required by the manifest (flatpak/README.md).
    if [ ! -d "$FLATPAK_DIR/shared-modules" ] || [ -z "$(ls -A "$FLATPAK_DIR/shared-modules" 2>/dev/null)" ]; then
        echo "ERROR: flatpak/shared-modules/ is missing or empty." >&2
        echo "       The manifest references it for the system-tray dependency." >&2
        echo "       Check it out first (from the flatpak/ directory):" >&2
        echo "         git clone https://github.com/flathub/shared-modules.git flatpak/shared-modules" >&2
        exit 1
    fi

    # Offline Go sources required by the manifest. Generated separately by
    # flatpak/generate-sources.sh — this script must NOT regenerate them.
    if [ ! -f "$FLATPAK_DIR/go.mod.yml" ] || [ ! -f "$FLATPAK_DIR/modules.txt" ]; then
        echo "ERROR: flatpak/go.mod.yml and/or flatpak/modules.txt are missing." >&2
        echo "       Generate them once with:" >&2
        echo "         ./flatpak/generate-sources.sh" >&2
        exit 1
    fi
}

# ---------------------------------------------------------------------------
# Build with the existing manifest, then export a single-file bundle and copy
# it into scripts/build/.
# ---------------------------------------------------------------------------
# The manifest declares runtime-version 51 (GNOME) but lists the Go SDK
# extension without a branch. The host flatpak-builder resolves sdk-extensions
# against the runtime-version (51), and there is no golang extension at branch
# 51 — it ships on the freedesktop SDK branch the GNOME runtime is based on
# (e.g. 25.08). The Flatpak-packaged builder (org.flatpak.Builder) resolves the
# extension against the SDK's base version correctly, which is also exactly what
# Flathub CI uses. So we prefer it when available and fall back to host
# flatpak-builder otherwise.
#
# BUILDER_KIND is "flatpak" (org.flatpak.Builder) or "host" (plain binary).
BUILDER_KIND=""
select_builder() {
    if flatpak --user info org.flatpak.Builder >/dev/null 2>&1 \
        || flatpak info org.flatpak.Builder >/dev/null 2>&1; then
        BUILDER_KIND="flatpak"
        echo "==> Using org.flatpak.Builder (Flathub's builder)"
        return
    fi
    echo "==> org.flatpak.Builder not installed; installing it (matches Flathub CI)..."
    if flatpak install --user -y flathub org.flatpak.Builder >/dev/null 2>&1; then
        BUILDER_KIND="flatpak"
        echo "==> Using org.flatpak.Builder (Flathub's builder)"
        return
    fi
    echo "==> Falling back to host flatpak-builder"
    BUILDER_KIND="host"
}

run_builder() {
    # Args: passed straight through to the builder after the common flags.
    if [ "$BUILDER_KIND" = "flatpak" ]; then
        # org.flatpak.Builder is sandboxed; grant it read access to the project
        # tree and read/write to the scratch dir and the output folder so it can
        # read sources and write the repo/build-dir.
        flatpak run \
            --filesystem="$PROJECT_ROOT" \
            --filesystem="$WORK_DIR" \
            --filesystem="$BUILD_DIR" \
            org.flatpak.Builder "$@"
    else
        flatpak-builder "$@"
    fi
}

build_and_export() {
    mkdir -p "$BUILD_DIR"

    select_builder

    echo "==> Building Flatpak (scratch dir: $WORK_DIR)..."
    # Run from the flatpak/ dir so the manifest's `path: ..` (project root) and
    # `shared-modules/...` module references resolve exactly as the automated
    # build expects. All output dirs live under $WORK_DIR, not flatpak/.
    (
        cd "$FLATPAK_DIR"
        run_builder \
            --user \
            --force-clean \
            --disable-rofiles-fuse \
            --repo="$BUILDER_REPO_DIR" \
            --state-dir="$BUILDER_STATE_DIR" \
            "$BUILDER_BUILD_DIR" \
            "$(basename "$MANIFEST")"
    )

    echo "==> Exporting single-file bundle..."
    rm -f "$BUNDLE_OUTPUT"
    flatpak build-bundle \
        "$BUILDER_REPO_DIR" \
        "$BUNDLE_OUTPUT" \
        "$APP_ID"

    echo "    Created: $BUNDLE_OUTPUT"
}

main() {
    check_prereqs
    build_and_export

    echo ""
    echo "==> Flatpak build complete! Output in: $BUILD_DIR/"
    ls -lh "$BUNDLE_OUTPUT" 2>/dev/null || true
    echo ""
    echo "Install locally with:"
    echo "    flatpak install --user \"$BUNDLE_OUTPUT\""
    echo "Run with:"
    echo "    flatpak run $APP_ID"
}

main "$@"
