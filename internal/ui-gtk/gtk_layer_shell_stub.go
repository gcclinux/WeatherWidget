//go:build linux && !layershell

// This is the default build (no "layershell" build tag). It provides no-op
// stubs so the package compiles and links WITHOUT libgtk-layer-shell. All
// current packages (bin, deb, rpm, appimage, snap, flatpak) use this variant,
// so their build scripts and dependencies are unchanged.
//
// To enable native Wayland layer-shell positioning, build with:
//
//	go build -tags layershell ./cmd/weatherwidget-gtk/
//
// which compiles gtk_layer_shell.go instead of this file.

package uitk

import "github.com/gotk3/gotk3/gtk"

// layerShellBuilt is false in the default build: the layer-shell library is not
// linked, so the layer-shell positioning path is never selected.
const layerShellBuilt = false

// layerShellSupported always reports false without the library.
func layerShellSupported() bool { return false }

// layerShellInit is a no-op without the library.
func layerShellInit(_ *gtk.Window) {}

// layerShellPosition is a no-op without the library.
func layerShellPosition(_ *gtk.Window, _, _ bool, _, _ int) {}
