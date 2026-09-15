#!/usr/bin/env bash
#
# generate-sources.sh — Generate the offline Go module sources for the Flatpak.
#
# Flathub builds run in a sandbox with NO network access. Go normally downloads
# modules during the build, so we must declare every dependency ahead of time.
# This script runs dennwc/flatpak-go-mod against the project to produce:
#
#   flatpak/go.mod.yml    -> Flatpak `sources` directives (referenced by the
#                            manifest) that download+verify each module and
#                            populate ./vendor during the build.
#   flatpak/modules.txt   -> Go's vendor manifest, dropped into ./vendor so the
#                            `-mod=vendor` build resolves correctly.
#
# Re-run this whenever go.mod / go.sum change (new deps or version bumps).
#
# Requirements: a working Go toolchain (>= the version in go.mod) and network
# access AT GENERATION TIME (this step runs on your machine, not in the sandbox).
#
# Usage:
#   ./flatpak/generate-sources.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Detect Go.
if command -v go >/dev/null 2>&1; then
    GO_CMD="go"
elif [ -x "/usr/local/go/bin/go" ]; then
    GO_CMD="/usr/local/go/bin/go"
else
    echo "ERROR: Go toolchain not found. Install Go >= $(grep '^go ' "$PROJECT_ROOT/go.mod" | awk '{print $2}')." >&2
    exit 1
fi

echo "==> Using Go: $($GO_CMD version)"

# This project uses a Go workspace (go.work). flatpak-go-mod must see the app
# module in isolation, so disable the workspace for this invocation.
export GOWORK=off

echo "==> Running dennwc/flatpak-go-mod against the project root..."
cd "$PROJECT_ROOT"

# Generate YAML directives + modules.txt. The tool writes them into CWD.
$GO_CMD run github.com/dennwc/flatpak-go-mod@latest .

# Move the generated files into the flatpak/ directory next to the manifest.
if [ -f "$PROJECT_ROOT/go.mod.yml" ]; then
    mv -f "$PROJECT_ROOT/go.mod.yml" "$SCRIPT_DIR/go.mod.yml"
else
    echo "ERROR: expected go.mod.yml was not generated." >&2
    exit 1
fi

if [ -f "$PROJECT_ROOT/modules.txt" ]; then
    mv -f "$PROJECT_ROOT/modules.txt" "$SCRIPT_DIR/modules.txt"
else
    echo "ERROR: expected modules.txt was not generated." >&2
    exit 1
fi

echo ""
echo "==> Done. Generated:"
echo "    $SCRIPT_DIR/go.mod.yml"
echo "    $SCRIPT_DIR/modules.txt"
echo ""
echo "    Commit both files. The manifest references go.mod.yml under the"
echo "    weatherwidget module's sources; modules.txt is pulled into ./vendor"
echo "    automatically by a directive inside go.mod.yml."
