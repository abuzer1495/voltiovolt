# VOLTIOVOLT (VLT UPS MANAGEMENT)
## INSTRUCTION SET – MESSAGE 8
## PORTABLE MODE: ONE APPLICATION, NO CHANGE TO THE OPERATING SYSTEM

### DOCUMENT IDENTIFICATION

- Document number: VLT-IS-008
- Revision: 00
- Date: 2026-11-10
- Reference documents: VLT-IS-001 Revision 06. VLT-IS-002 to VLT-IS-007 Revision 00. VLT-REF-001 Revision 00.
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: Application structure, Vault, shutdown executor, packages, install scripts, documentation. Release: MVP.
- Type: Build instruction.

---

## 00-00-00 GENERAL

### 00-00-01 Problem

1. The test package of VLT-IS-007 does not work for the Owner. The VLT GUI reports an access error (user rights, group).
2. Cause: the MVP design uses a Windows service, a local group, a user right and a protected data folder. A new group membership is active only after the user logs out and logs in again.
3. The Owner requirement: VLT MUST NOT change the operating system. The Owner wants one application that works when he starts it.

### 00-00-02 Purpose

1. Make the portable mode the default mode of VLT. One application, VoltioVolt.exe, contains the VLT GUI and the monitoring engine.
2. The default package and its install scripts make no change to the operating system (SEC-102).
3. The service mode (vltd, vlt-setup) stays in the source code. It is not in the default package. It becomes an optional advanced mode in a later release.
4. Do the tasks in the order of 03-00-00. Then write Report 8.

### 00-00-03 Not in this instruction

- New Channels or new GUI screens other than the changes in 03-00-00.
- MSI, deb and rpm installers.

---

## 01-00-00 DECISIONS

### 01-10-00 Owner decisions

| No. | Subject | Decision |
|-----|---------|----------|
| OD-34 | Change to the operating system | VLT makes no change to the operating system in the default package. Refer to SEC-102. |
| OD-35 | Default mode | Portable mode: one application, monitoring only while it runs. |
| OD-36 | Service mode | Stays in the code. Not packaged. Optional advanced mode later (release v1.0 or later). |
| OD-37 | PIN | Optional in portable mode. If no PIN is set, all functions are available without the elevated state. Secret fields stay write-only. |
| OD-38 | Install script | Copies files into the folder of the user only, with no administrator rights. A Start menu shortcut is made only when the user agrees. |
| OD-39 | Start at logon | Optional. Default: off. The user turns it on in the VLT GUI. It uses only the startup folder of the user. |

### 01-20-00 Changes to the rules

These changes make VLT-IS-001 Revision 07.

| Item | Change |
|------|--------|
| SEC-102 (new) | The default package, its install script and VoltioVolt.exe MUST NOT: install a service; make or change a user, a group or a user right; ask for administrator rights; write to the registry; install a driver; add a firewall rule; write outside the folder of the application, the profile folder of the user and the temporary folder. Exception: the start-at-logon shortcut in the startup folder of the user, only when the user turns it on. |
| SEC-020, SEC-021 | Apply to the service mode only. |
| SEC-003 | Portable mode on Windows: DPAPI in the scope of the current user, with the application entropy. |
| SEC-004, SEC-005 | Portable mode on Linux: refer to TASK 03-00-00-002 step 3. |
| SEC-022 | Applies only when a PIN is set (OD-37). |
| Data folder permission check (VLT-IS-003 TASK 03-00-00-003 step 6) | Applies to the service mode only. |

### 01-30-00 Roadmap

| Step | Content | Release |
|------|---------|---------|
| VLT-IS-008 | Portable mode, packages without a change to the operating system | MVP |
| VLT-IS-009 | Fixes from the Owner tests, Reference UPS profile, release candidate | MVP |
| VLT-IS-010 | Security hardening: Gmail OAuth, SEC-094 again, code signing, review of all SEC rules | v1.0 |
| VLT-IS-011 to 017 | Analyzer, reports, scheduled tests, CONFIG commands, Telegram administration, WhatsApp, voice call, NUT server, encrypted backup, optional service mode with an installer | v1.0 |
| VLT-IS-018 to 024 | VLT Agent, Wake-on-LAN, more than one UPS, web interface, MQTT and Home Assistant, USB 4G modem, Mega(USB) and SEC dialects | v2.0 |

---

## 02-00-00 GENERAL RULES

1. Branch name: vlt-is-008/task-name. One task, one pull request. Squash merge after CI passes.
2. SEC-086 gives Owner approval to merge each pull request of this instruction.
3. The rules of VLT-IS-007 01-30-00 ("These rules stay") stay.

---

## 03-00-00 TASKS

### TASK 03-00-00-001 Make the portable application

Branch: vlt-is-008/portable-app. Directory: cmd/voltiovolt.

1. New binary: VoltioVolt.exe (Windows) and voltiovolt (Linux). It contains the VLT GUI and the monitoring engine in one process.
2. The engine runs the same packages as vltd: Device Manager, Poller, Measurements, Event Engine, Policy Engine, Notification Router, Channels, Store, Vault, audit.
3. Connect the VLT GUI to the engine through an in-process connection (for example net.Pipe) with the existing IPC server and the typed client. Do not open a named pipe or a Unix socket in portable mode.
4. Single instance: only one VoltioVolt process for each user. A second start shows the window of the first process and stops.
5. Tray behavior:
    1. A close of the window hides it. The monitoring continues. The first time, show the text: "VoltioVolt continues to monitor the UPS in the tray."
    2. The tray menu "Quit" stops the monitoring. Before it stops, show the confirmation: "If you quit, VoltioVolt stops the monitoring and the automatic shutdown."
6. Start at logon (OD-39): a switch in the General screen. On: make a shortcut in the startup folder of the user (Windows: shell:startup; Linux: ~/.config/autostart/voltiovolt.desktop). Off: delete it. Default: off.
7. The old binaries vltd and vlt-gui stay in the repository and in CI. They are not in the default package.

### TASK 03-00-00-002 Change the data folder and the Vault for portable mode

Branch: vlt-is-008/portable-data.

1. Data folder:
    1. Default: the folder "data" next to the binary.
    2. If that folder is not writable, use the profile folder of the user: Windows %LOCALAPPDATA%\VoltioVolt, Linux ~/.local/share/voltiovolt.
    3. The General screen shows the data folder path.
    4. Do not check or change the ACL of the folder.
2. Windows Vault: DPAPI, current user scope (no CRYPTPROTECT_LOCAL_MACHINE), entropy "VoltioVolt-DEK-v1" and the install ID. Keep the canary and the Vault states.
3. Linux Vault:
    1. If the Secret Service (D-Bus org.freedesktop.secrets) is available, keep the DEK there.
    2. If not, keep the DEK in a file with mode 0600 in the data folder. The Vault state is "reduced protection".
    3. Use a pure Go library or direct D-Bus calls (godbus is already used). Record the dependency.
4. The service mode keeps its old Vault code without changes.

### TASK 03-00-00-003 Change the access rules for portable mode

Branch: vlt-is-008/portable-access.

1. PIN optional (OD-37):
    1. If no PIN is set, the engine gives the elevated state to the in-process connection at all times.
    2. If a PIN is set, the rules of VLT-IS-003 TASK 03-00-00-005 apply.
    3. The General screen has "Set PIN" and "Remove PIN" (Remove needs the current PIN).
2. The first-run wizard: step 1 "PIN" becomes optional, with a "Skip" button. Text: "A PIN prevents other people at this computer from changing the shutdown settings."
3. Confirmation texts for dangerous actions stay (Vault reset, test until low, policy activation, UPS output off).

### TASK 03-00-00-004 Change the shutdown executor for a normal user

Branch: vlt-is-008/portable-shutdown.

1. Windows: enable SeShutdownPrivilege in the token of the process with AdjustTokenPrivileges. A normal user has this right on Windows 10 and 11 by default. Do not ask for administrator rights.
2. If the privilege is not available, the Simulation step "OS shutdown" shows the error "This user cannot shut down the computer." The policy cannot be activated.
3. Linux: call logind PowerOff or Hibernate on the system D-Bus as the user. The default polkit rules permit this for the active local session. Do not install a polkit rule in portable mode. If logind refuses, show the same error as step 2.
4. Add a check of this right to the Simulation and to the first-run wizard (step "Shutdown Policy").

### TASK 03-00-00-005 Show port access problems without a change to the system

Branch: vlt-is-008/port-access.

1. Windows: if the COM port is busy, show the existing "port busy" help text (UPSilon 2000 and RupsMon).
2. Linux: if the open of /dev/ttyUSB* or /dev/ttyACM* fails with "permission denied", show this text. Do not run the command:
    "Your user cannot open this port. Run this command one time, then log out and log in: sudo usermod -aG dialout <user name>"
3. Linux HID (hidraw) with "permission denied": show a similar text with the udev rule as an example. Do not install it.

### TASK 03-00-00-006 Change the test packages and the install scripts

Branch: vlt-is-008/packages. File: .github/workflows/test-build.yml.

1. Windows zip: VoltioVolt-test-windows-amd64.zip.

| File | Content |
|------|---------|
| VoltioVolt.exe | Portable application. It works from the extracted folder without installation. |
| vlt-probe.exe | Capture tool (T-01). |
| install.cmd | No administrator rights. Copies VoltioVolt.exe and vlt-probe.exe to %LOCALAPPDATA%\Programs\VoltioVolt. Asks: "Make a Start menu shortcut? (Y/N)". With Y, makes the shortcut in the Start menu folder of the user. Starts VoltioVolt.exe. |
| uninstall.cmd | No administrator rights. Asks first. Removes the folder, the Start menu shortcut and the startup shortcut of the user. Asks separately before it deletes the data folder. |
| remove-old-test-service.cmd | Only for computers with the VLT-IS-007 test installation. Asks for administrator rights. Stops and removes the service vltd, removes the user right of NT SERVICE\vltd, removes the group "VLT Operators" if it has no other members, removes C:\Program Files\VoltioVolt-Test and the old shortcut. Asks before it deletes C:\ProgramData\VoltioVolt. |
| OKUBENI-README.txt | Turkish and English. Method 1: extract the zip and start VoltioVolt.exe. Method 2: run install.cmd. SmartScreen ("More info", "Run anyway"). Stop UPSilon 2000 and RupsMon before the start. WebView2 is necessary (installed on most Windows 10 and 11 computers). If the old test was installed, run remove-old-test-service.cmd one time. |

2. Linux tar.gz: VoltioVolt-test-linux-amd64.tar.gz with voltiovolt, vlt-probe, install.sh (copies to ~/.local/opt/voltiovolt; asks before it makes ~/.local/share/applications/voltiovolt.desktop; no sudo), uninstall.sh and OKUBENI-README.txt (run-time packages libgtk-4-1 and libwebkitgtk-6.0-4; dialout text of TASK 03-00-00-005).
3. Remove vltd, vlt-gui, vlt-setup, install-test.cmd, uninstall-test.cmd, install-test.sh and uninstall-test.sh from the default packages.
4. Add a CI test that extracts the Windows zip and checks the file list. Add a script check: install.cmd and uninstall.cmd contain no "runas", no "sc ", no "net localgroup", no "reg add" and no "icacls".
5. The release job stays (pre-release "test-latest").

### TASK 03-00-00-007 Update the documentation

Branch: vlt-is-008/docs.

1. docs/user/owner-tests.md (English and Turkish), new structure:
    1. Preparation: download the test package, stop UPSilon 2000 and RupsMon, start VoltioVolt.exe from the extracted folder.
    2. If the VLT-IS-007 test was installed: run remove-old-test-service.cmd one time.
    3. Remove the tests that need the service mode (T-02, T-03, T-05, T-07 and the service parts of T-04 and T-06). Keep their text in a section "Service mode (later)".
    4. Keep T-01, T-08 to T-13. Change their steps for portable mode.
    5. Put T-01 (Capture) first.
2. docs/architecture.md: portable mode and service mode, the in-process connection, the data folder, the Vault in portable mode.
3. docs/security.md: SEC-102 and the changes in 01-20-00.
4. docs/roadmap.md: the table of 01-30-00.
5. docs/decisions.md: the decisions of 01-00-00.

---

## 04-00-00 ACCEPTANCE CRITERIA

- VoltioVolt.exe starts from the extracted zip folder on Windows without administrator rights and without an install script. Give the CI evidence (a start test on windows-latest that runs the binary with the simulator and stops it).
- The Windows zip has only the files of TASK 03-00-00-006 step 1.
- The script check of TASK 03-00-00-006 step 4 passes.
- No code in portable mode calls a service, group, user right, registry or ACL function. Give the test name.
- CI passes for each pull request. The pre-release "test-latest" exists with the new packages.

---

## 05-00-00 REPORT 8 REQUEST

1. Write Report 8 after all tasks. Stop after Report 8.
2. Title: VLT-RPT-008, Revision 00. Markdown. Simplified Technical English.
3. Sections, in this order:
    1. Summary (5 sentences or fewer)
    2. Download (release URL, asset names, SHA-256 values)
    3. Tasks (table: task, status, pull request link)
    4. Tests (short)
    5. Deviations
    6. Questions
4. Do not add other sections.

END OF MESSAGE VLT-IS-008
