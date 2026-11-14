# Build and test VoltioVolt from source

This guide is for developers. Users download the pre-release "Test build" (tag `test-latest`): see the [user guide](user/README.md). CI ([.github/workflows/ci.yml](../.github/workflows/ci.yml) and [test-build.yml](../.github/workflows/test-build.yml)) uses the same commands.

## 1. Requirements

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.27.4 (`toolchain go1.27.4` in each `go.mod`) | |
| Node.js and npm | Node.js 24 | Only for the front end in `web/` |
| Linux packages (Debian, Ubuntu) | | `build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev libsoup-3.0-dev libx11-dev`. Only for VoltioVolt and vlt-gui (cgo, GTK 4, WebKitGTK 6.0). |
| Windows | Windows 10 or 11, x64 | No C compiler: VoltioVolt builds with `CGO_ENABLED=0`. The WebView2 Runtime is necessary to run it. |

## 2. Modules

| Module | Directory | Contents |
|--------|-----------|----------|
| Root | `.` | All `internal/` packages, vlt-probe, vlt-simulator, vlt-sim, vltd, vlt-setup. Pure Go, `CGO_ENABLED=0`. |
| VoltioVolt | `cmd/voltiovolt` | The portable application (Wails v3.1.0). Separate `go.mod`. |
| vlt-gui | `cmd/vlt-gui` | The GUI of service mode (Wails v3.1.0). Separate `go.mod`. Not in the default package. |
| winresgen | `tools/winresgen` | Makes the Windows version resource and icon (`.syso`). Separate `go.mod`. |
| Front end | `web/` | TypeScript, Vite, Preact, uPlot. |

## 3. Front end

```
cd web
npm ci --ignore-scripts
npm run typecheck
npm test
npm run build
```

`npm run build` writes the front end to `cmd/vlt-gui/frontend/dist` and copies it to `cmd/voltiovolt/frontend/dist`. Both folders are not in git. Build the front end before you build VoltioVolt or vlt-gui.

A new npm package needs Owner approval, an allowed license (SEC-061) and a row in [dependencies.md](dependencies.md). `npm run licenses` checks the licenses.

## 4. Root module: tools and tests

```
gofmt -l .
go vet ./...
CGO_ENABLED=0 go build ./...
go test ./...
go test -race ./...          # Linux; the race detector needs cgo
```

Tools:

```
CGO_ENABLED=0 go build -trimpath -o vlt-probe ./cmd/vlt-probe
CGO_ENABLED=0 go build -trimpath -o vlt-simulator ./cmd/vlt-simulator
```

Fuzz smoke test (each target for 30 seconds), for example:

```
go test -run='^$' -fuzz='^FuzzParseQ1$' -fuzztime=30s ./internal/dialect
```

## 5. VoltioVolt

### Linux

```
cd cmd/voltiovolt
CGO_ENABLED=1 go vet -tags production .
CGO_ENABLED=1 go test -tags production .
CGO_ENABLED=1 go build -tags production -trimpath -ldflags "-X main.version=0.0.0-dev" -o voltiovolt .
```

### Windows

```
cd tools\winresgen
go run . -version 0.0.0-dev -name VoltioVolt -file VoltioVolt.exe -out ..\..\cmd\voltiovolt\rsrc_windows_amd64.syso
cd ..\..\cmd\voltiovolt
set CGO_ENABLED=0
go vet -tags production .
go test -tags production .
go build -tags production -trimpath -ldflags "-H windowsgui -X main.version=0.0.0-dev" -o VoltioVolt.exe .
```

The `.syso` file gives the icon and the version resource (product name VoltioVolt, description "VoltioVolt UPS Management"). It is not in git. Without it, the build works, but the program has no icon.

Always use the build tag `production`. It turns off the Wails development server and the developer tools ([architecture.md](architecture.md) section 13.1).

## 6. Test packages

Do not make a release or a tag by hand. The workflow `test-build` makes the packages and replaces the pre-release `test-latest` after each change on `main`. Its steps (package contents, SHA256SUMS.txt) are in [test-build.yml](../.github/workflows/test-build.yml).

## 7. Tests without hardware

Do not run tests against a real UPS. Do not send CONTROL or CONFIG commands to real hardware.

- `internal/sim` and `cmd/vlt-sim`: the simulator for unit tests and the pty endpoint (Linux).
- `cmd/vlt-simulator`: the interactive simulator for the test connection ([user/simulator.md](user/simulator.md)). `vlt-simulator --headless --scenario N` runs a scenario without a screen.
- Demo mode (`internal/demo`) runs a simulator in the VoltioVolt process.
- The setting `test.real_shutdown` stays false in all tests.

## 8. Checks before a pull request

1. The commands of sections 3, 4 and 5.
2. gitleaks: `gitleaks dir --no-banner -c .gitleaks.toml .`
3. The official help links: `tools/check-help-links.sh`.
4. [CONTRIBUTING.md](../CONTRIBUTING.md): DCO sign-off, branch and pull request rules.
