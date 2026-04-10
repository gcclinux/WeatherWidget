package config

import (
	"testing"
)

func city(name, region string) CityConfig {
	return CityConfig{Name: name, Region: region, Timezone: "UTC"}
}

// --- AddCity tests ---

func TestAddCity_AppendsToList(t *testing.T) {
	cities := []CityConfig{city("A", "R1")}
	result, err := AddCity(cities, city("B", "R2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected length 2, got %d", len(result))
	}
	if result[0].Name != "A" {
		t.Errorf("expected first city A, got %s", result[0].Name)
	}
	if result[1].Name != "B" {
		t.Errorf("expected second city B, got %s", result[1].Name)
	}
}

func TestAddCity_RejectsWhenFull(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2"), city("C", "R3")}
	_, err := AddCity(cities, city("D", "R4"))
	if err == nil {
		t.Fatal("expected error when adding to full list")
	}
	if err.Error() != "maximum of 3 cities reached" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAddCity_AllowsUpToThree(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2")}
	result, err := AddCity(cities, city("C", "R3"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected length 3, got %d", len(result))
	}
}

// --- RemoveCity tests ---

func TestRemoveCity_RemovesAtIndex(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2"), city("C", "R3")}
	result, err := RemoveCity(cities, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected length 2, got %d", len(result))
	}
	if result[0].Name != "A" || result[1].Name != "C" {
		t.Errorf("expected [A, C], got [%s, %s]", result[0].Name, result[1].Name)
	}
}

func TestRemoveCity_RejectsLastCity(t *testing.T) {
	cities := []CityConfig{city("A", "R1")}
	_, err := RemoveCity(cities, 0)
	if err == nil {
		t.Fatal("expected error when removing last city")
	}
	if err.Error() != "cannot remove the last city" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRemoveCity_RejectsOutOfBounds(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2")}

	_, err := RemoveCity(cities, -1)
	if err == nil {
		t.Error("expected error for negative index")
	}

	_, err = RemoveCity(cities, 2)
	if err == nil {
		t.Error("expected error for index equal to length")
	}
}

func TestRemoveCity_FirstElement(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2")}
	result, err := RemoveCity(cities, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].Name != "B" {
		t.Errorf("expected [B], got %v", result)
	}
}

func TestRemoveCity_LastElement(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2")}
	result, err := RemoveCity(cities, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].Name != "A" {
		t.Errorf("expected [A], got %v", result)
	}
}

// --- ReorderCities tests ---

func TestReorderCities_ValidPermutation(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2"), city("C", "R3")}
	result, err := ReorderCities(cities, []int{2, 0, 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0].Name != "C" || result[1].Name != "A" || result[2].Name != "B" {
		t.Errorf("expected [C, A, B], got [%s, %s, %s]", result[0].Name, result[1].Name, result[2].Name)
	}
}

func TestReorderCities_IdentityPermutation(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2")}
	result, err := ReorderCities(cities, []int{0, 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0].Name != "A" || result[1].Name != "B" {
		t.Errorf("expected [A, B], got [%s, %s]", result[0].Name, result[1].Name)
	}
}

func TestReorderCities_RejectsWrongLength(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2")}
	_, err := ReorderCities(cities, []int{0})
	if err == nil {
		t.Error("expected error for wrong length newOrder")
	}
}

func TestReorderCities_RejectsDuplicateIndices(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2")}
	_, err := ReorderCities(cities, []int{0, 0})
	if err == nil {
		t.Error("expected error for duplicate indices")
	}
}

func TestReorderCities_RejectsOutOfBoundsIndex(t *testing.T) {
	cities := []CityConfig{city("A", "R1"), city("B", "R2")}
	_, err := ReorderCities(cities, []int{0, 5})
	if err == nil {
		t.Error("expected error for out-of-bounds index")
	}
}

func TestReorderCities_SingleCity(t *testing.T) {
	cities := []CityConfig{city("A", "R1")}
	result, err := ReorderCities(cities, []int{0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].Name != "A" {
		t.Errorf("expected [A], got %v", result)
	}
}
