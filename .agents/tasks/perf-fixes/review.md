# Idle CPU and memory reductions for the Windows widget

Reduces idle redraw cost in ui-windows with five changes: a 100 ms named redraw interval that drops to once per second when nothing moves, fast redraw only for a ticker with items or a multi-frame GIF icon, a cached `*time.Location` per timezone, rounded-rect routines that skip interior pixels, and icons pre-scaled once and cached as `*image.RGBA`. The diff touches only the five requested areas. None of the out-of-scope items (buffer reuse, font or measureText caching, static layers, dirty rects, GIF memory, background cache bounds, GC tuning, pause when hidden) appear.

Watch for: icon output is within 2 levels per byte of the old output, not bit-identical (confirmed, stated in notes and bounded by a test). The clock loop itself has no test because it needs the Win32 widget (confirmed, disclosed in notes).

**Verdict**: APPROVED

## High-level view

The clock loop now sleeps for `animationFrameInterval` (100 ms) only while the last render drew something that moves, and otherwise sleeps to the next whole second. `Render` records whether a ticker with items or a multi-frame GIF was drawn and `redraw` wakes the loop through a buffered channel when that state flips, so data, settings and theme changes take effect at once. Switching to 5 fps is a one-constant change.

Timezone lookups go through a `sync.Map` cache that also stores errors. `format.go` and the draw call sites share it, and invalid names still fall back to UTC. GTK and macOS keep calling `time.LoadLocation`, and their behaviour is unchanged by the perf work.

`strokeRoundedRect` and `fillRoundedRect` now touch only border and corner pixels and share the original per-pixel arithmetic. A reference test compares them byte for byte with the old code across sizes, radii (including radius larger than half the size) and clipped offsets. Icons are converted to RGBA at load and scaled once per (source, width, height), then composited with `draw.Over`.

<details>
<summary>Issues (3)</summary>

1. **Idle loop untested (possible, low)** — `startClockLoop` and the wake path have no automated test. Run the widget once and confirm the ticker or settings toggle starts and stops fast redraw.
2. **Icon output not bit-identical (confirmed, low)** — Scaling is rounded to 8 bits before blending. A test bounds the difference to 2 levels per byte; confirm visually on one build.
3. **Notes ordering (confirmed, trivial)** — The Verification section says "see the end of this file" and then lists the full `go test ./...` result as its last line. Harmless, but could be tidied.

</details>

<details>
<summary>Details</summary>

### Animation gating and loop wake-up

`Render` clears `animSeen` on entry and publishes it to `animating` on exit via defer, including the early return for no cities. The flag is set in `drawEnhancedTicker` after the items and width checks, and in `weatherIcon` only for GIFs with more than one frame. Because the ticker position is derived from wall-clock time, every drawn ticker moves, so setting the flag whenever items exist is correct. `lastAnimating` is read and written under `redrawMu`, which `redraw` holds for its whole body. The wake channel is size 1 with a non-blocking send, and the loop drains the timer before resetting, so there is no lost or doubled wake. The old config-based check (`ShowTicker` or Original theme) is gone. Notes correctly report that several Original-theme icons are real GIFs and still animate, which corrects the investigation report.

One residual behaviour: while idle the loop wakes about 5 ms after each second boundary, so a state change that happens without a `redraw` (for example the sunset swap from a day GIF to a night icon) is picked up within 1 s. That is acceptable for the stated goal.

### Rounded rect pixel identity

Both functions reuse `roundedCornerDist`, a direct extraction of the original branch arithmetic, so per-pixel results come from the same float operations. The overlap case (corner squares meeting when the radius exceeds half the size) falls back to classifying the whole row. The reference test in `ref_rect_test.go` keeps the original implementations and covers that case. `drawSmoke` was not changed, which is fine since it was only "where relevant".

### Pre-scaled icon cache

The cache key is the source pointer plus final width and height, so DPI or scale changes produce new entries, and GIF frames are keyed per frame. It is mutex-guarded and cleared at 512 entries. The clear is a coarse bound, but a normal session stays far below it. Two goroutines can scale the same key concurrently and the last store wins, which only wastes work.

### Constraints and notes coverage

Nothing is committed. `git status` shows only modified and untracked files, and no tag, branch or stash changes appear in the diff. Earlier uncommitted work (forecast header, icon gap, reorder, locale files, ui-gtk and ui-darwin `headlineValues`) is still present and is not part of this review. The GTK and Darwin diffs belong to that earlier work, not to the perf change, and the perf change adds nothing to them. `simple_draw.go` is CRLF in the working tree and stays consistently CRLF (458 CR and 458 LF), and the other perf files are LF. Git only warns about CRLF normalisation.

`notes.md` covers all five changes, the benchmark, Task Manager measurement steps, optional pprof instructions (explicitly not committed), the switch to 5 fps (change to 200 ms and update `TestNextClockDelay`), what was not verified at runtime, and a Verification section recording `go build ./...`, `go vet`, and `go test ./...` passing after the last code change.

</details>

<details>
<summary>File map</summary>

Perf-related files: `internal/ui-windows/manager.go` (interval constant, clock loop, wake), `widget_draw.go` (animation flag, RGBA load, scaled icon cache, rounded rects), `animation.go` (GIF flag), `enhanced_draw.go` (ticker flag, cached location), `simple_draw.go` (cached location), `internal/weather/format.go` and new `location.go` (cache), new tests `perf_test.go`, `ref_rect_test.go`, `location_test.go`. Full diff: `git diff HEAD`.

</details>
