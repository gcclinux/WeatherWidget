#!/bin/bash
set -e

# WeatherWidget Snap Build Script (Docker Container Build)
# Builds a strict-confinement Snap package for Linux using GTK3 native binary
# inside the official Canonical Snapcraft OCI image (ghcr.io/canonical/snapcraft:8_core24).
#
# All build dependencies, tools, and headers (GTK3, AppIndicator, Go)
# are installed inside the Docker container, isolating the host system.
#
# NOTE: This does NOT use the gnome extension (broken in container builds).
# Instead, GTK3 libraries are bundled directly via stage-packages.

APP_NAME="WeatherWidget"
BINARY_NAME="weatherwidget"
GTK_CMD_PATH="./cmd/weatherwidget-gtk/"
APP_ICON="assets/icons/day/clear_day.png"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION=$(cat "$PROJECT_ROOT/release" 2>/dev/null | tr -d '[:space:]')
if [ -z "$VERSION" ]; then
    echo "Error: could not read version from 'release' file"
    exit 1
fi

echo "==> Preparing WeatherWidget $VERSION Snap package build (Docker / Core24 GTK3 native)..."

# 1. Verify Docker installation
if ! command -v docker >/dev/null 2>&1; then
    echo "Error: Docker is not installed or not in PATH."
    echo "Please install Docker to use this build script:"
    echo "    sudo apt update && sudo apt install -y docker.io"
    echo "    sudo usermod -aG docker \$USER"
    echo "    (Log out and log back in for group changes to take effect)"
    exit 1
fi

# Ensure user can connect to Docker daemon
if ! docker info >/dev/null 2>&1; then
    echo "Error: Cannot connect to the Docker daemon."
    echo "Ensure Docker is running and your user has permissions to run docker."
    echo "Try: sudo systemctl start docker || sudo usermod -aG docker \$USER"
    exit 1
fi

# 2. Ensure snap/gui directory exists
mkdir -p "$PROJECT_ROOT/snap/gui"

# 3. Copy app icon
if [ -f "$PROJECT_ROOT/$APP_ICON" ]; then
    echo "==> Copying app icon..."
    cp "$PROJECT_ROOT/$APP_ICON" "$PROJECT_ROOT/snap/gui/$BINARY_NAME.png"
fi

# 4. Create desktop entry
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

# 5. Build the snapcraft.yaml (GTK3 native build for Ubuntu Core24)
SNAPCRAFT_YAML="$PROJECT_ROOT/snap/snapcraft.yaml"
echo "==> Writing snap/snapcraft.yaml..."

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

plugs:
  gnome-46-2404:
    interface: content
    target: \$SNAP/gnome-platform
    default-provider: gnome-46-2404
  gtk-3-themes:
    interface: content
    target: \$SNAP/data-dir/themes
    default-provider: gtk-common-themes
  icon-themes:
    interface: content
    target: \$SNAP/data-dir/icons
    default-provider: gtk-common-themes
  sound-themes:
    interface: content
    target: \$SNAP/data-dir/sounds
    default-provider: gtk-common-themes

environment:
  # Library paths
  LD_LIBRARY_PATH: \$SNAP/gnome-platform/usr/lib/x86_64-linux-gnu:\$SNAP/gnome-platform/usr/lib:\$SNAP/usr/lib/x86_64-linux-gnu:\$SNAP/usr/lib:\$LD_LIBRARY_PATH
  # GTK and GDK configuration
  GTK_PATH: \$SNAP/gnome-platform/usr/lib/x86_64-linux-gnu/gtk-3.0
  GTK_EXE_PREFIX: \$SNAP/gnome-platform/usr
  GTK_IM_MODULE_DIR: \$SNAP/gnome-platform/usr/lib/x86_64-linux-gnu/gtk-3.0/3.0.0/immodules
  GTK_IM_MODULE_FILE: \$SNAP/gnome-platform/usr/lib/x86_64-linux-gnu/gtk-3.0/3.0.0/immodules.cache
  GDK_PIXBUF_MODULE_FILE: \$SNAP/gnome-platform/usr/lib/x86_64-linux-gnu/gdk-pixbuf-2.0/2.10.0/loaders.cache
  # XDG paths for themes, icons, and data
  XDG_DATA_DIRS: \$SNAP/data-dir:\$SNAP/gnome-platform/usr/share:\$XDG_DATA_DIRS
  XDG_CONFIG_DIRS: \$SNAP/gnome-platform/etc/xdg:\$XDG_CONFIG_DIRS
  # Font configuration
  FONTCONFIG_PATH: \$SNAP/gnome-platform/etc/fonts
  FONTCONFIG_FILE: \$SNAP/gnome-platform/etc/fonts/fonts.conf
  # GIO/GLib
  GIO_MODULE_DIR: \$SNAP/gnome-platform/usr/lib/x86_64-linux-gnu/gio/modules
  GI_TYPELIB_PATH: \$SNAP/gnome-platform/usr/lib/x86_64-linux-gnu/girepository-1.0:\$SNAP/usr/lib/x86_64-linux-gnu/girepository-1.0
  # Locale
  LOCPATH: \$SNAP/gnome-platform/usr/lib/locale
  # Force X11 for window positioning
  GDK_BACKEND: x11

apps:
  $BINARY_NAME:
    command: bin/$BINARY_NAME
    desktop: meta/gui/$BINARY_NAME.desktop
    autostart: $BINARY_NAME.desktop
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

layout:
  /usr/lib/x86_64-linux-gnu/gdk-pixbuf-2.0:
    bind: \$SNAP/gnome-platform/usr/lib/x86_64-linux-gnu/gdk-pixbuf-2.0
  /usr/lib/x86_64-linux-gnu/gtk-3.0:
    bind: \$SNAP/gnome-platform/usr/lib/x86_64-linux-gnu/gtk-3.0
  /usr/share/glib-2.0:
    bind: \$SNAP/gnome-platform/usr/share/glib-2.0
  /usr/share/icons:
    bind: \$SNAP/data-dir/icons
  /usr/share/mime:
    bind: \$SNAP/gnome-platform/usr/share/mime
  /usr/share/xml/iso-codes:
    bind: \$SNAP/gnome-platform/usr/share/xml/iso-codes
  /etc/fonts:
    bind: \$SNAP/gnome-platform/etc/fonts
  /etc/gtk-3.0:
    bind: \$SNAP/gnome-platform/etc/gtk-3.0

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
      - librsvg2-common
      - shared-mime-info
    override-build: |
      VERSION=\$(cat release 2>/dev/null | tr -d '[:space:]')
      if [ -z "\$VERSION" ]; then
        VERSION="dev"
      fi
      craftctl set-version "\$VERSION" 2>/dev/null || snapcraftctl set-version "\$VERSION"
      GO_BIN=\$(command -v go || echo "/usr/local/go/bin/go")
      DEST_DIR="\${CRAFT_PART_INSTALL:-\$SNAPCRAFT_PART_INSTALL}"
      mkdir -p "\$DEST_DIR/bin"
      mkdir -p "\$DEST_DIR/data-dir/themes"
      mkdir -p "\$DEST_DIR/data-dir/icons"
      mkdir -p "\$DEST_DIR/data-dir/sounds"
      mkdir -p "\$DEST_DIR/gnome-platform"
      unset CFLAGS CPPFLAGS LDFLAGS
      export CGO_CFLAGS="-Wno-deprecated-declarations \$(pkg-config --cflags gtk+-3.0 ayatana-appindicator3-0.1 2>/dev/null)"
      export CGO_LDFLAGS="\$(pkg-config --libs gtk+-3.0 ayatana-appindicator3-0.1 2>/dev/null)"
      CGO_ENABLED=1 \$GO_BIN build -buildvcs=false -p 1 -v \\
        -ldflags="-s -w -X main.version=\$VERSION" \\
        -o "\$DEST_DIR/bin/$BINARY_NAME" \\
        $GTK_CMD_PATH
EOF

echo "    Snap metadata written."
echo ""

# 6. Clean previous build artifacts to avoid stale state
echo "==> Cleaning previous snapcraft build state..."
# Use docker to remove root-owned build artifacts from prior runs
if [ -d "$PROJECT_ROOT/parts" ] || [ -d "$PROJECT_ROOT/stage" ] || [ -d "$PROJECT_ROOT/prime" ]; then
    docker run --rm -v "$PROJECT_ROOT":/build -w /build ubuntu:24.04 rm -rf parts stage prime
fi

# 7. Execute build using official Canonical Snapcraft rock image
echo "==> Executing Snapcraft inside Docker container..."
cd "$PROJECT_ROOT"

docker run --rm \
    -v "$PROJECT_ROOT":/project \
    ghcr.io/canonical/snapcraft:8_core24 \
    pack --verbosity=brief

# Adjust file ownership of artifacts back to host user if needed
HOST_UID=$(id -u)
HOST_GID=$(id -g)
if [ -n "$HOST_UID" ] && [ -n "$HOST_GID" ]; then
    docker run --rm -v "$PROJECT_ROOT":/build -w /build ubuntu:24.04 chown -R "$HOST_UID:$HOST_GID" . 2>/dev/null || true
fi

echo ""
mkdir -p "$SCRIPT_DIR/build"
SNAP_FILE=$(find "$PROJECT_ROOT" -maxdepth 1 -name "${BINARY_NAME}_*.snap" | head -n 1)

if [ -n "$SNAP_FILE" ] && [ -f "$SNAP_FILE" ]; then
    mv "$SNAP_FILE" "$SCRIPT_DIR/build/" 2>/dev/null || cp "$SNAP_FILE" "$SCRIPT_DIR/build/"
    FINAL_SNAP="$SCRIPT_DIR/build/$(basename "$SNAP_FILE")"
    echo "==> Snap package build complete!"
    echo "    Saved: $FINAL_SNAP"
else
    echo "==> Snap package build complete!"
    echo "    Check $PROJECT_ROOT for ${BINARY_NAME}_${VERSION}_*.snap"
fi
