package assets

import "testing"

// airFilenames is the canonical set of embedded air-quality icon files.
// NH3 maps to the file "nh2.png" per the Air_Icon_Set spec.
var airFilenames = []string{
	"aqi.png",
	"co.png",
	"no.png",
	"no2.png",
	"o3.png",
	"so2.png",
	"nh2.png",
	"pm25.png",
	"pm10.png",
}

// TestAirIconsResolve verifies every air-quality icon resolves from the
// embedded filesystem with non-empty bytes (Requirements 4.1, 4.2).
func TestAirIconsResolve(t *testing.T) {
	for _, name := range airFilenames {
		path := "air/" + name
		data, err := AirIcons.ReadFile(path)
		if err != nil {
			t.Errorf("AirIcons.ReadFile(%q) returned error: %v", path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("AirIcons.ReadFile(%q) returned empty bytes", path)
		}
	}
}

// TestExistingEmbedsResolve guards against regressions in the pre-existing
// Icons, DemoPNG, and Fonts embeds (Requirement 4.3).
func TestExistingEmbedsResolve(t *testing.T) {
	if len(DemoPNG) == 0 {
		t.Error("DemoPNG is empty; expected embedded demo image bytes")
	}

	fontEntries, err := Fonts.ReadDir("fonts")
	if err != nil {
		t.Fatalf("Fonts.ReadDir(\"fonts\") returned error: %v", err)
	}
	if len(fontEntries) == 0 {
		t.Error("Fonts embed has no entries under fonts/")
	}

	iconEntries, err := Icons.ReadDir("icons")
	if err != nil {
		t.Fatalf("Icons.ReadDir(\"icons\") returned error: %v", err)
	}
	if len(iconEntries) == 0 {
		t.Error("Icons embed has no entries under icons/")
	}
}
