//go:build linux

package uitk

import (
	"github.com/gotk3/gotk3/gtk"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

// weatherPanel is the common interface for both simple and enhanced city panels.
// It allows the manager to work with either panel type uniformly.
type weatherPanel interface {
	// GetRoot returns the root GTK widget for embedding in a container.
	GetRoot() *gtk.Box

	// GetWidth returns the card width for this panel type.
	GetWidth() int

	// update refreshes all displayed fields with new weather data.
	update(d *weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme)

	// showError displays an error state on the panel.
	showError(isStale bool)

	// setNoBackground adjusts the panel background CSS class.
	setNoBackground(enable bool)

	// setTintAlpha updates the panel's tint alpha value used for compositing icons.
	setTintAlpha(alpha float64)

	// applyDisplayFields shows or hides individual elements based on the config.
	applyDisplayFields(df *config.DisplayFields)

	// applyPollutionRows updates the pollution metric rows.
	applyPollutionRows(pf *config.PollutionFields)

	// applyIconSize rescales the currently displayed icon to a new pixel size.
	applyIconSize(size int)

	// stopClock stops the clock goroutine.
	stopClock()
}

// Verify that both panel types implement the weatherPanel interface.
var (
	_ weatherPanel = (*cityPanel)(nil)
	_ weatherPanel = (*simplePanel)(nil)
)
