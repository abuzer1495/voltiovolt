# VoltioVolt (VLT UPS Management)

VoltioVolt (VLT) is open-source UPS monitoring and safe shutdown software for Windows and Linux.

**Status: pre-alpha.** There is no release. Do not use VLT to protect equipment.

## Purpose

- vltd is the VLT service. It is the only component that talks to the UPS.
- vltd reads the UPS state with READ commands. It records Measurements and Events.
- vltd shuts down the computer safely when the battery is low (Shutdown Policies).
- vltd sends Notifications on Channels (desktop, Telegram, e-mail).
- The VLT GUI shows the data. It changes Settings only through vltd.

## Platforms

| Platform | Status |
|----------|--------|
| Windows 10 and Windows 11, x86_64 | Planned (MVP) |
| Linux with systemd, x86_64 | Planned (MVP) |

## Reference UPS

The Owner tests VLT on one unit: Tunçmatik Newtech Pro II 1 kVA, online type. The UPS uses a variant of the Megatec Q protocol family over USB HID or RS-232 serial. The exact Dialect of this unit is UNVERIFIED.

## Safety

> **WARNING:** CONTROL commands (for example shutdown, battery test or beeper commands) and CONFIG commands change the UPS state or the UPS settings. An incorrect command can stop the power to the protected equipment. VLT has no encoder for CONTROL and CONFIG commands in this release. The Transport accepts only READ commands from the command registry. Do not add code that can send other commands without an approved instruction.

## Repository contents

| Path | Content |
|------|---------|
| `internal/dialect` | Command registry, READ encoder, response parsers |
| `internal/transport` | USB HID and serial Transports |
| `internal/sim` | UPS simulator for tests |
| `cmd/vlt-sim` | Simulator with a pty endpoint (Linux) |
| `cmd/vlt-probe` | READ-only capture tool for the Owner |

## Documentation

- [Architecture](docs/architecture.md)
- [Security](docs/security.md)
- [Decisions](docs/decisions.md)
- [Protocol facts](docs/protocol.md)
- [Source register](docs/references.md)
- [Dependency register](docs/dependencies.md)
- [vlt-probe user guide](docs/user/probe.md)
- [Instructions](docs/instructions/)
- [Security policy](SECURITY.md)
- [Contributing](CONTRIBUTING.md)

## License

VLT is free software under the GNU Affero General Public License, version 3 or later (AGPL-3.0-or-later). Refer to [LICENSE](LICENSE).
