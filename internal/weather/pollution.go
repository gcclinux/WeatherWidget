package weather

import (
	"fmt"

	"weatherwidget/internal/config"
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

// FormatPollutant renders a µg/m³ value with one decimal and the unit suffix.
func FormatPollutant(v float64) string { return fmt.Sprintf("%.1f µg/m³", v) }

// PlanPollutionRows returns the rows to render, already ordered, containing
// only metrics that are both selected in fields AND have a value present in d.
// Pure: no GTK, no I/O. When fields is nil, config.DefaultPollutionFields()
// is used. No header entry is emitted.
func PlanPollutionRows(fields *config.PollutionFields, d PollutionData) []PollutionRow {
	if fields == nil {
		fields = config.DefaultPollutionFields()
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
				value = FormatAQI(*d.AQI)
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
