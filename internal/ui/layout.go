package ui

// PanelWidth is the width of a single CityPanel in device-independent pixels.
const PanelWidth = 160

// PanelHeight is the height of a single CityPanel in device-independent pixels.
const PanelHeight = 185

// CalculateLayout computes the widget dimensions for the given number of city panels.
// It returns the total width (cityCount × 160 dip), height (185 dip), and number of panel slots.
func CalculateLayout(cityCount int) (width, height, slots int) {
	return cityCount * PanelWidth, PanelHeight, cityCount
}
