package weather

import (
	"fmt"
	"regexp"
	"testing"

	"weatherwidget/assets"
	"weatherwidget/internal/config"

	"pgregory.net/rapid"
)

// drawPollutionFields draws a random *config.PollutionFields using nine
// independent boolean selections.
func drawPollutionFields(t *rapid.T) *config.PollutionFields {
	return &config.PollutionFields{
		ShowAQI:  rapid.Bool().Draw(t, "showAQI"),
		ShowCO:   rapid.Bool().Draw(t, "showCO"),
		ShowNO:   rapid.Bool().Draw(t, "showNO"),
		ShowNO2:  rapid.Bool().Draw(t, "showNO2"),
		ShowO3:   rapid.Bool().Draw(t, "showO3"),
		ShowSO2:  rapid.Bool().Draw(t, "showSO2"),
		ShowNH3:  rapid.Bool().Draw(t, "showNH3"),
		ShowPM25: rapid.Bool().Draw(t, "showPM25"),
		ShowPM10: rapid.Bool().Draw(t, "showPM10"),
	}
}

// drawPollutionData draws a random PollutionData where each of the nine
// pointer fields is independently either nil (absent) or a pointer to a
// random value (present).
func drawPollutionData(t *rapid.T) PollutionData {
	intPtrOrNil := func(name string) *int {
		if !rapid.Bool().Draw(t, name+"_present") {
			return nil
		}
		v := rapid.Int().Draw(t, name+"_value")
		return &v
	}
	floatPtrOrNil := func(name string) *float64 {
		if !rapid.Bool().Draw(t, name+"_present") {
			return nil
		}
		v := rapid.Float64().Draw(t, name+"_value")
		return &v
	}
	return PollutionData{
		AQI:  intPtrOrNil("aqi"),
		CO:   floatPtrOrNil("co"),
		NO:   floatPtrOrNil("no"),
		NO2:  floatPtrOrNil("no2"),
		O3:   floatPtrOrNil("o3"),
		SO2:  floatPtrOrNil("so2"),
		NH3:  floatPtrOrNil("nh3"),
		PM25: floatPtrOrNil("pm25"),
		PM10: floatPtrOrNil("pm10"),
	}
}

// selectedFor reports whether a metric is selected in the given fields.
func selectedFor(fields *config.PollutionFields, m PollutionMetric) bool {
	switch m {
	case MetricAQI:
		return fields.ShowAQI
	case MetricCO:
		return fields.ShowCO
	case MetricNO:
		return fields.ShowNO
	case MetricNO2:
		return fields.ShowNO2
	case MetricO3:
		return fields.ShowO3
	case MetricSO2:
		return fields.ShowSO2
	case MetricNH3:
		return fields.ShowNH3
	case MetricPM25:
		return fields.ShowPM25
	case MetricPM10:
		return fields.ShowPM10
	default:
		return false
	}
}

// presentFor reports whether a metric's value pointer is non-nil in the data.
func presentFor(d PollutionData, m PollutionMetric) bool {
	switch m {
	case MetricAQI:
		return d.AQI != nil
	case MetricCO:
		return d.CO != nil
	case MetricNO:
		return d.NO != nil
	case MetricNO2:
		return d.NO2 != nil
	case MetricO3:
		return d.O3 != nil
	case MetricSO2:
		return d.SO2 != nil
	case MetricNH3:
		return d.NH3 != nil
	case MetricPM25:
		return d.PM25 != nil
	case MetricPM10:
		return d.PM10 != nil
	default:
		return false
	}
}

// **Feature: gtk-pollution-display, Property 1: Selection-and-presence membership**
// **Validates: Requirements 5.4, 5.5**

func TestProperty1_SelectionAndPresenceMembership(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		fields := drawPollutionFields(t)
		data := drawPollutionData(t)

		rows := PlanPollutionRows(fields, data)

		emitted := make(map[PollutionMetric]bool, len(rows))
		for _, r := range rows {
			emitted[r.Metric] = true
		}

		for _, m := range PollutionMetricOrder {
			want := selectedFor(fields, m) && presentFor(data, m)
			got := emitted[m]
			if got != want {
				t.Fatalf("metric %d: emitted=%v, want (selected && present)=%v (selected=%v present=%v)",
					m, got, want, selectedFor(fields, m), presentFor(data, m))
			}
		}
	})
}

// **Feature: gtk-pollution-display, Property 2: Canonical output ordering**
// **Validates: Requirements 5.3, 6.1, 6.5**

func TestProperty2_CanonicalOutputOrdering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		fields := drawPollutionFields(t)
		data := drawPollutionData(t)

		rows := PlanPollutionRows(fields, data)

		// Build the expected sequence: canonical order filtered to the
		// selected-and-present set.
		var want []PollutionMetric
		for _, m := range PollutionMetricOrder {
			if selectedFor(fields, m) && presentFor(data, m) {
				want = append(want, m)
			}
		}

		if len(rows) != len(want) {
			t.Fatalf("row count = %d, want %d", len(rows), len(want))
		}

		seen := make(map[PollutionMetric]bool, len(rows))
		known := make(map[PollutionMetric]bool, len(PollutionMetricOrder))
		for _, m := range PollutionMetricOrder {
			known[m] = true
		}

		for i, r := range rows {
			if r.Metric != want[i] {
				t.Fatalf("position %d: metric = %d, want %d (sequence %v vs %v)",
					i, r.Metric, want[i], metricSeq(rows), want)
			}
			if !known[r.Metric] {
				t.Fatalf("position %d: unknown metric %d", i, r.Metric)
			}
			if seen[r.Metric] {
				t.Fatalf("position %d: duplicate metric %d", i, r.Metric)
			}
			seen[r.Metric] = true
		}
	})
}

func metricSeq(rows []PollutionRow) []PollutionMetric {
	out := make([]PollutionMetric, len(rows))
	for i, r := range rows {
		out[i] = r.Metric
	}
	return out
}

// **Feature: gtk-pollution-display, Property 3: AQI category formatting**
// **Validates: Requirements 6.2, 6.3**

func TestProperty3_FormatAQI(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := rapid.Int().Draw(t, "aqi")

		var want string
		switch v {
		case 1:
			want = "Good"
		case 2:
			want = "Fair"
		case 3:
			want = "Moderate"
		case 4:
			want = "Poor"
		case 5:
			want = "Very Poor"
		default:
			want = ""
		}

		expected := fmt.Sprintf("%d (%s)", v, want)
		got := FormatAQI(v)
		if got != expected {
			t.Fatalf("FormatAQI(%d) = %q, want %q", v, got, expected)
		}
	})

	// Explicit examples.
	if got := FormatAQI(2); got != "2 (Fair)" {
		t.Fatalf("FormatAQI(2) = %q, want %q", got, "2 (Fair)")
	}
	if got := FormatAQI(0); got != "0 ()" {
		t.Fatalf("FormatAQI(0) = %q, want %q", got, "0 ()")
	}
	if got := FormatAQI(6); got != "6 ()" {
		t.Fatalf("FormatAQI(6) = %q, want %q", got, "6 ()")
	}
}

// **Feature: gtk-pollution-display, Property 4: AQI category mapping**
// **Validates: Requirements 6.2, 6.3**

func TestProperty4_AQICategory(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := rapid.Int().Draw(t, "aqi")

		var want string
		switch v {
		case 1:
			want = "Good"
		case 2:
			want = "Fair"
		case 3:
			want = "Moderate"
		case 4:
			want = "Poor"
		case 5:
			want = "Very Poor"
		default:
			want = ""
		}

		got := AQICategory(v)
		if got != want {
			t.Fatalf("AQICategory(%d) = %q, want %q", v, got, want)
		}
	})
}

// **Feature: gtk-pollution-display, Property 5: Micrograms unit formatting**
// **Validates: Requirements 6.4**

func TestProperty5_FormatPollutant(t *testing.T) {
	pollutantRe := regexp.MustCompile(`^-?\d+\.\d µg/m³$`)
	rapid.Check(t, func(t *rapid.T) {
		// Use a finite range to guard against NaN/Inf, which "%.1f" would
		// render as "NaN"/"+Inf" and which are not valid pollutant readings.
		v := rapid.Float64Range(-1e9, 1e9).Draw(t, "pollutant")
		out := FormatPollutant(v)
		if !pollutantRe.MatchString(out) {
			t.Fatalf("FormatPollutant(%v) = %q does not match %s", v, out, pollutantRe.String())
		}
	})
}

// **Feature: gtk-pollution-display, Property 6: Icon filename mapping resolves in the embed**
// **Validates: Requirements 4.1, 4.2, 7.1, 7.2**

func TestProperty6_IconMappingResolvesInEmbed(t *testing.T) {
	for _, m := range PollutionMetricOrder {
		file := AirIconFile(m)
		if file == "" {
			t.Fatalf("AirIconFile(metric %d) returned empty filename", m)
		}
		data, err := assets.AirIcons.ReadFile("air/" + file)
		if err != nil {
			t.Fatalf("AirIcons.ReadFile(%q) error: %v", "air/"+file, err)
		}
		if len(data) == 0 {
			t.Fatalf("AirIcons.ReadFile(%q) returned empty bytes", "air/"+file)
		}
	}

	// Explicit NH3 -> nh2.png mapping (Requirement 7.2).
	if got := AirIconFile(MetricNH3); got != "nh2.png" {
		t.Fatalf("AirIconFile(MetricNH3) = %q, want %q", got, "nh2.png")
	}
}
