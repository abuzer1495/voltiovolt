# VoltioVolt (VLT)

VoltioVolt monitors a UPS (uninterruptible power supply) and shuts down the computer safely when the power fails. It reads the values of the UPS over USB HID or RS-232 serial (Megatec Q family and Voltronic dialects), keeps a history, sends notifications, and runs a Shutdown Policy that you test with a Simulation before you activate it.

> **Status: test builds only.** VoltioVolt is pre-alpha. The only download is the pre-release **"Test build"** (tag `test-latest`), made by CI from `main`. It is not for production computers. Use a test computer. Version 0.9.0 (release candidate) and 1.0 are planned: see [docs/roadmap.md](docs/roadmap.md).

## Features

- **Portable application.** One program: `VoltioVolt.exe` (Windows) or `voltiovolt` (Linux). The GUI and the monitoring engine run in one process and connect through an in-process connection (`net.Pipe`). No service, no administrator rights, no change to the operating system. Start at logon is an optional switch.
- **Optional PIN.** Without a PIN, all functions are available. With a PIN, changes need an unlocked session. The session locks after a time without input (default 10 minutes). **Forgot PIN** resets the PIN after you type `RESET`.
- **Vault** for the Secrets (Telegram bot token, SMTP password): Windows DPAPI (current user); Linux Secret Service, or a file with mode 0600 ("reduced protection"). Secret fields are write-only.
- **Simple and Expert mode.** Simple (the default) shows one screen with the most important values. Expert shows all screens and settings.
- **Compact Windows 11 style interface** with a first-run setup wizard (760 × 560), a menu bar (File, View, Tools, Help), Turkish and English, light and dark theme, and an **interface size** setting (Auto, 80 % to 200 %; Ctrl+Plus, Ctrl+Minus, Ctrl+0).
- **Dashboard, live History charts** (1 h to 1 y, live update, pause and resume, series visibility that is saved), Events, UPS Control (battery tests, beeper), Shutdown Policy with Simulation, Notifications, Security and audit log.
- **Notifications:** desktop, Telegram and e-mail (SMTP, Gmail preset).
- **Help in the application:** a one-line help text for each field, a "?" panel, links to official help pages only (Telegram, Google, Microsoft), and F1 for the [user guide](docs/user/README.md).
- **Safe testing:**
  - **Demo mode:** an in-process simulator. A shutdown is only a message ("Demo: the computer would shut down now.").
  - **Test connection** to **vlt-simulator**, a terminal UPS simulator with 9 scenarios. Its data has the source "test". A real shutdown is off (`test.real_shutdown` is false by default).
- **vlt-probe:** a READ-only capture tool for the analysis of a UPS.

## Download and quick start

1. Open [Releases](https://github.com/abuzer1495/voltiovolt/releases) and select the pre-release **"Test build"** (tag `test-latest`).
2. Download `VoltioVolt-test-windows-amd64.zip` or `VoltioVolt-test-linux-amd64.tar.gz`, and `SHA256SUMS.txt`.
3. Check the SHA-256:
   - Windows (PowerShell): `Get-FileHash VoltioVolt-test-windows-amd64.zip -Algorithm SHA256` must give the value in `SHA256SUMS.txt`.
   - Linux: `sha256sum --ignore-missing -c SHA256SUMS.txt` must show `OK`.
4. Start the program:
   - Windows: extract the zip and double-click `VoltioVolt.exe`. The files are not signed: at "Windows protected your PC", select "More info", then "Run anyway". WebView2 is necessary (Windows 10 and 11 have it). Optional: `install.cmd` copies the files to `%LOCALAPPDATA%\Programs\VoltioVolt`.
   - Linux: `tar xzf VoltioVolt-test-linux-amd64.tar.gz`, then `./voltiovolt` in the extracted folder. Run-time packages (Debian, Ubuntu): `sudo apt install libgtk-4-1 libwebkitgtk-6.0-4`. Optional: `sh install.sh` (no sudo) copies the files to `~/.local/opt/voltiovolt`.
5. Stop other UPS programs (for example UPSilon 2000 and RupsMon). Only one program can use the UPS port.
6. The setup wizard opens at the first start. To try VoltioVolt without a UPS, select **Demo (simulator)** or **Test connection (simulator)** on the Device screen.

The package also contains `vlt-probe`, `vlt-simulator`, uninstall scripts and `OKUBENI-README.txt` (Turkish and English). The [user guide](docs/user/README.md) has the details.

## Safety

- VoltioVolt can shut down your computer. Test the Shutdown Policy with the Simulation, and use Demo mode or the test connection first.
- VoltioVolt sends only commands from its command registry. It sends a CONTROL command (battery test, beeper, UPS output off and restore) only after your confirmation. It has no function that sends a CONFIG command. vlt-probe sends READ commands only.
- Do not run VoltioVolt and another UPS program, or VoltioVolt and vlt-probe, on the same port at the same time.

## Build from source

Requirements: Go 1.27.4, Node.js 24, and on Linux the GTK 4 and WebKitGTK 6.0 development packages. Short form:

```
cd web && npm ci --ignore-scripts && npm run build && cd ..
CGO_ENABLED=0 go build ./...
cd cmd/voltiovolt && go build -tags production -trimpath .
```

See [docs/building.md](docs/building.md) for all steps, the tests and the Windows resources.

## Documentation

| Document | Contents |
|----------|----------|
| [docs/user/README.md](docs/user/README.md) | User guide: install, wizard, PIN, modes, demo, test connection, History, uninstall |
| [docs/user/simulator.md](docs/user/simulator.md) | vlt-simulator: keys, menu, scenarios, connection |
| [docs/user/probe.md](docs/user/probe.md) | vlt-probe capture tool |
| [docs/user/owner-tests.md](docs/user/owner-tests.md) | Owner test plan |
| [docs/README.md](docs/README.md) | Index of all documents (architecture, protocol, security, decisions, roadmap) |
| [docs/building.md](docs/building.md) | Build and test from source |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute (DCO sign-off) |
| [SECURITY.md](SECURITY.md) | Report a vulnerability |
| [CHANGELOG.md](CHANGELOG.md) | Changes |

## License

GNU Affero General Public License v3.0 or later (AGPL-3.0-or-later). See [LICENSE](LICENSE). Each source file has an SPDX header. Third-party licenses: [docs/dependencies.md](docs/dependencies.md).
