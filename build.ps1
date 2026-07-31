# Weather Widget Build Script for Windows
# Usage: .\build.ps1 [build|test|clean|vet]

param(
    [Parameter(Position=0)]
    [ValidateSet("build", "test", "clean", "vet")]
    [string]$Target = "build"
)

$BinaryName = "weatherwidget.exe"
$CmdPath = "./cmd/weatherwidget/"

# Read version from the release file.
$ReleaseFile = Join-Path $PSScriptRoot "release"
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

$env:GOOS = "windows"
$env:GOARCH = "amd64"

# Update version in all locale JSON files so the About tab matches.
function Update-LocaleVersions {
    param([string]$Ver)
    $localeDir = Join-Path $PSScriptRoot "internal" "i18n" "locales"
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

switch ($Target) {
    "build" {
        Write-Host "Building $BinaryName v$Version..."
        Update-LocaleVersions -Ver $Version
        go build -ldflags="$LdFlags" -o $BinaryName $CmdPath
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Build successful: $BinaryName (v$Version)"
        } else {
            Write-Host "Build failed" -ForegroundColor Red
            exit 1
        }
    }
    "test" {
        Write-Host "Running tests..."
        go test ./...
    }
    "clean" {
        Write-Host "Cleaning build artifacts..."
        if (Test-Path $BinaryName) {
            Remove-Item $BinaryName
            Write-Host "Removed $BinaryName"
        } else {
            Write-Host "Nothing to clean"
        }
    }
    "vet" {
        Write-Host "Running go vet..."
        go vet ./...
    }
}
