# VOLTIOVOLT (VLT UPS MANAGEMENT)
## INSTRUCTION SET – MESSAGE 5
## DECISIONS ON REPORT 4 AND VLT GUI (MVP SCREENS)

### DOCUMENT IDENTIFICATION

- Document number: VLT-IS-005
- Revision: 00
- Date: 2026-11-10
- Reference documents: VLT-IS-001 Revision 03. VLT-IS-002 to VLT-IS-004 Revision 00. VLT-RPT-004 Revision 00.
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: vltd fixes, VLT GUI. Release: MVP.
- Type: Build instruction.

---

## 00-00-00 GENERAL

### 00-00-01 Purpose

1. This instruction gives the decisions on VLT-RPT-004.
2. This instruction builds the VLT GUI with the MVP screens. The Owner uses the VLT GUI for the tests T-08 and T-09.
3. Do only the tasks in 03-00-00. Then write Report 5. Refer to 05-00-00.

### 00-00-02 Not in this instruction

- Channels (Telegram, e-mail) and their GUI screens. They come in VLT-IS-006.
- Encoders for CONTROL commands and CONFIG commands.
- Installers (MSI, deb, rpm). Start of the VLT GUI at user logon.
- Analyzer screens (battery health, power quality, energy).
- Releases and tags.

### 00-00-03 Failure rule

CAUTION: If a task fails, do not use a workaround that changes the scope.

1. Stop the failed task.
2. Continue with the tasks that do not depend on it.
3. Write the failure in Report 5.

---

## 01-00-00 DECISIONS

### 01-10-00 Answers to the questions for the Instruction Author

| No. | Decision |
|-----|----------|
| Q-IA-1 | Changed. A safety function MUST NOT depend on the Vault. When the Vault state is "locked", vltd ignores the stored policy Settings and uses the Fallback Policy (TASK 03-00-00-001 step 1). When the Vault state is "not configured", there is no active policy. The first-run wizard sets the policy. |
| Q-IA-2 | Accepted. Clear Events have the same type, severity INFO and "active": false. |
| Q-IA-3 | Accepted. Voltronic fast group: QS, QGS. Slow group: QMOD, QBV, QWS, QFLAG. |
| Q-IA-4 | Accepted. Pass rule for comm_lost_on_battery: a shutdown request at a time equal to or less than (comm lost time + trigger.comm_lost_on_battery_seconds + countdown_seconds + 10 s). |
| Q-IA-5 | Accepted. Methods without elevation need a set PIN. The error codes unavailable, no_simulation, low_battery and not_found are accepted. |

### 01-20-00 Owner decisions

| No. | Subject | Decision |
|-----|---------|----------|
| OD-22 | IPC client for T-09 (Q-O-2) | No command-line client. The VLT GUI of this instruction is the client. |
| OD-23 | Owner tests (Q-O-1) | No change. The Owner does T-01 to T-09, then T-10 (TASK 03-00-00-009). |

### 01-30-00 Changes to VLT-IS-001

These changes make VLT-IS-001 Revision 04.

| Item | Change |
|------|--------|
| 00-10-00 | Add the term "Fallback Policy": a fixed Shutdown Policy in the code. vltd uses it only when the Vault state is "locked". |
| SEC-055 | Add: "The Fallback Policy is an exception. Its Simulation runs in CI." |
| SEC-094 (new) | Do not add a dependency version (Go module or npm package) that was published less than 14 days before the day you add it. |
| SEC-095 (new) | The VLT GUI loads no remote content. All front-end files are in the binary. The Content Security Policy is "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'". No eval. No inline scripts. |
| SEC-096 (new) | The front end never receives a Secret value. A Secret goes from the input field to the Go side of the VLT GUI, then to vltd with secret.set. The Go side clears the buffer after the call. |
| SEC-097 (new) | Required status checks on main use the "strict" mode: a branch must be up to date with main before the merge. |

---

## 02-00-00 GENERAL RULES

1. The rules of VLT-IS-003 02-00-00 stay. Branch name: vlt-is-005/task-name.
2. SEC-086 gives Owner approval to merge each pull request of this instruction.
3. CGO_ENABLED=0 for all code except cmd/vlt-gui on Linux (OD-03).
4. The Builder never starts a real shutdown. The Builder does not run the VLT GUI against an installed vltd.
5. Record each new dependency (Go and npm) in docs/dependencies.md.

---

## 03-00-00 TASKS

### TASK 03-00-00-001 Fix the items of Report 4

Branch: vlt-is-005/fixes.

1. Fallback Policy (Q-IA-1):
    1. Fixed values in the code:
        - action: shutdown
        - triggers: low_battery_flag on, runtime_below_minutes 2, comm_lost_on_battery_seconds 300
        - all other triggers: off
        - countdown_seconds: 30
        - cancel_when_mains_returns: on
        - force_close_applications: on
    2. vltd uses it only in the Vault state "locked". Write an audit entry and a log entry when vltd starts to use it.
    3. policy.get returns the Fallback Policy with the field "fallback": true. policy.set, policy.activate and policy.deactivate return the error "vault_locked" in this state.
    4. Add a CI test that runs the Simulation of the Fallback Policy. Pass rules: low_battery gives a shutdown request; power_fail_short gives a cancel; comm_lost_on_battery gives a shutdown request per the rule of Q-IA-4.
2. Poll groups (Q-IA-3): change the Voltronic row. Update the tests and docs/architecture.md.
3. Simulation pass rule (Q-IA-4): add the rule to the Simulation and to its tests.
4. Ruleset (SEC-097): set strict mode for the required status checks of main with the GitHub API. If the API refuses, write it in Report 5. The Owner then sets it in the GitHub UI.

### TASK 03-00-00-002 Set up the VLT GUI project

Branch: vlt-is-005/gui-base. Directories: cmd/vlt-gui, internal/gui (Go side), web/ (front end).

1. Use Wails v3. Find the latest stable version. Obey SEC-094. Pin the version in go.mod.
2. Front end:
    1. TypeScript, Vite, Preact and uPlot.
    2. No other runtime npm dependency without approval. Development dependencies (test runner, type checker) are permitted. They MUST obey SEC-061 and SEC-094.
    3. Commit package-lock.json. Pin exact versions.
    4. Install with "npm ci --ignore-scripts" only.
3. Content Security Policy per SEC-095. Add a test that loads the built index.html and checks the policy text.
4. Architecture:
    1. The Go side of the VLT GUI uses internal/ipc/client only. It does not import Store, Vault or SQLite packages.
    2. The front end calls Go methods through the Wails bindings. The front end has no direct IPC access.
    3. Push messages from vltd go to the front end as Wails events.
5. CI:
    1. Add a job "gui-linux" on ubuntu-latest with the WebKitGTK and GTK packages that Wails v3 needs. CGO_ENABLED=1 for this job only.
    2. Add a job "gui-windows" on windows-latest.
    3. Add npm checks to the gui jobs: "npm ci --ignore-scripts", type check, front-end tests, "npm audit --audit-level=high" and a license check with the SEC-061 allow list.
    4. Add both gui jobs to the required status checks of main.
    5. Artifacts: vlt-gui for linux-amd64 and windows-amd64, with SHA-256 files. Retention: 14 days.
6. A GUI close does not stop vltd.

### TASK 03-00-00-003 Make the connection, PIN and lock functions

Branch: vlt-is-005/gui-auth.

1. Connection:
    1. At start, the VLT GUI connects to vltd and calls system.hello.
    2. If the connection fails, show the reason. "Access denied" has a help text: Linux, add the user to the group vlt, then log out and log in. Windows, add the user to the group "VLT Operators", then log out and log in.
    3. Reconnect automatically every 5 seconds. Show the connection state at all times.
2. First start (no PIN): show only the "Set PIN" screen. Show the PIN rules of VLT-IS-003 TASK 03-00-00-005.
3. Login: the PIN screen calls auth.login. Show the remaining block time when auth.status gives a block.
4. Lock:
    1. The VLT GUI locks when the elevated state ends (vltd idle time).
    2. A "Lock" button locks at once (auth.logout).
    3. The locked VLT GUI still shows the Dashboard (methods without elevation).
5. Change PIN: in the Security screen, with the elevated state.

### TASK 03-00-00-004 Make the Dashboard and the tray icon

Branch: vlt-is-005/gui-dashboard.

1. Dashboard with status.subscribe:
    1. Mode: mains, battery, bypass, fault or comm lost. Text and icon, not only color.
    2. Values: input voltage and frequency, output voltage and frequency, output current, load %, battery voltage, battery charge %, runtime (minutes), temperature. A value with no data shows "—", not zero.
    3. The last 10 Events.
    4. Countdown banner: trigger, remaining seconds, "Cancel" button. The button needs the elevated state. If LOW_BATTERY is active, the button is off and the banner shows the reason.
    5. Fallback banner: if policy.get gives "fallback": true, show a WARNING banner with the text: "The Vault is locked. VLT uses the Fallback Policy."
2. Tray icon:
    1. States: green (mains), yellow (battery), red (CRITICAL Event active or countdown), grey (no connection to vltd or comm lost).
    2. The icon shape changes for each state. Color is not the only signal.
    3. Tooltip: state and runtime.
    4. Menu: Open, Lock, Quit (Quit closes the VLT GUI only).

### TASK 03-00-00-005 Make the Device, History and Events screens

Branch: vlt-is-005/gui-data.

1. Device screen:
    1. device.status: Transport, port, Dialect, supported commands, nominal values, model data. QID stays redacted.
    2. Device settings (elevated): transport (auto, hid, serial), serial port, DTR, RTS. A change asks for confirmation, then calls device.settings.set.
2. History screen:
    1. uPlot charts with measurements.query.
    2. Ranges: 1 hour (raw), 24 hours (1 min), 7 days (1 min), 30 days (1 h), 1 year (1 h).
    3. Charts: voltages, frequency, load, battery voltage and charge, temperature.
    4. Each chart has a text table view for screen readers.
3. Events screen: events.list with paging. Filter by severity and by type.

### TASK 03-00-00-006 Make the Shutdown Policy screen

Branch: vlt-is-005/gui-policy.

WARNING: This screen controls the automatic shutdown of the computer. An error can turn off a computer at the wrong time.

1. Show policy.get: Settings, version, active state, last Simulation result, fallback state.
2. Edit (elevated): all Settings of VLT-IS-004 TASK 03-00-00-006 step 1. Show the default value and the permitted range next to each field. Reject values outside the range before policy.set.
3. After policy.set, show the text: "The policy is now inactive. Run the Simulation to activate it."
4. Simulation (elevated): call policy.simulate. Show each scenario with its steps and virtual times. Show "passed" or "failed" for each scenario.
5. Activation (elevated): the "Activate" button is on only after a passed Simulation of the current version. Ask for confirmation with the text: "This computer will turn off automatically when the trigger conditions occur."
6. Deactivate (elevated): one button with confirmation.
7. Show shutdown.status at all times.

### TASK 03-00-00-007 Make the Security and General screens

Branch: vlt-is-005/gui-security.

1. Security screen:
    1. Vault state with its reason. For "reduced protection", show the SEC-005 notice.
    2. Vault reset (elevated): the user types "RESET VAULT". Explain that all Secrets are deleted.
    3. Change PIN.
    4. Audit log (elevated): audit.list with paging.
2. General screen:
    1. Language: Turkish or English. Store it in the Setting "ui.language" through settings.set. Default: the OS language if it is Turkish, else English.
    2. Theme: follows the OS (light or dark).
    3. About: VLT version, vltd version, protocol version, license AGPL-3.0-or-later, link to the source code.

### TASK 03-00-00-008 Add language, accessibility and front-end tests

Branch: vlt-is-005/gui-quality.

1. Language:
    1. All user text is in web/i18n/en.json and web/i18n/tr.json.
    2. A test fails if a key is missing in one of the two files.
    3. A test fails if a component has user text that is not from the i18n files.
    4. Turkish text uses the correct Turkish characters (ç, ğ, ı, İ, ö, ş, ü). A review by the Owner is necessary (T-10).
2. Accessibility:
    1. All functions work with the keyboard only. The focus is always visible.
    2. Each control has an accessible name.
    3. Text contrast is 4.5:1 or more in the light theme and in the dark theme.
    4. State is never shown by color only.
3. Tests:
    1. Go side: tests with a fake IPC server for each GUI method. A test proves that no Go method returns a Secret value to the front end (SEC-096).
    2. Front end: component tests for the PIN screen, the countdown banner, the policy form range checks and the activation button rule.

### TASK 03-00-00-009 Update the documentation and the Owner tests

Branch: vlt-is-005/docs. Put the other documentation changes in the pull request of the related task.

1. docs/architecture.md: VLT GUI architecture, Wails bindings, push events, Fallback Policy.
2. docs/security.md: the changes in 01-30-00, the GUI rules of SEC-095 and SEC-096.
3. docs/decisions.md: one row for each decision in 01-00-00.
4. docs/dependencies.md: each new Go module and npm package.
5. docs/user/owner-tests.md (English and Turkish):
    1. Change T-08 and T-09. Use the VLT GUI for all steps.
    2. Add T-10: VLT GUI test. Steps: connect, set PIN, login, lock, Dashboard values, Device screen, History charts, Events, policy edit, Simulation, activation, language change. Review of all Turkish text.
    3. Add to T-02 and T-03: add the Owner user to the group vlt (Linux) or "VLT Operators" (Windows).
6. Change packaging/linux/vlt-dev-init.sh: if SUDO_USER is set, add that user to the group vlt.
7. Change cmd/vlt-setup dev-install: add the user who started the command to the group "VLT Operators".

---

## 04-00-00 ACCEPTANCE CRITERIA

- CI passes on Linux and Windows for each pull request, including gui-linux and gui-windows.
- CGO_ENABLED=0 go build ./... passes for all packages except cmd/vlt-gui.
- The Fallback Policy Simulation passes in CI.
- The CSP test of TASK 03-00-00-002 step 3 passes.
- The SEC-096 test of TASK 03-00-00-008 step 3.1 passes.
- The i18n tests of TASK 03-00-00-008 step 1 pass.
- npm audit gives no high or critical item. The npm license check passes.
- No dependency violates SEC-094. Give the publish dates in Report 5.
- CI makes vlt-gui artifacts for Linux and Windows with SHA-256 files.
- gitleaks finds no Secret in the full history.
- The token check of SEC-088 shows "absent" at the end of each task.

---

## 05-00-00 REPORT 5 REQUEST

### 05-00-01 General

1. Write Report 5 after all tasks.
2. Stop after Report 5. Do not start work that is not in this instruction.

### 05-10-00 Report format

1. Title: VLT-RPT-005, Revision 00.
2. Format: Markdown. Simplified Technical English.
3. Sections: the same 13 sections as VLT-RPT-004, in the same order. Section 7 "Research results" contains the Wails v3 version check and the list of WebKitGTK and GTK packages for Linux, with sources.
4. Add 3 screenshots of the VLT GUI (Dashboard, Shutdown Policy, Security) from the simulator in a fake IPC test setup, if the Builder computer permits it. Write "not possible" with the reason if not.
5. Do not add other sections.

END OF MESSAGE VLT-IS-005
