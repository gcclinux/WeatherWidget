package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

// mockProvider implements weather.WeatherProvider for testing.
type mockProvider struct {
	mu        sync.Mutex
	callCount int
	data      map[string]*weather.WeatherData
}

func (m *mockProvider) FetchWeather(_ context.Context, city config.CityConfig) (*weather.WeatherData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if d, ok := m.data[city.Name]; ok {
		return d, nil
	}
	return &weather.WeatherData{
		CityName:    city.Name,
		Region:      city.Region,
		Temperature: 20,
		Description: "Clear",
		IconCode:    weather.IconClear,
		LocalTime:   time.Now(),
		FetchedAt:   time.Now(),
	}, nil
}

func (m *mockProvider) TestConnection(_ context.Context) error { return nil }

func (m *mockProvider) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func testCities() []config.CityConfig {
	return []config.CityConfig{
		{Name: "TestCity", Region: "TC", Timezone: "UTC"},
	}
}

func TestStart_ImmediateFetch(t *testing.T) {
	provider := &mockProvider{}
	ws := weather.NewWeatherService(provider)
	sched := NewRefreshScheduler(time.Hour, ws) // long interval so ticker won't fire
	sched.SetCities(testCities())

	var updateCount atomic.Int32
	sched.SetOnUpdate(func(results []weather.WeatherResult) {
		updateCount.Add(1)
	})

	sched.Start()
	// Give the immediate fetch a moment to complete
	time.Sleep(50 * time.Millisecond)
	sched.Stop()

	if got := updateCount.Load(); got < 1 {
		t.Errorf("expected at least 1 immediate update, got %d", got)
	}
	if got := provider.getCallCount(); got < 1 {
		t.Errorf("expected at least 1 provider call from immediate fetch, got %d", got)
	}
}

func TestStart_TickerFires(t *testing.T) {
	provider := &mockProvider{}
	ws := weather.NewWeatherService(provider)
	sched := NewRefreshScheduler(50*time.Millisecond, ws)
	sched.SetCities(testCities())

	var updateCount atomic.Int32
	sched.SetOnUpdate(func(results []weather.WeatherResult) {
		updateCount.Add(1)
	})

	sched.Start()
	// Wait for immediate fetch + at least 2 ticks
	time.Sleep(180 * time.Millisecond)
	sched.Stop()

	// Expect immediate + at least 2 ticker fires = 3+
	if got := updateCount.Load(); got < 3 {
		t.Errorf("expected at least 3 updates (immediate + 2 ticks), got %d", got)
	}
}

func TestStop_PreventsMoreFetches(t *testing.T) {
	provider := &mockProvider{}
	ws := weather.NewWeatherService(provider)
	sched := NewRefreshScheduler(50*time.Millisecond, ws)
	sched.SetCities(testCities())

	sched.Start()
	time.Sleep(30 * time.Millisecond) // let immediate fetch complete
	sched.Stop()

	countAfterStop := provider.getCallCount()
	time.Sleep(150 * time.Millisecond) // wait past several would-be ticks

	if got := provider.getCallCount(); got != countAfterStop {
		t.Errorf("expected no more fetches after Stop, had %d, now %d", countAfterStop, got)
	}
}

func TestStop_SafeToCallMultipleTimes(t *testing.T) {
	provider := &mockProvider{}
	ws := weather.NewWeatherService(provider)
	sched := NewRefreshScheduler(time.Hour, ws)
	sched.SetCities(testCities())

	sched.Start()
	time.Sleep(30 * time.Millisecond)

	// Should not panic
	sched.Stop()
	sched.Stop()
	sched.Stop()
}

func TestStop_BeforeStart_NoPanic(t *testing.T) {
	provider := &mockProvider{}
	ws := weather.NewWeatherService(provider)
	sched := NewRefreshScheduler(time.Hour, ws)

	// Should not panic when Stop is called without Start
	sched.Stop()
}

func TestSetInterval_ChangesTickRate(t *testing.T) {
	provider := &mockProvider{}
	ws := weather.NewWeatherService(provider)
	// Start with a very long interval
	sched := NewRefreshScheduler(time.Hour, ws)
	sched.SetCities(testCities())

	var updateCount atomic.Int32
	sched.SetOnUpdate(func(results []weather.WeatherResult) {
		updateCount.Add(1)
	})

	sched.Start()
	time.Sleep(30 * time.Millisecond) // let immediate fetch complete

	countBefore := updateCount.Load()

	// Shorten interval to trigger ticks
	sched.SetInterval(50 * time.Millisecond)
	time.Sleep(180 * time.Millisecond)
	sched.Stop()

	countAfter := updateCount.Load()
	tickFetches := countAfter - countBefore
	if tickFetches < 2 {
		t.Errorf("expected at least 2 fetches after SetInterval, got %d", tickFetches)
	}
}

func TestStart_NoCitiesNoFetch(t *testing.T) {
	provider := &mockProvider{}
	ws := weather.NewWeatherService(provider)
	sched := NewRefreshScheduler(50*time.Millisecond, ws)
	// No cities set

	sched.Start()
	time.Sleep(100 * time.Millisecond)
	sched.Stop()

	if got := provider.getCallCount(); got != 0 {
		t.Errorf("expected 0 provider calls with no cities, got %d", got)
	}
}

func TestStart_DoubleStartIsNoop(t *testing.T) {
	provider := &mockProvider{}
	ws := weather.NewWeatherService(provider)
	sched := NewRefreshScheduler(time.Hour, ws)
	sched.SetCities(testCities())

	sched.Start()
	time.Sleep(30 * time.Millisecond)
	sched.Start() // should be a no-op
	time.Sleep(30 * time.Millisecond)
	sched.Stop()

	// Just verify it didn't panic and only 1 immediate fetch happened
	if got := provider.getCallCount(); got != 1 {
		t.Errorf("expected exactly 1 fetch (double start should be noop), got %d", got)
	}
}

func TestOnError_CalledOnFailure(t *testing.T) {
	// Use a provider that returns errors
	failProvider := &failingProvider{err: context.DeadlineExceeded}
	ws := weather.NewWeatherService(failProvider)
	sched := NewRefreshScheduler(time.Hour, ws)
	sched.SetCities(testCities())

	var errorCities []string
	var errMu sync.Mutex
	sched.SetOnError(func(city string, err error) {
		errMu.Lock()
		errorCities = append(errorCities, city)
		errMu.Unlock()
	})

	sched.Start()
	time.Sleep(50 * time.Millisecond)
	sched.Stop()

	errMu.Lock()
	defer errMu.Unlock()
	if len(errorCities) == 0 {
		t.Error("expected onError to be called for failing city")
	}
	if errorCities[0] != "TestCity" {
		t.Errorf("expected error for TestCity, got %s", errorCities[0])
	}
}

// failingProvider always returns an error.
type failingProvider struct {
	err error
}

func (f *failingProvider) FetchWeather(_ context.Context, _ config.CityConfig) (*weather.WeatherData, error) {
	return nil, f.err
}

func (f *failingProvider) TestConnection(_ context.Context) error { return f.err }
