# Weather Widget Build Configuration
BINARY_NAME=weatherwidget
CMD_PATH=./cmd/weatherwidget/
ifeq ($(shell test -x /usr/local/go/bin/go && echo yes),yes)
    GO_CMD=/usr/local/go/bin/go
else
    GO_CMD=/usr/bin/go
endif

# Detect OS
ifeq ($(OS),Windows_NT)
    BINARY_NAME=weatherwidget.exe
    LDFLAGS=-H windowsgui -s -w
    GOOS_VAL=windows
else
    # Linux/other
    LDFLAGS=-s -w
    GOOS_VAL=$($(GO_CMD) env GOOS)
endif

# Detect host OS for build target selection
UNAME_S := $(shell uname -s)

.PHONY: build build-linux build-darwin test clean vet

ifeq ($(UNAME_S),Darwin)
build: build-darwin
else
build: build-linux
endif

build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO_CMD) build -v -ldflags="-s -w" -o $(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(GO_CMD) build -v -ldflags="-s -w" -o $(BINARY_NAME)-linux-arm64 $(CMD_PATH)

build-darwin:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO_CMD) build -v -ldflags="-s -w" -o $(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO_CMD) build -v -ldflags="-s -w" -o $(BINARY_NAME)-darwin-arm64 $(CMD_PATH)

test:
	$(GO_CMD) test ./...

clean:
	rm -f $(BINARY_NAME)-linux-amd64 $(BINARY_NAME)-linux-arm64 $(BINARY_NAME)-darwin-amd64 $(BINARY_NAME)-darwin-arm64

vet:
	$(GO_CMD) vet ./...
