# ============================================================================
# Weather Widget - MSI Package Builder (Microsoft Store / Sideload)
# ============================================================================
# Usage:
#   # Sign with certificate from Windows certificate store (by thumbprint):
#   .\installer\build-msi.ps1 -Version "0.0.6.1" -CertThumbprint "A9D97675327F7BF344BA308A357E91141A1DDE50"
#
#   # Sign with a .pfx file:
#   .\installer\build-msi.ps1 -Version "0.0.6.1" -CertPath "cert.pfx" -CertPassword "pass"
#
#   # Build without signing (for testing only):
#   .\installer\build-msi.ps1 -Version "0.0.6.1" -SkipSign
#
# Prerequisites:
#   - Go 1.25+ with CGO enabled (for Fyne)
#   - go-winres: go install github.com/tc-hib/go-winres@latest
#   - WiX v4+: dotnet tool install --global wix  (or WiX v7 via winget)
#   - signtool.exe (from Windows SDK)
#   - Code signing certificate (installed in cert store or as .pfx file)
# ============================================================================

param(
    [Parameter(Mandatory=$true)]
    [string]$Version,

    [string]$CertThumbprint = "A9D97675327F7BF344BA308A357E91141A1DDE50",
    [string]$CertPath = "",
    [string]$CertPassword = "",
    [switch]$SkipSign,
    [switch]$SkipBuild
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

# Determine signing mode
$SignMode = "none"
if (-not $SkipSign) {
    if ($CertThumbprint) {
        $SignMode = "thumbprint"
    } elseif ($CertPath) {
        $SignMode = "pfx"
    }
}

# Helper function to sign a file
function Sign-File {
    param([string]$FilePath, [string]$Description)

    if ($SignMode -eq "none") { return }

    $signArgs = @("sign", "/fd", "SHA256", "/tr", "http://timestamp.digicert.com", "/td", "SHA256")

    if ($SignMode -eq "thumbprint") {
        $signArgs += @("/sha1", $CertThumbprint, "/s", "My")
    } else {
        $signArgs += @("/f", $CertPath)
        if ($CertPassword) { $signArgs += @("/p", $CertPassword) }
    }

    if ($Description) {
        $signArgs += @("/d", $Description)
    }

    $signArgs += $FilePath

    & signtool @signArgs
    if ($LASTEXITCODE -ne 0) { throw "Signing failed for $FilePath" }
}

Push-Location $ProjectRoot

try {
    $BuildDir = ".\build"
    $OutputMsi = ".\build\WeatherWidget-$Version.msi"

    Write-Host ""
    Write-Host "============================================" -ForegroundColor Cyan
    Write-Host "  Weather Widget MSI Builder v$Version" -ForegroundColor Cyan
    Write-Host "  Sign mode: $SignMode" -ForegroundColor Cyan
    Write-Host "============================================" -ForegroundColor Cyan
    Write-Host ""

    # -------------------------------------------------------------------------
    # Step 1: Clean previous build
    # -------------------------------------------------------------------------
    Write-Host "[1/5] Cleaning previous build..." -ForegroundColor Yellow
    if (Test-Path $BuildDir) { Remove-Item -Recurse -Force $BuildDir }
    New-Item -ItemType Directory -Path $BuildDir -Force | Out-Null
    Write-Host "       Done." -ForegroundColor Green

    # -------------------------------------------------------------------------
    # Step 2: Generate Windows resources
    # -------------------------------------------------------------------------
    if (-not $SkipBuild) {
        Write-Host "[2/5] Generating Windows resources..." -ForegroundColor Yellow

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
        Write-Host "[3/5] Building weatherwidget.exe..." -ForegroundColor Yellow
        $env:GOOS = "windows"
        $env:GOARCH = "amd64"
        $env:CGO_ENABLED = "1"

        $ldflags = "-H windowsgui -s -w -X main.version=$Version"
        go build -ldflags="$ldflags" -o "$BuildDir\weatherwidget.exe" .\cmd\weatherwidget\
        if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
        Write-Host "       Done. Output: $BuildDir\weatherwidget.exe" -ForegroundColor Green
    } else {
        Write-Host "[2/5] Skipping (SkipBuild)." -ForegroundColor DarkGray
        Write-Host "[3/5] Skipping (SkipBuild)." -ForegroundColor DarkGray
        if (-not (Test-Path "$BuildDir\weatherwidget.exe")) {
            throw "No existing exe found at $BuildDir\weatherwidget.exe. Remove -SkipBuild flag."
        }
    }

    # -------------------------------------------------------------------------
    # Step 4: Sign the executable
    # -------------------------------------------------------------------------
    if ($SignMode -ne "none") {
        Write-Host "[4/5] Signing executable..." -ForegroundColor Yellow
        Sign-File -FilePath "$BuildDir\weatherwidget.exe" -Description "Weather Widget"
        Write-Host "       Done." -ForegroundColor Green
    } else {
        Write-Host "[4/5] Skipping signing." -ForegroundColor DarkGray
    }

    # -------------------------------------------------------------------------
    # Step 5: Build MSI with WiX
    # -------------------------------------------------------------------------
    Write-Host "[5/5] Building MSI with WiX..." -ForegroundColor Yellow

    # Resolve wix.exe — check PATH first, then known install locations
    $wixExe = (Get-Command "wix" -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source)
    if (-not $wixExe) {
        $wixPaths = @(
            "${env:ProgramFiles}\WiX Toolset v7.0\bin\wix.exe",
            "${env:ProgramFiles}\WiX Toolset v4.0\bin\wix.exe"
        )
        foreach ($p in $wixPaths) {
            if (Test-Path $p) { $wixExe = $p; break }
        }
    }
    if (-not $wixExe) {
        throw "WiX not found. Install with: dotnet tool install --global wix"
    }
    Write-Host "       Using WiX: $wixExe" -ForegroundColor DarkGray

    & $wixExe build .\installer\Package.wxs -d BuildDir=$BuildDir -d Version=$Version -o $OutputMsi
    if ($LASTEXITCODE -ne 0) { throw "WiX build failed" }
    Write-Host "       MSI created." -ForegroundColor Green

    # Sign the MSI
    if ($SignMode -ne "none") {
        Write-Host "       Signing MSI..." -ForegroundColor Yellow
        Sign-File -FilePath $OutputMsi -Description "Weather Widget Installer"
        Write-Host "       MSI signed." -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "============================================" -ForegroundColor Green
    Write-Host "  BUILD SUCCESSFUL" -ForegroundColor Green
    Write-Host "  Output: $OutputMsi" -ForegroundColor Green
    Write-Host "============================================" -ForegroundColor Green
    Write-Host ""

    if ($SignMode -eq "none") {
        Write-Host "  WARNING: Package is unsigned. Microsoft Store" -ForegroundColor Yellow
        Write-Host "  requires SHA256 code signing (Policy 10.2.9)." -ForegroundColor Yellow
        Write-Host "  Use -CertThumbprint or -CertPath to sign." -ForegroundColor Yellow
        Write-Host ""
    }

} finally {
    Pop-Location
}
