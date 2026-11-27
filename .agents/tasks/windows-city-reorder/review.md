# Up/down city re-ordering in the Windows settings panel

Saved Cities rows in the Windows settings panel now have up and down arrow buttons left of Remove. The first row shows only down, the last only up, and a single city shows none. Moves swap entries in the pending `currentConfig.cities`, the same state add and remove use, so Cancel discards them and Save sends the new order through `collectConfigFromUI`. Tooltip and aria-label text comes from two new locale keys, present in all 12 locale files and in `TestEnGBCompleteness`.

Watch for: reordering the default demo cities without a license is rejected on Save by the existing licence check (possible, non-blocking). The WebView2 UI was not run, so click behaviour and the widget's order after Save are unverified at runtime (confirmed gap, covered by code reading below).

**Verdict**: APPROVED

## High-level view

The change is confined to `app.js`, `style.css` and the locale files. It follows the GTK/darwin approach of swapping entries in the in-memory city list and saving it with the config. No Go code changed.

After Save, `saveSettings` compares old and new city lists with `reflect.DeepEqual`. A reorder counts as a change, so cached results are cleared and a fetch is triggered. This matters because the drawers pair `cities[i]` with `results[i]` by index, and stale results would otherwise show weather under the wrong city.

Live preview does not fire for city edits, and the preview redraw always uses the saved city list. The widget therefore does not change order until Save, which is sensible.

<details>
<summary>Issues (2)</summary>

1. **Licence check on reorder of default cities (possible)**: an unlicensed user who reorders the three default cities gets `licenseRequired` on Save because the list no longer DeepEquals `DefaultCities()`. Confirm this is acceptable for demo mode, or ignore it if GTK/darwin behave the same.
2. **No automated test for reorder (confirmed)**: the repo has no JS test harness, and no Go test covers a reordered-cities save. Consider a `saveSettings` test asserting the order persists and results are cleared.

</details>

<details>
<summary>Details</summary>

### Pending-state semantics and persistence

`moveCity` swaps in place and no-ops on out-of-range moves. The buttons call it and then re-render the list. Because `collectConfigFromUI` clones `currentConfig`, Save carries the order to `goSaveConfig`. Cancel closes the window without saving, and the earlier Cancel fix is not touched by this diff.

### Result alignment after Save

Results are index-aligned with cities in `simple_draw.go` and `enhanced_draw.go`. `saveSettings` clears `m.results` and calls `FetchNow` when `old.Cities` differs from `cfg.Cities`, which includes order. The widget briefly shows no data after Save, then repopulates in the new order. The behaviour is the same as for add and remove.

### Licence check interaction

`saveSettings` rejects any city list that is not the default list when `!cfg.HasLicense()`. A reorder of the defaults is therefore refused with the licence error. The user's scenario (5 cities) implies a licence, so this is non-blocking.

</details>

<details>
<summary>File map</summary>

- `internal/ui-windows/web/app.js`: `moveCity` helper, arrow buttons per row, two new keys in the translation fetch list
- `internal/ui-windows/web/style.css`: `.city-item-actions` and `.btn-move-city` styles
- `internal/i18n/locales/*.json` (12 files): `settings.locations.moveUp` and `moveDown`
- `internal/i18n/locale_test.go`: the two keys added to the completeness list

Full diff: `git diff` in the repo. The tree also holds unrelated earlier uncommitted work that was not reviewed.

</details>
