#!/bin/bash
set -e

# WeatherWidget Snap Build Script
# Prepares resources and builds a strict-confinement Snap package for Linux

APP_NAME="WeatherWidget"
BINARY_NAME="weatherwidget"
CMD_PATH="./cmd/weatherwidget/"
APP_ICON="assets/icons/clear.png"
VERSION=$(cat release 2>/dev/null | tr -d '[:space:]')

if [ -z "$VERSION" ]; then
    echo "Error: could not read version from 'release' file"
    exit 1
fi

echo "==> Preparing WeatherWidget $VERSION Snap package build..."

# Detect Go binary
if [ -x "/usr/local/go/bin/go" ]; then
    GO_CMD="/usr/local/go/bin/go"
elif [ -x "/snap/bin/go" ]; then
    GO_CMD="/snap/bin/go"
else
    GO_CMD="go"
fi

# Ensure snap/gui directory exists
mkdir -p snap/gui

# Copy app icon to snap/gui location if available
if [ -f "$APP_ICON" ]; then
    echo "==> Copying app icon from $APP_ICON to snap/gui/$BINARY_NAME.png..."
    cp "$APP_ICON" "snap/gui/$BINARY_NAME.png"
fi

# Ensure desktop file is present
if [ ! -f "snap/gui/$BINARY_NAME.desktop" ]; then
    echo "==> Creating snap/gui/$BINARY_NAME.desktop..."
    cat > "snap/gui/$BINARY_NAME.desktop" <<EOF
[Desktop Entry]
Name=$APP_NAME
GenericName=Weather Widget
Comment=Compact desktop weather widget
Exec=$BINARY_NAME
Icon=\${SNAP}/meta/gui/$BINARY_NAME.png
Terminal=false
Type=Application
Categories=Utility;Clock;Weather;
StartupWMClass=$BINARY_NAME
X-GNOME-Autostart-enabled=true
EOF
fi

# Ensure snapcraft.yaml exists
if [ ! -f "snap/snapcraft.yaml" ]; then
    echo "==> Creating snap/snapcraft.yaml..."
    cat > "snap/snapcraft.yaml" <<EOF
name: $BINARY_NAME
title: $APP_NAME
summary: Compact desktop weather widget
description: |
  WeatherWidget is a compact, customizable weather widget for your desktop system tray and screen.
  Supports multiple city locations, opacity/transparency options, and customizable weather providers.

confinement: strict
base: core22
adopt-info: $BINARY_NAME

license: AGPL-3.0
source-code: https://github.com/gcclinux/weatherwidget
website: https://easysmartapps.co.uk/weatherwidget
contact: https://easysmartapps.co.uk/contact
issues: https://github.com/gcclinux/weatherwidget/issues
donation: https://buy.stripe.com/bJe3cvaJOa650fQ8cPdZ603

apps:
  $BINARY_NAME:
    command: bin/$BINARY_NAME
    desktop: snap/gui/$BINARY_NAME.desktop
    autostart: $BINARY_NAME.desktop
    extensions: [gnome]
    plugs:
      - network
      - network-bind
      - desktop
      - desktop-legacy
      - x11
      - wayland
      - unity7
      - opengl
      - gsettings

parts:
  $BINARY_NAME:
    plugin: nil
    source: .
    source-type: local
    build-packages:
      - gcc
      - g++
      - pkg-config
      - libgl1-mesa-dev
      - xorg-dev
      - libx11-dev
      - libxrandr-dev
      - libxinerama-dev
      - libxi-dev
      - libxcursor-dev
      - libxxf86vm-dev
    override-build: |
      VERSION=\$(cat release 2>/dev/null | tr -d '[:space:]')
      if [ -z "\$VERSION" ]; then
        VERSION="dev"
      fi
      craftctl set-version "\$VERSION" 2>/dev/null || snapcraftctl set-version "\$VERSION"
      GO_BIN=\$(command -v go || echo "/usr/local/go/bin/go")
      DEST_DIR="\${CRAFT_PART_INSTALL:-\$SNAPCRAFT_PART_INSTALL}"
      mkdir -p "\$DEST_DIR/bin"
      \$GO_BIN build -v -ldflags="-s -w -X main.version=\$VERSION" -o "\$DEST_DIR/bin/$BINARY_NAME" $CMD_PATH
EOF
fi

# Build snap using snapcraft if installed
if command -v snapcraft >/dev/null 2>&1; then
    echo "==> Executing snapcraft pack to build package..."
    if ! snapcraft pack "$@"; then
        echo ""
        echo "==> Container VM builder (LXD/Multipass) failed."
        echo "    Cleaning stale build directories and retrying with --destructive-mode..."
        if [ "$EUID" -ne 0 ]; then
            sudo rm -rf parts stage prime
            sudo snapcraft pack --destructive-mode "$@"
        else
            rm -rf parts stage prime
            snapcraft pack --destructive-mode "$@"
        fi
    fi
    echo ""
    echo "==> Snap package build complete!"
    echo "    Check current directory for ${BINARY_NAME}_${VERSION}_*.snap"
else
    echo "==> Snapcraft metadata preparation complete."
    echo "    To build the snap package, install snapcraft and run:"
    echo "        sudo snap install snapcraft --classic"
    echo "        sudo snapcraft pack --destructive-mode"
fi
