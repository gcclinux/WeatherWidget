//go:build windows

package uiwindows

import (
	"testing"
)

func TestAutoStartQuery(t *testing.T) {
	// Querying autostart should execute safely without crashing.
	enabled := isAutoStartEnabled()
	t.Logf("isAutoStartEnabled: %v", enabled)
}
