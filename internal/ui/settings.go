package ui

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/bradfitz/latlong"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
)

// generateClientReferenceID creates a random UUID v4 string (e.g. "f169fecc-de0c-4de1-b8c8-4ec3701b671c")
// to be used as a unique client_reference_id for Stripe payment links.
func generateClientReferenceID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	// Set version 4 (bits 12-15 of time_hi_and_version).
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant bits (10xx for RFC 4122).
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

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
	winW := float32(900)
	if float32(screenW) < winW {
		winW = float32(screenW) * 0.9
	}
	winH := float32(screenH) * 0.46
	win.Resize(fyne.NewSize(winW, winH))

	state := &settingsState{
		cities: copyCities(cfg.Cities),
		window: win,
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

	getApiBtn := widget.NewButton("Get FREE API", func() {
		parsedURL, _ := url.Parse("https://openweathermap.org/")
		_ = u.app.OpenURL(parsedURL)
	})
	if providerSelect.Selected == "EasyWeatherWidget (Pro)" {
		getApiBtn.SetText("Get PRO API")
		getApiBtn.OnTapped = func() {
			clientRef := generateClientReferenceID()
			parsedURL, _ := url.Parse("https://buy.stripe.com/bJe3cvaJOa650fQ8cPdZ603?client_reference_id=" + clientRef)
			_ = u.app.OpenURL(parsedURL)
		}
	}

	apiKeyEntry := widget.NewEntry()
	apiKeyEntry.SetPlaceHolder("API Key")
	if cfg.APIConfig != nil {
		apiKeyEntry.SetText(cfg.APIConfig.APIKey)
	}

	noteLabel := widget.NewLabel("Note: \nFree = 120 minutes refresh rate (limited). \nPro = 10 minutes refresh rate (unlimited).")
	noteLabel.Wrapping = fyne.TextWrapWord

	apiSection := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Provider", container.NewBorder(nil, nil, nil, getApiBtn, providerSelect)),
			widget.NewFormItem("API Key", apiKeyEntry),
		),
		noteLabel,
	)

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

	// ── Monitor selector ─────────────────────────────────────────────────
	monitorCount := u.GetMonitorCount()
	var monitorOptions []string
	for i := 0; i < monitorCount; i++ {
		monitorOptions = append(monitorOptions, fmt.Sprintf("Monitor %d", i+1))
	}
	monitorSelect := widget.NewSelect(monitorOptions, nil)
	if cfg.MonitorIndex >= 0 && cfg.MonitorIndex < monitorCount {
		monitorSelect.SetSelected(fmt.Sprintf("Monitor %d", cfg.MonitorIndex+1))
	} else {
		monitorSelect.SetSelected("Monitor 1")
	}
	// Build the position card contents — include monitor selector only
	// when multiple monitors are detected.
	positionItems := []fyne.CanvasObject{positionRadio, customPosLabel}
	if monitorCount > 1 {
		monitorLabel := widget.NewLabel("Display Monitor")
		monitorLabel.TextStyle = fyne.TextStyle{Bold: true}
		positionItems = append(positionItems, monitorLabel, monitorSelect)
	}

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
	intervalSlider := widget.NewSlider(10, 120)
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
			intervalSlider.Min = 10
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

		if newProvider == "openweathermap" {
			getApiBtn.SetText("Get FREE API")
			getApiBtn.OnTapped = func() {
				parsedURL, _ := url.Parse("https://openweathermap.org/")
				_ = u.app.OpenURL(parsedURL)
			}
		} else {
			getApiBtn.SetText("Get PRO API")
			getApiBtn.OnTapped = func() {
				clientRef := generateClientReferenceID()
				parsedURL, _ := url.Parse("https://buy.stripe.com/bJe3cvaJOa650fQ8cPdZ603?client_reference_id=" + clientRef)
				_ = u.app.OpenURL(parsedURL)
			}
		}

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

	addBtnSizer := canvas.NewRectangle(color.Transparent)
	addBtnSizer.SetMinSize(fyne.NewSize(160, 0))
	addBtnContainer := container.NewMax(addBtnSizer, addBtn)

	addForm := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Name", nameItemContent),
			widget.NewFormItem("Region", addRegionEntry),
			widget.NewFormItem("Latitude", addLatEntry),
			widget.NewFormItem("Longitude", addLonEntry),
			widget.NewFormItem("Timezone", addTimezoneEntry),
		),
		container.NewHBox(layout.NewSpacer(), addBtnContainer),
	)

	// ── Auto-start toggle ────────────────────────────────────────────────
	autoStartCheck := widget.NewCheck("Launch WeatherWidget when Windows starts", nil)
	autoStartCheck.SetChecked(isAutoStartEnabled())
	autoStartCheck.OnChanged = func(checked bool) {
		if err := setAutoStartEnabled(checked); err != nil {
			dialog.ShowError(fmt.Errorf("failed to update auto-start: %w", err), win)
			// Revert the checkbox to the actual state.
			autoStartCheck.SetChecked(isAutoStartEnabled())
		}
	}

	// ── Tabs Assembly ─────────────────────────────────────────────────────
	appearanceContent := container.NewPadded(container.NewVScroll(container.NewVBox(
		widget.NewCard("Widget Position", "Where should the widget appear?",
			container.NewVBox(positionItems...),
		),
		widget.NewCard("Background Transparency", "Adjust how see-through the widget is.",
			opacityRadio,
		),
		widget.NewCard("Refresh Interval", "How often to fetch new data.",
			container.NewHBox(intervalSlider, intervalLabel),
		),
		widget.NewCard("Startup", "Start automatically when you log in.",
			autoStartCheck,
		),
	)))
	appearanceTab := container.NewTabItemWithIcon("Appearance", theme.ColorPaletteIcon(), appearanceContent)

	providerContent := container.NewPadded(container.NewVScroll(container.NewVBox(
		widget.NewCard("Data Provider & API Key", "Configure the weather data source.",
			apiSection,
		),
	)))
	providerTab := container.NewTabItemWithIcon("Data Provider", theme.SettingsIcon(), providerContent)

	locationsContent := container.NewPadded(
		container.NewBorder(
			nil,
			widget.NewCard("Add New City", "", addForm),
			nil,
			nil,
			widget.NewCard("Saved Cities", "Manage your tracked locations (1–5 cities).", cityListScroll),
		),
	)
	locationsTab := container.NewTabItemWithIcon("Locations", theme.ListIcon(), locationsContent)

	// ── About tab ─────────────────────────────────────────────────────────
	aboutVersion := widget.NewRichTextFromMarkdown("**Version:** 0.0.5")
	aboutDesc := widget.NewLabel("A compact, transparent time & weather widget that lives on your desktop.\nMonitor up to 5 cities at a glance with a beautiful, always-on-top overlay.")
	aboutDesc.Wrapping = fyne.TextWrapWord

	websiteLink := widget.NewHyperlink("gcclinux.github.io/WeatherWidget", parseURL("https://gcclinux.github.io/WeatherWidget/"))

	demoResource := fyne.NewStaticResource("demo.png", assets.DemoPNG)
	demoImage := canvas.NewImageFromResource(demoResource)
	demoImage.FillMode = canvas.ImageFillContain
	demoImage.SetMinSize(fyne.NewSize(400, 250))

	aboutContent := container.NewPadded(container.NewVScroll(container.NewVBox(
		widget.NewCard("WeatherWidget", "", container.NewVBox(
			aboutDesc,
			container.NewHBox(widget.NewLabel("Website:"), websiteLink),
			aboutVersion,
		)),
		widget.NewCard("Preview", "", container.NewCenter(demoImage)),
	)))
	aboutTab := container.NewTabItemWithIcon("About", theme.InfoIcon(), aboutContent)

	tabs := container.NewAppTabs(appearanceTab, providerTab, locationsTab, aboutTab)
	tabs.SetTabLocation(container.TabLocationLeading)

	// ── SAVE BAR (full width, pinned at bottom) ───────────────────────────
	saveBtn := widget.NewButton("Save", func() {
		newCfg := buildConfigFromUI(
			providerSelect, apiKeyEntry,
			intervalSlider, state, positionValueMap, positionRadio, monitorSelect, opacityRadio, opacityMap, cfg,
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

	saveBtnSizer := canvas.NewRectangle(color.Transparent)
	saveBtnSizer.SetMinSize(fyne.NewSize(160, 0))
	saveBtnContainer := container.NewMax(saveBtnSizer, saveBtn)

	saveBar := container.NewBorder(
		widget.NewSeparator(), nil, nil, nil,
		container.NewPadded(container.NewPadded(container.NewHBox(layout.NewSpacer(), saveBtnContainer))),
	)

	win.SetContent(container.NewBorder(nil, saveBar, nil, nil, tabs))

	win.SetOnClosed(func() { u.settings = nil })
	win.Show()
}

// buildConfigFromUI reads all form fields and constructs a Config struct.
func buildConfigFromUI(
	providerSelect *widget.Select,
	apiKeyEntry *widget.Entry,
	intervalSlider *widget.Slider,
	state *settingsState,
	positionValueMap map[string]string,
	positionRadio *widget.RadioGroup,
	monitorSelect *widget.Select,
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
	// When the user picks a corner position, clear any previous custom
	// drag coordinates so applyPosition uses SetCorner instead.
	var customX, customY *int
	if positionRadio.Selected == "" && current.CustomX != nil && current.CustomY != nil {
		// No corner selected — preserve existing custom coordinates.
		customX = current.CustomX
		customY = current.CustomY
	}

	// Parse monitor index from the "Monitor N" selection (0-based).
	monitorIndex := 0
	if monitorSelect.Selected != "" {
		fmt.Sscanf(monitorSelect.Selected, "Monitor %d", &monitorIndex)
		monitorIndex-- // convert 1-based display to 0-based index
		if monitorIndex < 0 {
			monitorIndex = 0
		}
	}

	cfg := &config.Config{
		DataSource:      config.DataSourceRemoteAPI,
		Cities:          copyCities(state.cities),
		RefreshInterval: int(intervalSlider.Value),
		CornerPosition:  cornerPosition,
		MonitorIndex:    monitorIndex,
		CustomX:         customX,
		CustomY:         customY,
		Opacity:         opacity,
	}
	provider := providerDisplayToValue[providerSelect.Selected]
	if provider == "" {
		provider = "openweathermap"
	}
	cfg.APIConfig = &config.APIConfig{
		Provider: provider,
		APIKey:   strings.TrimSpace(apiKeyEntry.Text),
	}
	return cfg
}

// copyCities returns a deep copy of a city slice.
func copyCities(cities []config.CityConfig) []config.CityConfig {
	result := make([]config.CityConfig, len(cities))
	copy(result, cities)
	return result
}

// parseURL is a convenience wrapper around url.Parse that discards the error.
func parseURL(rawURL string) *url.URL {
	u, _ := url.Parse(rawURL)
	return u
}
