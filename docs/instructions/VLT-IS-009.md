# VOLTIOVOLT (VLT UPS MANAGEMENT)
## INSTRUCTION SET – MESSAGE 9
## OWNER TEST FIXES (ROUND 1): PIN UNLOCK DEFECT, COMPACT INTERFACE, HELP TEXT, EXAMPLES, DEMO MODE

### DOCUMENT IDENTIFICATION

- Document number: VLT-IS-009
- Revision: 00
- Date: 2026-11-13
- Reference documents: VLT-IS-001 Revision 07. VLT-IS-002 to VLT-IS-008 Revision 00. VLT-RPT-007 and VLT-RPT-008 Revision 00.
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: VoltioVolt (portable mode), VLT GUI, auth, test suite. Release: MVP.
- Type: Build instruction.

---

## 00-00-00 GENERAL

### 00-00-01 Owner test results (Windows, portable package 68fdd43)

| No. | Result |
|-----|--------|
| R-01 | VoltioVolt.exe connects to the Reference UPS on COM3 and shows live values. PASS. |
| R-02 | DEFECT: after the Owner set a PIN, "Unlock" did not work with the correct PIN. The Owner deleted the data folder to continue. |
| R-03 | Without a PIN, VLT works correctly. |
| R-04 | DEFECT: the first-run wizard is too large. The page scrolls down. The text and the buttons are too large. |
| R-05 | DEFECT: the setup screens have no help text and no example values. Example: the field "Telegram bot token" does not tell how to get a token. |
| R-06 | The shutdown tests (T-09, T-12) were not done. The Owner uses the computer for work. |

### 00-00-02 Purpose

1. Find and fix the cause of R-02.
2. Add a PIN reset that does not need a deletion of the data folder.
3. Make the VLT GUI and the wizard compact (R-04).
4. Add help text, example values and official links to all setup fields (R-05).
5. Add a demo mode with simulator data.
6. Fix the unstable test of VLT-RPT-008 section 5 item 5.
7. Do the tasks in the order of 03-00-00. Then write Report 9.

### 00-00-03 Not in this instruction

- The Reference UPS profile. It needs the Capture of T-01.
- New Channels and new functions other than the items in 03-00-00.

---

## 01-00-00 DECISIONS

| No. | Subject | Decision |
|-----|---------|----------|
| OD-40 | PIN reset | A "Forgot PIN" function exists in portable mode. It removes the PIN, deletes all Secrets (Vault reset), and makes the Shutdown Policy inactive. Settings, Measurements and Events stay. It writes an audit entry. If a Channel stays configured after the reset, no message is possible, because the Secrets are deleted. |
| OD-41 | Wizard start | At the first start, VLT shows a small dialog: "Start the setup wizard?" with "Start" and "Later". "Later" opens the Dashboard. The wizard is also available in the General screen. |
| OD-42 | Links | Help text can have links to official documentation of the vendor only (Telegram, Google, Microsoft). A link opens in the default browser of the user. |
| OD-43 | Demo mode | A device choice "Demo (simulator)". The OS shutdown executor and all CONTROL commands are dry runs in demo mode. |

---

## 02-00-00 GENERAL RULES

1. Branch name: vlt-is-009/task-name. One task, one pull request. Squash merge after CI passes.
2. SEC-086 gives Owner approval to merge each pull request of this instruction.
3. The rules of VLT-IS-007 01-30-00 ("These rules stay") and SEC-102 stay.
4. All new user text is in en.json and tr.json. Write the Turkish text in natural Turkish with the correct characters (ç, ğ, ı, İ, ö, ş, ü).

---

## 03-00-00 TASKS

### TASK 03-00-00-001 Fix the PIN unlock defect (R-02)

Branch: vlt-is-009/pin-fix.

1. Reproduce the defect first:
    1. Write an end-to-end test with the portable engine (in-process connection) and the GUI Go side: set a PIN in the wizard, lock, unlock with the same PIN.
    2. Write the same test for these cases: a PIN with digits only, a PIN with letters, a PIN set in the General screen, a lock by the "Lock" button, a lock by idle time, a restart of VoltioVolt between the set and the unlock.
    3. Also check the front end: the value that the PIN field sends (for example spaces, a trimmed value, a number type that removes leading zeros, an IME or keyboard layout problem).
2. Find the root cause. Write it in Report 9 section 5 with the file and the line.
3. Fix the cause. All tests of step 1 MUST pass.
4. If the lockout block (5 incorrect entries) was a part of the problem, show the remaining block time on the unlock screen clearly.

### TASK 03-00-00-002 Add "Forgot PIN" (OD-40)

Branch: vlt-is-009/pin-reset.

1. The unlock screen has a link "Forgot PIN".
2. It shows this text: "This removes the PIN and deletes all saved passwords and tokens (Telegram, e-mail). The automatic shutdown becomes inactive. Your settings and your history stay."
3. The user types "RESET" to confirm.
4. Then VLT starts the wizard at the step "PIN".
5. Unit test and end-to-end test.

### TASK 03-00-00-003 Make the VLT GUI compact (R-04, OD-41)

Branch: vlt-is-009/compact-ui.

1. Base values:

| Item | Value |
|------|-------|
| Base font size | 14 px |
| Heading sizes | 20 px (screen title), 16 px (section title) |
| Button height | 32 px |
| Input height | 32 px |
| Space between form rows | 8 to 12 px |
| Minimum window size | 1024 × 680 px |

2. Forms: the label and the field are on one row when the window is wide enough. Use two columns for short fields.
3. Wizard:
    1. The wizard is a dialog of 760 × 560 px, centered on the main window.
    2. Each step fits in the dialog without a scroll bar at 100 % display scale. If a step has more content, divide it into two steps.
    3. A step indicator at the top shows all steps and the current step.
    4. The buttons "Back", "Skip" and "Next" are at the bottom right at all times.
4. First start dialog (OD-41): "Start the setup wizard?" with "Start" and "Later". Do not show it again after the wizard is completed or after "Later" two times. The General screen keeps the button "Start the setup wizard".
5. Apply the same values to all screens: Dashboard, Device, History, Events, Shutdown Policy, UPS Control, Notifications, Security, General.
6. Screenshots: make screenshots of each wizard step and of each screen at 1280 × 720 in CI with the demo mode (TASK 03-00-00-005), if possible with the existing tools. Attach them to the release assets as a zip file "screenshots.zip". If it is not possible, write the reason in Report 9.

### TASK 03-00-00-004 Add help text, example values and official links (R-05, OD-42)

Branch: vlt-is-009/help.

1. Each setup field has:
    1. A short help line under the field (one sentence).
    2. An example value as a placeholder (a fake value for a Secret).
    3. A "?" button that opens a longer help text in a small panel, when the field needs it.
2. Content for the fields:

| Field or screen | Help content |
|-----------------|--------------|
| Serial port | What a COM port is. How to find it: Windows Device Manager, "Ports (COM & LPT)". Example: COM3. Linux example: /dev/ttyUSB0. |
| DTR and RTS | "Keep the default (unchanged) unless the UPS does not answer." |
| Port busy | The existing help text (UPSilon 2000 and RupsMon). |
| Shutdown Policy, each trigger | What the trigger does, with an example. Example for trigger.on_battery_seconds: "300 = shut down after 5 minutes on battery." |
| countdown_seconds | "Time to save your work before the shutdown." |
| ups_off settings | The existing WARNING, plus an example timeline: "Power fails, after 5 min the computer shuts down, 2 min later the UPS turns off, 1 min later it turns on again." |
| Minimum severity | What INFO, WARNING and CRITICAL mean, with one example Event for each. |
| Telegram bot token | Steps: (1) In Telegram, open @BotFather. (2) Send /newbot. (3) Give a name and a user name that ends with "bot". (4) BotFather sends the token. Copy it here. Example format: 123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ (fake). Link: https://core.telegram.org/bots/tutorial |
| Telegram pairing | Steps: open your bot in Telegram, send "/start" and the 6-digit code. The chat appears in the list. Then send /durum to test. |
| E-mail, Gmail preset | Steps: turn on 2-Step Verification, make an app password, paste the 16-character app password (not the normal password). Example user: name@gmail.com. Link: https://support.google.com/accounts/answer/185833 |
| E-mail, other servers | Example values: server, port 465 (TLS) or 587 (STARTTLS). "Ask your e-mail provider for these values." |
| PIN | The existing text of VLT-IS-008 TASK 03-00-00-003 step 2. |
| Battery estimate settings | "Used only when the UPS does not report the charge. Keep the defaults unless you know the battery data." |
| Modem question | The existing text. |
| Data folder | Where it is. What it contains. "Make a copy of this folder to keep your history." |

3. A link opens in the default browser of the user. Use only the links in the table, or other links on core.telegram.org, support.google.com and support.microsoft.com.
4. Add a CI step (allowed to fail without a block of the merge) that checks that each link returns HTTP 200. Write the result in Report 9.

### TASK 03-00-00-005 Add the demo mode (OD-43)

Branch: vlt-is-009/demo.

1. Device screen and wizard: a transport choice "Demo (simulator)".
2. In demo mode:
    1. The engine uses the simulator. The values change slowly. A button "Simulate power failure" switches the simulator to battery for 60 seconds.
    2. The OS shutdown executor is a dry run. A countdown that ends shows "Demo: the computer would shut down now."
    3. All CONTROL commands are dry runs.
    4. A yellow banner "DEMO" is on all screens.
    5. Notifications are sent with the prefix "[DEMO]".
3. A change from demo mode to a real port asks for confirmation. Measurements and Events of the demo stay marked "demo". The History and Events screens hide them by default.
4. Tests: no real shutdown and no CONTROL bytes in demo mode.

### TASK 03-00-00-006 Fix the unstable test

Branch: vlt-is-009/flaky.

1. Find the cause of the unstable result of TestUPSControlIntegration ("the cancelled test has no result", VLT-RPT-008 section 5 item 5).
2. Fix the cause. Do not only increase a timeout.
3. Run the test 50 times on Linux and on Windows in CI (-count=50). All MUST pass.

### TASK 03-00-00-007 Update the documentation

Branch: vlt-is-009/docs.

1. docs/user/owner-tests.md (English and Turkish): add T-14: PIN set, lock, unlock, restart, unlock, "Forgot PIN". Add T-15: wizard and screens at 1280 × 720 and at the display scale of the Owner. Add T-16: demo mode.
2. docs/decisions.md: OD-40 to OD-43.

---

## 04-00-00 ACCEPTANCE CRITERIA

- All PIN tests of TASK 03-00-00-001 step 1 pass. Report 9 gives the root cause.
- Each wizard step fits in 760 × 560 px without a scroll bar (screenshots, or the reason why not).
- Each field of TASK 03-00-00-004 step 2 has its help line and its example value.
- Demo mode makes no real shutdown and sends no CONTROL bytes (tests).
- TestUPSControlIntegration passes 50 times on Linux and on Windows.
- CI passes for each pull request. The pre-release "test-latest" exists with the new packages.

---

## 05-00-00 REPORT 9 REQUEST

1. Write Report 9 after all tasks. Stop after Report 9.
2. Title: VLT-RPT-009, Revision 00. Markdown. Simplified Technical English.
3. Sections, in this order:
    1. Summary (5 sentences or fewer)
    2. Download (release URL, asset names, SHA-256 values)
    3. Tasks (table: task, status, pull request link)
    4. Tests (short)
    5. Deviations (include the root cause of R-02 here)
    6. Questions
4. Do not add other sections.

END OF MESSAGE VLT-IS-009
