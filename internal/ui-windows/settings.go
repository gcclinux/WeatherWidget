//go:build windows

package uiwindows

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jchv/go-webview2"
	"weatherwidget/internal/config"
	"weatherwidget/internal/weather/remoteapi"
)

// webFS embeds the HTML, CSS, and JS files for the Settings window.
//
//go:embed web/*
var webFS embed.FS

var (
	settingsMu sync.Mutex
	activeWV   webview2.WebView
)

// OpenSettings opens the Edge WebView2 settings dialog.
func (m *Manager) OpenSettings() {
	settingsMu.Lock()
	if activeWV != nil {
		settingsMu.Unlock()
		return
	}
	settingsMu.Unlock()

	go func() {
		wv := webview2.NewWithOptions(webview2.WebViewOptions{
			Debug:     false,
			DataPath:  m.appDataDir,
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title:  "WeatherWidget Settings",
				Width:  860,
				Height: 640,
			},
		})
		if wv == nil {
			log.Println("uiwindows: failed to initialize WebView2 settings window")
			return
		}

		settingsMu.Lock()
		activeWV = wv
		settingsMu.Unlock()

		defer func() {
			settingsMu.Lock()
			activeWV = nil
			settingsMu.Unlock()
			wv.Destroy()
		}()

		// Bind RPC functions for Go <-> JS IPC
		_ = wv.Bind("goGetConfig", func() (string, error) {
			m.mu.Lock()
			cfg := m.cfg
			m.mu.Unlock()
			data, err := json.Marshal(cfg)
			if err != nil {
				return "{}", nil
			}
			return string(data), nil
		})

		_ = wv.Bind("goSaveConfig", func(jsonStr string) (string, error) {
			var newCfg config.Config
			if err := json.Unmarshal([]byte(jsonStr), &newCfg); err != nil {
				return fmt.Sprintf("invalid config json: %v", err), nil
			}

			// Save to config service
			if err := m.cfgSvc.Save(&newCfg); err != nil {
				return fmt.Sprintf("failed to save config: %v", err), nil
			}

			m.mu.Lock()
			m.cfg = &newCfg
			m.mu.Unlock()

			// Update provider and scheduler interval
			if m.sched != nil {
				m.sched.SetInterval(time.Duration(newCfg.RefreshInterval) * time.Minute)
				m.sched.SetCities(newCfg.Cities)
			}

			// Update widget placement and redraw
			if m.widget != nil {
				m.widget.ApplyPlacement(newCfg.CustomX, newCfg.CustomY, newCfg.CornerPosition, newCfg.MonitorIndex)
			}
			m.manualRefresh()

			log.Println("uiwindows: settings saved successfully")
			return "", nil
		})

		_ = wv.Bind("goPreviewConfig", func(jsonStr string) {
			var previewCfg config.Config
			if err := json.Unmarshal([]byte(jsonStr), &previewCfg); err != nil {
				return
			}
			m.mu.Lock()
			m.cfg.ViewMode = previewCfg.ViewMode
			m.cfg.Opacity = previewCfg.Opacity
			m.cfg.NoBackground = previewCfg.NoBackground
			m.cfg.TemperatureUnit = previewCfg.TemperatureUnit
			m.cfg.WindSpeedUnit = previewCfg.WindSpeedUnit
			m.mu.Unlock()

			m.redraw()
		})

		_ = wv.Bind("goCloseSettings", func() {
			wv.Terminate()
		})

		_ = wv.Bind("goTestConnection", func(provider, apiKey string) (string, error) {
			adapter := remoteapi.NewRemoteAPIAdapter(provider, apiKey)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()

			if err := adapter.TestConnection(ctx); err != nil {
				return err.Error(), nil
			}
			return "ok", nil
		})

		_ = wv.Bind("goGetAutoStart", func() bool {
			return isAutoStartEnabled()
		})

		_ = wv.Bind("goSetAutoStart", func(enable bool) {
			_ = setAutoStartEnabled(enable)
		})

		// Load bundled web content
		htmlContent := buildBundledHTML()
		wv.SetHtml(htmlContent)

		wv.Run()
	}()
}

// buildBundledHTML inlines CSS and JS into index.html so it loads self-contained in WebView2.
func buildBundledHTML() string {
	indexHTML, err := webFS.ReadFile("web/index.html")
	if err != nil {
		return "<html><body>Failed to load settings HTML</body></html>"
	}
	styleCSS, _ := webFS.ReadFile("web/style.css")
	appJS, _ := webFS.ReadFile("web/app.js")

	html := string(indexHTML)
	// Replace external stylesheet link with inline <style>
	html = strings.Replace(
		html,
		`<link rel="stylesheet" href="style.css">`,
		fmt.Sprintf("<style>%s</style>", string(styleCSS)),
		1,
	)
	// Replace external script tag with inline <script>
	html = strings.Replace(
		html,
		`<script src="app.js"></script>`,
		fmt.Sprintf("<script>%s</script>", string(appJS)),
		1,
	)

	return html
}
