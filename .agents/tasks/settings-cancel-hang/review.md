# Review: Settings Cancel hang (Windows)
Verdict: APPROVED

Scope: only the `goCloseSettings` change (and import reorder) in `internal/ui-windows/settings.go`. Other uncommitted work was ignored.

Evidence
- go-webview2 `Terminate()` is only `PostQuitMessage`; `Run()` returns on WM_QUIT with the HWND still alive. `Destroy()` only posts WM_CLOSE. The old Cancel path (Terminate, then the deferred `wv.Destroy()`) therefore posted WM_CLOSE to a window whose thread had stopped pumping. That leaves a live window with no message loop, which is the "Not Responding" symptom. The root cause is plausible and matches the library source I read.
- The fix (`wv.Destroy()` in `goCloseSettings`) posts WM_CLOSE while the loop runs. wndproc handles WM_CLOSE with DestroyWindow, then WM_DESTROY calls Terminate, so `Run()` exits after the window is gone. `PostMessageW` is thread-safe, so it is safe wherever the binding runs.
- The deferred cleanup still reverts the preview (`clearSettingsPreview`), restores position when unsaved, and clears `activeWV`. The late `wv.Destroy()` targets a destroyed HWND and is harmless. The X button already used the WM_CLOSE path, so it is consistent. No Esc handler exists, so there is nothing to fix there. Save is untouched. gtk and darwin are unchanged, which is appropriate.
- The import reorder is gofmt-only.
- Recorded verification: gofmt, build, vet and ui-windows tests pass. The notes state that the runtime hang was not reproduced. The cause is statically established from library source, so I accept that.

No blockers.
