package i18n

import (
	"encoding/json"
	"sort"
	"testing"
	"testing/fstest"

	"pgregory.net/rapid"
)

// **Feature: i18n-localization, Property 1: Lookup returns translated string for valid keys**
// **Validates: Requirements 1.4**

func TestProperty1_LookupReturnsTranslatedStringForValidKeys(t *testing.T) {
	// Load the real en-GB locale to get all valid keys.
	lm, err := NewLocaleManager(LocaleFS)
	if err != nil {
		t.Fatalf("NewLocaleManager(LocaleFS) error = %v", err)
	}

	enGBKeys := make([]string, 0, len(lm.fallback))
	for k := range lm.fallback {
		enGBKeys = append(enGBKeys, k)
	}
	sort.Strings(enGBKeys)

	// Collect available locale codes.
	locales := lm.AvailableLocales()
	localeCodes := make([]string, len(locales))
	for i, l := range locales {
		localeCodes[i] = l.Code
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Pick a random valid key from en-GB.
		keyIdx := rapid.IntRange(0, len(enGBKeys)-1).Draw(rt, "keyIdx")
		key := enGBKeys[keyIdx]

		// Pick a random available locale.
		localeIdx := rapid.IntRange(0, len(localeCodes)-1).Draw(rt, "localeIdx")
		locale := localeCodes[localeIdx]

		// Create a fresh manager and set the locale.
		mgr, err := NewLocaleManager(LocaleFS)
		if err != nil {
			rt.Fatalf("NewLocaleManager error = %v", err)
		}
		if err := mgr.SetLocale(locale); err != nil {
			rt.Fatalf("SetLocale(%q) error = %v", locale, err)
		}

		result := mgr.T(key)

		if result == "" {
			rt.Fatalf("T(%q) with locale %q returned empty string", key, locale)
		}
	})
}

// **Feature: i18n-localization, Property 2: Missing key fallback to en-GB**
// **Validates: Requirements 1.5**

func TestProperty2_MissingKeyFallbackToEnGB(t *testing.T) {
	// Load en-GB to get the reference keys and values.
	refLM, err := NewLocaleManager(LocaleFS)
	if err != nil {
		t.Fatalf("NewLocaleManager(LocaleFS) error = %v", err)
	}

	enGBKeys := make([]string, 0, len(refLM.fallback))
	for k := range refLM.fallback {
		enGBKeys = append(enGBKeys, k)
	}
	sort.Strings(enGBKeys)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random subset of en-GB keys to KEEP in the test locale.
		// We need at least the two metadata keys.
		subsetSize := rapid.IntRange(2, len(enGBKeys)-1).Draw(rt, "subsetSize")

		// Always include the metadata keys.
		kept := map[string]bool{
			metaCodeKey:    true,
			metaDisplayKey: true,
		}

		// Randomly pick additional keys to keep (up to subsetSize).
		nonMetaKeys := make([]string, 0, len(enGBKeys))
		for _, k := range enGBKeys {
			if k != metaCodeKey && k != metaDisplayKey {
				nonMetaKeys = append(nonMetaKeys, k)
			}
		}

		// Shuffle by picking random indices.
		keepCount := subsetSize - 2 // subtract the 2 metadata keys
		if keepCount > len(nonMetaKeys) {
			keepCount = len(nonMetaKeys)
		}
		// Use rapid to select which keys to keep.
		for i := 0; i < keepCount; i++ {
			idx := rapid.IntRange(0, len(nonMetaKeys)-1).Draw(rt, "keepIdx")
			kept[nonMetaKeys[idx]] = true
		}

		// Build a test locale file with only the kept keys.
		testLocale := make(map[string]string)
		for k, v := range refLM.fallback {
			if kept[k] {
				testLocale[k] = v
			}
		}
		// Override metadata for the test locale.
		testLocale[metaCodeKey] = "xx-XX"
		testLocale[metaDisplayKey] = "Test Locale"

		testJSON, err := json.Marshal(testLocale)
		if err != nil {
			rt.Fatalf("json.Marshal error = %v", err)
		}

		// Build en-GB JSON from the reference.
		enGBJSON, err := json.Marshal(refLM.fallback)
		if err != nil {
			rt.Fatalf("json.Marshal en-GB error = %v", err)
		}

		// Create in-memory FS.
		fs := fstest.MapFS{
			"locales/en-GB.json": &fstest.MapFile{Data: enGBJSON},
			"locales/xx-XX.json": &fstest.MapFile{Data: testJSON},
		}

		mgr, err := NewLocaleManager(fs)
		if err != nil {
			rt.Fatalf("NewLocaleManager error = %v", err)
		}
		if err := mgr.SetLocale("xx-XX"); err != nil {
			rt.Fatalf("SetLocale(xx-XX) error = %v", err)
		}

		// For every en-GB key missing from the test locale, T should return the en-GB value.
		for _, key := range enGBKeys {
			if kept[key] {
				continue // skip keys that exist in the test locale
			}
			result := mgr.T(key)
			expected := refLM.fallback[key]
			if result != expected {
				rt.Fatalf("T(%q) with missing key: got %q, want en-GB value %q", key, result, expected)
			}
		}
	})
}

// **Feature: i18n-localization, Property 5: Locale file JSON round-trip**
// **Validates: Requirements 8.1**

func TestProperty5_LocaleFileJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random locale map with metadata keys and additional keys.
		numKeys := rapid.IntRange(0, 20).Draw(rt, "numKeys")

		original := make(map[string]string)
		original[metaCodeKey] = rapid.StringMatching(`[a-z]{2}-[A-Z]{2}`).Draw(rt, "code")
		original[metaDisplayKey] = rapid.StringMatching(`[A-Za-z ]{3,30}`).Draw(rt, "displayName")

		for i := 0; i < numKeys; i++ {
			// Generate keys that look like dotted message keys.
			key := rapid.StringMatching(`[a-z]{2,8}\.[a-z]{2,8}`).Draw(rt, "key")
			// Avoid overwriting metadata keys.
			if key == metaCodeKey || key == metaDisplayKey {
				continue
			}
			value := rapid.StringMatching(`[A-Za-z0-9 .,!?%]{1,50}`).Draw(rt, "value")
			original[key] = value
		}

		// Serialise to JSON.
		data, err := json.Marshal(original)
		if err != nil {
			rt.Fatalf("json.Marshal error = %v", err)
		}

		// Deserialise back.
		var roundTripped map[string]string
		if err := json.Unmarshal(data, &roundTripped); err != nil {
			rt.Fatalf("json.Unmarshal error = %v", err)
		}

		// Verify identical keys and values.
		if len(original) != len(roundTripped) {
			rt.Fatalf("key count mismatch: original=%d, roundTripped=%d", len(original), len(roundTripped))
		}
		for k, v := range original {
			rv, ok := roundTripped[k]
			if !ok {
				rt.Fatalf("key %q missing after round-trip", k)
			}
			if v != rv {
				rt.Fatalf("value mismatch for key %q: original=%q, roundTripped=%q", k, v, rv)
			}
		}
	})
}

// **Feature: i18n-localization, Property 6: Missing keys detection returns correct set difference**
// **Validates: Requirements 9.1**

func TestProperty6_MissingKeysReturnsCorrectSetDifference(t *testing.T) {
	// Load en-GB to get the reference keys.
	refLM, err := NewLocaleManager(LocaleFS)
	if err != nil {
		t.Fatalf("NewLocaleManager(LocaleFS) error = %v", err)
	}

	enGBKeys := make([]string, 0, len(refLM.fallback))
	for k := range refLM.fallback {
		enGBKeys = append(enGBKeys, k)
	}
	sort.Strings(enGBKeys)

	rapid.Check(t, func(rt *rapid.T) {
		// Decide which en-GB keys to include in the test locale.
		// Always include metadata keys.
		included := map[string]bool{
			metaCodeKey:    true,
			metaDisplayKey: true,
		}

		// For each non-metadata key, randomly decide whether to include it.
		for _, k := range enGBKeys {
			if k == metaCodeKey || k == metaDisplayKey {
				continue
			}
			if rapid.Bool().Draw(rt, "include_"+k) {
				included[k] = true
			}
		}

		// Build the test locale file.
		testLocale := make(map[string]string)
		for k := range included {
			testLocale[k] = refLM.fallback[k]
		}
		testLocale[metaCodeKey] = "xx-XX"
		testLocale[metaDisplayKey] = "Test Locale"

		testJSON, err := json.Marshal(testLocale)
		if err != nil {
			rt.Fatalf("json.Marshal error = %v", err)
		}

		enGBJSON, err := json.Marshal(refLM.fallback)
		if err != nil {
			rt.Fatalf("json.Marshal en-GB error = %v", err)
		}

		fs := fstest.MapFS{
			"locales/en-GB.json": &fstest.MapFile{Data: enGBJSON},
			"locales/xx-XX.json": &fstest.MapFile{Data: testJSON},
		}

		mgr, err := NewLocaleManager(fs)
		if err != nil {
			rt.Fatalf("NewLocaleManager error = %v", err)
		}

		missing, err := mgr.MissingKeys("xx-XX")
		if err != nil {
			rt.Fatalf("MissingKeys error = %v", err)
		}

		// Compute expected missing keys: keys in en-GB but not in the test locale.
		var expectedMissing []string
		for _, k := range enGBKeys {
			if !included[k] {
				expectedMissing = append(expectedMissing, k)
			}
		}
		sort.Strings(expectedMissing)

		// Compare.
		if len(missing) != len(expectedMissing) {
			rt.Fatalf("missing key count: got %d, want %d\ngot:  %v\nwant: %v",
				len(missing), len(expectedMissing), missing, expectedMissing)
		}
		for i := range missing {
			if missing[i] != expectedMissing[i] {
				rt.Fatalf("missing key mismatch at index %d: got %q, want %q",
					i, missing[i], expectedMissing[i])
			}
		}
	})
}
