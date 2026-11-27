# Expand the Saved Cities list in the GTK Locations tab

The scrolled city list in `buildLocationsTab` (internal/ui-gtk/settings.go) now expands into the tab's free vertical space. The change removes the expanding spacer that sat above the "Add New City" frame. It also sets `SetVExpand(true)` and packs the scroller with expand/fill. The minimum height rises from 180px to 220px. The edit is one file, +6/-12, and no other tab is touched.

Watch for: the 220px minimum is an estimate and may be slightly under five rows' natural height, which only matters on small windows (possible). Nothing was verified at runtime on Linux (the commit message says so). **Verdict**: APPROVED

## High-level view

The old layout fixed the list at 180px and added a spacer that absorbed all free space, so the list never grew. The spacer is gone and the list takes that role. At the default 600x800 dialog size, five rows should fit with no scrollbar.

The per-tab outer scroller wraps the content in a viewport, which gives the tab at least the visible height. The expanding list therefore receives the extra space. On small windows the outer scroller and the inner automatic-policy scroller remain as fallbacks, as the task requires.

<details>
<summary>Issues (2)</summary>

1. **Minimum height estimate** (possible) — five rows of buttons (about 34-40px each), four separators and 4px spacing come to roughly 206-240px, so 220px may be marginal on windows too small to expand. Optionally raise it a little. A scrollbar is acceptable there.
2. **No Linux runtime verification** (confirmed) — the build was not run here (cgo/GTK is unavailable on Windows). The user plans to test on Linux, so confirm the row heights and that no scrollbar appears with 5 cities.

</details>

<details>
<summary>Details</summary>

### Spacer replaced by an expanding scroller

Before, the spacer took the free space and the list stayed at its 180px request. Now the list holds the expand slot, and the Add New City frame stays at the bottom of the tab. The result is the same visual layout with the list taller. The unused `spacer` variable is removed, so no dead code is left. The comments were updated to match the new behavior.

### Fallback on small windows

The policy stays `POLICY_NEVER` horizontally and `POLICY_AUTOMATIC` vertically. Shrinking the window still scrolls instead of clipping. The only open point is the exact 220px floor (Issue 1). It has no effect at the default dialog size.

</details>

<details>
<summary>File map</summary>

- internal/ui-gtk/settings.go: city list scroller expands, spacer removed, minimum height 220px

Full diff: `git show 623f40f`

</details>
