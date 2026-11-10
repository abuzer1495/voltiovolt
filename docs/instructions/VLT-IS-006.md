# VOLTIOVOLT (VLT UPS MANAGEMENT)
## INSTRUCTION SET – MESSAGE 6
## DECISIONS ON REPORT 5, MEGATEC SERIAL ALIGNMENT, UPS CONTROL COMMANDS, UPS OUTPUT OFF, ROADMAP

### DOCUMENT IDENTIFICATION

- Document number: VLT-IS-006
- Revision: 00
- Date: 2026-11-10
- Reference documents: VLT-IS-001 Revision 04. VLT-IS-002 to VLT-IS-005 Revision 00. VLT-RPT-005 Revision 00. VLT-REF-001 Revision 00 (given with this instruction).
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: Dialect layer, Transport layer, Device Manager, CONTROL commands, Shutdown Policy, IPC, VLT GUI, documentation. Release: MVP.
- Type: Build instruction.

---

## 00-00-00 GENERAL

### 00-00-01 Purpose

1. This instruction gives the decisions on VLT-RPT-005.
2. This instruction aligns VLT with VLT-REF-001. The serial Megatec path (Q1, F, I) is the primary path of the Reference UPS.
3. This instruction adds the CONTROL commands (tests, beeper, UPS shutdown and restore) with an authorization model.
4. This instruction adds the UPS output off step to the Shutdown Policy.
5. This instruction records the new roadmap.
6. Do only the tasks in 03-00-00. Then write Report 6. Refer to 05-00-00.

### 00-00-02 Not in this instruction

- Channels (Telegram, e-mail, desktop notifications). They come in VLT-IS-007.
- CONFIG commands (Voltronic PE and PD commands). Release v1.0.
- The Mega(USB) dialect and the SEC dialect (VLT-REF-001 sections 8 and 9). Release v2.0.
- Installers (MSI, deb, rpm). They come in VLT-IS-008.
- Analyzer screens. Release v1.0.
- Releases and tags.

### 00-00-03 Failure rule

CAUTION: If a task fails, do not use a workaround that changes the scope.

1. Stop the failed task.
2. Continue with the tasks that do not depend on it.
3. Write the failure in Report 6.

---

## 01-00-00 DECISIONS

### 01-10-00 Answers to the questions for the Instruction Author

| No. | Decision |
|-----|----------|
| Q-IA-1 | Changed. Settings with the key prefix "ui." are not security Settings. A client can write them without the elevated state. A set PIN is necessary. Write an audit entry for each change. |
| Q-IA-2 | Accepted. The package internal/ipc/client/typed stays. |
| Q-IA-3 | Accepted. The Simulation discharge model (0.1 % each virtual second on battery) and the pass-rule limits of the Fallback Policy stay. |
| Q-IA-4 | Accepted with a condition. Refer to TASK 03-00-00-001 step 2. |
| Q-IA-5 | Yes. VLT-IS-007 adds the Channel screens with Secret input fields. |

### 01-20-00 Owner decisions

| No. | Subject | Decision |
|-----|---------|----------|
| OD-24 | VLT-REF-001 | VLT-REF-001 is an accepted source for SEC-060. It is the Owner interoperability analysis. It contains behavior only and synthetic examples. |
| OD-25 | Owner analysis files | The folder ANALIZ on the Builder computer is private. Do not read it. Do not commit it. Use only VLT-REF-001. |
| OD-26 | Primary path of the Reference UPS | Serial Megatec (Q1, F, I) on a virtual COM port. HID VID 0665 PID 5161 is not the path of the Reference UPS. It stays supported for other units. |
| OD-27 | Selection of the serial port | The user selects the port in the VLT GUI. vltd does not send bytes to a serial port that the user did not select or confirm. |
| OD-28 | UPS output off | The function exists in the MVP. The default is off. The Owner turns it on only after test T-12 passes. |
| OD-29 | Roadmap | The roadmap in 01-40-00 replaces all earlier plans. |

### 01-30-00 Changes to VLT-IS-001

These changes make VLT-IS-001 Revision 05.

| Item | Change |
|------|--------|
| 01-00-04 | New rows: "Connection: USB cable. The OS shows a virtual COM port (FIELD, VLT-REF-001)." "Bridge chip VID and PID: UNVERIFIED." "Working path: serial Megatec Q1, F, I (FIELD)." The row "USB identity 0665:5161" becomes: "Not the path of the Reference UPS." |
| 00-10-00 | Dialect names: megatec (Q1, F, I), voltronic-qs (QS family), voltronic (QGS family). |
| 03-20-00 | Add "C" (cancel shutdown) to the CONTROL class. UNVERIFIED. VLT does not send it automatically. |
| SEC-060 | Add: "VLT-REF-001 is an accepted source." |
| SEC-098 (new) | Only the package internal/control can make CONTROL request bytes. Only these callers can use internal/control: the IPC handlers for an elevated request with a confirmation, and the Policy Engine for the UPS output off step. |
| SEC-099 (new) | vltd does not send a byte to a serial port that the user did not select or confirm in the VLT GUI. |
| SEC-100 (new) | VLT never sends "S.0". The permitted shutdown delays are 0.2 to 0.9 minutes (12 to 54 seconds, steps of 6 seconds) and 1 to 10 minutes. |
| SEC-101 (new) | Do not commit a file or a folder of the Owner analysis. A CI check fails if the repository contains the path "ANALIZ/" or a Ghidra project file. |

### 01-40-00 Roadmap

| Step | Content | Release |
|------|---------|---------|
| VLT-IS-006 | Megatec serial alignment, CONTROL commands, UPS output off, UPS Control screen | MVP |
| VLT-IS-007 | Notification Router, Telegram, Gmail (OAuth) and SMTP, desktop notifications, Channel screens, first-run wizard: Channels and the modem question | MVP |
| VLT-IS-008 | Installers (MSI, deb, rpm), start of the VLT GUI at logon, upgrade, uninstall, release preparation | MVP |
| VLT-IS-009 | Fixes from the Owner tests T-01 to T-12, Reference UPS profile, release candidate | MVP |
| VLT-IS-010 to 016 | Analyzer, reports, scheduled tests, CONFIG commands, Telegram administration with TOTP, WhatsApp, voice call, NUT server, encrypted backup | v1.0 |
| VLT-IS-017 to 023 | VLT Agent, Wake-on-LAN, more than one UPS, web interface, MQTT and Home Assistant, USB 4G modem, Mega(USB) and SEC dialects | v2.0 |

NOTE: The Owner tests are necessary before VLT-IS-009. A test result can change the order.

---

## 02-00-00 GENERAL RULES

1. The rules of VLT-IS-003 02-00-00 stay. Branch name: vlt-is-006/task-name.
2. SEC-086 gives Owner approval to merge each pull request of this instruction.
3. CGO_ENABLED=0 for all code except cmd/vlt-gui on Linux.
4. The Builder never sends a CONTROL command to real hardware. All tests use the simulator.
5. Record each new dependency in docs/dependencies.md.

---

## 03-00-00 TASKS

### TASK 03-00-00-001 Fix the items of Report 5

Branch: vlt-is-006/fixes.

1. Settings with the prefix "ui." (Q-IA-1):
    1. settings.set accepts a "ui." key without the elevated state.
    2. settings.set rejects all other keys without the elevated state.
    3. Tests for both cases. The audit entry exists for both cases.
2. Wails (Q-IA-4):
    1. If the work starts on 2026-11-15 or later, update cmd/vlt-gui to Wails v3.1.0. Use a later stable version only if it obeys SEC-094.
    2. If the work starts before 2026-11-15, do not update. Write it in Report 6.

### TASK 03-00-00-002 Add the reference document and the repository check

Branch: vlt-is-006/reference.

1. Commit VLT-REF-001 without changes as docs/reference/VLT-REF-001.md.
2. docs/references.md: add VLT-REF-001 as a source (OD-24).
3. .gitignore: add "ANALIZ/".
4. Add a CI step to the job "gitleaks". The step fails if a tracked path contains "ANALIZ/", or if a tracked file has the extension .gpr, .rep or .gzf (Ghidra files).
5. Do not open, read or copy a file in the folder ANALIZ (OD-25).

### TASK 03-00-00-003 Align the Megatec dialect

Branch: vlt-is-006/megatec. Directory: internal/dialect, internal/device, internal/poller, internal/measure.

1. Dialect "megatec": Q1, F and I. Refer to VLT-REF-001 sections 5 and 6.
2. Parsers:
    1. Q1: exactly 47 bytes with the CR. Check the positions of VLT-REF-001 6.1.
    2. F: exactly 22 bytes with the CR. Check the positions of VLT-REF-001 6.2. Accept the number formats of the NOTE in 6.2.
    3. I: exactly 39 bytes with the CR. Check the positions of VLT-REF-001 6.3. Remove the spaces on the right of each field.
    4. Accept leading spaces in each numeric field. A field with only spaces has no value.
3. Test vectors: add all synthetic examples of VLT-REF-001 sections 6.1, 6.2 and 6.3. Also add negative vectors: one byte too short, one byte too long, a wrong separator position, a letter in a numeric field.
4. Probe List (new order): Q1, F, I, QS, QGS, QMOD, QBV, QWS, QFLAG, QRI, QID, QVFW, QMD. Change vlt-probe and the Device Manager. Change the simulator recorder tests.
5. Dialect selection order: voltronic (QGS answers), voltronic-qs (QS answers), megatec (Q1 answers).
6. Poll groups, new row:

| Dialect | Fast group (every 2 s) | Slow group (every 30 s) | Once (at start and every 6 h) |
|---------|------------------------|-------------------------|-------------------------------|
| megatec | Q1 | none | F, I |

7. Response timeout: 1500 ms. Comm lost: after 3 failed fast-group polls in sequence (no change).
8. Battery charge estimate for the megatec dialect:
    1. New Measurement: battery.charge_estimate, unit %. It is separate from battery.charge (QBV).
    2. On-line UPS (b3 = 0): linear between the Settings battery.cell_empty_v (default 1.667) and battery.cell_full_v (default 2.267).
    3. Standby UPS (b3 = 1): blocks = F rated battery voltage / 12. Linear between blocks × battery.block_empty_v (default 10.0) and blocks × battery.block_full_v (default 13.6).
    4. Limit the result to 0 to 100. Do not round values above 90 to 100.
    5. If the inputs are not available, store no value.
    6. The Dashboard shows "estimate" next to this value.
9. Event rule: for b5 = 1 and b3 = 1, the AVR_ACTIVE data contains "boost" when the input voltage is less than the output voltage, else "buck".
10. docs/protocol.md: mark each fact with its status label from VLT-REF-001.

### TASK 03-00-00-004 Change the serial Transport and the port selection

Branch: vlt-is-006/serial.

1. DTR and RTS: new value "unchanged" for both Settings. Default: "unchanged". With "unchanged", the Transport does not set DTR or RTS at open and after open. Verify the behavior of go.bug.st/serial at open on Windows and Linux. Write the result in section 7 of Report 6.
2. Port busy:
    1. If the open fails because another program uses the port, set the device state "port busy".
    2. The VLT GUI shows the help text: "Another program uses this port. Stop UPSilon 2000 and the RupsMon service, or the other program, then try again."
3. Port selection (OD-27, SEC-099):
    1. Change the default transport to "not configured". vltd sends no byte until the user selects a Transport.
    2. The value "auto" stays. With "auto", vltd probes HID VID 0665 PID 5161 devices and only the serial ports in the Setting device.serial.allowed_ports. The default list is empty.
    3. New IPC method device.scan (elevated, with confirmation): lists the serial ports and sends only "Q1" CR to each port that the user marks. It returns the result for each port: response, no response, busy.
    4. The VLT GUI Device screen: port list with the OS port names and the USB descriptions where the OS gives them, a "Scan" button with the WARNING text: "VLT sends the text Q1 to each marked port. Mark only ports with a UPS.", and a "Use this port" button.
4. Linux: add SupplementaryGroups=dialout to vltd.service and vltd-fallback.service.
5. Windows: no code change. Owner test T-03 checks that NT SERVICE\vltd can open the COM port.

### TASK 03-00-00-005 Make the CONTROL package

Branch: vlt-is-006/control. Directory: internal/control.

WARNING: CONTROL commands can cut power to the protected equipment. Do each step. Do not make a simpler method.

1. Encoders (megatec, voltronic-qs and voltronic dialects):

| Command | Input | Bytes | Status |
|---------|-------|-------|--------|
| Test 10 s | none | T CR | STATIC, PUBLIC |
| Test minutes | n = 1 to 99 | "T" + n with 2 digits + CR | STATIC, PUBLIC |
| Test until low | none | TL CR | STATIC, PUBLIC |
| Cancel test | none | CT CR | STATIC, PUBLIC |
| Beeper toggle | none | Q CR | STATIC, PUBLIC |
| Shutdown and restore | delay (SEC-100), restore minutes 1 to 9999 | VLT-REF-001 5.1 | STATIC, PUBLIC |
| Cancel shutdown | none | C CR | PUBLIC only, UNVERIFIED |

2. There is no encoder for a shutdown without restore. VLT always sends a restore time.
3. Authorization:
    1. An encoder needs a value of the type control.Grant.
    2. Only two functions make a Grant: one for an IPC request in the elevated state with a confirmation text, one for the Policy Engine.
    3. Add a go/ast test like VLT-IS-004 TASK 03-00-00-007 step 4. It fails if a package other than internal/ipc and internal/policy makes a Grant, or if a package other than internal/control calls an encoder.
4. Verification after a send, with the next fast-group polls:

| Command | Expected status | Retries |
|---------|-----------------|---------|
| Test (all types) | b2 = 1 within 2 polls | Up to 3 sends, 2 s apart |
| Cancel test | b2 = 0 within 2 polls | 1 send |
| Shutdown and restore | b1 = 1 within 2 polls | Up to 3 sends, 2 s apart |
| Beeper | b0 changes within 2 polls | 1 send, then 1 retry |
| Cancel shutdown | b1 = 0 within 2 polls | 1 send |

5. Beeper: the function sets a wanted state (on or off). It reads b0 first. It sends Q only if b0 is not the wanted state.
6. The Poller sends a CONTROL command in place of the next fast-group poll. Keep the 500 ms minimum time between requests.
7. A test ends when b2 changes from 1 to 0. TEST_PASSED if b4 and b6 stayed 0 during the test. Else TEST_FAILED.
8. Write an audit entry for each CONTROL send, each verification result and each failure.
9. Simulator: the model reacts to each CONTROL command. Test: b2 = 1 for the test time. Shutdown: b1 = 1, output off after the delay, output on after the restore time if mains power is present. Beeper: b0 changes.
10. The simulator recorder test of the Poller stays: the Poller sends only READ commands, except the CONTROL commands from internal/control.

### TASK 03-00-00-006 Add the UPS output off step to the Shutdown Policy

Branch: vlt-is-006/policy-upsoff.

WARNING: An incorrect delay can turn off the power before the computer has shut down. Data can be lost.

1. New Settings of the Shutdown Policy:

| Setting | Default | Permitted values |
|---------|---------|------------------|
| ups_off.enabled | off | on, off |
| ups_off.delay_minutes | 2 | 1 to 10 (whole minutes) |
| ups_off.restore_minutes | 1 | 1 to 9999 |

2. Sequence at the end of the countdown, when ups_off.enabled is on:
    1. Send the shutdown and restore command through internal/control with the Policy Engine Grant.
    2. Verify b1 = 1 (TASK 03-00-00-005 step 4).
    3. Call the OS shutdown executor. Do this also when the verification fails. Write an audit entry for the failure.
3. The Fallback Policy keeps ups_off.enabled off.
4. A change of these Settings makes the policy inactive (SEC-093).
5. Simulation: the result contains the UPS output off step with its virtual time and the command bytes. The dry-run Simulation sends no command.
6. Tests: the sequence, the verification failure path, the Simulation step, the Fallback Policy value.

### TASK 03-00-00-007 Add the IPC methods

Branch: vlt-is-006/ipc.

| Method | Elevated state needed | Confirmation text | Result |
|--------|------------------------|-------------------|--------|
| device.scan | Yes | "SCAN PORTS" | Result for each marked port |
| ups.control.capabilities | No | none | Supported CONTROL functions for the Dialect |
| ups.test.start | Yes | "START TEST" (type 10 s or minutes); "TEST UNTIL LOW" (type until low) | Result of the verification |
| ups.test.cancel | Yes | none | Result of the verification |
| ups.beeper.set | Yes | none | Result of the verification |

1. ups.test.start rejects a request when the UPS is on battery or when a countdown runs.
2. No IPC method sends the shutdown and restore command or the cancel shutdown command. Only the Policy Engine sends the shutdown and restore command (SEC-098).
3. Add the new methods to the no-Secret response test.
4. Integration test with the simulator: test start, verification, test end, beeper set.

### TASK 03-00-00-008 Change the VLT GUI

Branch: vlt-is-006/gui.

1. New screen "UPS Control":
    1. Buttons: Test 10 seconds, Test for minutes (1 to 99), Test until battery low, Cancel test, Beeper on, Beeper off.
    2. Each button needs the elevated state. Each button is off when ups.control.capabilities does not give the function.
    3. "Test until battery low" shows the WARNING: "This test discharges the battery until it is low. The protected equipment has less battery time after the test until the battery is charged again." The user types "TEST UNTIL LOW".
    4. Show the result of the last test (TEST_PASSED or TEST_FAILED) and its time.
2. Shutdown Policy screen:
    1. Add the 3 Settings of TASK 03-00-00-006 with their ranges.
    2. Show the WARNING of TASK 03-00-00-006 next to ups_off.enabled.
    3. Show the text: "The UPS turns off its output [delay] minutes after the shutdown starts. The computer must shut down in less time. The UPS turns on its output [restore] minutes later."
3. Device screen: the changes of TASK 03-00-00-004 step 3.4. Show the help text for "port busy".
4. Dashboard: show "estimate" next to battery.charge_estimate.
5. Add all new text to en.json and tr.json. The i18n tests MUST pass.

### TASK 03-00-00-009 Update the documentation and the Owner tests

Branch: vlt-is-006/docs. Put the other documentation changes in the pull request of the related task.

1. docs/roadmap.md: the table of 01-40-00 and its NOTE.
2. docs/architecture.md: the CONTROL package, the Grant, the verification, the UPS output off step, the port selection rules.
3. docs/security.md: SEC-098 to SEC-101 and the changes in 01-30-00.
4. docs/decisions.md: one row for each decision in 01-00-00.
5. docs/user/owner-tests.md (English and Turkish):
    1. T-01: add these preconditions. Stop UPSilon 2000 and the RupsMon service before the test. Start them again after the test. Use "--transport serial --port COM3" on Windows. On Linux, find the port with "vlt-probe scan" (/dev/ttyUSB* or /dev/ttyACM*). Write the hardware ID of the port (Windows Device Manager: Ports, COM3, Properties, Details, Hardware Ids).
    2. Add T-11: UPS Control. Beeper off and on. Test 10 seconds. Check the Events and the audit log.
    3. Add T-12: UPS output off and restore. Write this WARNING before the steps:
       "WARNING: The UPS turns off its output during this test. All equipment on the UPS loses power. Connect only the test computer to the UPS. Save all work. Make sure that the battery is fully charged."
       Steps: set ups_off.enabled on, delay 2, restore 1. Run the Simulation and activate the policy. Disconnect the UPS input power. Record the times of the OS shutdown, the UPS output off and the UPS output on. Do the test one time with the input power connected again before the restore time, and one time with the input power not connected.

---

## 04-00-00 ACCEPTANCE CRITERIA

- CI passes on Linux and Windows for each pull request, including gui-linux and gui-windows.
- CGO_ENABLED=0 go build ./... passes for the root module.
- All VLT-REF-001 synthetic examples and all negative vectors have passing parser tests.
- The CI check of SEC-101 exists and passes.
- No test sends a byte to a serial port that was not selected (SEC-099). Give the evidence in Report 6.
- The go/ast test of TASK 03-00-00-005 step 3.3 passes.
- No code path can make "S.0" or a shutdown without restore. Give the test names in Report 6.
- The simulator tests of TASK 03-00-00-005, 006 and 007 pass.
- The i18n tests pass with the new text.
- gitleaks finds no Secret in the full history.
- The token check of SEC-088 shows "absent" at the end of each task.

---

## 05-00-00 REPORT 6 REQUEST

### 05-00-01 General

1. Write Report 6 after all tasks.
2. Stop after Report 6. Do not start work that is not in this instruction.

### 05-10-00 Report format

1. Title: VLT-RPT-006, Revision 00.
2. Format: Markdown. Simplified Technical English.
3. Sections: the same 13 sections as VLT-RPT-005, in the same order. Section 7 "Research results" contains the DTR and RTS behavior of go.bug.st/serial (TASK 03-00-00-004 step 1) and the Wails version decision (TASK 03-00-00-001 step 2).
4. Do not add other sections.

END OF MESSAGE VLT-IS-006
