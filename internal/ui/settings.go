package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/bradfitz/latlong"

	"weatherwidget/internal/config"
)

// providerDisplayToValue maps UI display strings to internal config values.
var providerDisplayToValue = map[string]string{
	"OpenWeatherMap (Free)":   "openweathermap",
	"EasyWeatherWidget (Pro)": "easyweatherwidget",
}

// providerValueToDisplay maps internal config values to UI display strings.
var providerValueToDisplay = map[string]string{
	"openweathermap":    "OpenWeatherMap (Free)",
	"easyweatherwidget": "EasyWeatherWidget (Pro)",
}

// settingsState holds mutable UI state for the settings dialog.
type settingsState struct {
	cities []config.CityConfig
	window fyne.Window
}

// ShowSettings opens a settings dialog window.
func (u *UIManager) ShowSettings(cfg *config.Config, onSave func(*config.Config) error) {
	if u.settings != nil {
		u.settings.RequestFocus()
		return
	}

	win := u.app.NewWindow("WeatherWidget Settings")
	u.settings = win
	win.SetFixedSize(false)

	screenW, screenH := getScreenSize()
	winW := float32(1456)
	if float32(screenW) < winW {
		winW = float32(screenW) * 0.9
	}
	winH := float32(screenH) * 0.40
	win.Resize(fyne.NewSize(winW, winH))

	state := &settingsState{
		cities: copyCities(cfg.Cities),
		window: win,
	}

	// ── Data Source ──────────────────────────────────────────────────────
	dataSourceRadio := widget.NewRadioGroup([]string{"Remote API", "Local Database"}, nil)
	if cfg.DataSource == config.DataSourceLocalDatabase {
		dataSourceRadio.SetSelected("Local Database")
	} else {
		dataSourceRadio.SetSelected("Remote API")
	}

	// ── API config ───────────────────────────────────────────────────────
	providerSelect := widget.NewSelect([]string{"OpenWeatherMap (Free)", "EasyWeatherWidget (Pro)"}, nil)
	if cfg.APIConfig != nil && cfg.APIConfig.Provider != "" {
		if display, ok := providerValueToDisplay[cfg.APIConfig.Provider]; ok {
			providerSelect.SetSelected(display)
		} else {
			providerSelect.SetSelected("OpenWeatherMap (Free)")
		}
	} else {
		providerSelect.SetSelected("OpenWeatherMap (Free)")
	}
	apiKeyEntry := widget.NewEntry()
	apiKeyEntry.SetPlaceHolder("API Key")
	if cfg.APIConfig != nil {
		apiKeyEntry.SetText(cfg.APIConfig.APIKey)
	}
	apiSection := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Provider", providerSelect),
			widget.NewFormItem("API Key", apiKeyEntry),
		),
	)

	// ── Database config ──────────────────────────────────────────────────
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
	dbSection := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Host", dbHostEntry),
			widget.NewFormItem("Port", dbPortEntry),
			widget.NewFormItem("Database", dbNameEntry),
			widget.NewFormItem("Username", dbUserEntry),
			widget.NewFormItem("Password", dbPassEntry),
			widget.NewFormItem("Query", dbQueryEntry),
		),
	)

	apiSection.Show()
	dbSection.Hide()
	if cfg.DataSource == config.DataSourceLocalDatabase {
		apiSection.Hide()
		dbSection.Show()
	}
	dataSourceRadio.OnChanged = func(s string) {
		if s == "Local Database" {
			apiSection.Hide()
			dbSection.Show()
		} else {
			apiSection.Show()
			dbSection.Hide()
		}
	}

	// ── Position ─────────────────────────────────────────────────────────
	positionValueMap := map[string]string{
		"Top-Left": "top-left", "Top-Right": "top-right",
		"Bottom-Left": "bottom-left", "Bottom-Right": "bottom-right",
	}
	positionLabelMap := map[string]string{
		"top-left": "Top-Left", "top-right": "Top-Right",
		"bottom-left": "Bottom-Left", "bottom-right": "Bottom-Right",
	}
	positionRadio := widget.NewRadioGroup(
		[]string{"Top-Left", "Top-Right", "Bottom-Left", "Bottom-Right"}, nil,
	)
	positionRadio.Horizontal = true
	if label, ok := positionLabelMap[cfg.CornerPosition]; ok {
		positionRadio.SetSelected(label)
	} else {
		positionRadio.SetSelected("Bottom-Right")
	}
	customPosLabel := widget.NewLabel("")
	if cfg.CustomX != nil && cfg.CustomY != nil {
		customPosLabel.SetText(fmt.Sprintf("Custom: (%d, %d) — picking a corner clears it.", *cfg.CustomX, *cfg.CustomY))
	}
	positionRadio.OnChanged = func(_ string) { customPosLabel.SetText("") }

	// ── Transparency ─────────────────────────────────────────────────────
	opacityRadio := widget.NewRadioGroup([]string{"25%", "50%", "75%", "100%"}, nil)
	opacityRadio.Horizontal = true
	opacityMap := map[string]int{"25%": 25, "50%": 50, "75%": 75, "100%": 100}
	opacityLabelMap := map[int]string{25: "25%", 50: "50%", 75: "75%", 100: "100%"}
	currentOpacity := cfg.Opacity
	if currentOpacity == 0 {
		currentOpacity = 100
	}
	if label, ok := opacityLabelMap[currentOpacity]; ok {
		opacityRadio.SetSelected(label)
	} else {
		opacityRadio.SetSelected("100%")
	}

	// ── Refresh interval ─────────────────────────────────────────────────
	intervalSlider := widget.NewSlider(30, 120)
	intervalSlider.Step = 1
	intervalSlider.Value = float64(cfg.RefreshInterval)
	intervalLabel := widget.NewLabel(fmt.Sprintf("%d min", cfg.RefreshInterval))
	intervalSlider.OnChanged = func(v float64) {
		intervalLabel.SetText(fmt.Sprintf("%d min", int(v)))
	}

	// Apply initial slider constraints based on current provider.
	applyProviderSliderConstraints := func(providerValue string) {
		switch providerValue {
		case "openweathermap":
			intervalSlider.Min = 120
			intervalSlider.Max = 120
			intervalSlider.SetValue(120)
		case "easyweatherwidget":
			intervalSlider.Min = 30
			intervalSlider.Max = 120
			if intervalSlider.Value < 30 {
				intervalSlider.SetValue(30)
			}
		}
		intervalLabel.SetText(fmt.Sprintf("%d min", int(intervalSlider.Value)))
	}

	// Set initial constraints from current config.
	if cfg.APIConfig != nil && cfg.APIConfig.Provider != "" {
		applyProviderSliderConstraints(cfg.APIConfig.Provider)
	} else {
		applyProviderSliderConstraints("openweathermap")
	}

	// Track the initial provider so we can detect changes.
	initialProvider := "openweathermap"
	if cfg.APIConfig != nil && cfg.APIConfig.Provider != "" {
		initialProvider = cfg.APIConfig.Provider
	}

	// Provider change handler is set below, after refreshCityList is defined,
	// because switching providers clears the city list.
	providerSelect.OnChanged = func(selected string) {
		applyProviderSliderConstraints(providerDisplayToValue[selected])
	}

	// ── LEFT PANEL (scrollable) ───────────────────────────────────────────
	leftContent := container.NewVBox(
		widget.NewLabelWithStyle("System", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel("Data Source"),
		dataSourceRadio,
		apiSection,
		dbSection,
		widget.NewSeparator(),
		widget.NewLabel("Widget Position"),
		positionRadio,
		customPosLabel,
		widget.NewSeparator(),
		widget.NewLabel("Background Transparency"),
		opacityRadio,
		widget.NewSeparator(),
		widget.NewLabel("Refresh Interval"),
		container.NewHBox(intervalSlider, intervalLabel),
	)
	leftPanel := container.NewVScroll(leftContent)

	// ── RIGHT PANEL ───────────────────────────────────────────────────────
	// City list in a scroll area (top), add-form fixed below.

	cityListBox := container.NewVBox()
	var refreshCityList func()
	refreshCityList = func() {
		cityListBox.RemoveAll()
		for i := range state.cities {
			idx := i
			c := state.cities[i]
			lbl := widget.NewLabel(fmt.Sprintf("#%d  %s, %s", idx+1, c.Name, c.Region))
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
			cityListBox.Add(container.NewHBox(lbl, layout.NewSpacer(), upBtn, downBtn, removeBtn))
		}
		cityListBox.Refresh()
	}
	refreshCityList()

	// Now that refreshCityList is defined, upgrade the provider change handler
	// to also clear cities when the provider switches.
	providerSelect.OnChanged = func(selected string) {
		newProvider := providerDisplayToValue[selected]
		applyProviderSliderConstraints(newProvider)

		// Clear the city list when switching providers — each provider uses
		// its own search API, so existing cities may not be valid.
		if newProvider != initialProvider {
			state.cities = nil
			refreshCityList()
		}
	}

	cityListScroll := container.NewVScroll(cityListBox)
	cityListScroll.SetMinSize(fyne.NewSize(0, 120))

	// Add-city form (fixed, never scrolls away).
	addNameEntry := widget.NewEntry()
	addNameEntry.SetPlaceHolder("City name")
	addRegionEntry := widget.NewEntry()
	addRegionEntry.SetPlaceHolder("Region / Country (e.g. BR)")
	addLatEntry := widget.NewEntry()
	addLatEntry.SetPlaceHolder("Latitude (optional)")
	addLonEntry := widget.NewEntry()
	addLonEntry.SetPlaceHolder("Longitude (optional)")
	addTimezoneEntry := widget.NewEntry()
	addTimezoneEntry.SetPlaceHolder("Timezone (America/Sao_Paulo)")

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

	var searchBtn *widget.Button
	searchBtn = widget.NewButton("Search API", func() {
		name := strings.TrimSpace(addNameEntry.Text)
		if name == "" {
			dialog.ShowError(fmt.Errorf("city name is required to search"), win)
			return
		}

		apiKey := strings.TrimSpace(apiKeyEntry.Text)
		if apiKey == "" {
			dialog.ShowError(fmt.Errorf("API Key is required in the settings above to search"), win)
			return
		}

		currentProvider := providerDisplayToValue[providerSelect.Selected]

		searchBtn.SetText("Searching...")
		searchBtn.Disable()

		switch currentProvider {
		case "easyweatherwidget":
			region := strings.TrimSpace(addRegionEntry.Text)
			if region == "" {
				dialog.ShowError(fmt.Errorf("Region / Country is required to search via EasyWeatherWidget"), win)
				searchBtn.SetText("Search API")
				searchBtn.Enable()
				return
			}

			go func(searchName, searchRegion, searchKey string) {
				defer fyne.Do(func() {
					searchBtn.SetText("Search API")
					searchBtn.Enable()
				})

				// EWW uses the weather endpoint for city lookup:
				// https://easyweatherwidget.org:8043/api/v1/weather/key=API_KEY/City,Region
				query := searchName + "," + searchRegion
				uStr := fmt.Sprintf("https://easyweatherwidget.org:8043/api/v1/weather/key=%s/%s",
					url.PathEscape(searchKey), url.PathEscape(query))
				resp, err := http.Get(uStr)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("search failed: %v", err), win) })
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("search API error: %d", resp.StatusCode), win) })
					return
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("read error: %v", err), win) })
					return
				}

				var result struct {
					Neighborhood string  `json:"Neighborhood"`
					Country      string  `json:"Country"`
					Lat          float64 `json:"Lat"`
					Lon          float64 `json:"Lon"`
				}
				if err := json.Unmarshal(body, &result); err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("failed to parse search response: %v", err), win) })
					return
				}

				if result.Neighborhood == "" {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("no city found matching '%s'", searchName), win) })
					return
				}

				tz := latlong.LookupZoneName(result.Lat, result.Lon)

				fyne.Do(func() {
					addNameEntry.SetText(result.Neighborhood)
					addRegionEntry.SetText(result.Country)
					addLatEntry.SetText(fmt.Sprintf("%f", result.Lat))
					addLonEntry.SetText(fmt.Sprintf("%f", result.Lon))
					if tz != "" {
						addTimezoneEntry.SetText(tz)
					}
				})
			}(name, strings.TrimSpace(addRegionEntry.Text), apiKey)

		default: // openweathermap
			go func(searchName, searchKey string) {
				defer fyne.Do(func() {
					searchBtn.SetText("Search API")
					searchBtn.Enable()
				})

				uStr := fmt.Sprintf("http://api.openweathermap.org/geo/1.0/direct?q=%s&limit=1&appid=%s", url.QueryEscape(searchName), url.QueryEscape(searchKey))
				resp, err := http.Get(uStr)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("search failed: %v", err), win) })
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("search API error: %d", resp.StatusCode), win) })
					return
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("read error: %v", err), win) })
					return
				}

				var results []struct {
					Name    string  `json:"name"`
					Lat     float64 `json:"lat"`
					Lon     float64 `json:"lon"`
					Country string  `json:"country"`
					State   string  `json:"state"`
				}
				if err := json.Unmarshal(body, &results); err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("failed to parse search response: %v", err), win) })
					return
				}

				if len(results) == 0 {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("no city found matching '%s'", searchName), win) })
					return
				}

				res := results[0]
				region := res.Country
				if res.State != "" {
					region = res.State + ", " + region
				}
				tz := latlong.LookupZoneName(res.Lat, res.Lon)

				fyne.Do(func() {
					addNameEntry.SetText(res.Name)
					addRegionEntry.SetText(region)
					addLatEntry.SetText(fmt.Sprintf("%f", res.Lat))
					addLonEntry.SetText(fmt.Sprintf("%f", res.Lon))
					if tz != "" {
						addTimezoneEntry.SetText(tz)
					}
				})
			}(name, apiKey)
		}
	})

	nameItemContent := container.NewBorder(nil, nil, nil, searchBtn, addNameEntry)

	addForm := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Add City", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Name", nameItemContent),
			widget.NewFormItem("Region", addRegionEntry),
			widget.NewFormItem("Latitude", addLatEntry),
			widget.NewFormItem("Longitude", addLonEntry),
			widget.NewFormItem("Timezone", addTimezoneEntry),
		),
		addBtn,
	)

	// Right panel: city list scrolls, add form is fixed at bottom.
	rightPanel := container.NewBorder(
		widget.NewLabelWithStyle("City List (1–5 cities)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		addForm,
		nil, nil,
		cityListScroll,
	)

	// ── SAVE BAR (full width, pinned at bottom) ───────────────────────────
	saveBtn := widget.NewButton("Save", func() {
		newCfg := buildConfigFromUI(
			dataSourceRadio, providerSelect, apiKeyEntry,
			dbHostEntry, dbPortEntry, dbNameEntry, dbUserEntry, dbPassEntry, dbQueryEntry,
			intervalSlider, state, positionValueMap, positionRadio, opacityRadio, opacityMap, cfg,
		)
		errs := config.Validate(newCfg)
		if len(errs) > 0 {
			var msgs []string
			for _, e := range errs {
				msgs = append(msgs, e.Field+": "+e.Message)
			}
			dialog.ShowError(fmt.Errorf("%s", strings.Join(msgs, "\n")), win)
			return
		}
		if err := onSave(newCfg); err != nil {
			dialog.ShowError(err, win)
			return
		}
		dialog.ShowInformation("Saved", "Settings saved successfully!", win)
	})

	saveBar := container.NewBorder(
		widget.NewSeparator(), nil, nil, nil,
		saveBtn,
	)

	// ── MAIN LAYOUT ───────────────────────────────────────────────────────
	// Left panel fixed on the left, right panel fills remaining space,
	// save bar pinned at the very bottom.
	body := container.NewHSplit(leftPanel, rightPanel)
	body.SetOffset(0.45)

	win.SetContent(container.NewBorder(nil, saveBar, nil, nil, body))

	win.SetOnClosed(func() { u.settings = nil })
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
	positionValueMap map[string]string,
	positionRadio *widget.RadioGroup,
	opacityRadio *widget.RadioGroup,
	opacityMap map[string]int,
	current *config.Config,
) *config.Config {
	cornerPosition := positionValueMap[positionRadio.Selected]
	if cornerPosition == "" {
		cornerPosition = "bottom-right"
	}
	opacity := opacityMap[opacityRadio.Selected]
	if opacity == 0 {
		opacity = 100
	}
	cfg := &config.Config{
		Cities:          copyCities(state.cities),
		RefreshInterval: int(intervalSlider.Value),
		CornerPosition:  cornerPosition,
		CustomX:         current.CustomX,
		CustomY:         current.CustomY,
		Opacity:         opacity,
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
		provider := providerDisplayToValue[providerSelect.Selected]
		if provider == "" {
			provider = "openweathermap"
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
