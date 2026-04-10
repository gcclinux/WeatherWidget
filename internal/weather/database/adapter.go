package database

import (
	"context"
	"fmt"
	"time"
	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool defines the interface for database connection pool operations.
// This allows mocking in tests without requiring a real PostgreSQL instance.
type Pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) Row
	Ping(ctx context.Context) error
	Close()
}

// Row defines the interface for scanning a single database row result.
type Row interface {
	Scan(dest ...any) error
}

// pgxPoolWrapper wraps a *pgxpool.Pool to satisfy the Pool interface.
type pgxPoolWrapper struct {
	pool *pgxpool.Pool
}

func (w *pgxPoolWrapper) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return w.pool.QueryRow(ctx, sql, args...)
}

func (w *pgxPoolWrapper) Ping(ctx context.Context) error {
	return w.pool.Ping(ctx)
}

func (w *pgxPoolWrapper) Close() {
	w.pool.Close()
}

// DatabaseAdapter implements WeatherProvider for a local PostgreSQL database.
// It uses a connection pool and a user-configured SQL query to fetch weather data.
type DatabaseAdapter struct {
	pool  Pool
	query string // User-configured SQL query; city name is passed as $1
}

// NewDatabaseAdapter creates a new DatabaseAdapter by establishing a connection pool
// to the PostgreSQL database specified by connString. The query parameter is the SQL
// query that will be executed to fetch weather data, with $1 as the city name placeholder.
// Expected result columns: temperature (int), description (string), icon_code (string).
func NewDatabaseAdapter(connString, query string) (*DatabaseAdapter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("database: failed to create connection pool: %w", err)
	}

	return &DatabaseAdapter{
		pool:  &pgxPoolWrapper{pool: pool},
		query: query,
	}, nil
}

// NewDatabaseAdapterWithPool creates a DatabaseAdapter with a provided Pool implementation.
// This is primarily useful for testing with mock pools.
func NewDatabaseAdapterWithPool(pool Pool, query string) *DatabaseAdapter {
	return &DatabaseAdapter{
		pool:  pool,
		query: query,
	}
}

// FetchWeather executes the configured SQL query with the city name as parameter
// and scans the result into a WeatherData struct.
func (d *DatabaseAdapter) FetchWeather(ctx context.Context, city config.CityConfig) (*weather.WeatherData, error) {
	var temperature int
	var description string
	var iconCode string

	row := d.pool.QueryRow(ctx, d.query, city.Name)
	if err := row.Scan(&temperature, &description, &iconCode); err != nil {
		return nil, fmt.Errorf("database: failed to fetch weather for %q: %w", city.Name, err)
	}

	loc, err := time.LoadLocation(city.Timezone)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now()
	return &weather.WeatherData{
		CityName:    city.Name,
		Region:      city.Region,
		Temperature: temperature,
		Description: description,
		IconCode:    iconCode,
		LocalTime:   now.In(loc),
		FetchedAt:   now,
	}, nil
}

// TestConnection verifies the database connection by pinging the pool.
func (d *DatabaseAdapter) TestConnection(ctx context.Context) error {
	if err := d.pool.Ping(ctx); err != nil {
		return fmt.Errorf("database: connection test failed: %w", err)
	}
	return nil
}

// Close releases the connection pool resources.
func (d *DatabaseAdapter) Close() {
	d.pool.Close()
}
