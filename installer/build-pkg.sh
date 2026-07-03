#!/usr/bin/env bash
# =============================================================================
# Weather Widget - macOS PKG Installer Builder
# =============================================================================
# Produces a signed (or unsigned) macOS .pkg installer that places
# WeatherWidget.app into /Applications via the standard macOS installer UI.
#
# Usage:
#   # Build unsigned installer (for testing):
#   ./installer/build-pkg.sh --version 1.2.0 --skip-sign
#
#   # Build signed installer (for distribution):
#   ./installer/build-pkg.sh --version 1.2.0 \
#       --sign-app "Developer ID Application: Your Name (TEAMID)" \
#       --sign-pkg "Developer ID Installer: Your Name (TEAMID)"
#
#   # Build signed + notarized installer (required for Gatekeeper on macOS 10.15+):
#   ./installer/build-pkg.sh --version 1.2.0 \
#       --sign-app "Developer ID Application: Your Name (TEAMID)" \
#       --sign-pkg "Developer ID Installer: Your Name (TEAMID)" \
#       --notarize \
#       --apple-id "you@example.com" \
#       --team-id "YOURTEAMID" \
#       --app-password "xxxx-xxxx-xxxx-xxxx"
#
# Prerequisites:
#   - macOS with Xcode Command Line Tools (pkgbuild, productbuild, codesign,
#     xcrun notarytool — all built in, no extra installs needed)
#   - For signing: Apple Developer account with the correct certificates
#     installed in Keychain
#   - For notarization: App-specific password from appleid.apple.com
#
# Output:
#   build/WeatherWidget-<version>.pkg
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
VERSION=""
SIGN_APP_IDENTITY=""
SIGN_PKG_IDENTITY=""
SKIP_SIGN=false
NOTARIZE=false
APPLE_ID=""
TEAM_ID=""
APP_PASSWORD=""
BUNDLE_ID="com.weatherwidget"
APP_NAME="WeatherWidget"
BINARY_NAME="weatherwidget"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)      VERSION="$2";           shift 2 ;;
        --sign-app)     SIGN_APP_IDENTITY="$2"; shift 2 ;;
        --sign-pkg)     SIGN_PKG_IDENTITY="$2"; shift 2 ;;
        --skip-sign)    SKIP_SIGN=true;         shift   ;;
        --notarize)     NOTARIZE=true;          shift   ;;
        --apple-id)     APPLE_ID="$2";          shift 2 ;;
        --team-id)      TEAM_ID="$2";           shift 2 ;;
        --app-password) APP_PASSWORD="$2";      shift 2 ;;
        --bundle-id)    BUNDLE_ID="$2";         shift 2 ;;
        --help|-h)
            sed -n '3,40p' "$0" | sed 's/^# *//'
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# ---------------------------------------------------------------------------
# Validation
# ---------------------------------------------------------------------------
if [[ -z "$VERSION" ]]; then
    echo "ERROR: --version is required (e.g. --version 1.2.0)"
    exit 1
fi

if [[ "$SKIP_SIGN" == false && -z "$SIGN_APP_IDENTITY" && -z "$SIGN_PKG_IDENTITY" ]]; then
    echo "WARNING: No signing identity provided. Building unsigned package."
    echo "         Use --skip-sign to suppress this warning, or provide"
    echo "         --sign-app and --sign-pkg for distribution builds."
    SKIP_SIGN=true
fi

if [[ "$NOTARIZE" == true ]]; then
    if [[ -z "$APPLE_ID" || -z "$TEAM_ID" || -z "$APP_PASSWORD" ]]; then
        echo "ERROR: --notarize requires --apple-id, --team-id, and --app-password"
        exit 1
    fi
    if [[ "$SKIP_SIGN" == true || -z "$SIGN_APP_IDENTITY" || -z "$SIGN_PKG_IDENTITY" ]]; then
        echo "ERROR: --notarize requires --sign-app and --sign-pkg (Apple requires signed packages for notarization)"
        exit 1
    fi
fi

APP_BUNDLE="${APP_NAME}-${VERSION}.app"
APP_BUNDLE_PATH="$PROJECT_ROOT/${APP_BUNDLE}"
PKG_OUTPUT="$BUILD_DIR/${APP_NAME}-${VERSION}.pkg"
COMPONENT_PKG="$BUILD_DIR/${APP_NAME}-component.pkg"
PKG_SCRIPTS_DIR="$BUILD_DIR/scripts"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
step() { echo; echo "==> $*"; }
ok()   { echo "    OK: $*"; }
warn() { echo "    WARNING: $*"; }

# ---------------------------------------------------------------------------
# Step 1: Verify the .app bundle exists
# ---------------------------------------------------------------------------
step "[1/6] Verifying .app bundle..."

if [[ ! -d "$APP_BUNDLE_PATH" ]]; then
    echo "ERROR: ${APP_BUNDLE} not found at $PROJECT_ROOT"
    echo "       Run 'make build-darwin-app VERSION=${VERSION}' first."
    exit 1
fi

if [[ ! -x "$APP_BUNDLE_PATH/Contents/MacOS/$BINARY_NAME" ]]; then
    echo "ERROR: Executable not found inside bundle: Contents/MacOS/$BINARY_NAME"
    exit 1
fi
ok "Found $APP_BUNDLE"

# ---------------------------------------------------------------------------
# Step 2: Code-sign the .app bundle
# ---------------------------------------------------------------------------
step "[2/6] Code-signing .app bundle..."

if [[ "$SKIP_SIGN" == false && -n "$SIGN_APP_IDENTITY" ]]; then
    codesign \
        --force \
        --deep \
        --options runtime \
        --sign "$SIGN_APP_IDENTITY" \
        --timestamp \
        --entitlements "$PROJECT_ROOT/installer/entitlements.plist" \
        "$APP_BUNDLE_PATH"
    ok "Signed with: $SIGN_APP_IDENTITY"

    # Verify the signature
    codesign --verify --deep --strict "$APP_BUNDLE_PATH"
    ok "Signature verified"
else
    warn "Skipping code signing (unsigned builds will trigger Gatekeeper on macOS 10.15+)"
fi

# ---------------------------------------------------------------------------
# Step 3: Prepare staging area and install scripts
# ---------------------------------------------------------------------------
step "[3/6] Preparing staging area..."

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/root/Applications"
mkdir -p "$PKG_SCRIPTS_DIR"

# Copy the .app into the staging root (this becomes the install root)
cp -R "$APP_BUNDLE_PATH" "$BUILD_DIR/root/Applications/"
ok "Staged $APP_BUNDLE -> /Applications/$APP_BUNDLE"

# Post-install script: launch the app after installation
cat > "$PKG_SCRIPTS_DIR/postinstall" << 'EOF'
#!/bin/bash
# Open the app after installation so users see it immediately.
# Run as the actual logged-in user (not root).
LOGGED_IN_USER=$(stat -f "%Su" /dev/console 2>/dev/null || echo "")
APP_PATH="/Applications/WeatherWidget-VERSION_PLACEHOLDER.app"

if [[ -n "$LOGGED_IN_USER" && "$LOGGED_IN_USER" != "root" ]]; then
    sudo -u "$LOGGED_IN_USER" open "$APP_PATH" &
fi

exit 0
EOF

# Replace the version placeholder in the postinstall script
sed -i '' "s/VERSION_PLACEHOLDER/$VERSION/g" "$PKG_SCRIPTS_DIR/postinstall"
chmod +x "$PKG_SCRIPTS_DIR/postinstall"
ok "Created postinstall script"

# ---------------------------------------------------------------------------
# Step 4: Build component .pkg
# ---------------------------------------------------------------------------
step "[4/6] Building component package..."

PKGBUILD_ARGS=(
    --root "$BUILD_DIR/root"
    --identifier "$BUNDLE_ID"
    --version "$VERSION"
    --install-location "/"
    --scripts "$PKG_SCRIPTS_DIR"
)

if [[ "$SKIP_SIGN" == false && -n "$SIGN_PKG_IDENTITY" ]]; then
    PKGBUILD_ARGS+=(--sign "$SIGN_PKG_IDENTITY")
fi

pkgbuild "${PKGBUILD_ARGS[@]}" "$COMPONENT_PKG"
ok "Component package created"

# ---------------------------------------------------------------------------
# Step 5: Build distribution .pkg with productbuild
# ---------------------------------------------------------------------------
step "[5/6] Building distribution installer..."

# Generate a Distribution XML that shows a welcome screen and license
DIST_XML="$BUILD_DIR/Distribution.xml"

cat > "$DIST_XML" << DISTXML
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="2">
    <title>${APP_NAME} ${VERSION}</title>
    <organization>${BUNDLE_ID}</organization>
    <domains enable_localSystem="true" enable_currentUserHome="false"/>
    <options customize="never" require-scripts="false" rootVolumeOnly="true"/>

    <welcome    file="Welcome.rtf"    mime-type="text/rtf"/>
    <license    file="License.rtf"    mime-type="text/rtf"/>
    <conclusion file="Conclusion.rtf" mime-type="text/rtf"/>

    <pkg-ref id="${BUNDLE_ID}"/>

    <choices-outline>
        <line choice="${BUNDLE_ID}"/>
    </choices-outline>

    <choice id="${BUNDLE_ID}" visible="false">
        <pkg-ref id="${BUNDLE_ID}"/>
    </choice>

    <pkg-ref id="${BUNDLE_ID}" version="${VERSION}" onConclusion="none">${APP_NAME}-component.pkg</pkg-ref>
</installer-gui-script>
DISTXML

ok "Distribution XML created"

# Generate installer text resources (RTF format required by productbuild)
# Welcome screen
cat > "$BUILD_DIR/Welcome.rtf" << 'RTF'
{\rtf1\ansi\ansicpg1252\cocoartf2639
{\fonttbl\f0\fswiss\fcharset0 Helvetica;}
{\colortbl;\red255\green255\blue255;}
\paperw11900\paperh16840\margl1440\margr1440\vieww9000\viewh8400\viewkind0
\pard\tx566\tx1133\tx1700\tx2267\tx2834\tx3401\tx3968\tx4535\tx5102\tx5669\tx6236\tx6803\pardirnatural\partightenfactor0
\f0\b\fs28 \cf0 Welcome to the WeatherWidget Installer\
\b0\fs24 \
This installer will place WeatherWidget.app in your /Applications folder.\
\
WeatherWidget shows live weather information in your menu bar. After installation the app will open automatically.\
\
Click Continue to proceed.}
RTF

# License screen (points to the project LICENSE file content)
LICENSE_PATH="$PROJECT_ROOT/LICENSE"
if [[ -f "$LICENSE_PATH" ]]; then
    LICENSE_TEXT=$(cat "$LICENSE_PATH")
else
    LICENSE_TEXT="MIT License — see https://github.com/your-repo/WeatherWidget"
fi

cat > "$BUILD_DIR/License.rtf" << RTF
{\rtf1\ansi\ansicpg1252\cocoartf2639
{\fonttbl\f0\fswiss\fcharset0 Helvetica;\f1\fmodern\fcharset0 Courier;}
{\colortbl;\red255\green255\blue255;}
\paperw11900\paperh16840\margl1440\margr1440\vieww9000\viewh8400\viewkind0
\pard\tx566\pardirnatural\partightenfactor0
\f1\fs20 \cf0 ${LICENSE_TEXT}}
RTF

# Conclusion screen
cat > "$BUILD_DIR/Conclusion.rtf" << 'RTF'
{\rtf1\ansi\ansicpg1252\cocoartf2639
{\fonttbl\f0\fswiss\fcharset0 Helvetica;}
{\colortbl;\red255\green255\blue255;}
\paperw11900\paperh16840\margl1440\margr1440\vieww9000\viewh8400\viewkind0
\pard\tx566\pardirnatural\partightenfactor0
\f0\b\fs28 \cf0 Installation Complete\
\b0\fs24 \
WeatherWidget has been installed in /Applications.\
\
The app is launching now. You will see the weather icon in your menu bar.}
RTF

# Create entitlements.plist if it doesn't exist yet
ENTITLEMENTS_PATH="$PROJECT_ROOT/installer/entitlements.plist"
if [[ ! -f "$ENTITLEMENTS_PATH" ]]; then
    cat > "$ENTITLEMENTS_PATH" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <!-- Allow outbound network connections for weather API calls -->
    <key>com.apple.security.network.client</key>
    <true/>
    <!-- Allow reading the user's location (if used) -->
    <key>com.apple.security.personal-information.location</key>
    <true/>
</dict>
</plist>
PLIST
    ok "Created installer/entitlements.plist (review and adjust before distribution)"
fi

# Build final .pkg
PRODUCTBUILD_ARGS=(
    --distribution "$DIST_XML"
    --resources "$BUILD_DIR"
    --package-path "$BUILD_DIR"
)

if [[ "$SKIP_SIGN" == false && -n "$SIGN_PKG_IDENTITY" ]]; then
    PRODUCTBUILD_ARGS+=(--sign "$SIGN_PKG_IDENTITY")
fi

productbuild "${PRODUCTBUILD_ARGS[@]}" "$PKG_OUTPUT"
ok "Installer created: $PKG_OUTPUT"

# Clean up intermediate files
rm -f "$COMPONENT_PKG" "$DIST_XML" "$BUILD_DIR/Welcome.rtf" "$BUILD_DIR/License.rtf" "$BUILD_DIR/Conclusion.rtf"
rm -rf "$PKG_SCRIPTS_DIR" "$BUILD_DIR/root"

# ---------------------------------------------------------------------------
# Step 6: Notarize (optional — required for Gatekeeper on macOS 10.15+)
# ---------------------------------------------------------------------------
step "[6/6] Notarization..."

if [[ "$NOTARIZE" == true ]]; then
    echo "    Submitting to Apple notary service (this can take a few minutes)..."

    xcrun notarytool submit "$PKG_OUTPUT" \
        --apple-id "$APPLE_ID" \
        --team-id "$TEAM_ID" \
        --password "$APP_PASSWORD" \
        --wait

    echo "    Stapling notarization ticket..."
    xcrun stapler staple "$PKG_OUTPUT"
    ok "Notarized and stapled"
else
    warn "Skipping notarization. Users on macOS 10.15+ may see a Gatekeeper warning."
    warn "Use --notarize for public distribution builds."
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
echo "============================================"
echo "  BUILD SUCCESSFUL"
echo "  Output: $PKG_OUTPUT"
if [[ "$SKIP_SIGN" == true ]]; then
    echo
    echo "  NOTE: Package is unsigned."
    echo "  For public distribution use --sign-app and --sign-pkg"
    echo "  with your Developer ID certificates, and add --notarize."
fi
echo "============================================"
echo
