// Package tzlookup resolves IANA timezone names from coordinates.
//
// bradfitz/latlong uses coarse polygons, so small or coastal places (for
// example Singapore) can return an empty string. Lookup falls back to probing
// progressively larger rings around the point and returns the zone of the
// first ring that hits land.
package tzlookup

import (
	"math"

	"github.com/bradfitz/latlong"
)

// ringRadiiDeg are the probe radii (in degrees) tried, in order, when the
// exact point has no zone. Kept small so we never jump to a distant country.
var ringRadiiDeg = []float64{0.02, 0.05, 0.1, 0.2, 0.35, 0.5}

const ringProbes = 16

// Lookup returns the IANA timezone for lat/lon, or "" if none could be
// determined. Callers should treat "" as "unknown" rather than defaulting to UTC.
func Lookup(lat, lon float64) string {
	if tz := latlong.LookupZoneName(lat, lon); tz != "" {
		return tz
	}
	for _, r := range ringRadiiDeg {
		votes := map[string]int{}
		best, bestN := "", 0
		for i := 0; i < ringProbes; i++ {
			a := 2 * math.Pi * float64(i) / ringProbes
			pLat := lat + r*math.Sin(a)
			pLon := lon + r*math.Cos(a)
			if pLat > 90 || pLat < -90 {
				continue
			}
			tz := latlong.LookupZoneName(pLat, wrapLon(pLon))
			if tz == "" {
				continue
			}
			votes[tz]++
			if votes[tz] > bestN {
				best, bestN = tz, votes[tz]
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}

func wrapLon(lon float64) float64 {
	for lon > 180 {
		lon -= 360
	}
	for lon < -180 {
		lon += 360
	}
	return lon
}
