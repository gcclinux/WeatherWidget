#!/usr/bin/env bash
#
# build-flatpak.sh — Build and install WeatherWidget inside the Flatpak sandbox.
#
# This is invoked by the flatpak-builder manifest (buildsystem: simple) instead
# of a long list of inline build-commands. Keeping the logic here means the
# build is maintained upstream (in the app repo) and is testable on its own.
#
# It expects to run from the source tree root (flatpak-builder copies the repo
# in and runs here), with the Go SDK extension on PATH and ./vendor already
# populated by the go.mod.yml sources.
#
set -euo pipefail

PREFIX="${FLATPAK_DEST:-/app}"
APP_ID="uk.co.easysmartapps.WeatherWidget"
ICON_SRC="assets/icons/day/clear_day.png"

# --- Build ------------------------------------------------------------------
export GOPATH="${PWD}/.gopath"
VERSION="$(cat release 2>/dev/null | tr -d '[:space:]')"
[ -z "${VERSION}" ] && VERSION="dev"

echo "==> Building weatherwidget ${VERSION} (offline, vendored)"
go build -v -mod=vendor \
  -ldflags="-s -w -X main.version=${VERSION}" \
  -o weatherwidget \
  ./cmd/weatherwidget-gtk/

# --- Install ----------------------------------------------------------------
echo "==> Installing to ${PREFIX}"
install -Dm755 weatherwidget "${PREFIX}/bin/weatherwidget"

# Desktop entry + AppStream metainfo are maintained in this repo (upstream) and
# installed from here.
install -Dm644 "flatpak/${APP_ID}.desktop" \
  "${PREFIX}/share/applications/${APP_ID}.desktop"
install -Dm644 "flatpak/${APP_ID}.metainfo.xml" \
  "${PREFIX}/share/metainfo/${APP_ID}.metainfo.xml"

# Icons at the standard hicolor sizes.
for size in 64 128 256; do
  install -Dm644 "${ICON_SRC}" \
    "${PREFIX}/share/icons/hicolor/${size}x${size}/apps/${APP_ID}.png"
done

echo "==> Done"
