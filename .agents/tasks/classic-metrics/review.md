# Classic-view weather metrics on GTK, macOS and Windows

Classic/Simple view now renders Sunrise, Sunset, Cloud Coverage, Visibility and Precipitation Rate as extra lines after the classic six when their Show* flags are set. Each platform reuses its existing display helpers. GTK adds five labels, Windows extends the simple metric list and card height count, and macOS extends the simple lines from 6 to 11. Behaviour with none of the five selected is unchanged, and Enhanced view paths are untouched (macOS keeps the six-tile cap).

Watch for: the macOS Objective-C (`panel.m`) was never compiled and the GTK and macOS changes were never run (confirmed, per coder notes). Unrelated uncommitted edits to `internal/ui-gtk/settings.go` and `TODO.md` from the earlier cities-list request are in the tree (confirmed).

**Verdict**: APPROVED

## High-level view

All three platforms show the five metrics only in live grid mode, in the fixed order Sunrise, Sunset, Clouds, Visibility, Precip rate, and hide them in forecast grid modes. This is consistent across platforms and matches the Enhanced tile ordering.

GTK and Windows are small, local additions. macOS needed a metrics-limit parameter and a larger line array in the native panel. The macOS change is the riskiest because it is unverified Objective-C.

<details>
<summary>Issues (3)</summary>

1. **macOS Objective-C unverified (likely OK)** - `panel.m` was not compiled or run; build and visually check Classic view on macOS with and without the five selected, including switching live to forecast and back.
2. **No automated test for the new Classic paths (confirmed)** - no tests were added for `simpleMetrics`, `cardMetrics` or GTK `refreshMetrics`; consider a small Windows/darwin table test for ordering and omission.
3. **Unrelated uncommitted changes in tree (confirmed)** - `settings.go` and `TODO.md` diffs belong to the prior cities-list request; keep them out of this change when committing.

</details>

<details>
<summary>Details</summary>

### macOS simple lines versus tiles

`cardMetrics` returns up to 11 lines for simple+live and always reports 11 slots for every simple grid mode. So lines 6..10 get hidden when leaving live mode, and the earlier stale-line risk is handled. Enhanced still uses `visibleMetrics` with six slots. In `panel.m`, indices 6..10 have no tile, and `setMetricTileAt` uses a nil tile (messages to nil are no-ops under Objective-C) and skips tile width recompute. The extra lines start hidden. Placeholder names are English until Go pushes localized ones, the same as the existing six. Confidence that this is correct: likely. It cannot be confirmed without a macOS build.

### GTK and Windows

GTK labels use the existing `simple-info-label` class and are hidden in forecast modes by the early-return branch. Windows counts the five in `simpleCardMetricCount` so the card grows. Windows build, vet and tests passed per the coder notes. GTK compile is unverified here (cgo toolchain), so a Linux build is needed (likely OK, the code mirrors adjacent label patterns).

### Scope

The darwin `manager.go` diff is limited to the two metric loops. The gofmt state of three files was already unclean at HEAD, and the coder avoided reformatting churn. Nothing is committed: HEAD is 8795919 and no new branches exist. Commit 623f40f is not on any branch.

</details>

<details>
<summary>File map</summary>

- internal/ui-darwin/manager.go: both metric loops use `cardMetrics`
- internal/ui-darwin/metrics.go: `liveMetrics` limit, `cardMetrics`, 11-line constant
- internal/ui-darwin/panel.m: 11 simple lines, extras line-only and hidden by default
- internal/ui-gtk/panel_simple.go: five labels, live fill, visibility, hide in forecast
- internal/ui-windows/simple_draw.go: five metrics in the list and count
- Out of scope, from earlier request: internal/ui-gtk/settings.go, TODO.md

Full diff: `git diff HEAD`

</details>
