//go:build !windows

package power

// ResumeNotifier returns a channel that receives a value when the system
// resumes from sleep or hibernation. On non-Windows platforms this is a
// no-op: the returned channel will never receive.
func ResumeNotifier() <-chan struct{} {
	return make(chan struct{})
}
