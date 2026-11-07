//go:build windows

package uiwindows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather/remoteapi"
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adapter := remoteapi.NewRemoteAPIAdapter(provider, key)
	city, err := adapter.SearchCity(ctx, name, region)
	if err != nil {
		var se *remoteapi.CitySearchError
		if errors.As(err, &se) {
			switch se.Kind {
			case remoteapi.SearchErrInvalidProvider:
				return config.CityConfig{}, fmt.Errorf("%s: %s", lm.T("settings.provider.label"), lm.T("settings.windows.invalid"))
			case remoteapi.SearchErrRequest:
				return config.CityConfig{}, fmt.Errorf("%s", lm.T("settings.windows.searchRequest"))
			case remoteapi.SearchErrAPIStatus:
				return config.CityConfig{}, fmt.Errorf(lm.T("error.settings.searchApiError"), se.StatusCode)
			case remoteapi.SearchErrDecode:
				return config.CityConfig{}, fmt.Errorf("%s: %w", lm.T("settings.windows.searchResponse"), se.Err)
			case remoteapi.SearchErrNotFound:
				return config.CityConfig{}, fmt.Errorf(lm.T("error.settings.noCityFound"), name)
			}
		}
		return config.CityConfig{}, err
	}
	if city.Timezone == "" {
		return city, fmt.Errorf("%s", lm.T("settings.windows.timezoneUnknown"))
	}
	return city, nil
}
