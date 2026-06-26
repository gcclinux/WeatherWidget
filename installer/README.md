# Weather Widget - Installer Packaging

This directory contains everything needed to build production-ready installers
for the Weather Widget application.

## Packaging Options

| Format | Platform | Use Case | Script |
|--------|----------|----------|--------|
| **PKG** | macOS | Standard macOS installer wizard | `build-pkg.sh` |
| **MSI** | Windows | Traditional Windows installer | `build-msi.ps1` |
| **MSIX** | Windows | Microsoft Store submission | `build-msix.ps1` |

---

## macOS PKG Installer

The `.pkg` installer runs the standard macOS installer wizard (the same UI
you see when installing most apps). It places `WeatherWidget.app` into
`/Applications` and launches it automatically when done.

### Prerequisites

All tools are built into macOS — no extra installs needed:
- `pkgbuild`, `productbuild` — package builders (part of Xcode CLT)
- `codesign` — code signing (part of Xcode CLT)
- `xcrun notarytool` — notarization (part of Xcode CLT, macOS 12+)

For **signed/notarized** builds you need an Apple Developer account with:
- **Developer ID Application** certificate (signs the `.app`)
- **Developer ID Installer** certificate (signs the `.pkg`)

### Quick Start

```bash
# Build unsigned installer (for testing):
make build-darwin-pkg VERSION=1.2.0

# Or call the script directly:
./installer/build-pkg.sh --version 1.2.0 --skip-sign
```

```bash
# Build signed installer (for distribution):
make build-darwin-pkg VERSION=1.2.0 \
    SIGN_APP="Developer ID Application: Your Name (TEAMID)" \
    SIGN_PKG="Developer ID Installer: Your Name (TEAMID)"
```

```bash
# Build signed + notarized installer (required for Gatekeeper on macOS 10.15+):
make build-darwin-pkg VERSION=1.2.0 \
    SIGN_APP="Developer ID Application: Your Name (TEAMID)" \
    SIGN_PKG="Developer ID Installer: Your Name (TEAMID)" \
    NOTARIZE=1 \
    APPLE_ID=you@example.com \
    TEAM_ID=YOURTEAMID \
    APP_PWD=xxxx-xxxx-xxxx-xxxx
```

Output goes to `build/WeatherWidget-<version>.pkg`.

### Signing Identities

To find your certificate names, run:
```bash
security find-identity -v -p codesigning
```

Look for entries like:
- `Developer ID Application: Your Name (XXXXXXXXXX)`
- `Developer ID Installer: Your Name (XXXXXXXXXX)`

### Notarization

Notarization is required for Gatekeeper to allow users to open your app
without warnings on macOS 10.15+. You need an **app-specific password**:

1. Go to [appleid.apple.com](https://appleid.apple.com) → Security → App-Specific Passwords
2. Generate a new password for "WeatherWidget notarization"
3. Pass it as `APP_PWD=xxxx-xxxx-xxxx-xxxx`

### Entitlements

`installer/entitlements.plist` is generated automatically on first run. Edit
it to match the actual capabilities your app uses before signing for
distribution. The defaults allow outbound network access (for weather API
calls).

---

## Windows MSI Installer

### Prerequisites

1. **Go 1.25+** with CGO enabled (Fyne requires CGO)
2. **go-winres** — embeds icon, manifest, and version info into the exe:
   ```powershell
   go install github.com/tc-hib/go-winres@latest
   ```
3. **WiX Toolset v4+**:
   ```powershell
   dotnet tool install --global wix
   ```
4. **A code signing certificate**

### Build

```powershell
# Unsigned (for testing only):
.\installer\build-msi.ps1 -Version "1.0.0.0" -SkipSign

# Signed with certificate from Windows certificate store:
.\installer\build-msi.ps1 -Version "1.0.0.0" -CertThumbprint "THUMBPRINT"

# Signed with a .pfx file:
.\installer\build-msi.ps1 -Version "1.0.0.0" -CertPath "cert.pfx" -CertPassword "pass"
```

---

## Windows MSIX (Microsoft Store)

### Build

```powershell
# For Store submission (Microsoft signs it for you):
.\installer\build-msix.ps1 -Version "1.0.0.0" -StoreUpload

# For local sideload testing:
.\installer\build-msix.ps1 -Version "1.0.0.0" -CertPath "cert.pfx" -CertPassword "pass"
```

The `-StoreUpload` flag produces an unsigned `.msixupload` for Partner Center.

### Store Assets

Place properly sized PNG images in `installer/store-assets/`:

| File | Size | Purpose |
|------|------|---------|
| `StoreLogo.png` | 50x50 | Store listing logo |
| `Square44x44Logo.png` | 44x44 | Taskbar, Start menu |
| `Square150x150Logo.png` | 150x150 | Start menu tile |
| `Wide310x150Logo.png` | 310x150 | Wide Start tile |
| `Square310x310Logo.png` | 310x310 | Large Start tile |

### AppxManifest.xml

Verify `Identity Name` and `Identity Publisher` match your Partner Center
**Product Identity** page exactly before uploading.

---

## File Structure

```
installer/
├── build-pkg.sh          # macOS PKG installer script
├── entitlements.plist    # macOS app entitlements (auto-generated, then edit)
├── AppxManifest.xml      # MSIX package manifest (Windows)
├── Package.wxs           # WiX MSI definition (Windows)
├── build-msi.ps1         # MSI build script (Windows)
├── build-msix.ps1        # MSIX build script (Windows)
├── store-assets/         # Windows Store tile images
└── README.md             # This file

build/                    # Build output (git-ignored)
└── WeatherWidget-*.pkg   # macOS installer output
```
