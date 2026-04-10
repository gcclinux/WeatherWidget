# Weather Widget Build Configuration
BINARY_NAME=weatherwidget
CMD_PATH=./cmd/weatherwidget/
GO_CMD=/usr/local/go/bin/go

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

.PHONY: build test clean vet

build:
	GOOS=$(GOOS_VAL) $(GO_CMD) build -v -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_PATH)

test:
	$(GO_CMD) test ./...

clean:
	rm -f $(BINARY_NAME)

vet:
	$(GO_CMD) vet ./...
