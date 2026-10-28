# Weather Widget Build Script for Windows
# Usage: .\scripts\build.ps1 [build|test|clean|vet]
# Automatically builds for the current host architecture (amd64 or arm64).

param(
    [Parameter(Position = 0)]
    [ValidateSet("build", "test", "clean", "vet")]
    [string]$Target = "build"
)

$CmdPath = "./cmd/weatherwidget/"

# Read version from the release file.
$ProjectRoot = Split-Path -Path $PSScriptRoot -Parent
$ReleaseFile = Join-Path $ProjectRoot "release"
if (-Not (Test-Path $ReleaseFile)) {
    Write-Host "Error: could not find 'release' file" -ForegroundColor Red
    exit 1
}
$Version = (Get-Content $ReleaseFile -Raw).Trim()
if ([string]::IsNullOrWhiteSpace($Version)) {
    Write-Host "Error: 'release' file is empty" -ForegroundColor Red
    exit 1
}

$LdFlags = "-H windowsgui -s -w -X main.version=$Version"

# Detect current host architecture
$HostArch = if ($env:PROCESSOR_ARCHITECTURE -match "ARM64|AARCH64" -or $env:PROCESSOR_ARCHITEW6432 -match "ARM64|AARCH64") {
    "arm64"
}
else {
    "amd64"
}

# Configure compiler and environment variables for the current host architecture
function Set-CompilerForHost {
    $env:GOOS = "windows"
    $env:GOARCH = $HostArch
    $env:CGO_ENABLED = "1"

    if ($HostArch -eq "arm64") {
        if (Test-Path "C:\msys64\clangarm64\bin\clang.exe") {
            $env:PATH = "C:\msys64\clangarm64\bin;$env:PATH"
            $env:CC = "clang"
            $env:CXX = "clang++"
        }
        elseif (Test-Path "C:\msys64\clangarm64\bin\gcc.exe") {
            $env:PATH = "C:\msys64\clangarm64\bin;$env:PATH"
            $env:CC = "gcc"
            $env:CXX = "g++"
        }
    }
    elseif ($HostArch -eq "amd64") {
        if (Test-Path "C:\msys64\ucrt64\bin\gcc.exe") {
            $env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
            $env:CC = "gcc"
            $env:CXX = "g++"
        }
        elseif (Test-Path "C:\msys64\mingw64\bin\gcc.exe") {
            $env:PATH = "C:\msys64\mingw64\bin;$env:PATH"
            $env:CC = "gcc"
            $env:CXX = "g++"
        }
    }
}

# Update version in all locale JSON files so the About tab matches.
function Update-LocaleVersions {
    param([string]$Ver)
    $localeDir = Join-Path $ProjectRoot "internal/i18n/locales"
    if (Test-Path $localeDir) {
        Get-ChildItem -Path $localeDir -Filter "*.json" | ForEach-Object {
            $content = Get-Content $_.FullName -Raw
            $updated = $content -replace '"settings\.about\.version":\s*"(\*\*[^:*]+:\*\*)\s*[^"]*"', "`"settings.about.version`": `"`$1 $Ver`""
            if ($updated -ne $content) {
                Set-Content -Path $_.FullName -Value $updated -NoNewline
            }
        }
    }
}

# Update version in winres/winres.json so Windows executable properties match.
function Update-WinresVersion {
    param([string]$Ver)
    $winresFile = Join-Path $ProjectRoot "winres/winres.json"
    if (Test-Path $winresFile) {
        $content = Get-Content $winresFile -Raw
        $updated = $content -replace '("version":\s*")[^"]*(")', "`${1}$Ver`${2}"
        $updated = $updated -replace '("file_version":\s*")[^"]*(")', "`${1}$Ver`${2}"
        $updated = $updated -replace '("product_version":\s*")[^"]*(")', "`${1}$Ver`${2}"
        $updated = $updated -replace '("FileVersion":\s*")[^"]*(")', "`${1}$Ver`${2}"
        $updated = $updated -replace '("ProductVersion":\s*")[^"]*(")', "`${1}$Ver`${2}"
        if ($updated -ne $content) {
            Set-Content -Path $winresFile -Value $updated -NoNewline
        }
    }
}

$BuildDir = Join-Path $PSScriptRoot "build"
$BinaryPath = Join-Path $BuildDir "weatherwidget.exe"

switch ($Target) {
    "build" {
        if (-not (Test-Path $BuildDir)) {
            New-Item -ItemType Directory -Path $BuildDir -Force | Out-Null
        }

        Update-LocaleVersions -Ver $Version
        Update-WinresVersion -Ver $Version

        $patchScript = Join-Path $PSScriptRoot "patch-fyne.ps1"
        if (Test-Path $patchScript) {
            & $patchScript
        }

        Write-Host "Building weatherwidget.exe v$Version ($HostArch)..." -ForegroundColor Cyan
        Set-CompilerForHost

        go build -ldflags="$LdFlags" -o $BinaryPath $CmdPath
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Build successful: $BinaryPath (v$Version, $HostArch)" -ForegroundColor Green
        }
        else {
            Write-Host "Build failed for $HostArch" -ForegroundColor Red
            exit 1
        }
    }
    "test" {
        Write-Host "Running tests ($HostArch)..."
        Set-CompilerForHost
        go test ./...
    }
    "clean" {
        Write-Host "Cleaning build artifacts..."
        if (Test-Path $BuildDir) {
            $artifacts = Get-ChildItem -Path $BuildDir -Filter "weatherwidget*.exe" -ErrorAction SilentlyContinue
            if ($artifacts) {
                $artifacts | Remove-Item -Force
                Write-Host "Removed: $($artifacts.Name -join ', ')"
            }
            else {
                Write-Host "Nothing to clean in $BuildDir"
            }
        }
        else {
            Write-Host "Nothing to clean"
        }
    }
    "vet" {
        Write-Host "Running go vet ($HostArch)..."
        Set-CompilerForHost
        go vet ./...
    }
}