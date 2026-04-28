# Weather Widget - Windows Installer Packaging

This directory contains everything needed to build production-ready Windows
installers for the Weather Widget application.

## Two Packaging Options

| Format | Use Case | Script |
|--------|----------|--------|
| **MSI** | Traditional Windows installer, sideloading | `build-msi.ps1` |
| **MSIX** | Microsoft Store submission (recommended) | `build-msix.ps1` |

## Prerequisites

### Required for both

1. **Go 1.25+** with CGO enabled (Fyne requires CGO)
2. **go-winres** — embeds icon, manifest, and version info into the exe:
   ```powershell
   go install github.com/tc-hib/go-winres@latest
   ```
3. **A code signing certificate** — required for production/Store builds.
   For Store apps, you get one through Microsoft Partner Center.

### For MSI builds

4. **WiX Toolset v4+**:
   ```powershell
   dotnet tool install --global wix
   ```

### For MSIX builds (Microsoft Store)

4. **Windows 10/11 SDK** — provides `MakeAppx.exe` and `SignTool.exe`.
   Install via Visual Studio Installer or the standalone SDK installer.

## Quick Start

### Build an unsigned MSI (for testing)

```powershell
.\installer\build-msi.ps1 -Version "1.0.0.0" -SkipSign
```

### Build a signed MSI

```powershell
.\installer\build-msi.ps1 -Version "1.0.0.0" -CertPath ".\cert.pfx" -CertPassword "yourpassword"
```

### Build an MSIX for Microsoft Store (recommended)

```powershell
# For Store submission — no certificate needed, Microsoft signs it:
.\installer\build-msix.ps1 -Version "1.0.0.0" -StoreUpload

# For local sideload testing with your own cert:
.\installer\build-msix.ps1 -Version "1.0.0.0" -CertPath ".\cert.pfx" -CertPassword "yourpassword"
```

The `-StoreUpload` flag produces an unsigned `.msixupload` file that you
upload directly to Partner Center. Microsoft signs the package during
certification — you do not need your own code signing certificate for Store
submissions.

Output goes to `.\build\`.

## Store Assets (MSIX only)

Before submitting to the Microsoft Store, place properly sized PNG images in
`installer/store-assets/`:

| File | Size | Purpose |
|------|------|---------|
| `StoreLogo.png` | 50x50 | Store listing logo |
| `Square44x44Logo.png` | 44x44 | Taskbar, Start menu |
| `Square150x150Logo.png` | 150x150 | Start menu tile |
| `Wide310x150Logo.png` | 310x150 | Wide Start tile |
| `Square310x310Logo.png` | 310x310 | Large Start tile |

If these are missing, the build script creates 1x1 placeholders so the
package builds, but you must replace them before Store submission.

## Icon File

Place your application icon as `winres/icon.ico`. This gets embedded into the
exe and is used for the taskbar, window title, and Explorer file icon.

To convert a PNG to ICO, use ImageMagick:
```bash
magick convert icon.png -define icon:auto-resize=256,128,64,48,32,16 icon.ico
```

## AppxManifest.xml Configuration

Before submitting to the Store, verify these fields in `AppxManifest.xml`
match your Partner Center **Product Identity** page exactly:

- `Identity Name` — from Partner Center > Product Identity > "Package/Identity/Name"
- `Identity Publisher` — from Partner Center > Product Identity > "Package/Identity/Publisher"
- `PublisherDisplayName` — your publisher display name from Partner Center

The manifest is currently configured with the Partner Center App ID
(`47955afa-afc7-46ee-abc1-02ab2632b4ad`) as a placeholder. You **must**
replace the Name and Publisher with the exact values from your Product
Identity page before uploading, or Partner Center will reject the package.

### How to find your Product Identity values

1. Go to [Partner Center](https://partner.microsoft.com/dashboard)
2. Select your app (Weather Widget)
3. Go to **Product management** > **Product Identity**
4. Copy the **Package/Identity/Name** and **Package/Identity/Publisher** values
5. Update `installer/AppxManifest.xml` with those values

## File Structure

```
installer/
├── AppxManifest.xml      # MSIX package manifest
├── Package.wxs           # WiX MSI definition
├── build-msi.ps1         # MSI build script
├── build-msix.ps1        # MSIX build script
├── store-assets/         # Store tile images (you provide these)
│   ├── StoreLogo.png
│   ├── Square44x44Logo.png
│   ├── Square150x150Logo.png
│   ├── Wide310x150Logo.png
│   └── Square310x310Logo.png
└── README.md             # This file

winres/
├── winres.json           # Windows resource definitions
└── icon.ico              # Application icon (you provide this)
```
