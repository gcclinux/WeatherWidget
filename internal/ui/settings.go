package ui

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

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
	"weatherwidget/internal/ui/panel"
	"weatherwidget/internal/weather/remoteapi"
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
	cities           []config.CityConfig
	window           fyne.Window
	selectedLang     string                 // locale code selected in the language dropdown
	selectedUnit     config.TemperatureUnit // temperature unit selected in the appearance tab
	saved            bool                   // true if Save was clicked (suppresses revert on close)
	previewPanels    []*panel.CityPanel     // live preview panels in the About tab
	appearancePanels []*panel.CityPanel     // live preview panels in the Appearance tab
	displayFields    *config.DisplayFields  // current display field checkbox state
	customX          *int                   // pending custom X position
	customY          *int                   // pending custom Y position
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
	winW := float32(945)
	if float32(screenW) < winW {
		winW = float32(screenW) * 0.9
	}
	winH := float32(screenH) * 0.55
	if winH < 560 {
		winH = 560
	}
	if winH > float32(screenH)*0.9 {
		winH = float32(screenH) * 0.9
	}
	win.Resize(fyne.NewSize(winW, winH))

	state := &settingsState{
		cities:        copyCities(cfg.Cities),
		window:        win,
		selectedLang:  cfg.Locale,
		selectedUnit:  config.NormalizeTemperatureUnit(cfg.TemperatureUnit),
		displayFields: cfg.GetDisplayFields(),
		customX:       cfg.CustomX,
		customY:       cfg.CustomY,
	}
	if state.selectedLang == "" {
		state.selectedLang = "en-GB"
	}

	// Capture original values for live preview revert on close without save.
	origOpacity := cfg.Opacity
	if origOpacity == 0 {
		origOpacity = 100
	}
	origPosition := cfg.CornerPosition
	origMonitor := cfg.MonitorIndex
	origUnit := config.NormalizeTemperatureUnit(cfg.TemperatureUnit)
	origCustomX := cfg.CustomX
	origCustomY := cfg.CustomY

	// buildSettingsUI constructs the full settings UI content.
	// It is called initially and again when the language changes to rebuild
	// all labels with the new locale.
	selectedTabIndex := 0 // tracks the active tab across rebuilds
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

		// ── Refresh interval ─────────────────────────────────────────────────
		intervalSlider := widget.NewSlider(10, 120)
		intervalSlider.Step = 1
		intervalSlider.Value = float64(cfg.RefreshInterval)
		intervalLabel := widget.NewLabel(u.tFmt("settings.interval.format", cfg.RefreshInterval))
		intervalSlider.OnChanged = func(v float64) {
			intervalLabel.SetText(u.tFmt("settings.interval.format", int(v)))
		}

		// applyProviderSliderConstraints updates the slider range and, when
		// enforceFloor is true, also clamps the value to the provider minimum.
		// Pass enforceFloor=false on initial load so the saved config value is
		// preserved; pass enforceFloor=true when the user switches providers.
		applyProviderSliderConstraints := func(providerValue string, enforceFloor bool) {
			switch providerValue {
			case "openweathermap":
				intervalSlider.Min = 120
				intervalSlider.Max = 120
				intervalSlider.SetValue(120)
			case "easyweatherwidget":
				intervalSlider.Min = 10
				intervalSlider.Max = 120
				if enforceFloor && intervalSlider.Value < 30 {
					intervalSlider.SetValue(30)
				}
			}
			intervalLabel.SetText(u.tFmt("settings.interval.format", int(intervalSlider.Value)))
		}

		// Set initial constraints from current config — do NOT enforce the
		// floor so the value saved in config is shown as-is.
		if cfg.APIConfig != nil && cfg.APIConfig.Provider != "" {
			applyProviderSliderConstraints(cfg.APIConfig.Provider, false)
		} else {
			applyProviderSliderConstraints("openweathermap", false)
		}

		// Track the initial provider so we can detect changes.
		initialProvider := "openweathermap"
		if cfg.APIConfig != nil && cfg.APIConfig.Provider != "" {
			initialProvider = cfg.APIConfig.Provider
		}

		// Provider change handler is set below, after refreshCityList is defined,
		// because switching providers clears the city list.
		providerSelect.OnChanged = func(selected string) {
			applyProviderSliderConstraints(providerDisplayToValue[selected], true)
		}

		apiSection := container.NewVBox(
			widget.NewForm(
				widget.NewFormItem(u.t("settings.provider.label"), container.NewBorder(nil, nil, nil, getApiBtn, providerSelect)),
				widget.NewFormItem(u.t("settings.provider.apiKeyLabel"), apiKeyEntry),
				widget.NewFormItem(u.t("settings.interval.title"), container.NewHBox(intervalSlider, intervalLabel)),
			),
			noteLabel,
			activationCard,
		)

		// ── Position ─────────────────────────────────────────────────────────
		var positionValueMap map[string]string
		var positionRadio *widget.RadioGroup
		var monitorSelect *widget.Select
		var positionItems []fyne.CanvasObject

		var curX, curY int
		if state.customX != nil && state.customY != nil {
			curX, curY = *state.customX, *state.customY
		} else {
			curX, curY = u.GetPosition()
		}

		xEntry := widget.NewEntry()
		xEntry.SetText(strconv.Itoa(curX))

		yEntry := widget.NewEntry()
		yEntry.SetText(strconv.Itoa(curY))

		moveAndSaveCustom := func(x, y int) {
			u.SetPosition(x, y)
			cx, cy := x, y
			state.customX = &cx
			state.customY = &cy
			xEntry.SetText(strconv.Itoa(x))
			yEntry.SetText(strconv.Itoa(y))
		}

		applyPosBtn := widget.NewButton("Apply", func() {
			x, errX := strconv.Atoi(strings.TrimSpace(xEntry.Text))
			y, errY := strconv.Atoi(strings.TrimSpace(yEntry.Text))
			if errX == nil && errY == nil {
				moveAndSaveCustom(x, y)
			}
		})
		xEntry.OnSubmitted = func(_ string) { applyPosBtn.OnTapped() }
		yEntry.OnSubmitted = func(_ string) { applyPosBtn.OnTapped() }

		const nudge = 10
		leftBtn := widget.NewButton("◄", func() {
			x, errX := strconv.Atoi(strings.TrimSpace(xEntry.Text))
			y, errY := strconv.Atoi(strings.TrimSpace(yEntry.Text))
			if errX != nil || errY != nil {
				x, y = u.GetPosition()
			}
			moveAndSaveCustom(x-nudge, y)
		})
		upBtn := widget.NewButton("▲", func() {
			x, errX := strconv.Atoi(strings.TrimSpace(xEntry.Text))
			y, errY := strconv.Atoi(strings.TrimSpace(yEntry.Text))
			if errX != nil || errY != nil {
				x, y = u.GetPosition()
			}
			moveAndSaveCustom(x, y-nudge)
		})
		downBtn := widget.NewButton("▼", func() {
			x, errX := strconv.Atoi(strings.TrimSpace(xEntry.Text))
			y, errY := strconv.Atoi(strings.TrimSpace(yEntry.Text))
			if errX != nil || errY != nil {
				x, y = u.GetPosition()
			}
			moveAndSaveCustom(x, y+nudge)
		})
		rightBtn := widget.NewButton("►", func() {
			x, errX := strconv.Atoi(strings.TrimSpace(xEntry.Text))
			y, errY := strconv.Atoi(strings.TrimSpace(yEntry.Text))
			if errX != nil || errY != nil {
				x, y = u.GetPosition()
			}
			moveAndSaveCustom(x+nudge, y)
		})

		xEntrySizer := canvas.NewRectangle(color.Transparent)
		xEntrySizer.SetMinSize(fyne.NewSize(70, 0))
		xEntryContainer := container.NewMax(xEntrySizer, xEntry)

		yEntrySizer := canvas.NewRectangle(color.Transparent)
		yEntrySizer.SetMinSize(fyne.NewSize(70, 0))
		yEntryContainer := container.NewMax(yEntrySizer, yEntry)

		xIcon := widget.NewIcon(theme.ComputerIcon())
		yIcon := widget.NewIcon(theme.ComputerIcon())

		posRow := container.NewHBox(
			xIcon,
			widget.NewLabel("X:"),
			xEntryContainer,
			widget.NewLabel("  "),
			yIcon,
			widget.NewLabel("Y:"),
			yEntryContainer,
			widget.NewLabel("  "),
			leftBtn,
			upBtn,
			downBtn,
			rightBtn,
			widget.NewLabel("  "),
			applyPosBtn,
		)

		if runtime.GOOS == "linux" {
			noticeLabel := widget.NewLabel(u.t("settings.position.linuxNotice"))
			noticeLabel.Wrapping = fyne.TextWrapWord
			positionItems = []fyne.CanvasObject{noticeLabel, posRow}
		} else {
			topLeftLabel := u.t("settings.position.topLeft")
			topRightLabel := u.t("settings.position.topRight")
			bottomLeftLabel := u.t("settings.position.bottomLeft")
			bottomRightLabel := u.t("settings.position.bottomRight")

			positionValueMap = map[string]string{
				topLeftLabel: "top-left", topRightLabel: "top-right",
				bottomLeftLabel: "bottom-left", bottomRightLabel: "bottom-right",
			}
			positionLabelMap := map[string]string{
				"top-left": topLeftLabel, "top-right": topRightLabel,
				"bottom-left": bottomLeftLabel, "bottom-right": bottomRightLabel,
			}
			positionRadio = widget.NewRadioGroup(
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
			monitorSelect = widget.NewSelect(monitorOptions, nil)
			if cfg.MonitorIndex >= 0 && cfg.MonitorIndex < monitorCount {
				monitorSelect.SetSelected(u.tFmt("settings.monitor.format", cfg.MonitorIndex+1))
			} else {
				monitorSelect.SetSelected(u.tFmt("settings.monitor.format", 1))
			}

			// Live preview: move widget when position or monitor changes.
			applyPositionPreview := func() {
				pos := positionValueMap[positionRadio.Selected]
				if pos == "" {
					pos = "bottom-right"
				}
				monIdx := 0
				for i := 0; i < monitorCount; i++ {
					if monitorSelect.Selected == u.tFmt("settings.monitor.format", i+1) {
						monIdx = i
						break
					}
				}
				u.SetCorner(pos, monIdx)
				state.customX = nil
				state.customY = nil
				nx, ny := u.GetPosition()
				xEntry.SetText(strconv.Itoa(nx))
				yEntry.SetText(strconv.Itoa(ny))
			}
			positionRadio.OnChanged = func(_ string) { applyPositionPreview() }
			monitorSelect.OnChanged = func(_ string) { applyPositionPreview() }

			positionItems = []fyne.CanvasObject{positionRadio}
			if monitorCount > 1 {
				monitorLabel := widget.NewLabel(u.t("settings.monitor.label"))
				monitorLabel.TextStyle = fyne.TextStyle{Bold: true}
				positionItems = append(positionItems, monitorLabel, monitorSelect)
			}
			positionItems = append(positionItems, posRow)
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

		// Live preview: apply opacity change immediately to the widget.
		opacityRadio.OnChanged = func(selected string) {
			if val, ok := opacityMap[selected]; ok {
				u.SetOpacity(val)
			}
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
				// Live preview: re-render main widget panels with the new unit immediately.
				u.RerenderPanels(state.selectedUnit)
				// Also re-render the Widget tab preview panels.
				for _, p := range state.appearancePanels {
					p.Rerender(state.selectedUnit)
				}
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

		// ── RIGHT PANEL ───────────────────────────────────────────────────────
		// City list in a scroll area (top), add-form fixed below.

		cityListBox := container.NewVBox()
		var refreshCityList func()
		refreshCityList = func() {
			cityListBox.RemoveAll()
			hasLicense := cfg.HasLicense()
			for i := range state.cities {
				idx := i
				c := state.cities[i]

				// City name styled as bold text.
				cityName := widget.NewRichTextFromMarkdown(fmt.Sprintf("**%s, %s**", c.Name, c.Region))

				// Small weather icon instead of position number.
				iconCodes := []string{"clear", "partly_cloudy", "cloudy", "rain", "snow"}
				iconCode := iconCodes[idx%len(iconCodes)]
				iconData, _ := assets.Icons.ReadFile(fmt.Sprintf("icons/%s.png", iconCode))
				var cityIcon *canvas.Image
				if iconData != nil {
					res := fyne.NewStaticResource(iconCode+".png", iconData)
					cityIcon = canvas.NewImageFromResource(res)
				} else {
					cityIcon = canvas.NewImageFromResource(theme.HomeIcon())
				}
				cityIcon.FillMode = canvas.ImageFillContain
				cityIcon.SetMinSize(fyne.NewSize(24, 24))

				upBtn := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
					if idx > 0 {
						state.cities[idx], state.cities[idx-1] = state.cities[idx-1], state.cities[idx]
						refreshCityList()
					}
				})
				downBtn := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() {
					if idx < len(state.cities)-1 {
						state.cities[idx], state.cities[idx+1] = state.cities[idx+1], state.cities[idx]
						refreshCityList()
					}
				})
				removeBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
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
				// In free mode, disable remove/reorder — default cities are locked.
				if !hasLicense {
					removeBtn.Disable()
					upBtn.Disable()
					downBtn.Disable()
				}

				// Build a card-like row for each city.
				leftContent := container.NewHBox(cityIcon, cityName)
				buttons := container.NewHBox(upBtn, downBtn, removeBtn)
				row := container.NewBorder(nil, nil, leftContent, buttons)
				cityListBox.Add(row)
				cityListBox.Add(widget.NewSeparator())
			}
			cityListBox.Refresh()
		}
		refreshCityList()

		// Now that refreshCityList is defined, upgrade the provider change handler
		// to also clear cities when the provider switches.
		providerSelect.OnChanged = func(selected string) {
			newProvider := providerDisplayToValue[selected]
			applyProviderSliderConstraints(newProvider, true)

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
			// Block adding cities in free mode (no license).
			if !cfg.HasLicense() {
				dialog.ShowError(fmt.Errorf("%s", u.t("error.settings.licenseRequired")), win)
				return
			}
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
			// Block searching for cities in free mode (no license).
			if !cfg.HasLicense() {
				dialog.ShowError(fmt.Errorf("%s", u.t("error.settings.licenseRequired")), win)
				return
			}
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
					uStr := fmt.Sprintf("https://weather-gateway-ricardo.web.app/api/v1/weather/key=%s/%s",
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

		// In free mode, disable the add-city form entirely.
		if !cfg.HasLicense() {
			addNameEntry.Disable()
			addRegionEntry.Disable()
			addLatEntry.Disable()
			addLonEntry.Disable()
			addTimezoneEntry.Disable()
			addBtn.Disable()
			searchBtn.Disable()
		}

		// Coordinates on one row (side by side).
		coordRow := container.NewGridWithColumns(2, addLatEntry, addLonEntry)

		addForm := container.NewVBox(
			widget.NewForm(
				widget.NewFormItem(u.t("settings.locations.nameLabel"), nameItemContent),
				widget.NewFormItem(u.t("settings.locations.regionLabel"), addRegionEntry),
				widget.NewFormItem(u.t("settings.locations.coordLabel"), coordRow),
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

		// ── Language card grid ───────────────────────────────────────────────
		// Each locale gets a card with a coloured badge (2-letter code) and
		// the full language name. Clicking a card switches the locale and
		// rebuilds the UI. The active card is highlighted with an accent border.

		// badgeColor returns a distinct accent colour per locale code.
		badgeColor := func(code string) color.Color {
			switch code {
			case "en-GB":
				return color.NRGBA{R: 0, G: 82, B: 165, A: 255} // blue
			case "pt-BR":
				return color.NRGBA{R: 0, G: 156, B: 59, A: 255} // green
			case "de-DE":
				return color.NRGBA{R: 60, G: 60, B: 60, A: 255} // dark grey
			case "es-ES":
				return color.NRGBA{R: 198, G: 146, B: 20, A: 255} // golden yellow
			case "nl-NL":
				return color.NRGBA{R: 255, G: 122, B: 0, A: 255} // Dutch orange
			case "pl-PL":
				return color.NRGBA{R: 227, G: 10, B: 23, A: 255} // Turkish red (swapped)
			case "fr-FR":
				return color.NRGBA{R: 0, G: 35, B: 149, A: 255} // French blue
			case "it-IT":
				return color.NRGBA{R: 180, G: 120, B: 60, A: 255} // warm light brown
			case "tr-TR":
				return color.NRGBA{R: 0, G: 158, B: 158, A: 255} // teal
			default:
				return color.NRGBA{R: 100, G: 100, B: 100, A: 255}
			}
		}

		// badgeCode returns the 2-letter display code for a locale.
		badgeCode := func(code string) string {
			switch code {
			case "en-GB":
				return "EN"
			case "pt-BR":
				return "PT"
			case "de-DE":
				return "DE"
			case "es-ES":
				return "ES"
			case "nl-NL":
				return "NL"
			case "pl-PL":
				return "PL"
			case "fr-FR":
				return "FR"
			case "it-IT":
				return "IT"
			case "tr-TR":
				return "TR"
			default:
				if len(code) >= 2 {
					return strings.ToUpper(code[:2])
				}
				return code
			}
		}

		// langNativeName returns the language name without the country code suffix.
		langNativeName := func(displayName string) string {
			// Strip " (XX)" suffix if present — the badge already shows the code.
			if idx := strings.Index(displayName, " ("); idx != -1 {
				return displayName[:idx]
			}
			return displayName
		}

		var langLocales []i18n.LocaleInfo
		if u.lm != nil {
			langLocales = u.lm.AvailableLocales()
		}

		// buildLangCard creates a single language card. It is a function so it
		// can be called again when the selection changes to refresh all cards.
		var langCardObjects []fyne.CanvasObject
		var rebuildLangCards func()
		rebuildLangCards = func() {
			langCardObjects = nil
			for _, loc := range langLocales {
				loc := loc // capture
				isSelected := loc.Code == state.selectedLang
				accent := badgeColor(loc.Code)

				// Badge: coloured rounded rectangle with bold 2-letter code.
				badgeRect := canvas.NewRectangle(accent)
				badgeRect.CornerRadius = 6
				badgeRect.SetMinSize(fyne.NewSize(52, 36))

				badgeLabel := canvas.NewText(badgeCode(loc.Code), color.White)
				badgeLabel.TextSize = 16
				badgeLabel.TextStyle = fyne.TextStyle{Bold: true}
				badgeLabel.Alignment = fyne.TextAlignCenter

				badge := container.NewStack(badgeRect, container.NewCenter(badgeLabel))

				// Language name label.
				nameLabel := widget.NewLabel(langNativeName(loc.DisplayName))
				nameLabel.Alignment = fyne.TextAlignCenter
				nameLabel.TextStyle = fyne.TextStyle{Bold: isSelected}

				// Card background: accent tint when selected, default otherwise.
				var cardBg *canvas.Rectangle
				if isSelected {
					r, g, b, _ := accent.RGBA()
					cardBg = canvas.NewRectangle(color.NRGBA{
						R: uint8(r>>8) / 5,
						G: uint8(g>>8) / 5,
						B: uint8(b>>8) / 5,
						A: 60,
					})
				} else {
					cardBg = canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0})
				}
				cardBg.CornerRadius = 8

				// Border rectangle: accent colour when selected, muted otherwise.
				var borderColor color.Color
				if isSelected {
					borderColor = accent
				} else {
					borderColor = color.NRGBA{R: 100, G: 100, B: 100, A: 80}
				}
				borderRect := canvas.NewRectangle(color.Transparent)
				borderRect.StrokeColor = borderColor
				borderRect.StrokeWidth = 2
				borderRect.CornerRadius = 8

				inner := container.NewVBox(
					container.NewCenter(badge),
					container.NewCenter(nameLabel),
				)
				padded := container.NewPadded(inner)
				card := container.NewStack(cardBg, borderRect, padded)

				// Wrap in a tappable object.
				tappable := newTappableContainer(card, func() {
					if loc.Code == state.selectedLang {
						return
					}
					state.selectedLang = loc.Code
					if u.lm != nil {
						_ = u.lm.SetLocale(loc.Code)
					}
					selectedTabIndex = 3 // stay on the Language tab after rebuild
					win.SetTitle(u.t("settings.title"))
					buildSettingsUI()
				})
				langCardObjects = append(langCardObjects, tappable)
			}
		}
		rebuildLangCards()

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

		// ── Panel Display checkboxes ─────────────────────────────────────────
		df := cfg.GetDisplayFields()
		chkCity := widget.NewCheck(u.t("settings.display.city"), nil)
		chkCity.Checked = df.ShowCity
		chkIcon := widget.NewCheck(u.t("settings.display.icon"), nil)
		chkIcon.Checked = df.ShowIcon
		chkTemp := widget.NewCheck(u.t("settings.display.temp"), nil)
		chkTemp.Checked = df.ShowTemp
		chkDesc := widget.NewCheck(u.t("settings.display.desc"), nil)
		chkDesc.Checked = df.ShowDesc
		chkHumidWind := widget.NewCheck(u.t("settings.display.humidWind"), nil)
		chkHumidWind.Checked = df.ShowHumidWind
		chkTime := widget.NewCheck(u.t("settings.display.time"), nil)
		chkTime.Checked = df.ShowTime
		chkDate := widget.NewCheck(u.t("settings.display.date"), nil)
		chkDate.Checked = df.ShowDate

		displayChecks := container.NewGridWithColumns(4,
			chkCity, chkIcon, chkTemp, chkDesc,
			chkHumidWind, chkTime, chkDate,
		)

		// Live preview panels for the Appearance tab.
		defaultCitiesAppearance := config.DefaultCities()
		appearancePreviewPanels := make([]*panel.CityPanel, len(defaultCitiesAppearance))
		appearancePreviewObjects := make([]fyne.CanvasObject, len(defaultCitiesAppearance))
		for i := range defaultCitiesAppearance {
			p := panel.NewCityPanel(u.lm)
			appearancePreviewPanels[i] = p
			appearancePreviewObjects[i] = p.Container()
		}
		appearancePreviewGrid := container.NewGridWithColumns(len(defaultCitiesAppearance), appearancePreviewObjects...)
		state.appearancePanels = appearancePreviewPanels

		// Helper: read current checkbox state and apply to all preview panels.
		applyDisplayPreview := func() {
			fields := &config.DisplayFields{
				ShowCity:      chkCity.Checked,
				ShowIcon:      chkIcon.Checked,
				ShowTemp:      chkTemp.Checked,
				ShowDesc:      chkDesc.Checked,
				ShowHumidWind: chkHumidWind.Checked,
				ShowTime:      chkTime.Checked,
				ShowDate:      chkDate.Checked,
			}
			state.displayFields = fields
			for _, p := range appearancePreviewPanels {
				p.ApplyDisplayFields(fields)
			}
		}

		// Wire up OnChanged for all checkboxes.
		chkCity.OnChanged = func(_ bool) { applyDisplayPreview() }
		chkIcon.OnChanged = func(_ bool) { applyDisplayPreview() }
		chkTemp.OnChanged = func(_ bool) { applyDisplayPreview() }
		chkDesc.OnChanged = func(_ bool) { applyDisplayPreview() }
		chkHumidWind.OnChanged = func(_ bool) { applyDisplayPreview() }
		chkTime.OnChanged = func(_ bool) { applyDisplayPreview() }
		chkDate.OnChanged = func(_ bool) { applyDisplayPreview() }

		// Fetch live weather data for appearance preview panels.
		go func(cities []config.CityConfig, panels []*panel.CityPanel, unit config.TemperatureUnit) {
			adapter := remoteapi.NewRemoteAPIAdapter("easyweatherwidget", "free")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			for i, city := range cities {
				data, err := adapter.FetchWeather(ctx, city)
				if err != nil {
					continue
				}
				idx := i
				d := data
				fyne.Do(func() {
					panels[idx].Update(d, unit)
					panels[idx].StartClock(cities[idx].Timezone)
				})
			}
		}(defaultCitiesAppearance, appearancePreviewPanels, state.selectedUnit)

		appearanceContent := container.NewPadded(container.NewVScroll(container.NewVBox(
			sectionBlock(u.t("settings.position.title"), u.t("settings.position.subtitle"),
				container.NewVBox(positionItems...),
			),
			sectionBlock(u.t("settings.transparency.title"), u.t("settings.transparency.subtitle"),
				opacityRadio,
			),
			sectionBlock(u.t("settings.startup.title"), u.t("settings.startup.subtitle"),
				autoStartCheck,
			),
		)))
		appearanceTab := container.NewTabItemWithIcon(u.t("settings.tab.appearance"), theme.ColorPaletteIcon(), appearanceContent)

		// ── Widget tab ────────────────────────────────────────────────────────
		widgetContent := container.NewPadded(container.NewVScroll(container.NewVBox(
			sectionBlock(u.t("settings.display.title"), u.t("settings.display.subtitle"),
				displayChecks,
			),
			sectionBlock(u.t("settings.temperature.title"), u.t("settings.temperature.subtitle"),
				unitRadio,
			),
			appearancePreviewGrid,
		)))
		widgetTab := container.NewTabItemWithIcon(u.t("settings.tab.widget"), theme.ComputerIcon(), widgetContent)

		// ── Language tab ──────────────────────────────────────────────────────
		langGrid := container.NewGridWithColumns(3, langCardObjects...)
		languageContent := container.NewPadded(container.NewVScroll(container.NewVBox(
			sectionBlock(u.t("settings.language.title"), u.t("settings.language.subtitle"),
				langGrid,
			),
		)))
		languageTab := container.NewTabItemWithIcon(u.t("settings.tab.language"), theme.MailComposeIcon(), languageContent)

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

		websiteLink := widget.NewHyperlink("easysmartapps.co.uk/weatherwidget", parseURL("https://easysmartapps.co.uk/weatherwidget"))
		manualLink := widget.NewHyperlink("easysmartapps.co.uk/weatherwidget-manual", parseURL("https://easysmartapps.co.uk/weatherwidget-manual"))

		// Live preview: create 3 CityPanel instances for the default cities.
		defaultCities := config.DefaultCities()
		previewPanels := make([]*panel.CityPanel, len(defaultCities))
		previewObjects := make([]fyne.CanvasObject, len(defaultCities))
		for i := range defaultCities {
			p := panel.NewCityPanel(u.lm)
			previewPanels[i] = p
			previewObjects[i] = p.Container()
		}
		state.previewPanels = previewPanels
		previewGrid := container.NewGridWithColumns(len(defaultCities), previewObjects...)

		// Fetch live weather data for preview panels in the background.
		go func(cities []config.CityConfig, panels []*panel.CityPanel, unit config.TemperatureUnit) {
			adapter := remoteapi.NewRemoteAPIAdapter("easyweatherwidget", "free")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			for i, city := range cities {
				data, err := adapter.FetchWeather(ctx, city)
				if err != nil {
					continue
				}
				idx := i
				d := data
				fyne.Do(func() {
					panels[idx].Update(d, unit)
					panels[idx].StartClock(cities[idx].Timezone)
				})
			}
		}(defaultCities, previewPanels, state.selectedUnit)

		previewLabel := widget.NewRichTextFromMarkdown("**" + u.t("settings.about.previewLabel") + "**")

		aboutContent := container.NewPadded(container.NewVScroll(container.NewVBox(
			widget.NewCard(u.t("settings.about.appName"), "", container.NewVBox(
				aboutDesc,
				container.NewHBox(widget.NewLabel(u.t("settings.about.websiteLabel")), websiteLink),
				container.NewHBox(widget.NewLabel(u.t("settings.about.manualLabel")), manualLink),
				aboutVersion,
			)),
			previewLabel,
			previewGrid,
		)))
		aboutTab := container.NewTabItemWithIcon(u.t("settings.tab.about"), theme.InfoIcon(), aboutContent)

		tabs := container.NewAppTabs(appearanceTab, widgetTab, providerTab, locationsTab, languageTab, aboutTab)
		tabs.SetTabLocation(container.TabLocationLeading)
		if selectedTabIndex > 0 && selectedTabIndex < len(tabs.Items) {
			tabs.SelectIndex(selectedTabIndex)
		}

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
			state.saved = true
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

	win.SetOnClosed(func() {
		// Stop preview panel clocks.
		for _, p := range state.previewPanels {
			p.StopClock()
		}
		for _, p := range state.appearancePanels {
			p.StopClock()
		}
		// Revert live preview changes if the user closed without saving.
		if !state.saved {
			u.SetOpacity(origOpacity)
			u.RerenderPanels(origUnit)
			if origCustomX != nil && origCustomY != nil {
				u.SetPosition(*origCustomX, *origCustomY)
			} else {
				u.SetCorner(origPosition, origMonitor)
			}
		}
		u.settings = nil
	})
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
	cornerPosition := current.CornerPosition
	if cornerPosition == "" {
		cornerPosition = "bottom-right"
	}
	if positionRadio != nil && positionRadio.Selected != "" {
		if pos, ok := positionValueMap[positionRadio.Selected]; ok {
			cornerPosition = pos
		}
	}

	opacity := opacityMap[opacityRadio.Selected]
	if opacity == 0 {
		opacity = 100
	}
	// Custom coordinates logic: if state.customX/Y are set, use them;
	// otherwise if no corner was selected, preserve existing custom coordinates.
	var customX, customY *int
	if state.customX != nil && state.customY != nil {
		customX = state.customX
		customY = state.customY
	} else if (positionRadio == nil || positionRadio.Selected == "") && current.CustomX != nil && current.CustomY != nil {
		// No corner selected — preserve existing custom coordinates.
		customX = current.CustomX
		customY = current.CustomY
	}

	// Parse monitor index from the "Monitor N" selection (0-based).
	monitorIndex := 0
	if monitorSelect != nil && monitorSelect.Selected != "" {
		fmt.Sscanf(monitorSelect.Selected, "Monitor %d", &monitorIndex)
		monitorIndex-- // convert 1-based display to 0-based index
		if monitorIndex < 0 {
			monitorIndex = 0
		}
	} else {
		monitorIndex = current.MonitorIndex
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
	cfg.DisplayFields = state.displayFields
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

// tappableContainer wraps a fyne.CanvasObject and fires a callback on tap.
// Used to make the language selection cards clickable.
type tappableContainer struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTappableContainer(content fyne.CanvasObject, onTap func()) *tappableContainer {
	t := &tappableContainer{content: content, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

func (t *tappableContainer) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tappableContainer) TappedSecondary(_ *fyne.PointEvent) {}
