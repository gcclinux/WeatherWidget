package weather

import (
	"context"
	"log"
	"sync"

	"weatherwidget/internal/config"
)

// WeatherResult holds the outcome of a weather fetch for a single city.
type WeatherResult struct {
	Data     *WeatherData
	Error    error
	IsStale  bool // true after 3 consecutive failures
	HasError bool // true after any failure (using cached data)
}

// WeatherService orchestrates weather data fetching across cities,
// caching last successful results and tracking per-city failure counts.
type WeatherService struct {
	mu       sync.Mutex
	provider WeatherProvider
	cache    map[string]*WeatherData // keyed by city name
	failures map[string]int          // consecutive failure count per city
}

// NewWeatherService creates a WeatherService with the given provider.
func NewWeatherService(provider WeatherProvider) *WeatherService {
	return &WeatherService{
		provider: provider,
		cache:    make(map[string]*WeatherData),
		failures: make(map[string]int),
	}
}

// FetchAll fetches weather data for all cities from the active provider.
// On success it caches the result and resets the failure counter.
// On failure it increments the failure counter and returns cached data
// with HasError=true. After 3 consecutive failures IsStale is also set.
func (ws *WeatherService) FetchAll(ctx context.Context, cities []config.CityConfig) []WeatherResult {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	results := make([]WeatherResult, len(cities))

	for i, city := range cities {
		data, err := ws.provider.FetchWeather(ctx, city)
		if err != nil {
			ws.failures[city.Name]++
			log.Printf("WeatherService: fetch failed for %s: %v", city.Name, err)
			cached := ws.cache[city.Name]
			results[i] = WeatherResult{
				Data:     cached,
				Error:    err,
				HasError: true,
				IsStale:  ws.failures[city.Name] >= 3,
			}
		} else {
			ws.failures[city.Name] = 0
			ws.cache[city.Name] = data
			results[i] = WeatherResult{
				Data: data,
			}
		}
	}

	return results
}

// SwitchProvider replaces the active provider and resets all failure counters.
// The cache is preserved so stale data can still be served during the transition.
func (ws *WeatherService) SwitchProvider(provider WeatherProvider) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.provider = provider
	ws.failures = make(map[string]int)
}
