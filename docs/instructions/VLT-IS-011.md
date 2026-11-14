# VOLTIOVOLT (VLT UPS MANAGEMENT)
## INSTRUCTION SET – MESSAGE 11
## PIN DEFECT (ROUND 2), LIVE CHARTS, SIMPLE AND EXPERT MODES, INTERFACE SCALE, WINDOWS LOOK

### DOCUMENT IDENTIFICATION

- Document number: VLT-IS-011
- Revision: 00
- Date: 2026-11-14
- Reference documents: VLT-IS-001 Revision 07. VLT-IS-002 to VLT-IS-010 Revision 00. VLT-RPT-010 Revision 00.
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: auth, VLT GUI (all screens), History charts, Windows binary metadata, test suite. Release: MVP.
- Type: Build instruction.

---

## 00-00-00 GENERAL

### 00-00-01 Owner test results (Windows, test build 287be7e)

| No. | Result |
|-----|--------|
| R-07 | The simulator and the test connection work well. PASS. |
| R-08 | DEFECT (PIN, round 2): when the Owner sets a PIN in the wizard, VoltioVolt goes to the locked state at once. There was no restart of VoltioVolt and no restart of the computer. The wizard does not continue. The Owner must skip all steps to the end, then unlock with the PIN. After that, VoltioVolt works. |
| R-09 | DEFECT: the History charts do not update by themselves. The Owner must press "Refresh". The Owner wants to see the changes live. |
| R-10 | REQUEST: two interface modes. The current interface becomes the "Expert" mode. A new "Simple" mode shows only the basic values (status, health, load, battery and similar). |
| R-11 | REQUEST: one setting that scales the text and the controls together, for very large high-resolution screens and for small old screens. |
| R-12 | REQUEST: VoltioVolt looks more like a Windows application. |

### 00-00-02 Probable cause of R-08

1. In portable mode with no PIN, the engine gives the elevated state to the connection at all times (VLT-IS-008 OD-37).
2. When the first PIN is set, the rule changes to "a PIN is set, login is necessary". The connection that set the PIN does not get the elevated state.
3. The next wizard step (for example the port selection, device.settings.set) needs the elevated state. It fails, and the VLT GUI shows the lock screen.
4. The fix of VLT-IS-009 (#57) changed only the unlock screen. It did not test this sequence.

Confirm or reject this cause in Report 11 section 5.

### 00-00-03 Purpose

1. Fix R-08 with the exact Owner sequence as a test.
2. Make the History charts live (R-09).
3. Add the Simple mode and the Expert mode (R-10).
4. Add the interface scale setting (R-11).
5. Make VoltioVolt look and behave like a Windows application (R-12).
6. Fix the Windows test flake of VLT-RPT-010 section 5 item 5 (answer to question 1).
7. Do the tasks in the order of 03-00-00. Then write Report 11.

---

## 01-00-00 DECISIONS

| No. | Subject | Decision |
|-----|---------|----------|
| Q-1 (RPT-010) | Windows test flake | Approved. Use a separate pipe name for each test with a test-only option. The product pipe name does not change. |
| OD-50 | Session after a PIN set | The connection that sets the first PIN, or changes the PIN, keeps the elevated state. The idle timer starts from that time. |
| OD-51 | Wizard and lock | The lock screen never covers the wizard. If a wizard step needs the elevated state and the state is locked, the wizard shows a PIN field inside the dialog. |
| OD-52 | Idle time | User input in the VLT GUI (keyboard, mouse) resets the idle timer. Background polling does not reset it. |
| OD-53 | Interface modes | Two modes: Simple and Expert. Default at the first start: Simple. The wizard asks the user. The mode is a "ui." Setting. |
| OD-54 | Interface scale | One setting "Interface size": Auto (follows the Windows display scale), 80 %, 90 %, 100 %, 110 %, 125 %, 150 %, 175 %, 200 %. Default: Auto. Keyboard: Ctrl + Plus, Ctrl + Minus, Ctrl + 0. |
| OD-55 | Windows look | Follow the Windows 11 design style (Fluent): the Windows font, the Windows accent color, light and dark theme from Windows, a left navigation pane like the Windows Settings application, a menu bar, native dialogs for confirmations. |
| OD-56 | Roadmap | After this instruction: VLT-IS-012 release candidate 0.9.0 (version number, changelog, stable release flow). Then the v1.0 features. |

---

## 02-00-00 GENERAL RULES

1. Branch name: vlt-is-011/task-name. One task, one pull request. Squash merge after CI passes.
2. SEC-086 gives Owner approval to merge each pull request of this instruction.
3. The rules of VLT-IS-007 01-30-00 ("These rules stay") and SEC-102 stay.
4. All new user text is in en.json and tr.json, in natural Turkish with the correct characters.
5. A new npm dependency needs a license from the SEC-061 list. Record it in docs/dependencies.md.

---

## 03-00-00 TASKS

### TASK 03-00-00-001 Fix the PIN defect, round 2 (R-08, OD-50 to OD-52)

Branch: vlt-is-011/pin-session.

1. Write the Owner sequence as an end-to-end test first (Go engine in-process and the GUI Go side, plus a front-end test of the wizard flow):
    1. New empty data folder. Start. The wizard starts.
    2. Set a PIN in the wizard (digits, for example 4826).
    3. Click "Next". The port step opens without the lock screen.
    4. Select "Test connection (simulator)" and save (device.settings.set). This MUST pass without a PIN request.
    5. Complete the Shutdown Policy step: Simulation, activation. This MUST pass without a PIN request.
    6. Complete the wizard. The Dashboard opens. The state is unlocked.
    7. Click "Lock". The lock screen opens. Unlock with 4826. This MUST pass.
2. Repeat the test with the PIN set in the General screen (no wizard).
3. Fix the cause. Implement OD-50, OD-51 and OD-52.
4. OD-52: the VLT GUI sends a keep-alive request (for example auth.touch) after user input, a maximum of one time each 30 seconds. Polling and push messages do not reset the idle timer.
5. Show the lock screen only on: the "Lock" button, the idle time, the start of VoltioVolt with a set PIN, and an elevated request that fails outside the wizard.
6. Write the confirmed cause in Report 11 section 5 with the file and the line.
7. Add T-14 steps 1.1 to 1.7 to docs/user/owner-tests.md.

### TASK 03-00-00-002 Make the History charts live (R-09)

Branch: vlt-is-011/live-charts.

1. The History screen updates the charts without a user action.

| Range | Update interval | Data |
|-------|-----------------|------|
| 1 hour | 2 seconds | raw |
| 24 hours, 7 days | 60 seconds | 1-minute values |
| 30 days, 1 year | 10 minutes | 1-hour values |

2. Get only the new points after the last time stamp. Add them with setData on the existing chart object (VLT-IS-010 OD-49). Remove the points that are older than the range.
3. The hidden series stay hidden (OD-49).
4. If the user zooms or moves the chart, keep the view. When the view includes the current time, it follows the new data.
5. A "Live" indicator and a "Pause" button. The "Refresh" button stays.
6. Stop the updates when the History screen is not visible or the window is minimized.
7. Front-end tests: new points appear without a user action, hidden series stay hidden, the zoom stays.

### TASK 03-00-00-003 Make the Simple mode and the Expert mode (R-10, OD-53)

Branch: vlt-is-011/modes.

1. Expert mode: the current interface with all screens.
2. Simple mode: one screen and a small settings panel. No other screens.
3. Simple screen content, from top to bottom:

| Part | Content |
|------|---------|
| Status card (large) | One sentence, an icon and a color. Examples: "Mains power is on. Everything is normal." / "The UPS runs on battery. Approximately 12 minutes remain." / "Attention: the battery is low." |
| Value tiles | Battery charge %, Load %, Estimated runtime, Input voltage, Output voltage, UPS health (step 4) |
| Automatic shutdown | "On" or "Off", with the main trigger in one sentence. The countdown banner with "Cancel" when a countdown runs. |
| Small chart | Last 1 hour: input voltage and load, live (TASK 03-00-00-002). |
| Last events | The last 3 Events in plain language, with the time. |
| Buttons | "Expert mode", "Settings" (language, interface size, mode, start the wizard). |

4. UPS health in Simple mode:

| State | Conditions |
|-------|------------|
| Good (green) | Mains, no WARNING or CRITICAL Event active |
| Attention (yellow) | On battery, AVR or bypass active, load 90 % or more, temperature 45 °C or more, communication lost |
| Problem (red) | Battery low, UPS fault, overload (100 % or more), over temperature, countdown running, battery replace |

    Show the state as text, icon and color (not color only).
5. The mode switch: a menu item (View, Simple mode or Expert mode) and the buttons of step 3. The switch needs no PIN.
6. The wizard has a step "Interface": Simple or Expert, with one sentence for each.
7. In Simple mode, Settings that need the elevated state are not visible. The countdown cancel needs the elevated state only when a PIN is set.
8. The tray icon, the Notifications and the monitoring are the same in both modes.
9. Front-end tests: the Simple screen with each health state, the mode switch, the stored mode after a restart.

### TASK 03-00-00-004 Add the interface scale (R-11, OD-54)

Branch: vlt-is-011/scale.

1. Setting "ui.scale": auto, 80, 90, 100, 110, 125, 150, 175, 200. Default: auto.
2. All sizes of the interface (font, controls, spaces, icons, chart labels) use relative units from one root value. A change of ui.scale changes all of them together.
3. "Auto": use the display scale of Windows (or of the Linux desktop) through the WebView. Do not scale two times.
4. The setting is in the General screen (Expert) and in the Settings panel (Simple). The change is immediate. No restart.
5. Keyboard: Ctrl + Plus, Ctrl + Minus, Ctrl + 0 (back to Auto).
6. The minimum window size changes with the scale. If the screen is too small for the minimum size, use a smaller scale automatically and show a short message once.
7. Check the layout at these combinations and write the result in Report 11: 1024 × 768 at 100 %, 1366 × 768 at 100 %, 1920 × 1080 at 125 %, 2560 × 1440 at 150 %, 3840 × 2160 at 200 %. Use front-end layout tests where possible.

### TASK 03-00-00-005 Make VoltioVolt look like a Windows application (R-12, OD-55)

Branch: vlt-is-011/windows-look.

1. Visual style:
    1. Font: "Segoe UI Variable", then "Segoe UI", then the system font (Linux).
    2. Colors: the Windows accent color for primary buttons and selection, when the WebView gives it. Light and dark theme from Windows.
    3. Corners 4 px, controls 32 px high at 100 % (VLT-IS-009), thin borders, subtle shadows on dialogs.
    4. Left navigation pane with icons and text, like the Windows Settings application. The pane can be collapsed to icons.
    5. Use the Fluent UI web components (MIT) only if they make the work smaller. Else use CSS only.
2. Native parts (Wails v3):
    1. A menu bar: File (Exit), View (Simple mode, Expert mode, Interface size), Tools (Setup wizard, Simulator help), Help (Documentation, About).
    2. Native confirmation dialogs (Yes, No, Cancel) for the confirmations without a typed word. Keep the typed-word confirmations in the application dialogs.
    3. Native window title bar with the application icon.
3. Windows binary data:
    1. An application icon (a simple original design: a battery with a lightning symbol). Use it for the exe, the window, the tray and the Start menu shortcut.
    2. Version resource: product name "VoltioVolt", file description "VoltioVolt UPS Management", version from the build, copyright "VoltioVolt contributors", license AGPL-3.0-or-later.
    3. Application manifest: per-monitor DPI awareness v2. Confirm what Wails v3 already sets.
4. Keyboard: F1 opens the documentation link. Alt shows the menu access keys. Tab order follows the screen order.
5. Linux: the same style with the system font and the GTK theme colors where possible.

### TASK 03-00-00-006 Fix the Windows test flake

Branch: vlt-is-011/sim-test-pipe.

1. Add a test-only option for the pipe name (Windows) and the socket path (Linux). The product endpoint does not change.
2. Each headless test in cmd/vlt-simulator uses its own name.
3. Run the package tests 50 times on Windows in CI (-count=50). All MUST pass.

### TASK 03-00-00-007 Update the documentation

Branch: vlt-is-011/docs.

1. docs/user/owner-tests.md: change T-14 (TASK 03-00-00-001 step 7). Add T-18: Simple mode and Expert mode. Add T-19: interface size at the Owner screens. Add T-20: live History charts with the simulator.
2. docs/user/simulator.md: no change, except links to the new tests.
3. docs/architecture.md: modes, scale, session rules of OD-50 to OD-52.
4. docs/roadmap.md: add VLT-IS-012 (OD-56).
5. docs/decisions.md: Q-1, OD-50 to OD-56.

---

## 04-00-00 ACCEPTANCE CRITERIA

- The Owner sequence test of TASK 03-00-00-001 step 1 passes, and the General screen variant passes.
- The History charts update without a user action. Hidden series stay hidden. The zoom stays (tests).
- Simple mode shows the content of TASK 03-00-00-003 step 3. The health rules have tests.
- ui.scale changes all sizes at once, without a restart (tests).
- The exe has the icon and the version resource. The menu bar exists on Windows.
- The simulator tests pass 50 times on Windows.
- CI passes for each pull request. The pre-release "test-latest" exists with the new packages.

---

## 05-00-00 REPORT 11 REQUEST

1. Write Report 11 after all tasks. Stop after Report 11.
2. Title: VLT-RPT-011, Revision 00. Markdown. Simplified Technical English.
3. Sections, in this order:
    1. Summary (5 sentences or fewer)
    2. Download (release URL, asset names, SHA-256 values)
    3. Tasks (table: task, status, pull request link)
    4. Tests (short, include the layout results of TASK 03-00-00-004 step 7)
    5. Deviations (include the confirmed cause of R-08 here)
    6. Questions
4. Do not add other sections.

END OF MESSAGE VLT-IS-011
