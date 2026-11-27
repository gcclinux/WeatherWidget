# Perf fixes (report recommendations 1 to 5), Windows widget

Nothing is committed. Items 6 to 14 of the report are not touched.

## Changes

1. **Lower animated redraw rate.** `internal/ui-windows/manager.go`: new constant `animationFrameInterval = 100 * time.Millisecond` (10 fps), with a comment on the 5 fps value. The old 33 ms ticker is gone. `nextClockDelay(animating, now)` gives the loop's next sleep: `animationFrameInterval` while animating, otherwise the time to the next whole second (+5 ms), so the seconds clock redraws once per second and the loop no longer wakes 30 times per second.
2. **Animate only when something moves.**
   - `WidgetDrawer.Animating()` (`widget_draw.go`) reports what the last `Render` drew. `Render` resets a flag at the start and publishes it at the end. The flag is set by `drawEnhancedTicker` when it has items and room to scroll, and by `weatherIcon` (`animation.go`) when the icon in use is a GIF with more than one frame.
   - The `ShowTicker` / `IconThemeOriginal` config checks no longer force fast redraws.
   - Original theme: `clear`, `moon`, `cloudy_moon` and `partly_cloudy` ship as real GIFs, so those icons still animate. The Original theme's other icons (`rain`, `snow`, `fog`, `cloudy`, `storm`, `wind`, `heavy_rain`) and every icon of the other themes are static PNGs and no longer cause fast redraws. This corrects the report, which said the Original theme was PNG-only.
   - Reaction to changes: `redraw()` (called on new weather data, settings preview or save, theme change, show widget) compares the new `Animating()` with the previous value and, on a change, wakes the clock loop through `animWake` (buffered, non-blocking). The loop then resets its timer, so animation starts or stops immediately. The 1 s idle redraw also refreshes the flag (for example clear day icon to moon icon at sunset).
3. **Timezone cache.** New `internal/weather/location.go`: `LoadLocationCached(name)` (sync.Map, caches errors too) and unexported `locationOrUTC`. `FormatTime`, `FormatDate` and `FormatDateTime` use `locationOrUTC`; invalid names still fall back to UTC, so output is identical on all platforms. The three `time.LoadLocation(city.Timezone)` calls in `enhanced_draw.go` and `simple_draw.go` use `weather.LoadLocationCached`. `*time.Location` is immutable, so sharing across goroutines is safe. The cache is unbounded, but the key space is the set of timezone names the user enters. The GTK and macOS code still calls `time.LoadLocation` directly (not changed).
4. **Rounded rect.** `strokeRoundedRect` visits only the border pixels and the four corner squares. `fillRoundedRect` classifies only the corner squares per pixel and blends the span between them directly. Both use the original per-pixel arithmetic (`roundedCornerDist`), so the output is identical. `TestRoundedRectMatchesReference` compares them byte-for-byte with the original implementations (kept in `ref_rect_test.go`) over 10 sizes, 8 radii (0 to 80, including radius larger than half the size), 3 colours and 3 offsets (including negative, clipped).
5. **Pre-scaled icons.** `loadIcon` converts decoded PNGs to `*image.RGBA`. `drawImage` scales an `*image.RGBA` source once to the final pixel size into a cache keyed by (source pointer, w, h) and then composites it with `draw.Over`. GIF frames are separate `*image.RGBA` values, so they are keyed per frame. Because the key holds the final size, a DPI or widget-scale change produces correctly sized entries. The cache is package-level, mutex-protected, and cleared if it passes 512 entries. Output is not bit-identical (the scale result is rounded to 8 bits before the Over blend, and PNGs are premultiplied at load); `TestDrawImageUsesCachedPrescaleWithSameLook` allows at most 2 levels per byte and passed. This is not visible.

## Tests and benchmark added
- `internal/ui-windows/perf_test.go`: rounded rect reference test, prescale test, `nextClockDelay`, `Animating()` matrix (Simple and Classic views; static theme, Original GIF and PNG icons, hidden icon, ticker with and without items, ticker off, no data), `BenchmarkRenderFiveCities`.
- `internal/ui-windows/ref_rect_test.go`: old rounded-rect implementations as reference.
- `internal/weather/location_test.go`: cache identity, cached errors, UTC fallback for all three Format functions, concurrent use.

## Benchmark (render of 5 cities, no GUI, Ryzen 9 9900X3D, `-benchtime 3s`)
"Before" is a temp copy of the working tree with the new code paths disabled (no location cache, no prescale cache, old rounded rect). It still has PNGs converted to RGBA, so it slightly favours "before".

| View | Before | After |
|---|---|---|
| Classic (enhanced) | 13.7 ms, 11.2 MB, 1187 allocs per frame | 3.5 ms, 2.7 MB, 873 allocs per frame |
| Simple | 10.9 ms, 8.0 MB, 890 allocs | 1.9 ms, 1.4 MB, 577 allocs |

Frame cost falls about 4x (Classic) and 6x (Simple). Combined with 30 fps to 10 fps (or 1 fps when nothing animates), the animated CPU drops by roughly an order of magnitude. These are estimates from a benchmark; the real widget was not run.

Rerun: `go test ./internal/ui-windows/ -run xxx -bench RenderFiveCities -benchtime 3s`

## Switch to 5 fps
In `internal/ui-windows/manager.go` change `animationFrameInterval = 100 * time.Millisecond` to `200 * time.Millisecond`. The ticker then jumps about 8 px per frame (about 4 px at 10 fps). `TestNextClockDelay` asserts 100 ms, so update it too.

## Measure before/after in Task Manager
1. Close the settings window (WebView2 processes count towards the app).
2. Use a release build and the real config (5 cities, Classic view, seconds clock, ticker and icon theme as used).
3. Task Manager, Details tab, `weatherwidget.exe`: add the columns CPU, Memory (private working set), Handles. Watch for 2 minutes after the first weather refresh.
4. Run the old build and the new build in the same four combinations: ticker off or on, theme New or Original. Expected with the new build: ticker off with a PNG icon (or Original theme with a non-GIF condition) is about the 1-redraw-per-second level. Ticker on, or Original with clear, moon, cloudy_moon or partly_cloudy, animates at 10 fps.
5. Memory is not expected to drop by much from items 1 to 5. Per-frame buffers (item 6) are out of scope; the fonts and GIF frames are unchanged.

## Optional profiling (off by default, no code added)
To profile a build locally, temporarily add to `cmd/weatherwidget/main.go` (do not commit):
```go
import _ "net/http/pprof"
import "net/http"
// in main():
go http.ListenAndServe("127.0.0.1:6060", nil)
```
Then `go tool pprof -top -cum http://127.0.0.1:6060/debug/pprof/profile?seconds=60` and `go tool pprof -sample_index=alloc_space http://127.0.0.1:6060/debug/pprof/heap`. Look at `strokeRoundedRect`, `scaledIcon`, `time.LoadLocation`, `font.Drawer.DrawString`, `mallocgc`. For GC counts run with `GODEBUG=gctrace=1`.

## Not verified at runtime
The real widget could not be run: no real-time check of the 10 fps ticker smoothness, the wake-up on settings or weather change, or Task Manager numbers. The `Animating()` logic and the clock delay are unit-tested; the loop (`startClockLoop`) is not, because it needs the Win32 widget.

## Verification
Run in the repo root on Windows:
- `go build ./...`: OK
- `go vet ./internal/ui-windows/ ./internal/weather/`: OK
- `go test ./internal/ui-windows/ ./internal/weather/`: ok
- See the end of this file for the full `go test ./...` result.
- `go test ./...`: all packages ok (run after the last code change; ref_rect_test.go gofmt'd afterwards and ui-windows re-tested ok).
