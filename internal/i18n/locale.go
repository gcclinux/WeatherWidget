package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

const (
	defaultLocale  = "en-GB"
	localesDir     = "locales"
	metaCodeKey    = "_locale.code"
	metaDisplayKey = "_locale.displayName"
)

// LocaleInfo describes an available locale.
type LocaleInfo struct {
	Code        string // e.g. "en-GB", "pt-BR"
	DisplayName string // e.g. "English (UK)", "Português (Brasil)"
}

// LocaleManager loads and serves translated strings.
type LocaleManager struct {
	fsys         fs.FS             // embedded locale files
	defaultLang  string            // "en-GB"
	activeLang   string            // current locale
	translations map[string]string // active locale translations
	fallback     map[string]string // en-GB translations (always loaded)
	available    []LocaleInfo      // discovered locales
}

// NewLocaleManager creates a manager from the given filesystem.
// It discovers all locale files, loads en-GB as the fallback, and sets it as the active locale.
func NewLocaleManager(fsys fs.FS) (*LocaleManager, error) {
	lm := &LocaleManager{
		fsys:        fsys,
		defaultLang: defaultLocale,
		activeLang:  defaultLocale,
	}

	// Discover available locales
	if err := lm.discoverLocales(); err != nil {
		return nil, fmt.Errorf("discovering locales: %w", err)
	}

	// Load en-GB as fallback (required)
	fallback, err := lm.loadLocaleFile(defaultLocale)
	if err != nil {
		return nil, fmt.Errorf("loading default locale %s: %w", defaultLocale, err)
	}
	lm.fallback = fallback
	lm.translations = fallback

	return lm, nil
}

// SetLocale switches the active locale. Falls back to en-GB on error.
func (lm *LocaleManager) SetLocale(code string) error {
	if code == lm.defaultLang {
		lm.activeLang = lm.defaultLang
		lm.translations = lm.fallback
		return nil
	}

	translations, err := lm.loadLocaleFile(code)
	if err != nil {
		// Fall back to en-GB
		lm.activeLang = lm.defaultLang
		lm.translations = lm.fallback
		return fmt.Errorf("loading locale %s, falling back to %s: %w", code, lm.defaultLang, err)
	}

	lm.activeLang = code
	lm.translations = translations
	return nil
}

// T returns the translated string for the given key.
// Falls back to en-GB if the key is missing in the active locale.
// Returns the key itself if not found in either locale.
func (lm *LocaleManager) T(key string) string {
	if val, ok := lm.translations[key]; ok {
		return val
	}
	if val, ok := lm.fallback[key]; ok {
		return val
	}
	return key
}

// TWithArgs returns a translated string with fmt.Sprintf-style arguments.
func (lm *LocaleManager) TWithArgs(key string, args ...interface{}) string {
	template := lm.T(key)
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// ActiveLocale returns the current locale code.
func (lm *LocaleManager) ActiveLocale() string {
	return lm.activeLang
}

// AvailableLocales returns all discovered locale codes with display names.
func (lm *LocaleManager) AvailableLocales() []LocaleInfo {
	result := make([]LocaleInfo, len(lm.available))
	copy(result, lm.available)
	return result
}

// MissingKeys compares a locale against en-GB and returns missing keys.
func (lm *LocaleManager) MissingKeys(localeCode string) ([]string, error) {
	translations, err := lm.loadLocaleFile(localeCode)
	if err != nil {
		return nil, fmt.Errorf("loading locale %s: %w", localeCode, err)
	}

	var missing []string
	for key := range lm.fallback {
		if _, ok := translations[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing, nil
}

// ValidateLocaleFile checks if a JSON byte slice is a valid locale file.
// It uses a custom decoder to detect duplicate keys.
// Returns the parsed map on success, or an error if JSON is invalid or contains issues.
func ValidateLocaleFile(data []byte) (map[string]string, error) {
	// First check if it's valid JSON at all
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON")
	}

	// Use a decoder to detect duplicate keys
	dec := json.NewDecoder(strings.NewReader(string(data)))
	result := make(map[string]string)

	// Read opening brace
	t, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("reading JSON: %w", err)
	}
	delim, ok := t.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("expected JSON object")
	}

	// Read key-value pairs
	for dec.More() {
		// Read key
		t, err = dec.Token()
		if err != nil {
			return nil, fmt.Errorf("reading key: %w", err)
		}
		key, ok := t.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %T", t)
		}
		if key == "" {
			return nil, fmt.Errorf("empty key not allowed")
		}

		// Check for duplicate
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate key: %q", key)
		}

		// Read value
		t, err = dec.Token()
		if err != nil {
			return nil, fmt.Errorf("reading value for key %q: %w", key, err)
		}
		val, ok := t.(string)
		if !ok {
			return nil, fmt.Errorf("expected string value for key %q, got %T", key, t)
		}

		result[key] = val
	}

	// Check required metadata keys
	if _, ok := result[metaCodeKey]; !ok {
		return nil, fmt.Errorf("missing required key %q", metaCodeKey)
	}
	if _, ok := result[metaDisplayKey]; !ok {
		return nil, fmt.Errorf("missing required key %q", metaDisplayKey)
	}

	return result, nil
}

// discoverLocales reads the locales directory and populates the available list.
func (lm *LocaleManager) discoverLocales() error {
	entries, err := fs.ReadDir(lm.fsys, localesDir)
	if err != nil {
		return fmt.Errorf("reading locales directory: %w", err)
	}

	var locales []LocaleInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := fs.ReadFile(lm.fsys, localesDir+"/"+entry.Name())
		if err != nil {
			continue // skip unreadable files
		}

		parsed, err := ValidateLocaleFile(data)
		if err != nil {
			continue // skip invalid files
		}

		locales = append(locales, LocaleInfo{
			Code:        parsed[metaCodeKey],
			DisplayName: parsed[metaDisplayKey],
		})
	}

	if len(locales) == 0 {
		return fmt.Errorf("no valid locale files found")
	}

	// Sort locales by code for deterministic ordering
	sort.Slice(locales, func(i, j int) bool {
		return locales[i].Code < locales[j].Code
	})

	lm.available = locales
	return nil
}

// loadLocaleFile reads and parses a locale JSON file from the embedded FS.
func (lm *LocaleManager) loadLocaleFile(code string) (map[string]string, error) {
	filename := localesDir + "/" + code + ".json"
	data, err := fs.ReadFile(lm.fsys, filename)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filename, err)
	}

	translations, err := ValidateLocaleFile(data)
	if err != nil {
		return nil, fmt.Errorf("validating %s: %w", filename, err)
	}

	return translations, nil
}
