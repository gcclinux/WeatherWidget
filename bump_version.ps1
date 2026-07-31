# Bump Version Script for Windows
# Updates the version across all project files.
#
# Usage:
#   .\bump_version.ps1 0.1.0
#
# This updates:
#   - release (source of truth)
#   - internal/i18n/locales/*.json (About tab version display)
#   - installer/AppxManifest.xml (MSIX package version, 4-part)
#   - docs/site/index.html (hero badge version)
#   - README.md (example commands)

param(
    [Parameter(Position=0, Mandatory=$true)]
    [string]$NewVersion
)

# Validate version format (semver-like: digits separated by dots).
if ($NewVersion -notmatch '^\d+\.\d+\.\d+(\.\d+)?$') {
    Write-Host "Error: Invalid version format '$NewVersion'. Expected format: X.Y.Z or X.Y.Z.W" -ForegroundColor Red
    Write-Host "Example: .\bump_version.ps1 0.1.0"
    exit 1
}

$ProjectRoot = $PSScriptRoot
Write-Host "Bumping version to $NewVersion" -ForegroundColor Cyan
Write-Host ""

# Track results
$updated = @()
$failed = @()

# --- 1. Update release file ---
$releaseFile = Join-Path $ProjectRoot "release"
try {
    Set-Content -Path $releaseFile -Value $NewVersion -NoNewline
    $updated += "release"
    Write-Host "  [OK] release" -ForegroundColor Green
} catch {
    $failed += "release: $_"
    Write-Host "  [FAIL] release: $_" -ForegroundColor Red
}

# --- 2. Update locale JSON files (settings.about.version) ---
$localeDir = Join-Path $ProjectRoot "internal" "i18n" "locales"
if (Test-Path $localeDir) {
    Get-ChildItem -Path $localeDir -Filter "*.json" | ForEach-Object {
        $filePath = $_.FullName
        $fileName = $_.Name
        try {
            $content = Get-Content $filePath -Raw
            # Match: "settings.about.version": "**Label:** oldversion"
            $newContent = $content -replace '("settings\.about\.version":\s*"(\*\*[^*]+\*\*)\s*)[^"]*"', "`${1}$NewVersion`""
            if ($newContent -ne $content) {
                Set-Content -Path $filePath -Value $newContent -NoNewline
                $updated += "internal/i18n/locales/$fileName"
                Write-Host "  [OK] internal/i18n/locales/$fileName" -ForegroundColor Green
            } else {
                Write-Host "  [--] internal/i18n/locales/$fileName (no match or already current)" -ForegroundColor Gray
            }
        } catch {
            $failed += "internal/i18n/locales/${fileName}: $_"
            Write-Host "  [FAIL] internal/i18n/locales/${fileName}: $_" -ForegroundColor Red
        }
    }
}

# --- 3. Update installer/AppxManifest.xml (needs 4-part version) ---
$manifestFile = Join-Path $ProjectRoot "installer" "AppxManifest.xml"
if (Test-Path $manifestFile) {
    try {
        # AppxManifest requires exactly 4-part version (Major.Minor.Patch.Build).
        $parts = $NewVersion.Split('.')
        if ($parts.Count -eq 3) {
            $msixVersion = "$NewVersion.0"
        } else {
            $msixVersion = $NewVersion
        }
        $content = Get-Content $manifestFile -Raw
        # Only update the Version attribute inside the <Identity> element.
        $newContent = $content -replace '(<Identity[^>]*\bVersion=")[^"]*(")', "`${1}$msixVersion`${2}"
        if ($newContent -ne $content) {
            Set-Content -Path $manifestFile -Value $newContent -NoNewline
            $updated += "installer/AppxManifest.xml"
            Write-Host "  [OK] installer/AppxManifest.xml (version: $msixVersion)" -ForegroundColor Green
        } else {
            Write-Host "  [--] installer/AppxManifest.xml (no change)" -ForegroundColor Gray
        }
    } catch {
        $failed += "installer/AppxManifest.xml: $_"
        Write-Host "  [FAIL] installer/AppxManifest.xml: $_" -ForegroundColor Red
    }
}

# --- 4. Update docs/site/index.html (hero badge) ---
$indexHtml = Join-Path $ProjectRoot "docs" "site" "index.html"
if (Test-Path $indexHtml) {
    try {
        $content = Get-Content $indexHtml -Raw
        # Match: <span id="app-version">v0.0.6.1</span>
        $newContent = $content -replace '(<span id="app-version">v)[^<]*(</span>)', "`${1}$NewVersion`${2}"
        if ($newContent -ne $content) {
            Set-Content -Path $indexHtml -Value $newContent -NoNewline
            $updated += "docs/site/index.html"
            Write-Host "  [OK] docs/site/index.html" -ForegroundColor Green
        } else {
            Write-Host "  [--] docs/site/index.html (no change)" -ForegroundColor Gray
        }
    } catch {
        $failed += "docs/site/index.html: $_"
        Write-Host "  [FAIL] docs/site/index.html: $_" -ForegroundColor Red
    }
}

# --- 5. Update README.md (example MSI build commands) ---
$readmeFile = Join-Path $ProjectRoot "README.md"
if (Test-Path $readmeFile) {
    try {
        $content = Get-Content $readmeFile -Raw
        # Match: -Version "X.Y.Z.W" or -Version "X.Y.Z"
        $newContent = $content -replace '(-Version\s+")\d+\.\d+\.\d+(\.\d+)?(")', "`${1}$NewVersion`${3}"
        if ($newContent -ne $content) {
            Set-Content -Path $readmeFile -Value $newContent -NoNewline
            $updated += "README.md"
            Write-Host "  [OK] README.md" -ForegroundColor Green
        } else {
            Write-Host "  [--] README.md (no change)" -ForegroundColor Gray
        }
    } catch {
        $failed += "README.md: $_"
        Write-Host "  [FAIL] README.md: $_" -ForegroundColor Red
    }
}

# --- Summary ---
Write-Host ""
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "  Version bump complete: $NewVersion" -ForegroundColor Cyan
Write-Host "  Files updated: $($updated.Count)" -ForegroundColor Green
if ($failed.Count -gt 0) {
    Write-Host "  Files failed:  $($failed.Count)" -ForegroundColor Red
    foreach ($f in $failed) {
        Write-Host "    - $f" -ForegroundColor Red
    }
}
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Tip: Run 'node installer/check-versions.js' to verify all versions are in sync." -ForegroundColor Gray
