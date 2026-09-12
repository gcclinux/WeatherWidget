#!/bin/bash
set -euo pipefail

# =============================================================================
#  WeatherWidget macOS Build Script
#
#  Builds a universal .app bundle (arm64 + amd64) using the native AppKit UI
#  layer (internal/ui-darwin).  Fyne is only used for the settings window;
#  the widget window itself is pure Cocoa/NSView.
#
#  Usage:
#    ./scripts/build-darwin.sh                        # unsigned .app + .dmg
#    ./scripts/build-darwin.sh --sign-app "Developer ID Application: You (TEAM)"
#    ./scripts/build-darwin.sh --sign-app "..." --sign-pkg "Developer ID Installer: You (TEAM)"
#    ./scripts/build-darwin.sh --sign-app "..." --notarize \
#        --apple-id you@example.com --team-id ABCDEF1234 --app-password xxxx-xxxx-xxxx-xxxx
#
#  Optional env vars:
#    VERSION    — override the version read from ./release  (e.g. VERSION=1.2.3)
#    BUILD_DIR  — output directory (default: scripts/build)
# =============================================================================

# ── Configuration ─────────────────────────────────────────────────────────────

APP_NAME="WeatherWidget"
BINARY_NAME="weatherwidget"
BUNDLE_ID="com.weatherwidget"
CMD_PATH="./cmd/weatherwidget/"
APP_ICON="assets/icons/day/clear_day.png"

BUILD_DIR="${BUILD_DIR:-scripts/build}"

# ── Version ───────────────────────────────────────────────────────────────────

if [ -z "${VERSION:-}" ]; then
    VERSION=$(cat release 2>/dev/null | tr -d '[:space:]')
fi
if [ -z "$VERSION" ]; then
    echo "Error: could not determine version. Set VERSION= or create a 'release' file."
    exit 1
fi

echo "==> Building WeatherWidget $VERSION (native AppKit / ui-darwin)"

# ── Argument parsing ──────────────────────────────────────────────────────────

SIGN_APP=""
SIGN_PKG=""
NOTARIZE=0
APPLE_ID=""
TEAM_ID=""
APP_PWD=""
BUILD_PKG=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --sign-app)   SIGN_APP="$2";  shift 2 ;;
        --sign-pkg)   SIGN_PKG="$2";  shift 2 ;;
        --notarize)   NOTARIZE=1;     shift   ;;
        --apple-id)   APPLE_ID="$2";  shift 2 ;;
        --team-id)    TEAM_ID="$2";   shift 2 ;;
        --app-password) APP_PWD="$2"; shift 2 ;;
        --pkg)        BUILD_PKG=1;    shift   ;;
        *) echo "Unknown argument: $1"; exit 1 ;;
    esac
done

mkdir -p "$BUILD_DIR"

# ── Update version string in locale JSON files ────────────────────────────────

LOCALE_DIR="internal/i18n/locales"
if [ -d "$LOCALE_DIR" ]; then
    for f in "$LOCALE_DIR"/*.json; do
        sed -i '' -E \
            "s/\"settings\\.about\\.version\": \"(\*\*[^:*]+:\*\*) [^\"]*\"/\"settings.about.version\": \"\1 $VERSION\"/" \
            "$f"
    done
fi

# ── Compile ───────────────────────────────────────────────────────────────────
# The native build does not use Fyne for the widget window but still imports
# it for the settings dialog, so CGO_ENABLED=1 is required on both arches.

LDFLAGS="-s -w -X main.version=$VERSION"

echo "    Compiling darwin/amd64..."
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
    go build -v -ldflags="$LDFLAGS" \
    -o "$BUILD_DIR/$BINARY_NAME-darwin-amd64" "$CMD_PATH"

echo "    Compiling darwin/arm64..."
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
    go build -v -ldflags="$LDFLAGS" \
    -o "$BUILD_DIR/$BINARY_NAME-darwin-arm64" "$CMD_PATH"

# ── Assemble .app bundle ──────────────────────────────────────────────────────

APP_BUNDLE="$BUILD_DIR/$APP_NAME-$VERSION.app"
echo "==> Assembling $APP_BUNDLE..."

rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

# Universal binary (lipo merges Intel + Apple Silicon slices).
lipo -create \
    -output "$APP_BUNDLE/Contents/MacOS/$BINARY_NAME" \
    "$BUILD_DIR/$BINARY_NAME-darwin-amd64" \
    "$BUILD_DIR/$BINARY_NAME-darwin-arm64"
chmod +x "$APP_BUNDLE/Contents/MacOS/$BINARY_NAME"

# ── .icns ─────────────────────────────────────────────────────────────────────

ICONSET="/tmp/$APP_NAME.iconset"
rm -rf "$ICONSET"
mkdir -p "$ICONSET"
sips -z 16   16   "$APP_ICON" --out "$ICONSET/icon_16x16.png"       > /dev/null
sips -z 32   32   "$APP_ICON" --out "$ICONSET/icon_16x16@2x.png"    > /dev/null
sips -z 32   32   "$APP_ICON" --out "$ICONSET/icon_32x32.png"       > /dev/null
sips -z 64   64   "$APP_ICON" --out "$ICONSET/icon_32x32@2x.png"    > /dev/null
sips -z 128  128  "$APP_ICON" --out "$ICONSET/icon_128x128.png"     > /dev/null
sips -z 256  256  "$APP_ICON" --out "$ICONSET/icon_128x128@2x.png"  > /dev/null
sips -z 256  256  "$APP_ICON" --out "$ICONSET/icon_256x256.png"     > /dev/null
sips -z 512  512  "$APP_ICON" --out "$ICONSET/icon_256x256@2x.png"  > /dev/null
cp "$APP_ICON"    "$ICONSET/icon_512x512.png"
sips -z 512  512  "$APP_ICON" --out "$ICONSET/icon_512x512@2x.png"  > /dev/null
iconutil -c icns "$ICONSET" -o "$APP_BUNDLE/Contents/Resources/$APP_NAME.icns"
rm -rf "$ICONSET"

# ── Info.plist ────────────────────────────────────────────────────────────────
#
# Key notes for the native AppKit build:
#   LSUIElement = true  — hides the Dock icon; the app lives entirely in the
#                         menu bar (NSStatusItem) and as a desktop overlay.
#                         Remove or set to false if you want a Dock icon.
#   NSAppTransportSecurity — allows HTTP for weather API calls during development.
#                            Remove or tighten for App Store submission.

cat > "$APP_BUNDLE/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundleExecutable</key>
    <string>${BINARY_NAME}</string>
    <key>CFBundleIconFile</key>
    <string>${APP_NAME}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>

    <!-- macOS version requirement -->
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>

    <!-- Menu-bar-only app, no Dock icon. Matches the proven minimal test. -->
    <key>LSUIElement</key>
    <true/>

    <!-- Retina / dynamic display support -->
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSSupportsAutomaticGraphicsSwitching</key>
    <true/>

    <!-- Network access for weather API (required for App Store sandbox) -->
    <key>NSAppTransportSecurity</key>
    <dict>
        <key>NSAllowsArbitraryLoads</key>
        <true/>
    </dict>

    <!-- Human-readable usage descriptions required for App Store review -->
    <key>NSLocationUsageDescription</key>
    <string>WeatherWidget uses your location to show local weather.</string>
</dict>
</plist>
PLIST

echo "==> Created: $APP_BUNDLE"

# ── Entitlements (for signing / notarization / App Store) ────────────────────
#
# The native AppKit build does NOT use OpenGL or GLFW, so we need only:
#   - network.client  — outbound HTTPS for weather API
#   - app-sandbox     — required for Mac App Store
#
# If you distribute outside the App Store (direct download / notarized DMG)
# you do NOT need app-sandbox, but having it doesn't hurt.

ENTITLEMENTS_FILE="$BUILD_DIR/WeatherWidget.entitlements"
cat > "$ENTITLEMENTS_FILE" <<ENTS
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <!-- Outbound network for weather API calls -->
    <key>com.apple.security.network.client</key>
    <true/>

    <!-- Remove the sandbox key for direct-download / notarized-only builds -->
    <key>com.apple.security.app-sandbox</key>
    <true/>
</dict>
</plist>
ENTS

# ── Code signing ──────────────────────────────────────────────────────────────

if [ -n "$SIGN_APP" ]; then
    echo "==> Signing $APP_BUNDLE with: $SIGN_APP"
    codesign --force --deep --options runtime \
        --entitlements "$ENTITLEMENTS_FILE" \
        --sign "$SIGN_APP" \
        "$APP_BUNDLE"
    echo "    Verifying signature..."
    codesign --verify --deep --strict "$APP_BUNDLE" && echo "    ✓ Signature valid"
else
    echo "    (skipping code signing — pass --sign-app to sign)"
fi

# ── .dmg ─────────────────────────────────────────────────────────────────────

DMG_NAME="$BUILD_DIR/$APP_NAME-$VERSION.dmg"
echo "==> Creating $DMG_NAME..."
rm -f "$DMG_NAME"

# Stage: .app + Applications symlink for drag-install UX.
DMG_STAGE="$BUILD_DIR/dmg-stage"
rm -rf "$DMG_STAGE"
mkdir -p "$DMG_STAGE"
cp -R "$APP_BUNDLE" "$DMG_STAGE/"
ln -s /Applications "$DMG_STAGE/Applications"

DMG_TMP="$BUILD_DIR/$APP_NAME-$VERSION-tmp.dmg"
rm -f "$DMG_TMP"

# Add 50 MB slack so hdiutil can't under-allocate the writable image.
DMG_SIZE=$(du -sm "$DMG_STAGE" | awk '{print $1 + 50}')

hdiutil create \
    -volname "$APP_NAME $VERSION" \
    -srcfolder "$DMG_STAGE" \
    -fs HFS+ -format UDRW -size "${DMG_SIZE}m" \
    -ov "$DMG_TMP" > /dev/null

hdiutil convert "$DMG_TMP" -format UDZO -o "$DMG_NAME" > /dev/null

rm -f "$DMG_TMP"
rm -rf "$DMG_STAGE"

# Sign the DMG itself when a signing identity is available.
if [ -n "$SIGN_APP" ]; then
    echo "    Signing DMG..."
    codesign --sign "$SIGN_APP" "$DMG_NAME"
fi

# ── Notarization ──────────────────────────────────────────────────────────────

if [ "$NOTARIZE" -eq 1 ]; then
    if [ -z "$APPLE_ID" ] || [ -z "$TEAM_ID" ] || [ -z "$APP_PWD" ]; then
        echo "Error: --notarize requires --apple-id, --team-id, and --app-password"
        exit 1
    fi
    echo "==> Submitting $DMG_NAME for notarization..."
    xcrun notarytool submit "$DMG_NAME" \
        --apple-id "$APPLE_ID" \
        --team-id  "$TEAM_ID" \
        --password "$APP_PWD" \
        --wait
    echo "    Stapling notarization ticket..."
    xcrun stapler staple "$DMG_NAME"
    xcrun stapler staple "$APP_BUNDLE"
    echo "    ✓ Notarized and stapled"
fi

# ── Optional .pkg installer ───────────────────────────────────────────────────

if [ "$BUILD_PKG" -eq 1 ]; then
    PKG_ARGS="--version $VERSION"
    if [ -n "$SIGN_APP" ] && [ -n "$SIGN_PKG" ]; then
        PKG_ARGS="$PKG_ARGS --sign-app \"$SIGN_APP\" --sign-pkg \"$SIGN_PKG\""
    else
        PKG_ARGS="$PKG_ARGS --skip-sign"
    fi
    if [ "$NOTARIZE" -eq 1 ]; then
        PKG_ARGS="$PKG_ARGS --notarize --apple-id $APPLE_ID --team-id $TEAM_ID --app-password $APP_PWD"
    fi
    eval bash installer/build-pkg.sh $PKG_ARGS
fi

# ── Cleanup intermediate binaries ─────────────────────────────────────────────

rm -f \
    "$BUILD_DIR/$BINARY_NAME-darwin-amd64" \
    "$BUILD_DIR/$BINARY_NAME-darwin-arm64"

# ── Summary ───────────────────────────────────────────────────────────────────

echo ""
echo "==> Build complete!"
echo "    App:  $APP_BUNDLE"
echo "    DMG:  $DMG_NAME"
[ "$BUILD_PKG" -eq 1 ] && echo "    PKG:  $BUILD_DIR/$APP_NAME-$VERSION.pkg"
echo ""
echo "    To install: open $DMG_NAME and drag to /Applications"
