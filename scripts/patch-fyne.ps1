# scripts/patch-fyne.ps1
# Patches Fyne v2 to enable desktop OpenGL 2.1 on Windows ARM64 (fixing upstream Fyne issue #6483).
# Windows ARM64 is a desktop environment using WGL desktop OpenGL, but Fyne's upstream build tags
# incorrectly route all non-Darwin arm64 targets to mobile GLES (requiring WGL_ARB_create_context_es2_profile).

$ErrorActionPreference = "Stop"

$fyneDir = (go list -m -f "{{.Dir}}" fyne.io/fyne/v2 2>$null)
if (-not $fyneDir -or -not (Test-Path $fyneDir)) {
    Write-Host "Fyne module directory not found. Run 'go mod download' first." -ForegroundColor Yellow
    exit 0
}

# 1. Driver GLFW
$glfwCore = Join-Path $fyneDir "internal\driver\glfw\glfw_core.go"
$glfwEs = Join-Path $fyneDir "internal\driver\glfw\glfw_es.go"

# 2. Painter GL
$glCore = Join-Path $fyneDir "internal\painter\gl\gl_core.go"
$glEs = Join-Path $fyneDir "internal\painter\gl\gl_es.go"

# 3. Shaders
$shaders = Join-Path $fyneDir "internal\painter\gl\shaders.go"
$shadersEs = Join-Path $fyneDir "internal\painter\gl\shaders_es.go"

function Patch-File {
    param([string]$Path, [string]$Pattern, [string]$Replacement)
    if (-not (Test-Path $Path)) { return }
    $content = Get-Content $Path -Raw
    if ($content -match $Pattern) {
        Set-ItemProperty -Path $Path -Name IsReadOnly -Value $false
        $updated = $content -replace $Pattern, $Replacement
        Set-Content -Path $Path -Value $updated -NoNewline
        Set-ItemProperty -Path $Path -Name IsReadOnly -Value $true
        Write-Host "  Patched: $(Split-Path $Path -Leaf)" -ForegroundColor Green
    }
}

Patch-File $glfwCore '//go:build \(\(!gles && !arm && !arm64\) \|\| darwin\) && !wasm && !test_web_driver' '//go:build ((!gles && !arm && !arm64) || darwin || windows) && !wasm && !test_web_driver'
Patch-File $glfwEs '//go:build \(gles \|\| arm \|\| arm64\) && !darwin && !wasm && !test_web_driver' '//go:build (gles || arm || arm64) && !darwin && !windows && !wasm && !test_web_driver'

Patch-File $glCore 'darwin && !mobile && !ios && !wasm && !test_web_driver\)' '(darwin || windows) && !mobile && !ios && !wasm && !test_web_driver)'
Patch-File $glEs '!darwin && !wasm && !test_web_driver' '!darwin && !windows && !wasm && !test_web_driver'

Patch-File $shaders 'darwin && !mobile && !ios && !wasm && !test_web_driver\)' '(darwin || windows) && !mobile && !ios && !wasm && !test_web_driver)'
Patch-File $shadersEs '!darwin && !wasm && !test_web_driver' '!darwin && !windows && !wasm && !test_web_driver'
