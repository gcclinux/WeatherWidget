package i18n

import "testing"

func TestForecastTitle(t *testing.T) {
	if got := ForecastTitle(nil, "today"); got != "Today's\nForecast" {
		t.Errorf("today = %q", got)
	}
	if got := ForecastTitle(nil, "tomorrow"); got != "Tomorrow's\nForecast" {
		t.Errorf("tomorrow = %q", got)
	}
	if got := ForecastTitle(nil, "live"); got != "" {
		t.Errorf("live = %q", got)
	}
}
