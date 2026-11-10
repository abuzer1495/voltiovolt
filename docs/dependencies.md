# Dependency register (SEC-061)

Permitted licenses for Go modules in VLT binaries: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, MPL-2.0. CI checks this list with go-licenses. Versions are from proxy.golang.org; licenses are from go-licenses v2.0.1 and pkg.go.dev (2026-11-10).

## Go modules in VLT binaries

| Name | Version | License | Reason | Used by |
|------|---------|---------|--------|---------|
| golang.org/x/sys | v0.50.0 | BSD-3-Clause | hidraw (unix) and SetupAPI, hid.dll (windows) without cgo | internal/transport, internal/sim |
| go.bug.st/serial | v1.8.0 | BSD-3-Clause | Serial Transport (no cgo on Windows and Linux) | internal/transport |
| github.com/creack/pty | v1.1.24 | MIT | pty endpoint of the simulator (Linux) | internal/sim, cmd/vlt-sim (not in vltd) |
| github.com/coreos/go-systemd/v22 | v22.7.0 | Apache-2.0 | sd_notify READY=1 and STOPPING=1 for Type=notify (daemon package only) (VLT-IS-003 TASK 03-00-00-008) | internal/platform (Linux) |
| github.com/Microsoft/go-winio | v0.6.4 | MIT | Windows named pipe for the IPC server and client (VLT-IS-003 TASK 03-00-00-007; see the note below) | internal/ipc, internal/ipc/client (Windows only) |
| golang.org/x/crypto | v0.58.0 | BSD-3-Clause | Argon2id PIN hash (argon2 package) (VLT-IS-003 TASK 03-00-00-005) | internal/auth |
| modernc.org/sqlite | v1.62.0 | BSD-3-Clause | Store: SQLite without cgo (VLT-IS-003 TASK 03-00-00-003) | internal/store |
| modernc.org/libc | v1.80.1 (indirect) | BSD-3-Clause | Required by modernc.org/sqlite | internal/store |
| modernc.org/mathutil | v1.7.1 (indirect) | BSD-3-Clause | Required by modernc.org/sqlite | internal/store |
| modernc.org/memory | v1.12.1 (indirect) | BSD-3-Clause | Required by modernc.org/sqlite | internal/store |
| github.com/dustin/go-humanize | v1.1.0 (indirect) | MIT | Required by modernc.org/libc | internal/store |
| github.com/google/uuid | v1.6.0 (indirect) | BSD-3-Clause | Required by modernc.org/libc | internal/store |
| github.com/mattn/go-isatty | v0.0.24 (indirect) | MIT | Required by modernc.org/libc | internal/store |
| github.com/ncruces/go-strftime | v1.1.0 (indirect) | MIT | Required by modernc.org/libc | internal/store |
| github.com/remyoudompheng/bigfft | v0.0.0-20230129092748-24d4a6f8daec (indirect) | BSD-3-Clause | Required by modernc.org/mathutil | internal/store |
| github.com/godbus/dbus/v5 | v5.2.4 | BSD-2-Clause | System D-Bus calls to org.freedesktop.login1.Manager (ListInhibitors, CanHibernate, PowerOff, Hibernate) without cgo (VLT-IS-004 TASK 03-00-00-007 step 3.1). License checked in the module zip from proxy.golang.org (LICENSE file, 2 clauses) and with go-licenses. | internal/shutdown (Linux only) |

Note on go-winio (VLT-IS-003 TASK 03-00-00-007 step 3.4). Choice: github.com/Microsoft/go-winio, not own code with golang.org/x/sys/windows. Reason: go-winio is maintained by Microsoft and used by large projects. It gives overlapped I/O, deadlines (needed for the 30-second partial-frame limit), more than one pipe instance, a security descriptor in SDDL format, and it always sets PIPE_REJECT_REMOTE_CLIENTS (pipe.go, v0.6.4). Own code must do all of this again with less test use. go-winio adds no new module except golang.org/x/sys, which VLT already uses.

## Go modules in the VLT GUI (cmd/vlt-gui, separate module; VLT-IS-005)

cmd/vlt-gui is a separate Go module (cmd/vlt-gui/go.mod) with `replace github.com/abuzer1495/voltiovolt => ../..`. So Wails and its cgo code are not in the root module, and `CGO_ENABLED=0 go build ./...` of the root module does not build them. SEC-094: each version below was published 14 days or more before 2026-11-10 (publish time from proxy.golang.org `.info`). The other modules that Wails v3.0.0 lists in its go.mod are not linked into vlt-gui (go list -deps, linux and windows, tag production).

| Name | Version | Published (proxy.golang.org) | License | Reason | Used by |
|------|---------|------------------------------|---------|--------|---------|
| github.com/wailsapp/wails/v3 | v3.0.0 | 2026-10-26T10:57:44Z | MIT | GUI framework: WebView window, bindings, events, tray icon (TASK 03-00-00-002). v3.1.0 (2026-11-01) is the latest stable but is only 9 days old (SEC-094). | cmd/vlt-gui |
| github.com/adrg/xdg | v0.5.3 (indirect) | 2024-10-31T16:19:38Z | MIT | Required by Wails (XDG directories) | cmd/vlt-gui |
| github.com/go-ole/go-ole | v1.3.0 (indirect) | 2023-08-04T03:10:36Z | MIT | Required by Wails on Windows (COM for WebView2) | cmd/vlt-gui (Windows) |
| github.com/godbus/dbus/v5 | v5.2.4 (indirect) | already in the root module | BSD-2-Clause | Required by Wails on Linux (tray) | cmd/vlt-gui (Linux) |
| github.com/Microsoft/go-winio, golang.org/x/sys | as above | already in the root module | MIT, BSD-3-Clause | internal/ipc/client, Wails | cmd/vlt-gui |

Linux system packages for the cgo build of Wails v3.0.0 (CI job gui-linux): build-essential, pkg-config, libgtk-4-dev, libwebkitgtk-6.0-dev, libsoup-3.0-dev, libx11-dev. Sources: wails v3.0.0 pkg/application/linux_cgo.go (`#cgo linux pkg-config: gtk4 webkitgtk-6.0`; the GTK 3 files need the build tag gtk3), internal/assetserver/webview/webkit_linux.go (`gtk4 webkitgtk-6.0 libsoup-3.0 gio-unix-2.0`), pkg/application/global_shortcut_linux_x11.go (`x11`), internal/doctor/packagemanager/apt.go (Debian and Ubuntu package names).

## npm packages of the VLT GUI front end (web/; VLT-IS-005)

Installed with `npm ci --ignore-scripts` only. Exact versions in package.json; package-lock.json is committed. All versions in package-lock.json (92 packages, with the optional packages of other platforms) were resolved with `npm install --before=2026-10-27T23:59:59Z` and checked with `npm view <name> time` (SEC-094). CI checks the licenses of all 92 packages with web/scripts/check-licenses.mjs (SEC-061 allow list) and runs `npm audit --audit-level=high`.

| Name | Version | Published (registry.npmjs.org) | License | Kind | Reason |
|------|---------|--------------------------------|---------|------|--------|
| preact | 11.1.0 | 2026-10-27T11:52:26Z | MIT | runtime | UI components (TASK 03-00-00-002 step 2.1) |
| uplot | 1.6.32 | 2025-03-14T02:51:47Z | MIT | runtime | History charts (TASK 03-00-00-005) |
| vite | 8.3.5 | 2026-10-21T06:38:54Z | MIT | development | Build of the front end |
| typescript | 7.0.2 | 2026-07-08T15:55:18Z | Apache-2.0 | development | Type check |
| vitest | 5.0.5 | 2026-10-15T14:29:01Z | MIT | development | Front-end tests |
| happy-dom | 21.0.4 | 2026-10-20T18:05:53Z | MIT | development | DOM for the front-end tests |

Indirect development packages: their licenses are MIT, ISC, Apache-2.0, BSD-2-Clause (entities), BSD-3-Clause (source-map-js) and MPL-2.0 (lightningcss and its platform packages). The list with the publish dates is in VLT-RPT-005.

The Wails runtime is not an npm package here. The VLT GUI serves the runtime of the Wails Go module at /wails/runtime.js (application.BundledAssetFileServer).

## CI tools (not in VLT binaries)

| Name | Version | License | Reason |
|------|---------|---------|--------|
| actions/checkout | v7.0.1 (3d3c42e5aac5ba805825da76410c181273ba90b1) | MIT | Check out the repository |
| actions/setup-go | v7.0.0 (b7ad1dad31e06c5925ef5d2fc7ad053ef454303e) | MIT | Install Go 1.27.3 |
| actions/upload-artifact | v7.0.2 (cf430e030ddbb5b0abf93d22962f4752f3646cd9) | MIT | vlt-probe artifacts |
| actions/setup-node | v7.1.0 (949feb2413d6458794dcd2491c4babbbce0c15c1), published 2026-10-08 | MIT | Node.js 24 for the gui jobs (VLT-IS-005) |
| gitleaks (binary) | v8.30.1 | MIT | Secret scan of the full history (SEC-063) |
| golang.org/x/vuln (govulncheck) | v1.9.0 | BSD-3-Clause | Vulnerability check (SEC-062) |
| github.com/google/go-licenses/v2 | v2.0.1 | Apache-2.0 | License check (SEC-061) |
