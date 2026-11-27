# Windows city re-order

## Change (uncommitted)
- internal/ui-windows/web/app.js: moveCity() helper; each Saved Cities row gets up/down buttons left of Remove (first row: down only, last row: up only, single city: none). Moves edit pending currentConfig.cities only, so Cancel discards and Save (collectConfigFromUI -> Go save) persists the order, same as add/remove.
- web/style.css: .city-item-actions, .btn-move-city (muted, matches Remove button sizing).
- Locale keys settings.locations.moveUp / moveDown added to all 12 locale JSON files (alphabetical position) and to TestEnGBCompleteness. Used as tooltip and aria-label ("<text>: <city>"). Glyphs are U+25B2/U+25BC.
- Live preview: the preview only fires for Appearance/Display/Ticker tabs; city add/remove/move never preview (unchanged). A later preview payload carries the pending order; harmless.
- GTK/darwin reference: in-place slice swap of the city list, saved with the config. Windows does the same.

## Run
- node --check app.js: ok. moveCity checked in node: edge moves return false, valid moves swap.
- go build ./internal/...: ok. go vet ui-windows, i18n: clean.
- go test ui-windows, i18n, config: pass. One earlier ui-windows run hit a nil-pointer panic in scheduler/weather FetchAll (background goroutine, no Go code changed here); next runs with -count=1 passed. Looks like a pre-existing flaky test.
- gofmt -l lists many pre-existing files (none I edited).
- No JS test harness exists in the repo, so no automated reorder test was added.

## Not verified
- WebView2 UI not run: rendering, button click behaviour, and widget showing the new order after Save are unverified at runtime.
