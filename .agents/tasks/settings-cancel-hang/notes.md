# Settings Cancel hang (Windows)

## Root cause
`internal/ui-windows/settings.go`: the `goCloseSettings` binding (called only by the Cancel button) ran `wv.Dispatch(wv.Terminate)`. In go-webview2, `Terminate` only calls `PostQuitMessage`, so `Run()` returns while the HWND still exists. The deferred cleanup then called `wv.Destroy()`, which only *posts* `WM_CLOSE` to that window. The settings goroutine had already left its message loop, so nothing handled `WM_CLOSE`. The window stayed alive on a thread with no message pump, so it showed "(Not Responding)" and could not be closed.

This was not a lock, preview-revert or wrong-thread problem. I read the `redraw`, `redrawMu`, `m.mu` and `widget.mu` lock ordering and found no cycle. The preview revert (`clearSettingsPreview`) runs correctly in the same deferred block.

Save is unaffected because it never closes the window. The title-bar X button works because it sends `WM_CLOSE`, which leads to `DestroyWindow`, then `WM_DESTROY`, then `Terminate`, so the window is destroyed before `Run()` exits. Cancel was the only path that quit the loop first.

## Fix
`goCloseSettings` now calls `wv.Destroy()`, which posts `WM_CLOSE` while the loop is still running. The window is destroyed by its own pump, and `WM_DESTROY` then ends `Run()`. The existing deferred cleanup is unchanged: it restores the widget position if nothing was saved, clears the preview and redraws with the saved config, and clears `activeWV`. The later `wv.Destroy()` in that defer posts to an already-destroyed window, which does nothing.

A gofmt-style reorder of the `go-webview2` import in `settings.go` is also in the diff. It came from tooling, is harmless, and leaves the file gofmt-clean.

Esc: the Windows settings UI has no Esc handler, so there is nothing to fix there. I did not add one.

## Other platforms
`ui-gtk` and `ui-darwin` use native dialogs and panels (GTK response handlers, `NSButton onCancel:`), not WebView2, so this cause does not apply. I changed nothing there.

## Verification
- `gofmt -l internal/ui-windows/settings.go`: clean. Other files in that folder (`draw_test.go`, `enhanced_draw_test.go`, `tray.go`) were already listed before my change and I did not touch them.
- `go build ./...`: pass.
- `go vet ./internal/ui-windows/`: pass.
- `go test -count=1 ./internal/ui-windows/`: pass.
- Not verified at runtime: I could not run the WebView2 settings window interactively, so I did not reproduce the hang or confirm the fix with the real window. The cause comes from reading the go-webview2 `Run`, `Terminate`, `Destroy` and `wndproc` code. No regression test was added because the behaviour depends on a live WebView2 window and message loop.
- Nothing committed, stashed or branched.
