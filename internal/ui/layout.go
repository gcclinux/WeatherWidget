package ui

import "weatherwidget/internal/config"

// PanelWidth is the width of a single CityPanel in device-independent pixels.
const PanelWidth = 160

// PanelHeight is the default height of a single CityPanel in device-independent pixels.
const PanelHeight = 185

// Element height contributions in dip (approximate).
const (
	heightCity      = 24 // city name text + spacing
	heightIcon      = 70 // icon (64) + spacing
	heightTemp      = 48 // large temperature text + spacing
	heightDesc      = 18 // description text + spacing
	heightHumidWind = 18 // humidity/wind row + spacing
	heightTime      = 26 // time text
	heightDate      = 18 // date text
	heightSeparator = 8  // separator line + spacing
	heightPadding   = 16 // container padding (top + bottom)
	heightSpacers   = 10 // spacers between sections
)

// CalculateLayout computes the widget dimensions for the given number of city panels.
// It returns the total width (cityCount x 160 dip), height (185 dip), and number of panel slots.
func CalculateLayout(cityCount int) (width, height, slots int) {
	return cityCount * PanelWidth, PanelHeight, cityCount
}

// CalculateLayoutWithFields computes the widget dimensions accounting for
// which display fields are visible. Returns total width, dynamic height, and slot count.
func CalculateLayoutWithFields(cityCount int, df *config.DisplayFields) (width, height, slots int) {
	if df == nil {
		df = config.DefaultDisplayFields()
	}

	h := heightPadding + heightSpacers
	if df.ShowCity {
		h += heightCity
	}
	if df.ShowIcon {
		h += heightIcon
	}
	if df.ShowTemp {
		h += heightTemp
	}
	if df.ShowDesc {
		h += heightDesc
	}
	if df.ShowHumidWind {
		h += heightHumidWind
	}
	if df.ShowTime || df.ShowDate {
		h += heightSeparator
	}
	if df.ShowTime {
		h += heightTime
	}
	if df.ShowDate {
		h += heightDate
	}

	return cityCount * PanelWidth, h, cityCount
}
