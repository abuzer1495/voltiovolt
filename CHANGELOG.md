# Changelog

All notable changes to VoltioVolt are in this file. The format is [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the versions use [Semantic Versioning](https://semver.org/). Turkish: [docs/user/surum-notlari.md](docs/user/surum-notlari.md).

## [Unreleased]

### Fixed

- Windows: opening a serial port no longer asserts DTR and RTS when the profile is "DTR and RTS unchanged" (VLT-RPT-006). go.bug.st/serial turns both lines on if `InitialStatusBits` is nil, so VoltioVolt now always passes the bits at open and sets them again immediately after open. "unchanged" and "off" are off; "on" is on. The Known issue in 0.9.0-rc.1 is still accurate for that release. FIELD: the Owner should confirm the lines on the Reference UPS (T-01).

## [0.9.0-rc.1] – 2026-11-20

### Added

- VoltioVolt, a portable application for Windows and Linux (x86_64): one program with a tray icon. It makes no change to the operating system. It keeps its data in the folder `data` next to the program.
- UPS link through a USB serial port (COM port) or USB HID. VoltioVolt finds the Megatec, Voltronic QS or Voltronic protocol. The profile "Tunçmatik Newtech Pro II (Megatec)" is ready for the Reference UPS.
- A port with a known USB-to-serial chip has the mark "Probable UPS". VoltioVolt sends nothing to a port that you did not select.
- Dashboard with live values, History charts, Events and a battery charge estimate.
- Shutdown Policy: the computer shuts down or hibernates after a time on battery, at a low runtime or at a low battery charge. A countdown with Cancel and a Simulation before you turn it on. Optional: the UPS turns off its output and turns it on again after a set time.
- Notifications on the desktop, in Telegram and by e-mail (Gmail or SMTP).
- Optional PIN for the settings, with "Forgot PIN". Passwords and tokens are kept encrypted.
- Setup wizard, Simple mode and Expert mode, interface size, English and Turkish.
- Demo mode and a test connection with the simulator vlt-simulator: try VoltioVolt without a UPS.
- Battery replace signal, from a failed battery test or from the battery date that you write.
- Help for each field, with links to the official documentation.

### Changed

- The default package is portable. The service mode (vltd) stays only in the source code.
- One list of Turkish terms for the interface, for example "Basit mod", "Uzman mod" and "Kapatma politikası".
- The version is in About, in the tray tooltip, in `--version` and, for a release candidate, in the window title.

### Fixed

- The PIN unlock works after the first PIN is set.
- The History charts update live and keep the series that you selected.
- Each wizard step fits without a scroll bar. The window behind a dialog does not move. No text is cut.
- In portable mode the header shows the UPS link, for example "UPS'e bağlı (COM3)", not "vltd'ye bağlı".

### Known issues

- The shutdown tests T-09 and T-12 are not done on real hardware yet.
- The Capture of the Reference UPS (T-01) is not done yet. Commands other than Q1, F and I are not verified on the Reference UPS.
- Windows SmartScreen shows a warning, because the files are not signed.
- On Windows, DTR and RTS are on when the port opens (VLT-RPT-006).
