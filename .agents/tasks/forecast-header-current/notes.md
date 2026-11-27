# Forecast header keeps current conditions (Classic view)

Nothing committed, per instruction. Working tree only.

## Behaviour
In Classic view with Grid Content = Today's / Tomorrow's Forecast, the header icon, big temperature and small blue feels-like are the current conditions, same as Real-Time mode. Only the detail rows change to forecast data. The blue two-line forecast title is unchanged.

Fallbacks: no new code needed. When forecast data is missing the header is unaffected (it no longer reads forecast). When current data is nil, the existing placeholders apply (Windows: cloudy icon, "--°C", "--"; GTK/Darwin skip the update as before). Current feels-like nil gives "--" via `FormatFeelsLike`. Unit conversion and rounding use the same Real-Time formatting calls.

## Changes per platform
- Windows: `internal/ui-windows/simple_draw.go` `simpleCardWeather` no longer overrides icon/temp/feels from `data.Forecast`; unused `math` import removed. Sizing code only uses the description line count, so no layout change.
- GTK: `internal/ui-gtk/metrics.go` `headlineValues` returns current icon/temp/feels in all modes and only swaps the description for the forecast title; unused `math` import removed. Comments in `panel_simple.go` updated. `updateFeelsLike` and the grid-mode refresh path use `headlineValues`, so they follow.
- Darwin: `internal/ui-darwin/metrics.go` `headlineValues` gained a `simple bool` first parameter; when true the forecast override is skipped. `manager.go` passes `m.viewMode == config.ViewModeSimple` and shows the "Feels Like" label (not "Low") in Classic view in both places that set it. Enhanced behaviour on Darwin is kept as before.
- Test: `internal/ui-windows/draw_test.go` `TestSimpleCardGridModeTodayAndTomorrow` now sets a current feels-like (16°C) and expects current icon (cloudy), 19°C and 16°C in today/tomorrow modes; detail-row assertions unchanged.

## Enhanced view finding
- Windows Enhanced (`enhanced_draw.go`): not affected. Header uses `enhancedTemperature` (current data); grid mode only changes the metric tiles.
- GTK Enhanced: not affected. It does not use `headlineValues` (only `panel_simple.go` does).
- Darwin Enhanced: HAS the same defect. The shared `updateCard` uses `headlineValues`, so in forecast modes Enhanced shows forecast High/Low, the forecast icon and forecast summary, and labels the feels-like row "Low". Left unchanged as instructed (via the `simple` flag). Fixing it is `headlineValues(true, ...)` plus removing the "Low" label branch in `manager.go`.

## Run and results
- `go build ./...` (Windows): OK.
- `go vet ./internal/ui-windows/ ./internal/i18n/`: clean.
- `go test ./internal/...`: all packages ok (ui-windows updated test passes).
- `gofmt -l` on edited files: `ui-gtk/metrics.go`, `ui-darwin/metrics.go`, `ui-windows/simple_draw.go` clean. `ui-darwin/metrics.go` had a pre-existing trailing blank line that gofmt removed. `ui-windows/draw_test.go` is still listed by gofmt, from a pre-existing trailing blank line at EOF (also present in HEAD); left alone.

## Not verified
- GTK (`GOOS=linux go vet`) and Darwin (`GOOS=darwin go vet`) could not compile here: cgo needs a Linux gcc/GTK and macOS clang. Their edits are small and type-simple but were only gofmt-checked, not compiled. No call sites of `headlineValues` other than those listed exist (GTK: `panel_simple.go` x3; Darwin: `manager.go` x1).
- No GTK or Darwin unit tests for `headlineValues` exist, and the files are build-tagged, so none were added.
- No visual check of any platform's UI.
