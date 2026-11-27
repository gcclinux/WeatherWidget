package weather

import (
	"sync"
	"time"
)

// locationCache memoises time.LoadLocation results (including failures) per
// timezone name. time.LoadLocation does not cache, and every call parses the
// embedded tzdata again. *time.Location values are immutable, so sharing them
// between goroutines is safe.
var locationCache sync.Map // map[string]locationEntry

type locationEntry struct {
	loc *time.Location
	err error
}

// LoadLocationCached behaves like time.LoadLocation but parses each timezone
// name only once. Errors are cached too, so invalid names stay cheap.
func LoadLocationCached(name string) (*time.Location, error) {
	if v, ok := locationCache.Load(name); ok {
		e := v.(locationEntry)
		return e.loc, e.err
	}
	loc, err := time.LoadLocation(name)
	locationCache.Store(name, locationEntry{loc: loc, err: err})
	return loc, err
}

// locationOrUTC returns the cached location for name, or UTC when it is invalid.
func locationOrUTC(name string) *time.Location {
	loc, err := LoadLocationCached(name)
	if err != nil {
		return time.UTC
	}
	return loc
}
