package i18n

import "strings"

// ForecastTitle returns the localized two-line title shown in place of the
// forecast summary on Simple (Classic) cards for the "today" and "tomorrow"
// grid modes. It reuses the Grid Content labels (settings.gridMode.today and
// settings.gridMode.tomorrow) and breaks them at the first space, e.g.
// "Today's\nForecast". A label without a space (e.g. Japanese, Chinese) stays
// on one line. Any other mode returns "". A nil manager or missing key falls
// back to English.
func ForecastTitle(lm *LocaleManager, mode string) string {
	key, fallback := "", ""
	switch mode {
	case "today":
		key, fallback = "settings.gridMode.today", "Today's Forecast"
	case "tomorrow":
		key, fallback = "settings.gridMode.tomorrow", "Tomorrow's Forecast"
	default:
		return ""
	}
	title := fallback
	if lm != nil {
		if v := strings.TrimSpace(lm.T(key)); v != "" && v != key {
			title = v
		}
	}
	if first, rest, ok := strings.Cut(title, " "); ok {
		return first + "\n" + strings.TrimSpace(rest)
	}
	return title
}
