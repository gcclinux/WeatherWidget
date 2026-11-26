package remoteapi

import "strings"

// countryAliases maps common informal or non-ISO country/nation names (lowercased)
// to their ISO 3166-1 alpha-2 country codes for city auto-fill lookups.
var countryAliases = map[string]string{
	"uk":          "GB",
	"england":     "GB",
	"scotland":    "GB",
	"holland":     "NL",
	"nederland":   "NL",
	"netherland":  "NL",
	"netherlands": "NL",
	"india":       "IN",
}

// normalizeSearchLocation trims whitespace from name and region and expands any
// recognized country aliases into ISO 3166-1 alpha-2 codes. If region is empty
// and name has a trailing ", <alias>", the city name and resolved country code
// are split and returned separately.
func normalizeSearchLocation(name, region string) (string, string) {
	name = strings.TrimSpace(name)
	region = strings.TrimSpace(region)

	if region != "" {
		return name, normalizeRegionCountryAlias(region)
	}

	if idx := strings.LastIndex(name, ","); idx != -1 {
		cityPart := strings.TrimSpace(name[:idx])
		countryPart := strings.TrimSpace(name[idx+1:])
		if cityPart != "" {
			if code, ok := countryAliases[strings.ToLower(countryPart)]; ok {
				return cityPart, code
			}
		}
	}

	return name, region
}

// normalizeRegionCountryAlias replaces a recognized country alias (either as the
// entire region string or as the trailing comma-separated segment) with its ISO
// 3166-1 alpha-2 code.
func normalizeRegionCountryAlias(region string) string {
	region = strings.TrimSpace(region)
	if region == "" {
		return ""
	}

	if code, ok := countryAliases[strings.ToLower(region)]; ok {
		return code
	}

	if idx := strings.LastIndex(region, ","); idx != -1 {
		prefix := strings.TrimSpace(region[:idx])
		countryPart := strings.TrimSpace(region[idx+1:])
		if code, ok := countryAliases[strings.ToLower(countryPart)]; ok {
			if prefix == "" {
				return code
			}
			return prefix + ", " + code
		}
	}

	return region
}
