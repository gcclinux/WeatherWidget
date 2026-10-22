package weather

import (
	"fmt"
	"hash/fnv"
	"strings"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
)

// PollutionMetric identifies one air-quality metric.
type PollutionMetric int

const (
	MetricAQI PollutionMetric = iota
	MetricCO
	MetricNO
	MetricNO2
	MetricO3
	MetricSO2
	MetricNH3
	MetricPM25
	MetricPM10
)

// PollutionMetricOrder is the canonical render order: AQI first, then the
// micrograms pollutants in the fixed sequence.
var PollutionMetricOrder = []PollutionMetric{
	MetricAQI, MetricCO, MetricNO, MetricNO2, MetricO3,
	MetricSO2, MetricNH3, MetricPM25, MetricPM10,
}

// airIconFile maps each metric to its embedded filename under air/.
// Note: NH3 maps to nh2.png (the on-disk asset name).
var airIconFile = map[PollutionMetric]string{
	MetricAQI:  "aqi.png",
	MetricCO:   "co.png",
	MetricNO:   "no.png",
	MetricNO2:  "no2.png",
	MetricO3:   "o3.png",
	MetricSO2:  "so2.png",
	MetricNH3:  "nh2.png",
	MetricPM25: "pm25.png",
	MetricPM10: "pm10.png",
}

// AirIconFile returns the embedded air/ filename for a metric,
// or "" for an unknown metric.
func AirIconFile(m PollutionMetric) string { return airIconFile[m] }

// PollutionData is the presence-aware view the planner consumes.
// A nil pointer means the value is absent; a non-nil pointer (even to 0)
// means the value is present.
type PollutionData struct {
	AQI                                   *int
	CO, NO, NO2, O3, SO2, NH3, PM25, PM10 *float64
}

// PollutionOf extracts a PollutionData view from a WeatherData. A nil input
// yields an all-nil view (all metrics absent).
func PollutionOf(d *WeatherData) PollutionData {
	if d == nil {
		return PollutionData{}
	}
	return PollutionData{
		AQI:  d.AQI,
		CO:   d.CO,
		NO:   d.NO,
		NO2:  d.NO2,
		O3:   d.O3,
		SO2:  d.SO2,
		NH3:  d.NH3,
		PM25: d.PM25,
		PM10: d.PM10,
	}
}

// PollutionRow is one planned, ready-to-render pollution row.
type PollutionRow struct {
	Metric    PollutionMetric
	IconFile  string // filename within the AirIcons embed (e.g. "co.png")
	ValueText string // fully formatted value text
}

// AQICategory maps the OpenWeather AQI index (1-5) to its qualitative label.
// Values outside 1-5 return "" (empty category).
func AQICategory(v int) string {
	switch v {
	case 1:
		return "Good"
	case 2:
		return "Fair"
	case 3:
		return "Moderate"
	case 4:
		return "Poor"
	case 5:
		return "Very Poor"
	default:
		return ""
	}
}

// FormatAQI renders the AQI as "N (Category)" for indices 1-5, e.g. "2 (Fair)".
// For values outside 1-5, the category is empty and the output is "N ()".
func FormatAQI(v int) string {
	return fmt.Sprintf("%d (%s)", v, AQICategory(v))
}

// aqiCategoryKey maps each AQI index (1-5) to its i18n key and English fallback.
var aqiCategoryKey = map[int]struct {
	key      string
	fallback string
}{
	1: {"weather.aqi.good", "Good"},
	2: {"weather.aqi.fair", "Fair"},
	3: {"weather.aqi.moderate", "Moderate"},
	4: {"weather.aqi.poor", "Poor"},
	5: {"weather.aqi.veryPoor", "Very Poor"},
}

// AQICategoryLocalized is like AQICategory but returns the qualitative label in
// the active locale (falling back to English when lm is nil or the key is
// missing). Values outside 1-5 return "".
func AQICategoryLocalized(v int, lm *i18n.LocaleManager) string {
	e, ok := aqiCategoryKey[v]
	if !ok {
		return ""
	}
	return tr(lm, e.key, e.fallback)
}

// FormatAQILocalized renders "N (Category)" with the category localized.
func FormatAQILocalized(v int, lm *i18n.LocaleManager) string {
	return fmt.Sprintf("%d (%s)", v, AQICategoryLocalized(v, lm))
}

// FormatPollutant renders a µg/m³ value with one decimal and the unit suffix.
func FormatPollutant(v float64) string { return fmt.Sprintf("%.1f µg/m³", v) }

// PlanPollutionRows returns the rows to render, already ordered, containing
// only metrics that are both selected in fields AND have a value present in d.
// Pure: no GTK, no I/O. When fields is nil, config.DefaultPollutionFields()
// is used. No header entry is emitted.
// An optional *i18n.LocaleManager may be passed to localize the AQI category
// label (e.g. "Good" → "Gut"). It is variadic so existing callers that don't
// have a LocaleManager keep working unchanged.
func PlanPollutionRows(fields *config.PollutionFields, d PollutionData, lm ...*i18n.LocaleManager) []PollutionRow {
	if fields == nil {
		fields = config.DefaultPollutionFields()
	}

	var localeMgr *i18n.LocaleManager
	if len(lm) > 0 {
		localeMgr = lm[0]
	}

	var rows []PollutionRow
	for _, m := range PollutionMetricOrder {
		var (
			selected bool
			value    string
			present  bool
		)
		switch m {
		case MetricAQI:
			selected = fields.ShowAQI
			if d.AQI != nil {
				present = true
				value = FormatAQILocalized(*d.AQI, localeMgr)
			}
		case MetricCO:
			selected = fields.ShowCO
			if d.CO != nil {
				present = true
				value = FormatPollutant(*d.CO)
			}
		case MetricNO:
			selected = fields.ShowNO
			if d.NO != nil {
				present = true
				value = FormatPollutant(*d.NO)
			}
		case MetricNO2:
			selected = fields.ShowNO2
			if d.NO2 != nil {
				present = true
				value = FormatPollutant(*d.NO2)
			}
		case MetricO3:
			selected = fields.ShowO3
			if d.O3 != nil {
				present = true
				value = FormatPollutant(*d.O3)
			}
		case MetricSO2:
			selected = fields.ShowSO2
			if d.SO2 != nil {
				present = true
				value = FormatPollutant(*d.SO2)
			}
		case MetricNH3:
			selected = fields.ShowNH3
			if d.NH3 != nil {
				present = true
				value = FormatPollutant(*d.NH3)
			}
		case MetricPM25:
			selected = fields.ShowPM25
			if d.PM25 != nil {
				present = true
				value = FormatPollutant(*d.PM25)
			}
		case MetricPM10:
			selected = fields.ShowPM10
			if d.PM10 != nil {
				present = true
				value = FormatPollutant(*d.PM10)
			}
		}

		if selected && present {
			rows = append(rows, PollutionRow{
				Metric:    m,
				IconFile:  AirIconFile(m),
				ValueText: value,
			})
		}
	}
	return rows
}

// TickerItemKind identifies the purpose of an environmental ticker message.
// The planner emits an urgent alert before a positive opportunity or a
// pollutant context item, so desktop users see practical guidance first.
type TickerItemKind string

const (
	TickerItemAlert       TickerItemKind = "alert"
	TickerItemOpportunity TickerItemKind = "opportunity"
	TickerItemContext     TickerItemKind = "context"
)

// PollutionTickerItem is one planned, localized environmental ticker message.
// Existing renderers split Symbol, StatusText, and SourcesText into differently
// styled segments; the names are retained so all three desktop UIs can render
// both the legacy pollutant context and the new actionable guidance.
type PollutionTickerItem struct {
	Kind         TickerItemKind
	Metric       PollutionMetric
	Symbol       string // e.g. "PM2.5", "NO₂", "O₃", "AQI"
	FullName     string // localized full name, e.g. "Particulate 2.5"
	CurrentlyLbl string // localized "Currently"
	StatusText   string // localized qualitative status, e.g. "Good"
	StatusLevel  int    // 1=Good, 2=Fair, 3=Moderate, 4=Poor, 5=Very Poor
	SourcesLbl   string // localized "Primary sources"
	SourcesText  string // localized primary sources list
}

var pollutantMeta = map[PollutionMetric]struct {
	symbol         string
	nameKey        string
	nameFallback   string
	sourceKey      string
	sourceFallback string
}{
	MetricAQI: {
		symbol:         "AQI",
		nameKey:        "pollution.aqi",
		nameFallback:   "Air Quality Index",
		sourceKey:      "pollution.sources.aqi",
		sourceFallback: "Composite index of all monitored air pollutants",
	},
	MetricCO: {
		symbol:         "CO",
		nameKey:        "pollution.co",
		nameFallback:   "Carbon Monoxide",
		sourceKey:      "pollution.sources.co",
		sourceFallback: "Vehicle exhaust, gas heaters & tobacco smoke",
	},
	MetricNO: {
		symbol:         "NO",
		nameKey:        "pollution.no",
		nameFallback:   "Nitric Oxide",
		sourceKey:      "pollution.sources.no",
		sourceFallback: "Vehicle engines & power plant furnaces",
	},
	MetricNO2: {
		symbol:         "NO₂",
		nameKey:        "pollution.no2",
		nameFallback:   "Nitrogen Dioxide",
		sourceKey:      "pollution.sources.no2",
		sourceFallback: "Diesel & petrol vehicles, power plants, gas heating",
	},
	MetricO3: {
		symbol:         "O₃",
		nameKey:        "pollution.o3",
		nameFallback:   "Ozone",
		sourceKey:      "pollution.sources.o3",
		sourceFallback: "Sunlight reacting with vehicle exhaust & fumes",
	},
	MetricSO2: {
		symbol:         "SO₂",
		nameKey:        "pollution.so2",
		nameFallback:   "Sulphur Dioxide",
		sourceKey:      "pollution.sources.so2",
		sourceFallback: "Coal & oil burning, refineries & shipping fuel",
	},
	MetricNH3: {
		symbol:         "NH₃",
		nameKey:        "pollution.nh3",
		nameFallback:   "Ammonia",
		sourceKey:      "pollution.sources.nh3",
		sourceFallback: "Agriculture, fertilizers & vehicle catalysts",
	},
	MetricPM25: {
		symbol:         "PM2.5",
		nameKey:        "pollution.pm25",
		nameFallback:   "Particulate 2.5",
		sourceKey:      "pollution.sources.pm25",
		sourceFallback: "Vehicle exhaust, wood burning & industry",
	},
	MetricPM10: {
		symbol:         "PM10",
		nameKey:        "pollution.pm10",
		nameFallback:   "Particulate 10",
		sourceKey:      "pollution.sources.pm10",
		sourceFallback: "Road & construction dust, farming & quarries",
	},
}

// PollutantSymbol returns the short chemical/index symbol for a metric (e.g. "NO₂", "PM2.5").
func PollutantSymbol(m PollutionMetric) string {
	return pollutantMeta[m].symbol
}

// PollutantLevel maps a pollutant concentration (µg/m³) to a 1-5 status level
// (1=Good, 2=Fair, 3=Moderate, 4=Poor, 5=Very Poor) using the UK DAQI and
// OpenWeather environmental thresholds.
func PollutantLevel(m PollutionMetric, val float64) int {
	switch m {
	case MetricPM25:
		switch {
		case val <= 11:
			return 1
		case val <= 35:
			return 2
		case val <= 53:
			return 3
		case val <= 70:
			return 4
		default:
			return 5
		}
	case MetricPM10:
		switch {
		case val <= 16:
			return 1
		case val <= 50:
			return 2
		case val <= 75:
			return 3
		case val <= 100:
			return 4
		default:
			return 5
		}
	case MetricNO2:
		switch {
		case val <= 67:
			return 1
		case val <= 200:
			return 2
		case val <= 400:
			return 3
		case val <= 600:
			return 4
		default:
			return 5
		}
	case MetricO3:
		switch {
		case val <= 33:
			return 1
		case val <= 100:
			return 2
		case val <= 160:
			return 3
		case val <= 240:
			return 4
		default:
			return 5
		}
	case MetricSO2:
		switch {
		case val <= 88:
			return 1
		case val <= 266:
			return 2
		case val <= 532:
			return 3
		case val <= 1064:
			return 4
		default:
			return 5
		}
	case MetricCO:
		switch {
		case val < 4400:
			return 1
		case val < 9400:
			return 2
		case val < 12400:
			return 3
		case val < 15400:
			return 4
		default:
			return 5
		}
	case MetricNO:
		switch {
		case val < 20:
			return 1
		case val < 50:
			return 2
		case val < 80:
			return 3
		case val < 100:
			return 4
		default:
			return 5
		}
	case MetricNH3:
		switch {
		case val < 40:
			return 1
		case val < 100:
			return 2
		case val < 150:
			return 3
		case val < 200:
			return 4
		default:
			return 5
		}
	default:
		return 1
	}
}

// tickerProfileMeta defines a local preference's alert sensitivity and
// practical response. These profiles do not collect or diagnose health data.
var tickerProfileMeta = map[config.TickerProfile]struct {
	labelKey, labelFallback   string
	actionKey, actionFallback string
	alertLevel                int
}{
	config.TickerProfileEveryday: {
		"settings.pollution.profile.everyday", "Everyday Outdoors",
		"pollution.ticker.action.everyday", "consider reducing prolonged outdoor activity", 4,
	},
	config.TickerProfileActive: {
		"settings.pollution.profile.active", "Runner & Cyclist",
		"pollution.ticker.action.active", "reduce strenuous outdoor activity", 3,
	},
	config.TickerProfileFamily: {
		"settings.pollution.profile.family", "Family & School Run",
		"pollution.ticker.action.family", "limit prolonged outdoor time for children", 3,
	},
	config.TickerProfileSensitive: {
		"settings.pollution.profile.sensitive", "Sensitive Airways",
		"pollution.ticker.action.sensitive", "take extra care outdoors", 2,
	},
	config.TickerProfileVentilation: {
		"settings.pollution.profile.ventilation", "Open Windows",
		"pollution.ticker.action.ventilation", "keep windows closed for now", 3,
	},
}

func tickerProfileDetails(profile config.TickerProfile, lm *i18n.LocaleManager) (label, action string, alertLevel int) {
	meta := tickerProfileMeta[config.NormalizeTickerProfile(profile)]
	return tr(lm, meta.labelKey, meta.labelFallback), tr(lm, meta.actionKey, meta.actionFallback), meta.alertLevel
}

// PlanPollutionTicker returns the localized, priority-ranked messages for the
// enhanced ticker. It always chooses the most severe selected pollutant as
// context. When that pollutant breaches the local profile's threshold, an
// alert leads the rotation; otherwise a positive current-condition opportunity
// leads it. Returns nil when the Pro ticker is disabled or has no usable data.
func PlanPollutionTicker(fields *config.PollutionFields, d PollutionData, lm ...*i18n.LocaleManager) []PollutionTickerItem {
	return planPollutionTicker(fields, d, nil, config.TemperatureUnitCelsius, lm...)
}

// PlanPollutionTickerWithWeather behaves like PlanPollutionTicker but also
// considers the city's current weather. When air quality is favourable, the
// opportunity message is enriched with practical advice (e.g. take an
// umbrella when it is raining, dress warm and take a torch on a cold night).
// tempUnit is only used to format the "feels like" value shown in the advice.
func PlanPollutionTickerWithWeather(fields *config.PollutionFields, w *WeatherData, tempUnit config.TemperatureUnit, lm ...*i18n.LocaleManager) []PollutionTickerItem {
	return planPollutionTicker(fields, PollutionOf(w), w, tempUnit, lm...)
}

// Thresholds (°C, apparent temperature) and UV level for weather advice.
const (
	tickerColdFeelsLikeC = 5.0
	tickerHotFeelsLikeC  = 30.0
	tickerHighUV         = 6.0
)

// tickerVariant picks a stable wording variant (0 or 1) for a city and local
// hour, so the message does not flicker on every refresh but still changes
// over the day and differs between cities.
func tickerVariant(w *WeatherData) int {
	if w == nil {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(w.CityName))
	return int((h.Sum32() + uint32(w.LocalTime.Hour())) % 2)
}

// trVariant returns the localized text for key, or its ".alt" form when
// variant is 1 and an alternative exists.
func trVariant(lm *i18n.LocaleManager, key, fallback, fallbackAlt string, variant int) string {
	if variant == 1 {
		return tr(lm, key+".alt", fallbackAlt)
	}
	return tr(lm, key, fallback)
}

// weatherAdvice builds one friendly sentence of practical advice for the
// current weather, e.g. "Feels like 3°C, so wrap up warm 🧥 and take a torch 🔦".
// The main clause is the most relevant condition (storm, snow, rain, fog,
// then cold or heat); darkness or high UV is appended as a short extra action,
// or stands alone when nothing else applies. Returns "" when no advice is needed.
func weatherAdvice(w *WeatherData, tempUnit config.TemperatureUnit, lm *i18n.LocaleManager) string {
	if w == nil {
		return ""
	}
	v := tickerVariant(w)

	// Prefer the apparent temperature; fall back to the air temperature.
	feels := float64(w.Temperature)
	feelsPtr := w.FeelsLike
	if feelsPtr != nil {
		feels = *feelsPtr
	} else {
		t := feels
		feelsPtr = &t
	}
	feelsStr := FormatFeelsLike(feelsPtr, tempUnit)

	desc := strings.ToLower(w.Description)
	cond := NormalizeCondition(w.IconCode)

	var main string
	switch {
	case cond == IconStorm || strings.Contains(desc, "thunder"):
		main = trVariant(lm, "pollution.ticker.weather.storm",
			"Thunderstorms about, best to stay indoors ⛈️", "Stormy out there, better to stay inside ⛈️", v)
	case cond == IconSnow || strings.Contains(desc, "snow") || strings.Contains(desc, "sleet"):
		main = trVariant(lm, "pollution.ticker.weather.snow",
			"It's snowing, so wrap up and watch for ice ❄️", "Snow about, so wrap up warm and mind the ice ❄️", v)
	case cond == IconRain || cond == IconHeavyRain ||
		strings.Contains(desc, "rain") || strings.Contains(desc, "drizzle") || strings.Contains(desc, "shower"):
		main = trVariant(lm, "pollution.ticker.weather.rain",
			"It's raining, so you may get wet, take an umbrella ☔", "Rain about, so bring an umbrella to stay dry ☔", v)
	case cond == IconFog || strings.Contains(desc, "fog") || strings.Contains(desc, "mist"):
		main = trVariant(lm, "pollution.ticker.weather.fog",
			"It's foggy, so take care out there 🌫️", "Visibility is low in the fog, so take it easy 🌫️", v)
	case feels <= tickerColdFeelsLikeC:
		main = fmt.Sprintf(trVariant(lm, "pollution.ticker.weather.cold",
			"Feels like %s, so wrap up warm 🧥", "Brr, it feels like %s, so dress warm 🧥", v), feelsStr)
	case feels >= tickerHotFeelsLikeC:
		main = fmt.Sprintf(trVariant(lm, "pollution.ticker.weather.hot",
			"Feels like %s, so drink plenty of water and find some shade ☀️",
			"It's hot out there, feels like %s, so stay hydrated and seek shade ☀️", v), feelsStr)
	}

	// Darkness or strong sun: a short extra action, or a full sentence on its own.
	var full, add string
	if !w.LocalTime.IsZero() {
		if IsNight(w.LocalTime) {
			full = tr(lm, "pollution.ticker.weather.night", "It's dark out, so take a torch 🔦")
			add = tr(lm, "pollution.ticker.weather.night.add", "take a torch 🔦")
		} else if w.UVIndex >= tickerHighUV {
			full = tr(lm, "pollution.ticker.weather.uv", "The UV is high, so wear sun protection 😎")
			add = tr(lm, "pollution.ticker.weather.uv.add", "wear sun protection 😎")
		}
	}

	switch {
	case main != "" && add != "":
		return main + " " + tr(lm, "pollution.ticker.weather.and", "and") + " " + add
	case main != "":
		return main
	default:
		return full
	}
}

func planPollutionTicker(fields *config.PollutionFields, d PollutionData, w *WeatherData, tempUnit config.TemperatureUnit, lm ...*i18n.LocaleManager) []PollutionTickerItem {
	if fields == nil || !fields.ShowTicker {
		return nil
	}

	var localeMgr *i18n.LocaleManager
	if len(lm) > 0 {
		localeMgr = lm[0]
	}

	currentlyLbl := tr(localeMgr, "pollution.ticker.currently", "Currently")
	sourcesLbl := tr(localeMgr, "pollution.ticker.sources", "Primary sources")

	var selectedContexts []PollutionTickerItem
	var presentContexts []PollutionTickerItem
	for _, m := range PollutionMetricOrder {
		var (
			selected bool
			present  bool
			level    int
		)
		switch m {
		case MetricAQI:
			selected = fields.ShowAQI
			if d.AQI != nil {
				present = true
				lvl := *d.AQI
				if lvl < 1 {
					lvl = 1
				} else if lvl > 5 {
					lvl = 5
				}
				level = lvl
			}
		case MetricCO:
			selected = fields.ShowCO
			if d.CO != nil {
				present = true
				level = PollutantLevel(m, *d.CO)
			}
		case MetricNO:
			selected = fields.ShowNO
			if d.NO != nil {
				present = true
				level = PollutantLevel(m, *d.NO)
			}
		case MetricNO2:
			selected = fields.ShowNO2
			if d.NO2 != nil {
				present = true
				level = PollutantLevel(m, *d.NO2)
			}
		case MetricO3:
			selected = fields.ShowO3
			if d.O3 != nil {
				present = true
				level = PollutantLevel(m, *d.O3)
			}
		case MetricSO2:
			selected = fields.ShowSO2
			if d.SO2 != nil {
				present = true
				level = PollutantLevel(m, *d.SO2)
			}
		case MetricNH3:
			selected = fields.ShowNH3
			if d.NH3 != nil {
				present = true
				level = PollutantLevel(m, *d.NH3)
			}
		case MetricPM25:
			selected = fields.ShowPM25
			if d.PM25 != nil {
				present = true
				level = PollutantLevel(m, *d.PM25)
			}
		case MetricPM10:
			selected = fields.ShowPM10
			if d.PM10 != nil {
				present = true
				level = PollutantLevel(m, *d.PM10)
			}
		}

		if present {
			meta := pollutantMeta[m]
			item := PollutionTickerItem{
				Kind:         TickerItemContext,
				Metric:       m,
				Symbol:       meta.symbol,
				FullName:     tr(localeMgr, meta.nameKey, meta.nameFallback),
				CurrentlyLbl: currentlyLbl,
				StatusText:   AQICategoryLocalized(level, localeMgr),
				StatusLevel:  level,
				SourcesLbl:   sourcesLbl,
				SourcesText:  tr(localeMgr, meta.sourceKey, meta.sourceFallback),
			}
			presentContexts = append(presentContexts, item)
			if selected {
				selectedContexts = append(selectedContexts, item)
			}
		}
	}
	if len(presentContexts) == 0 {
		return nil
	}

	dominantPool := selectedContexts
	if len(dominantPool) == 0 {
		dominantPool = presentContexts
	}

	// Stable tie-breaking preserves PollutionMetricOrder, so a rerender does
	// not make the ticker jump between equally severe pollutants.
	dominant := dominantPool[0]
	for _, item := range dominantPool[1:] {
		if item.StatusLevel > dominant.StatusLevel {
			dominant = item
		}
	}

	_, _, alertLevel := tickerProfileDetails(fields.TickerProfile, localeMgr)
	alertLabel := tr(localeMgr, "pollution.ticker.alert", "Alert")
	opportunityLabel := tr(localeMgr, "pollution.ticker.opportunity", "Opportunity")
	contextLabel := tr(localeMgr, "pollution.ticker.context", "Context")

	// Profile- and level-specific guidance, optionally followed by a tip about
	// the dominant pollutant. The profile name itself is not repeated: the user
	// already chose it in settings.
	guidance := tickerGuidance(fields.TickerProfile, dominant.StatusLevel, localeMgr)
	advise := guidance
	if tip := pollutantTipText(dominant.Metric, localeMgr); tip != "" {
		advise = guidance + " • " + tip
	}

	// Turn the dominant raw pollutant item into a compact explanatory fallback.
	context := dominant
	context.Kind = TickerItemContext
	context.Symbol = "ℹ " + contextLabel
	context.FullName = ""
	context.StatusText = fmt.Sprintf("%s: %s", dominant.Symbol, dominant.StatusText)

	if dominant.StatusLevel >= alertLevel {
		items := []PollutionTickerItem{
			{
				Kind:        TickerItemAlert,
				Metric:      dominant.Metric,
				Symbol:      "⚠ " + alertLabel,
				StatusText:  fmt.Sprintf("%s: %s", dominant.Symbol, dominant.StatusText),
				StatusLevel: dominant.StatusLevel,
				SourcesText: advise,
			},
			context,
		}
		return append(items, selectedContexts...)
	}

	// Do not present moderate conditions as a positive opportunity merely
	// because they remain below a less-sensitive profile's alert threshold.
	// Show the profile's moderate-level guidance instead of the generic sources.
	if dominant.StatusLevel > 2 {
		context.SourcesText = advise
		return append([]PollutionTickerItem{context}, selectedContexts...)
	}

	// Friendly headline plus either weather-aware advice or the profile's
	// own "good to go" message.
	v := tickerVariant(w)
	var status string
	if dominant.StatusLevel <= 1 {
		status = trVariant(localeMgr, "pollution.ticker.opportunity.message",
			"Fresh, clean air 🌿", "Lovely air out there 🌿", v)
	} else {
		status = trVariant(localeMgr, "pollution.ticker.opportunity.fair",
			"Air quality is decent 🙂", "Pretty good air today 🙂", v)
	}
	tail := weatherAdvice(w, tempUnit, localeMgr)
	if tail == "" {
		tail = guidance
	}

	opp := PollutionTickerItem{
		Kind:        TickerItemOpportunity,
		Metric:      dominant.Metric,
		Symbol:      "✓ " + opportunityLabel,
		StatusText:  status,
		StatusLevel: 1,
		SourcesText: tail,
	}

	return append([]PollutionTickerItem{opp, context}, selectedContexts...)
}
