# Classic view forecast title replaces summary sentence

In Simple (Classic) view, the Today's and Tomorrow's Forecast grid modes now show a localized two-line title ("Today's" / "Forecast") in place of the long forecast summary. A new helper, `i18n.ForecastTitle`, reuses the existing `settings.gridMode.today` and `settings.gridMode.tomorrow` keys, which are present in all 12 locale files. It breaks the label at the first space. Windows, GTK and Darwin all call it, and Live mode and Enhanced view are untouched. No commit, branch or push was made: HEAD equals origin/release/2026, and the only staged items are the earlier gtk-cities-list work.

Watch for: GTK and macOS two-line rendering and the Linux no-widening result were not run (possible, per results.md). No unit test covers the newline branch of `wrapDescription`.

**Verdict**: APPROVED

## High-level view

The title comes from one shared helper, so all three platforms get the same text and break points. Single-word labels (ja-JP, zh-CN) stay on one line, and a missing locale manager falls back to English. No new locale keys were needed, and all 12 locale files already have both keys.

Windows keeps the explicit newline in `wrapDescription`, so the card width and height calculations see two short lines. On GTK the title is returned only from the simple-panel `headlineValues` (Enhanced uses `d.Description` directly), and the label is centre-justified. On Darwin the title replaces the description only when the view mode is Simple, and `panel.m` allows 2 lines only when the text contains a newline. The Linux card no longer sizes to the summary sentence, because the summary is no longer used.

<details>
<summary>Issues (2)</summary>

1. **No test for newline branch of wrapDescription** (confirmed) — the new early return for explicit newlines is only covered indirectly through the `draw_test.go` expectations. Consider adding a case to `TestWrapDescription`.
2. **Unverified native rendering** (possible) — GTK label width and macOS 2-line NSTextField height were not run here. Check them on Linux and macOS, including a long label such as fr-FR "Prévisions / d'aujourd'hui".

</details>

<details>
<summary>Details</summary>

### Title derivation and locale coverage

`ForecastTitle` returns "" for any mode other than today or tomorrow, so Live mode keeps the real description on every platform. Break points come out sensibly: "Vorhersage / heute", "Pronóstico / de hoy", "Verwachting / vandaag". Tamil, Japanese and Chinese labels have no space in the checked values and stay on one line. The Tamil labels are long for a single line, which is worth a visual check on Windows, where the card width is measured from the line.

### Platform wiring

Windows removes the summary override in `simpleCardWeather` and applies the title after the mode switch. The unit-test expectations were updated to match. GTK `headlineValues` applies the title before the forecast-nil check, so the title also appears when forecast data has not loaded. Windows does the same. Darwin does the same via the Simple-only override in `updateCard`. The three platforms behave consistently.

### Scope

The tree also holds the earlier extra-metrics work (panel_simple extras, `cardMetrics`, simple_draw metrics). It was not reviewed here. The gofmt-only churn in `manager.go` is cosmetic and bundled.

</details>

<details>
<summary>File map</summary>

- internal/i18n/forecast_title.go, forecast_title_test.go: new helper and test
- internal/ui-windows/simple_draw.go, draw_test.go: title used, newline kept by wrapDescription
- internal/ui-gtk/metrics.go, panel_simple.go: title in headlineValues, centre justify
- internal/ui-darwin/manager.go, panel.m: Simple-only title, 2-line label

Full diff: `git diff`

</details>
