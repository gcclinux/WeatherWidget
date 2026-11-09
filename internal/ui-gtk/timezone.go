//go:build linux

package uitk

import "weatherwidget/internal/tzlookup"

// gtkLookupTimezone returns the IANA timezone name for the given lat/lon,
// or "" if it cannot be determined.
func gtkLookupTimezone(lat, lon float64) string {
	return tzlookup.Lookup(lat, lon)
}
