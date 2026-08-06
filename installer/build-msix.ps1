# ============================================================================
# Weather Widget - MSIX Package Builder (Microsoft Store)
# ============================================================================
# Usage:
#   # For Microsoft Store submission (unsigned — Store signs it for you):
#   .\installer\build-msix.ps1 -Version "1.0.3" -StoreUpload
#
#   # For local testing with self-signed cert:
#   .\installer\build-msix.ps1 -Version "1.0.3" -CertPath "cert.pfx" -CertPassword "pass"
#
#   # For local testing without signing:
#   .\installer\build-msix.ps1 -Version "1.0.3" -SkipSign
#
# Prerequisites:
#   - Go 1.25+ with CGO enabled (for Fyne)
#   - go-winres: go install github.com/tc-hib/go-winres@latest
#   - Windows SDK (for MakeAppx.exe and SignTool.exe)
#   - Code signing certificate (only for local/sideload builds)
#
# Microsoft Store Notes:
#   - Use -StoreUpload to produce an .msixupload file for Partner Center.
#   - The Store signs the package — you do NOT need a certificate for submission.
#   - The AppxManifest.xml Identity Name and Publisher must match Partner Center
#     values exactly. Check Partner Center > Product Identity for your values.
#   - Partner Center App ID: 47955afa-afc7-46ee-abc1-02ab2632b4ad
# ============================================================================

param(
    [Parameter(Mandatory=$true)]
    [string]$Version,

    [string]$CertPath = "",
    [string]$CertPassword = "",
    [switch]$SkipSign,
    [switch]$SkipBuild,
    [switch]$StoreUpload
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
if (-not $ProjectRoot) { $ProjectRoot = (Get-Location).Path }

# Resolve go-winres from GOPATH/bin (may not be on PATH)
$GoPath = (go env GOPATH) | Out-String
$GoPath = $GoPath.Trim()
$GoWinRes = Join-Path $GoPath "bin\go-winres.exe"
if (-not (Test-Path $GoWinRes)) {
    throw "go-winres not found at $GoWinRes. Install with: go install github.com/tc-hib/go-winres@latest"
}

# Resolve paths relative to project root
Push-Location $ProjectRoot

try {
    $BuildDir = ".\build"
    $PackageDir = ".\build\package"
    $OutputMsix = ".\build\WeatherWidget-$Version.msix"
    $OutputMsixUpload = ".\build\WeatherWidget-$Version.msixupload"

    # Determine total steps based on mode
    if ($StoreUpload) {
        $totalSteps = 7
    } else {
        $totalSteps = 6
    }

    Write-Host ""
    Write-Host "============================================" -ForegroundColor Cyan
    Write-Host "  Weather Widget MSIX Builder v$Version" -ForegroundColor Cyan
    if ($StoreUpload) {
        Write-Host "  Mode: Microsoft Store Upload" -ForegroundColor Cyan
    }
    Write-Host "============================================" -ForegroundColor Cyan
    Write-Host ""

    # -------------------------------------------------------------------------
    # Step 1: Clean previous build
    # -------------------------------------------------------------------------
    Write-Host "[1/$totalSteps] Cleaning previous build..." -ForegroundColor Yellow
    if (Test-Path $BuildDir) { Remove-Item -Recurse -Force $BuildDir }
    New-Item -ItemType Directory -Path $PackageDir -Force | Out-Null
    New-Item -ItemType Directory -Path "$PackageDir\assets" -Force | Out-Null
    Write-Host "       Done." -ForegroundColor Green

    # -------------------------------------------------------------------------
    # Step 2: Generate Windows resources (icon, manifest, version info)
    # -------------------------------------------------------------------------
    if (-not $SkipBuild) {
        Write-Host "[2/$totalSteps] Generating Windows resources..." -ForegroundColor Yellow

        # Update version in winres.json before generating
        $winresJson = Get-Content ".\winres\winres.json" -Raw | ConvertFrom-Json
        $winresJson.RT_VERSION.'#1'.'0409'.fixed.file_version = $Version
        $winresJson.RT_VERSION.'#1'.'0409'.fixed.product_version = $Version
        $winresJson.RT_VERSION.'#1'.'0409'.info.'0409'.FileVersion = $Version
        $winresJson.RT_VERSION.'#1'.'0409'.info.'0409'.ProductVersion = $Version
        $winresJson | ConvertTo-Json -Depth 10 | Set-Content ".\winres\winres.json"

        & $GoWinRes make --in .\winres\winres.json --product-version $Version --file-version $Version
        if ($LASTEXITCODE -ne 0) { throw "go-winres failed" }
        Write-Host "       Done." -ForegroundColor Green

        # ---------------------------------------------------------------------
        # Step 3: Build the executable
        # ---------------------------------------------------------------------
        Write-Host "[3/$totalSteps] Building weatherwidget.exe..." -ForegroundColor Yellow
        $env:GOOS = "windows"
        $env:GOARCH = "amd64"
        $env:CGO_ENABLED = "1"

        $ldflags = "-H windowsgui -s -w -X main.version=$Version"
        go build -ldflags="$ldflags" -o "$BuildDir\weatherwidget.exe" .\cmd\weatherwidget\
        if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
        Write-Host "       Done. Output: $BuildDir\weatherwidget.exe" -ForegroundColor Green
    } else {
        Write-Host "[2/$totalSteps] Skipping resource generation (SkipBuild)." -ForegroundColor DarkGray
        Write-Host "[3/$totalSteps] Skipping build (SkipBuild)." -ForegroundColor DarkGray
        if (-not (Test-Path "$BuildDir\weatherwidget.exe")) {
            throw "No existing exe found at $BuildDir\weatherwidget.exe. Remove -SkipBuild flag."
        }
    }

    # -------------------------------------------------------------------------
    # Step 4: Sign the executable (skip for Store uploads)
    # -------------------------------------------------------------------------
    if ($StoreUpload) {
        Write-Host "[4/$totalSteps] Skipping exe signing (Store signs the package)." -ForegroundColor DarkGray
    } elseif (-not $SkipSign -and $CertPath) {
        Write-Host "[4/$totalSteps] Signing executable..." -ForegroundColor Yellow
        $signArgs = @(
            "sign", "/fd", "SHA256",
            "/tr", "http://timestamp.digicert.com",
            "/td", "SHA256",
            "/f", $CertPath
        )
        if ($CertPassword) { $signArgs += @("/p", $CertPassword) }
        $signArgs += "$BuildDir\weatherwidget.exe"

        & signtool @signArgs
        if ($LASTEXITCODE -ne 0) { throw "Executable signing failed" }
        Write-Host "       Done." -ForegroundColor Green
    } else {
        Write-Host "[4/$totalSteps] Skipping exe signing." -ForegroundColor DarkGray
    }

    # -------------------------------------------------------------------------
    # Step 5: Assemble MSIX package layout
    # -------------------------------------------------------------------------
    Write-Host "[5/$totalSteps] Assembling package layout..." -ForegroundColor Yellow

    # Copy exe
    Copy-Item "$BuildDir\weatherwidget.exe" "$PackageDir\"

    # Copy and update AppxManifest with current version
    # Use a lookbehind to only replace the Version inside the Identity element,
    # not the version="1.0" in the <?xml ?> declaration.
    $manifest = Get-Content ".\installer\AppxManifest.xml" -Raw
    $manifest = $manifest -replace '(?<=<Identity\s[^>]*)Version="[^"]*"', "Version=`"$Version`""
    # MakeAppx requires UTF-8 without BOM — Set-Content adds a BOM which breaks the XML declaration
    [System.IO.File]::WriteAllText("$PackageDir\AppxManifest.xml", $manifest, (New-Object System.Text.UTF8Encoding $false))

    # Copy store assets (user must provide these)
    $storeAssetsDir = ".\installer\store-assets"
    if (Test-Path $storeAssetsDir) {
        Copy-Item "$storeAssetsDir\*" "$PackageDir\assets\" -Force
        Write-Host "       Store assets copied." -ForegroundColor Green
    } else {
        Write-Host "       WARNING: No store-assets found at $storeAssetsDir" -ForegroundColor Red
        Write-Host "       Creating placeholder assets. Replace with real ones before submission!" -ForegroundColor Red
        # Create minimal placeholder PNGs (1x1 transparent) - replace these!
        $placeholders = @("StoreLogo.png", "Square44x44Logo.png", "Square150x150Logo.png", "Wide310x150Logo.png", "Square310x310Logo.png")
        foreach ($p in $placeholders) {
            # Create a minimal valid PNG (1x1 pixel, transparent)
            [byte[]]$png = @(137,80,78,71,13,10,26,10,0,0,0,13,73,72,68,82,0,0,0,1,0,0,0,1,8,6,0,0,0,31,21,196,137,0,0,0,13,73,68,65,84,120,156,98,0,0,0,2,0,1,226,33,188,51,0,0,0,0,73,69,78,68,174,66,96,130)
            [System.IO.File]::WriteAllBytes("$PackageDir\assets\$p", $png)
        }
    }

    Write-Host "       Done." -ForegroundColor Green

    # -------------------------------------------------------------------------
    # Step 6: Create MSIX package
    # -------------------------------------------------------------------------
    Write-Host "[6/$totalSteps] Creating MSIX package..." -ForegroundColor Yellow

    # Find MakeAppx.exe from Windows SDK
    $sdkPaths = @(
        "${env:ProgramFiles(x86)}\Windows Kits\10\bin\10.0.26100.0\x64",
        "${env:ProgramFiles(x86)}\Windows Kits\10\bin\10.0.22621.0\x64",
        "${env:ProgramFiles(x86)}\Windows Kits\10\bin\10.0.22000.0\x64",
        "${env:ProgramFiles(x86)}\Windows Kits\10\bin\10.0.19041.0\x64"
    )

    $makeAppx = $null
    foreach ($sdkPath in $sdkPaths) {
        $candidate = Join-Path $sdkPath "MakeAppx.exe"
        if (Test-Path $candidate) {
            $makeAppx = $candidate
            break
        }
    }

    if (-not $makeAppx) {
        # Try finding it on PATH
        $makeAppx = Get-Command "MakeAppx.exe" -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
    }

    if (-not $makeAppx) {
        throw "MakeAppx.exe not found. Install the Windows 10/11 SDK."
    }

    & $makeAppx pack /d $PackageDir /p $OutputMsix /o
    if ($LASTEXITCODE -ne 0) { throw "MakeAppx failed" }
    Write-Host "       Done." -ForegroundColor Green

    # -------------------------------------------------------------------------
    # Step 6b: Sign the MSIX (local/sideload only — not for Store)
    # -------------------------------------------------------------------------
    if ($StoreUpload) {
        # No signing needed — Partner Center signs Store packages
    } elseif (-not $SkipSign -and $CertPath) {
        Write-Host "       Signing MSIX..." -ForegroundColor Yellow
        $signArgs = @(
            "sign", "/fd", "SHA256",
            "/tr", "http://timestamp.digicert.com",
            "/td", "SHA256",
            "/f", $CertPath
        )
        if ($CertPassword) { $signArgs += @("/p", $CertPassword) }
        $signArgs += $OutputMsix

        & signtool @signArgs
        if ($LASTEXITCODE -ne 0) { throw "MSIX signing failed" }
        Write-Host "       Signed." -ForegroundColor Green
    }

    # -------------------------------------------------------------------------
    # Step 7 (Store only): Create .msixupload for Partner Center
    # -------------------------------------------------------------------------
    if ($StoreUpload) {
        Write-Host "[7/$totalSteps] Creating .msixupload for Partner Center..." -ForegroundColor Yellow

        # .msixupload is a ZIP containing the .msix (and optionally .msixsym)
        # Partner Center expects this format for Store submissions.
        # Compress-Archive in Windows PowerShell 5.1 only allows .zip extension,
        # so create as .zip first, then rename to .msixupload.
        if (Test-Path $OutputMsixUpload) { Remove-Item $OutputMsixUpload -Force }
        $tempZip = "$OutputMsixUpload.zip"
        if (Test-Path $tempZip) { Remove-Item $tempZip -Force }
        Compress-Archive -Path $OutputMsix -DestinationPath $tempZip -Force
        Move-Item -Path $tempZip -Destination $OutputMsixUpload -Force

        Write-Host "       Done." -ForegroundColor Green
    }

    # -------------------------------------------------------------------------
    # Summary
    # -------------------------------------------------------------------------
    Write-Host ""
    Write-Host "============================================" -ForegroundColor Green
    Write-Host "  BUILD SUCCESSFUL" -ForegroundColor Green
    Write-Host "  MSIX:   $OutputMsix" -ForegroundColor Green
    if ($StoreUpload) {
        Write-Host "  Upload: $OutputMsixUpload" -ForegroundColor Green
    }
    Write-Host "============================================" -ForegroundColor Green
    Write-Host ""

    if ($StoreUpload) {
        Write-Host "  NEXT STEPS for Microsoft Store submission:" -ForegroundColor Cyan
        Write-Host "  1. Verify AppxManifest.xml Identity Name and Publisher" -ForegroundColor White
        Write-Host "     match your Partner Center > Product Identity values." -ForegroundColor White
        Write-Host "  2. Replace placeholder store assets in installer\store-assets\" -ForegroundColor White
        Write-Host "     with properly sized PNGs before final submission." -ForegroundColor White
        Write-Host "  3. Upload $OutputMsixUpload to Partner Center." -ForegroundColor White
        Write-Host "     (Partner Center > Your App > Packages)" -ForegroundColor White
        Write-Host "  4. Microsoft will sign the package during certification." -ForegroundColor White
        Write-Host ""
        Write-Host "  Partner Center App ID: 47955afa-afc7-46ee-abc1-02ab2632b4ad" -ForegroundColor DarkGray
        Write-Host ""
    } elseif ($SkipSign -or -not $CertPath) {
        Write-Host "  NOTE: Package is unsigned. For sideloading," -ForegroundColor Yellow
        Write-Host "  provide -CertPath and -CertPassword parameters." -ForegroundColor Yellow
        Write-Host ""
    }

} finally {
    Pop-Location
}
