package weather

import (
	"fmt"

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

// PollutionTickerItem represents one planned ticker sentence for a selected,
// present air-quality metric in Enhanced mode.
type PollutionTickerItem struct {
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

// PlanPollutionTicker returns the localized ticker items for all metrics that
// are selected in fields AND present in d, when fields.ShowTicker is true.
// Returns nil when fields is nil or fields.ShowTicker is false.
func PlanPollutionTicker(fields *config.PollutionFields, d PollutionData, lm ...*i18n.LocaleManager) []PollutionTickerItem {
	if fields == nil || !fields.ShowTicker {
		return nil
	}

	var localeMgr *i18n.LocaleManager
	if len(lm) > 0 {
		localeMgr = lm[0]
	}

	currentlyLbl := tr(localeMgr, "pollution.ticker.currently", "Currently")
	sourcesLbl := tr(localeMgr, "pollution.ticker.sources", "Primary sources")

	var items []PollutionTickerItem
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

		if selected && present {
			meta := pollutantMeta[m]
			items = append(items, PollutionTickerItem{
				Metric:       m,
				Symbol:       meta.symbol,
				FullName:     tr(localeMgr, meta.nameKey, meta.nameFallback),
				CurrentlyLbl: currentlyLbl,
				StatusText:   AQICategoryLocalized(level, localeMgr),
				StatusLevel:  level,
				SourcesLbl:   sourcesLbl,
				SourcesText:  tr(localeMgr, meta.sourceKey, meta.sourceFallback),
			})
		}
	}
	return items
}
