package config

import (
	"errors"
	"fmt"
)

// AddCity appends a city to the list. Returns an error if the list already has 5 cities.
func AddCity(cities []CityConfig, city CityConfig) ([]CityConfig, error) {
	if len(cities) >= 5 {
		return cities, errors.New("maximum of 5 cities reached")
	}
	return append(cities, city), nil
}

// RemoveCity removes the city at the given index. Returns an error if the list has only 1 city
// or if the index is out of bounds.
func RemoveCity(cities []CityConfig, index int) ([]CityConfig, error) {
	if len(cities) <= 1 {
		return cities, errors.New("cannot remove the last city")
	}
	if index < 0 || index >= len(cities) {
		return cities, fmt.Errorf("index %d out of bounds for city list of length %d", index, len(cities))
	}
	result := make([]CityConfig, 0, len(cities)-1)
	result = append(result, cities[:index]...)
	result = append(result, cities[index+1:]...)
	return result, nil
}

// ReorderCities returns a new slice with cities arranged according to newOrder.
// newOrder must be a valid permutation of indices [0, len(cities)-1].
func ReorderCities(cities []CityConfig, newOrder []int) ([]CityConfig, error) {
	if len(newOrder) != len(cities) {
		return cities, fmt.Errorf("newOrder length %d does not match cities length %d", len(newOrder), len(cities))
	}

	seen := make(map[int]bool, len(cities))
	for _, idx := range newOrder {
		if idx < 0 || idx >= len(cities) {
			return cities, fmt.Errorf("index %d out of bounds for city list of length %d", idx, len(cities))
		}
		if seen[idx] {
			return cities, fmt.Errorf("duplicate index %d in newOrder", idx)
		}
		seen[idx] = true
	}

	result := make([]CityConfig, len(cities))
	for i, idx := range newOrder {
		result[i] = cities[idx]
	}
	return result, nil
}
