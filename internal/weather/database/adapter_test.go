package database

import (
	"context"
	"errors"
	"testing"
	"weatherwidget/internal/config"
)

// mockRow implements the Row interface for testing.
type mockRow struct {
	temperature int
	description string
	iconCode    string
	scanErr     error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if len(dest) >= 3 {
		*dest[0].(*int) = r.temperature
		*dest[1].(*string) = r.description
		*dest[2].(*string) = r.iconCode
	}
	return nil
}

// mockPool implements the Pool interface for testing.
type mockPool struct {
	row       Row
	pingErr   error
	closed    bool
	lastQuery string
	lastArgs  []any
}

func (p *mockPool) QueryRow(_ context.Context, sql string, args ...any) Row {
	p.lastQuery = sql
	p.lastArgs = args
	return p.row
}

func (p *mockPool) Ping(_ context.Context) error {
	return p.pingErr
}

func (p *mockPool) Close() {
	p.closed = true
}

func TestFetchWeather_Success(t *testing.T) {
	pool := &mockPool{
		row: &mockRow{
			temperature: 25,
			description: "clear sky",
			iconCode:    "clear",
		},
	}
	adapter := NewDatabaseAdapterWithPool(pool, "SELECT temp, desc, icon FROM weather WHERE city = $1")

	city := config.CityConfig{
		Name:     "Holambra",
		Region:   "SP",
		Timezone: "America/Sao_Paulo",
	}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.CityName != "Holambra" {
		t.Errorf("CityName = %q, want %q", data.CityName, "Holambra")
	}
	if data.Region != "SP" {
		t.Errorf("Region = %q, want %q", data.Region, "SP")
	}
	if data.Temperature != 25 {
		t.Errorf("Temperature = %d, want 25", data.Temperature)
	}
	if data.Description != "clear sky" {
		t.Errorf("Description = %q, want %q", data.Description, "clear sky")
	}
	if data.IconCode != "clear" {
		t.Errorf("IconCode = %q, want %q", data.IconCode, "clear")
	}
	if data.LocalTime.IsZero() {
		t.Error("expected non-zero LocalTime")
	}
	if data.FetchedAt.IsZero() {
		t.Error("expected non-zero FetchedAt")
	}

	// Verify the query and args were passed correctly
	if pool.lastQuery != "SELECT temp, desc, icon FROM weather WHERE city = $1" {
		t.Errorf("query = %q, want expected SQL", pool.lastQuery)
	}
	if len(pool.lastArgs) != 1 || pool.lastArgs[0] != "Holambra" {
		t.Errorf("args = %v, want [Holambra]", pool.lastArgs)
	}
}

func TestFetchWeather_ScanError(t *testing.T) {
	pool := &mockPool{
		row: &mockRow{scanErr: errors.New("no rows in result set")},
	}
	adapter := NewDatabaseAdapterWithPool(pool, "SELECT temp, desc, icon FROM weather WHERE city = $1")

	city := config.CityConfig{
		Name:     "UnknownCity",
		Region:   "XX",
		Timezone: "UTC",
	}

	_, err := adapter.FetchWeather(context.Background(), city)
	if err == nil {
		t.Fatal("expected error for scan failure")
	}
}

func TestFetchWeather_InvalidTimezone(t *testing.T) {
	pool := &mockPool{
		row: &mockRow{
			temperature: 10,
			description: "cloudy",
			iconCode:    "cloudy",
		},
	}
	adapter := NewDatabaseAdapterWithPool(pool, "SELECT temp, desc, icon FROM weather WHERE city = $1")

	city := config.CityConfig{
		Name:     "TestCity",
		Region:   "XX",
		Timezone: "Invalid/Timezone",
	}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to UTC for invalid timezone
	if data.LocalTime.Location().String() != "UTC" {
		t.Errorf("expected UTC fallback for invalid timezone, got %s", data.LocalTime.Location())
	}
}

func TestTestConnection_Success(t *testing.T) {
	pool := &mockPool{pingErr: nil}
	adapter := NewDatabaseAdapterWithPool(pool, "SELECT 1")

	err := adapter.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestConnection_Failure(t *testing.T) {
	pool := &mockPool{pingErr: errors.New("connection refused")}
	adapter := NewDatabaseAdapterWithPool(pool, "SELECT 1")

	err := adapter.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error for ping failure")
	}
}

func TestClose(t *testing.T) {
	pool := &mockPool{}
	adapter := NewDatabaseAdapterWithPool(pool, "SELECT 1")

	adapter.Close()

	if !pool.closed {
		t.Error("expected pool.Close() to be called")
	}
}
