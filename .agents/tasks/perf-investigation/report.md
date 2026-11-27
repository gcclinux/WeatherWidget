# Performance investigation: idle CPU (about 10%) and memory (73 MB), Windows widget

Read-only. Nothing was run or profiled. All numbers below are estimates from reading the code, and each needs the measurements in section 5 before any change.

## 1. Summary answer

Yes, both can be reduced a lot, and the CPU is the easy win.

The widget redraws and re-uploads the whole window up to 30 times per second whenever the ticker or the "Original" animated icon theme is enabled. That matches the user's setup ("optional tickers/animations"). Each redraw is a full CPU software render with no caching of the finished frame:

- A new `image.RGBA` is allocated for all 5 cities.
- Text is laid out and measured many times.
- The icons are bilinear-rescaled from their source size.
- `time.LoadLocation` is called repeatedly, and it parses embedded tzdata every time because Go does not cache it.
- The result is converted to BGRA, and possibly rescaled again for DPI, before `UpdateLayeredWindow`.

If neither the ticker nor the Original theme is on, the redraw rate drops to 1 per second, and 10% CPU would not be expected. So the animation setting is the dominant factor.

The 73 MB is probably mostly Go heap churn from the per-frame allocations, plus about 7 MB of parsed fonts and any GIF frames decoded into RGBA. The GC lets the heap grow to about 2x live data. Cutting per-frame allocation cuts the memory too.

Top recommendations, ranked below in section 4:

1. Lower the animation redraw rate from 30 fps to about 10 to 15 fps, or redraw only when something visibly changed.
2. Cache the time zone `*time.Location` per city, and pre-scale icons once.
3. Reuse the render buffers instead of allocating each frame.
4. Cache static card content in a layer, so only the dynamic parts (clock, ticker, animated icon) are redrawn.

## 2. What runs periodically or continuously

| What | Where | Frequency | Notes |
|---|---|---|---|
| Clock/animation loop | `internal/ui-windows/manager.go` `startClockLoop` (line 306), started by `go m.startClockLoop()` at line 157 | `time.NewTicker(33ms)`, about 30 Hz wake-ups | Always wakes 30 times per second, even when nothing needs drawing. Calls `m.redraw()` when the widget is visible and either `animated` or the Unix second changed. |
| `animated` condition | `manager.go:318` | n/a | `animated = cfg.GetPollutionFields().ShowTicker \|\| IconTheme == IconThemeOriginal`. If true, the widget does a full `redraw()` on every 33 ms tick (about 30 fps). If false, 1 redraw per second (seconds clock). |
| Weather refresh | `internal/scheduler/scheduler.go:66` `time.NewTicker(r.interval)` plus a goroutine at line 70 | `RefreshInterval` minutes, network I/O | Not a CPU concern. |
| Power resume listener | `manager.go:180` | event driven | Negligible. |
| Drag debounce | `manager.go:347` `time.AfterFunc(400ms)` | only on drag | Negligible. |
| Win32 message pump | `manager.go:213` `GetMessageW` | blocking | Fine. No busy loop. |
| Settings WebView2 | `settings.go:183-215` `OpenSettings` | only while the settings window is open | The `msedgewebview2` processes exist only while it is open. If Task Manager groups them under the app, close the settings window before measuring. |
| Tray | `tray.go` | event driven | Negligible. |

The ticker scroll is time-derived. `drawEnhancedTicker` (`enhanced_draw.go:617-665`) computes the x offset from `now.UnixMilli()` at 25 ms per pixel, so it moves 1 px per 25 ms. The scroll therefore has at most 40 distinct positions per second. Redrawing at 33 ms is slightly finer than the scroll needs. The animated GIF icon (`animation.go` `weatherIcon`) also picks its frame from `now`, with frame delays of 20 ms or more, usually 100 ms.

`startClockLoop` does not stop when the widget is hidden. It only skips the redraw, so the 30 Hz wake-up remains. This is small, about 0.1% CPU.

## 3. Where the CPU goes in one frame

Each frame is `Manager.redraw` (`manager.go:258-303`), followed by `Render`, then `UpdateSurfaceScaled`. All of it is CPU software rendering on one goroutine. There is no GDI+ or Direct2D.

### 3.1 Findings by cost

**A. A full re-render every frame, with a fresh allocation.**
- `renderEnhancedView` allocates `dst := image.NewRGBA(...)` (`enhanced_draw.go:83`). `renderSimpleView` does the same at `simple_draw.go:333`.
- At about 600 px wide and 5 stacked cards (Classic view), that is around 0.6 to 3 MB per frame. At 30 fps that is roughly 20 to 100 MB/s of garbage, which drives GC and the resident-memory fluctuation.
- `redraw()` also copies `cities` and the `results` slice every frame (`manager.go:271-285`). These are small.

**B. Time zone database parsing each frame.**
- `time.LoadLocation(city.Timezone)` is called per card per frame at `enhanced_draw.go:221`, `simple_draw.go:350` and `simple_draw.go:380`.
- It is also called inside `weather.FormatTime` and `weather.FormatDate` (`internal/weather/format.go:88` and `:102`). Those are called both in the width pass (`enhancedInfoWidth`, `enhanced_draw.go:125-128`, `simple_draw.go:313`) and in the draw pass (`enhanced_draw.go:257` and `:260`).
- Go does not cache `LoadLocation`. On Windows with no system zoneinfo it falls back to the embedded `time/tzdata` (`cmd/weatherwidget/main.go:15`, `internal/weather/format.go:8`). Every call decompresses and parses a zone from that embedded zip.
- Estimated at about 4 to 6 calls per city per frame, so 20 to 30 calls per frame, and 600 to 900 per second at 30 fps. It also allocates a `*Location` with transition tables each time. **I rank this as a top CPU and allocation cost, but it is unmeasured.**

**C. Icon scaling every frame.**
- `drawImage` (`widget_draw.go:219-225`) calls `xdraw.BiLinear.Scale` from the source image to the target rect (84x84 in Classic, 64x64 in Simple) each frame. The icon sources are large: `clear.png` 21 KB, `cloudy_moon.png` 134 KB, `partly_cloudy.gif` 574 KB.
- `loadIcon` (`widget_draw.go:145-172`) decodes PNGs through `png.Decode`, which produces `*image.NRGBA` or `*image.Paletted`. That is not `*image.RGBA`, so `x/image/draw` uses its slower generic interface path instead of the fast RGBA path.
- Per card per frame, there is a weather icon scale plus a thermometer scale (`enhanced_draw.go:277`, `simple_draw.go:391`). Pollution icons add more (`enhanced_draw.go:605`, `simple_draw.go:437`).
- The decode is cached, but the scale is not.

**D. Text layout and measurement repeated per frame.**
- `measureText` is called about 20 or more times per card (`enhanced_draw.go:125-136`, `:247`, `:274`, `:297`, `:341-358`, `:499-520`, and the ticker at `:639-663`). The fonts are cached, so no font is created per draw.
- `drawText` (`widget_draw.go:196-207`) allocates `image.NewUniform` per call.
- `font.Drawer.DrawString` rasterizes glyphs each time. With `font.HintingFull` and a `fallbackFace` wrapper, the glyph cache is only as good as the `opentype` face cache.
- The ticker (`enhanced_draw.go:617-665`) re-plans its items every frame via `weather.PlanPollutionTickerWithWeather`, which builds strings, and calls `StripEmoji` and `measureText` several times. This runs for every card, every frame.
- `getFace` takes `wd.mu` and builds a key with `fmt.Sprintf("%v:%.1f", ...)` (`widget_draw.go:102`) on every call. Many calls per frame means many small allocations.

**E. Per-pixel rounded-rect math.**
- `strokeRoundedRect` (`widget_draw.go:303-344`) loops over every pixel of the whole card rectangle (`for py ... for px ...`), not only the border. It does corner tests, and `math.Sqrt` in the corners. Each card is called once per frame (`enhanced_draw.go:231`, `:276`; `simple_draw.go:357`).
- For a 600x200 card that is about 120k iterations per card, 600k iterations for 5 cards, per frame, just to draw a 1 px border. This is one of the biggest pure-compute costs in the render.
- `fillRoundedRect` has the same full-area loop. It is only used in the cached `cardBackground` build (`enhanced_draw.go:187-189`) and in `drawSmoke` (`NoBackground` mode), which runs every frame (`enhanced_draw.go:225`, `simple_draw.go:347`).

**F. Compose and upload.**
- Background layers are cached in `backgroundCache` and composited with `draw.Draw(..., draw.Over)` each frame (`enhanced_draw.go:228`). That is a full-card alpha blend per frame, about 600k pixels for 5 cards. It is acceptable but not free.
- `UpdateSurfaceScaled` (`widget.go:224-341`) does a per-pixel loop converting the image to the DIB section. It is a straight copy at 1:1, or `fastScaleAndPremultiply` (line 344 onward) when the DPI scale is not 1.0, such as 125% or 150% on a laptop. The scaled path is a per-pixel bilinear loop with 4 multiplies per channel over the whole target.
- The DIB section is only recreated on a size change (`widget.go:254`), which is good.
- `UpdateLayeredWindow` is called with the whole surface each time (`widget.go:327`). There is no dirty rect (`UpdateLayeredWindow` supports one via `pdprcDirty` in `UPDATELAYEREDWINDOWINFO`/`UpdateLayeredWindowIndirect`, but it is not used). The DWM re-composites the window, which also costs some CPU and GPU time, probably visible under "DWM" rather than this process.

**G. Locking.** `redraw` holds `redrawMu` for the whole render. That is fine on a single goroutine, but `onWeatherUpdate` and the clock loop can contend briefly. It is not a cost driver.

### 3.2 Rough order of per-frame cost (hypothesis, to be measured)

1. `strokeRoundedRect` and `drawSmoke` full-rect loops.
2. `LoadLocation` and the tz parse and allocation.
3. `BiLinear.Scale` of the icons from large non-RGBA sources.
4. Text rasterization and the ticker planning.
5. The upload conversion, especially when the DPI scale is not 100%.
6. GC, driven by the per-frame `image.NewRGBA` and the items above.

## 4. Where the memory goes

| Source | Estimate | Evidence | Notes |
|---|---|---|---|
| Per-frame output image and GC headroom | Likely the largest, about 20 to 40 MB of the working set | `enhanced_draw.go:83`, `simple_draw.go:333` | Each frame allocates a multi-MB RGBA buffer. Go's GC target is 2x the live heap, so the heap oscillates. |
| Parsed fonts | About 7 MB | `widget_draw.go:57-92` | `NewWidgetDrawer` calls `assets.Fonts.ReadFile` and `opentype.Parse`. `embed.FS.ReadFile` returns a copy, and the parsed font keeps that copy. Loaded: `segoeui.ttf` 0.96 MB, `segoeuib.ttf` 0.95 MB, `droidsansfallback.ttf` **4.0 MB**, `notosanstamil.ttf` 0.56 MB, plus `seguisym.ttf` from `%WINDIR%`. Fallback faces are built for every size (`widget_draw.go:129-139`), each with its own glyph cache. |
| Decoded icons | Few MB | `iconCache` in `widget_draw.go:145-172` | PNGs are decoded at source size and kept (`NRGBA`, 4 bytes per pixel). Cached, so not a leak, but the full-size decode is far larger than the 64 to 84 px displayed. Only icons actually used are loaded. |
| GIF animations (Original theme) | Possibly 5 to 20 MB or more | `animation.go` `loadNativeAnimation` | Every frame is fully composited into its own `image.RGBA` (`copyFrame`, lines 69-71). `partly_cloudy.gif` is 574 KB compressed, `cloudy_moon.gif` 233 KB, and so on. Decoded frames can be hundreds of KB each, times the number of frames, per distinct icon in use. `gif.DecodeAll` also keeps its own paletted frames temporarily. |
| Background cache | Small | `backgroundCache` in `enhanced_draw.go:166-196` | One RGBA layer per distinct size, night and opacity key. The key includes the size, so a ticker or description change that alters the card size can add new entries without evicting old ones. This is bounded by the number of distinct sizes, but it is never cleaned. Worth checking in the heap profile. Decoded `day.jpg` and `night.jpg` are stored in `iconCache` (`enhanced_draw.go:162`). |
| DIB section | Size of the window, about 1 to 4 MB | `widget.go:254-285` | Reused, so it is not a leak. Only recreated on resize. |
| Locale data | Small | `internal/i18n/locale.go:199-235` `discoverLocales` | Reads and validates every embedded locale JSON once at startup. Parsed maps are kept. Probably 1 to 3 MB, and it is a one-off. |
| Go runtime baseline | About 8 to 12 MB | | Plus the Win32 and syscall stack, the HTTP client and TLS state. |
| Binary and embedded assets | Mapped from the exe, mostly not resident | `assets/embed.go` | The embedded assets total about 14 MB, including 7.2 MB of fonts. `embed.FS.ReadFile` copies into the heap only for assets that are read. |

I found no obvious goroutine or handle leak in `internal/ui-windows` in the files I read. The goroutines are `startClockLoop` (one), the resume listener (one), the scheduler (one) and the settings goroutine (one, only while the window is open). I did not review the network layer. The `OpenSettings` path in `settings.go` was not checked for WebView2 cleanup on Cancel, and it is relevant to the earlier hang. Confirm that the WebView2 browser processes exit after the settings window closes.

## 5. How to measure before changing anything

Do these in order. Use a release build, the real config (5 cities, Classic view, seconds clock, ticker and the icon theme the user uses), and the settings window closed.

1. **Establish the baseline per setting.** Record Task Manager CPU and memory for 2 minutes in each of these four combinations: ticker off with theme "new", ticker on with theme "new", ticker off with theme "original", and both on. Expect the first to be near 0 to 2%. This confirms that the 30 fps path is the cause.
2. **CPU profile with pprof.** Add a temporary, local-only `net/http/pprof` listener, or `runtime/pprof.StartCPUProfile` behind the existing `--debug` flag in `cmd/weatherwidget/main.go`. Capture 60 s at idle, then run `go tool pprof -top -cum weatherwidget.exe cpu.prof` and `-list`. Look for `strokeRoundedRect`, `time.LoadLocation` and `time.(*Location)`, `xdraw` scalers, `font.Drawer.DrawString`, `runtime.mallocgc` and `runtime.gcBgMarkWorker`, and `fastScaleAndPremultiply`.
3. **Heap and allocation profile.** Take `pprof -sample_index=alloc_space` and `inuse_space` heap profiles. `alloc_space` shows the churn (expect `image.NewRGBA`, `LoadLocation` and `Sprintf` at the top). `inuse_space` shows the resident set (expect fonts and GIF frames).
4. **Go runtime metrics.** Log `runtime.ReadMemStats` (`HeapAlloc`, `HeapSys`, `NumGC`, `PauseTotalNs`, `Mallocs`) or use `runtime/metrics` every 10 s. Compare `NumGC` per minute at 30 fps and at 1 fps. Also try running once with `GODEBUG=gctrace=1`.
5. **Windows Performance Monitor / Task Manager details.** Add `Process(weatherwidget)\% Processor Time`, `Private Bytes`, `Working Set`, `Handle Count` and `Thread Count`. A steady rise in handles over 30 minutes would indicate a GDI or window leak. Also check GDI Objects in the Task Manager details columns. Also record the DWM process CPU, to see the compositor cost of the full-window updates.
6. **Per-frame timing.** Log the elapsed time of `m.drawer.Render` and `UpdateSurfaceScaled` separately, averaged over 10 s. This tells whether a frame takes 2 ms or 20 ms, which sets how much a lower frame rate buys.
7. **Optional.** Windows Performance Recorder / Analyzer (CPU sampling) or `go tool trace` for GC pauses.

After each change, repeat steps 1, 2 and 4 and compare.

## 6. Ranked recommendations

Impact is the estimated reduction in this process's CPU at idle with the animation setting on. Risk is regression risk. All estimates are unverified.

| # | Change | Where | Est. impact | Risk |
|---|---|---|---|---|
| 1 | **Lower the animated redraw rate** to 10 to 15 fps (tick every 66 to 100 ms), or compute the next-needed-redraw time. The ticker moves 1 px per 25 ms, so at 15 fps it steps about 2.7 px per frame, which is still smooth. GIF frames at 100 ms need 10 fps. A tick interval of 66 ms (15 fps) halves the CPU. | `manager.go:307` and `:318-322` | **About 50% (at 15 fps) to 67% (at 10 fps) of the animation CPU** | Low. A slightly less smooth ticker. Make it a constant, or an "animation smoothness" choice. |
| 2 | **Stop animating when nothing is animating.** The current `animated` flag is a config check only. Redraw per tick only if the ticker actually has items (`PlanPollutionTickerWithWeather` returns at least one) or the current icons are GIFs (`anim != nil`). `IconThemeOriginal` has PNG-only icons (`clear.png`, `fog.png`, `rain.png`, `snow.png`, ...), which never animate but still trigger 30 fps redraws. | `manager.go:318`, `animation.go` `weatherIcon`, `enhanced_draw.go:617` | Large when those conditions apply, since it falls back to 1 fps. | Low to medium. Needs a "has animation" signal from the drawer. |
| 3 | **Cache the `*time.Location` per timezone string**, in a `sync.Map` or on `WidgetDrawer`, and change `weather.FormatTime/FormatDate` to use it (or compute the localized time once per card per frame and pass it down). | `internal/weather/format.go:71,88,102`, `enhanced_draw.go:221`, `simple_draw.go:350,380` | Likely 10 to 30% of the frame cost, plus a big cut in allocations. | Low. Locations are immutable. Keep the invalid-zone fallback to UTC. |
| 4 | **Draw only the border pixels in `strokeRoundedRect`**, and make `drawSmoke`/`fillRoundedRect` fill rows with `draw.Draw` and only evaluate the 4 corner squares. Better, pre-render the border and smoke layers once per size into the same cache as `cardBackground`. | `widget_draw.go:250-344`, `enhanced_draw.go:225,231`, `simple_draw.go:347,357` | Possibly 20 to 40% of the per-frame compute. | Low, if the output is checked pixel-for-pixel by existing tests (`draw_test.go`, `enhanced_draw_test.go`). |
| 5 | **Pre-scale icons once** to the target size as `*image.RGBA` and cache by (path, size) and for GIFs by (frame, size). Also convert decoded PNGs to `*image.RGBA` at load. | `widget_draw.go:145-172`, `:219-225`, `animation.go`, `enhanced_draw.go:268,277,605`, `simple_draw.go:388,391,437` | 10 to 25% CPU. Also reduces memory when frames are stored at 84x84 instead of the full source size. | Low to medium. Visual change possible from scale quality and from DPI scale changes, so key the cache by the final pixel size. |
| 6 | **Reuse the render buffer.** Keep one `*image.RGBA` per drawer, resize only when the dimensions change, and clear it each frame (`clear(dst.Pix)`). | `enhanced_draw.go:83`, `simple_draw.go:333`, plus the `redraw` copies at `manager.go:271-285` | Removes most per-frame garbage, so GC CPU falls and the resident memory is lower and steadier. | Medium. Needs care that `Render` returns an image that `UpdateSurfaceScaled` reads synchronously (it does, under `redrawMu`). |
| 7 | **Cache font faces by a struct key**, not `fmt.Sprintf`. Cache `measureText` results per (face, string) for strings that do not change. | `widget_draw.go:98-142`, `measureText` | A few %. Fewer allocations. | Low. |
| 8 | **Cache static content.** Render everything except the dynamic parts (clock, ticker, animated icon) into a per-card cached layer, rebuilt only on weather, config or size change, and draw the dynamic parts over a copy each frame. The clock changes once per second, so it only needs a redraw at 1 Hz. | `Render`, `drawEnhancedCard`, `drawSimpleCard` | Potentially 60 to 80% of per-frame CPU with the animation on. | **Highest effort and risk.** Layout depends on text widths that change with the time string, so the cache needs careful invalidation. Do it after 1 to 7 if the numbers are still too high. |
| 9 | **Use a dirty rect or smaller update region for the ticker-only frames.** Only the ticker strip and animated icon change between seconds. | `widget.go:327` `UpdateLayeredWindow` (`pdprcDirty`) | Reduces DWM and upload cost. | Medium. Depends on #8 to know which region changed. |
| 10 | **Reduce font memory.** Load `droidsansfallback.ttf` (4 MB) and the Tamil fonts lazily, only when a glyph is missing from Segoe UI. Share one `*opentype.Font` instance and avoid building fallback faces for every size until needed. | `widget_draw.go:57-92`, `:128-139`, `font_fallback.go` | About 5 to 7 MB. | Low to medium. Fallback glyph lookup must still work for CJK/Tamil city names. |
| 11 | **Limit GIF frame memory** for the Original theme: store frames at the display size (#5), or share frames between cities with the same icon (already shared by path), and release animations when the theme is changed. | `animation.go:46-95` | 5 to 20 MB, depending on the icons in use. | Low to medium. |
| 12 | **Bound `backgroundCache`**: drop entries whose size is no longer in use when a new size is added (keep the last few). | `enhanced_draw.go:166-196` | Small, but prevents slow growth. | Low. |
| 13 | **Tune the GC**: `debug.SetGCPercent(50)` or a `debug.SetMemoryLimit` soft cap at startup. This trades CPU for memory. Do it only after #6, since removing the garbage is better than collecting it faster. | `cmd/weatherwidget/main.go` | Lower resident memory, slightly higher GC CPU. | Low. |
| 14 | **Pause the loop when hidden.** Stop the ticker when `isWidgetVisible` is false, and restart it in `showWidget`. Also consider pausing while the session is locked or the screen is off. | `manager.go:306-327`, `:396-418` | About 0.1% CPU. Mostly saves wake-ups. | Low. |

### Suggested order

1. Measure per section 5 steps 1 to 4. This confirms the animation setting and shows the real hot spots.
2. Apply #1 and #2 first. These are the simplest and have the biggest effect.
3. Apply #3, #4 and #5 next. These are local changes with low risk and good payback.
4. Apply #6 and #7 to cut the GC churn and memory, then re-measure.
5. Only then consider #8 and #9, and the memory items #10 to #13.

### What the user can do today without code changes

- Turn the ticker off, and use the "new" icon theme instead of "Original". This should bring the idle CPU to the 1-redraw-per-second path.
- Fewer cities and Simple view both reduce the per-frame area.
- Setting the widget scale explicitly to 100% avoids the per-pixel upscale path, if the display DPI scale is above 100%.

## 7. Shared and other platforms

- `internal/weather/format.go` (`FormatTime`, `FormatDate`, `FormatDateTime`) is shared by every UI. Caching the location there (#3) helps all three platforms.
- `internal/ui-gtk/panel.go:1160,1203` and `panel_simple.go:916,954`, and `internal/ui-darwin/manager.go:312,740`, also call `time.LoadLocation` per use. I did not examine how often they run. GTK and macOS use native toolkit drawing, so the per-pixel software render in section 3 applies only to Windows.
- `internal/i18n/locale.go` is loaded once. It is not a periodic cost.
- `internal/scheduler/scheduler.go` runs one goroutine and one ticker. It is not a concern.
- Tests that cover the code that would change, to run after any change: `internal/ui-windows/draw_test.go`, `enhanced_draw_test.go`, `widget_test.go`, and the `internal/weather` format tests. Run them with `go test ./internal/ui-windows/... ./internal/weather/...` on Windows.

## 8. Confidence and gaps

- High confidence: the 33 ms ticker and the full re-render path (`manager.go:307-325`, `enhanced_draw.go:83`, `widget.go:224-341`), and the uncached `LoadLocation` calls.
- Medium confidence: the relative cost order in 3.2 and the memory split in section 4. These are estimates from code reading, with no profile.
- Not examined: the weather provider and HTTP client memory, the `webview2` library's process usage, the tray code, and the `font_fallback.go` glyph cache behavior. The exact pixel dimensions of the 5-city window were not measured.
