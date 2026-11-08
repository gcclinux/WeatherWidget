package winres

import (
	_ "embed"
	"encoding/json"
	"os"
	"strings"
)

//go:embed winres.json
var winresJSON []byte

// Version returns the application version defined in winres.json or the release file.
// It ensures the version is always accurate and does not need to be statically updated.
func Version() string {
	// Parse embedded winres.json (compiled into binary)
	type manifestIdentity struct {
		Version string `json:"version"`
	}
	type manifestEntry struct {
		Identity manifestIdentity `json:"identity"`
	}
	type schema struct {
		RTManifest map[string]map[string]manifestEntry `json:"RT_MANIFEST"`
		RTVersion  map[string]map[string]struct {
			Fixed struct {
				ProductVersion string `json:"product_version"`
			} `json:"fixed"`
		} `json:"RT_VERSION"`
	}
	var s schema
	if err := json.Unmarshal(winresJSON, &s); err == nil {
		for _, m := range s.RTVersion {
			for _, entry := range m {
				if v := strings.TrimSpace(entry.Fixed.ProductVersion); v != "" {
					return v
				}
			}
		}
		for _, m := range s.RTManifest {
			for _, entry := range m {
				if v := strings.TrimSpace(entry.Identity.Version); v != "" {
					return v
				}
			}
		}
	}

	// Check release file on disk if accessible (e.g. running from repository root)
	for _, path := range []string{"release", "../release", "../../release"} {
		if data, err := os.ReadFile(path); err == nil {
			if v := strings.TrimSpace(string(data)); v != "" {
				return v
			}
		}
	}

	return "2.3.0"
}
