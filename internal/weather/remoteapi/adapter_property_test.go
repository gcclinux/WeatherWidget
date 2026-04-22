package remoteapi

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"

	"pgregory.net/rapid"
)

// **Feature: easy-wether-widget-provider, Property 1: EWW Response Parsing Consistency**
// **Validates: Requirements 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 7.1, 7.3**

func TestProperty1_EWWResponseParsingConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate an arbitrary ewwResponse
		eww := ewwResponse{
			Temp:         rapid.Float64Range(-100, 100).Draw(t, "Temp"),
			Neighborhood: rapid.String().Draw(t, "Neighborhood"),
			Country:      rapid.String().Draw(t, "Country"),
			FreeText:     rapid.String().Draw(t, "FreeText"),
			ObsTimeLocal: rapid.String().Draw(t, "ObsTimeLocal"),
		}

		// Serve the response via httptest.Server
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(eww)
		}))
		defer srv.Close()

		adapter := NewRemoteAPIAdapter("easyweatherwidget", "test-key")
		adapter.BaseURL = srv.URL

		city := config.CityConfig{
			Name:     "TestCity",
			Region:   "TC",
			Timezone: "UTC",
		}

		data, err := adapter.FetchWeather(context.Background(), city)
		if err != nil {
			t.Fatalf("FetchWeather returned unexpected error: %v", err)
		}

		// Assert: Temperature == int(math.Round(Temp))
		expectedTemp := int(math.Round(eww.Temp))
		if data.Temperature != expectedTemp {
			t.Fatalf("Temperature = %d, want %d (from Temp = %f)", data.Temperature, expectedTemp, eww.Temp)
		}

		// Assert: CityName == city.Name
		if data.CityName != city.Name {
			t.Fatalf("CityName = %q, want %q", data.CityName, city.Name)
		}

		// Assert: Region == city.Region
		if data.Region != city.Region {
			t.Fatalf("Region = %q, want %q", data.Region, city.Region)
		}

		// Assert: Description == FreeText
		if data.Description != eww.FreeText {
			t.Fatalf("Description = %q, want %q", data.Description, eww.FreeText)
		}

		// Assert: IconCode ∈ AllIconCodes
		found := false
		for _, code := range weather.AllIconCodes {
			if data.IconCode == code {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("IconCode = %q is not in AllIconCodes %v", data.IconCode, weather.AllIconCodes)
		}
	})
}

// **Feature: easy-wether-widget-provider, Property 2: FreeText-to-Icon Totality**
// **Validates: Requirements 2.6, 7.2**

func TestProperty2_FreeTextToIconTotality(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate an arbitrary string
		freeText := rapid.String().Draw(t, "freeText")

		// Call mapEWWFreeTextToIcon
		icon := mapEWWFreeTextToIcon(freeText)

		// Assert result is a member of weather.AllIconCodes
		found := false
		for _, code := range weather.AllIconCodes {
			if icon == code {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("mapEWWFreeTextToIcon(%q) = %q, which is not in AllIconCodes %v", freeText, icon, weather.AllIconCodes)
		}
	})
}

// **Feature: easy-wether-widget-provider, Property 3: FreeText Keyword Mapping Correctness**
// **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7**

func TestProperty3_FreeTextKeywordMappingCorrectness(t *testing.T) {
	// allKeywords lists every keyword recognised by mapEWWFreeTextToIcon.
	allKeywords := []string{
		"storm", "thunder",
		"snow",
		"rain", "drizzle",
		"fog", "mist", "haze",
		"cloud",
		"clear",
	}

	// keywordTier describes one priority tier for the FreeText icon mapping.
	type keywordTier struct {
		name           string
		keywords       []string
		higherKeywords []string // keywords from all higher-priority tiers
		expectedIcon   string
	}

	tiers := []keywordTier{
		{
			name:           "storm/thunder",
			keywords:       []string{"storm", "thunder"},
			higherKeywords: nil, // highest priority — nothing above
			expectedIcon:   weather.IconStorm,
		},
		{
			name:           "snow",
			keywords:       []string{"snow"},
			higherKeywords: []string{"storm", "thunder"},
			expectedIcon:   weather.IconSnow,
		},
		{
			name:           "rain/drizzle",
			keywords:       []string{"rain", "drizzle"},
			higherKeywords: []string{"storm", "thunder", "snow"},
			expectedIcon:   weather.IconRain,
		},
		{
			name:           "fog/mist/haze",
			keywords:       []string{"fog", "mist", "haze"},
			higherKeywords: []string{"storm", "thunder", "snow", "rain", "drizzle"},
			expectedIcon:   weather.IconFog,
		},
		{
			name:           "cloud",
			keywords:       []string{"cloud"},
			higherKeywords: []string{"storm", "thunder", "snow", "rain", "drizzle", "fog", "mist", "haze"},
			expectedIcon:   weather.IconCloudy,
		},
		{
			name:           "clear",
			keywords:       []string{"clear"},
			higherKeywords: []string{"storm", "thunder", "snow", "rain", "drizzle", "fog", "mist", "haze", "cloud"},
			expectedIcon:   weather.IconClear,
		},
		{
			name:           "default (no keywords)",
			keywords:       nil, // no keyword inserted
			higherKeywords: allKeywords,
			expectedIcon:   weather.IconPartlyCloudy,
		},
	}

	// safeStringGen returns a rapid generator that produces strings not containing
	// any of the forbidden keywords (case-insensitive).
	safeStringGen := func(forbidden []string) *rapid.Generator[string] {
		return rapid.Custom[string](func(t *rapid.T) string {
			// Use alphanumeric characters that cannot accidentally form keywords.
			// We pick from digits and a safe subset of letters that don't start any keyword.
			const safeChars = "0123456789xqjvwbXQJVWB _-"
			length := rapid.IntRange(0, 20).Draw(t, "safeLen")
			buf := make([]byte, length)
			for i := range buf {
				buf[i] = safeChars[rapid.IntRange(0, len(safeChars)-1).Draw(t, "safeChar")]
			}
			return string(buf)
		})
	}

	for _, tier := range tiers {
		tier := tier // capture
		t.Run(tier.name, func(t *testing.T) {
			rapid.Check(t, func(rt *rapid.T) {
				// Generate a safe prefix and suffix that contain no keywords at all.
				prefix := safeStringGen(allKeywords).Draw(rt, "prefix")
				suffix := safeStringGen(allKeywords).Draw(rt, "suffix")

				var input string
				if tier.keywords != nil {
					// Pick one keyword from this tier.
					keyword := rapid.SampledFrom(tier.keywords).Draw(rt, "keyword")
					// Optionally randomise the case of the keyword.
					cased := randomiseCase(rt, keyword)
					input = prefix + " " + cased + " " + suffix
				} else {
					// Default tier: no keyword at all.
					input = prefix + suffix
				}

				// Safety check: the generated string must not contain any
				// higher-priority keywords (case-insensitive).
				lower := strings.ToLower(input)
				for _, hk := range tier.higherKeywords {
					if strings.Contains(lower, hk) {
						// Extremely unlikely with our safe alphabet, but skip
						// this iteration rather than produce a false failure.
						rt.Skip("generated string accidentally contains higher-priority keyword")
					}
				}

				got := mapEWWFreeTextToIcon(input)
				if got != tier.expectedIcon {
					rt.Fatalf("mapEWWFreeTextToIcon(%q) = %q, want %q", input, got, tier.expectedIcon)
				}
			})
		})
	}
}

// randomiseCase randomly upper- or lower-cases each character in s.
func randomiseCase(t *rapid.T, s string) string {
	var buf strings.Builder
	for _, ch := range s {
		if rapid.Bool().Draw(t, "upper") {
			buf.WriteString(strings.ToUpper(string(ch)))
		} else {
			buf.WriteString(strings.ToLower(string(ch)))
		}
	}
	return buf.String()
}

// **Feature: easy-wether-widget-provider, Property 4: FreeText Case-Insensitive Matching**
// **Validates: Requirements 3.8**

func TestProperty4_FreeTextCaseInsensitiveMatching(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate an arbitrary string
		s := rapid.String().Draw(t, "freeText")

		// Map the original, upper-cased, and lower-cased versions
		original := mapEWWFreeTextToIcon(s)
		upper := mapEWWFreeTextToIcon(strings.ToUpper(s))
		lower := mapEWWFreeTextToIcon(strings.ToLower(s))

		// Assert all three produce the same icon code
		if original != upper {
			t.Fatalf("mapEWWFreeTextToIcon(%q) = %q, but mapEWWFreeTextToIcon(ToUpper(%q)) = %q", s, original, s, upper)
		}
		if original != lower {
			t.Fatalf("mapEWWFreeTextToIcon(%q) = %q, but mapEWWFreeTextToIcon(ToLower(%q)) = %q", s, original, s, lower)
		}
	})
}
