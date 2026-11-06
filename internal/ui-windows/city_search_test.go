//go:build windows

package uiwindows

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"weatherwidget/internal/i18n"
)

type searchRoundTripper func(*http.Request) (*http.Response, error)

func (fn searchRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCityAutofillRequiresCountryAndUsesItInSearch(t *testing.T) {
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatal(err)
	}
	old := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = old })
	calls := 0
	http.DefaultClient = &http.Client{Transport: searchRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		if got := req.URL.Query().Get("q"); got != "London,GB" {
			t.Errorf("search query = %q", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`[{"name":"London","country":"GB","lat":51.5074,"lon":-0.1278}]`)), Header: make(http.Header)}, nil
	})}
	if _, err := searchSettingsCity("London", "", "openweathermap", "key", lm); err == nil || calls != 0 {
		t.Fatal("search without country must be rejected before making a request")
	}
	city, err := searchSettingsCity("London", "GB", "openweathermap", "key", lm)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || city.Timezone != "Europe/London" || city.Latitude != 51.5074 || city.Longitude != -0.1278 {
		t.Fatalf("incorrect auto-fill result: %+v", city)
	}
}
