# Weather Widget Build Script for Windows
# Usage: 
#   .\scripts\build.ps1                         # Builds for current host arch
#   .\scripts\build.ps1 build amd64             # Builds amd64
#   .\scripts\build.ps1 build arm64             # Builds arm64
#   .\scripts\build.ps1 build both              # Builds both amd64 and arm64
#   .\scripts\build.ps1 -Target build -Arch all # Builds both amd64 and arm64
#   .\scripts\build.ps1 test
#   .\scripts\build.ps1 clean
#   .\scripts\build.ps1 vet

param(
    [Parameter(Position = 0)]
    [ValidateSet("build", "test", "clean", "vet", "amd64", "arm64", "both", "all")]
    [string]$Target = "build",

    [Parameter(Position = 1)]
    [ValidateSet("auto", "amd64", "arm64", "both", "all")]
    [string]$Arch = "auto"
)

# Allow passing arch directly as first argument (e.g., .\scripts\build.ps1 arm64)
if ($Target -in "amd64", "arm64", "both", "all") {
    $Arch = $Target
    $Target = "build"
}

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

# Detect host architecture
$HostArch = if ($env:PROCESSOR_ARCHITECTURE -match "ARM64|AARCH64" -or $env:PROCESSOR_ARCHITEW6432 -match "ARM64|AARCH64") {
    "arm64"
} else {
    "amd64"
}

# Determine target architectures
[string[]]$TargetArches = switch ($Arch.ToLower()) {
    "both"  { "amd64", "arm64" }
    "all"   { "amd64", "arm64" }
    "amd64" { "amd64" }
    "arm64" { "arm64" }
    default {
        if ($env:GOARCH -and ($env:GOARCH -in "amd64", "arm64")) {
            $env:GOARCH
        } else {
            $HostArch
        }
    }
}

# Configure compiler and environment variables for the given architecture
function Set-CompilerForArch {
    param([string]$TargetArch)

    $env:GOOS = "windows"
    $env:GOARCH = $TargetArch
    $env:CGO_ENABLED = "1"

    if ($TargetArch -eq "arm64") {
        if (Test-Path "C:\msys64\clangarm64\bin\clang.exe") {
            $env:PATH = "C:\msys64\clangarm64\bin;$env:PATH"
            $env:CC = "clang"
            $env:CXX = "clang++"
        } elseif (Test-Path "C:\msys64\clangarm64\bin\gcc.exe") {
            $env:PATH = "C:\msys64\clangarm64\bin;$env:PATH"
            $env:CC = "gcc"
            $env:CXX = "g++"
        }
    } elseif ($TargetArch -eq "amd64") {
        if (Test-Path "C:\msys64\ucrt64\bin\gcc.exe") {
            $env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
            $env:CC = "gcc"
            $env:CXX = "g++"
        } elseif (Test-Path "C:\msys64\mingw64\bin\gcc.exe") {
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

switch ($Target) {
    "build" {
        $BuildDir = Join-Path $PSScriptRoot "build"
        if (-not (Test-Path $BuildDir)) {
            New-Item -ItemType Directory -Path $BuildDir -Force | Out-Null
        }

        Update-LocaleVersions -Ver $Version
        Update-WinresVersion -Ver $Version

        $successList = @()
        $failList = @()

        foreach ($currArch in $TargetArches) {
            $binaryName = "weatherwidget-$currArch.exe"
            $binaryPath = Join-Path $BuildDir $binaryName
            Write-Host "Building $binaryName v$Version ($currArch)..." -ForegroundColor Cyan

            Set-CompilerForArch -TargetArch $currArch

            go build -ldflags="$LdFlags" -o $binaryPath $CmdPath
            if ($LASTEXITCODE -eq 0) {
                Write-Host "Build successful: $binaryPath (v$Version)" -ForegroundColor Green
                $successList += $binaryPath
            } else {
                Write-Host "Build failed for $currArch" -ForegroundColor Red
                $failList += $currArch
            }
        }

        if ($failList.Count -gt 0) {
            Write-Host "Failed to build architecture(s): $($failList -join ', ')" -ForegroundColor Red
            exit 1
        }
    }
    "test" {
        Write-Host "Running tests..."
        $archToTest = $TargetArches[0]
        Set-CompilerForArch -TargetArch $archToTest
        go test ./...
    }
    "clean" {
        Write-Host "Cleaning build artifacts..."
        $BuildDir = Join-Path $PSScriptRoot "build"
        if (Test-Path $BuildDir) {
            $artifacts = Get-ChildItem -Path $BuildDir -Filter "weatherwidget*.exe" -ErrorAction SilentlyContinue
            if ($artifacts) {
                $artifacts | Remove-Item -Force
                Write-Host "Removed: $($artifacts.Name -join ', ')"
            } else {
                Write-Host "Nothing to clean in $BuildDir"
            }
        } else {
            Write-Host "Nothing to clean"
        }
    }
    "vet" {
        Write-Host "Running go vet..."
        $archToVet = $TargetArches[0]
        Set-CompilerForArch -TargetArch $archToVet
        go vet ./...
    }
}
