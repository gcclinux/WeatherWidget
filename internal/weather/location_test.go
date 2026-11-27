package weather

import (
	"sync"
	"testing"
	"time"
)

func TestLoadLocationCached(t *testing.T) {
	a, err := LoadLocationCached("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := LoadLocationCached("Asia/Tokyo")
	if a != b {
		t.Fatal("expected the same *time.Location from the cache")
	}
	if a.String() != "Asia/Tokyo" {
		t.Fatalf("location = %s", a)
	}
	if _, err := LoadLocationCached("Not/AZone"); err == nil {
		t.Fatal("invalid zone must keep returning an error")
	}
	if _, err := LoadLocationCached("Not/AZone"); err == nil {
		t.Fatal("cached invalid zone must keep returning an error")
	}
	if loc, err := LoadLocationCached(""); err != nil || loc != time.UTC {
		t.Fatalf("empty name = %v, %v; want UTC", loc, err)
	}
}

func TestFormatInvalidTimezoneFallsBackToUTC(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if got := FormatTime(at, "Not/AZone", nil); got != "03:04:05" {
			t.Fatalf("FormatTime = %q", got)
		}
		if got := FormatDate(at, "Not/AZone", nil); got != "02/01/2026" {
			t.Fatalf("FormatDate = %q", got)
		}
		if got := FormatDateTime(at, "Not/AZone", nil); got != "02/01/2026 - 03:04:05" {
			t.Fatalf("FormatDateTime = %q", got)
		}
		if got := FormatTime(at, "Asia/Tokyo", nil); got != "12:04:05" {
			t.Fatalf("FormatTime Tokyo = %q", got)
		}
	}
}

func TestLoadLocationCachedConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			FormatTime(time.Now(), "America/New_York", nil)
			FormatDate(time.Now(), "Bad/Zone", nil)
		}()
	}
	wg.Wait()
}
