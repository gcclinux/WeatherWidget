#!/usr/bin/env bash
#
# scripts/build-linux.sh — Build WeatherWidget binaries and packages for Linux (.bin, .deb, .rpm, .AppImage)
#
# Usage:
#   ./scripts/build-linux.sh [bin|deb|rpm|appimage|all]
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

APP_NAME="weatherwidget"
APP_DISPLAY_NAME="WeatherWidget"
APP_VERSION="$(cat "$PROJECT_ROOT/release" 2>/dev/null | tr -d '[:space:]')"
if [ -z "$APP_VERSION" ]; then
    APP_VERSION="1.0.5"
fi

APP_DESCRIPTION="A compact desktop weather application"
APP_LICENSE="AGPL-3.0"
APP_MAINTAINER="WeatherWidget Team <support@weatherwidget.app>"
APP_CATEGORIES="Utility;Clock;"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64)  DEB_ARCH="amd64"; RPM_ARCH="x86_64" ;;
    aarch64) DEB_ARCH="arm64"; RPM_ARCH="aarch64" ;;
    *)       DEB_ARCH="$ARCH"; RPM_ARCH="$ARCH" ;;
esac

BUILD_DIR="$SCRIPT_DIR/build"
mkdir -p "$BUILD_DIR"

# Detect Go binary
if [ -x "/usr/local/go/bin/go" ]; then
    GO_CMD="/usr/local/go/bin/go"
elif [ -x "/snap/bin/go" ]; then
    GO_CMD="/snap/bin/go"
else
    GO_CMD="go"
fi

BIN_OUTPUT="$BUILD_DIR/${APP_NAME}_${APP_VERSION}_${DEB_ARCH}.bin"
GTK_BIN_OUTPUT="$BUILD_DIR/${APP_NAME}-gtk_${APP_VERSION}_${DEB_ARCH}.bin"

build_bin() {
    echo "==> Building standalone binary (Fyne): $(basename "$BIN_OUTPUT")..."
    $GO_CMD build -ldflags="-s -w -X main.version=$APP_VERSION" -o "$BIN_OUTPUT" "$PROJECT_ROOT/cmd/weatherwidget/"
    chmod +x "$BIN_OUTPUT"
    echo "    Created: $BIN_OUTPUT"
}

build_gtk_bin() {
    echo "==> Building standalone binary (GTK3 native): $(basename "$GTK_BIN_OUTPUT")..."
    if ! pkg-config --exists gtk+-3.0 2>/dev/null; then
        echo "    WARNING: GTK3 development headers not found (install libgtk-3-dev)."
        echo "    Skipping GTK binary build — Fyne binary still available."
        return 0
    fi
    if ! pkg-config --exists ayatana-appindicator3-0.1 2>/dev/null; then
        echo "    WARNING: AppIndicator headers not found (install libayatana-appindicator3-dev)."
        echo "    Skipping GTK binary build — Fyne binary still available."
        return 0
    fi
    # Suppress deprecation warnings from gotk3 and appindicator C headers.
    CGO_CFLAGS="${CGO_CFLAGS:-} -Wno-deprecated-declarations" \
    $GO_CMD build -p 1 \
        -ldflags="-s -w -X main.version=$APP_VERSION" \
        -o "$GTK_BIN_OUTPUT" \
        "$PROJECT_ROOT/cmd/weatherwidget-gtk/"
    chmod +x "$GTK_BIN_OUTPUT"
    echo "    Created: $GTK_BIN_OUTPUT"
}

prepare_staging() {
    local staging="$1"
    rm -rf "$staging"
    mkdir -p "$staging/usr/bin"
    mkdir -p "$staging/usr/share/applications"
    mkdir -p "$staging/usr/share/pixmaps"
    mkdir -p "$staging/usr/share/icons/hicolor/48x48/apps"
    mkdir -p "$staging/usr/share/icons/hicolor/64x64/apps"
    mkdir -p "$staging/usr/share/icons/hicolor/128x128/apps"
    mkdir -p "$staging/usr/share/icons/hicolor/256x256/apps"
    mkdir -p "$staging/usr/share/icons/hicolor/512x512/apps"
    mkdir -p "$staging/usr/share/icons/hicolor/scalable/apps"

    # Prefer the native GTK binary; fall back to Fyne if it wasn't built.
    if [ -f "$GTK_BIN_OUTPUT" ]; then
        cp "$GTK_BIN_OUTPUT" "$staging/usr/bin/$APP_NAME"
        echo "    Packaging: GTK3 native binary"
    else
        cp "$BIN_OUTPUT" "$staging/usr/bin/$APP_NAME"
        echo "    Packaging: Fyne binary (GTK build unavailable)"
    fi

    # Desktop file
    cat > "$staging/usr/share/applications/$APP_NAME.desktop" <<EOF
[Desktop Entry]
Name=$APP_DISPLAY_NAME
Comment=$APP_DESCRIPTION
Exec=$APP_NAME
Icon=$APP_NAME
Terminal=false
Type=Application
Categories=$APP_CATEGORIES
Keywords=weather;widget;forecast;clock;temperature;
StartupWMClass=$APP_NAME
EOF

    # Icon
    local icon_src=""
    if [ -f "$PROJECT_ROOT/assets/icons/clear.png" ]; then
        icon_src="$PROJECT_ROOT/assets/icons/clear.png"
    elif [ -f "$PROJECT_ROOT/snap/gui/weatherwidget.png" ]; then
        icon_src="$PROJECT_ROOT/snap/gui/weatherwidget.png"
    fi

    if [ -n "$icon_src" ]; then
        cp "$icon_src" "$staging/usr/share/pixmaps/$APP_NAME.png"
        cp "$icon_src" "$staging/usr/share/icons/hicolor/48x48/apps/$APP_NAME.png"
        cp "$icon_src" "$staging/usr/share/icons/hicolor/64x64/apps/$APP_NAME.png"
        cp "$icon_src" "$staging/usr/share/icons/hicolor/128x128/apps/$APP_NAME.png"
        cp "$icon_src" "$staging/usr/share/icons/hicolor/256x256/apps/$APP_NAME.png"
        cp "$icon_src" "$staging/usr/share/icons/hicolor/512x512/apps/$APP_NAME.png"
        cp "$icon_src" "$staging/usr/share/icons/hicolor/scalable/apps/$APP_NAME.png"
    fi
}

build_deb() {
    echo "==> Building .deb package..."
    local staging="$BUILD_DIR/deb-staging"
    prepare_staging "$staging"

    # DEBIAN control file
    mkdir -p "$staging/DEBIAN"
    cat > "$staging/DEBIAN/control" <<EOF
Package: $APP_NAME
Version: $APP_VERSION
Section: utils
Priority: optional
Architecture: $DEB_ARCH
Maintainer: $APP_MAINTAINER
Description: $APP_DESCRIPTION
 WeatherWidget is a compact, customizable desktop weather widget.
 Supports multiple cities, opacity options, and live weather updates.
EOF

    # DEBIAN postinst script (updates KDE / GNOME menu database and icon cache)
    cat > "$staging/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = "configure" ] || [ "$1" = "triggered" ]; then
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database -q /usr/share/applications || true
    fi
    if command -v gtk-update-icon-cache >/dev/null 2>&1; then
        gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
    fi
fi
EOF
    chmod 755 "$staging/DEBIAN/postinst"

    # DEBIAN postrm script
    cat > "$staging/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = "remove" ] || [ "$1" = "purge" ]; then
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database -q /usr/share/applications || true
    fi
    if command -v gtk-update-icon-cache >/dev/null 2>&1; then
        gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
    fi
fi
EOF
    chmod 755 "$staging/DEBIAN/postrm"

    # Fix permissions
    find "$staging" -type d -exec chmod 755 {} \;
    find "$staging/usr" -type f -exec chmod 644 {} \;
    chmod 755 "$staging/usr/bin/$APP_NAME"

    local pkg_name="$APP_NAME"
    if [ -f "$GTK_BIN_OUTPUT" ]; then
        pkg_name="${APP_NAME}-gtk"
    fi
    local output="$BUILD_DIR/${pkg_name}_${APP_VERSION}_${DEB_ARCH}.deb"
    dpkg-deb --build --root-owner-group "$staging" "$output"
    rm -rf "$staging"
    echo "    Created: $output"
}

build_rpm() {
    echo "==> Building .rpm package..."
    local rpmbuild_dir="$BUILD_DIR/rpmbuild"
    rm -rf "$rpmbuild_dir"
    mkdir -p "$rpmbuild_dir"/{SPECS,SOURCES,BUILD,RPMS,SRPMS}

    local tarball_name="${APP_NAME}-${APP_VERSION}"
    local tar_staging="$BUILD_DIR/tar-staging/$tarball_name"
    rm -rf "$BUILD_DIR/tar-staging"
    mkdir -p "$tar_staging"
    prepare_staging "$tar_staging"
    tar czf "$rpmbuild_dir/SOURCES/$tarball_name.tar.gz" -C "$BUILD_DIR/tar-staging" "$tarball_name"
    rm -rf "$BUILD_DIR/tar-staging"

    cat > "$rpmbuild_dir/SPECS/$APP_NAME.spec" <<EOF
%global debug_package %{nil}

Name:           $APP_NAME
Version:        $APP_VERSION
Release:        1%{?dist}
Summary:        $APP_DESCRIPTION
License:        $APP_LICENSE
Source0:        %{name}-%{version}.tar.gz

%description
WeatherWidget is a compact, customizable desktop weather widget.
Supports multiple cities, opacity options, and live weather updates.

%prep
%setup -q

%install
cp -a usr %{buildroot}/usr

%post
if command -v update-desktop-database &>/dev/null; then
    update-desktop-database -q %{_datadir}/applications || :
fi
if command -v gtk-update-icon-cache &>/dev/null; then
    gtk-update-icon-cache -q -t -f %{_datadir}/icons/hicolor || :
fi

%postun
if command -v update-desktop-database &>/dev/null; then
    update-desktop-database -q %{_datadir}/applications || :
fi
if command -v gtk-update-icon-cache &>/dev/null; then
    gtk-update-icon-cache -q -t -f %{_datadir}/icons/hicolor || :
fi

%files
%{_bindir}/$APP_NAME
%{_datadir}/applications/$APP_NAME.desktop
%{_datadir}/pixmaps/$APP_NAME.png
%{_datadir}/icons/hicolor/*/apps/$APP_NAME.png
EOF

    rpmbuild --define "_topdir $rpmbuild_dir" --define "_dbpath $rpmbuild_dir/rpmdb" -bb "$rpmbuild_dir/SPECS/$APP_NAME.spec"
    local rpm_generated
    rpm_generated=$(find "$rpmbuild_dir/RPMS" -name "*.rpm" | head -1)
    if [ -n "$rpm_generated" ]; then
        local pkg_name="$APP_NAME"
        if [ -f "$GTK_BIN_OUTPUT" ]; then
            pkg_name="${APP_NAME}-gtk"
        fi
        local output="$BUILD_DIR/${pkg_name}_${APP_VERSION}_${DEB_ARCH}.rpm"
        cp "$rpm_generated" "$output"
        rm -rf "$rpmbuild_dir"
        echo "    Created: $output"
    else
        echo "    ERROR: rpm not found in $rpmbuild_dir/RPMS"
        return 1
    fi
}

build_appimage() {
    echo "==> Building .AppImage package..."
    local appdir="$BUILD_DIR/$APP_NAME.AppDir"
    rm -rf "$appdir"
    mkdir -p "$appdir/usr/bin"

    # Prefer GTK binary; fall back to Fyne if GTK wasn't built.
    if [ -f "$GTK_BIN_OUTPUT" ]; then
        cp "$GTK_BIN_OUTPUT" "$appdir/usr/bin/$APP_NAME"
        echo "    Packaging: GTK3 native binary"
    else
        cp "$BIN_OUTPUT" "$appdir/usr/bin/$APP_NAME"
        echo "    Packaging: Fyne binary (GTK build unavailable)"
    fi

    # Desktop file
    cat > "$appdir/$APP_NAME.desktop" <<EOF
[Desktop Entry]
Name=$APP_DISPLAY_NAME
Comment=$APP_DESCRIPTION
Exec=$APP_NAME
Icon=$APP_NAME
Terminal=false
Type=Application
Categories=$APP_CATEGORIES
StartupWMClass=$APP_NAME
EOF

    # Icon at root of AppDir and standard paths inside AppDir
    local icon_src=""
    if [ -f "$PROJECT_ROOT/assets/icons/clear.png" ]; then
        icon_src="$PROJECT_ROOT/assets/icons/clear.png"
    elif [ -f "$PROJECT_ROOT/snap/gui/weatherwidget.png" ]; then
        icon_src="$PROJECT_ROOT/snap/gui/weatherwidget.png"
    fi

    if [ -n "$icon_src" ]; then
        cp "$icon_src" "$appdir/$APP_NAME.png"
        cp "$icon_src" "$appdir/.DirIcon"
        mkdir -p "$appdir/usr/share/pixmaps"
        mkdir -p "$appdir/usr/share/icons/hicolor/256x256/apps"
        cp "$icon_src" "$appdir/usr/share/pixmaps/$APP_NAME.png"
        cp "$icon_src" "$appdir/usr/share/icons/hicolor/256x256/apps/$APP_NAME.png"
    fi

    # AppRun script
    cat > "$appdir/AppRun" <<'APPRUN'
#!/usr/bin/env bash
SELF="$(readlink -f "$0")"
APPDIR="$(dirname "$SELF")"
exec "$APPDIR/usr/bin/weatherwidget" "$@"
APPRUN
    chmod +x "$appdir/AppRun"

    # Get appimagetool if not available in PATH
    local appimagetool=""
    if command -v appimagetool >/dev/null 2>&1; then
        appimagetool="appimagetool"
    elif [ -x "$BUILD_DIR/appimagetool" ]; then
        appimagetool="$BUILD_DIR/appimagetool"
    else
        echo "    Downloading appimagetool..."
        appimagetool="$BUILD_DIR/appimagetool"
        curl -sL "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-${ARCH}.AppImage" -o "$appimagetool"
        chmod +x "$appimagetool"
    fi

    local pkg_name="$APP_NAME"
    if [ -f "$GTK_BIN_OUTPUT" ]; then
        pkg_name="${APP_NAME}-gtk"
    fi
    local output="$BUILD_DIR/${pkg_name}_${APP_VERSION}_${DEB_ARCH}.AppImage"
    rm -f "$output"
    ARCH="$ARCH" "$appimagetool" --appimage-extract-and-run "$appdir" "$output" || \
    ARCH="$ARCH" "$appimagetool" --appimage-extract-and-run --no-appstream "$appdir" "$output"
    rm -rf "$appdir"
    echo "    Created: $output"
}

main() {
    local target="${1:-all}"

    build_bin

    case "$target" in
        bin)
            build_gtk_bin
            ;;
        deb)
            build_gtk_bin
            build_deb
            ;;
        rpm)
            build_gtk_bin
            build_rpm
            ;;
        appimage)
            build_gtk_bin
            build_appimage
            ;;
        all)
            build_gtk_bin
            build_deb
            build_rpm
            build_appimage
            ;;
        *)
            echo "Usage: $0 [bin|deb|rpm|appimage|all]"
            exit 1
            ;;
    esac

    echo ""
    echo "==> Build complete! Output files in: $BUILD_DIR/"
    ls -lh "$BUILD_DIR"/${APP_NAME}*_${APP_VERSION}_* 2>/dev/null || true
}

main "$@"
