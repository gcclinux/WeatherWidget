//go:build windows

package uiwindows

import (
	"strings"
	"testing"
)

func TestBuildBundledHTML(t *testing.T) {
	html := buildBundledHTML()
	if !strings.Contains(html, "<style>") {
		t.Error("expected inlined <style> in bundled HTML")
	}
	if !strings.Contains(html, "<script>") {
		t.Error("expected inlined <script> in bundled HTML")
	}
	if !strings.Contains(html, "WeatherWidget Settings") {
		t.Error("expected title in bundled HTML")
	}
}
