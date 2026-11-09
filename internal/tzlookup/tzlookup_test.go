package tzlookup

import "testing"

func TestLookup(t *testing.T) {
	cases := []struct {
		name     string
		lat, lon float64
		want     string
	}{
		{"Singapore", 1.289918, 103.851907, "Asia/Singapore"},
		{"London", 51.5074, -0.1278, "Europe/London"},
		{"New York", 40.7128, -74.0060, "America/New_York"},
	}
	for _, c := range cases {
		if got := Lookup(c.lat, c.lon); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
