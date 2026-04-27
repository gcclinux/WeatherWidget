# ============================================================================
# Weather Widget - MSI Package Builder (Traditional Installer)
# ============================================================================
# Usage:
#   .\installer\build-msi.ps1 -Version "1.0.0.0"
#   .\installer\build-msi.ps1 -Version "1.0.0.0" -CertPath "cert.pfx" -CertPassword "pass"
#   .\installer\build-msi.ps1 -Version "1.0.0.0" -SkipSign
#
# Prerequisites:
#   - Go 1.25+ with CGO enabled (for Fyne)
#   - go-winres: go install github.com/tc-hib/go-winres@latest
#   - WiX v4+: dotnet tool install --global wix
#   - Code signing certificate (for production builds)
# ============================================================================

param(
    [Parameter(Mandatory=$true)]
    [string]$Version,

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

Push-Location $ProjectRoot

try {
    $BuildDir = ".\build"
    $OutputMsi = ".\build\WeatherWidget-$Version.msi"

    Write-Host ""
    Write-Host "============================================" -ForegroundColor Cyan
    Write-Host "  Weather Widget MSI Builder v$Version" -ForegroundColor Cyan
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
        Write-Host "       Done." -ForegroundColor Green
    } else {
        Write-Host "[2/5] Skipping (SkipBuild)." -ForegroundColor DarkGray
        Write-Host "[3/5] Skipping (SkipBuild)." -ForegroundColor DarkGray
    }

    # -------------------------------------------------------------------------
    # Step 4: Sign the executable
    # -------------------------------------------------------------------------
    if (-not $SkipSign -and $CertPath) {
        Write-Host "[4/5] Signing executable..." -ForegroundColor Yellow
        $signArgs = @("sign", "/fd", "SHA256", "/tr", "http://timestamp.digicert.com", "/td", "SHA256", "/f", $CertPath)
        if ($CertPassword) { $signArgs += @("/p", $CertPassword) }
        $signArgs += "$BuildDir\weatherwidget.exe"
        & signtool @signArgs
        if ($LASTEXITCODE -ne 0) { throw "Signing failed" }
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
            "${env:ProgramFiles}\WiX Toolset v4.0\bin\wix.exe",
            "${env:ProgramFiles(x86)}\WiX Toolset v3.14\bin\candle.exe"
        )
        foreach ($p in $wixPaths) {
            if (Test-Path $p) { $wixExe = $p; break }
        }
    }
    if (-not $wixExe) {
        throw "WiX not found. Install with: winget install WiXToolset.WiXCLI"
    }
    Write-Host "       Using WiX: $wixExe" -ForegroundColor DarkGray

    & $wixExe build .\installer\Package.wxs -d BuildDir=$BuildDir -d Version=$Version -o $OutputMsi
    if ($LASTEXITCODE -ne 0) { throw "WiX build failed" }

    # Sign the MSI
    if (-not $SkipSign -and $CertPath) {
        Write-Host "       Signing MSI..." -ForegroundColor Yellow
        $signArgs = @("sign", "/fd", "SHA256", "/tr", "http://timestamp.digicert.com", "/td", "SHA256", "/f", $CertPath)
        if ($CertPassword) { $signArgs += @("/p", $CertPassword) }
        $signArgs += $OutputMsi
        & signtool @signArgs
        if ($LASTEXITCODE -ne 0) { throw "MSI signing failed" }
    }

    Write-Host ""
    Write-Host "============================================" -ForegroundColor Green
    Write-Host "  BUILD SUCCESSFUL" -ForegroundColor Green
    Write-Host "  Output: $OutputMsi" -ForegroundColor Green
    Write-Host "============================================" -ForegroundColor Green
    Write-Host ""

} finally {
    Pop-Location
}
