package weather

import (
	"context"
	"errors"
	"testing"
	"time"

	"weatherwidget/internal/config"
)

// mockProvider is a test double for WeatherProvider.
type mockProvider struct {
	results map[string]*WeatherData
	errs    map[string]error
}

func (m *mockProvider) FetchWeather(_ context.Context, city config.CityConfig) (*WeatherData, error) {
	if err, ok := m.errs[city.Name]; ok && err != nil {
		return nil, err
	}
	if data, ok := m.results[city.Name]; ok {
		return data, nil
	}
	return nil, errors.New("no data configured for city")
}

func (m *mockProvider) TestConnection(_ context.Context) error {
	return nil
}

func newMockData(cityName string) *WeatherData {
	return &WeatherData{
		CityName:    cityName,
		Region:      "TST",
		Temperature: 20,
		Description: "Clear",
		IconCode:    IconClear,
		LocalTime:   time.Now(),
		FetchedAt:   time.Now(),
	}
}

func TestFetchAll_Success(t *testing.T) {
	provider := &mockProvider{
		results: map[string]*WeatherData{
			"CityA": newMockData("CityA"),
		},
	}
	ws := NewWeatherService(provider)
	cities := []config.CityConfig{{Name: "CityA", Region: "R", Timezone: "UTC"}}

	results := ws.FetchAll(context.Background(), cities)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Data == nil {
		t.Fatal("expected non-nil Data on success")
	}
	if r.HasError {
		t.Error("expected HasError=false on success")
	}
	if r.IsStale {
		t.Error("expected IsStale=false on success")
	}
	if r.Error != nil {
		t.Errorf("expected nil Error, got %v", r.Error)
	}
}

func TestFetchAll_FailureReturnsCachedData(t *testing.T) {
	provider := &mockProvider{
		results: map[string]*WeatherData{
			"CityA": newMockData("CityA"),
		},
	}
	ws := NewWeatherService(provider)
	cities := []config.CityConfig{{Name: "CityA", Region: "R", Timezone: "UTC"}}

	// First fetch succeeds, populating cache
	ws.FetchAll(context.Background(), cities)

	// Switch to failing provider
	failProvider := &mockProvider{
		errs: map[string]error{"CityA": errors.New("network error")},
	}
	ws.SwitchProvider(failProvider)

	results := ws.FetchAll(context.Background(), cities)
	r := results[0]

	if !r.HasError {
		t.Error("expected HasError=true after failure")
	}
	if r.Data == nil {
		t.Fatal("expected cached Data after failure")
	}
	if r.Data.CityName != "CityA" {
		t.Errorf("expected cached city CityA, got %s", r.Data.CityName)
	}
	if r.Error == nil {
		t.Error("expected non-nil Error")
	}
}

func TestFetchAll_StaleAfterThreeFailures(t *testing.T) {
	failProvider := &mockProvider{
		errs: map[string]error{"CityA": errors.New("fail")},
	}
	ws := NewWeatherService(failProvider)
	cities := []config.CityConfig{{Name: "CityA", Region: "R", Timezone: "UTC"}}

	// First two failures: HasError but not IsStale
	for i := 0; i < 2; i++ {
		results := ws.FetchAll(context.Background(), cities)
		if results[0].IsStale {
			t.Errorf("iteration %d: expected IsStale=false before 3 failures", i)
		}
		if !results[0].HasError {
			t.Errorf("iteration %d: expected HasError=true", i)
		}
	}

	// Third failure: IsStale should be true
	results := ws.FetchAll(context.Background(), cities)
	if !results[0].IsStale {
		t.Error("expected IsStale=true after 3 consecutive failures")
	}
	if !results[0].HasError {
		t.Error("expected HasError=true after 3 consecutive failures")
	}
}

func TestFetchAll_SuccessResetsFailureCounter(t *testing.T) {
	fetchErr := errors.New("fail")
	provider := &mockProvider{
		errs: map[string]error{"CityA": fetchErr},
	}
	ws := NewWeatherService(provider)
	cities := []config.CityConfig{{Name: "CityA", Region: "R", Timezone: "UTC"}}

	// Two failures
	ws.FetchAll(context.Background(), cities)
	ws.FetchAll(context.Background(), cities)

	// Now succeed
	successProvider := &mockProvider{
		results: map[string]*WeatherData{"CityA": newMockData("CityA")},
	}
	ws.SwitchProvider(successProvider)

	results := ws.FetchAll(context.Background(), cities)
	if results[0].HasError {
		t.Error("expected HasError=false after success")
	}
	if results[0].IsStale {
		t.Error("expected IsStale=false after success")
	}

	// Fail again twice — should not be stale yet (counter was reset)
	failProvider2 := &mockProvider{
		errs: map[string]error{"CityA": fetchErr},
	}
	ws.SwitchProvider(failProvider2)
	ws.FetchAll(context.Background(), cities)
	results = ws.FetchAll(context.Background(), cities)
	if results[0].IsStale {
		t.Error("expected IsStale=false, counter should have been reset by success")
	}
}

func TestSwitchProvider_ResetsFailureCounters(t *testing.T) {
	failProvider := &mockProvider{
		errs: map[string]error{"CityA": errors.New("fail")},
	}
	ws := NewWeatherService(failProvider)
	cities := []config.CityConfig{{Name: "CityA", Region: "R", Timezone: "UTC"}}

	// Accumulate 2 failures
	ws.FetchAll(context.Background(), cities)
	ws.FetchAll(context.Background(), cities)

	// Switch provider (still failing) — counters reset
	failProvider2 := &mockProvider{
		errs: map[string]error{"CityA": errors.New("fail again")},
	}
	ws.SwitchProvider(failProvider2)

	// One more failure — should be count=1, not stale
	results := ws.FetchAll(context.Background(), cities)
	if results[0].IsStale {
		t.Error("expected IsStale=false after provider switch reset counters")
	}
}

func TestSwitchProvider_PreservesCache(t *testing.T) {
	provider := &mockProvider{
		results: map[string]*WeatherData{"CityA": newMockData("CityA")},
	}
	ws := NewWeatherService(provider)
	cities := []config.CityConfig{{Name: "CityA", Region: "R", Timezone: "UTC"}}

	// Populate cache
	ws.FetchAll(context.Background(), cities)

	// Switch to failing provider
	failProvider := &mockProvider{
		errs: map[string]error{"CityA": errors.New("fail")},
	}
	ws.SwitchProvider(failProvider)

	// Should still return cached data
	results := ws.FetchAll(context.Background(), cities)
	if results[0].Data == nil {
		t.Fatal("expected cached data to be preserved after provider switch")
	}
	if results[0].Data.CityName != "CityA" {
		t.Errorf("expected cached CityA, got %s", results[0].Data.CityName)
	}
}

func TestFetchAll_MultipleCities(t *testing.T) {
	provider := &mockProvider{
		results: map[string]*WeatherData{
			"CityA": newMockData("CityA"),
			"CityB": newMockData("CityB"),
		},
		errs: map[string]error{
			"CityC": errors.New("fail"),
		},
	}
	ws := NewWeatherService(provider)
	cities := []config.CityConfig{
		{Name: "CityA", Region: "R", Timezone: "UTC"},
		{Name: "CityB", Region: "R", Timezone: "UTC"},
		{Name: "CityC", Region: "R", Timezone: "UTC"},
	}

	results := ws.FetchAll(context.Background(), cities)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].HasError || results[1].HasError {
		t.Error("expected no errors for CityA and CityB")
	}
	if !results[2].HasError {
		t.Error("expected HasError for CityC")
	}
	if results[2].Data != nil {
		t.Error("expected nil Data for CityC with no cache")
	}
}

func TestFetchAll_EmptyCities(t *testing.T) {
	provider := &mockProvider{}
	ws := NewWeatherService(provider)

	results := ws.FetchAll(context.Background(), nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil cities, got %d", len(results))
	}

	results = ws.FetchAll(context.Background(), []config.CityConfig{})
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty cities, got %d", len(results))
	}
}
