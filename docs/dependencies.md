# Dependency register (SEC-061)

Permitted licenses for Go modules in VLT binaries: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, MPL-2.0. CI checks this list with go-licenses. Versions are from proxy.golang.org; licenses are from go-licenses v2.0.1 and pkg.go.dev (2026-11-10).

## Go modules in VLT binaries

| Name | Version | License | Reason | Used by |
|------|---------|---------|--------|---------|
| golang.org/x/sys | v0.50.0 | BSD-3-Clause | hidraw (unix) and SetupAPI, hid.dll (windows) without cgo | internal/transport, internal/sim |
| go.bug.st/serial | v1.8.0 | BSD-3-Clause | Serial Transport (no cgo on Windows and Linux) | internal/transport |
| github.com/creack/pty | v1.1.24 | MIT | pty endpoint of the simulator (Linux) | internal/sim, cmd/vlt-sim (not in vltd) |
| github.com/coreos/go-systemd/v22 | v22.7.0 | Apache-2.0 | sd_notify READY=1 and STOPPING=1 for Type=notify (daemon package only) (VLT-IS-003 TASK 03-00-00-008) | internal/platform (Linux) |
| github.com/Microsoft/go-winio | v0.6.4 | MIT | Windows named pipe for the IPC server and client (VLT-IS-003 TASK 03-00-00-007; see the note below), and for the vlt-simulator endpoint of OD-44 (VLT-IS-010 TASK 03-00-00-002). | internal/ipc, internal/ipc/client, internal/sim/server (Windows only) |
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
| github.com/gdamore/tcell/v2 | v2.13.10 | Apache-2.0 | Terminal UI for vlt-simulator; it works in the classic cmd window of Windows 10 and 11 (VLT-IS-010 TASK 03-00-00-003). License checked in the module (LICENSE file, Apache 2.0) and with go-licenses. Published 2026-05-06T16:35:18Z on proxy.golang.org. | cmd/vlt-simulator |
| github.com/gdamore/encoding | v1.0.1 (indirect) | Apache-2.0 | Required by tcell | cmd/vlt-simulator |
| github.com/lucasb-eyer/go-colorful | v1.3.0 (indirect) | MIT | Required by tcell | cmd/vlt-simulator |
| github.com/rivo/uniseg | v0.4.7 (indirect) | MIT | Required by tcell | cmd/vlt-simulator |
| golang.org/x/term | v0.47.0 (indirect) | BSD-3-Clause | Required by tcell | cmd/vlt-simulator |
| golang.org/x/text | v0.43.0 (indirect) | BSD-3-Clause | Required by tcell | cmd/vlt-simulator |

Note on go-winio (VLT-IS-003 TASK 03-00-00-007 step 3.4). Choice: github.com/Microsoft/go-winio, not own code with golang.org/x/sys/windows. Reason: go-winio is maintained by Microsoft and used by large projects. It gives overlapped I/O, deadlines (needed for the 30-second partial-frame limit), more than one pipe instance, a security descriptor in SDDL format, and it always sets PIPE_REJECT_REMOTE_CLIENTS (pipe.go, v0.6.4). Own code must do all of this again with less test use. go-winio adds no new module except golang.org/x/sys, which VLT already uses.

## Go modules in the VLT GUI (cmd/vlt-gui, separate module; VLT-IS-005)

cmd/vlt-gui is a separate Go module (cmd/vlt-gui/go.mod) with `replace github.com/abuzer1495/voltiovolt => ../..`. So Wails and its cgo code are not in the root module, and `CGO_ENABLED=0 go build ./...` of the root module does not build them. VLT-IS-007 TASK 03-00-00-002: Wails v3.1.0 (SEC-094 is suspended, VLT-IS-007 01-30-00; v3.1.0 is the latest stable version on 2026-11-11). The other modules that Wails v3.1.0 lists in its go.mod (for example github.com/ebitengine/purego v0.11.1 and golang.org/x/term v0.46.0) are not linked into vlt-gui (go list -deps, linux and windows, tag production). cmd/vlt-gui/go.mod has golang.org/x/term v0.47.0 as indirect, selected by the root module (tcell, VLT-IS-010 TASK 03-00-00-003).

| Name | Version | Published (proxy.golang.org) | License | Reason | Used by |
|------|---------|------------------------------|---------|--------|---------|
| github.com/wailsapp/wails/v3 | v3.1.0 | 2026-11-01T11:57:25Z | MIT | GUI framework: WebView window, bindings, events, tray icon (VLT-IS-005 TASK 03-00-00-002; updated from v3.0.0 by VLT-IS-007 TASK 03-00-00-002). | cmd/vlt-gui |
| git.sr.ht/~jackmordaunt/go-toast/v2 | v2.0.3 (indirect) | 2025-01-17T09:34:50Z | Unlicense OR MIT | Required by the Wails notifications service on Windows (toast notifications, VLT-IS-007 TASK 03-00-00-004). License checked in the module (LICENSE file); VLT uses it under MIT. go-licenses in CI ignores this module because it refuses the other choice, Unlicense. It writes the toast activator keys under HKEY_CURRENT_USER\Software\Classes. | cmd/vlt-gui (Windows) |
| github.com/go-ole/go-ole | v1.3.0 (indirect) | 2023-08-04T03:10:36Z | MIT | Required by Wails on Windows (COM for WebView2) | cmd/vlt-gui (Windows) |
| github.com/godbus/dbus/v5 | v5.2.4 (indirect) | already in the root module | BSD-2-Clause | Required by Wails on Linux (tray, and the notifications service: org.freedesktop.Notifications) | cmd/vlt-gui (Linux) |
| github.com/Microsoft/go-winio, golang.org/x/sys | as above | already in the root module | MIT, BSD-3-Clause | internal/ipc/client, Wails | cmd/vlt-gui |

Linux system packages for the cgo build of Wails v3.1.0 (CI jobs gui-linux and test-build-linux; the same as for v3.0.0): build-essential, pkg-config, libgtk-4-dev, libwebkitgtk-6.0-dev, libsoup-3.0-dev, libx11-dev. Sources: wails v3.1.0 pkg/application/linux_cgo.go (`#cgo linux pkg-config: gtk4 webkitgtk-6.0`; the GTK 3 files need the build tag gtk3), internal/assetserver/webview/webkit_linux.go (`gtk4 webkitgtk-6.0 libsoup-3.0 gio-unix-2.0`), pkg/application/global_shortcut_linux_x11.go (`x11`), internal/doctor/packagemanager/apt.go (Debian and Ubuntu package names).

## Go modules of the build tool tools/winresgen (VLT-IS-011 TASK 03-00-00-005)

tools/winresgen is a separate Go module (tools/winresgen/go.mod, `replace github.com/abuzer1495/voltiovolt => ../..`). It runs before the Windows build and writes the resource file (.syso) of VoltioVolt.exe and vlt-gui.exe: the application icon (internal/appicon), the version resource and the application manifest (internal/winmeta). Its modules are not linked into the binaries; only the resource data is. CI checks its licenses with go-licenses (job gui-windows, SEC-061 list). Choice: github.com/tc-hib/winres was not used: its license is 0BSD, which is not in the SEC-061 list.

| Name | Version | Published (proxy.golang.org) | License | Reason | Used by |
|------|---------|------------------------------|---------|--------|---------|
| github.com/josephspurrier/goversioninfo | v1.7.0 | 2026-05-11T03:48:46Z | MIT | Version resource, icon group and manifest in a COFF .syso file | tools/winresgen (build only) |
| github.com/akavel/rsrc | v0.10.2 (indirect) | 2021-03-12T14:42:57Z | MIT | COFF resource writer of goversioninfo | tools/winresgen (build only) |

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

The front-end tests use preact/test-utils, a part of the preact package (no other package). Tasks 03-00-00-003 to 03-00-00-009 of VLT-IS-005 add no dependency.

The Wails runtime is not an npm package here. The VLT GUI serves the runtime of the Wails Go module at /wails/runtime.js (application.BundledAssetFileServer).

## CI tools (not in VLT binaries)

| Name | Version | License | Reason |
|------|---------|---------|--------|
| actions/checkout | v7.0.1 (3d3c42e5aac5ba805825da76410c181273ba90b1) | MIT | Check out the repository |
| actions/setup-go | v7.0.0 (b7ad1dad31e06c5925ef5d2fc7ad053ef454303e) | MIT | Install Go 1.27.4 (was 1.27.3; VLT-IS-007: GO-2026-6859, CVE-2026-97054, fixed in 1.27.4) |
| actions/upload-artifact | v7.0.2 (cf430e030ddbb5b0abf93d22962f4752f3646cd9) | MIT | vlt-probe artifacts |
| actions/setup-node | v7.1.0 (949feb2413d6458794dcd2491c4babbbce0c15c1), published 2026-10-08 | MIT | Node.js 24 for the gui jobs (VLT-IS-005) |
| gitleaks (binary) | v8.30.1 | MIT | Secret scan of the full history (SEC-063) |
| golang.org/x/vuln (govulncheck) | v1.9.0 | BSD-3-Clause | Vulnerability check (SEC-062) |
| github.com/google/go-licenses/v2 | v2.0.1 | Apache-2.0 | License check (SEC-061) |
