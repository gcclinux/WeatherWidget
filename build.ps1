# Weather Widget Build Script for Windows
# Usage: .\build.ps1 [build|test|clean|vet]

param(
    [Parameter(Position=0)]
    [ValidateSet("build", "test", "clean", "vet")]
    [string]$Target = "build"
)

$BinaryName = "weatherwidget.exe"
$CmdPath = "./cmd/weatherwidget/"
$LdFlags = "-H windowsgui -s -w"

$env:GOOS = "windows"
$env:GOARCH = "amd64"

switch ($Target) {
    "build" {
        Write-Host "Building $BinaryName..."
        go build -ldflags="$LdFlags" -o $BinaryName $CmdPath
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Build successful: $BinaryName"
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
