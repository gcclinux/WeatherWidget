package config

import (
	"testing"

	"pgregory.net/rapid"
)

// **Feature: windows-weather-widget, Property 4: City list add preserves existing and appends**
// **Validates: Requirements 3a.2**

func TestProperty4_CityListAddPreservesExistingAndAppends(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a list of 1-2 cities
		count := rapid.IntRange(1, 2).Draw(rt, "cityCount")
		cities := make([]CityConfig, count)
		for i := range cities {
			cities[i] = genCityConfig(rt, "existing"+string(rune('0'+i)))
		}

		// Generate a new city to add
		newCity := genCityConfig(rt, "newCity")

		// Call AddCity
		result, err := AddCity(cities, newCity)
		if err != nil {
			rt.Fatalf("AddCity() unexpected error: %v", err)
		}

		// Assert length == original + 1
		if len(result) != len(cities)+1 {
			rt.Fatalf("expected length %d, got %d", len(cities)+1, len(result))
		}

		// Assert first N elements are identical to original
		for i, c := range cities {
			if result[i] != c {
				rt.Fatalf("element %d changed: expected %+v, got %+v", i, c, result[i])
			}
		}

		// Assert last element is the new city
		if result[len(result)-1] != newCity {
			rt.Fatalf("last element: expected %+v, got %+v", newCity, result[len(result)-1])
		}
	})
}

// **Feature: windows-weather-widget, Property 5: City list remove decreases length and excludes target**
// **Validates: Requirements 3a.4**

func TestProperty5_CityListRemoveDecreasesLengthAndExcludesTarget(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a list of 2-3 cities
		count := rapid.IntRange(2, 3).Draw(rt, "cityCount")
		cities := make([]CityConfig, count)
		for i := range cities {
			cities[i] = genCityConfig(rt, "city"+string(rune('0'+i)))
		}

		// Generate a random valid index
		index := rapid.IntRange(0, count-1).Draw(rt, "removeIndex")
		removed := cities[index]

		// Call RemoveCity
		result, err := RemoveCity(cities, index)
		if err != nil {
			rt.Fatalf("RemoveCity() unexpected error: %v", err)
		}

		// Assert length == original - 1
		if len(result) != len(cities)-1 {
			rt.Fatalf("expected length %d, got %d", len(cities)-1, len(result))
		}

		// Assert the removed city is not in the result
		for i, c := range result {
			if c == removed {
				rt.Fatalf("removed city %+v still present at index %d", removed, i)
			}
		}
	})
}

// **Feature: windows-weather-widget, Property 6: City list reorder preserves set**
// **Validates: Requirements 3a.5**

func TestProperty6_CityListReorderPreservesSet(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a list of 1-3 cities
		count := rapid.IntRange(1, 3).Draw(rt, "cityCount")
		cities := make([]CityConfig, count)
		for i := range cities {
			cities[i] = genCityConfig(rt, "city"+string(rune('0'+i)))
		}

		// Generate a random permutation using Fisher-Yates
		perm := make([]int, count)
		for i := range perm {
			perm[i] = i
		}
		// Shuffle: for each position from last to first, swap with a random earlier position
		for i := count - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(rt, "swap"+string(rune('0'+i)))
			perm[i], perm[j] = perm[j], perm[i]
		}

		// Call ReorderCities
		result, err := ReorderCities(cities, perm)
		if err != nil {
			rt.Fatalf("ReorderCities() unexpected error: %v", err)
		}

		// Assert same length
		if len(result) != len(cities) {
			rt.Fatalf("expected length %d, got %d", len(cities), len(result))
		}

		// Assert the result contains exactly the same multiset of cities
		// by checking that result[i] == cities[perm[i]] for all i
		for i, idx := range perm {
			if result[i] != cities[idx] {
				rt.Fatalf("position %d: expected %+v (from perm[%d]=%d), got %+v",
					i, cities[idx], i, idx, result[i])
			}
		}

		// Also verify same multiset by counting
		origCounts := make(map[CityConfig]int)
		resultCounts := make(map[CityConfig]int)
		for _, c := range cities {
			origCounts[c]++
		}
		for _, c := range result {
			resultCounts[c]++
		}
		for k, v := range origCounts {
			if resultCounts[k] != v {
				rt.Fatalf("multiset mismatch for %+v: original has %d, result has %d", k, v, resultCounts[k])
			}
		}
	})
}
