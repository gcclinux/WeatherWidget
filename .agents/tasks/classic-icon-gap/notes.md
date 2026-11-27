# Classic icon gap (Windows)

Change in `internal/ui-windows/simple_draw.go` (Simple tile header): thermometer x `center+14` -> `center+20`, feels-like text x `center+8` -> `center+14`. The gap between the weather icon and the thermometer block grows by 6 px. Nothing else moves: the icon, big temperature, city and rows are untouched, and the icon is still positioned relative to `center`.

The file uses plain pixel offsets with no separate DPI conversion. The tile is drawn in logical pixels and scaled as a whole, so the 6 px follows the widget scale like the neighbouring constants.

Fit: the block's right edge is about center+14+45 (for "-12°C" or "-12°F" at size 14), roughly 60 px from center. The default tile is 380 px wide (190 px half-width), so there is no clipping.

Checks: `go build ./...` and `go vet` passed. `go test` for internal/ui-windows passed. No tests hard-code these positions.

`gofmt -l` still lists simple_draw.go, tray.go and two test files. The cause is CRLF line endings, which were already there. This change did not introduce them.

Not verified visually: no screenshot was taken, so the look at different scales and the group centring (the icon+thermometer group is now about 3 px off-centre) are unchecked. Nothing was committed.

Follow-up: shifted a further 4 px right together: thermometer x center+20 -> center+24, feels-like text x center+14 -> center+18. Right edge about center+18+45 = ~63 px, well inside the 190 px half-width.
