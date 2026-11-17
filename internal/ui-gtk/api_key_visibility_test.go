//go:build linux

package uitk

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/gotk3/gotk3/gtk"

	"weatherwidget/internal/config"
)

func TestAPIKeyVisibilityX11(t *testing.T) {
	if os.Getenv("WEATHERWIDGET_GUI_TEST") != "1" || !x11DisplayAvailable() {
		t.Skip("requires an explicitly enabled X11 display")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	forceX11()
	gtk.Init(nil)

	const key = "12345678-1234-1234-1234-123456789abc"
	cfg := config.DefaultConfig()
	cfg.APIConfig = &config.APIConfig{Provider: "easyweatherwidget", APIKey: key}
	tab, _, _ := buildProviderTab(&manager{cfg: cfg}, nil, nil, nil)
	defer tab.Destroy()

	child := func(container *gtk.Container, index uint) interface{} {
		t.Helper()
		widget, ok := container.GetChildren().NthData(index).(*gtk.Widget)
		if !ok {
			t.Fatalf("child %d is not a GTK widget", index)
		}
		result, err := widget.Cast()
		if err != nil {
			t.Fatalf("cast child %d: %v", index, err)
		}
		return result
	}
	row := child(&tab.Container, 3).(*gtk.Box)
	entry := child(&row.Container, 0).(*gtk.Entry)
	toggle := child(&row.Container, 1).(*gtk.Button)
	frame := child(&tab.Container, 7).(*gtk.Frame)
	box := child(&frame.Container, 0).(*gtk.Box)
	label := child(&box.Container, 0).(*gtk.Label)

	check := func(visible bool, expected string) {
		t.Helper()
		if entry.GetVisibility() != visible {
			t.Fatalf("entry visibility = %v, want %v", entry.GetVisibility(), visible)
		}
		text, err := label.GetText()
		if err != nil || text != expected {
			t.Fatalf("activation key = %q, want %q (error: %v)", text, expected, err)
		}
	}
	masked := strings.Repeat("•", 8) + "-" + strings.Repeat("•", 4) + "-" + strings.Repeat("•", 4) + "-" + strings.Repeat("•", 4) + "-" + strings.Repeat("•", 12)
	check(false, masked)
	toggle.Clicked()
	check(true, key)
	toggle.Clicked()
	check(false, masked)
	entry.SetText("")
	check(false, "")
	entry.SetText("new-key")
	check(false, "•••-•••")
	toggle.Clicked()
	check(true, "new-key")
}
