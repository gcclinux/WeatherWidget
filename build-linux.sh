#!/bin/bash
# build-linux.sh - Build WeatherWidget for the current system architecture

set -e

BINARY_NAME="weatherwidget"
CMD_PATH="./cmd/weatherwidget/"
LDFLAGS="-s -w"

# Detect Go binary
if [ -x "/usr/local/go/bin/go" ]; then
    GO_CMD="/usr/local/go/bin/go"
elif [ -x "/snap/bin/go" ]; then
    GO_CMD="/snap/bin/go"
else
    GO_CMD="/usr/bin/go"
fi

# Detect system architecture
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        GOARCH="amd64"
        ;;
    aarch64|arm64)
        GOARCH="arm64"
        ;;
    armv7l|armhf)
        GOARCH="arm"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

OUTPUT="${BINARY_NAME}-linux-${GOARCH}"

echo "Building for linux/${GOARCH} (detected: ${ARCH})..."
CGO_ENABLED=1 GOOS=linux GOARCH="$GOARCH" "$GO_CMD" build -v -ldflags="$LDFLAGS" -o "$OUTPUT" "$CMD_PATH"
echo "Done: $OUTPUT"
