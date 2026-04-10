//go:build linux

package ui

import (
	"log"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
)

// applyToolWindowStyle attempts to remove the title bar on Linux/X11
// by setting the _MOTIF_WM_HINTS property via xprop.
func applyToolWindowStyle(_ string) {
	// We need to wait a brief moment for the window to be managed by the WM
	// so that xprop can find it by its title.
	go func() {
		time.Sleep(500 * time.Millisecond)
		
		// Use xprop to remove decorations. 
		// _MOTIF_WM_HINTS: 2 = functions/decorations, 0 = no decorations
		cmd := exec.Command("xprop", "-name", widgetTitle, "-f", "_MOTIF_WM_HINTS", "32c", "-set", "_MOTIF_WM_HINTS", "0x2, 0x0, 0x0, 0x0, 0x0")
		if err := cmd.Run(); err != nil {
			log.Printf("Linux: failed to remove title bar via xprop (is x11-utils installed?): %v", err)
			return
		}
		log.Printf("Linux: successfully requested title bar removal via xprop")
	}()
}

// getScreenSize for Linux (fallback values, as Fyne usually handles this well on Linux)
func getScreenSize() (int, int) {
	return 1920, 1080
}

// moveWindow is a no-op on Linux.
func moveWindow(_ fyne.Window, _, _ int) {
}
