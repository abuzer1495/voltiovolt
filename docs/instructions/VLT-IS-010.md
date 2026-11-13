# VOLTIOVOLT (VLT UPS MANAGEMENT)
## INSTRUCTION SET – MESSAGE 10
## CONSOLE UPS SIMULATOR, TEST CONNECTION, CHART SERIES VISIBILITY FIX

### DOCUMENT IDENTIFICATION

- Document number: VLT-IS-010
- Revision: 00
- Date: 2026-11-13
- Reference documents: VLT-IS-001 Revision 07. VLT-IS-002 to VLT-IS-009 Revision 00. VLT-RPT-009 Revision 00. VLT-REF-001 Revision 00.
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: new tool vlt-simulator, Transport layer, Device Manager, VLT GUI History screen, test packages. Release: MVP.
- Type: Build instruction.

---

## 00-00-00 GENERAL

### 00-00-01 Owner requests

| No. | Request |
|-----|---------|
| RQ-01 | A simple console program (one exe, opens in a cmd window) that simulates a UPS. It has menus. The user moves with the up and down arrow keys and changes values with the keyboard. |
| RQ-02 | A test connection in VoltioVolt. When the user selects it and the simulator runs, VoltioVolt connects to the simulator and communicates with the Megatec protocol, as with a real UPS. The user changes the UPS conditions in the simulator and VoltioVolt shows the changes at once. |
| RQ-03 | DEFECT: in the History charts (for example input voltage), when the user hides a series, it becomes visible again by itself. A hidden series MUST stay hidden. The recording continues in the background. |

### 00-00-02 Purpose

1. Make the console tool vlt-simulator (RQ-01).
2. Add the transport "Test connection (simulator)" to VoltioVolt (RQ-02). It uses the real Driver code (Dialect detection, parsers, Poller), not the internal demo model.
3. Fix the chart series visibility (RQ-03).
4. Do the tasks in the order of 03-00-00. Then write Report 10.

### 00-00-03 Not in this instruction

- The Reference UPS profile (it needs the Capture of T-01).
- A virtual COM port driver (for example com0com). It changes the operating system (SEC-102).

---

## 01-00-00 DECISIONS

| No. | Subject | Decision |
|-----|---------|----------|
| OD-44 | Link between VoltioVolt and vlt-simulator | A local stream connection, not a COM port: Windows named pipe \\.\pipe\voltiovolt-simulator (with PIPE_REJECT_REMOTE_CLIENTS); Linux Unix socket $XDG_RUNTIME_DIR/voltiovolt-simulator.sock, or /tmp/voltiovolt-simulator-<uid>.sock when XDG_RUNTIME_DIR is not set. No TCP. No change to the operating system (SEC-102). |
| OD-45 | Protocol on the link | The same bytes as on a serial line: ASCII command + CR, response + CR (VLT-REF-001). |
| OD-46 | OS shutdown in test connection mode | Dry run by default, as in demo mode. A Setting "test.real_shutdown" (default off) permits a real shutdown. The VLT GUI shows a WARNING when the user turns it on. |
| OD-47 | CONTROL commands in test connection mode | VoltioVolt sends them to the simulator. This is permitted, because the simulator is not real hardware. |
| OD-48 | Data of the test connection | Measurements and Events have the source "test". The History and Events screens show the data of the current source by default. A filter shows the other sources (real, demo, test). |
| OD-49 | Chart series visibility | The visibility of each series is a user choice. It stays after a data refresh, a change of the range and a restart of VoltioVolt. Hiding a series does not change the recording. |

---

## 02-00-00 GENERAL RULES

1. Branch name: vlt-is-010/task-name. One task, one pull request. Squash merge after CI passes.
2. SEC-086 gives Owner approval to merge each pull request of this instruction.
3. The rules of VLT-IS-007 01-30-00 ("These rules stay") and SEC-102 stay.
4. All new user text of VoltioVolt is in en.json and tr.json. The text of vlt-simulator is in Turkish and English (option --lang tr or en; default: the OS language if it is Turkish, else English).

---

## 03-00-00 TASKS

### TASK 03-00-00-001 Fix the chart series visibility (RQ-03, OD-49)

Branch: vlt-is-010/chart-visibility.

1. Find the cause. Probable cause: the chart object is made again at each data refresh, and the series visibility goes back to the default. Write the cause in Report 10 section 5.
2. Keep one chart object for each chart. Update the data with the update function of uPlot (setData). Do not make the chart again for a data refresh.
3. Keep the visibility of each series in the component state. Store it in the Setting "ui.history.series_visibility" (a "ui." Setting, no elevated state necessary). Load it at start.
4. A hidden series stays hidden after: a data refresh, a range change, a screen change, a restart.
5. Hiding a series changes only the display. The engine records all Measurements without change.
6. Apply the same fix to all charts in VoltioVolt.
7. Front-end tests: hide a series, refresh the data 10 times, change the range, the series stays hidden. Restore the stored visibility at start.

### TASK 03-00-00-002 Make the simulator protocol server

Branch: vlt-is-010/sim-server. Directory: internal/sim/server.

1. The server uses the existing simulator model (internal/sim). It listens on the endpoint of OD-44.
2. It accepts one client at a time. A second client gets no connection. The server shows "Client connected" and "Client disconnected".
3. It reads bytes until CR and answers as a UPS (VLT-REF-001, VLT-IS-002 03-50-02).
4. Dialect profiles (selectable at run time):

| Profile | Answers | Other commands |
|---------|---------|----------------|
| megatec | Q1, F, I and the CONTROL commands of VLT-REF-001 section 5 | Selectable reply: echo, NAK or none |
| voltronic-qs | megatec plus QS, QMOD, QBV | Same |
| voltronic | voltronic-qs plus QGS, QWS, QFLAG, QRI, QID, QVFW, QMD | Same |

5. Response formats: exactly the byte positions of VLT-REF-001 sections 6.1, 6.2 and 6.3 (Q1 47 bytes, F 22 bytes, I 39 bytes with the CR). For the Voltronic commands, use the formats of the test vectors of VLT-IS-002 03-50-02.
6. CONTROL commands change the model:

| Command | Effect in the model |
|---------|---------------------|
| T, Tnn, TL | Test runs: b2 = 1. The UPS uses the battery. The test ends after 10 s, after nn minutes, or when battery low. |
| CT | Test ends: b2 = 0. |
| Q | Beeper toggle: b0 changes. |
| S...R... | b1 = 1. Output off after the delay. Output on after the restore time when mains power is on. |
| C | Shutdown cancelled: b1 = 0 (if the output is still on). |

7. Fault injection (selectable): none, no response, garbage bytes, split response (2 parts with 200 ms between), wrong length, old bytes before the response. Response delay: 0 to 2000 ms.
8. A log keeps the last 200 requests and responses with the time.
9. Unit tests: each response format with the parsers of internal/dialect, each CONTROL effect, each fault injection.

### TASK 03-00-00-003 Make the console program vlt-simulator (RQ-01)

Branch: vlt-is-010/sim-console. Directory: cmd/vlt-simulator.

1. One binary: vlt-simulator.exe (Windows), vlt-simulator (Linux). It runs in a console window (cmd, Windows Terminal, Linux terminal). No administrator rights. No installation.
2. Use a pure Go terminal library with a permitted license (for example tcell or Bubble Tea). It MUST work in the classic cmd window of Windows 10 and 11.
3. Screen layout (one screen, no scroll at 100 × 30 characters or more):
    1. Top line: "VoltioVolt UPS Simulator", the endpoint, the client state (connected or waiting), the dialect.
    2. Left part: the menu.
    3. Right part: the live status (all Q1 values, the 8 status bits as text, output on or off, the active timers).
    4. Bottom part: the last 8 lines of the log (time, request, response).
    5. Last line: the key help.
4. Keys:

| Key | Function |
|-----|----------|
| Up, Down | Select a menu item |
| Left, Right | Change the value by one step |
| Page Up, Page Down | Change the value by ten steps |
| Enter or Space | Switch an on/off item, or run an action |
| 1 to 9 | Run a scenario (step 6) |
| R | Reset to the default values |
| L | Show or hide the full log |
| Q or Esc | Quit (asks for confirmation) |

5. Menu items:

| Item | Values | Default |
|------|--------|---------|
| Mains power | on, off | on |
| Input voltage | 0 to 300 V, step 1 V | 230 V |
| Input frequency | 45.0 to 65.0 Hz, step 0.1 Hz | 50.0 Hz |
| Output voltage (on mains) | 200 to 240 V, step 1 V | 220 V |
| Load | 0 to 150 %, step 1 % | 30 % |
| Battery charge | 0 to 100 %, step 1 % | 100 % |
| Battery speed | x1, x10, x60 (discharge and charge speed) | x1 |
| Temperature | 0 to 80 °C, step 1 °C | 25 °C |
| UPS type | on-line, standby | on-line |
| Battery low (b6) | auto, forced on | auto (on below 20 %) |
| UPS fault (b4) | off, on | off |
| Bypass or AVR (b5) | off, on | off |
| Beeper (b0) | off, on | on |
| Dialect | megatec, voltronic-qs, voltronic | megatec |
| Reply to other commands | echo, NAK, none | echo |
| Response delay | 0 to 2000 ms, step 50 ms | 50 ms |
| Fault injection | none, no response, garbage, split, wrong length, old bytes | none |
| Rated values (F) | 220 V or 230 V; battery 24 V or 36 V; 50 Hz or 60 Hz | 220 V, 24 V, 50 Hz |

6. Scenarios (keys 1 to 9):

| Key | Scenario |
|-----|----------|
| 1 | Power failure now (mains off) |
| 2 | Power failure for 2 minutes, then mains on |
| 3 | Low input voltage: 180 V |
| 4 | High input voltage: 260 V |
| 5 | Overload: 110 % |
| 6 | Battery low: charge 15 % and mains off |
| 7 | Communication loss for 30 seconds (no response) |
| 8 | High temperature: 55 °C |
| 9 | Back to normal (defaults, mains on) |

7. Calculated values:
    1. Battery voltage from the charge: on-line type per cell (1.667 V at 0 % to 2.267 V at 100 %); standby type total (blocks × 10.0 V at 0 % to blocks × 13.6 V at 100 %). Blocks = rated battery voltage / 12.
    2. On mains: the charge increases. On battery: the charge decreases at a rate that depends on the load and the battery speed.
    3. Runtime for QBV: from the charge and the load.
    4. On battery: input voltage 0, input fault voltage = the last input voltage.
8. Command-line options: --lang tr or en; --headless (no screen, for CI); --scenario <number> (with --headless: run one scenario); --dialect <name>.
9. A second vlt-simulator on the same computer shows "The simulator already runs" and stops.

### TASK 03-00-00-004 Add the test connection to VoltioVolt (RQ-02)

Branch: vlt-is-010/test-connection.

1. New transport choice in the Device screen and in the wizard: "Test connection (simulator)".
2. The transport connects to the endpoint of OD-44. It uses the normal Exchange steps (VLT-IS-002 TASK 03-00-00-006 step 2).
3. The Device Manager uses the normal Probe List, the normal Dialect selection and the normal Poller. Do not use the demo model of VLT-IS-009.
4. If vlt-simulator does not run, the device state is "simulator not found". The help text: "Start vlt-simulator.exe, then wait. VoltioVolt connects automatically." Try again every 5 seconds.
5. A blue banner "TEST CONNECTION" is on all screens.
6. OS shutdown: dry run by default (OD-46). Text at the end of a countdown: "Test: the computer would shut down now." The Setting "test.real_shutdown" is in the Device screen, off by default, with this WARNING: "With this setting on, a test can turn off this computer."
7. CONTROL commands go to the simulator (OD-47).
8. Data source "test" (OD-48). Notifications have the prefix "[TEST]".
9. A change from the test connection to a real port asks for confirmation.

### TASK 03-00-00-005 Add an end-to-end test in CI

Branch: vlt-is-010/e2e.

1. On windows-latest and on ubuntu-latest:
    1. Start vlt-simulator with --headless.
    2. Start VoltioVolt with --self-test and a new option that selects the test connection. Use a temporary data folder.
    3. Check: the Dialect megatec is detected, Q1 values arrive, and the input voltage is 230 V.
    4. Run scenario 1 in the simulator (a control option of --headless, for example a second process call or a timed scenario option). Check: the Event ON_BATTERY.
    5. Check: no real OS shutdown (dry run).
2. Repeat step 1.3 with --dialect voltronic. Check: the Dialect voltronic is detected.

### TASK 03-00-00-006 Change the test packages

Branch: vlt-is-010/packages.

1. Add vlt-simulator.exe to the Windows zip and vlt-simulator to the Linux tar.gz.
2. Update the CI file-list check.
3. install.cmd and install.sh also copy the simulator. No shortcut for the simulator.
4. OKUBENI-README.txt: a section "UPS simulator" (Turkish and English) with these steps: start vlt-simulator.exe; in VoltioVolt select Device, "Test connection (simulator)"; change the values in the simulator with the arrow keys; watch VoltioVolt. Include the key table of TASK 03-00-00-003 step 4.

### TASK 03-00-00-007 Update the documentation

Branch: vlt-is-010/docs.

1. docs/user/simulator.md (Turkish and English): the screen, the keys, the menu items, the scenarios, the connection to VoltioVolt.
2. docs/user/owner-tests.md: add T-17: simulator test. Steps: start the simulator, connect VoltioVolt, run scenarios 1, 2, 5, 6 and 7, check the Dashboard, the Events, the Notifications and the countdown (dry run). Check that a hidden chart series stays hidden (RQ-03).
3. docs/architecture.md: the simulator server, the endpoint, the test connection transport.
4. docs/decisions.md: OD-44 to OD-49.

---

## 04-00-00 ACCEPTANCE CRITERIA

- A hidden chart series stays hidden after 10 data refreshes, a range change and a restart (front-end tests).
- vlt-simulator runs in the classic cmd window. Its menu works with the keys of TASK 03-00-00-003 step 4.
- The end-to-end tests of TASK 03-00-00-005 pass on Windows and Linux.
- In test connection mode, no real OS shutdown happens unless "test.real_shutdown" is on (test).
- The simulator responses pass the parsers of internal/dialect for all three Dialect profiles (tests).
- The packages contain the simulator. CI passes for each pull request. The pre-release "test-latest" exists with the new packages.

---

## 05-00-00 REPORT 10 REQUEST

1. Write Report 10 after all tasks. Stop after Report 10.
2. Title: VLT-RPT-010, Revision 00. Markdown. Simplified Technical English.
3. Sections, in this order:
    1. Summary (5 sentences or fewer)
    2. Download (release URL, asset names, SHA-256 values)
    3. Tasks (table: task, status, pull request link)
    4. Tests (short)
    5. Deviations (include the cause of RQ-03 here)
    6. Questions
4. Do not add other sections.

END OF MESSAGE VLT-IS-010
