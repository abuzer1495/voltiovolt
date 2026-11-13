# VOLTIOVOLT (VLT UPS MANAGEMENT)
## INSTRUCTION SET – MESSAGE 7
## TEST BUILDS ON GITHUB ACTIONS, LIGHTER PROCESS, NOTIFICATION CHANNELS

### DOCUMENT IDENTIFICATION

- Document number: VLT-IS-007
- Revision: 00
- Date: 2026-11-10
- Reference documents: VLT-IS-001 Revision 05. VLT-IS-002 to VLT-IS-006 Revision 00. VLT-RPT-006 Revision 00. VLT-REF-001 Revision 00.
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: CI, test packages, Notification Router, Channels, VLT GUI. Release: MVP.
- Type: Build instruction.

---

## 00-00-00 GENERAL

### 00-00-01 Purpose

1. The Owner cannot test VLT now. There is no package to download. This instruction makes GitHub Actions build a test package for each merge to main and publish it on the Releases page.
2. This instruction makes the process lighter. Refer to 01-30-00.
3. This instruction adds the Notification Router and the Channels: desktop, Telegram and e-mail (SMTP).
4. Do the tasks in the order of 03-00-00. TASK 03-00-00-001 has the highest priority. Merge it first.

### 00-00-02 Not in this instruction

- Gmail OAuth. It comes in the security hardening step (01-40-00).
- WhatsApp, voice call, Telegram administrator commands (release v1.0).
- MSI, deb and rpm installers (VLT-IS-008).

---

## 01-00-00 DECISIONS

### 01-10-00 Answers to the questions for the Instruction Author

| No. | Decision |
|-----|----------|
| Q-IA-1 | Accepted for now. On Windows, DTR and RTS are on after the open. Do not change the library. If T-01 shows a problem, a later instruction changes it. |
| Q-IA-2 | Accepted. |
| Q-IA-3 | In this instruction (TASK 03-00-00-002). SEC-094 is suspended (01-30-00). |
| Q-IA-4 | No VLT action. The computer is off at that time. The GUI text tells the user the behavior after T-12. A later instruction adds the result of T-12 to the text. |

### 01-20-00 Owner decisions

| No. | Subject | Decision |
|-----|---------|----------|
| OD-30 | Test builds | GitHub Actions builds all binaries. The Builder does not build packages on the Builder computer for the Owner. |
| OD-31 | Test releases | The Owner approves a pre-release with the fixed tag "test-latest". The workflow replaces it at each merge to main. Real releases still need Owner approval (SEC-067). |
| OD-32 | Process | Less security process now. The security hardening step comes before release v1.0 (01-40-00). |
| OD-33 | E-mail in the MVP | SMTP with a password (Gmail: app password). Gmail OAuth moves to the security hardening step. |

### 01-30-00 Changes to the rules

These changes make VLT-IS-001 Revision 06.

| Item | Change |
|------|--------|
| SEC-094 | Suspended until the security hardening step. |
| SEC-088 | Keep: do not export a token in the shell. Remove: the token check log at the start and the end of each task. |
| SEC-067 | Add: "Exception: the pre-release test-latest (OD-31)." |
| New code | New code needs normal unit tests. New go/ast tests and fuzz targets are not necessary. |
| Report | The report format is shorter (05-10-00). |
| These rules stay | No CONTROL command to real hardware by the Builder. No Secret in the repository. No Secret in logs. Secret fields stay write-only in the GUI. The ANALIZ folder stays private. CI must pass before a merge. |

### 01-40-00 Roadmap

| Step | Content | Release |
|------|---------|---------|
| VLT-IS-007 | Test builds on GitHub Actions, Wails update, Notification Router, desktop, Telegram and SMTP Channels, first-run wizard | MVP |
| VLT-IS-008 | Installers (MSI, deb, rpm), start of the VLT GUI at logon, upgrade, uninstall | MVP |
| VLT-IS-009 | Fixes from the Owner tests, Reference UPS profile, release candidate | MVP |
| VLT-IS-010 | Security hardening: Gmail OAuth, SEC-094 again, code signing, review of all SEC rules | v1.0 |
| VLT-IS-011 to 017 | Analyzer, reports, scheduled tests, CONFIG commands, Telegram administration with TOTP, WhatsApp, voice call, NUT server, encrypted backup | v1.0 |
| VLT-IS-018 to 024 | VLT Agent, Wake-on-LAN, more than one UPS, web interface, MQTT and Home Assistant, USB 4G modem, Mega(USB) and SEC dialects | v2.0 |

---

## 02-00-00 GENERAL RULES

1. Branch name: vlt-is-007/task-name. One task, one pull request. Squash merge after CI passes.
2. SEC-086 gives Owner approval to merge each pull request of this instruction.
3. CGO_ENABLED=0 for all code except cmd/vlt-gui on Linux.

---

## 03-00-00 TASKS

### TASK 03-00-00-001 Make the test build workflow (highest priority)

Branch: vlt-is-007/test-build. File: .github/workflows/test-build.yml.

1. Triggers: push to main, and workflow_dispatch (manual start from the Actions page).
2. Version text: "0.0.0-test+" and the short commit SHA. Put it in all binaries with -ldflags. vltd, vlt-gui and vlt-probe show it.
3. Windows job (windows-latest):
    1. Build vltd.exe, vlt-gui.exe, vlt-setup.exe and vlt-probe.exe.
    2. Make the file VoltioVolt-test-windows-amd64.zip with this content:

| File | Content |
|------|---------|
| vltd.exe, vlt-gui.exe, vlt-setup.exe, vlt-probe.exe | Binaries |
| install-test.cmd | Asks for Administrator rights (UAC) by itself. Copies the binaries to "C:\Program Files\VoltioVolt-Test\". Runs vlt-setup dev-install with that path. Makes a Start menu shortcut "VoltioVolt (test)" for vlt-gui.exe. Shows "Done" or the error. |
| uninstall-test.cmd | Asks for Administrator rights. Runs vlt-setup dev-uninstall. Removes the folder and the shortcut. Asks before it deletes the data. |
| OKUBENI-README.txt | Turkish and English. Steps: download, SmartScreen ("More info", "Run anyway"), stop UPSilon 2000 and RupsMon, run install-test.cmd, start "VoltioVolt (test)", link to docs/user/owner-tests.md. |

    3. Change vlt-setup dev-install if necessary so that it accepts the install folder.
4. Linux job (ubuntu-latest):
    1. Build vltd, vlt-gui (cgo, GTK 4, WebKitGTK 6.0) and vlt-probe.
    2. Make the file VoltioVolt-test-linux-amd64.tar.gz with the binaries, the packaging/linux files, install-test.sh (runs as root: copies the binaries to /usr/local/lib/voltiovolt-test/, runs vlt-dev-init.sh, installs the units and the polkit rule, starts vltd), uninstall-test.sh and OKUBENI-README.txt (Turkish and English, with the run-time packages libgtk-4-1 and libwebkitgtk-6.0-4).
5. Release job (runs after both jobs pass):
    1. permissions: contents: write. Only this job has write permission.
    2. Delete the old release and the old tag "test-latest". Make a new pre-release with the tag "test-latest" on the current commit.
    3. Title: "Test build" with the date and the short SHA.
    4. Body: commit SHA, the text "Test build. Not for normal use.", a link to docs/user/owner-tests.md.
    5. Assets: the zip file, the tar.gz file, and SHA256SUMS.txt.
6. Merge this task first. Then confirm that the workflow ran on main and that the release exists. Write the release URL in Report 7.

### TASK 03-00-00-002 Update Wails

Branch: vlt-is-007/wails.

1. Update cmd/vlt-gui to Wails v3.1.0, or to a later stable version.
2. CI MUST pass, including the test-build workflow.

### TASK 03-00-00-003 Make the Notification Router

Branch: vlt-is-007/notify. Directory: internal/notify.

1. Input: the Events of the Event Engine and the countdown state of the Policy Engine.
2. Settings for each Channel: on or off, and the minimum severity (INFO, WARNING, CRITICAL). Default: WARNING.
3. Flood control: the same Event type in 60 seconds or less goes in one combined message.
4. Queue: when a send fails, try again after 10 s, 30 s, 1 min, 5 min, then every 5 min for 1 hour. A message sent late has the text "late" with the original time.
5. Message text from templates in Turkish and English. The language is the Setting ui.language.
6. Message content: Event, time, mode, input voltage, load, battery charge, runtime when available. No Secret. No serial number.
7. Unit tests: severity filter, flood control, retry, templates.

### TASK 03-00-00-004 Add the desktop Channel

Branch: vlt-is-007/desktop.

1. vltd sends the Notifications to the VLT GUI as push messages.
2. The VLT GUI shows them as OS notifications (Windows notification, Linux desktop notification).
3. If the VLT GUI does not run, no desktop Notification is shown. The Events stay in the Events screen.

### TASK 03-00-00-005 Add the Telegram Channel

Branch: vlt-is-007/telegram.

1. Use the Telegram Bot API over HTTPS with long polling (getUpdates). No webhook.
2. The bot token is a Secret (S-01). Store it in the Vault.
3. Pairing:
    1. The GUI makes a one-time code of 6 digits. The code expires after 10 minutes.
    2. The user sends "/start" and the code to the bot.
    3. vltd records the chat ID with the name of the chat.
4. vltd accepts commands only from paired chat IDs. It ignores other chats and writes an audit entry.
5. Commands: /status and /durum (the same function): mode, voltages, load, battery, runtime, active policy, last Event.
6. Telegram sends the Notifications of the Router to each paired chat.
7. Unit tests with a fake Telegram HTTP server.

### TASK 03-00-00-006 Add the e-mail Channel (SMTP)

Branch: vlt-is-007/email.

1. Settings: server, port, security (TLS on port 465, or STARTTLS on port 587), user name, sender address, receiver addresses (1 to 5).
2. The password is a Secret (S-04). Store it in the Vault.
3. A preset "Gmail": smtp.gmail.com, port 465, TLS. The GUI shows the help text: "Use a Google app password, not your normal password. Google account, Security, 2-Step Verification, App passwords."
4. Verify the server certificate. If STARTTLS fails, do not send.
5. Subject format: "[VLT][severity] Event".
6. Unit tests with a fake SMTP server.

### TASK 03-00-00-007 Add the Channel screens and IPC methods

Branch: vlt-is-007/channels-gui.

1. IPC methods (elevated state for all changes): channels.list, channels.settings.set, channels.test (sends a test message), telegram.pair.start, telegram.chats.list, telegram.chats.remove.
2. GUI screen "Notifications":
    1. One section for each Channel: on or off, minimum severity, settings, "Send test message".
    2. Secret fields (Telegram token, SMTP password) are write-only. After save, show only the hint.
    3. Telegram: "Pair a chat" button, the code and the steps, the list of paired chats with "Remove".
3. Add all text to en.json and tr.json.

### TASK 03-00-00-008 Make the first-run wizard

Branch: vlt-is-007/wizard.

1. The wizard starts when no PIN is set. The user can also start it from the General screen.
2. Steps:
    1. Set the PIN.
    2. Select the UPS port (Device screen functions, with "Scan").
    3. Show the live values. The user confirms that they are correct.
    4. Shutdown Policy: show the defaults, run the Simulation, activate it.
    5. Notifications: Telegram and e-mail (each step can be skipped).
    6. Question: "Are the modem and the router connected to the UPS?" If the answer is No, show: "During a power failure, the internet stops. Telegram and e-mail cannot send messages."
    7. Summary.
3. Each step has "Back" and "Skip" (except step 1).

### TASK 03-00-00-009 Update the documentation

Branch: vlt-is-007/docs.

1. docs/user/owner-tests.md (English and Turkish): at the start, add "How to get the test package": Releases page, "Test build", download the zip or the tar.gz, check SHA256SUMS.txt.
2. Add T-13: Notifications. Desktop, Telegram (pairing, /durum, a Notification during a power failure), e-mail (Gmail with an app password, test message).
3. docs/roadmap.md: the table of 01-40-00.
4. docs/decisions.md: the decisions of 01-00-00.

---

## 04-00-00 ACCEPTANCE CRITERIA

- The test-build workflow runs on main and makes the pre-release "test-latest" with the zip, the tar.gz and SHA256SUMS.txt.
- CI passes for each pull request.
- The unit tests of the Notification Router, Telegram and e-mail pass.
- No Secret in the repository, in the logs or in a test-build asset.

---

## 05-00-00 REPORT 7 REQUEST

1. Write Report 7 after all tasks. Stop after Report 7.
2. Title: VLT-RPT-007, Revision 00. Markdown. Simplified Technical English.
3. Sections, in this order:
    1. Summary (5 sentences or fewer)
    2. Download (release URL, asset names, SHA-256 values)
    3. Tasks (table: task, status, pull request link)
    4. Tests (short)
    5. Deviations
    6. Questions
4. Do not add other sections.

END OF MESSAGE VLT-IS-007
