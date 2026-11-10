# Dependency register (SEC-061)

Permitted licenses for Go modules in VLT binaries: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, MPL-2.0. CI checks this list with go-licenses. Versions are from proxy.golang.org; licenses are from go-licenses v2.0.1 and pkg.go.dev (2026-11-10).

## Go modules in VLT binaries

| Name | Version | License | Reason | Used by |
|------|---------|---------|--------|---------|
| golang.org/x/sys | v0.50.0 | BSD-3-Clause | hidraw (unix) and SetupAPI, hid.dll (windows) without cgo | internal/transport, internal/sim |
| go.bug.st/serial | v1.8.0 | BSD-3-Clause | Serial Transport (no cgo on Windows and Linux) | internal/transport |
| github.com/creack/pty | v1.1.24 | MIT | pty endpoint of the simulator (Linux) | internal/sim, cmd/vlt-sim (not in vltd) |
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

Note on go-winio (VLT-IS-003 TASK 03-00-00-007 step 3.4). Choice: github.com/Microsoft/go-winio, not own code with golang.org/x/sys/windows. Reason: go-winio is maintained by Microsoft and used by large projects. It gives overlapped I/O, deadlines (needed for the 30-second partial-frame limit), more than one pipe instance, a security descriptor in SDDL format, and it always sets PIPE_REJECT_REMOTE_CLIENTS (pipe.go, v0.6.4). Own code must do all of this again with less test use. go-winio adds no new module except golang.org/x/sys, which VLT already uses.

## CI tools (not in VLT binaries)

| Name | Version | License | Reason |
|------|---------|---------|--------|
| actions/checkout | v7.0.1 (3d3c42e5aac5ba805825da76410c181273ba90b1) | MIT | Check out the repository |
| actions/setup-go | v7.0.0 (b7ad1dad31e06c5925ef5d2fc7ad053ef454303e) | MIT | Install Go 1.27.3 |
| actions/upload-artifact | v7.0.2 (cf430e030ddbb5b0abf93d22962f4752f3646cd9) | MIT | vlt-probe artifacts |
| gitleaks (binary) | v8.30.1 | MIT | Secret scan of the full history (SEC-063) |
| golang.org/x/vuln (govulncheck) | v1.9.0 | BSD-3-Clause | Vulnerability check (SEC-062) |
| github.com/google/go-licenses/v2 | v2.0.1 | Apache-2.0 | License check (SEC-061) |
