//go:build linux

package uitk

import (
	"os"
	"strings"
)

// isDeepin reports whether the session is Deepin / DDE. Only used to scope
// Deepin-specific window fixes so other desktops are unaffected.
func isDeepin() bool {
	for _, v := range []string{os.Getenv("XDG_CURRENT_DESKTOP"), os.Getenv("XDG_SESSION_DESKTOP"), os.Getenv("DESKTOP_SESSION")} {
		l := strings.ToLower(v)
		if strings.Contains(l, "deepin") || strings.Contains(l, "dde") {
			return true
		}
	}
	return false
}
