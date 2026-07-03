//go:build !windows && !linux && !darwin

package ui

// isAutoStartEnabled returns false on unsupported platforms.
func isAutoStartEnabled() bool { return false }

// setAutoStartEnabled is a no-op on unsupported platforms.
func setAutoStartEnabled(_ bool) error { return nil }
