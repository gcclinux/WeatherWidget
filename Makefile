# Weather Widget Build Configuration
# Target: Windows 11 (amd64)

BINARY_NAME=weatherwidget.exe
CMD_PATH=./cmd/weatherwidget/
LDFLAGS=-H windowsgui -s -w

export GOOS=windows
export GOARCH=amd64

.PHONY: build test clean vet

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_PATH)

test:
	go test ./...

clean:
	rm -f $(BINARY_NAME)

vet:
	go vet ./...
