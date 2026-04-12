package scheduler

import (
	"context"
	"sync"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

// RefreshScheduler manages periodic weather data fetching with configurable intervals.
type RefreshScheduler struct {
	mu       sync.Mutex
	interval time.Duration
	ticker   *time.Ticker
	weather  *weather.WeatherService
	cities   []config.CityConfig
	onUpdate func([]weather.WeatherResult)
	onError  func(city string, err error)
	stopCh   chan struct{}
	resetCh  chan struct{} // signals the loop to re-read the ticker after SetInterval
	running  bool
}

// NewRefreshScheduler creates a RefreshScheduler with the given interval and weather service.
func NewRefreshScheduler(interval time.Duration, ws *weather.WeatherService) *RefreshScheduler {
	return &RefreshScheduler{
		interval: interval,
		weather:  ws,
	}
}

// SetCities updates the city list used when fetching weather data.
func (r *RefreshScheduler) SetCities(cities []config.CityConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cities = cities
}

// SetOnUpdate sets the callback invoked with results after each fetch cycle.
func (r *RefreshScheduler) SetOnUpdate(fn func([]weather.WeatherResult)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onUpdate = fn
}

// SetOnError sets the callback invoked when a city fetch fails.
func (r *RefreshScheduler) SetOnError(fn func(city string, err error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onError = fn
}

// Start performs an immediate fetch and then launches a goroutine that
// fetches weather data on each ticker tick. It is a no-op if already running.
func (r *RefreshScheduler) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.stopCh = make(chan struct{})
	r.resetCh = make(chan struct{}, 1)
	r.ticker = time.NewTicker(r.interval)
	r.mu.Unlock()

	// Launch fetch and loop in background.
	go func() {
		r.fetch()
		r.loop()
	}()
}

// Stop stops the ticker and signals the goroutine to exit.
// It is safe to call multiple times.
func (r *RefreshScheduler) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}
	r.running = false
	r.ticker.Stop()
	close(r.stopCh)
}

// SetInterval resets the ticker with a new interval.
// If the scheduler is running, the old ticker is stopped and replaced.
func (r *RefreshScheduler) SetInterval(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interval = d
	if r.running && r.ticker != nil {
		r.ticker.Stop()
		r.ticker = time.NewTicker(d)
		// Signal the loop to pick up the new ticker channel.
		select {
		case r.resetCh <- struct{}{}:
		default:
		}
	}
}

// FetchNow forces an immediate fetch out-of-band.
func (r *RefreshScheduler) FetchNow() {
	go r.fetch()
}

// loop runs in a goroutine, fetching on each tick until stopCh is closed.
func (r *RefreshScheduler) loop() {
	for {
		r.mu.Lock()
		tickerC := r.ticker.C
		stopCh := r.stopCh
		resetCh := r.resetCh
		r.mu.Unlock()

		select {
		case <-stopCh:
			return
		case <-resetCh:
			// Interval was changed; re-read the ticker on next iteration.
			continue
		case <-tickerC:
			r.fetch()
		}
	}
}

// fetch calls WeatherService.FetchAll and dispatches callbacks.
func (r *RefreshScheduler) fetch() {
	r.mu.Lock()
	cities := make([]config.CityConfig, len(r.cities))
	copy(cities, r.cities)
	onUpdate := r.onUpdate
	onError := r.onError
	ws := r.weather
	r.mu.Unlock()

	if ws == nil || len(cities) == 0 {
		return
	}

	ctx := context.Background()
	results := ws.FetchAll(ctx, cities)

	if onError != nil {
		for i, res := range results {
			if res.Error != nil && i < len(cities) {
				onError(cities[i].Name, res.Error)
			}
		}
	}

	if onUpdate != nil {
		onUpdate(results)
	}
}
