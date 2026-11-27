# Classic-view metrics: coder notes (uncommitted)

Change: in Classic/Simple view with Grid Content "live", Sunrise, Sunset, Cloud Coverage, Visibility and Precipitation Rate are shown as extra metric lines (after the classic six, in that order) when selected. Forecast grid modes and Enhanced view are unchanged. Nothing changes when none of the five is selected.

- ui-gtk/panel_simple.go: five new centered `simple-info-label` labels after UV Index; filled in live mode with weather.*Display helpers; visibility follows Show* flags; hidden in forecast modes. The vertical box grows automatically.
- ui-windows/simple_draw.go: simpleMetrics live branch appends the five metrics; simpleCardMetricCount counts them so card height (and widths via measured text) include the extra rows.
- ui-darwin: metrics.go `liveMetrics` takes a limit; `cardMetrics` returns up to 11 lines for simple+live, and in simple forecast modes still returns 11 slots so lines 6..10 are hidden (enhanced keeps 6 tiles); manager.go uses it in both metric-push loops; panel.m has 11 simple lines (indices 6..10 are line-only, hidden by default, no tile). Window height uses fittingSize so it grows.

## Review round 1 fixes
- Stale extra lines on macOS: `cardMetrics` now returns 11 slots for every simple-view grid mode, so leaving live mode hides lines 6..10.
- gofmt churn reverted: darwin manager.go now differs from HEAD only in the two metric loops; metrics.go keeps HEAD's trailing blank line; panel_simple.go field alignment restored. These three files were already not gofmt-clean at HEAD, so `gofmt -l` still lists them; that is pre-existing. (An IDE format-on-save reformatted the files after editor edits; the final versions were written from a script.)
- Windows full test panic: `go test ./internal/ui-windows` now passes (1 run plus `-count=3`); the earlier panic in scheduler/weather.FetchAll did not reproduce, so it looks environment/flaky and unrelated. Not re-run on the base commit.

## Verification (this round)
- Windows (native): `go build ./internal/ui-windows` OK, `go vet ./internal/ui-windows` OK, `go test ./internal/ui-windows` PASS, `-count=3` PASS.
- `go test ./internal/weather ./internal/config` PASS.
- `gofmt -e` syntax check OK on all four changed Go files; simple_draw.go is gofmt-clean.
- GTK/darwin: cannot compile here. `GOOS=linux go vet ./internal/ui-gtk` fails in runtime/cgo (clearenv with Windows gcc); `GOOS=darwin go vet ./internal/ui-darwin` fails (clang not found).
- Not verified at runtime: GTK rendering and sizing on Linux, macOS panel.m (Objective-C not compiled at all) and window sizing, Windows visual output. Needs a manual check on Linux and macOS.
