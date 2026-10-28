# Weather Widget Build Script for Windows
# Usage: .\scripts\build.ps1 [build|test|clean|vet] [-Arch amd64|arm64|both]
# Without -Arch, builds for the current host architecture (amd64 or arm64)
# and produces weatherwidget.exe.
#
# -Arch amd64  -> weatherwidget.exe + weatherwidget-amd64.exe (both amd64)
# -Arch arm64  -> weatherwidget.exe + weatherwidget-arm64.exe (both arm64)
# -Arch both   -> weatherwidget-amd64.exe + weatherwidget-arm64.exe
#
# -Arch only affects the "build" target; test/vet always use the host architecture.

param(
    [Parameter(Position = 0)]
    [ValidateSet("build", "test", "clean", "vet")]
    [string]$Target = "build",

    [Parameter(Position = 1)]
    [ValidateSet("amd64", "arm64", "both")]
    [string]$Arch
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

# Remember the original PATH so per-architecture toolchains don't accumulate.
$OriginalPath = $env:PATH

# Configure compiler and environment variables for the given target architecture
function Set-CompilerForArch {
    param([string]$TargetArch)

    $env:PATH = $OriginalPath
    Remove-Item Env:CC -ErrorAction SilentlyContinue
    Remove-Item Env:CXX -ErrorAction SilentlyContinue

    $env:GOOS = "windows"
    $env:GOARCH = $TargetArch
    $env:CGO_ENABLED = "1"

    if ($TargetArch -eq "arm64") {
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
    elseif ($TargetArch -eq "amd64") {
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

# Build a single binary for the given architecture. Exits on failure.
function Invoke-ArchBuild {
    param(
        [string]$TargetArch,
        [string]$OutputName
    )
    $outPath = Join-Path $BuildDir $OutputName

    Write-Host "Building $OutputName v$Version ($TargetArch)..." -ForegroundColor Cyan
    Set-CompilerForArch -TargetArch $TargetArch

    go build -ldflags="$LdFlags" -o $outPath $CmdPath
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Build successful: $outPath (v$Version, $TargetArch)" -ForegroundColor Green
    }
    else {
        Write-Host "Build failed for $TargetArch" -ForegroundColor Red
        exit 1
    }
    return $outPath
}

# Copy an already-built binary to another name in the build directory.
function Copy-ArchBuild {
    param(
        [string]$SourcePath,
        [string]$OutputName,
        [string]$TargetArch
    )
    $outPath = Join-Path $BuildDir $OutputName
    Copy-Item -Path $SourcePath -Destination $outPath -Force
    Write-Host "Build successful: $outPath (v$Version, $TargetArch)" -ForegroundColor Green
}

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

        switch ($Arch) {
            "amd64" {
                $built = Invoke-ArchBuild -TargetArch "amd64" -OutputName "weatherwidget-amd64.exe"
                Copy-ArchBuild -SourcePath $built -OutputName "weatherwidget.exe" -TargetArch "amd64"
            }
            "arm64" {
                $built = Invoke-ArchBuild -TargetArch "arm64" -OutputName "weatherwidget-arm64.exe"
                Copy-ArchBuild -SourcePath $built -OutputName "weatherwidget.exe" -TargetArch "arm64"
            }
            "both" {
                Invoke-ArchBuild -TargetArch "arm64" -OutputName "weatherwidget-arm64.exe" | Out-Null
                Invoke-ArchBuild -TargetArch "amd64" -OutputName "weatherwidget-amd64.exe" | Out-Null
            }
            default {
                # No -Arch flag: build for host architecture as weatherwidget.exe
                Invoke-ArchBuild -TargetArch $HostArch -OutputName "weatherwidget.exe" | Out-Null
            }
        }
        $env:PATH = $OriginalPath
    }
    "test" {
        Write-Host "Running tests ($HostArch)..."
        Set-CompilerForArch -TargetArch $HostArch
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
        Set-CompilerForArch -TargetArch $HostArch
        go vet ./...
    }
}