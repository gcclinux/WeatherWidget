#!/bin/bash
set -e

# Bump Version Script for macOS/Linux
# Updates the version across all project files.
#
# Usage:
#   ./scripts/bump_version.sh 0.1.0
#
# This updates:
#   - release (source of truth)
#   - internal/i18n/locales/*.json (About tab version display)
#   - winres/winres.json (Windows binary resource version)
#   - installer/AppxManifest.xml (MSIX package version, 4-part)
#   - docs/site/index.html (hero badge version)
#   - README.md (example commands)
#   - flatpak/uk.co.easysmartapps.WeatherWidget.metainfo.xml (new <release> entry)

NEW_VERSION="$1"

if [ -z "$NEW_VERSION" ]; then
    echo "Error: No version specified."
    echo "Usage: ./scripts/bump_version.sh 0.1.0"
    exit 1
fi

# Validate version format (digits separated by dots: X.Y.Z or X.Y.Z.W)
if ! echo "$NEW_VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?$'; then
    echo "Error: Invalid version format '$NEW_VERSION'. Expected format: X.Y.Z or X.Y.Z.W"
    echo "Example: ./scripts/bump_version.sh 0.1.0"
    exit 1
fi

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UPDATED=0
FAILED=0

echo ""
echo "Bumping version to $NEW_VERSION"
echo ""

# --- 1. Update release file ---
RELEASE_FILE="$PROJECT_ROOT/release"
if printf '%s' "$NEW_VERSION" > "$RELEASE_FILE"; then
    echo "  [OK] release"
    UPDATED=$((UPDATED + 1))
else
    echo "  [FAIL] release"
    FAILED=$((FAILED + 1))
fi

# --- 2. Update locale JSON files (settings.about.version) ---
LOCALE_DIR="$PROJECT_ROOT/internal/i18n/locales"
if [ -d "$LOCALE_DIR" ]; then
    for f in "$LOCALE_DIR"/*.json; do
        [ -f "$f" ] || continue
        BASENAME=$(basename "$f")
        # Match: "settings.about.version": "**Label:** oldversion"
        # Preserves the locale-specific label (e.g. **Version:**, **Versão:**, etc.)
        if sed -i '' -E "s/(\"settings\.about\.version\":[[:space:]]*\"(\*\*[^*]+\*\*)[[:space:]]*)[^\"]*\"/\1$NEW_VERSION\"/" "$f" 2>/dev/null; then
            echo "  [OK] internal/i18n/locales/$BASENAME"
            UPDATED=$((UPDATED + 1))
        else
            # Try GNU sed syntax (Linux)
            if sed -i -E "s/(\"settings\.about\.version\":[[:space:]]*\"(\*\*[^*]+\*\*)[[:space:]]*)[^\"]*\"/\1$NEW_VERSION\"/" "$f" 2>/dev/null; then
                echo "  [OK] internal/i18n/locales/$BASENAME"
                UPDATED=$((UPDATED + 1))
            else
                echo "  [FAIL] internal/i18n/locales/$BASENAME"
                FAILED=$((FAILED + 1))
            fi
        fi
    done
fi

# --- 3. Update winres/winres.json (Windows binary resource version) ---
WINRES_FILE="$PROJECT_ROOT/winres/winres.json"
if [ -f "$WINRES_FILE" ]; then
    if sed -i '' -E \
        -e "s/(\"version\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
        -e "s/(\"file_version\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
        -e "s/(\"product_version\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
        -e "s/(\"FileVersion\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
        -e "s/(\"ProductVersion\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
        "$WINRES_FILE" 2>/dev/null; then
        echo "  [OK] winres/winres.json"
        UPDATED=$((UPDATED + 1))
    else
        if sed -i -E \
            -e "s/(\"version\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
            -e "s/(\"file_version\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
            -e "s/(\"product_version\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
            -e "s/(\"FileVersion\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
            -e "s/(\"ProductVersion\":[[:space:]]*\")[^\"]*(\")/\1${NEW_VERSION}\2/" \
            "$WINRES_FILE" 2>/dev/null; then
            echo "  [OK] winres/winres.json"
            UPDATED=$((UPDATED + 1))
        else
            echo "  [FAIL] winres/winres.json"
            FAILED=$((FAILED + 1))
        fi
    fi
fi

# --- 4. Update installer/AppxManifest.xml (needs 4-part version) ---
MANIFEST_FILE="$PROJECT_ROOT/installer/AppxManifest.xml"
if [ -f "$MANIFEST_FILE" ]; then
    # AppxManifest requires exactly 4-part version (Major.Minor.Patch.Build)
    PART_COUNT=$(echo "$NEW_VERSION" | tr '.' '\n' | wc -l)
    if [ "$PART_COUNT" -eq 3 ]; then
        MSIX_VERSION="${NEW_VERSION}.0"
    else
        MSIX_VERSION="$NEW_VERSION"
    fi

    # Only update the Version attribute inside the <Identity> element.
    if sed -i '' -E "s/(<Identity[^>]*Version=\")[^\"]*(\")/\1${MSIX_VERSION}\2/" "$MANIFEST_FILE" 2>/dev/null; then
        echo "  [OK] installer/AppxManifest.xml (version: $MSIX_VERSION)"
        UPDATED=$((UPDATED + 1))
    else
        if sed -i -E "s/(<Identity[^>]*Version=\")[^\"]*(\")/\1${MSIX_VERSION}\2/" "$MANIFEST_FILE" 2>/dev/null; then
            echo "  [OK] installer/AppxManifest.xml (version: $MSIX_VERSION)"
            UPDATED=$((UPDATED + 1))
        else
            echo "  [FAIL] installer/AppxManifest.xml"
            FAILED=$((FAILED + 1))
        fi
    fi
fi

# --- 5. Update docs/site/index.html (hero badge) ---
INDEX_HTML="$PROJECT_ROOT/docs/site/index.html"
if [ -f "$INDEX_HTML" ]; then
    # Match: <span id="app-version">v0.0.6.1</span>
    if sed -i '' -E "s/(<span id=\"app-version\">v)[^<]*(<\/span>)/\1${NEW_VERSION}\2/" "$INDEX_HTML" 2>/dev/null; then
        echo "  [OK] docs/site/index.html"
        UPDATED=$((UPDATED + 1))
    else
        if sed -i -E "s/(<span id=\"app-version\">v)[^<]*(<\/span>)/\1${NEW_VERSION}\2/" "$INDEX_HTML" 2>/dev/null; then
            echo "  [OK] docs/site/index.html"
            UPDATED=$((UPDATED + 1))
        else
            echo "  [FAIL] docs/site/index.html"
            FAILED=$((FAILED + 1))
        fi
    fi
fi

# --- 6. Update README.md (example MSI build commands) ---
README_FILE="$PROJECT_ROOT/README.md"
if [ -f "$README_FILE" ]; then
    # Match: -Version "X.Y.Z.W" or -Version "X.Y.Z"
    README_PATTERN='s/(-Version ")[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?(")/\1'"${NEW_VERSION}"'\3/'
    if sed -i '' -E "$README_PATTERN" "$README_FILE" 2>/dev/null; then
        echo "  [OK] README.md"
        UPDATED=$((UPDATED + 1))
    else
        if sed -i -E "$README_PATTERN" "$README_FILE" 2>/dev/null; then
            echo "  [OK] README.md"
            UPDATED=$((UPDATED + 1))
        else
            echo "  [FAIL] README.md"
            FAILED=$((FAILED + 1))
        fi
    fi
fi

# --- 7. Update flatpak metainfo.xml (prepend a new <release> entry) ---
# AppStream expects newest-first releases. Rather than editing an existing
# version (the block is a changelog history), we prepend a new <release>
# element with today's date. Idempotent: skips if the newest entry already
# matches NEW_VERSION.
METAINFO_FILE="$PROJECT_ROOT/flatpak/uk.co.easysmartapps.WeatherWidget.metainfo.xml"
if [ -f "$METAINFO_FILE" ]; then
    RELEASE_DATE=$(date +%F)
    # Detect the version of the first (newest) <release> entry.
    CURRENT_TOP=$(grep -oE '<release version="[^"]*"' "$METAINFO_FILE" | head -n1 | sed -E 's/.*version="([^"]*)".*/\1/')

    if [ "$CURRENT_TOP" = "$NEW_VERSION" ]; then
        echo "  [SKIP] flatpak metainfo.xml (already at $NEW_VERSION)"
    else
        if awk -v ver="$NEW_VERSION" -v rdate="$RELEASE_DATE" '
            !inserted && /<releases>/ {
                print
                print "    <release version=\"" ver "\" date=\"" rdate "\">"
                print "      <description>"
                print "        <p>Maintenance release.</p>"
                print "      </description>"
                print "    </release>"
                inserted = 1
                next
            }
            { print }
        ' "$METAINFO_FILE" > "$METAINFO_FILE.tmp" && mv "$METAINFO_FILE.tmp" "$METAINFO_FILE"; then
            echo "  [OK] flatpak/uk.co.easysmartapps.WeatherWidget.metainfo.xml (release: $NEW_VERSION, $RELEASE_DATE)"
            UPDATED=$((UPDATED + 1))
        else
            rm -f "$METAINFO_FILE.tmp"
            echo "  [FAIL] flatpak/uk.co.easysmartapps.WeatherWidget.metainfo.xml"
            FAILED=$((FAILED + 1))
        fi
    fi
fi

# --- Summary ---
echo ""
echo "=================================================="
echo "  Version bump complete: $NEW_VERSION"
echo "  Files updated: $UPDATED"
if [ "$FAILED" -gt 0 ]; then
    echo "  Files failed:  $FAILED"
fi
echo "=================================================="
echo ""
echo "Tip: Run 'node installer/check-versions.js' to verify all versions are in sync."
