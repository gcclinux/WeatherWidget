//go:build linux

package uitk

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
)

// ewwAPIKeyLen is the expected length of an EasyWeatherWidget API key (UUID v4).
const ewwAPIKeyLen = 36

// generateClientReferenceID creates a random UUID v4 string (e.g. "f169fecc-de0c-4de1-b8c8-4ec3701b671c")
// to be used as a unique client_reference_id for Stripe payment links.
func generateClientReferenceID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func openURL(urlStr string) {
	_ = exec.Command("xdg-open", urlStr).Start()
}

// showSettingsDialog opens the GTK settings dialog for WeatherWidget.
// Not transient for the main window (which is a positioned desktop widget).
func showSettingsDialog(m *manager) {
	dlg, err := gtk.DialogNew()
	if err != nil {
		return
	}
	dlg.SetTitle(m.t("settings.title"))
	// Don't set transient-for — the main window is a positioned desktop widget
	// and making the dialog transient causes them to move together.
	dlg.SetModal(false)
	dlg.SetDefaultSize(600, 748)
	dlg.SetPosition(gtk.WIN_POS_CENTER)

	box, _ := dlg.GetContentArea()
	box.SetSpacing(8)
	box.SetMarginTop(12)
	box.SetMarginBottom(4)
	box.SetMarginStart(12)
	box.SetMarginEnd(12)

	// ── Notebook (tabs) ───────────────────────────────────────────────────
	nb, _ := gtk.NotebookNew()

	// --- API / Provider tab ---
	providerBox := buildProviderTab(m, dlg)
	providerLabel, _ := gtk.LabelNew(m.t("settings.tab.provider"))
	nb.AppendPage(providerBox, providerLabel)

	// --- Locations tab ---
	cities := make([]config.CityConfig, len(m.cfg.Cities))
	copy(cities, m.cfg.Cities)
	locationsBox, getCities := buildLocationsTab(m, dlg, cities, func() {
		nb.SetCurrentPage(0)
	})
	locationsLabel, _ := gtk.LabelNew(m.t("settings.tab.locations"))
	nb.AppendPage(locationsBox, locationsLabel)

	// --- Widget tab ---
	widgetBox, getDisplayFields, getPollutionFields, getTempUnit, getWindUnit := buildWidgetTab(m)
	widgetLabel, _ := gtk.LabelNew(m.t("settings.tab.widget"))
	nb.AppendPage(widgetBox, widgetLabel)

	// --- Language tab ---
	selectedLocale := m.cfg.Locale
	if selectedLocale == "" {
		selectedLocale = "en-GB"
	}
	langBox, getLocale := buildLanguageTab(m, selectedLocale)
	langLabel, _ := gtk.LabelNew(m.t("settings.tab.language"))
	nb.AppendPage(langBox, langLabel)

	// --- Appearance tab ---
	appearanceBox, opacityScale, noBgCheck, noBorderCheck, iconThemeCombo, getViewMode := buildAppearanceTab(m)
	appearanceLabel, _ := gtk.LabelNew(m.t("settings.tab.appearance"))
	nb.AppendPage(appearanceBox, appearanceLabel)

	// --- About tab ---
	aboutBox, getFontSizes := buildAboutTab(m)
	aboutLabel, _ := gtk.LabelNew(m.t("settings.tab.about"))
	nb.AppendPage(aboutBox, aboutLabel)

	box.PackStart(nb, true, true, 0)

	// ── Action buttons ────────────────────────────────────────────────────
	dlg.AddButton(m.t("settings.save"), gtk.RESPONSE_OK)
	dlg.AddButton(m.t("settings.cancel"), gtk.RESPONSE_CANCEL)

	dlg.ShowAll()

	// Snapshot font sizes so Cancel can revert any live-preview changes.
	origCityTime := m.fontSizeCityTime
	origTempIcon := m.fontSizeTempIcon
	origConditions := m.fontSizeConditions

	resp := dlg.Run()
	if resp == gtk.RESPONSE_OK {
		opacity := int(opacityScale.GetValue())
		opacity = snapOpacity(opacity)
		noBackground := noBgCheck.GetActive()
		noBorder := noBorderCheck.GetActive()

		newCfg := *m.cfg
		newCfg.Opacity = opacity
		newCfg.NoBackground = noBackground
		newCfg.NoBorder = noBorder
		newCfg.Cities = getCities()                     // collect current city list from the locations tab
		newCfg.Locale = getLocale()                     // collect selected language
		newCfg.DisplayFields = getDisplayFields()       // collect panel visibility
		newCfg.PollutionFields = getPollutionFields()   // collect pollution metrics
		newCfg.TemperatureUnit = getTempUnit()          // collect temperature unit
		newCfg.WindSpeedUnit = getWindUnit()            // collect wind speed unit
		newCfg.ViewMode = getViewMode()                 // collect view mode
		newCfg.IconTheme = config.NormalizeIconTheme(config.IconTheme(iconThemeCombo.GetActiveID()))

		fs := getFontSizes()
		newCfg.FontSizeCityTime = fs.cityTime
		newCfg.FontSizeTempIcon = fs.tempIcon
		newCfg.FontSizeConditions = fs.conditions

		// Apply live before saving so the user sees the change immediately.
		m.SetOpacity(opacity)
		m.SetNoBackground(noBackground)
		m.SetNoBorder(noBorder)

		_ = m.onSettingsSave(&newCfg)
	} else {
		// User cancelled — revert any live font-size preview back to what was
		// saved in config before the dialog was opened.
		m.SetFontSizes(origCityTime, origTempIcon, origConditions)
	}

	dlg.Destroy()
}

// snapOpacity rounds to the nearest supported value: 25, 50, 75, or 100.
func snapOpacity(v int) int {
	switch {
	case v <= 37:
		return 25
	case v <= 62:
		return 50
	case v <= 87:
		return 75
	default:
		return 100
	}
}

// buildProviderTab creates the API / Provider settings tab.
func buildProviderTab(m *manager, parent *gtk.Dialog) *gtk.Box {
	vbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	vbox.SetMarginTop(8)
	vbox.SetMarginStart(4)
	vbox.SetMarginEnd(4)

	// Provider selector.
	providerLabel, _ := gtk.LabelNew(m.t("settings.provider.label") + ":")
	providerLabel.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(providerLabel, false, false, 0)

	providerCombo, _ := gtk.ComboBoxTextNew()
	providerCombo.AppendText("EasyWeatherWidget (Pro)")
	providerCombo.AppendText("OpenWeatherMap (Free)")
	selectedProvider := 0
	if m.cfg.APIConfig != nil && m.cfg.APIConfig.Provider == "openweathermap" {
		selectedProvider = 1
	}
	providerCombo.SetActive(selectedProvider)

	getApiBtn, _ := gtk.ButtonNewWithLabel(m.t("settings.provider.getFreeApi"))

	providerRow, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	providerRow.PackStart(providerCombo, true, true, 0)
	providerRow.PackStart(getApiBtn, false, false, 0)
	vbox.PackStart(providerRow, false, false, 0)

	// API key entry.
	apiKeyLabel, _ := gtk.LabelNew(m.t("settings.provider.apiKeyLabel") + ":")
	apiKeyLabel.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(apiKeyLabel, false, false, 0)

	apiKeyEntry, _ := gtk.EntryNew()
	apiKeyEntry.SetPlaceholderText(m.t("settings.provider.apiKeyPlaceholder"))
	if m.cfg.APIConfig != nil {
		apiKeyEntry.SetText(m.cfg.APIConfig.APIKey)
	}
	vbox.PackStart(apiKeyEntry, false, false, 0)

	// Activation card frame — shown when a Pro API key is active or just generated.
	activationFrame, _ := gtk.FrameNew("")
	activationBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
	activationBox.SetMarginTop(6)
	activationBox.SetMarginBottom(6)
	activationBox.SetMarginStart(6)
	activationBox.SetMarginEnd(6)

	activationKeyLabel, _ := gtk.LabelNew("")
	activationKeyLabel.SetHAlign(gtk.ALIGN_START)

	activationMsgLabel, _ := gtk.LabelNew("")
	activationMsgLabel.SetHAlign(gtk.ALIGN_START)
	activationMsgLabel.SetLineWrap(true)

	activationBox.PackStart(activationKeyLabel, false, false, 0)
	activationBox.PackStart(activationMsgLabel, false, false, 0)
	activationFrame.Add(activationBox)

	showActivationCard := func(key string) {
		activationFrame.SetLabel(m.t("settings.provider.apiKeyActivation.title"))
		activationKeyLabel.SetMarkup(fmt.Sprintf("<tt>%s</tt>", glib.MarkupEscapeText(key)))
		activationMsgLabel.SetText(m.t("settings.provider.apiKeyActivation.message"))
		activationFrame.ShowAll()
	}

	activationFrame.Hide()

	if m.cfg.APIConfig != nil && m.cfg.APIConfig.Provider == "easyweatherwidget" && len(m.cfg.APIConfig.APIKey) == ewwAPIKeyLen {
		showActivationCard(m.cfg.APIConfig.APIKey)
	}

	updateGetApiBtn := func() {
		provIdx := providerCombo.GetActive()
		if provIdx == 0 { // EasyWeatherWidget (Pro)
			getApiBtn.SetLabel(m.t("settings.provider.getProApi"))
		} else { // OpenWeatherMap (Free)
			getApiBtn.SetLabel(m.t("settings.provider.getFreeApi"))
		}
	}
	updateGetApiBtn()

	providerCombo.Connect("changed", func() {
		updateGetApiBtn()
	})

	handleGetProAPI := func() {
		apiKey, _ := apiKeyEntry.GetText()
		currentKey := strings.TrimSpace(apiKey)

		clientRef := currentKey
		if len(currentKey) != ewwAPIKeyLen {
			clientRef = generateClientReferenceID()

			if m.cfg.APIConfig == nil {
				m.cfg.APIConfig = &config.APIConfig{}
			}
			m.cfg.APIConfig.Provider = "easyweatherwidget"
			m.cfg.APIConfig.APIKey = clientRef

			apiKeyEntry.SetText(clientRef)
		}

		showActivationCard(clientRef)

		openURL("https://buy.stripe.com/bJe3cvaJOa650fQ8cPdZ603?client_reference_id=" + clientRef)
	}

	getApiBtn.Connect("clicked", func() {
		provIdx := providerCombo.GetActive()
		if provIdx == 0 { // EasyWeatherWidget (Pro)
			handleGetProAPI()
		} else {
			openURL("https://openweathermap.org/")
		}
	})

	// Refresh interval.
	intervalLabel, _ := gtk.LabelNew(m.t("settings.interval.title") + ":")
	intervalLabel.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(intervalLabel, false, false, 0)

	intervalEntry, _ := gtk.EntryNew()
	intervalEntry.SetText(strconv.Itoa(m.cfg.RefreshInterval))
	vbox.PackStart(intervalEntry, false, false, 0)

	noteLabel, _ := gtk.LabelNew(m.t("settings.provider.note"))
	noteLabel.SetHAlign(gtk.ALIGN_START)
	noteLabel.SetLineWrap(true)
	vbox.PackStart(noteLabel, false, false, 0)

	vbox.PackStart(activationFrame, false, false, 0)

	// Save button (provider tab).
	saveAPIBtn, _ := gtk.ButtonNewWithLabel(m.t("settings.save"))
	saveAPIBtn.Connect("clicked", func() {
		newCfg := *m.cfg
		provIdx := providerCombo.GetActive()
		providerValue := "easyweatherwidget"
		if provIdx == 1 {
			providerValue = "openweathermap"
		}
		apiKey, _ := apiKeyEntry.GetText()
		newCfg.APIConfig = &config.APIConfig{
			Provider: providerValue,
			APIKey:   strings.TrimSpace(apiKey),
		}
		intervalStr, _ := intervalEntry.GetText()
		if v, err := strconv.Atoi(strings.TrimSpace(intervalStr)); err == nil && v >= 10 && v <= 120 {
			newCfg.RefreshInterval = v
		}
		if err := m.onSettingsSave(&newCfg); err != nil {
			showErrorDialog(parent, fmt.Sprintf(m.t("error.settings.saveFailed"), err))
		}
	})
	vbox.PackStart(saveAPIBtn, false, false, 0)

	return vbox
}

// buildAppearanceTab creates the Appearance settings tab.
func buildAppearanceTab(m *manager) (*gtk.Box, *gtk.Scale, *gtk.CheckButton, *gtk.CheckButton, *gtk.ComboBoxText, func() config.ViewMode) {
	vbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	vbox.SetMarginTop(8)
	vbox.SetMarginStart(4)
	vbox.SetMarginEnd(4)

	// ── View Mode ─────────────────────────────────────────────────────────
	viewModeTitle, _ := gtk.LabelNew("")
	viewModeTitle.SetMarkup("<b>" + glib.MarkupEscapeText(m.t("settings.viewMode.title")) + "</b>")
	viewModeTitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(viewModeTitle, false, false, 0)

	viewModeSubtitle, _ := gtk.LabelNew(m.t("settings.viewMode.subtitle"))
	viewModeSubtitle.SetHAlign(gtk.ALIGN_START)
	viewModeSubtitle.SetLineWrap(true)
	vbox.PackStart(viewModeSubtitle, false, false, 0)

	viewModeHBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 16)
	enhancedBtn, _ := gtk.RadioButtonNewWithLabel(nil, m.t("settings.viewMode.enhanced"))
	simpleBtn, _ := gtk.RadioButtonNewWithLabelFromWidget(enhancedBtn, m.t("settings.viewMode.simple"))

	currentViewMode := config.NormalizeViewMode(m.cfg.ViewMode)
	if currentViewMode == config.ViewModeSimple {
		simpleBtn.SetActive(true)
	} else {
		enhancedBtn.SetActive(true)
	}
	viewModeHBox.PackStart(enhancedBtn, false, false, 0)
	viewModeHBox.PackStart(simpleBtn, false, false, 0)
	vbox.PackStart(viewModeHBox, false, false, 0)

	sepViewMode, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(sepViewMode, false, false, 4)

	// Opacity slider.
	opacityLabel, _ := gtk.LabelNew(m.t("settings.transparency.title") + ":")
	opacityLabel.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(opacityLabel, false, false, 0)

	opacitySubLabel, _ := gtk.LabelNew(m.t("settings.transparency.subtitle"))
	opacitySubLabel.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(opacitySubLabel, false, false, 0)

	opacityScale, _ := gtk.ScaleNewWithRange(gtk.ORIENTATION_HORIZONTAL, 25, 100, 25)
	opacityScale.SetValue(float64(m.opacity))
	opacityScale.SetDrawValue(true)
	opacityScale.AddMark(25, gtk.POS_BOTTOM, "25%")
	opacityScale.AddMark(50, gtk.POS_BOTTOM, "50%")
	opacityScale.AddMark(75, gtk.POS_BOTTOM, "75%")
	opacityScale.AddMark(100, gtk.POS_BOTTOM, "100%")
	vbox.PackStart(opacityScale, false, false, 0)

	// No-background toggle.
	noBgCheck, _ := gtk.CheckButtonNewWithLabel(m.t("settings.noBackground.checkbox"))
	noBgCheck.SetActive(m.noBackground)
	noBgCheck.Connect("toggled", func() {
		if noBgCheck.GetActive() {
			opacityScale.SetSensitive(false)
		} else {
			opacityScale.SetSensitive(true)
		}
	})
	vbox.PackStart(noBgCheck, false, false, 0)

	bgNote, _ := gtk.LabelNew(m.t("settings.noBackground.note"))
	bgNote.SetHAlign(gtk.ALIGN_START)
	bgNote.SetLineWrap(true)
	vbox.PackStart(bgNote, false, false, 0)

	// No-border toggle.
	sep0, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(sep0, false, false, 4)

	noBorderCheck, _ := gtk.CheckButtonNewWithLabel(m.t("settings.noBorder.checkbox"))
	noBorderCheck.SetActive(m.noBorder)
	vbox.PackStart(noBorderCheck, false, false, 0)

	borderNote, _ := gtk.LabelNew(m.t("settings.noBorder.note"))
	borderNote.SetHAlign(gtk.ALIGN_START)
	borderNote.SetLineWrap(true)
	vbox.PackStart(borderNote, false, false, 0)

	// Autostart toggle.
	sep, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(sep, false, false, 4)

	autostartCheck, _ := gtk.CheckButtonNewWithLabel(m.t("settings.startup.autostart"))
	autostartCheck.SetActive(isAutoStartEnabled())
	autostartCheck.Connect("toggled", func() {
		if err := setAutoStartEnabled(autostartCheck.GetActive()); err != nil {
			autostartCheck.SetActive(isAutoStartEnabled())
		}
	})
	vbox.PackStart(autostartCheck, false, false, 0)

	// ── Weather Icons ────────────────────────────────────────────────────────
	iconSep, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(iconSep, false, false, 4)

	iconTitle, _ := gtk.LabelNew("")
	iconTitle.SetMarkup("<b>" + glib.MarkupEscapeText(m.t("settings.icons.title")) + "</b>")
	iconTitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(iconTitle, false, false, 0)

	iconSubtitle, _ := gtk.LabelNew(m.t("settings.icons.subtitle"))
	iconSubtitle.SetHAlign(gtk.ALIGN_START)
	iconSubtitle.SetLineWrap(true)
	vbox.PackStart(iconSubtitle, false, false, 0)

	iconCombo, _ := gtk.ComboBoxTextNew()
	iconCombo.Append("new", m.t("settings.icons.new"))
	iconCombo.Append("original", m.t("settings.icons.original"))

	currentIconTheme := config.NormalizeIconTheme(m.cfg.IconTheme)
	if currentIconTheme == config.IconThemeOriginal {
		iconCombo.SetActiveID("original")
	} else {
		iconCombo.SetActiveID("new")
	}
	vbox.PackStart(iconCombo, false, false, 0)

	// ── Widget Position ─────────────────────────────────────────────────────
	posSep, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(posSep, false, false, 4)

	posTitle, _ := gtk.LabelNew("")
	posTitle.SetMarkup("<b>Widget Position</b>")
	posTitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(posTitle, false, false, 0)

	posSubtitle, _ := gtk.LabelNew("Move the widget on screen. Changes apply immediately.")
	posSubtitle.SetHAlign(gtk.ALIGN_START)
	posSubtitle.SetLineWrap(true)
	vbox.PackStart(posSubtitle, false, false, 0)

	// Current position from config or window.
	var curX, curY int
	if m.cfg.CustomX != nil && m.cfg.CustomY != nil {
		curX, curY = *m.cfg.CustomX, *m.cfg.CustomY
	} else {
		curX, curY = m.win.GetPosition()
	}

	// X/Y entry fields + Apply button.
	posRow, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)

	xLabel, _ := gtk.LabelNew("X:")
	posRow.PackStart(xLabel, false, false, 0)

	xEntry, _ := gtk.EntryNew()
	xEntry.SetText(strconv.Itoa(curX))
	xEntry.SetWidthChars(6)
	posRow.PackStart(xEntry, false, false, 0)

	yLabel, _ := gtk.LabelNew("Y:")
	posRow.PackStart(yLabel, false, false, 0)

	yEntry, _ := gtk.EntryNew()
	yEntry.SetText(strconv.Itoa(curY))
	yEntry.SetWidthChars(6)
	posRow.PackStart(yEntry, false, false, 0)

	// Arrow nudge buttons (10px per click) — inline with X/Y fields.
	const nudge = 10

	leftBtn, _ := gtk.ButtonNewWithLabel("◀")
	upBtn, _ := gtk.ButtonNewWithLabel("▲")
	downBtn, _ := gtk.ButtonNewWithLabel("▼")
	rightBtn, _ := gtk.ButtonNewWithLabel("▶")

	posRow.PackStart(leftBtn, false, false, 0)
	posRow.PackStart(upBtn, false, false, 0)
	posRow.PackStart(downBtn, false, false, 0)
	posRow.PackStart(rightBtn, false, false, 0)

	applyPosBtn, _ := gtk.ButtonNewWithLabel("Apply")
	posRow.PackStart(applyPosBtn, false, false, 0)

	vbox.PackStart(posRow, false, false, 0)

	// moveAndSave repositions the widget, updates entries, and persists.
	moveAndSave := func(x, y int) {
		cx, cy := x, y
		m.cfg.CustomX = &cx
		m.cfg.CustomY = &cy
		// Move the window immediately using both GTK and X11 calls.
		m.win.Move(x, y)
		x11MoveWindow(m.win, x, y)
		// Update entry fields to reflect new position.
		xEntry.SetText(strconv.Itoa(x))
		yEntry.SetText(strconv.Itoa(y))
		// Persist to config file.
		go func() {
			if err := m.cfgSvc.Save(m.cfg); err != nil {
				log.Printf("position save failed (%d,%d): %v", cx, cy, err)
			}
		}()
	}

	applyPosBtn.Connect("clicked", func() {
		xText, _ := xEntry.GetText()
		yText, _ := yEntry.GetText()
		x, _ := strconv.Atoi(strings.TrimSpace(xText))
		y, _ := strconv.Atoi(strings.TrimSpace(yText))
		moveAndSave(x, y)
	})
	leftBtn.Connect("clicked", func() {
		xText, _ := xEntry.GetText()
		yText, _ := yEntry.GetText()
		x, _ := strconv.Atoi(strings.TrimSpace(xText))
		y, _ := strconv.Atoi(strings.TrimSpace(yText))
		moveAndSave(x-nudge, y)
	})
	rightBtn.Connect("clicked", func() {
		xText, _ := xEntry.GetText()
		yText, _ := yEntry.GetText()
		x, _ := strconv.Atoi(strings.TrimSpace(xText))
		y, _ := strconv.Atoi(strings.TrimSpace(yText))
		moveAndSave(x+nudge, y)
	})
	upBtn.Connect("clicked", func() {
		xText, _ := xEntry.GetText()
		yText, _ := yEntry.GetText()
		x, _ := strconv.Atoi(strings.TrimSpace(xText))
		y, _ := strconv.Atoi(strings.TrimSpace(yText))
		moveAndSave(x, y-nudge)
	})
	downBtn.Connect("clicked", func() {
		xText, _ := xEntry.GetText()
		yText, _ := yEntry.GetText()
		x, _ := strconv.Atoi(strings.TrimSpace(xText))
		y, _ := strconv.Atoi(strings.TrimSpace(yText))
		moveAndSave(x, y+nudge)
	})

	getViewMode := func() config.ViewMode {
		if simpleBtn.GetActive() {
			return config.ViewModeSimple
		}
		return config.ViewModeEnhanced
	}

	return vbox, opacityScale, noBgCheck, noBorderCheck, iconCombo, getViewMode
}

// showErrorDialog shows a simple error message dialog.
func showErrorDialog(parent *gtk.Dialog, msg string) {
	ed := gtk.MessageDialogNew(parent, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, "%s", msg)
	ed.Run()
	ed.Destroy()
}

// showProUpgradeDialog displays an eye-candy modal dialog highlighting the benefits of Pro tier.
func showProUpgradeDialog(parent *gtk.Dialog, m *manager, onGoToProvider func()) {
	dlg, err := gtk.DialogNew()
	if err != nil {
		return
	}
	dlg.SetTransientFor(parent)
	dlg.SetModal(true)
	dlg.SetTitle(m.t("dialog.pro.title"))
	dlg.SetDefaultSize(480, -1)
	dlg.SetResizable(false)

	contentArea, err := dlg.GetContentArea()
	if err != nil {
		return
	}

	mainBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 14)
	mainBox.SetMarginTop(18)
	mainBox.SetMarginBottom(18)
	mainBox.SetMarginStart(20)
	mainBox.SetMarginEnd(20)

	// CSS for eye-candy styling
	proCSS := `
.pro-header-badge {
    border-radius: 20px;
    padding: 4px 14px;
    background: linear-gradient(135deg, #f59e0b, #d97706);
    color: #ffffff;
    font-weight: bold;
    font-size: 11px;
}
.pro-feature-item {
    background: rgba(30, 41, 59, 0.75);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 10px;
    padding: 10px 14px;
    transition: all 0.15s ease-in-out;
}
.pro-feature-item:hover {
    background: rgba(51, 65, 85, 0.85);
    border-color: rgba(56, 189, 248, 0.35);
}
.pro-cta-button {
    background: linear-gradient(135deg, #0284c7, #0369a1);
    color: #ffffff;
    font-weight: bold;
    font-size: 13px;
    border-radius: 8px;
    padding: 10px 18px;
    border: none;
}
.pro-cta-button:hover {
    background: linear-gradient(135deg, #0ea5e9, #0284c7);
}
.pro-dismiss-button {
    background: rgba(255, 255, 255, 0.06);
    color: #94a3b8;
    border-radius: 8px;
    padding: 10px 16px;
    border: 1px solid rgba(255, 255, 255, 0.1);
}
.pro-dismiss-button:hover {
    background: rgba(255, 255, 255, 0.12);
    color: #f8fafc;
}
`
	cssProvider, _ := gtk.CssProviderNew()
	cssProvider.LoadFromData(proCSS)

	// Top Badge & Title Box
	headerBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	headerBox.SetHAlign(gtk.ALIGN_CENTER)

	badgeLbl, _ := gtk.LabelNew("⭐ PRO UPGRADE")
	badgeSc, _ := badgeLbl.GetStyleContext()
	badgeSc.AddProvider(cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	badgeSc.AddClass("pro-header-badge")
	headerBox.PackStart(badgeLbl, false, false, 0)

	titleLbl, _ := gtk.LabelNew("")
	titleLbl.SetMarkup(fmt.Sprintf("<span font_size='15000' font_weight='bold' foreground='#38bdf8'>%s</span>", glib.MarkupEscapeText(m.t("dialog.pro.title"))))
	titleLbl.SetHAlign(gtk.ALIGN_CENTER)
	headerBox.PackStart(titleLbl, false, false, 0)

	subtitleLbl, _ := gtk.LabelNew("")
	subtitleLbl.SetMarkup(fmt.Sprintf("<span font_size='9500' foreground='#94a3b8'>%s</span>", glib.MarkupEscapeText(m.t("dialog.pro.subtitle"))))
	subtitleLbl.SetLineWrap(true)
	subtitleLbl.SetHAlign(gtk.ALIGN_CENTER)
	subtitleLbl.SetJustify(gtk.JUSTIFY_CENTER)
	headerBox.PackStart(subtitleLbl, false, false, 0)

	mainBox.PackStart(headerBox, false, false, 0)

	// Features list
	featuresBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	features := []struct {
		icon  string
		title string
		desc  string
	}{
		{"🏙️", m.t("dialog.pro.feature1"), m.t("dialog.pro.feature1_desc")},
		{"⚡", m.t("dialog.pro.feature2"), m.t("dialog.pro.feature2_desc")},
		{"📊", m.t("dialog.pro.feature3"), m.t("dialog.pro.feature3_desc")},
	}

	for _, f := range features {
		fBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
		fSc, _ := fBox.GetStyleContext()
		fSc.AddProvider(cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
		fSc.AddClass("pro-feature-item")

		iconLbl, _ := gtk.LabelNew("")
		iconLbl.SetMarkup(fmt.Sprintf("<span font_size='16000'>%s</span>", f.icon))
		fBox.PackStart(iconLbl, false, false, 0)

		textVBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
		fTitle, _ := gtk.LabelNew("")
		fTitle.SetMarkup(fmt.Sprintf("<span font_weight='bold' font_size='10500' foreground='#f8fafc'>%s</span>", glib.MarkupEscapeText(f.title)))
		fTitle.SetHAlign(gtk.ALIGN_START)
		textVBox.PackStart(fTitle, false, false, 0)

		fDesc, _ := gtk.LabelNew("")
		fDesc.SetMarkup(fmt.Sprintf("<span font_size='8500' foreground='#cbd5e1'>%s</span>", glib.MarkupEscapeText(f.desc)))
		fDesc.SetHAlign(gtk.ALIGN_START)
		fDesc.SetLineWrap(true)
		textVBox.PackStart(fDesc, false, false, 0)

		fBox.PackStart(textVBox, true, true, 0)
		featuresBox.PackStart(fBox, false, false, 0)
	}

	mainBox.PackStart(featuresBox, false, false, 0)

	// Action buttons
	btnBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	btnBox.SetHAlign(gtk.ALIGN_FILL)
	btnBox.SetMarginTop(8)

	dismissBtn, _ := gtk.ButtonNewWithLabel(m.t("dialog.pro.dismiss"))
	dismissSc, _ := dismissBtn.GetStyleContext()
	dismissSc.AddProvider(cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	dismissSc.AddClass("pro-dismiss-button")
	dismissBtn.Connect("clicked", func() {
		dlg.Destroy()
	})
	btnBox.PackStart(dismissBtn, true, true, 0)

	ctaBtn, _ := gtk.ButtonNew()
	ctaBtn.SetLabel("🚀  " + m.t("dialog.pro.cta"))
	ctaSc, _ := ctaBtn.GetStyleContext()
	ctaSc.AddProvider(cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	ctaSc.AddClass("pro-cta-button")
	ctaBtn.Connect("clicked", func() {
		dlg.Destroy()
		if onGoToProvider != nil {
			onGoToProvider()
		}
	})
	btnBox.PackStart(ctaBtn, true, true, 0)

	mainBox.PackStart(btnBox, false, false, 0)

	contentArea.Add(mainBox)
	dlg.ShowAll()
}

// fontSizes holds the three live font size values threaded through the Widget tab.
type fontSizes struct {
	cityTime   int
	tempIcon   int
	conditions int
}

// buildWidgetTab creates the Widget tab for controlling panel element visibility,
// pollution metrics, temperature unit, and wind speed unit.
// Returns the tab box, a getDisplayFields() func, a getPollutionFields() func,
// a getTempUnit() func, and a getWindUnit() func.
func buildWidgetTab(m *manager) (*gtk.Box, func() *config.DisplayFields, func() *config.PollutionFields, func() config.TemperatureUnit, func() config.WindSpeedUnit) {
	vbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 12)
	vbox.SetMarginTop(12)
	vbox.SetMarginStart(8)
	vbox.SetMarginEnd(8)

	// ── Panel Display ─────────────────────────────────────────────────────
	displayTitle, _ := gtk.LabelNew("")
	displayTitle.SetMarkup("<b>" + glib.MarkupEscapeText(m.t("settings.display.title")) + "</b>")
	displayTitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(displayTitle, false, false, 0)

	displaySubtitle, _ := gtk.LabelNew(m.t("settings.display.subtitle"))
	displaySubtitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(displaySubtitle, false, false, 0)

	df := m.cfg.GetDisplayFields()

	// 4-column grid of checkboxes matching the screenshot layout.
	grid, _ := gtk.GridNew()
	grid.SetRowSpacing(8)
	grid.SetColumnSpacing(16)
	grid.SetMarginStart(4)

	chkCity, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.city"))
	chkCity.SetActive(df.ShowCity)
	grid.Attach(chkCity, 0, 0, 1, 1)

	chkIcon, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.icon"))
	chkIcon.SetActive(df.ShowIcon)
	grid.Attach(chkIcon, 1, 0, 1, 1)

	chkTemp, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.temp"))
	chkTemp.SetActive(df.ShowTemp)
	grid.Attach(chkTemp, 2, 0, 1, 1)

	chkDesc, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.desc"))
	chkDesc.SetActive(df.ShowDesc)
	grid.Attach(chkDesc, 3, 0, 1, 1)

	chkHumidity, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.humidity"))
	chkHumidity.SetActive(df.ShowHumidity)
	grid.Attach(chkHumidity, 0, 1, 1, 1)

	chkWind, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.wind"))
	chkWind.SetActive(df.ShowWind)
	grid.Attach(chkWind, 1, 1, 1, 1)

	chkTime, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.time"))
	chkTime.SetActive(df.ShowTime)
	grid.Attach(chkTime, 2, 1, 1, 1)

	chkDate, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.date"))
	chkDate.SetActive(df.ShowDate)
	grid.Attach(chkDate, 3, 1, 1, 1)

	chkWindGust, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.windGust"))
	chkWindGust.SetActive(df.ShowWindGust)
	grid.Attach(chkWindGust, 0, 2, 1, 1)

	chkDewPoint, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.dewPoint"))
	chkDewPoint.SetActive(df.ShowDewPoint)
	grid.Attach(chkDewPoint, 1, 2, 1, 1)

	chkPressure, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.pressure"))
	chkPressure.SetActive(df.ShowPressure)
	grid.Attach(chkPressure, 2, 2, 1, 1)

	chkUVIndex, _ := gtk.CheckButtonNewWithLabel(m.t("settings.display.uvIndex"))
	chkUVIndex.SetActive(df.ShowUVIndex)
	grid.Attach(chkUVIndex, 3, 2, 1, 1)

	vbox.PackStart(grid, false, false, 0)

	// ── Pollution ─────────────────────────────────────────────────────────
	sepPollution, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(sepPollution, false, false, 4)

	pollutionTitle, _ := gtk.LabelNew("")
	pollutionTitle.SetMarkup("<b>" + glib.MarkupEscapeText(m.t("settings.pollution.title")) + "</b>")
	pollutionTitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(pollutionTitle, false, false, 0)

	pollutionSubtitle, _ := gtk.LabelNew(m.t("settings.pollution.subtitle"))
	pollutionSubtitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(pollutionSubtitle, false, false, 0)

	// Pro note banner
	proBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	proBoxSc, _ := proBox.GetStyleContext()
	proNoteCSS, _ := gtk.CssProviderNew()
	proNoteCSS.LoadFromData(`
.pro-note-box {
    border-radius: 8px;
    padding: 8px 12px;
    background: rgba(14, 165, 233, 0.1);
    border: 1px solid rgba(56, 189, 248, 0.35);
}
`)
	proBoxSc.AddProvider(proNoteCSS, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	proBoxSc.AddClass("pro-note-box")

	proNote, _ := gtk.LabelNew("")
	proNote.SetMarkup(fmt.Sprintf("<span foreground='#38bdf8' font_size='9500'>%s</span>", glib.MarkupEscapeText(m.t("settings.pollution.proNote"))))
	proNote.SetHAlign(gtk.ALIGN_START)
	proNote.SetLineWrap(true)
	proBox.PackStart(proNote, true, true, 0)
	vbox.PackStart(proBox, false, false, 0)

	pf := m.cfg.GetPollutionFields()
	isPro := m.cfg.IsPro()

	gridPollution, _ := gtk.GridNew()
	gridPollution.SetRowSpacing(8)
	gridPollution.SetColumnSpacing(16)
	gridPollution.SetMarginStart(4)

	// AQI leads the group (canonical order: AQI first). The nine checkboxes
	// fill a 4-column grid: row 0 = AQI,CO,NO,NO2; row 1 = O3,SO2,NH3,PM25;
	// row 2 = PM10.
	chkAQI, _ := gtk.CheckButtonNewWithLabel(m.t("settings.pollution.aqi"))
	chkAQI.SetActive(pf.ShowAQI)
	chkAQI.SetSensitive(isPro)
	gridPollution.Attach(chkAQI, 0, 0, 1, 1)

	chkCO, _ := gtk.CheckButtonNewWithLabel(m.t("settings.pollution.co"))
	chkCO.SetActive(pf.ShowCO)
	chkCO.SetSensitive(isPro)
	gridPollution.Attach(chkCO, 1, 0, 1, 1)

	chkNO, _ := gtk.CheckButtonNewWithLabel(m.t("settings.pollution.no"))
	chkNO.SetActive(pf.ShowNO)
	chkNO.SetSensitive(isPro)
	gridPollution.Attach(chkNO, 2, 0, 1, 1)

	chkNO2, _ := gtk.CheckButtonNewWithLabel(m.t("settings.pollution.no2"))
	chkNO2.SetActive(pf.ShowNO2)
	chkNO2.SetSensitive(isPro)
	gridPollution.Attach(chkNO2, 3, 0, 1, 1)

	chkO3, _ := gtk.CheckButtonNewWithLabel(m.t("settings.pollution.o3"))
	chkO3.SetActive(pf.ShowO3)
	chkO3.SetSensitive(isPro)
	gridPollution.Attach(chkO3, 0, 1, 1, 1)

	chkSO2, _ := gtk.CheckButtonNewWithLabel(m.t("settings.pollution.so2"))
	chkSO2.SetActive(pf.ShowSO2)
	chkSO2.SetSensitive(isPro)
	gridPollution.Attach(chkSO2, 1, 1, 1, 1)

	chkNH3, _ := gtk.CheckButtonNewWithLabel(m.t("settings.pollution.nh3"))
	chkNH3.SetActive(pf.ShowNH3)
	chkNH3.SetSensitive(isPro)
	gridPollution.Attach(chkNH3, 2, 1, 1, 1)

	chkPM25, _ := gtk.CheckButtonNewWithLabel(m.t("settings.pollution.pm2_5"))
	chkPM25.SetActive(pf.ShowPM25)
	chkPM25.SetSensitive(isPro)
	gridPollution.Attach(chkPM25, 3, 1, 1, 1)

	chkPM10, _ := gtk.CheckButtonNewWithLabel(m.t("settings.pollution.pm10"))
	chkPM10.SetActive(pf.ShowPM10)
	chkPM10.SetSensitive(isPro)
	gridPollution.Attach(chkPM10, 0, 2, 1, 1)

	vbox.PackStart(gridPollution, false, false, 0)

	getDisplayFields := func() *config.DisplayFields {
		return &config.DisplayFields{
			ShowCity:     chkCity.GetActive(),
			ShowIcon:     chkIcon.GetActive(),
			ShowTemp:     chkTemp.GetActive(),
			ShowDesc:     chkDesc.GetActive(),
			ShowHumidity: chkHumidity.GetActive(),
			ShowWind:     chkWind.GetActive(),
			ShowTime:     chkTime.GetActive(),
			ShowDate:     chkDate.GetActive(),
			ShowWindGust: chkWindGust.GetActive(),
			ShowDewPoint: chkDewPoint.GetActive(),
			ShowPressure: chkPressure.GetActive(),
			ShowUVIndex:  chkUVIndex.GetActive(),
		}
	}

	getPollutionFields := func() *config.PollutionFields {
		return &config.PollutionFields{
			ShowAQI:  chkAQI.GetActive(),
			ShowCO:   chkCO.GetActive(),
			ShowNO:   chkNO.GetActive(),
			ShowNO2:  chkNO2.GetActive(),
			ShowO3:   chkO3.GetActive(),
			ShowSO2:  chkSO2.GetActive(),
			ShowNH3:  chkNH3.GetActive(),
			ShowPM25: chkPM25.GetActive(),
			ShowPM10: chkPM10.GetActive(),
		}
	}

	// ── Temperature Unit ──────────────────────────────────────────────────
	sep, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(sep, false, false, 4)

	tempTitle, _ := gtk.LabelNew("")
	tempTitle.SetMarkup("<b>" + glib.MarkupEscapeText(m.t("settings.temperature.title")) + "</b>")
	tempTitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(tempTitle, false, false, 0)

	tempSubtitle, _ := gtk.LabelNew(m.t("settings.temperature.subtitle"))
	tempSubtitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(tempSubtitle, false, false, 0)

	hbox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 16)
	celsiusBtn, _ := gtk.RadioButtonNewWithLabel(nil, "°C (Celsius)")
	fahBtn, _ := gtk.RadioButtonNewWithLabelFromWidget(celsiusBtn, "°F (Fahrenheit)")
	if m.cfg.TemperatureUnit == config.TemperatureUnitFahrenheit {
		fahBtn.SetActive(true)
	} else {
		celsiusBtn.SetActive(true)
	}
	hbox.PackStart(celsiusBtn, false, false, 0)
	hbox.PackStart(fahBtn, false, false, 0)
	vbox.PackStart(hbox, false, false, 0)

	// ── Wind Speed Unit ──────────────────────────────────────────────────
	sep2, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(sep2, false, false, 4)

	windTitle, _ := gtk.LabelNew("")
	windTitle.SetMarkup("<b>" + glib.MarkupEscapeText(m.t("settings.windspeed.title")) + "</b>")
	windTitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(windTitle, false, false, 0)

	windSubtitle, _ := gtk.LabelNew(m.t("settings.windspeed.subtitle"))
	windSubtitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(windSubtitle, false, false, 0)

	hboxWind, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 16)
	kmhBtn, _ := gtk.RadioButtonNewWithLabel(nil, "km/h")
	mphBtn, _ := gtk.RadioButtonNewWithLabelFromWidget(kmhBtn, "mph")
	knotsBtn, _ := gtk.RadioButtonNewWithLabelFromWidget(kmhBtn, "knots")
	switch m.cfg.WindSpeedUnit {
	case config.WindSpeedUnitMph:
		mphBtn.SetActive(true)
	case config.WindSpeedUnitKnots:
		knotsBtn.SetActive(true)
	default:
		kmhBtn.SetActive(true)
	}
	hboxWind.PackStart(kmhBtn, false, false, 0)
	hboxWind.PackStart(mphBtn, false, false, 0)
	hboxWind.PackStart(knotsBtn, false, false, 0)
	vbox.PackStart(hboxWind, false, false, 0)

	getTempUnit := func() config.TemperatureUnit {
		if fahBtn.GetActive() {
			return config.TemperatureUnitFahrenheit
		}
		return config.TemperatureUnitCelsius
	}

	getWindUnit := func() config.WindSpeedUnit {
		if mphBtn.GetActive() {
			return config.WindSpeedUnitMph
		}
		if knotsBtn.GetActive() {
			return config.WindSpeedUnitKnots
		}
		return config.WindSpeedUnitKmh
	}

	return vbox, getDisplayFields, getPollutionFields, getTempUnit, getWindUnit
}

// buildLocationsTab builds the city management tab.
// Returns the tab widget and a getCities() func that returns the current list
// at the time of calling (used by the Save button in showSettingsDialog).
func buildLocationsTab(m *manager, dlg *gtk.Dialog, initialCities []config.CityConfig, onGoToProvider func()) (*gtk.Box, func() []config.CityConfig) {
	cities := make([]config.CityConfig, len(initialCities))
	copy(cities, initialCities)

	hasLicense := m.cfg.HasLicense()

	outer, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	outer.SetMarginTop(8)
	outer.SetMarginStart(4)
	outer.SetMarginEnd(4)

	// ── Saved Cities section ──────────────────────────────────────────────
	savedTitle, _ := gtk.LabelNew("")
	savedTitle.SetMarkup("<b>" + glib.MarkupEscapeText(m.t("settings.locations.savedTitle")) + "</b>")
	savedTitle.SetHAlign(gtk.ALIGN_START)
	outer.PackStart(savedTitle, false, false, 0)

	subTitle, _ := gtk.LabelNew(m.t("settings.locations.savedSubtitle"))
	subTitle.SetHAlign(gtk.ALIGN_START)
	outer.PackStart(subTitle, false, false, 0)

	// Callout box for Pro note
	proBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	proBox.SetMarginTop(2)
	proBox.SetMarginBottom(6)
	proBoxSc, _ := proBox.GetStyleContext()
	proNoteCSS, _ := gtk.CssProviderNew()
	proNoteCSS.LoadFromData(`
.pro-note-box {
    border-radius: 8px;
    padding: 8px 12px;
    background: rgba(14, 165, 233, 0.1);
    border: 1px solid rgba(56, 189, 248, 0.35);
}
`)
	proBoxSc.AddProvider(proNoteCSS, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	proBoxSc.AddClass("pro-note-box")

	proNote, _ := gtk.LabelNew("")
	proNote.SetMarkup(fmt.Sprintf("<span foreground='#38bdf8' font_size='9500'>%s</span>", glib.MarkupEscapeText(m.t("settings.locations.proNote"))))
	proNote.SetHAlign(gtk.ALIGN_START)
	proNote.SetLineWrap(true)
	proBox.PackStart(proNote, true, true, 0)
	outer.PackStart(proBox, false, false, 0)

	// Scrollable city list.
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	scroll.SetSizeRequest(-1, 200)

	listBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
	scroll.Add(listBox)
	outer.PackStart(scroll, false, false, 0)

	// refreshList rebuilds the city rows from the current cities slice.
	var refreshList func()
	refreshList = func() {
		// Remove all existing rows.
		listBox.GetChildren().Foreach(func(item interface{}) {
			if w, ok := item.(gtk.IWidget); ok {
				listBox.Remove(w)
			}
		})

		for i := range cities {
			idx := i
			city := cities[i]

			row, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
			row.SetMarginStart(4)
			row.SetMarginEnd(4)

			// City label.
			lbl, _ := gtk.LabelNew(fmt.Sprintf("<b>%s, %s</b>", city.Name, city.Region))
			lbl.SetUseMarkup(true)
			lbl.SetHAlign(gtk.ALIGN_START)
			lbl.SetHExpand(true)
			row.PackStart(lbl, true, true, 0)

			if hasLicense {
				upBtn, _ := gtk.ButtonNewWithLabel("↑")
				upBtn.SetSensitive(idx > 0)
				upBtn.Connect("clicked", func() {
					if idx > 0 {
						cities[idx], cities[idx-1] = cities[idx-1], cities[idx]
						refreshList()
					}
				})
				row.PackStart(upBtn, false, false, 0)

				// Move Down button.
				downBtn, _ := gtk.ButtonNewWithLabel("↓")
				downBtn.SetSensitive(idx < len(cities)-1)
				downBtn.Connect("clicked", func() {
					if idx < len(cities)-1 {
						cities[idx], cities[idx+1] = cities[idx+1], cities[idx]
						refreshList()
					}
				})
				row.PackStart(downBtn, false, false, 0)

				// Delete button.
				delBtn, _ := gtk.ButtonNewWithLabel("🗑")
				delBtn.SetSensitive(len(cities) > 1)
				delBtn.Connect("clicked", func() {
					result, err := config.RemoveCity(cities, idx, nil)
					if err != nil {
						showErrorDialog(dlg, err.Error())
						return
					}
					cities = result
					refreshList()
				})
				row.PackStart(delBtn, false, false, 0)
			}

			listBox.PackStart(row, false, false, 0)

			// Separator between rows.
			if idx < len(cities)-1 {
				sep, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
				listBox.PackStart(sep, false, false, 0)
			}
		}
		listBox.ShowAll()
	}
	refreshList()

	// ── Add New City section ──────────────────────────────────────────────
	sep, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	outer.PackStart(sep, false, false, 4)

	addTitle, _ := gtk.LabelNew("")
	addTitle.SetMarkup("<b>" + glib.MarkupEscapeText(m.t("settings.locations.addTitle")) + "</b>")
	addTitle.SetHAlign(gtk.ALIGN_START)
	outer.PackStart(addTitle, false, false, 0)

	grid, _ := gtk.GridNew()
	grid.SetRowSpacing(6)
	grid.SetColumnSpacing(8)
	grid.SetMarginStart(4)

	makeLabel := func(text string) *gtk.Label {
		l, _ := gtk.LabelNew(text)
		l.SetHAlign(gtk.ALIGN_END)
		return l
	}

	// Name row with Search API button.
	nameLabel := makeLabel(m.t("settings.locations.nameLabel"))
	nameEntry, _ := gtk.EntryNew()
	nameEntry.SetPlaceholderText(m.t("settings.locations.namePlaceholder"))
	nameEntry.SetHExpand(true)
	searchBtn, _ := gtk.ButtonNewWithLabel(m.t("settings.locations.searchBtn"))
	nameRow, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	nameRow.SetHExpand(true)
	nameRow.PackStart(nameEntry, true, true, 0)
	nameRow.PackStart(searchBtn, false, false, 0)
	grid.Attach(nameLabel, 0, 0, 1, 1)
	grid.Attach(nameRow, 1, 0, 1, 1)

	// Region row.
	regionLabel := makeLabel(m.t("settings.locations.regionLabel"))
	regionEntry, _ := gtk.EntryNew()
	regionEntry.SetPlaceholderText(m.t("settings.locations.regionPlaceholder"))
	regionEntry.SetHExpand(true)
	grid.Attach(regionLabel, 0, 1, 1, 1)
	grid.Attach(regionEntry, 1, 1, 1, 1)

	// Coordinates row.
	coordLabel := makeLabel(m.t("settings.locations.coordLabel"))
	coordBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	latEntry, _ := gtk.EntryNew()
	latEntry.SetPlaceholderText(m.t("settings.locations.latPlaceholder"))
	latEntry.SetHExpand(true)
	lonEntry, _ := gtk.EntryNew()
	lonEntry.SetPlaceholderText(m.t("settings.locations.lonPlaceholder"))
	lonEntry.SetHExpand(true)
	coordBox.PackStart(latEntry, true, true, 0)
	coordBox.PackStart(lonEntry, true, true, 0)
	grid.Attach(coordLabel, 0, 2, 1, 1)
	grid.Attach(coordBox, 1, 2, 1, 1)

	// Timezone row.
	tzLabel := makeLabel(m.t("settings.locations.tzLabel"))
	tzEntry, _ := gtk.EntryNew()
	tzEntry.SetPlaceholderText(m.t("settings.locations.tzPlaceholder"))
	tzEntry.SetHExpand(true)
	grid.Attach(tzLabel, 0, 3, 1, 1)
	grid.Attach(tzEntry, 1, 3, 1, 1)

	outer.PackStart(grid, false, false, 0)

	if !hasLicense {
		nameEntry.SetSensitive(false)
		regionEntry.SetSensitive(false)
		latEntry.SetSensitive(false)
		lonEntry.SetSensitive(false)
		tzEntry.SetSensitive(false)
		searchBtn.SetSensitive(false)
	}

	// Status label for search feedback.
	statusLbl, _ := gtk.LabelNew("")
	statusLbl.SetHAlign(gtk.ALIGN_START)
	outer.PackStart(statusLbl, false, false, 0)

	// Search API button handler.
	searchBtn.Connect("clicked", func() {
		if !hasLicense {
			showErrorDialog(dlg, m.t("error.settings.licenseRequired"))
			return
		}
		name, _ := nameEntry.GetText()
		name = strings.TrimSpace(name)
		if name == "" {
			showErrorDialog(dlg, m.t("error.settings.cityNameRequiredSearch"))
			return
		}
		apiKey := ""
		if m.cfg.APIConfig != nil {
			apiKey = m.cfg.APIConfig.APIKey
		}
		if apiKey == "" {
			showErrorDialog(dlg, m.t("error.settings.apiKeyRequired"))
			return
		}

		provider := "easyweatherwidget"
		if m.cfg.APIConfig != nil && m.cfg.APIConfig.Provider != "" {
			provider = m.cfg.APIConfig.Provider
		}

		statusLbl.SetText(m.t("settings.locations.searching"))
		searchBtn.SetSensitive(false)

		region, _ := regionEntry.GetText()
		region = strings.TrimSpace(region)

		go func() {
			defer glib.IdleAdd(func() { searchBtn.SetSensitive(true) })

			var foundName, foundRegion, foundTZ string
			var foundLat, foundLon float64
			var searchErr error

			switch provider {
			case "easyweatherwidget":
				if region == "" {
					glib.IdleAdd(func() {
						statusLbl.SetText(m.t("error.settings.regionRequiredEww"))
					})
					return
				}
				query := name + "," + region
				uStr := fmt.Sprintf("https://weather-gateway-ricardo.web.app/api/v1/weather/key=%s/%s",
					url.PathEscape(apiKey), url.PathEscape(query))
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				req, _ := http.NewRequestWithContext(ctx, "GET", uStr, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					searchErr = err
					break
				}
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				var result struct {
					Neighborhood string  `json:"Neighborhood"`
					Country      string  `json:"Country"`
					Lat          float64 `json:"Lat"`
					Lon          float64 `json:"Lon"`
				}
				if err := json.Unmarshal(body, &result); err != nil || result.Neighborhood == "" {
					glib.IdleAdd(func() { statusLbl.SetText(fmt.Sprintf(m.t("error.settings.noCityFound"), name)) })
					return
				}
				foundName = result.Neighborhood
				foundRegion = result.Country
				foundLat = result.Lat
				foundLon = result.Lon
				foundTZ = lookupTimezone(foundLat, foundLon)

			default: // openweathermap
				uStr := fmt.Sprintf("http://api.openweathermap.org/geo/1.0/direct?q=%s&limit=1&appid=%s",
					url.QueryEscape(name), url.QueryEscape(apiKey))
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				req, _ := http.NewRequestWithContext(ctx, "GET", uStr, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					searchErr = err
					break
				}
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				var results []struct {
					Name    string  `json:"name"`
					Lat     float64 `json:"lat"`
					Lon     float64 `json:"lon"`
					Country string  `json:"country"`
					State   string  `json:"state"`
				}
				if err := json.Unmarshal(body, &results); err != nil || len(results) == 0 {
					glib.IdleAdd(func() { statusLbl.SetText(fmt.Sprintf(m.t("error.settings.noCityFound"), name)) })
					return
				}
				r := results[0]
				foundName = r.Name
				foundRegion = r.Country
				if r.State != "" {
					foundRegion = r.State + ", " + r.Country
				}
				foundLat = r.Lat
				foundLon = r.Lon
				foundTZ = lookupTimezone(foundLat, foundLon)
			}

			if searchErr != nil {
				glib.IdleAdd(func() { statusLbl.SetText(fmt.Sprintf(m.t("error.settings.searchFailed"), searchErr)) })
				return
			}

			glib.IdleAdd(func() {
				nameEntry.SetText(foundName)
				regionEntry.SetText(foundRegion)
				latEntry.SetText(fmt.Sprintf("%f", foundLat))
				lonEntry.SetText(fmt.Sprintf("%f", foundLon))
				tzEntry.SetText(foundTZ)
				statusLbl.SetText("✓ " + foundName + ", " + foundRegion)
			})
		}()
	})

	// Add City button.
	btnBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	addBtn, _ := gtk.ButtonNewWithLabel(m.t("settings.locations.addBtn"))
	addBtn.SetHAlign(gtk.ALIGN_END)
	if !hasLicense {
		addBtn.SetSensitive(false)
	}
	btnBox.PackEnd(addBtn, false, false, 0)
	outer.PackStart(btnBox, false, false, 0)

	addBtn.Connect("clicked", func() {
		if !hasLicense {
			showErrorDialog(dlg, m.t("error.settings.licenseRequired"))
			return
		}
		name, _ := nameEntry.GetText()
		name = strings.TrimSpace(name)
		if name == "" {
			showErrorDialog(dlg, m.t("error.settings.cityNameRequired"))
			return
		}
		region, _ := regionEntry.GetText()
		tzStr, _ := tzEntry.GetText()
		latStr, _ := latEntry.GetText()
		lonStr, _ := lonEntry.GetText()

		newCity := config.CityConfig{
			Name:     name,
			Region:   strings.TrimSpace(region),
			Timezone: strings.TrimSpace(tzStr),
		}
		if lat, err := strconv.ParseFloat(strings.TrimSpace(latStr), 64); err == nil {
			newCity.Latitude = lat
		}
		if lon, err := strconv.ParseFloat(strings.TrimSpace(lonStr), 64); err == nil {
			newCity.Longitude = lon
		}

		maxCities := config.MaxCitiesFree
		if m.cfg.IsPro() {
			maxCities = config.MaxCitiesPro
		}
		if !m.cfg.IsPro() && len(cities) >= config.MaxCitiesFree {
			showProUpgradeDialog(dlg, m, onGoToProvider)
			return
		}
		var tFunc config.TranslateFunc
		if m.lm != nil {
			tFunc = m.lm.T
		}
		result, err := config.AddCityWithLimit(cities, newCity, maxCities, tFunc)
		if err != nil {
			if !m.cfg.IsPro() && len(cities) >= config.MaxCitiesFree {
				showProUpgradeDialog(dlg, m, onGoToProvider)
			} else {
				showErrorDialog(dlg, err.Error())
			}
			return
		}
		cities = result

		// Clear the form.
		nameEntry.SetText("")
		regionEntry.SetText("")
		latEntry.SetText("")
		lonEntry.SetText("")
		tzEntry.SetText("")
		statusLbl.SetText("")

		refreshList()
	})

	return outer, func() []config.CityConfig {
		return cities
	}
}

// lookupTimezone attempts to find the IANA timezone for the given coordinates
// using the bradfitz/latlong package that is already a project dependency.
func lookupTimezone(lat, lon float64) string {
	// Import at call time to avoid circular build issues — the package is
	// already in go.mod as a direct dependency via the Fyne settings code.
	// We just call the same latlong.LookupZoneName used in settings.go.
	_ = lat
	_ = lon
	// Fallback: return empty string — user can fill in manually.
	// A proper call would be: return latlong.LookupZoneName(lat, lon)
	// but importing bradfitz/latlong here would require adding it to the
	// ui-gtk package imports. We do it inline below.
	return gtkLookupTimezone(lat, lon)
}

// gtkLocaleInfo holds display data for a single language option in GTK settings.
type gtkLocaleInfo struct {
	code       string // e.g. "en-GB"
	nativeName string // "English"
	subName    string // "English"
	flagFile   string // "icons/flags/en-GB.png"
}

// gtkLocaleData is the 2-column ordered list of supported languages.
var gtkLocaleData = []gtkLocaleInfo{
	{"en-GB", "English", "English", "icons/flags/en-GB.png"},
	{"es-ES", "Español", "Spanish", "icons/flags/es-ES.png"},
	{"fr-FR", "Français", "French", "icons/flags/fr-FR.png"},
	{"de-DE", "Deutsch", "German", "icons/flags/de-DE.png"},
	{"it-IT", "Italiano", "Italian", "icons/flags/it-IT.png"},
	{"pt-BR", "Português", "Português (BR)", "icons/flags/pt-BR.png"},
	{"nl-NL", "Nederlands", "Dutch", "icons/flags/nl-NL.png"},
	{"pl-PL", "Polski", "Polish", "icons/flags/pl-PL.png"},
	{"tr-TR", "Türkçe", "Turkish", "icons/flags/tr-TR.png"},
	{"ta-IN", "தமிழ்", "Tamil", "icons/flags/ta-IN.png"},
	{"ja-JP", "日本語", "Japanese", "icons/flags/ja-JP.png"},
	{"zh-CN", "中文", "Chinese", "icons/flags/zh-CN.png"},
}

// buildLanguageTab builds the language selection tab.
// Returns the tab widget and a getLocale() function that returns the currently
// selected locale code at call time (used by the Save button).
func buildLanguageTab(m *manager, initialLocale string) (*gtk.Box, func() string) {
	selected := initialLocale
	if selected == "" {
		selected = "en-GB"
	}

	outer, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	outer.SetMarginTop(12)
	outer.SetMarginStart(12)
	outer.SetMarginEnd(12)
	outer.SetMarginBottom(12)

	// ── Top Header Row ─────────────────────────────────────────
	headerBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)

	titleLeft, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	globeLbl, _ := gtk.LabelNew("")
	globeLbl.SetMarkup("<span font_size='14000' foreground='#38bdf8'>🌐</span>")
	titleLeft.PackStart(globeLbl, false, false, 0)

	titleLbl, _ := gtk.LabelNew("")
	titleLbl.SetMarkup(fmt.Sprintf("<span font_size='12500' font_weight='bold' foreground='#f8fafc'>%s</span>", glib.MarkupEscapeText(m.t("settings.language.title"))))
	titleLeft.PackStart(titleLbl, false, false, 0)
	headerBox.PackStart(titleLeft, false, false, 0)

	badgeLbl, _ := gtk.LabelNew(fmt.Sprintf("%d LANGUAGES", len(gtkLocaleData)))
	badgeSc, _ := badgeLbl.GetStyleContext()
	badgeSc.AddClass("lang-badge")
	headerBox.PackEnd(badgeLbl, false, false, 0)

	outer.PackStart(headerBox, false, false, 0)

	sep, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	outer.PackStart(sep, false, false, 4)

	// CSS for language buttons — badges and selection highlight.
	langCSS := `
.lang-pill {
    border-radius: 10px;
    padding: 6px 12px;
    border: 1.5px solid rgba(255, 255, 255, 0.08);
    background: rgba(30, 41, 59, 0.75);
    min-width: 175px;
    min-height: 48px;
    transition: all 0.15s ease-in-out;
}
.lang-pill:hover {
    background: rgba(51, 65, 85, 0.85);
    border-color: rgba(255, 255, 255, 0.25);
}
.lang-pill-selected {
    border: 2px solid #38bdf8;
    background: rgba(14, 165, 233, 0.18);
}
.lang-pill-selected:hover {
    background: rgba(14, 165, 233, 0.25);
    border-color: #38bdf8;
}
.lang-badge {
    border-radius: 12px;
    padding: 3px 10px;
    background: rgba(14, 165, 233, 0.15);
    border: 1px solid rgba(56, 189, 248, 0.4);
    color: #38bdf8;
    font-weight: bold;
    font-size: 11px;
}
`
	langProvider, _ := gtk.CssProviderNew()
	langProvider.LoadFromData(langCSS)

	// Apply CSS to badge
	badgeSc.AddProvider(langProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	// 2-column grid using a gtk.Grid.
	grid, _ := gtk.GridNew()
	grid.SetRowSpacing(8)
	grid.SetColumnSpacing(10)
	grid.SetHExpand(true)

	// Keep a reference to every button so we can toggle selection CSS.
	type btnRef struct {
		btn      *gtk.Button
		nameLbl  *gtk.Label
		checkLbl *gtk.Label
		code     string
		native   string
	}
	var buttons []btnRef

	var rebuildSelection func()

	for i, loc := range gtkLocaleData {
		loc := loc // capture
		col := i % 2
		row := i / 2

		btn, _ := gtk.ButtonNew()
		btn.SetRelief(gtk.RELIEF_NONE)
		btn.SetHExpand(true)

		// Inner horizontal layout
		hbox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
		hbox.SetMarginStart(4)
		hbox.SetMarginEnd(4)
		hbox.SetMarginTop(4)
		hbox.SetMarginBottom(4)

		// Flag image
		flagImg, _ := gtk.ImageNew()
		flagBytes, err := assets.Icons.ReadFile(loc.flagFile)
		if err == nil && len(flagBytes) > 0 {
			loader, err := gdk.PixbufLoaderNew()
			if err == nil {
				loader.Write(flagBytes)
				loader.Close()
				pb, err := loader.GetPixbuf()
				if err == nil && pb != nil {
					scaled, err := pb.ScaleSimple(34, 22, gdk.INTERP_BILINEAR)
					if err == nil && scaled != nil {
						flagImg.SetFromPixbuf(scaled)
					} else {
						flagImg.SetFromPixbuf(pb)
					}
				}
			}
		}
		hbox.PackStart(flagImg, false, false, 0)

		// Text vertical box
		vbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 1)
		vbox.SetHAlign(gtk.ALIGN_START)

		nameLbl, _ := gtk.LabelNew("")
		nameLbl.SetHAlign(gtk.ALIGN_START)
		vbox.PackStart(nameLbl, false, false, 0)

		subLbl, _ := gtk.LabelNew("")
		subLbl.SetMarkup(fmt.Sprintf("<span font_size='8500' foreground='#94a3b8'>%s</span>", glib.MarkupEscapeText(loc.subName)))
		subLbl.SetHAlign(gtk.ALIGN_START)
		vbox.PackStart(subLbl, false, false, 0)

		hbox.PackStart(vbox, true, true, 0)

		// Checkmark label on right
		checkLbl, _ := gtk.LabelNew("")
		checkLbl.SetHAlign(gtk.ALIGN_END)
		hbox.PackEnd(checkLbl, false, false, 0)

		btn.Add(hbox)

		// Apply base CSS class.
		sc, _ := btn.GetStyleContext()
		sc.AddProvider(langProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
		sc.AddClass("lang-pill")

		buttons = append(buttons, btnRef{
			btn:      btn,
			nameLbl:  nameLbl,
			checkLbl: checkLbl,
			code:     loc.code,
			native:   loc.nativeName,
		})

		btnCode := loc.code
		btn.Connect("clicked", func() {
			selected = btnCode
			rebuildSelection()
		})

		grid.Attach(btn, col, row, 1, 1)
	}

	// rebuildSelection updates the CSS selected class and label styles on all buttons.
	rebuildSelection = func() {
		for _, ref := range buttons {
			sc, _ := ref.btn.GetStyleContext()
			if ref.code == selected {
				sc.AddClass("lang-pill-selected")
				ref.nameLbl.SetMarkup(fmt.Sprintf("<span font_weight='bold' font_size='11000' foreground='#38bdf8'>%s</span>", glib.MarkupEscapeText(ref.native)))
				ref.checkLbl.SetMarkup("<span font_weight='bold' font_size='13000' foreground='#38bdf8'>✓</span>")
			} else {
				sc.RemoveClass("lang-pill-selected")
				ref.nameLbl.SetMarkup(fmt.Sprintf("<span font_weight='bold' font_size='11000' foreground='#f8fafc'>%s</span>", glib.MarkupEscapeText(ref.native)))
				ref.checkLbl.SetMarkup("")
			}
		}
	}
	rebuildSelection() // set initial selection

	outer.PackStart(grid, false, false, 0)

	return outer, func() string { return selected }
}

// buildAboutTab creates the About tab showing app info, links, and font size configuration.
func buildAboutTab(m *manager) (*gtk.Box, func() fontSizes) {
	vbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 12)
	vbox.SetMarginTop(20)
	vbox.SetMarginStart(20)
	vbox.SetMarginEnd(20)

	// App name heading.
	appNameLbl, _ := gtk.LabelNew("")
	appNameLbl.SetMarkup("<span font_size='xx-large' font_weight='bold'>" + glib.MarkupEscapeText(m.t("settings.about.appName")) + "</span>")
	appNameLbl.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(appNameLbl, false, false, 0)

	// Version.
	versionLbl, _ := gtk.LabelNew("")
	// Strip markdown bold markers from the i18n string ("**Version:** 1.0.6" → "Version: 1.0.6").
	versionText := m.t("settings.about.version")
	versionText = strings.ReplaceAll(versionText, "**", "")
	versionLbl.SetText(versionText)
	versionLbl.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(versionLbl, false, false, 0)

	sep, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(sep, false, false, 4)

	// Description.
	descLbl, _ := gtk.LabelNew(m.t("settings.about.description"))
	descLbl.SetHAlign(gtk.ALIGN_START)
	descLbl.SetLineWrap(true)
	descLbl.SetMaxWidthChars(70)
	vbox.PackStart(descLbl, false, false, 0)

	sep2, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(sep2, false, false, 4)

	// Links section in a styled frame with grid layout for alignment.
	linksFrame, _ := gtk.FrameNew("")
	linksFrame.SetShadowType(gtk.SHADOW_ETCHED_IN)
	sc, _ := linksFrame.GetStyleContext()
	sc.AddClass("links-frame")

	linksGrid, _ := gtk.GridNew()
	linksGrid.SetRowSpacing(4)
	linksGrid.SetColumnSpacing(12)
	linksGrid.SetMarginTop(12)
	linksGrid.SetMarginBottom(12)
	linksGrid.SetMarginStart(16)
	linksGrid.SetMarginEnd(16)

	// Website link.
	websiteLbl, _ := gtk.LabelNew(m.t("settings.about.websiteLabel"))
	websiteLbl.SetHAlign(gtk.ALIGN_END)
	websiteLink, _ := gtk.LinkButtonNewWithLabel(
		"https://easysmartapps.co.uk/weatherwidget",
		"easysmartapps.co.uk/weatherwidget",
	)
	websiteLink.SetHAlign(gtk.ALIGN_START)
	linksGrid.Attach(websiteLbl, 0, 0, 1, 1)
	linksGrid.Attach(websiteLink, 1, 0, 1, 1)

	// Manual link.
	manualLbl, _ := gtk.LabelNew(m.t("settings.about.manualLabel"))
	manualLbl.SetHAlign(gtk.ALIGN_END)
	manualLink, _ := gtk.LinkButtonNewWithLabel(
		"https://easysmartapps.co.uk/weatherwidget-manual",
		"easysmartapps.co.uk/weatherwidget-manual",
	)
	manualLink.SetHAlign(gtk.ALIGN_START)
	linksGrid.Attach(manualLbl, 0, 1, 1, 1)
	linksGrid.Attach(manualLink, 1, 1, 1, 1)

	// Air Index link.
	airIndexLbl, _ := gtk.LabelNew(m.t("settings.about.airIndexLabel"))
	airIndexLbl.SetHAlign(gtk.ALIGN_END)
	airIndexLink, _ := gtk.LinkButtonNewWithLabel(
		"https://easysmartapps.co.uk/weatherwidget-environmental",
		"easysmartapps.co.uk/weatherwidget-environmental",
	)
	airIndexLink.SetHAlign(gtk.ALIGN_START)
	linksGrid.Attach(airIndexLbl, 0, 2, 1, 1)
	linksGrid.Attach(airIndexLink, 1, 2, 1, 1)

	linksFrame.Add(linksGrid)
	vbox.PackStart(linksFrame, false, false, 0)

	// Font Size section
	sep3, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	vbox.PackStart(sep3, false, false, 4)

	fontTitle, _ := gtk.LabelNew("")
	fontTitle.SetMarkup("<b>" + glib.MarkupEscapeText(m.t("settings.fontSize.title")) + "</b>")
	fontTitle.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(fontTitle, false, false, 0)

	fontSubtitle, _ := gtk.LabelNew(m.t("settings.fontSize.subtitle"))
	fontSubtitle.SetHAlign(gtk.ALIGN_START)
	fontSubtitle.SetLineWrap(true)
	vbox.PackStart(fontSubtitle, false, false, 0)

	// Live values — kept in sync as the user clicks ▲ / ▼.
	curCityTime := m.fontSizeCityTime
	curTempIcon := m.fontSizeTempIcon
	curConditions := m.fontSizeConditions

	// applyLive fires SetFontSizes so the widget updates immediately.
	applyLive := func() {
		m.SetFontSizes(curCityTime, curTempIcon, curConditions)
	}

	// buildFontRow creates one labelled row: label — ▼ — Npx — ▲
	buildFontRow := func(labelKey string, initial, minVal, maxVal int, onChange func(int)) (*gtk.Box, func() int) {
		row, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)

		lbl, _ := gtk.LabelNew(m.t(labelKey))
		lbl.SetHAlign(gtk.ALIGN_START)
		lbl.SetHExpand(true)
		row.PackStart(lbl, true, true, 0)

		val := initial
		sizeLbl, _ := gtk.LabelNew(strconv.Itoa(val) + "px")
		sizeLbl.SetWidthChars(5)

		decBtn, _ := gtk.ButtonNewWithLabel("▼")
		incBtn, _ := gtk.ButtonNewWithLabel("▲")

		refresh := func() {
			sizeLbl.SetText(strconv.Itoa(val) + "px")
			onChange(val)
		}

		decBtn.Connect("clicked", func() {
			if val > minVal {
				val--
				refresh()
			}
		})
		incBtn.Connect("clicked", func() {
			if val < maxVal {
				val++
				refresh()
			}
		})

		row.PackStart(decBtn, false, false, 0)
		row.PackStart(sizeLbl, false, false, 0)
		row.PackStart(incBtn, false, false, 0)

		return row, func() int { return val }
	}

	// Font size controls in a styled frame.
	fontFrame, _ := gtk.FrameNew("")
	fontFrame.SetShadowType(gtk.SHADOW_ETCHED_IN)

	fontBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	fontBox.SetMarginTop(12)
	fontBox.SetMarginBottom(12)
	fontBox.SetMarginStart(16)
	fontBox.SetMarginEnd(16)

	rowCT, getCityTime := buildFontRow(
		"settings.fontSize.cityTime",
		curCityTime, 8, 48,
		func(v int) { curCityTime = v; applyLive() },
	)
	fontBox.PackStart(rowCT, false, false, 0)

	rowTI, getTempIconSize := buildFontRow(
		"settings.fontSize.tempIcon",
		curTempIcon, 10, 72,
		func(v int) { curTempIcon = v; applyLive() },
	)
	fontBox.PackStart(rowTI, false, false, 0)

	rowCond, getConditionsSize := buildFontRow(
		"settings.fontSize.conditions",
		curConditions, 6, 36,
		func(v int) { curConditions = v; applyLive() },
	)
	fontBox.PackStart(rowCond, false, false, 0)

	fontFrame.Add(fontBox)
	vbox.PackStart(fontFrame, false, false, 0)

	getFontSizes := func() fontSizes {
		return fontSizes{
			cityTime:   getCityTime(),
			tempIcon:   getTempIconSize(),
			conditions: getConditionsSize(),
		}
	}

	return vbox, getFontSizes
}
