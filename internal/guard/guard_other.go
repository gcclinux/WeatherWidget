//go:build !windows

package guard

// SingleInstanceGuard is a no-op stub on non-Windows platforms.
type SingleInstanceGuard struct{}

// NewSingleInstanceGuard returns a no-op guard on non-Windows platforms.
func NewSingleInstanceGuard(name string) (*SingleInstanceGuard, error) {
	return &SingleInstanceGuard{}, nil
}

// Release is a no-op on non-Windows platforms.
func (g *SingleInstanceGuard) Release() error {
	return nil
}
