# WeatherWidget — Flatpak / Flathub packaging

This directory contains everything needed to build WeatherWidget as a Flatpak
and submit it to [Flathub](https://flathub.org).

**App ID:** `uk.co.easysmartapps.WeatherWidget`

| File | Purpose |
|------|---------|
| `uk.co.easysmartapps.WeatherWidget.yml` | flatpak-builder manifest |
| `uk.co.easysmartapps.WeatherWidget.metainfo.xml` | AppStream store listing (name, description, screenshots, releases) |
| `uk.co.easysmartapps.WeatherWidget.desktop` | Desktop entry |
| `generate-sources.sh` | Regenerates the offline Go module sources |
| `go.mod.yml` | Generated: Flatpak `sources` for every Go dependency (offline) |
| `modules.txt` | Generated: Go vendor manifest, pulled into `./vendor` at build time |

---

## Why the extra files? (offline builds)

Flathub builds run in a **sandbox with no network access**, and Go normally
downloads its dependencies during the build. So we pre-declare every module:

- `generate-sources.sh` runs [`dennwc/flatpak-go-mod`](https://github.com/dennwc/flatpak-go-mod),
  which produces `go.mod.yml` (hashed download directives) and `modules.txt`.
- During the build, flatpak-builder downloads each module (verifying its
  `sha256`) into `./vendor`, and Go builds with `-mod=vendor`.

The system tray dependency (`libayatana-appindicator`, not part of the GNOME
runtime) comes from the maintained [`flathub/shared-modules`](https://github.com/flathub/shared-modules)
repo, added as a git submodule (see the submission steps below).

Regenerate `go.mod.yml` + `modules.txt` whenever `go.mod` / `go.sum` change:

```bash
./flatpak/generate-sources.sh
```

---

## Prerequisites (build machine)

```bash
# Flatpak + the builder
sudo apt install flatpak flatpak-builder      # Debian/Ubuntu
# or: sudo dnf install flatpak flatpak-builder # Fedora

# Add Flathub if you haven't already
flatpak remote-add --if-not-exists flathub \
  https://flathub.org/repo/flathub.flatpakrepo

# Runtime, SDK and the Go SDK extension used by the manifest
flatpak install -y flathub \
  org.gnome.Platform//49 \
  org.gnome.Sdk//49 \
  org.freedesktop.Sdk.Extension.golang//25.08
```

> The manifest targets GNOME **49** (a currently supported runtime). GNOME 48
> has aged out of Flathub, and 50 is available too — bump `runtime-version` in
> the manifest if you want the newest. The Go extension branch (`25.08`) tracks
> the freedesktop SDK that GNOME 49 is built on. If a branch isn't found, check
> <https://flathub.org/apps/org.freedesktop.Sdk.Extension.golang> and update
> both the manifest (`sdk-extensions`) and the install command.

---

## Local test build

The manifest references `shared-modules/...`, so for a **local** test build you
need that repo checked out next to the manifest. From this `flatpak/` directory:

```bash
git clone https://github.com/flathub/shared-modules.git

flatpak-builder --user --install --force-clean build-dir \
  uk.co.easysmartapps.WeatherWidget.yml

flatpak run uk.co.easysmartapps.WeatherWidget
```

(The `shared-modules` clone here is only for local testing. On Flathub it is a
**git submodule** of the app repo — see the next section.)

### Lint before submitting (what Flathub's CI runs)

```bash
flatpak install -y flathub org.flatpak.Builder

# Validate the metainfo
flatpak run --command=flatpak-builder-lint org.flatpak.Builder \
  appstream uk.co.easysmartapps.WeatherWidget.metainfo.xml

# Validate the manifest
flatpak run --command=flatpak-builder-lint org.flatpak.Builder \
  manifest uk.co.easysmartapps.WeatherWidget.yml

# Validate the built repo (after a build with --repo=repo)
flatpak run --command=flatpak-builder-lint org.flatpak.Builder \
  repo repo
```

Both errors **and warnings** are treated as fatal by Flathub CI. Fix everything
it reports before opening a PR.

#### Screenshot / icon errors on a LOCAL `repo` lint are expected

When you lint a locally built repo you will likely see:

```
"appstream-external-screenshot-url"
"appstream-remote-icon-not-mirrored"
```

These are **local-only** and do not indicate a problem with this packaging.
Screenshots and the remote icon in the metainfo point at
`raw.githubusercontent.com`. Flathub's own build pipeline downloads them,
mirrors them into the ostree repo, and rewrites the catalogue URLs to
`https://dl.flathub.org/media/...`. That URL-rewrite only happens when the
build detects it is running on Flathub's infrastructure, so a plain local
build leaves the mirrored paths un-prefixed and the linter flags them.

You can confirm mirroring works locally by building with the media flag:

```bash
flatpak-builder --force-clean --repo=repo --disable-rofiles-fuse \
  --mirror-screenshots-url=https://dl.flathub.org/media/ \
  build-dir uk.co.easysmartapps.WeatherWidget.yml
```

After this you should see `Committed screenshot ref: screenshots/x86_64` and a
populated `build-dir/files/share/app-info/media/` tree — proof the screenshots
and icon are fetched and mirrored. On Flathub these resolve cleanly.

> This packaging has been built and verified end-to-end on Ubuntu with
> `org.gnome.Platform//49` + `golang//25.08`: the Go/CGO/GTK3 build succeeds
> offline, the appindicator tray lib links from `/app/lib`, the binary runs,
> and the only remaining `repo` lint findings are the two mirror items above
> plus a non-fatal "newer runtime available" note.

---

## Submitting to Flathub

Full official guide: <https://docs.flathub.org/docs/for-app-authors/submission>

1. **Fork/clone the Flathub submissions repo** and create a branch named after
   the App ID:

   ```bash
   git clone https://github.com/flathub/flathub.git
   cd flathub
   git checkout -b uk.co.easysmartapps.WeatherWidget
   ```

2. **Add the packaging files** to the branch root. Copy from this directory:

   ```
   uk.co.easysmartapps.WeatherWidget.yml
   uk.co.easysmartapps.WeatherWidget.metainfo.xml
   uk.co.easysmartapps.WeatherWidget.desktop
   go.mod.yml
   modules.txt
   ```

3. **Add shared-modules as a submodule** (this is how the tray dependency is
   provided in CI):

   ```bash
   git submodule add https://github.com/flathub/shared-modules.git shared-modules
   ```

4. **Point the manifest at your tagged source instead of the local dir.** The
   `type: dir` source is only for local iteration. For submission, replace the
   `weatherwidget` module's `- type: dir\n  path: ..` source with a pinned git
   tag or release tarball, e.g.:

   ```yaml
       sources:
         - type: git
           url: https://github.com/gcclinux/WeatherWidget.git
           tag: v2.0.3
           commit: <full-40-char-commit-sha-of-that-tag>
         - go.mod.yml
   ```

   Flathub requires a stable, pinned source (tag + commit). Make sure a
   matching `v2.0.3` tag exists on GitHub.

5. **Commit and push** to your branch, then open a **pull request against
   `flathub/flathub`**. The Flathub build bot will build your PR and post
   results. Address any lint/build failures, push fixes, repeat.

6. **After acceptance**, Flathub creates a dedicated repo
   `github.com/flathub/uk.co.easysmartapps.WeatherWidget` where future updates
   land. You'll get collaborator access.

7. **Verify the app** (recommended) so it shows as developer-verified:
   <https://docs.flathub.org/docs/for-app-authors/verification>. Because the App
   ID is `uk.co.easysmartapps.*`, verification is done by proving control of the
   **easysmartapps.co.uk** domain — see the next section.

---

## Domain verification (easysmartapps.co.uk)

Flathub will ask you to prove you control the domain in the App ID. For a
website-based check it looks for a token file at a `.well-known` path on
`https://easysmartapps.co.uk/` (the Flathub verification page shows the exact
filename and token to use once your app is accepted).

> **Firebase Hosting caveat.** The site is served by Firebase Hosting, and its
> `firebase.json` has `"ignore": ["**/.*"]`, which means **dotfiles and
> `.well-known/` are NOT deployed by default.** If you place the verification
> file under `public/.well-known/` it will silently not publish. Fix it one of
> two ways in `firebase.json` under `hosting`:
>
> 1. Add an explicit rewrite/exception, or more simply, host the token at a
>    non-dotfile path and add a `redirects`/`rewrites` entry mapping the
>    `.well-known` path to it, **or**
> 2. Loosen the ignore so the well-known dir is kept, e.g.:
>
>    ```json
>    "ignore": ["firebase.json", "**/node_modules/**"]
>    ```
>
>    (dropping the blanket `**/.*`) and then deploy `public/.well-known/<token-file>`.
>
> After `firebase deploy`, confirm it is reachable:
>
> ```bash
> curl -i https://easysmartapps.co.uk/.well-known/<token-file>
> ```
>
> It must return `200` with the exact token contents before you click verify.

Alternatively, Flathub supports a **DNS TXT record** method if you'd rather not
touch the hosting config — add the TXT record Flathub gives you to the
`easysmartapps.co.uk` zone and verify from there.

---

## Keeping it updated

For each new release:

1. Tag the release on GitHub (`vX.Y.Z`) and update the `release` file.
2. Add a `<release>` entry to the metainfo XML.
3. If dependencies changed, rerun `./generate-sources.sh`.
4. Bump the `tag` + `commit` in the manifest.
5. Open a PR to the app's Flathub repo.

---

## Notes on this app's sandbox permissions

The manifest grants only what WeatherWidget needs:

- `--share=network` — fetch weather from provider APIs over HTTPS.
- `--socket=x11` + `--env=GDK_BACKEND=x11`, `--share=ipc`, `--device=dri` —
  GTK3 rendering on X11/XWayland with GPU acceleration.
- `--talk-name=org.kde.StatusNotifierWatcher`,
  `--talk-name=org.freedesktop.Notifications` — the system tray icon.

### Why `--socket=x11` (and `GDK_BACKEND=x11`) rather than `fallback-x11`+`wayland`

WeatherWidget is a **positioned desktop widget**: the user chooses an absolute
on-screen location (e.g. `1024,10`) in Settings, and that position is saved to
config and restored on launch.

Native Wayland deliberately forbids clients from setting their own absolute
window position — the compositor decides placement. So under a pure Wayland
backend (`--socket=wayland` with GDK auto-selecting Wayland), GNOME Mutter
ignores the saved coordinates and centres the widget. This was the observed
Flatpak bug: the config loaded correctly and `win.Move(x,y)` was called, but the
window still landed in the centre.

The app therefore runs on **XWayland**, exactly as the deb/rpm/AppImage/Snap
builds do, where its X11 positioning path (`WM_NORMAL_HINTS` USPosition +
`XMoveWindow` + `_NET_MOVERESIZE_WINDOW`) is honoured. That requires:

- `--socket=x11` — a real X11 socket. Note `--socket=fallback-x11` is **not**
  enough here: it only exposes an X11 socket when there is *no* Wayland session,
  so on a Wayland desktop it provides nothing and forcing x11 fails with
  `cannot open display`.
- `--env=GDK_BACKEND=x11` — forces the GDK X11 backend so the app connects to
  XWayland instead of auto-selecting native Wayland. This mirrors the Snap,
  which also sets `GDK_BACKEND=x11`.

The app's Go code honours a pre-set `GDK_BACKEND`: when it is `x11` it uses the
X11 positioning path; when unset (e.g. a native Wayland desktop package without
this env) it auto-selects Wayland and skips the X11-only calls. So this manifest
env is what selects the XWayland path inside the sandbox.

> Trade-off / Flathub review note: `flatpak-builder-lint manifest` accepts this
> configuration, but a reviewer may still ask about raw `--socket=x11`. It is
> justified here because absolute window positioning is a core feature that
> Wayland does not permit. If a narrower set is required, the fallback is
> `--socket=fallback-x11` + `--socket=wayland` (dropping `GDK_BACKEND=x11`),
> which builds and runs but **loses saved-position support on Wayland
> sessions** — the widget will open where the compositor decides.

Config is written to the app's sandboxed data dir
(`~/.var/app/uk.co.easysmartapps.WeatherWidget/config/…`). The GTK entry point
resolves this via `os.UserConfigDir()`, which honours `$XDG_CONFIG_HOME` — set
automatically by Flatpak — so settings and window position persist across
restarts. (Hardcoding `~/.config` would write to a host path the sandbox does
not persist, which is why early Flatpak builds appeared to "lose" all settings.)
