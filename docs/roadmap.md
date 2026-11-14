# Roadmap

This roadmap follows OD-29, VLT-IS-008 01-30-00 and VLT-IS-011 OD-56. The Owner decides the order. A result of the Owner tests can change it.

## Done

| Step | Content |
|------|---------|
| VLT-IS-001 | Definitions, requirements, component model, repository rules |
| VLT-IS-002 | Protocol layer: Transport (USB HID, serial), Megatec Q Dialect, simulator, vlt-probe |
| VLT-IS-003 | Vault, Store, IPC, vltd start and stop |
| VLT-IS-004 | Monitoring core: Device Manager, Poller, Measurements, Event Engine, Policy Engine and Simulation, shutdown executor |
| VLT-IS-005 | VLT GUI (Wails, web front end) |
| VLT-IS-006 | CONTROL commands: battery tests, beeper, UPS output off and restore |
| VLT-IS-007 | Notifications (desktop, Telegram, e-mail), first-run wizard, test packages |
| VLT-IS-008 | Portable mode: one application, no change to the operating system, optional PIN |
| VLT-IS-009 | Fixes from the Owner tests: Forgot PIN, field help with official links, Demo mode, compact window |
| VLT-IS-010 | vlt-simulator and the test connection, History series visibility |
| VLT-IS-011 | PIN session fix, live History charts, Simple and Expert mode, interface size, Windows 11 look |

The pre-release "Test build" (tag `test-latest`) is made by CI after each change on `main`.

## Next

| Step | Content | Release |
|------|---------|---------|
| VLT-IS-012 | Release candidate: version number, changelog, stable release flow | 0.9.0 (RC) |

## Later

| Content | Release |
|---------|---------|
| Security hardening: Gmail OAuth, SEC-094 again, code signing, review of all SEC rules | v1.0 |
| Analyzer, reports, scheduled tests, CONFIG commands, Telegram administration, WhatsApp, voice call, NUT server, encrypted backup, optional service mode with an installer | v1.0 |
| VLT Agent, Wake-on-LAN, more than one UPS, web interface, MQTT and Home Assistant, USB 4G modem, Mega(USB) and SEC dialects | v2.0 |

The step numbers after VLT-IS-012 are not fixed. The earlier table of VLT-IS-008 01-30-00 gave VLT-IS-010 to security hardening and VLT-IS-011 to 017 to the Analyzer; the Owner changed this order (VLT-IS-010 and VLT-IS-011 above, OD-56).
