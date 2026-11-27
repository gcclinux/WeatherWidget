# Review: forecast header keeps current conditions

Verdict: APPROVED. No blockers.

- Windows simple_draw.go: simpleCardWeather no longer overrides icon/temp/feels from Forecast; title kept. Test updated to expect current values.
- GTK metrics.go: headlineValues returns current icon/temp/feels in all modes; only desc becomes title. All 3 call sites in panel_simple.go (update, updateFeelsLike, applyDisplayFields) follow it. Detail rows (refreshMetrics) unchanged.
- Darwin: simple flag skips forecast override; Classic keeps the Feels Like label in both places. Enhanced Darwin unchanged and reported as having the same defect in notes.
- Fallbacks: nil current feels -> '--' via FormatFeelsLike; forecast absent no longer affects header.
- No commits (HEAD b94a859 predates task), no stash. Earlier uncommitted work (ForecastTitle, blue class, settings, reorder) not disturbed.
- Verification: notes record go build, vet, go test ./internal/... passing. GTK/Darwin not compilable on host (cgo); edits are small and type-simple, and call sites were checked in this review.
