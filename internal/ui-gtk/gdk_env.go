//go:build linux

package uitk

// #include <stdlib.h>
// #include <stdio.h>
//
// static void force_gdk_backend_x11(void) {
//     setenv("GDK_BACKEND", "x11", 1);
//     // Debug: verify from C side
//     fprintf(stderr, "C-level GDK_BACKEND=%s\n", getenv("GDK_BACKEND"));
// }
import "C"

import "os"

// ensureGDKBackendX11 forces GDK_BACKEND=x11 in BOTH the C runtime environment
// (which GTK reads) and Go's os environment (for consistency in logging).
// Must be called before gtk.Init().
func ensureGDKBackendX11() {
	C.force_gdk_backend_x11()
	os.Setenv("GDK_BACKEND", "x11")
}
