package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"weatherwidget/internal/config"
)

// settingsState holds mutable UI state for the settings dialog.
type settingsState struct {
	cities []config.CityConfig
	window fyne.Window
}

// ShowSettings opens a settings dialog window. cfg is the current config to
// populate the form. onSave is called with the edited config when the user
// clicks Save and validation passes; it should persist and test the connection,
// returning an error if the test fails.
func (u *UIManager) ShowSettings(cfg *config.Config, onSave func(*config.Config) error) {
	if u.settings != nil {
		u.settings.RequestFocus()
		return
	}

	win := u.app.NewWindow("WeatherWidget Settings")
	u.settings = win
	win.SetFixedSize(false)
	win.Resize(fyne.NewSize(520, 600))

	state := &settingsState{
		cities: copyCities(cfg.Cities),
		window: win,
	}

	// --- Data Source ---
	dataSourceRadio := widget.NewRadioGroup(
		[]string{"Remote API", "Local Database"},
		nil, // set below
	)
	if cfg.DataSource == config.DataSourceLocalDatabase {
		dataSourceRadio.SetSelected("Local Database")
	} else {
		dataSourceRadio.SetSelected("Remote API")
	}

	// --- API Configuration ---
	providerSelect := widget.NewSelect(
		[]string{"OpenWeatherMap", "Weather Underground"},
		nil,
	)
	switch {
	case cfg.APIConfig != nil && cfg.APIConfig.Provider == "weatherunderground":
		providerSelect.SetSelected("Weather Underground")
	default:
		providerSelect.SetSelected("OpenWeatherMap")
	}

	apiKeyEntry := widget.NewEntry()
	apiKeyEntry.SetPlaceHolder("API Key")
	if cfg.APIConfig != nil {
		apiKeyEntry.SetText(cfg.APIConfig.APIKey)
	}

	apiForm := widget.NewForm(
		widget.NewFormItem("Provider", providerSelect),
		widget.NewFormItem("API Key", apiKeyEntry),
	)
	apiSection := container.NewVBox(
		widget.NewLabel("API Configuration"),
		apiForm,
	)

	// --- Database Configuration ---
	dbHostEntry := widget.NewEntry()
	dbHostEntry.SetPlaceHolder("localhost")
	dbPortEntry := widget.NewEntry()
	dbPortEntry.SetPlaceHolder("5432")
	dbNameEntry := widget.NewEntry()
	dbNameEntry.SetPlaceHolder("weather_db")
	dbUserEntry := widget.NewEntry()
	dbUserEntry.SetPlaceHolder("postgres")
	dbPassEntry := widget.NewPasswordEntry()
	dbPassEntry.SetPlaceHolder("password")
	dbQueryEntry := widget.NewMultiLineEntry()
	dbQueryEntry.SetPlaceHolder("SELECT temperature, description, icon_code FROM weather WHERE city = $1")
	dbQueryEntry.SetMinRowsVisible(2)

	if cfg.DatabaseConfig != nil {
		dbHostEntry.SetText(cfg.DatabaseConfig.Host)
		if cfg.DatabaseConfig.Port > 0 {
			dbPortEntry.SetText(strconv.Itoa(cfg.DatabaseConfig.Port))
		}
		dbNameEntry.SetText(cfg.DatabaseConfig.DBName)
		dbUserEntry.SetText(cfg.DatabaseConfig.Username)
		dbPassEntry.SetText(cfg.DatabaseConfig.Password)
		dbQueryEntry.SetText(cfg.DatabaseConfig.Query)
	}

	dbForm := widget.NewForm(
		widget.NewFormItem("Host", dbHostEntry),
		widget.NewFormItem("Port", dbPortEntry),
		widget.NewFormItem("Database", dbNameEntry),
		widget.NewFormItem("Username", dbUserEntry),
		widget.NewFormItem("Password", dbPassEntry),
		widget.NewFormItem("Query", dbQueryEntry),
	)
	dbSection := container.NewVBox(
		widget.NewLabel("Database Configuration"),
		dbForm,
	)

	// Toggle visibility based on data source selection.
	apiSection.Show()
	dbSection.Hide()
	if cfg.DataSource == config.DataSourceLocalDatabase {
		apiSection.Hide()
		dbSection.Show()
	}
	dataSourceRadio.OnChanged = func(selected string) {
		if selected == "Local Database" {
			apiSection.Hide()
			dbSection.Show()
		} else {
			apiSection.Show()
			dbSection.Hide()
		}
	}

	dataSourceSection := container.NewVBox(
		widget.NewLabel("Data Source"),
		dataSourceRadio,
		apiSection,
		dbSection,
	)

	// --- City List ---
	cityListContainer := container.NewVBox()
	var refreshCityList func()

	refreshCityList = func() {
		cityListContainer.RemoveAll()
		for i, c := range state.cities {
			idx := i
			city := c
			posLabel := widget.NewLabel(fmt.Sprintf("#%d  %s, %s", idx+1, city.Name, city.Region))

			upBtn := widget.NewButton("↑", func() {
				if idx > 0 {
					state.cities[idx], state.cities[idx-1] = state.cities[idx-1], state.cities[idx]
					refreshCityList()
				}
			})
			downBtn := widget.NewButton("↓", func() {
				if idx < len(state.cities)-1 {
					state.cities[idx], state.cities[idx+1] = state.cities[idx+1], state.cities[idx]
					refreshCityList()
				}
			})
			removeBtn := widget.NewButton("Remove", func() {
				result, err := config.RemoveCity(state.cities, idx)
				if err != nil {
					dialog.ShowError(err, win)
					return
				}
				state.cities = result
				refreshCityList()
			})

			if idx == 0 {
				upBtn.Disable()
			}
			if idx == len(state.cities)-1 {
				downBtn.Disable()
			}
			if len(state.cities) <= 1 {
				removeBtn.Disable()
			}

			row := container.NewHBox(posLabel, layout.NewSpacer(), upBtn, downBtn, removeBtn)
			cityListContainer.Add(row)
		}
	}
	refreshCityList()

	// Add city controls
	addNameEntry := widget.NewEntry()
	addNameEntry.SetPlaceHolder("City name")
	addRegionEntry := widget.NewEntry()
	addRegionEntry.SetPlaceHolder("Region (e.g. SP)")
	addLatEntry := widget.NewEntry()
	addLatEntry.SetPlaceHolder("Latitude (optional)")
	addLonEntry := widget.NewEntry()
	addLonEntry.SetPlaceHolder("Longitude (optional)")
	addTimezoneEntry := widget.NewEntry()
	addTimezoneEntry.SetPlaceHolder("Timezone (e.g. America/Sao_Paulo)")

	addBtn := widget.NewButton("Add City", func() {
		name := strings.TrimSpace(addNameEntry.Text)
		if name == "" {
			dialog.ShowError(fmt.Errorf("city name is required"), win)
			return
		}
		newCity := config.CityConfig{
			Name:     name,
			Region:   strings.TrimSpace(addRegionEntry.Text),
			Timezone: strings.TrimSpace(addTimezoneEntry.Text),
		}
		if lat := strings.TrimSpace(addLatEntry.Text); lat != "" {
			v, err := strconv.ParseFloat(lat, 64)
			if err != nil {
				dialog.ShowError(fmt.Errorf("invalid latitude: %w", err), win)
				return
			}
			newCity.Latitude = v
		}
		if lon := strings.TrimSpace(addLonEntry.Text); lon != "" {
			v, err := strconv.ParseFloat(lon, 64)
			if err != nil {
				dialog.ShowError(fmt.Errorf("invalid longitude: %w", err), win)
				return
			}
			newCity.Longitude = v
		}
		result, err := config.AddCity(state.cities, newCity)
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		state.cities = result
		addNameEntry.SetText("")
		addRegionEntry.SetText("")
		addLatEntry.SetText("")
		addLonEntry.SetText("")
		addTimezoneEntry.SetText("")
		refreshCityList()
	})

	addForm := widget.NewForm(
		widget.NewFormItem("Name", addNameEntry),
		widget.NewFormItem("Region", addRegionEntry),
		widget.NewFormItem("Latitude", addLatEntry),
		widget.NewFormItem("Longitude", addLonEntry),
		widget.NewFormItem("Timezone", addTimezoneEntry),
	)

	citySection := container.NewVBox(
		widget.NewLabel("City List (1–3 cities)"),
		cityListContainer,
		widget.NewSeparator(),
		addForm,
		addBtn,
	)

	// --- Refresh Interval ---
	intervalSlider := widget.NewSlider(1, 60)
	intervalSlider.Step = 1
	intervalSlider.Value = float64(cfg.RefreshInterval)
	intervalLabel := widget.NewLabel(fmt.Sprintf("%d min", cfg.RefreshInterval))
	intervalSlider.OnChanged = func(v float64) {
		intervalLabel.SetText(fmt.Sprintf("%d min", int(v)))
	}

	refreshSection := container.NewVBox(
		widget.NewLabel("Refresh Interval"),
		container.NewHBox(intervalSlider, intervalLabel),
	)

	// --- Status / Error label ---
	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord

	// --- Save Button ---
	saveBtn := widget.NewButton("Save", func() {
		newCfg := buildConfigFromUI(
			dataSourceRadio, providerSelect, apiKeyEntry,
			dbHostEntry, dbPortEntry, dbNameEntry, dbUserEntry, dbPassEntry, dbQueryEntry,
			intervalSlider, state, cfg.CornerPosition,
		)

		errs := config.Validate(newCfg)
		if len(errs) > 0 {
			var msgs []string
			for _, e := range errs {
				msgs = append(msgs, e.Field+": "+e.Message)
			}
			statusLabel.SetText("Validation errors:\n" + strings.Join(msgs, "\n"))
			return
		}

		statusLabel.SetText("Saving and testing connection...")
		if err := onSave(newCfg); err != nil {
			statusLabel.SetText("Save failed: " + err.Error())
			dialog.ShowError(fmt.Errorf("connection test failed: %w", err), win)
			return
		}

		statusLabel.SetText("Settings saved successfully!")
		dialog.ShowInformation("Success", "Settings saved and connection verified.", win)
	})

	// --- Assemble tabs ---
	content := container.NewVBox(
		dataSourceSection,
		widget.NewSeparator(),
		citySection,
		widget.NewSeparator(),
		refreshSection,
		widget.NewSeparator(),
		saveBtn,
		statusLabel,
	)

	scrollable := container.NewVScroll(content)
	win.SetContent(scrollable)

	win.SetOnClosed(func() {
		u.settings = nil
	})

	win.Show()
}

// buildConfigFromUI reads all form fields and constructs a Config struct.
func buildConfigFromUI(
	dataSourceRadio *widget.RadioGroup,
	providerSelect *widget.Select,
	apiKeyEntry *widget.Entry,
	dbHostEntry, dbPortEntry, dbNameEntry, dbUserEntry, dbPassEntry, dbQueryEntry *widget.Entry,
	intervalSlider *widget.Slider,
	state *settingsState,
	cornerPosition string,
) *config.Config {
	cfg := &config.Config{
		Cities:          copyCities(state.cities),
		RefreshInterval: int(intervalSlider.Value),
		CornerPosition:  cornerPosition,
	}

	if dataSourceRadio.Selected == "Local Database" {
		cfg.DataSource = config.DataSourceLocalDatabase
		port, _ := strconv.Atoi(strings.TrimSpace(dbPortEntry.Text))
		cfg.DatabaseConfig = &config.DatabaseConfig{
			Host:     strings.TrimSpace(dbHostEntry.Text),
			Port:     port,
			DBName:   strings.TrimSpace(dbNameEntry.Text),
			Username: strings.TrimSpace(dbUserEntry.Text),
			Password: dbPassEntry.Text,
			Query:    strings.TrimSpace(dbQueryEntry.Text),
		}
	} else {
		cfg.DataSource = config.DataSourceRemoteAPI
		provider := "openweathermap"
		if providerSelect.Selected == "Weather Underground" {
			provider = "weatherunderground"
		}
		cfg.APIConfig = &config.APIConfig{
			Provider: provider,
			APIKey:   strings.TrimSpace(apiKeyEntry.Text),
		}
	}

	return cfg
}

// copyCities returns a deep copy of a city slice.
func copyCities(cities []config.CityConfig) []config.CityConfig {
	result := make([]config.CityConfig, len(cities))
	copy(result, cities)
	return result
}
