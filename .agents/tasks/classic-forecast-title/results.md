# Classic forecast title - results

Change (uncommitted): in Simple (Classic) view, Today's / Tomorrow's Forecast grid modes show a localized two-line title instead of the forecast summary. Live mode and Enhanced view unchanged.

- New `i18n.ForecastTitle(lm, mode)` (internal/i18n/forecast_title.go): reuses existing keys `settings.gridMode.today` / `settings.gridMode.tomorrow` (present in all 12 locales, so no locale file changes), splits at the first space ("Today's\nForecast"), single-word titles (ja-JP, zh-CN) stay one line, English fallback if no locale manager. Same font size as the old description.
- Windows `simple_draw.go`: `simpleCardWeather` uses the title instead of the summary; `wrapDescription` keeps explicit newlines. `draw_test.go` expectations updated.
- GTK: `headlineValues` (simple-panel only) returns the title; `descLbl` justified centre.
- Darwin: `manager.go` replaces descStr with the title only when viewMode is Simple; `panel.m` sets `_descLbl.maximumNumberOfLines` to 2 when the text contains a newline, else 1 (Enhanced unaffected).

Ran:
- `go test ./internal/...`: all pass (incl. new TestForecastTitle, i18n locale tests, ui-windows tests after updating draw_test.go).
- gofmt: my edited/new files are clean, except pre-existing CRLF files (e.g. draw_test.go) that gofmt already flagged before this change.
- `go vet` for GOOS=linux/darwin ui packages and `go build ./...` could not run (cgo: no usable C compiler/clang here).

Not verified at runtime: GTK and macOS rendering (two-line label, centre alignment, no card widening on Linux, NSTextField 2-line display), and the Windows visual output beyond the unit tests.
