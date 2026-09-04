package i18n

import (
	"sort"
	"testing"
	"testing/fstest"
)

// buildTestFS creates an in-memory FS with the given locale files.
func buildTestFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for name, content := range files {
		m["locales/"+name] = &fstest.MapFile{Data: []byte(content)}
	}
	return m
}

// validEnGB is a minimal valid en-GB locale file for testing.
const validEnGB = `{
  "_locale.code": "en-GB",
  "_locale.displayName": "English (UK)",
  "settings.title": "WeatherWidget Settings",
  "tray.quit": "Quit"
}`

// validPtBR is a minimal valid pt-BR locale file for testing.
const validPtBR = `{
  "_locale.code": "pt-BR",
  "_locale.displayName": "Português (Brasil)",
  "settings.title": "Configurações do WeatherWidget",
  "tray.quit": "Sair"
}`

// partialPtBR is a pt-BR file missing the "tray.quit" key.
const partialPtBR = `{
  "_locale.code": "pt-BR",
  "_locale.displayName": "Português (Brasil)",
  "settings.title": "Configurações do WeatherWidget"
}`

func TestNewLocaleManager_LoadsEnGB(t *testing.T) {
	fs := buildTestFS(map[string]string{"en-GB.json": validEnGB})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}
	if lm.ActiveLocale() != "en-GB" {
		t.Errorf("ActiveLocale() = %q, want %q", lm.ActiveLocale(), "en-GB")
	}
}

func TestNewLocaleManager_FailsWithoutEnGB(t *testing.T) {
	fs := buildTestFS(map[string]string{
		"pt-BR.json": validPtBR,
	})
	_, err := NewLocaleManager(fs)
	if err == nil {
		t.Fatal("NewLocaleManager() expected error when en-GB is missing")
	}
}

func TestNewLocaleManager_FailsWithEmptyDir(t *testing.T) {
	fs := fstest.MapFS{}
	_, err := NewLocaleManager(fs)
	if err == nil {
		t.Fatal("NewLocaleManager() expected error with empty FS")
	}
}

func TestSetLocale_SwitchesToPtBR(t *testing.T) {
	fs := buildTestFS(map[string]string{
		"en-GB.json": validEnGB,
		"pt-BR.json": validPtBR,
	})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	if err := lm.SetLocale("pt-BR"); err != nil {
		t.Fatalf("SetLocale(pt-BR) error = %v", err)
	}
	if lm.ActiveLocale() != "pt-BR" {
		t.Errorf("ActiveLocale() = %q, want %q", lm.ActiveLocale(), "pt-BR")
	}
	if got := lm.T("settings.title"); got != "Configurações do WeatherWidget" {
		t.Errorf("T(settings.title) = %q, want pt-BR translation", got)
	}
}

func TestSetLocale_SwitchesBackToEnGB(t *testing.T) {
	fs := buildTestFS(map[string]string{
		"en-GB.json": validEnGB,
		"pt-BR.json": validPtBR,
	})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	_ = lm.SetLocale("pt-BR")
	if err := lm.SetLocale("en-GB"); err != nil {
		t.Fatalf("SetLocale(en-GB) error = %v", err)
	}
	if lm.ActiveLocale() != "en-GB" {
		t.Errorf("ActiveLocale() = %q, want %q", lm.ActiveLocale(), "en-GB")
	}
	if got := lm.T("settings.title"); got != "WeatherWidget Settings" {
		t.Errorf("T(settings.title) = %q, want en-GB translation", got)
	}
}

func TestSetLocale_FallsBackOnInvalidLocale(t *testing.T) {
	fs := buildTestFS(map[string]string{"en-GB.json": validEnGB})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	err = lm.SetLocale("xx-XX")
	if err == nil {
		t.Fatal("SetLocale(xx-XX) expected error")
	}
	if lm.ActiveLocale() != "en-GB" {
		t.Errorf("ActiveLocale() = %q, want %q after fallback", lm.ActiveLocale(), "en-GB")
	}
}

func TestT_ReturnsTranslation(t *testing.T) {
	fs := buildTestFS(map[string]string{"en-GB.json": validEnGB})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	if got := lm.T("settings.title"); got != "WeatherWidget Settings" {
		t.Errorf("T(settings.title) = %q, want %q", got, "WeatherWidget Settings")
	}
}

func TestT_FallsBackToEnGB(t *testing.T) {
	fs := buildTestFS(map[string]string{
		"en-GB.json": validEnGB,
		"pt-BR.json": partialPtBR,
	})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	_ = lm.SetLocale("pt-BR")
	// "tray.quit" is missing in partialPtBR, should fall back to en-GB
	if got := lm.T("tray.quit"); got != "Quit" {
		t.Errorf("T(tray.quit) = %q, want %q (en-GB fallback)", got, "Quit")
	}
}

func TestT_ReturnsKeyWhenMissingEverywhere(t *testing.T) {
	fs := buildTestFS(map[string]string{"en-GB.json": validEnGB})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	key := "nonexistent.key"
	if got := lm.T(key); got != key {
		t.Errorf("T(%q) = %q, want key itself", key, got)
	}
}

func TestTWithArgs_FormatsString(t *testing.T) {
	enGB := `{
  "_locale.code": "en-GB",
  "_locale.displayName": "English (UK)",
  "validation.cities.count": "must contain 1 to 5 cities, got %d"
}`
	fs := buildTestFS(map[string]string{"en-GB.json": enGB})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	got := lm.TWithArgs("validation.cities.count", 7)
	want := "must contain 1 to 5 cities, got 7"
	if got != want {
		t.Errorf("TWithArgs() = %q, want %q", got, want)
	}
}

func TestTWithArgs_NoArgs(t *testing.T) {
	fs := buildTestFS(map[string]string{"en-GB.json": validEnGB})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	got := lm.TWithArgs("settings.title")
	want := "WeatherWidget Settings"
	if got != want {
		t.Errorf("TWithArgs() = %q, want %q", got, want)
	}
}

func TestAvailableLocales_ReturnsBoth(t *testing.T) {
	fs := buildTestFS(map[string]string{
		"en-GB.json": validEnGB,
		"pt-BR.json": validPtBR,
	})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	locales := lm.AvailableLocales()
	if len(locales) != 2 {
		t.Fatalf("AvailableLocales() returned %d locales, want 2", len(locales))
	}

	codes := make([]string, len(locales))
	for i, l := range locales {
		codes[i] = l.Code
	}
	sort.Strings(codes)
	if codes[0] != "en-GB" || codes[1] != "pt-BR" {
		t.Errorf("AvailableLocales() codes = %v, want [en-GB, pt-BR]", codes)
	}
}

func TestMissingKeys_DetectsMissing(t *testing.T) {
	fs := buildTestFS(map[string]string{
		"en-GB.json": validEnGB,
		"pt-BR.json": partialPtBR,
	})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	missing, err := lm.MissingKeys("pt-BR")
	if err != nil {
		t.Fatalf("MissingKeys() error = %v", err)
	}

	if len(missing) != 1 || missing[0] != "tray.quit" {
		t.Errorf("MissingKeys() = %v, want [tray.quit]", missing)
	}
}

func TestMissingKeys_NoneForComplete(t *testing.T) {
	fs := buildTestFS(map[string]string{
		"en-GB.json": validEnGB,
		"pt-BR.json": validPtBR,
	})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	missing, err := lm.MissingKeys("pt-BR")
	if err != nil {
		t.Fatalf("MissingKeys() error = %v", err)
	}

	if len(missing) != 0 {
		t.Errorf("MissingKeys() = %v, want empty", missing)
	}
}

func TestMissingKeys_ErrorForUnknownLocale(t *testing.T) {
	fs := buildTestFS(map[string]string{"en-GB.json": validEnGB})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	_, err = lm.MissingKeys("xx-XX")
	if err == nil {
		t.Fatal("MissingKeys(xx-XX) expected error")
	}
}

func TestValidateLocaleFile_ValidFile(t *testing.T) {
	data := []byte(validEnGB)
	result, err := ValidateLocaleFile(data)
	if err != nil {
		t.Fatalf("ValidateLocaleFile() error = %v", err)
	}
	if result["_locale.code"] != "en-GB" {
		t.Errorf("code = %q, want %q", result["_locale.code"], "en-GB")
	}
}

func TestValidateLocaleFile_InvalidJSON(t *testing.T) {
	data := []byte(`{not valid json}`)
	_, err := ValidateLocaleFile(data)
	if err == nil {
		t.Fatal("ValidateLocaleFile() expected error for invalid JSON")
	}
}

func TestValidateLocaleFile_DuplicateKeys(t *testing.T) {
	data := []byte(`{
  "_locale.code": "en-GB",
  "_locale.displayName": "English (UK)",
  "key1": "value1",
  "key1": "value2"
}`)
	_, err := ValidateLocaleFile(data)
	if err == nil {
		t.Fatal("ValidateLocaleFile() expected error for duplicate keys")
	}
}

func TestValidateLocaleFile_MissingMetaCode(t *testing.T) {
	data := []byte(`{
  "_locale.displayName": "English (UK)",
  "key1": "value1"
}`)
	_, err := ValidateLocaleFile(data)
	if err == nil {
		t.Fatal("ValidateLocaleFile() expected error for missing _locale.code")
	}
}

func TestValidateLocaleFile_MissingMetaDisplayName(t *testing.T) {
	data := []byte(`{
  "_locale.code": "en-GB",
  "key1": "value1"
}`)
	_, err := ValidateLocaleFile(data)
	if err == nil {
		t.Fatal("ValidateLocaleFile() expected error for missing _locale.displayName")
	}
}

func TestValidateLocaleFile_NonStringValue(t *testing.T) {
	data := []byte(`{
  "_locale.code": "en-GB",
  "_locale.displayName": "English (UK)",
  "key1": 123
}`)
	_, err := ValidateLocaleFile(data)
	if err == nil {
		t.Fatal("ValidateLocaleFile() expected error for non-string value")
	}
}

func TestValidateLocaleFile_EmptyObject(t *testing.T) {
	data := []byte(`{}`)
	_, err := ValidateLocaleFile(data)
	if err == nil {
		t.Fatal("ValidateLocaleFile() expected error for empty object (missing metadata)")
	}
}

func TestSetLocale_CorruptFileFallsBack(t *testing.T) {
	fs := buildTestFS(map[string]string{
		"en-GB.json": validEnGB,
		"fr-FR.json": `{not valid json at all`,
	})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	err = lm.SetLocale("fr-FR")
	if err == nil {
		t.Fatal("SetLocale(fr-FR) expected error for corrupt file")
	}
	if lm.ActiveLocale() != "en-GB" {
		t.Errorf("ActiveLocale() = %q, want %q after corrupt file fallback", lm.ActiveLocale(), "en-GB")
	}
	// Should still serve en-GB translations
	if got := lm.T("settings.title"); got != "WeatherWidget Settings" {
		t.Errorf("T(settings.title) = %q, want en-GB value after fallback", got)
	}
}

// TestEnGBCompleteness verifies the real embedded en-GB.json contains all expected keys.
func TestEnGBCompleteness(t *testing.T) {
	lm, err := NewLocaleManager(LocaleFS)
	if err != nil {
		t.Fatalf("NewLocaleManager(LocaleFS) error = %v", err)
	}

	expectedKeys := []string{
		"_locale.code",
		"_locale.displayName",
		"settings.title",
		"settings.tab.appearance",
		"settings.tab.provider",
		"settings.tab.locations",
		"settings.tab.about",
		"settings.position.title",
		"settings.position.subtitle",
		"settings.position.topLeft",
		"settings.position.topRight",
		"settings.position.bottomLeft",
		"settings.position.bottomRight",
		"settings.transparency.title",
		"settings.transparency.subtitle",
		"settings.icons.title",
		"settings.icons.subtitle",
		"settings.icons.new",
		"settings.icons.original",
		"settings.interval.title",
		"settings.interval.subtitle",
		"settings.interval.format",
		"settings.startup.title",
		"settings.startup.subtitle",
		"settings.startup.autostart",
		"settings.provider.title",
		"settings.provider.subtitle",
		"settings.provider.label",
		"settings.provider.apiKeyLabel",
		"settings.provider.apiKeyPlaceholder",
		"settings.provider.getFreeApi",
		"settings.provider.getProApi",
		"settings.provider.note",
		"settings.locations.savedTitle",
		"settings.locations.savedSubtitle",
		"settings.locations.proNote",
		"dialog.pro.title",
		"dialog.pro.subtitle",
		"dialog.pro.feature1",
		"dialog.pro.feature1_desc",
		"dialog.pro.feature2",
		"dialog.pro.feature2_desc",
		"dialog.pro.feature3",
		"dialog.pro.feature3_desc",
		"dialog.pro.cta",
		"dialog.pro.dismiss",
		"settings.locations.addTitle",
		"settings.locations.namePlaceholder",
		"settings.locations.regionPlaceholder",
		"settings.locations.latPlaceholder",
		"settings.locations.lonPlaceholder",
		"settings.locations.tzPlaceholder",
		"settings.locations.addBtn",
		"settings.locations.searchBtn",
		"settings.locations.searching",
		"settings.locations.removeBtn",
		"settings.locations.nameLabel",
		"settings.locations.regionLabel",
		"settings.locations.latLabel",
		"settings.locations.lonLabel",
		"settings.locations.tzLabel",
		"settings.monitor.label",
		"settings.monitor.format",
		"settings.save",
		"settings.cancel",
		"settings.dialog.saved",
		"settings.dialog.savedMsg",
		"settings.about.version",
		"settings.about.description",
		"settings.about.websiteLabel",
		"settings.about.manualLabel",
		"settings.about.previewLabel",
		"settings.about.appName",
		"settings.language.title",
		"settings.language.subtitle",
		"settings.temperature.title",
		"settings.temperature.subtitle",
		"settings.windspeed.title",
		"settings.windspeed.subtitle",
		"settings.pollution.title",
		"settings.pollution.subtitle",
		"settings.pollution.proNote",
		"settings.pollution.co",
		"settings.pollution.no",
		"settings.pollution.no2",
		"settings.pollution.o3",
		"settings.pollution.so2",
		"settings.pollution.nh3",
		"settings.pollution.pm2_5",
		"settings.pollution.pm10",
		"settings.pollution.aqi",
		"tray.showWidget",
		"tray.hideWidget",
		"tray.settings",
		"tray.quit",
		"panel.placeholder.city",
		"panel.placeholder.temp",
		"panel.placeholder.desc",
		"panel.placeholder.time",
		"panel.placeholder.date",
		"panel.staleWarning",
		"weather.gust",
		"weather.dew",
		"weather.tempSuffix",
		"weather.tempFormat",
		"weather.dateFormat",
		"weather.timeFormat",
		"weather.condition.clear_sky",
		"weather.condition.overcast_clouds",
		"weather.condition.broken_clouds",
		"weather.condition.few_clouds",
		"weather.condition.scattered_clouds",
		"weather.condition.light_rain",
		"weather.condition.moderate_rain",
		"weather.condition.heavy_intensity_rain",
		"weather.condition.very_heavy_rain",
		"validation.cities.count",
		"validation.cities.count.free",
		"validation.refreshInterval.min.owm",
		"validation.refreshInterval.min.eww",
		"validation.refreshInterval.max",
		"validation.refreshInterval.range",
		"validation.cornerPosition.invalid",
		"validation.apiConfig.required",
		"validation.apiConfig.apiKey.empty",
		"validation.apiConfig.provider.invalid",
		"validation.dbConfig.required",
		"validation.dbConfig.host.empty",
		"validation.dbConfig.port.range",
		"validation.dbConfig.dbName.empty",
		"validation.dbConfig.username.empty",
		"validation.city.name.empty",
		"validation.city.lat.range",
		"validation.city.lon.range",
		"validation.locale.invalid",
		"error.cities.max",
		"error.cities.maxFree",
		"error.cities.removeLast",
		"error.cities.indexOutOfBounds",
		"error.settings.cityNameRequired",
		"error.settings.cityNameRequiredSearch",
		"error.settings.apiKeyRequired",
		"error.settings.regionRequiredEww",
		"error.settings.invalidLat",
		"error.settings.invalidLon",
		"error.settings.searchFailed",
		"error.settings.searchApiError",
		"error.settings.readError",
		"error.settings.parseFailed",
		"error.settings.noCityFound",
		"error.settings.connectionFailed",
		"error.settings.saveFailed",
		"error.settings.autoStartFailed",
	}

	for _, key := range expectedKeys {
		val := lm.T(key)
		if val == key {
			t.Errorf("en-GB missing key %q (T returned key itself)", key)
		}
		if val == "" {
			t.Errorf("en-GB key %q has empty value", key)
		}
	}
}

// TestPtBRCompleteness verifies the real embedded pt-BR.json contains all en-GB keys.
func TestPtBRCompleteness(t *testing.T) {
	lm, err := NewLocaleManager(LocaleFS)
	if err != nil {
		t.Fatalf("NewLocaleManager(LocaleFS) error = %v", err)
	}

	missing, err := lm.MissingKeys("pt-BR")
	if err != nil {
		t.Fatalf("MissingKeys(pt-BR) error = %v", err)
	}

	if len(missing) > 0 {
		t.Errorf("pt-BR is missing %d keys from en-GB: %v", len(missing), missing)
	}
}

// TestLocaleSwitch_InPlace verifies SetLocale swaps translations without needing a new manager.
func TestLocaleSwitch_InPlace(t *testing.T) {
	fs := buildTestFS(map[string]string{
		"en-GB.json": validEnGB,
		"pt-BR.json": validPtBR,
	})
	lm, err := NewLocaleManager(fs)
	if err != nil {
		t.Fatalf("NewLocaleManager() error = %v", err)
	}

	// Start with en-GB
	if got := lm.T("settings.title"); got != "WeatherWidget Settings" {
		t.Errorf("initial T(settings.title) = %q, want en-GB", got)
	}

	// Switch to pt-BR
	_ = lm.SetLocale("pt-BR")
	if got := lm.T("settings.title"); got != "Configurações do WeatherWidget" {
		t.Errorf("after SetLocale(pt-BR) T(settings.title) = %q, want pt-BR", got)
	}

	// Switch back to en-GB
	_ = lm.SetLocale("en-GB")
	if got := lm.T("settings.title"); got != "WeatherWidget Settings" {
		t.Errorf("after SetLocale(en-GB) T(settings.title) = %q, want en-GB", got)
	}
}
