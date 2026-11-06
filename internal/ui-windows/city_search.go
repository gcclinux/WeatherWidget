//go:build windows

package uiwindows

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bradfitz/latlong"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
)

func searchSettingsCity(name, region, provider, key string, lm *i18n.LocaleManager) (config.CityConfig, error) {
	name, region, key = strings.TrimSpace(name), strings.TrimSpace(region), strings.TrimSpace(key)
	if name == "" {
		return config.CityConfig{}, fmt.Errorf("%s", lm.T("error.settings.cityNameRequiredSearch"))
	}
	if key == "" {
		return config.CityConfig{}, fmt.Errorf("%s", lm.T("error.settings.apiKeyRequired"))
	}
	if region == "" {
		return config.CityConfig{}, fmt.Errorf("%s", lm.T("settings.windows.autofillRequired"))
	}
	var endpoint string
	switch provider {
	case "easyweatherwidget":
		endpoint = "https://weather-gateway-ricardo.web.app/api/v1/weather/key=" + url.PathEscape(key) + "/" + url.PathEscape(name+","+region)
	case "openweathermap":
		query := url.Values{"q": {name + "," + region}, "limit": {"1"}, "appid": {key}}
		endpoint = "https://api.openweathermap.org/geo/1.0/direct?" + query.Encode()
	default:
		return config.CityConfig{}, fmt.Errorf("%s: %s", lm.T("settings.provider.label"), lm.T("settings.windows.invalid"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return config.CityConfig{}, fmt.Errorf("%s", lm.T("settings.windows.searchRequest"))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return config.CityConfig{}, fmt.Errorf("%s", lm.T("settings.windows.searchRequest"))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return config.CityConfig{}, fmt.Errorf(lm.T("error.settings.searchApiError"), resp.StatusCode)
	}
	var city config.CityConfig
	if provider == "easyweatherwidget" {
		var result struct {
			Neighborhood string  `json:"Neighborhood"`
			Country      string  `json:"Country"`
			Lat          float64 `json:"Lat"`
			Lon          float64 `json:"Lon"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return city, fmt.Errorf("%s: %w", lm.T("settings.windows.searchResponse"), err)
		}
		city = config.CityConfig{Name: result.Neighborhood, Region: result.Country, Latitude: result.Lat, Longitude: result.Lon}
	} else {
		var results []struct {
			Name, Country, State string
			Lat, Lon             float64
		}
		if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
			return city, fmt.Errorf("%s: %w", lm.T("settings.windows.searchResponse"), err)
		}
		if len(results) > 0 {
			r := results[0]
			city = config.CityConfig{Name: r.Name, Region: r.Country, Latitude: r.Lat, Longitude: r.Lon}
			if r.State != "" {
				city.Region = r.State + ", " + city.Region
			}
		}
	}
	if city.Name == "" {
		return city, fmt.Errorf(lm.T("error.settings.noCityFound"), name)
	}
	city.Timezone = latlong.LookupZoneName(city.Latitude, city.Longitude)
	if city.Timezone == "" {
		return city, fmt.Errorf("%s", lm.T("settings.windows.timezoneUnknown"))
	}
	return city, nil
}
