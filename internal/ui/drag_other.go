//go:build !windows && !linux

package ui

// enableWindowDrag is a no-op on unsupported platforms.
func enableWindowDrag(_ func()) {}
