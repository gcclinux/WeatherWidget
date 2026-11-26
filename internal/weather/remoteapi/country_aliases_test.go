package remoteapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeSearchLocation(t *testing.T) {
	tests := []struct {
		name       string
		inName     string
		inRegion   string
		wantName   string
		wantRegion string
	}{
		// Direct aliases in region
		{name: "UK uppercase", inName: "London", inRegion: "UK", wantName: "London", wantRegion: "GB"},
		{name: "uk lowercase", inName: "London", inRegion: "uk", wantName: "London", wantRegion: "GB"},
		{name: "England mixed case with spaces", inName: " Manchester ", inRegion: "  England ", wantName: "Manchester", wantRegion: "GB"},
		{name: "Scotland", inName: "Edinburgh", inRegion: "Scotland", wantName: "Edinburgh", wantRegion: "GB"},
		{name: "Holland", inName: "Amsterdam", inRegion: "Holland", wantName: "Amsterdam", wantRegion: "NL"},
		{name: "Nederland", inName: "Rotterdam", inRegion: "nederland", wantName: "Rotterdam", wantRegion: "NL"},
		{name: "Netherland", inName: "Utrecht", inRegion: "Netherland", wantName: "Utrecht", wantRegion: "NL"},
		{name: "Netherlands", inName: "Eindhoven", inRegion: "Netherlands", wantName: "Eindhoven", wantRegion: "NL"},
		{name: "India", inName: "Mumbai", inRegion: "India", wantName: "Mumbai", wantRegion: "IN"},
		{name: "india lowercase", inName: "Delhi", inRegion: " india ", wantName: "Delhi", wantRegion: "IN"},

		// Multi-part Region, Country values
		{name: "Surrey, England", inName: "Guildford", inRegion: "Surrey, England", wantName: "Guildford", wantRegion: "Surrey, GB"},
		{name: "Noord-Holland, Nederland", inName: "Haarlem", inRegion: "Noord-Holland, Nederland", wantName: "Haarlem", wantRegion: "Noord-Holland, NL"},
		{name: "Maharashtra, India", inName: "Pune", inRegion: "Maharashtra, India", wantName: "Pune", wantRegion: "Maharashtra, IN"},
		{name: "Leading comma in region", inName: "London", inRegion: ", UK", wantName: "London", wantRegion: "GB"},

		// Trailing alias in city name when region is empty
		{name: "London, UK in name", inName: "London, UK", inRegion: "", wantName: "London", wantRegion: "GB"},
		{name: "Amsterdam, Holland in name", inName: "Amsterdam, Holland", inRegion: "", wantName: "Amsterdam", wantRegion: "NL"},
		{name: "Mumbai, India in name", inName: "Mumbai, India", inRegion: "", wantName: "Mumbai", wantRegion: "IN"},

		// Unmatched values remain unchanged (beyond trimming)
		{name: "ISO GB unchanged", inName: "London", inRegion: "GB", wantName: "London", wantRegion: "GB"},
		{name: "CA, US unchanged", inName: "San Jose", inRegion: "CA, US", wantName: "San Jose", wantRegion: "CA, US"},
		{name: "Empty inputs", inName: " ", inRegion: " ", wantName: "", wantRegion: ""},
		{name: "London, GB in name with empty region unchanged", inName: "London, GB", inRegion: "", wantName: "London, GB", wantRegion: ""},
		{name: "Alias only in name with empty region unchanged", inName: ", UK", inRegion: "", wantName: ", UK", wantRegion: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotRegion := normalizeSearchLocation(tc.inName, tc.inRegion)
			if gotName != tc.wantName || gotRegion != tc.wantRegion {
				t.Errorf("normalizeSearchLocation(%q, %q) = (%q, %q), want (%q, %q)",
					tc.inName, tc.inRegion, gotName, gotRegion, tc.wantName, tc.wantRegion)
			}
		})
	}
}

func TestSearchCity_CountryAliases_OWM(t *testing.T) {
	tests := []struct {
		name      string
		inName    string
		inRegion  string
		wantQuery string
		respBody  string
		wantCity  string
		wantReg   string
	}{
		{
			name:      "UK to GB",
			inName:    "London",
			inRegion:  "UK",
			wantQuery: "London,GB",
			respBody:  `[{"name":"London","country":"GB","lat":51.5074,"lon":-0.1278}]`,
			wantCity:  "London",
			wantReg:   "GB",
		},
		{
			name:      "Scotland to GB",
			inName:    "Edinburgh",
			inRegion:  "Scotland",
			wantQuery: "Edinburgh,GB",
			respBody:  `[{"name":"Edinburgh","country":"GB","lat":55.9533,"lon":-3.1883}]`,
			wantCity:  "Edinburgh",
			wantReg:   "GB",
		},
		{
			name:      "Holland to NL",
			inName:    "Amsterdam",
			inRegion:  "Holland",
			wantQuery: "Amsterdam,NL",
			respBody:  `[{"name":"Amsterdam","country":"NL","lat":52.3676,"lon":4.9041}]`,
			wantCity:  "Amsterdam",
			wantReg:   "NL",
		},
		{
			name:      "India to IN",
			inName:    "Mumbai",
			inRegion:  "India",
			wantQuery: "Mumbai,IN",
			respBody:  `[{"name":"Mumbai","country":"IN","lat":19.0760,"lon":72.8777}]`,
			wantCity:  "Mumbai",
			wantReg:   "IN",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotQ string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQ = r.URL.Query().Get("q")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.respBody))
			}))
			defer srv.Close()

			adapter := NewRemoteAPIAdapter("openweathermap", "test-key")
			adapter.BaseURL = srv.URL

			city, err := adapter.SearchCity(context.Background(), tc.inName, tc.inRegion)
			if err != nil {
				t.Fatalf("SearchCity failed: %v", err)
			}
			if gotQ != tc.wantQuery {
				t.Errorf("query q = %q, want %q", gotQ, tc.wantQuery)
			}
			if city.Name != tc.wantCity || city.Region != tc.wantReg {
				t.Errorf("got city (%q, %q), want (%q, %q)", city.Name, city.Region, tc.wantCity, tc.wantReg)
			}
		})
	}
}

func TestSearchCity_CountryAliases_EWW(t *testing.T) {
	tests := []struct {
		name     string
		inName   string
		inRegion string
		wantPath string
		respBody string
		wantCity string
		wantReg  string
	}{
		{
			name:     "England to GB",
			inName:   "London",
			inRegion: "England",
			wantPath: "/api/v1/weather-news/key=test-key/London,GB",
			respBody: `{"city_name":"London","country_code":"GB","lat":51.5074,"lon":-0.1278}`,
			wantCity: "London",
			wantReg:  "GB",
		},
		{
			name:     "Nederland to NL",
			inName:   "Amsterdam",
			inRegion: "Nederland",
			wantPath: "/api/v1/weather-news/key=test-key/Amsterdam,NL",
			respBody: `{"city_name":"Amsterdam","country_code":"NL","lat":52.3676,"lon":4.9041}`,
			wantCity: "Amsterdam",
			wantReg:  "NL",
		},
		{
			name:     "India to IN",
			inName:   "Mumbai",
			inRegion: "India",
			wantPath: "/api/v1/weather-news/key=test-key/Mumbai,IN",
			respBody: `{"city_name":"Mumbai","country_code":"IN","lat":19.0760,"lon":72.8777}`,
			wantCity: "Mumbai",
			wantReg:  "IN",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.respBody))
			}))
			defer srv.Close()

			adapter := NewRemoteAPIAdapter("easyweatherwidget", "test-key")
			adapter.BaseURL = srv.URL

			city, err := adapter.SearchCity(context.Background(), tc.inName, tc.inRegion)
			if err != nil {
				t.Fatalf("SearchCity failed: %v", err)
			}
			if gotPath != tc.wantPath {
				t.Errorf("request path = %q, want %q", gotPath, tc.wantPath)
			}
			if city.Name != tc.wantCity || city.Region != tc.wantReg {
				t.Errorf("got city (%q, %q), want (%q, %q)", city.Name, city.Region, tc.wantCity, tc.wantReg)
			}
		})
	}
}
