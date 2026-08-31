# Weather Widget Build Configuration
BINARY_NAME=weatherwidget
APP_NAME=WeatherWidget
BUNDLE_ID=com.weatherwidget
CMD_PATH=./cmd/weatherwidget/
APP_ICON=assets/icons/day/clear_day.png
VERSION ?= dev

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

.PHONY: build build-linux build-snap build-darwin build-darwin-app build-darwin-dmg build-darwin-pkg test clean vet

ifeq ($(UNAME_S),Darwin)
build: build-darwin
else
build: build-linux
endif

build-snap:
	./scripts/build-snap.sh

build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO_CMD) build -v -ldflags="-s -w" -o $(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(GO_CMD) build -v -ldflags="-s -w" -o $(BINARY_NAME)-linux-arm64 $(CMD_PATH)

build-darwin:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO_CMD) build -v -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO_CMD) build -v -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY_NAME)-darwin-arm64 $(CMD_PATH)

# Build a universal macOS .app bundle (Intel + Apple Silicon)
build-darwin-app: build-darwin
	@echo "==> Assembling $(APP_NAME)-$(VERSION).app bundle..."

	# -- Create .app directory structure --
	@rm -rf $(APP_NAME)-$(VERSION).app
	@mkdir -p $(APP_NAME)-$(VERSION).app/Contents/MacOS
	@mkdir -p $(APP_NAME)-$(VERSION).app/Contents/Resources

	# -- Universal binary via lipo --
	@lipo -create -output $(APP_NAME)-$(VERSION).app/Contents/MacOS/$(BINARY_NAME) \
		$(BINARY_NAME)-darwin-amd64 \
		$(BINARY_NAME)-darwin-arm64
	@chmod +x $(APP_NAME)-$(VERSION).app/Contents/MacOS/$(BINARY_NAME)

	# -- Build .icns from the 512x512 source PNG --
	@mkdir -p /tmp/$(APP_NAME).iconset
	@sips -z 16 16     $(APP_ICON) --out /tmp/$(APP_NAME).iconset/icon_16x16.png     > /dev/null
	@sips -z 32 32     $(APP_ICON) --out /tmp/$(APP_NAME).iconset/icon_16x16@2x.png  > /dev/null
	@sips -z 32 32     $(APP_ICON) --out /tmp/$(APP_NAME).iconset/icon_32x32.png     > /dev/null
	@sips -z 64 64     $(APP_ICON) --out /tmp/$(APP_NAME).iconset/icon_32x32@2x.png  > /dev/null
	@sips -z 128 128   $(APP_ICON) --out /tmp/$(APP_NAME).iconset/icon_128x128.png   > /dev/null
	@sips -z 256 256   $(APP_ICON) --out /tmp/$(APP_NAME).iconset/icon_128x128@2x.png > /dev/null
	@sips -z 256 256   $(APP_ICON) --out /tmp/$(APP_NAME).iconset/icon_256x256.png   > /dev/null
	@sips -z 512 512   $(APP_ICON) --out /tmp/$(APP_NAME).iconset/icon_256x256@2x.png > /dev/null
	@cp                $(APP_ICON)    /tmp/$(APP_NAME).iconset/icon_512x512.png
	@sips -z 512 512   $(APP_ICON) --out /tmp/$(APP_NAME).iconset/icon_512x512@2x.png > /dev/null
	@iconutil -c icns /tmp/$(APP_NAME).iconset -o $(APP_NAME)-$(VERSION).app/Contents/Resources/$(APP_NAME).icns
	@rm -rf /tmp/$(APP_NAME).iconset

	# -- Write Info.plist --
	@printf '<?xml version="1.0" encoding="UTF-8"?>\n\
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n\
<plist version="1.0">\n\
<dict>\n\
    <key>CFBundleName</key>\n\
    <string>$(APP_NAME)</string>\n\
    <key>CFBundleDisplayName</key>\n\
    <string>$(APP_NAME)</string>\n\
    <key>CFBundleIdentifier</key>\n\
    <string>$(BUNDLE_ID)</string>\n\
    <key>CFBundleVersion</key>\n\
    <string>$(VERSION)</string>\n\
    <key>CFBundleShortVersionString</key>\n\
    <string>$(VERSION)</string>\n\
    <key>CFBundleExecutable</key>\n\
    <string>$(BINARY_NAME)</string>\n\
    <key>CFBundleIconFile</key>\n\
    <string>$(APP_NAME)</string>\n\
    <key>CFBundlePackageType</key>\n\
    <string>APPL</string>\n\
    <key>CFBundleSignature</key>\n\
    <string>????</string>\n\
    <key>NSHighResolutionCapable</key>\n\
    <true/>\n\
    <key>NSSupportsAutomaticGraphicsSwitching</key>\n\
    <true/>\n\
    <key>LSMinimumSystemVersion</key>\n\
    <string>11.0</string>\n\
    <key>LSUIElement</key>\n\
    <false/>\n\
</dict>\n\
</plist>\n' > $(APP_NAME)-$(VERSION).app/Contents/Info.plist

	@echo "==> Done: $(APP_NAME)-$(VERSION).app"

# Wrap the .app in a distributable .dmg (requires build-darwin-app first)
build-darwin-dmg: build-darwin-app
	@echo "==> Creating $(APP_NAME)-$(VERSION).dmg..."
	@rm -f $(APP_NAME)-$(VERSION).dmg $(APP_NAME)-$(VERSION)-tmp.dmg
	# Size the writable image from the bundle footprint + 50MB slack so hdiutil's
	# auto-sizer can't under-allocate (avoids "No space left on device").
	@SIZE=$$(du -sm $(APP_NAME)-$(VERSION).app | awk '{print $$1 + 50}'); \
	hdiutil create -volname "$(APP_NAME) $(VERSION)" \
		-srcfolder $(APP_NAME)-$(VERSION).app \
		-fs HFS+ -format UDRW -size $${SIZE}m \
		-ov $(APP_NAME)-$(VERSION)-tmp.dmg
	@hdiutil convert $(APP_NAME)-$(VERSION)-tmp.dmg \
		-format UDZO \
		-o $(APP_NAME)-$(VERSION).dmg
	@rm -f $(APP_NAME)-$(VERSION)-tmp.dmg
	@echo "==> Done: $(APP_NAME)-$(VERSION).dmg"

# Build a macOS .pkg installer (requires build-darwin-app first)
# Usage (unsigned):  make build-darwin-pkg VERSION=1.2.0
# Usage (signed):    make build-darwin-pkg VERSION=1.2.0 \
#                        SIGN_APP="Developer ID Application: You (TEAMID)" \
#                        SIGN_PKG="Developer ID Installer: You (TEAMID)"
# Usage (notarized): add NOTARIZE=1 APPLE_ID=you@x.com TEAM_ID=ABC APP_PWD=xxxx
SIGN_APP  ?=
SIGN_PKG  ?=
NOTARIZE  ?=
APPLE_ID  ?=
TEAM_ID   ?=
APP_PWD   ?=

build-darwin-pkg: build-darwin-app
	@echo "==> Building $(APP_NAME)-$(VERSION).pkg installer..."
	@PKG_ARGS="--version $(VERSION)"; \
	if [ -n "$(SIGN_APP)" ] && [ -n "$(SIGN_PKG)" ]; then \
		PKG_ARGS="$$PKG_ARGS --sign-app \"$(SIGN_APP)\" --sign-pkg \"$(SIGN_PKG)\""; \
	else \
		PKG_ARGS="$$PKG_ARGS --skip-sign"; \
	fi; \
	if [ "$(NOTARIZE)" = "1" ]; then \
		PKG_ARGS="$$PKG_ARGS --notarize --apple-id $(APPLE_ID) --team-id $(TEAM_ID) --app-password $(APP_PWD)"; \
	fi; \
	eval bash installer/build-pkg.sh $$PKG_ARGS
	@echo "==> Done: build/$(APP_NAME)-$(VERSION).pkg"

test:
	$(GO_CMD) test ./...

clean:
	rm -f $(BINARY_NAME)-linux-amd64 $(BINARY_NAME)-linux-arm64 $(BINARY_NAME)-darwin-amd64 $(BINARY_NAME)-darwin-arm64
	rm -rf $(APP_NAME)-*.app $(APP_NAME)-*.dmg build/

vet:
	$(GO_CMD) vet ./...
