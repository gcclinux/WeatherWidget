//go:build linux

package uitk

import "github.com/bradfitz/latlong"

// gtkLookupTimezone returns the IANA timezone name for the given lat/lon.
// Uses the bradfitz/latlong package which is already a project dependency.
func gtkLookupTimezone(lat, lon float64) string {
	return latlong.LookupZoneName(lat, lon)
}
