package ui

import "weatherwidget/internal/config"

// PanelWidth is the width of a single city card in device-independent pixels.
// The redesigned card is laid out horizontally (info block beside a metrics
// grid); it is wider than the old vertical column but kept compact.
const PanelWidth = 380

// PanelHeight is the default height of a single city card in device-independent pixels.
const PanelHeight = 260

// Element height contributions in dip (approximate).
//
// The card's top region height is driven by the taller of the left info block
// and the right metrics grid. The left block dominates, so the height estimate
// sums the visible left-block elements plus the air-quality row.
const (
	heightCity    = 26 // location line + spacing
	heightIcon    = 100 // weather icon (96) + spacing
	heightTemp    = 52 // large temperature text + spacing
	heightDesc    = 20 // condition text + spacing
	heightTime    = 30 // time text + spacing
	heightDate    = 20 // date text + spacing
	heightAirRow  = 54 // air-quality icon + value row
	heightPadding = 40 // card + content padding, border, inter-card gap (top + bottom)
)

// CalculateLayout computes the widget dimensions for the given number of city cards.
// Cards are stacked vertically, so the total width is a single card width and the
// total height is cityCount × card height. Returns width, height, and slot count.
func CalculateLayout(cityCount int) (width, height, slots int) {
	if cityCount < 1 {
		cityCount = 1
	}
	return PanelWidth, cityCount * PanelHeight, cityCount
}

// CalculateLayoutWithFields computes the widget dimensions accounting for
// which display fields are visible. Cards are stacked vertically, so the total
// width is a single card width and the total height is cityCount × per-card height.
//
// The air-quality row is a premium feature and is off by default, so this
// helper assumes no pollution metrics. Use CalculateLayoutWithPollution when a
// pollution selection is available.
func CalculateLayoutWithFields(cityCount int, df *config.DisplayFields) (width, height, slots int) {
	return CalculateLayoutWithPollution(cityCount, df, nil)
}

// CalculateLayoutWithPollution computes the widget dimensions accounting for
// both the visible display fields and whether any air-quality metric is shown.
// The air-quality row only contributes to the height when at least one
// pollution metric is enabled, so panels aren't padded with empty space when
// pollution data is hidden.
func CalculateLayoutWithPollution(cityCount int, df *config.DisplayFields, pf *config.PollutionFields) (width, height, slots int) {
	if cityCount < 1 {
		cityCount = 1
	}
	if df == nil {
		df = config.DefaultDisplayFields()
	}

	// Per-card height is dominated by the left info block. The air row only
	// adds height when at least one pollution metric is visible.
	perCard := heightPadding
	if anyPollutionVisible(pf) {
		perCard += heightAirRow
	}
	if df.ShowCity {
		perCard += heightCity
	}
	if df.ShowIcon {
		perCard += heightIcon
	}
	if df.ShowTime {
		perCard += heightTime
	}
	if df.ShowDate {
		perCard += heightDate
	}
	if df.ShowTemp {
		perCard += heightTemp
	}
	if df.ShowDesc {
		perCard += heightDesc
	}

	return PanelWidth, cityCount * perCard, cityCount
}

// anyPollutionVisible reports whether the given pollution selection enables at
// least one air-quality metric. A nil selection means none are visible.
func anyPollutionVisible(pf *config.PollutionFields) bool {
	if pf == nil {
		return false
	}
	return pf.ShowCO || pf.ShowNO || pf.ShowNO2 || pf.ShowO3 ||
		pf.ShowSO2 || pf.ShowNH3 || pf.ShowPM25 || pf.ShowPM10 || pf.ShowAQI
}
