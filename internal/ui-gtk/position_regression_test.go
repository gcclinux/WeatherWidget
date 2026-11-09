//go:build linux

package uitk

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"weatherwidget/internal/config"
)

func TestShouldTryX11(t *testing.T) {
	tests := []struct {
		name       string
		preset     string
		wayland    bool
		layerShell bool
		gnome      bool
		want       bool
	}{
		{"KDE explicit Wayland default build", "wayland", true, false, false, true},
		{"KDE automatic default build", "", true, false, false, true},
		{"Wayland first in backend list", "wayland,x11", true, false, false, true},
		{"KDE layer-shell build", "wayland", true, true, false, false},
		{"GNOME layer-shell build", "wayland", true, true, true, true},
		{"explicit X11", "x11", true, false, false, false},
		{"other explicit backend", "broadway", true, false, false, false},
		{"native X11", "", false, false, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldTryX11(test.preset, test.wayland, test.layerShell, test.gnome); got != test.want {
				t.Fatalf("shouldTryX11 = %t, want %t", got, test.want)
			}
		})
	}
}

func TestUnavailableX11KeepsWayland(t *testing.T) {
	t.Setenv("GDK_BACKEND", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", ":65530")
	ensureGDKBackend()
	if got := os.Getenv("GDK_BACKEND"); got != "wayland" {
		t.Fatalf("GDK_BACKEND = %q, want wayland when X11 is unavailable", got)
	}
}

func TestAvailableX11OverridesWayland(t *testing.T) {
	if os.Getenv("WEATHERWIDGET_TEST_X11_CHILD") == "1" {
		ensureGDKBackend()
		if got := os.Getenv("GDK_BACKEND"); got != "x11" {
			t.Fatalf("GDK_BACKEND = %q, want x11", got)
		}
		return
	}
	if layerShellBuilt || !x11DisplayAvailable() {
		t.Skip("default build or accessible X display unavailable")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestAvailableX11OverridesWayland$")
	cmd.Env = append(os.Environ(), "WEATHERWIDGET_TEST_X11_CHILD=1", "GDK_BACKEND=wayland", "WAYLAND_DISPLAY=wayland-0", "XDG_CURRENT_DESKTOP=KDE")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("X11 backend selection failed: %v\n%s", err, output)
	}
}

func TestSaveCustomPosition(t *testing.T) {
	service := config.NewConfigService(t.TempDir())
	manager := &manager{cfg: config.DefaultConfig(), cfgSvc: service}
	manager.saveCustomPosition(-120, 240)
	loaded, err := service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CustomX == nil || loaded.CustomY == nil || *loaded.CustomX != -120 || *loaded.CustomY != 240 {
		t.Fatalf("saved position = (%v, %v), want (-120, 240)", loaded.CustomX, loaded.CustomY)
	}
}

func TestX11WindowPositionIntegration(t *testing.T) {
	if os.Getenv("WEATHERWIDGET_GUI_TEST") != "1" || !x11DisplayAvailable() {
		t.Skip("requires an explicitly enabled X11 display")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	forceX11()
	gtk.Init(nil)
	cfg := config.DefaultConfig()
	cfg.Cities = cfg.Cities[:1]
	cfg.CornerPosition = "top-right"
	manager := &manager{cfg: cfg, cfgSvc: config.NewConfigService(t.TempDir()), opacity: 100, viewMode: cfg.ViewMode}
	if err := manager.buildWindow(); err != nil {
		t.Fatal(err)
	}
	appearance, _, _, _, _, _, applyPlacement := buildAppearanceTab(manager)
	var cornerCombo *gtk.ComboBoxText
	appearance.GetChildren().Foreach(func(item interface{}) {
		widget, ok := item.(*gtk.Widget)
		if !ok {
			return
		}
		cast, err := widget.Cast()
		if err != nil {
			return
		}
		if combo, ok := cast.(*gtk.ComboBoxText); ok && combo.GetActiveID() == "top-right" {
			cornerCombo = combo
		}
	})
	if cornerCombo == nil {
		manager.win.Destroy()
		t.Fatal("corner selector not found in the Display tab")
	}
	glib.TimeoutAdd(1500, func() bool {
		x, y := manager.win.GetPosition()
		display, _ := gdk.DisplayGetDefault()
		monitor, _ := display.GetMonitor(cfg.MonitorIndex)
		geometry := monitor.GetWorkarea()
		if y != geometry.GetY() {
			t.Errorf("window top edge = %d, want monitor top edge %d", y, geometry.GetY())
		}
		actualWidth, _ := manager.win.GetSize()
		if x+actualWidth != geometry.GetX()+geometry.GetWidth() {
			t.Errorf("window right edge = %d, want monitor right edge %d (window width %d)", x+actualWidth, geometry.GetX()+geometry.GetWidth(), actualWidth)
		}
		manager.win.Move(100, 100)
		glib.TimeoutAdd(800, func() bool {
			x, y := manager.win.GetPosition()
			if x != 100 || y != 100 {
				t.Errorf("moved position = (%d,%d), want (100,100)", x, y)
			}
			loaded, err := manager.cfgSvc.Load()
			if err != nil {
				t.Error(err)
			} else if loaded.CustomX == nil || loaded.CustomY == nil || *loaded.CustomX != 100 || *loaded.CustomY != 100 {
				t.Errorf("window-manager move was not saved: (%v,%v)", loaded.CustomX, loaded.CustomY)
			}
			cornerCombo.SetActiveID("bottom-left")
			applyPlacement(manager.cfg)
			if manager.cfg.CornerPosition != "bottom-left" || manager.cfg.CustomX != nil || manager.cfg.CustomY != nil {
				t.Error("corner selector did not clear the custom position")
			}
			if err := manager.cfgSvc.Save(manager.cfg); err != nil {
				t.Error(err)
			}
			manager.applyPosition()
			glib.TimeoutAdd(800, func() bool {
				x, y := manager.win.GetPosition()
				width, height := manager.win.GetSize()
				if x != geometry.GetX() || y+height != geometry.GetY()+geometry.GetHeight() {
					t.Errorf("bottom-left position = (%d,%d), height %d, monitor=(%d,%d %dx%d)", x, y, height, geometry.GetX(), geometry.GetY(), geometry.GetWidth(), geometry.GetHeight())
				}
				loaded, err := manager.cfgSvc.Load()
				if err != nil {
					t.Error(err)
				} else if loaded.CustomX != nil || loaded.CustomY != nil {
					t.Error("corner placement was overwritten by a custom position")
				}
				manager.win.SetSizeRequest(width+20, height+20)
				glib.TimeoutAdd(800, func() bool {
					x, y := manager.win.GetPosition()
					_, resizedHeight := manager.win.GetSize()
					if resizedHeight <= height || x != geometry.GetX() || y+resizedHeight != geometry.GetY()+geometry.GetHeight() {
						t.Errorf("resized bottom-left position = (%d,%d), height %d (was %d)", x, y, resizedHeight, height)
					}
					if manager.cfg.CustomX != nil || manager.cfg.CustomY != nil {
						t.Error("resizing a corner-anchored widget created a custom position")
					}
					manager.win.Destroy()
					appearance.Destroy()
					gtk.MainQuit()
					return false
				})
				return false
			})
			return false
		})
		return false
	})
	gtk.Main()
}
