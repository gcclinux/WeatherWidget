package ui

// PanelWidth is the width of a single CityPanel in device-independent pixels.
const PanelWidth = 300

// PanelHeight is the height of a single CityPanel in device-independent pixels.
const PanelHeight = 120

// CalculateLayout computes the widget dimensions for the given number of city panels.
// It returns the total width (cityCount × 300 dip), height (120 dip), and number of panel slots.
func CalculateLayout(cityCount int) (width, height, slots int) {
	return cityCount * PanelWidth, PanelHeight, cityCount
}
