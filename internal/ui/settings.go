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
	"weatherwidget/internal/i18n"
)

// ewwAPIKeyLen is the expected length of an EasyWeatherWidget API key (UUID v4).
const ewwAPIKeyLen = 36

// owmAPIKeyLen is the expected length of an OpenWeatherMap API key.
const owmAPIKeyLen = 33

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
	cities       []config.CityConfig
	window       fyne.Window
	selectedLang string                 // locale code selected in the language dropdown
	selectedUnit config.TemperatureUnit // temperature unit selected in the appearance tab
}

// t is a helper that returns the translated string for the given key.
// If the UIManager has no LocaleManager, it returns the key itself.
func (u *UIManager) t(key string) string {
	if u.lm != nil {
		return u.lm.T(key)
	}
	return key
}

// tFmt is a helper that returns a translated string with fmt.Sprintf-style arguments.
// It looks up the template via T(key) and then formats it with the given args.
func (u *UIManager) tFmt(key string, args ...interface{}) string {
	tmpl := u.t(key)
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

// ShowSettings opens a settings dialog window.
func (u *UIManager) ShowSettings(cfg *config.Config, onSave func(*config.Config) error) {
	if u.settings != nil {
		u.settings.RequestFocus()
		return
	}

	win := u.app.NewWindow(u.t("settings.title"))
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
		cities:       copyCities(cfg.Cities),
		window:       win,
		selectedLang: cfg.Locale,
		selectedUnit: config.NormalizeTemperatureUnit(cfg.TemperatureUnit),
	}
	if state.selectedLang == "" {
		state.selectedLang = "en-GB"
	}

	// buildSettingsUI constructs the full settings UI content.
	// It is called initially and again when the language changes to rebuild
	// all labels with the new locale.
	var buildSettingsUI func()
	buildSettingsUI = func() {
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
		apiKeyEntry.SetPlaceHolder(u.t("settings.provider.apiKeyPlaceholder"))
		if cfg.APIConfig != nil {
			apiKeyEntry.SetText(cfg.APIConfig.APIKey)
		}

		// Activation card — shown when a Pro API key is active or just generated.
		// Uses a Card with bold title, monospace key, and descriptive message
		// for a polished look that stands out from the plain note above.
		activationKeyText := widget.NewRichTextFromMarkdown("")
		activationMsg := widget.NewLabel("")
		activationMsg.Wrapping = fyne.TextWrapWord
		activationCard := widget.NewCard("", "", container.NewVBox(
			activationKeyText,
			activationMsg,
		))
		activationCard.Hide()

		// showActivationCard populates and reveals the activation card.
		showActivationCard := func(key string) {
			activationCard.SetTitle(u.t("settings.provider.apiKeyActivation.title"))
			activationKeyText.ParseMarkdown(fmt.Sprintf("`%s`", key))
			activationMsg.SetText(u.t("settings.provider.apiKeyActivation.message"))
			activationCard.Show()
		}

		// If the config already has an EWW-length key, show the activation card on load.
		if cfg.APIConfig != nil && cfg.APIConfig.Provider == "easyweatherwidget" && len(cfg.APIConfig.APIKey) == ewwAPIKeyLen {
			showActivationCard(cfg.APIConfig.APIKey)
		}

		// handleGetProAPI generates a new client reference ID, persists it as
		// the EWW API key in config.json, updates the UI entry and activation
		// label, then opens the Stripe payment link.
		handleGetProAPI := func() {
			currentKey := strings.TrimSpace(apiKeyEntry.Text)

			// If the key is already a 36-char EWW UUID, reuse it instead of
			// generating a new one — the user already has a key.
			clientRef := currentKey
			if len(currentKey) != ewwAPIKeyLen {
				clientRef = generateClientReferenceID()

				// Persist provider + new key to config.json immediately.
				if cfg.APIConfig == nil {
					cfg.APIConfig = &config.APIConfig{}
				}
				cfg.APIConfig.Provider = "easyweatherwidget"
				cfg.APIConfig.APIKey = clientRef

				// Update the UI entry so the user sees the new key.
				apiKeyEntry.SetText(clientRef)
			}

			// Show the activation card.
			showActivationCard(clientRef)

			parsedURL, _ := url.Parse("https://buy.stripe.com/bJe3cvaJOa650fQ8cPdZ603?client_reference_id=" + clientRef)
			_ = u.app.OpenURL(parsedURL)
		}

		getApiBtn := widget.NewButton(u.t("settings.provider.getFreeApi"), func() {
			parsedURL, _ := url.Parse("https://openweathermap.org/")
			_ = u.app.OpenURL(parsedURL)
		})
		if providerSelect.Selected == "EasyWeatherWidget (Pro)" {
			getApiBtn.SetText(u.t("settings.provider.getProApi"))
			getApiBtn.OnTapped = handleGetProAPI
		}

		noteLabel := widget.NewLabel(u.t("settings.provider.note"))
		noteLabel.Wrapping = fyne.TextWrapWord

		apiSection := container.NewVBox(
			widget.NewForm(
				widget.NewFormItem(u.t("settings.provider.label"), container.NewBorder(nil, nil, nil, getApiBtn, providerSelect)),
				widget.NewFormItem(u.t("settings.provider.apiKeyLabel"), apiKeyEntry),
			),
			noteLabel,
			activationCard,
		)

		// ── Position ─────────────────────────────────────────────────────────
		topLeftLabel := u.t("settings.position.topLeft")
		topRightLabel := u.t("settings.position.topRight")
		bottomLeftLabel := u.t("settings.position.bottomLeft")
		bottomRightLabel := u.t("settings.position.bottomRight")

		positionValueMap := map[string]string{
			topLeftLabel: "top-left", topRightLabel: "top-right",
			bottomLeftLabel: "bottom-left", bottomRightLabel: "bottom-right",
		}
		positionLabelMap := map[string]string{
			"top-left": topLeftLabel, "top-right": topRightLabel,
			"bottom-left": bottomLeftLabel, "bottom-right": bottomRightLabel,
		}
		positionRadio := widget.NewRadioGroup(
			[]string{topLeftLabel, topRightLabel, bottomLeftLabel, bottomRightLabel}, nil,
		)
		positionRadio.Horizontal = true
		if label, ok := positionLabelMap[cfg.CornerPosition]; ok {
			positionRadio.SetSelected(label)
		} else {
			positionRadio.SetSelected(bottomRightLabel)
		}

		// ── Monitor selector ─────────────────────────────────────────────────
		monitorCount := u.GetMonitorCount()
		var monitorOptions []string
		for i := 0; i < monitorCount; i++ {
			monitorOptions = append(monitorOptions, u.tFmt("settings.monitor.format", i+1))
		}
		monitorSelect := widget.NewSelect(monitorOptions, nil)
		if cfg.MonitorIndex >= 0 && cfg.MonitorIndex < monitorCount {
			monitorSelect.SetSelected(u.tFmt("settings.monitor.format", cfg.MonitorIndex+1))
		} else {
			monitorSelect.SetSelected(u.tFmt("settings.monitor.format", 1))
		}
		// Build the position card contents — include monitor selector only
		// when multiple monitors are detected.
		positionItems := []fyne.CanvasObject{positionRadio}
		if monitorCount > 1 {
			monitorLabel := widget.NewLabel(u.t("settings.monitor.label"))
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

		// ── Temperature unit ─────────────────────────────────────────────────
		unitCelsiusLabel := "°C (Celsius)"
		unitFahrenheitLabel := "°F (Fahrenheit)"

		unitValueMap := map[string]config.TemperatureUnit{
			unitCelsiusLabel:    config.TemperatureUnitCelsius,
			unitFahrenheitLabel: config.TemperatureUnitFahrenheit,
		}
		unitLabelMap := map[config.TemperatureUnit]string{
			config.TemperatureUnitCelsius:    unitCelsiusLabel,
			config.TemperatureUnitFahrenheit: unitFahrenheitLabel,
		}

		unitRadio := widget.NewRadioGroup(
			[]string{unitCelsiusLabel, unitFahrenheitLabel},
			func(selected string) {
				state.selectedUnit = unitValueMap[selected]
			},
		)
		unitRadio.Horizontal = true

		// Pre-select from current config, defaulting to Celsius.
		normalizedUnit := config.NormalizeTemperatureUnit(cfg.TemperatureUnit)
		if label, ok := unitLabelMap[normalizedUnit]; ok {
			unitRadio.SetSelected(label)
		} else {
			unitRadio.SetSelected(unitCelsiusLabel)
		}

		// ── Refresh interval ─────────────────────────────────────────────────
		intervalSlider := widget.NewSlider(10, 120)
		intervalSlider.Step = 1
		intervalSlider.Value = float64(cfg.RefreshInterval)
		intervalLabel := widget.NewLabel(u.tFmt("settings.interval.format", cfg.RefreshInterval))
		intervalSlider.OnChanged = func(v float64) {
			intervalLabel.SetText(u.tFmt("settings.interval.format", int(v)))
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
			intervalLabel.SetText(u.tFmt("settings.interval.format", int(intervalSlider.Value)))
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
				removeBtn := widget.NewButton(u.t("settings.locations.removeBtn"), func() {
					result, err := config.RemoveCity(state.cities, idx, nil)
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
				getApiBtn.SetText(u.t("settings.provider.getFreeApi"))
				getApiBtn.OnTapped = func() {
					parsedURL, _ := url.Parse("https://openweathermap.org/")
					_ = u.app.OpenURL(parsedURL)
				}
				activationCard.Hide()
			} else {
				getApiBtn.SetText(u.t("settings.provider.getProApi"))
				getApiBtn.OnTapped = handleGetProAPI

				// If the current key is already an EWW-length key, show the activation card.
				currentKey := strings.TrimSpace(apiKeyEntry.Text)
				if len(currentKey) == ewwAPIKeyLen {
					showActivationCard(currentKey)
				} else {
					activationCard.Hide()
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
		addNameEntry.SetPlaceHolder(u.t("settings.locations.namePlaceholder"))
		addRegionEntry := widget.NewEntry()
		addRegionEntry.SetPlaceHolder(u.t("settings.locations.regionPlaceholder"))
		addLatEntry := widget.NewEntry()
		addLatEntry.SetPlaceHolder(u.t("settings.locations.latPlaceholder"))
		addLonEntry := widget.NewEntry()
		addLonEntry.SetPlaceHolder(u.t("settings.locations.lonPlaceholder"))
		addTimezoneEntry := widget.NewEntry()
		addTimezoneEntry.SetPlaceHolder(u.t("settings.locations.tzPlaceholder"))

		addBtn := widget.NewButton(u.t("settings.locations.addBtn"), func() {
			name := strings.TrimSpace(addNameEntry.Text)
			if name == "" {
				dialog.ShowError(fmt.Errorf("%s", u.t("error.settings.cityNameRequired")), win)
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
					dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.invalidLat", err)), win)
					return
				}
				newCity.Latitude = v
			}
			if lon := strings.TrimSpace(addLonEntry.Text); lon != "" {
				v, err := strconv.ParseFloat(lon, 64)
				if err != nil {
					dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.invalidLon", err)), win)
					return
				}
				newCity.Longitude = v
			}
			result, err := config.AddCity(state.cities, newCity, nil)
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
		searchBtn = widget.NewButton(u.t("settings.locations.searchBtn"), func() {
			name := strings.TrimSpace(addNameEntry.Text)
			if name == "" {
				dialog.ShowError(fmt.Errorf("%s", u.t("error.settings.cityNameRequiredSearch")), win)
				return
			}

			apiKey := strings.TrimSpace(apiKeyEntry.Text)
			if apiKey == "" {
				dialog.ShowError(fmt.Errorf("%s", u.t("error.settings.apiKeyRequired")), win)
				return
			}

			currentProvider := providerDisplayToValue[providerSelect.Selected]

			searchBtn.SetText(u.t("settings.locations.searching"))
			searchBtn.Disable()

			switch currentProvider {
			case "easyweatherwidget":
				region := strings.TrimSpace(addRegionEntry.Text)
				if region == "" {
					dialog.ShowError(fmt.Errorf("%s", u.t("error.settings.regionRequiredEww")), win)
					searchBtn.SetText(u.t("settings.locations.searchBtn"))
					searchBtn.Enable()
					return
				}

				go func(searchName, searchRegion, searchKey string) {
					defer fyne.Do(func() {
						searchBtn.SetText(u.t("settings.locations.searchBtn"))
						searchBtn.Enable()
					})

					query := searchName + "," + searchRegion
					uStr := fmt.Sprintf("https://easyweatherwidget.org:8043/api/v1/weather/key=%s/%s",
						url.PathEscape(searchKey), url.PathEscape(query))
					resp, err := http.Get(uStr)
					if err != nil {
						fyne.Do(func() { dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.searchFailed", err)), win) })
						return
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						fyne.Do(func() {
							dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.searchApiError", resp.StatusCode)), win)
						})
						return
					}

					body, err := io.ReadAll(resp.Body)
					if err != nil {
						fyne.Do(func() { dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.readError", err)), win) })
						return
					}

					var result struct {
						Neighborhood string  `json:"Neighborhood"`
						Country      string  `json:"Country"`
						Lat          float64 `json:"Lat"`
						Lon          float64 `json:"Lon"`
					}
					if err := json.Unmarshal(body, &result); err != nil {
						fyne.Do(func() { dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.parseFailed", err)), win) })
						return
					}

					if result.Neighborhood == "" {
						fyne.Do(func() {
							dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.noCityFound", searchName)), win)
						})
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
						searchBtn.SetText(u.t("settings.locations.searchBtn"))
						searchBtn.Enable()
					})

					uStr := fmt.Sprintf("http://api.openweathermap.org/geo/1.0/direct?q=%s&limit=1&appid=%s", url.QueryEscape(searchName), url.QueryEscape(searchKey))
					resp, err := http.Get(uStr)
					if err != nil {
						fyne.Do(func() { dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.searchFailed", err)), win) })
						return
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						fyne.Do(func() {
							dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.searchApiError", resp.StatusCode)), win)
						})
						return
					}

					body, err := io.ReadAll(resp.Body)
					if err != nil {
						fyne.Do(func() { dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.readError", err)), win) })
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
						fyne.Do(func() { dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.parseFailed", err)), win) })
						return
					}

					if len(results) == 0 {
						fyne.Do(func() {
							dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.noCityFound", searchName)), win)
						})
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
				widget.NewFormItem(u.t("settings.locations.nameLabel"), nameItemContent),
				widget.NewFormItem(u.t("settings.locations.regionLabel"), addRegionEntry),
				widget.NewFormItem(u.t("settings.locations.latLabel"), addLatEntry),
				widget.NewFormItem(u.t("settings.locations.lonLabel"), addLonEntry),
				widget.NewFormItem(u.t("settings.locations.tzLabel"), addTimezoneEntry),
			),
			container.NewHBox(layout.NewSpacer(), addBtnContainer),
		)

		// ── Auto-start toggle ────────────────────────────────────────────────
		autoStartCheck := widget.NewCheck(u.t("settings.startup.autostart"), nil)
		autoStartCheck.SetChecked(isAutoStartEnabled())
		autoStartCheck.OnChanged = func(checked bool) {
			if err := setAutoStartEnabled(checked); err != nil {
				dialog.ShowError(fmt.Errorf("%s", u.tFmt("error.settings.autoStartFailed", err)), win)
				// Revert the checkbox to the actual state.
				autoStartCheck.SetChecked(isAutoStartEnabled())
			}
		}

		// ── Language selector ────────────────────────────────────────────────
		var langSelect *widget.Select
		var langLocales []i18n.LocaleInfo
		if u.lm != nil {
			langLocales = u.lm.AvailableLocales()
		}
		var langDisplayNames []string
		langDisplayToCode := make(map[string]string)
		langCodeToDisplay := make(map[string]string)
		for _, loc := range langLocales {
			langDisplayNames = append(langDisplayNames, loc.DisplayName)
			langDisplayToCode[loc.DisplayName] = loc.Code
			langCodeToDisplay[loc.Code] = loc.DisplayName
		}
		langSelect = widget.NewSelect(langDisplayNames, func(selected string) {
			code := langDisplayToCode[selected]
			if code == "" || code == state.selectedLang {
				return
			}
			state.selectedLang = code
			if u.lm != nil {
				_ = u.lm.SetLocale(code)
			}
			// Rebuild the settings UI with the new locale.
			win.SetTitle(u.t("settings.title"))
			buildSettingsUI()
		})
		if display, ok := langCodeToDisplay[state.selectedLang]; ok {
			langSelect.SetSelected(display)
		}

		// ── Tabs Assembly ─────────────────────────────────────────────────────

		// sectionBlock builds a consistent section with a bold title, a muted
		// subtitle, and the content below.
		sectionBlock := func(title, subtitle string, content fyne.CanvasObject) *fyne.Container {
			titleLabel := widget.NewLabel(title)
			titleLabel.TextStyle = fyne.TextStyle{Bold: true}
			subtitleLabel := widget.NewLabel(subtitle)
			subtitleLabel.TextStyle = fyne.TextStyle{Italic: true}
			return container.NewVBox(titleLabel, subtitleLabel, content, widget.NewSeparator())
		}

		appearanceContent := container.NewPadded(container.NewVScroll(container.NewVBox(
			sectionBlock(u.t("settings.position.title"), u.t("settings.position.subtitle"),
				container.NewVBox(positionItems...),
			),
			sectionBlock(u.t("settings.transparency.title"), u.t("settings.transparency.subtitle"),
				opacityRadio,
			),
			sectionBlock(u.t("settings.temperature.title"), u.t("settings.temperature.subtitle"),
				unitRadio,
			),
			sectionBlock(u.t("settings.interval.title"), u.t("settings.interval.subtitle"),
				container.NewHBox(intervalSlider, intervalLabel),
			),
			sectionBlock(u.t("settings.startup.title"), u.t("settings.startup.subtitle"),
				autoStartCheck,
			),
		)))
		appearanceTab := container.NewTabItemWithIcon(u.t("settings.tab.appearance"), theme.ColorPaletteIcon(), appearanceContent)

		// ── Language tab ──────────────────────────────────────────────────────
		languageContent := container.NewPadded(container.NewVScroll(container.NewVBox(
			sectionBlock(u.t("settings.language.title"), u.t("settings.language.subtitle"),
				langSelect,
			),
		)))
		languageTab := container.NewTabItemWithIcon(u.t("settings.tab.language"), theme.ComputerIcon(), languageContent)

		providerContent := container.NewPadded(container.NewVScroll(container.NewVBox(
			widget.NewCard(u.t("settings.provider.title"), u.t("settings.provider.subtitle"),
				apiSection,
			),
		)))
		providerTab := container.NewTabItemWithIcon(u.t("settings.tab.provider"), theme.SettingsIcon(), providerContent)

		locationsContent := container.NewPadded(
			container.NewBorder(
				nil,
				widget.NewCard(u.t("settings.locations.addTitle"), "", addForm),
				nil,
				nil,
				widget.NewCard(u.t("settings.locations.savedTitle"), u.t("settings.locations.savedSubtitle"), cityListScroll),
			),
		)
		locationsTab := container.NewTabItemWithIcon(u.t("settings.tab.locations"), theme.ListIcon(), locationsContent)

		// ── About tab ─────────────────────────────────────────────────────────
		aboutVersion := widget.NewRichTextFromMarkdown(u.t("settings.about.version"))
		aboutDesc := widget.NewLabel(u.t("settings.about.description"))
		aboutDesc.Wrapping = fyne.TextWrapWord

		websiteLink := widget.NewHyperlink("gcclinux.github.io/WeatherWidget", parseURL("https://gcclinux.github.io/WeatherWidget/"))

		demoResource := fyne.NewStaticResource("demo.png", assets.DemoPNG)
		demoImage := canvas.NewImageFromResource(demoResource)
		demoImage.FillMode = canvas.ImageFillContain
		demoImage.SetMinSize(fyne.NewSize(400, 250))

		aboutContent := container.NewPadded(container.NewVScroll(container.NewVBox(
			widget.NewCard(u.t("settings.about.appName"), "", container.NewVBox(
				aboutDesc,
				container.NewHBox(widget.NewLabel(u.t("settings.about.websiteLabel")), websiteLink),
				aboutVersion,
			)),
			widget.NewCard(u.t("settings.about.previewLabel"), "", container.NewCenter(demoImage)),
		)))
		aboutTab := container.NewTabItemWithIcon(u.t("settings.tab.about"), theme.InfoIcon(), aboutContent)

		tabs := container.NewAppTabs(appearanceTab, providerTab, locationsTab, languageTab, aboutTab)
		tabs.SetTabLocation(container.TabLocationLeading)

		// ── SAVE BAR (full width, pinned at bottom) ───────────────────────────
		saveBtn := widget.NewButton(u.t("settings.save"), func() {
			newCfg := buildConfigFromUI(
				providerSelect, apiKeyEntry,
				intervalSlider, state, positionValueMap, positionRadio, monitorSelect, opacityRadio, opacityMap, cfg,
			)
			errs := config.Validate(newCfg, nil)
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
			dialog.ShowInformation(u.t("settings.dialog.saved"), u.t("settings.dialog.savedMsg"), win)
		})

		saveBtnSizer := canvas.NewRectangle(color.Transparent)
		saveBtnSizer.SetMinSize(fyne.NewSize(160, 0))
		saveBtnContainer := container.NewMax(saveBtnSizer, saveBtn)

		saveBar := container.NewBorder(
			widget.NewSeparator(), nil, nil, nil,
			container.NewPadded(container.NewPadded(container.NewHBox(layout.NewSpacer(), saveBtnContainer))),
		)

		win.SetContent(container.NewBorder(nil, saveBar, nil, nil, tabs))
	}

	buildSettingsUI()

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

	locale := state.selectedLang
	if locale == "" {
		locale = "en-GB"
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
		Locale:          locale,
	}
	cfg.TemperatureUnit = config.NormalizeTemperatureUnit(state.selectedUnit)
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
