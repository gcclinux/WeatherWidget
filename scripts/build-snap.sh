#!/bin/bash
set -e

# WeatherWidget Snap Build Script
# Builds a strict-confinement Snap package for Linux using the native GTK3
# binary (cmd/weatherwidget-gtk). Falls back to the Fyne binary if GTK3
# development headers are not available on the build host.

APP_NAME="WeatherWidget"
BINARY_NAME="weatherwidget"

# GTK binary is the preferred entrypoint for the snap.
GTK_CMD_PATH="./cmd/weatherwidget-gtk/"
FYNE_CMD_PATH="./cmd/weatherwidget/"

APP_ICON="assets/icons/clear.png"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION=$(cat "$PROJECT_ROOT/release" 2>/dev/null | tr -d '[:space:]')
if [ -z "$VERSION" ]; then
    echo "Error: could not read version from 'release' file"
    exit 1
fi

echo "==> Preparing WeatherWidget $VERSION Snap package build (GTK3 native)..."

# Detect Go binary
if [ -x "/usr/local/go/bin/go" ]; then
    GO_CMD="/usr/local/go/bin/go"
elif [ -x "/snap/bin/go" ]; then
    GO_CMD="/snap/bin/go"
else
    GO_CMD="go"
fi

# Decide which binary to package: prefer GTK3, fall back to Fyne.
USE_GTK=true
if ! pkg-config --exists gtk+-3.0 ayatana-appindicator3-0.1 2>/dev/null; then
    echo "    NOTE: GTK3/appindicator headers not found — snap will use Fyne binary."
    USE_GTK=false
fi

if $USE_GTK; then
    SELECTED_CMD="$GTK_CMD_PATH"
    echo "    Selected binary: GTK3 native (cmd/weatherwidget-gtk)"
else
    SELECTED_CMD="$FYNE_CMD_PATH"
    echo "    Selected binary: Fyne cross-platform (cmd/weatherwidget)"
fi

# Ensure snap/gui directory exists
mkdir -p "$PROJECT_ROOT/snap/gui"

# Copy app icon
if [ -f "$PROJECT_ROOT/$APP_ICON" ]; then
    echo "==> Copying app icon..."
    cp "$PROJECT_ROOT/$APP_ICON" "$PROJECT_ROOT/snap/gui/$BINARY_NAME.png"
fi

# Create desktop entry
DESKTOP_FILE="$PROJECT_ROOT/snap/gui/$BINARY_NAME.desktop"
echo "==> Writing snap/gui/$BINARY_NAME.desktop..."
cat > "$DESKTOP_FILE" <<EOF
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

# Build the snapcraft.yaml — GTK3 build or Fyne build depending on availability.
SNAPCRAFT_YAML="$PROJECT_ROOT/snap/snapcraft.yaml"
echo "==> Writing snap/snapcraft.yaml..."

if $USE_GTK; then
    # GTK3 native build using the gnome extension for strict confinement.
    # The 'gnome' extension (core24 + gnome-46-2404) provides GTK3, Pango, Cairo,
    # and all GNOME runtime libs via a content snap — no staging needed and no
    # glibc mismatch. Strict confinement is required for Snap Store submission.
    #
    # Build requirement: must be built on Ubuntu 24.04 (core24).
    # On Ubuntu 26+: the build script falls back to Docker automatically.
    cat > "$SNAPCRAFT_YAML" <<EOF
name: $BINARY_NAME
title: $APP_NAME
summary: Compact desktop weather widget (GTK3 native)
description: |
  WeatherWidget is a compact, customizable weather widget for your Linux desktop.
  Native GTK3 build with true transparency, system tray via appindicator3,
  and first-class X11/XWayland support. Supports multiple city locations,
  opacity/transparency, and customizable weather providers.

confinement: strict
base: core24
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
    desktop: meta/gui/$BINARY_NAME.desktop
    autostart: $BINARY_NAME.desktop
    extensions: [gnome]
    environment:
      GDK_BACKEND: x11
    plugs:
      - home
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
      - golang
      - gcc
      - g++
      - pkg-config
      - libgtk-3-dev
      - libglib2.0-dev
      - libcairo2-dev
      - libayatana-appindicator3-dev
    stage-packages:
      - libayatana-appindicator3-1
      - libayatana-indicator3-7
      - libdbusmenu-glib4
      - libdbusmenu-gtk3-4
      - wmctrl
      - xdotool
      - x11-utils
      - x11-xserver-utils
      - libxdo3
    override-build: |
      VERSION=\$(cat release 2>/dev/null | tr -d '[:space:]')
      if [ -z "\$VERSION" ]; then
        VERSION="dev"
      fi
      craftctl set-version "\$VERSION" 2>/dev/null || snapcraftctl set-version "\$VERSION"
      # Use go from PATH (installed via build-packages: golang on core24)
      GO_BIN=\$(command -v go || echo "/usr/local/go/bin/go")
      DEST_DIR="\${CRAFT_PART_INSTALL:-\$SNAPCRAFT_PART_INSTALL}"
      mkdir -p "\$DEST_DIR/bin"
      unset CFLAGS CPPFLAGS LDFLAGS
      export CGO_CFLAGS="-Wno-deprecated-declarations \$(pkg-config --cflags gtk+-3.0 ayatana-appindicator3-0.1 2>/dev/null)"
      export CGO_LDFLAGS="\$(pkg-config --libs gtk+-3.0 ayatana-appindicator3-0.1 2>/dev/null)"
      CGO_ENABLED=1 \$GO_BIN build -p 1 -v \\
        -ldflags="-s -w -X main.version=\$VERSION" \\
        -o "\$DEST_DIR/bin/$BINARY_NAME" \\
        $GTK_CMD_PATH
EOF

else
    # Fyne fallback build — same as original, for systems without GTK3 headers.
    cat > "$SNAPCRAFT_YAML" <<EOF
name: $BINARY_NAME
title: $APP_NAME
summary: Compact desktop weather widget
description: |
  WeatherWidget is a compact, customizable weather widget for your desktop.
  Supports multiple city locations, opacity/transparency options, and
  customizable weather providers.

confinement: strict
base: core24
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
    desktop: meta/gui/$BINARY_NAME.desktop
    autostart: $BINARY_NAME.desktop
    extensions: [gnome]
    plugs:
      - home
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
    stage-packages:
      - wmctrl
      - xdotool
      - x11-utils
      - x11-xserver-utils
      - libxmu6
      - libxdo3
    stage:
      - bin/*
      - usr/bin/wmctrl
      - usr/bin/xdotool
      - usr/bin/xprop
      - usr/bin/xrandr
      - usr/bin/xwininfo
      - usr/lib/*/libXmu.so*
      - usr/lib/*/libxdo.so*
      - -usr/lib/*/libc.so*
      - -usr/lib/*/libGL*
      - -usr/lib/*/libX11.so*
      - -usr/lib/*/libxcb.so*
      - -usr/lib/*/libXdmcp.so*
    override-build: |
      VERSION=\$(cat release 2>/dev/null | tr -d '[:space:]')
      if [ -z "\$VERSION" ]; then
        VERSION="dev"
      fi
      craftctl set-version "\$VERSION" 2>/dev/null || snapcraftctl set-version "\$VERSION"
      GO_BIN=\$(command -v go || echo "/usr/local/go/bin/go")
      DEST_DIR="\${CRAFT_PART_INSTALL:-\$SNAPCRAFT_PART_INSTALL}"
      mkdir -p "\$DEST_DIR/bin"
      \$GO_BIN build -v -ldflags="-s -w -X main.version=\$VERSION" -o "\$DEST_DIR/bin/$BINARY_NAME" $FYNE_CMD_PATH
EOF
fi

echo "    Snap metadata written."
echo ""

# Build snap using snapcraft if installed
if command -v snapcraft >/dev/null 2>&1; then
    echo "==> Executing snapcraft to build package..."
    cd "$PROJECT_ROOT"

    # Detect host Ubuntu version — core24 snaps must be built on Ubuntu 24.x.
    HOST_VER=$(lsb_release -rs 2>/dev/null | cut -d. -f1)
    NEED_DOCKER=false
    if [ -n "$HOST_VER" ] && [ "$HOST_VER" -ge 25 ] 2>/dev/null; then
        NEED_DOCKER=true
    fi

    if $NEED_DOCKER; then
        echo "    Host is Ubuntu $HOST_VER — core24 snap requires Ubuntu 24."
        if command -v docker >/dev/null 2>&1; then
            echo "    Using Docker (ubuntu:24.04) to build..."
            docker run --rm \
                -v "$PROJECT_ROOT":/build \
                -w /build \
                ubuntu:24.04 bash -c "
                    set -e
                    apt-get update -qq
                    apt-get install -y -qq snapcraft golang gcc g++ pkg-config \
                        libgtk-3-dev libglib2.0-dev libcairo2-dev \
                        libayatana-appindicator3-dev 2>/dev/null
                    snapcraft pack --destructive-mode
                "
        else
            echo "    Docker not found. Install Docker or build on Ubuntu 24.x:"
            echo "      sudo apt install docker.io"
            echo "      sudo usermod -aG docker $USER  # then log out/in"
            echo "    Or build manually on Ubuntu 24.04:"
            echo "      snapcraft pack --destructive-mode"
            exit 1
        fi
    else
        # Same Ubuntu version — try LXD/Multipass first, fall back to destructive.
        if ! snapcraft pack "$@" 2>/dev/null; then
            echo "    Container build failed — retrying with --destructive-mode..."
            sudo rm -rf parts stage prime overlay
            sudo snapcraft pack --destructive-mode "$@"
        fi
    fi

    # Fix permissions of all generated files/directories back to default host user
    HOST_UID=$(id -u)
    HOST_GID=$(id -g)
    if [ -n "$HOST_UID" ] && [ -n "$HOST_GID" ]; then
        if command -v docker >/dev/null 2>&1 && [ "$NEED_DOCKER" = "true" ]; then
            docker run --rm -v "$PROJECT_ROOT":/build -w /build ubuntu:24.04 chown -R "$HOST_UID:$HOST_GID" . 2>/dev/null || true
        elif command -v sudo >/dev/null 2>&1; then
           sudo chown -R "$HOST_UID:$HOST_GID" "$PROJECT_ROOT/snap" "$PROJECT_ROOT"/*.snap "$SCRIPT_DIR/build" 2>/dev/null || true
           sudo rm -rf parts stage prime overlay 2>/dev/null || true
           sudo snapcraft clean
        fi
    fi

    echo ""
    mkdir -p "$SCRIPT_DIR/build"
    SNAP_FILE=$(find "$PROJECT_ROOT" -maxdepth 1 -name "${BINARY_NAME}_*.snap" | head -n 1)
    if [ -n "$SNAP_FILE" ] && [ -f "$SNAP_FILE" ]; then
        mv "$SNAP_FILE" "$SCRIPT_DIR/build/"
        echo "==> Snap package build complete!"
        echo "    Moved to: $SCRIPT_DIR/build/$(basename "$SNAP_FILE")"
    else
        echo "==> Snap package build complete!"
        echo "    Check $PROJECT_ROOT for ${BINARY_NAME}_${VERSION}_*.snap"
    fi
else
    echo "==> Snapcraft metadata preparation complete."
    echo ""
    echo "    To build the snap, install snapcraft and run:"
    echo "        sudo snap install snapcraft --classic"
    if $USE_GTK; then
        echo "        cd $PROJECT_ROOT && sudo snapcraft pack --destructive-mode"
    else
        echo "        cd $PROJECT_ROOT && sudo snapcraft pack --destructive-mode"
        echo ""
        echo "    NOTE: To build the GTK3 native snap, install the GTK3 and"
        echo "    ayatana-appindicator3 development headers first:"
        echo "        sudo apt install libgtk-3-dev libayatana-appindicator3-dev"
    fi
fi
