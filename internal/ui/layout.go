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

	// In the horizontal card, the middle region places the weather icon and the
	// info column (time, date, temp, condition) side-by-side in an HBox, with
	// the 3x2 metrics grid beside them. Their heights do not sum vertically.
	infoColHeight := 0
	if df.ShowTime {
		infoColHeight += heightTime
	}
	if df.ShowDate {
		infoColHeight += heightDate
	}
	if df.ShowTemp {
		infoColHeight += heightTemp
	}
	if df.ShowDesc {
		infoColHeight += heightDesc
	}

	middleHeight := infoColHeight
	if df.ShowIcon && heightIcon > middleHeight {
		middleHeight = heightIcon
	}
	if middleHeight < 110 {
		middleHeight = 110 // metrics grid minimum height
	}

	perCard := heightPadding + middleHeight
	if df.ShowCity {
		perCard += heightCity
	}
	if anyPollutionVisible(pf) {
		perCard += heightAirRow
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

// SimplePanelWidth is the width of a single CityPanel in Simple (Classic) view.
const SimplePanelWidth = 160

// SimplePanelHeight is the default height of a single CityPanel in Simple (Classic) view.
const SimplePanelHeight = 185

// Simple element height contributions in dip (approximate).
const (
	simpleHeightCity      = 24 // city name text + spacing
	simpleHeightIcon      = 70 // icon (64) + spacing
	simpleHeightTemp      = 48 // large temperature text + spacing
	simpleHeightDesc      = 18 // description text + spacing
	simpleHeightHumidity  = 18 // humidity row + spacing
	simpleHeightWindRow   = 18 // wind speed + direction row + spacing
	simpleHeightTime      = 26 // time text
	simpleHeightDate      = 18 // date text
	simpleHeightSeparator = 8  // separator line + spacing
	simpleHeightPadding   = 16 // container padding (top + bottom)
	simpleHeightSpacers   = 10 // spacers between sections
	simpleHeightInfoRow      = 18 // generic info row height (wind gust, dew point, pressure, UV, wind dir)
	simpleHeightPollutionRow = 18 // pollution row height
)

// CalculateSimpleLayout computes the widget dimensions for Simple view.
func CalculateSimpleLayout(cityCount int) (width, height, slots int) {
	if cityCount < 1 {
		cityCount = 1
	}
	return cityCount * SimplePanelWidth, SimplePanelHeight, cityCount
}

// CalculateSimpleLayoutWithFields computes the widget dimensions for Simple view
// accounting for which display fields are visible.
func CalculateSimpleLayoutWithFields(cityCount int, df *config.DisplayFields) (width, height, slots int) {
	return CalculateSimpleLayoutWithPollution(cityCount, df, nil)
}

// CalculateSimpleLayoutWithPollution computes the widget dimensions for Simple view
// accounting for visible display fields and air quality / pollution metrics.
func CalculateSimpleLayoutWithPollution(cityCount int, df *config.DisplayFields, pf *config.PollutionFields) (width, height, slots int) {
	count := 0
	if pf != nil {
		if pf.ShowAQI {
			count++
		}
		if pf.ShowCO {
			count++
		}
		if pf.ShowNO {
			count++
		}
		if pf.ShowNO2 {
			count++
		}
		if pf.ShowO3 {
			count++
		}
		if pf.ShowSO2 {
			count++
		}
		if pf.ShowNH3 {
			count++
		}
		if pf.ShowPM25 {
			count++
		}
		if pf.ShowPM10 {
			count++
		}
	}
	return CalculateSimpleLayoutWithPollutionRows(cityCount, df, count)
}

// CalculateSimpleLayoutWithPollutionRows computes the widget dimensions for Simple view
// given the exact number of visible pollution rows.
func CalculateSimpleLayoutWithPollutionRows(cityCount int, df *config.DisplayFields, pollutionRowCount int) (width, height, slots int) {
	if cityCount < 1 {
		cityCount = 1
	}
	if df == nil {
		df = config.DefaultDisplayFields()
	}

	h := simpleHeightPadding + simpleHeightSpacers
	if df.ShowCity {
		h += simpleHeightCity
	}
	if df.ShowIcon {
		h += simpleHeightIcon
	}
	if df.ShowTemp {
		h += simpleHeightTemp
	}
	if df.ShowDesc {
		h += simpleHeightDesc
	}
	if df.ShowHumidity {
		h += simpleHeightHumidity
	}
	if df.ShowWind {
		h += simpleHeightWindRow
	}
	if df.ShowWindGust {
		h += simpleHeightInfoRow
	}
	if df.ShowDewPoint {
		h += simpleHeightInfoRow
	}
	if df.ShowPressure {
		h += simpleHeightInfoRow
	}
	if df.ShowUVIndex {
		h += simpleHeightInfoRow
	}
	if pollutionRowCount > 0 {
		h += pollutionRowCount * simpleHeightPollutionRow
	}
	if df.ShowTime || df.ShowDate {
		h += simpleHeightSeparator
	}
	if df.ShowTime {
		h += simpleHeightTime
	}
	if df.ShowDate {
		h += simpleHeightDate
	}

	return cityCount * SimplePanelWidth, h, cityCount
}
