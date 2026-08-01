#!/bin/bash
set -e

# WeatherWidget macOS Build Script
# Builds a universal .app bundle and optionally a .dmg

APP_NAME="WeatherWidget"
BINARY_NAME="weatherwidget"
BUNDLE_ID="com.weatherwidget"
CMD_PATH="./cmd/weatherwidget/"
APP_ICON="assets/icons/clear.png"
VERSION=$(cat release 2>/dev/null | tr -d '[:space:]')
if [ -z "$VERSION" ]; then
    echo "Error: could not read version from 'release' file"
    exit 1
fi

echo "==> Building WeatherWidget $VERSION for macOS..."

# Update version in all locale JSON files so the About tab matches.
LOCALE_DIR="internal/i18n/locales"
if [ -d "$LOCALE_DIR" ]; then
    for f in "$LOCALE_DIR"/*.json; do
        sed -i '' -E "s/\"settings\.about\.version\": \"(\*\*[^:*]+:\*\*) [^\"]*\"/\"settings.about.version\": \"\1 $VERSION\"/" "$f"
    done
fi

# Build both architectures
echo "    Compiling darwin/amd64..."
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -v -ldflags="-s -w -X main.version=$VERSION" -o "$BINARY_NAME-darwin-amd64" "$CMD_PATH"

echo "    Compiling darwin/arm64..."
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -v -ldflags="-s -w -X main.version=$VERSION" -o "$BINARY_NAME-darwin-arm64" "$CMD_PATH"

# Assemble .app bundle
APP_BUNDLE="$APP_NAME-$VERSION.app"
echo "==> Assembling $APP_BUNDLE..."

rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

# Universal binary
lipo -create -output "$APP_BUNDLE/Contents/MacOS/$BINARY_NAME" \
    "$BINARY_NAME-darwin-amd64" \
    "$BINARY_NAME-darwin-arm64"
chmod +x "$APP_BUNDLE/Contents/MacOS/$BINARY_NAME"

# Build .icns
ICONSET="/tmp/$APP_NAME.iconset"
rm -rf "$ICONSET"
mkdir -p "$ICONSET"
sips -z 16 16     "$APP_ICON" --out "$ICONSET/icon_16x16.png"      > /dev/null
sips -z 32 32     "$APP_ICON" --out "$ICONSET/icon_16x16@2x.png"   > /dev/null
sips -z 32 32     "$APP_ICON" --out "$ICONSET/icon_32x32.png"      > /dev/null
sips -z 64 64     "$APP_ICON" --out "$ICONSET/icon_32x32@2x.png"   > /dev/null
sips -z 128 128   "$APP_ICON" --out "$ICONSET/icon_128x128.png"    > /dev/null
sips -z 256 256   "$APP_ICON" --out "$ICONSET/icon_128x128@2x.png" > /dev/null
sips -z 256 256   "$APP_ICON" --out "$ICONSET/icon_256x256.png"    > /dev/null
sips -z 512 512   "$APP_ICON" --out "$ICONSET/icon_256x256@2x.png" > /dev/null
cp "$APP_ICON" "$ICONSET/icon_512x512.png"
sips -z 512 512   "$APP_ICON" --out "$ICONSET/icon_512x512@2x.png" > /dev/null
iconutil -c icns "$ICONSET" -o "$APP_BUNDLE/Contents/Resources/$APP_NAME.icns"
rm -rf "$ICONSET"

# Write Info.plist
cat > "$APP_BUNDLE/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleDisplayName</key>
    <string>$APP_NAME</string>
    <key>CFBundleIdentifier</key>
    <string>$BUNDLE_ID</string>
    <key>CFBundleVersion</key>
    <string>$VERSION</string>
    <key>CFBundleShortVersionString</key>
    <string>$VERSION</string>
    <key>CFBundleExecutable</key>
    <string>$BINARY_NAME</string>
    <key>CFBundleIconFile</key>
    <string>$APP_NAME</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSSupportsAutomaticGraphicsSwitching</key>
    <true/>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>LSUIElement</key>
    <false/>
</dict>
</plist>
EOF

echo "==> Created: $APP_BUNDLE"

# Build .dmg
DMG_NAME="$APP_NAME-$VERSION.dmg"
echo "==> Creating $DMG_NAME..."
rm -f "$DMG_NAME"
hdiutil create -volname "$APP_NAME $VERSION" \
    -srcfolder "$APP_BUNDLE" \
    -ov -format UDZO \
    -o "$DMG_NAME"

echo ""
echo "==> Build complete!"
echo "    App:  $APP_BUNDLE"
echo "    DMG:  $DMG_NAME"
echo ""
echo "    To install: open $DMG_NAME and drag to /Applications"

# Cleanup intermediate binaries
rm -f "$BINARY_NAME-darwin-amd64" "$BINARY_NAME-darwin-arm64"
