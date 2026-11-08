package winres

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Fatal("expected non-empty version")
	}
	parts := strings.Split(v, ".")
	if len(parts) < 3 {
		t.Fatalf("expected semantic version (e.g. 2.3.0), got %q", v)
	}
	if v != "2.3.0" {
		t.Logf("Version is %s", v)
	}
}

