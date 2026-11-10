# VOLTIOVOLT (VLT UPS MANAGEMENT)
## INSTRUCTION SET – MESSAGE 4
## DECISIONS ON REPORT 3 AND MONITORING CORE: DEVICE MANAGER, POLLER, MEASUREMENTS, EVENTS, SHUTDOWN POLICY, SIMULATION, OS SHUTDOWN

### DOCUMENT IDENTIFICATION

- Document number: VLT-IS-004
- Revision: 00
- Date: 2026-11-10
- Reference documents: VLT-IS-001 Revision 02. VLT-IS-002 Revision 00. VLT-IS-003 Revision 00. VLT-RPT-003 Revision 00.
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: vltd monitoring core and shutdown. Release: MVP.
- Type: Build instruction.

---

## 00-00-00 GENERAL

### 00-00-01 Purpose

1. This instruction gives the decisions on VLT-RPT-003.
2. This instruction builds the monitoring core of vltd and the safe shutdown of the local computer.
3. Do only the tasks in 03-00-00. Then write Report 4. Refer to 05-00-00.

### 00-00-02 Not in this instruction

- Encoders for CONTROL commands and CONFIG commands. UPS output off, UPS restart, UPS tests and beeper control come in a later instruction.
- Channels (Telegram, e-mail, desktop notifications), VLT GUI, Analyzer, installers (MSI, deb, rpm).
- Device facts of the Reference UPS. The Owner Capture is not available yet. Use the simulator and the test vectors only.
- Releases and tags.

### 00-00-03 Failure rule

CAUTION: If a task fails, do not use a workaround that changes the scope.

1. Stop the failed task.
2. Continue with the tasks that do not depend on it.
3. Write the failure in Report 4.

---

## 01-00-00 DECISIONS

### 01-10-00 Answers to the questions for the Instruction Author

| No. | Decision |
|-----|----------|
| QI-1 | Accepted with changes. Refer to TASK 03-00-00-001. The fallback unit starts only when vltd.service enters the "failed" state. The primary unit uses Restart=on-failure, RestartSec=5, StartLimitBurst=3, StartLimitIntervalSec=60 and RestartPreventExitStatus=243. The variable name is VLT_FALLBACK_MODE=1. The Vault reason text is "primary unit failed". |
| QI-2 | Per source. The lockout key is the actor and the peer identity: the Linux uid of the peer, the Windows user SID of the peer, and later the Telegram chat ID. There is no global block. If the failures of all sources are more than 100 in one hour, write a SECURITY_ALERT audit entry. |
| QI-3 | Accepted. The polkit rule includes power-off-ignore-inhibit and hibernate-ignore-inhibit. vltd records the active inhibitors in the audit entry before each shutdown. |
| QI-4 | Changed. Pipe DACL: SYSTEM GA, NT SERVICE\vltd GA, Administrators GRGW, VLT Operators GRGW. |
| QI-5 | Yes. In this instruction, the development setup tool makes the group and gives the privileges (TASK 03-00-00-002). The MVP installer uses the same code later. |
| QI-6 | Accepted. A Setting never contains a Secret. A test MUST prove that no Setting key has the Secret type. |

### 01-20-00 Owner decisions

| No. | Subject | Decision |
|-----|---------|----------|
| OD-20 | GitHub token (QO-3) | No change. OD-19 stays. |
| OD-21 | Owner tests (QO-1, QO-2, QO-4) | The Owner does them with docs/user/owner-tests.md (TASK 03-00-00-009). |

### 01-30-00 Changes to VLT-IS-001

These changes make VLT-IS-001 Revision 03.

| Item | Change |
|------|--------|
| 01-00-05 item 6 | vltd can read the variable VLT_FALLBACK_MODE. It is a state flag, not a Setting and not a Secret. |
| SEC-024 | New text: "Count incorrect PIN entries per source (actor and peer identity). Block that source only." |
| SEC-092 (new) | vltd sends a shutdown request to the OS only from the Policy Engine. No IPC method starts a real shutdown directly. |
| SEC-093 (new) | Any change of a Shutdown Policy makes the policy inactive. A new Simulation is necessary before activation (SEC-055). |
| 03-30-00 | Add the Events AVR_ACTIVE (INFO), UPS_SHUTDOWN_ACTIVE (CRITICAL) and UPS_WARNING_BITS_CHANGED (INFO). |

---

## 02-00-00 GENERAL RULES

1. The rules of VLT-IS-003 02-00-00 stay. Branch name: vlt-is-004/task-name.
2. SEC-086 gives Owner approval to merge each pull request of this instruction.
3. Build all code with CGO_ENABLED=0.
4. The Builder never starts a real shutdown. All tests use a fake shutdown executor.
5. Record each new dependency in docs/dependencies.md.

---

## 03-00-00 TASKS

### TASK 03-00-00-001 Fix the items of Report 3

Branch: vlt-is-004/fixes.

1. Lockout per source (QI-2). Change internal/auth and the IPC peer data. Keep the counters in the Store. Tests:
    1. Source A is blocked.
    2. Source B can still log in.
    3. A restart does not reset the block of source A.
2. Pipe DACL (QI-4). Change internal/ipc/endpoint_windows.go. Add a test that parses the SDDL text and checks each ACE.
3. systemd fallback (QI-1):
    1. Change packaging/linux/vltd.service. Add Restart=on-failure, RestartSec=5, RestartPreventExitStatus=243 and OnFailure=vltd-fallback.service. Add StartLimitBurst=3 and StartLimitIntervalSec=60 in [Unit].
    2. Add packaging/linux/vltd-fallback.service. It has the same ExecStart, user, groups and hardening as vltd.service. It has no LoadCredentialEncrypted= line. It has Conflicts=vltd.service and Environment=VLT_FALLBACK_MODE=1.
    3. If VLT_FALLBACK_MODE=1, vltd sets the Vault state "locked" with the reason "primary unit failed". vltd does not try to read a DEK.
    4. Test in Go: start vltd in the test process with VLT_FALLBACK_MODE=1. The Vault state is "locked". The reason is correct.
4. polkit rule (QI-3). Add packaging/linux/50-vltd.rules. Actions: power-off, power-off-multiple-sessions, power-off-ignore-inhibit, hibernate, hibernate-multiple-sessions, hibernate-ignore-inhibit. Subject: user vltd only. vlt-dev-init.sh copies the file with owner root and mode 0644.
5. Settings type test (QI-6). Add a test that fails if a Setting key is registered with the Secret type.

### TASK 03-00-00-002 Make the Windows development setup tool

Branch: vlt-is-004/setup. Directory: cmd/vlt-setup (Windows only).

1. Subcommands: dev-install and dev-uninstall. No other subcommands. No flags for Settings.
2. dev-install steps:
    1. Make the local group "VLT Operators" with NetLocalGroupAdd. If it exists, continue.
    2. Make the service vltd with the virtual account NT SERVICE\vltd.
    3. Give SeShutdownPrivilege to the SID of NT SERVICE\vltd with LsaAddAccountRights.
    4. Set the required privileges of the service with ChangeServiceConfig2 and SERVICE_CONFIG_REQUIRED_PRIVILEGES_INFO. List: SeShutdownPrivilege and SeChangeNotifyPrivilege. Verify this list with a Microsoft source. Add other privileges only with a written reason.
    5. Make the data directory with the ACL of VLT-IS-003 TASK 03-00-00-003.
    6. Set the service recovery: restart after the first and the second failure, after 5 seconds.
    7. Start the service.
3. dev-uninstall: stop and remove the service. Remove the user right. Ask before it deletes the data directory. Do not delete the group if it has members.
4. Change packaging/windows/dev-install.ps1 and dev-uninstall.ps1. They only check for Administrator rights and then start vlt-setup.
5. CI artifacts: add vltd and vlt-setup for windows-amd64, and vltd for linux-amd64, with SHA-256 files. Retention: 14 days. This is not a release.

### TASK 03-00-00-003 Make the Device Manager and the Poller

Branch: vlt-is-004/poller. Directories: internal/device and internal/poller.

1. Device Settings (through IPC): transport (auto, hid or serial), serial port name, serial DTR and RTS. Default: auto.
2. Detection with "auto":
    1. Try HID devices with VID 0665 and PID 5161 first.
    2. Then try the serial ports.
    3. On each Transport, send the Probe List (VLT-IS-002 03-50-01) in order. Stop at the first command that does not return a valid response, then continue with the next command.
    4. Select the Dialect from the results. Order: Voltronic (QGS answers), QS (QS answers), Q1 (Q1 answers only).
    5. Record the device profile: Transport, port, Dialect, supported commands, nominal values (QRI or F), model data (I, QMD, QVFW). Redact QID in all logs and IPC results.
3. Poll groups:

| Dialect | Fast group (every 2 s) | Slow group (every 30 s) | Once (at start and every 6 h) |
|---------|------------------------|-------------------------|-------------------------------|
| Voltronic | QGS, QMOD | QS, QBV, QWS, QFLAG | QRI, I, QVFW, QMD |
| QS | QS, QMOD (if supported) | QBV, QWS (if supported) | F, I |
| Q1 | Q1 | none | F, I |

4. Timing: the minimum time between two requests is 500 ms (VLT-IS-002 TASK 03-00-00-006 step 3). The fast group has priority over the slow group.
5. Communication loss: after 3 failed fast-group polls in sequence, the state is "comm lost". Reconnect with waits of 1, 2, 5, 10 and 30 seconds, then 30 seconds each time. After reconnect, verify that the Dialect is the same.
6. Send only READ commands from the registry (SEC-037). A test with the simulator recorder proves this.
7. Tests with the simulator:
    1. Detection for each Dialect profile.
    2. Poll timing.
    3. Communication loss and reconnect.
    4. Fault injection of VLT-IS-002 TASK 03-00-00-007 step 4.

### TASK 03-00-00-004 Write the Measurements

Branch: vlt-is-004/measurements. Directory: internal/measure.

1. Measurement names and units:

| Name | Unit | Source |
|------|------|--------|
| input.voltage | V | Q1, QS, QGS |
| input.fault_voltage | V | Q1, QS |
| input.frequency | Hz | Q1, QS, QGS |
| output.voltage | V | Q1, QS, QGS |
| output.frequency | Hz | QGS |
| output.current | A | QGS |
| load.percent | % | Q1, QS, QGS |
| battery.voltage | V | QS, QGS, QBV; Q1 as received (VLT-IS-002 TASK 03-00-00-005 step 4f) |
| battery.charge | % | QBV |
| battery.runtime | min | QBV |
| ups.temperature | °C | Q1, QS, QGS |

2. A field with no value is not stored. Do not store zero for it.
3. Write all Measurements of one poll cycle in one transaction.
4. Retention:
    1. Raw values: 7 days.
    2. 1-minute values (minimum, maximum, mean): 400 days.
    3. 1-hour values (minimum, maximum, mean): no limit.
    4. A job makes the summaries and deletes old raw values every hour.
5. Tests: summary values, retention, no-value fields.

### TASK 03-00-00-005 Make the Event Engine

Branch: vlt-is-004/events. Directory: internal/event.

1. UPS state from Q1 or QS status bits (public Megatec document):

| Bit | Condition | Event |
|-----|-----------|-------|
| 7 = 1 | Utility fail | ON_BATTERY (WARNING). Back to 0: ONLINE (INFO). |
| 6 = 1 | Battery low | LOW_BATTERY (CRITICAL) |
| 5 = 1 and bit 3 = 0 | Bypass active (on-line type) | BYPASS_ACTIVE (WARNING) |
| 5 = 1 and bit 3 = 1 | Boost or buck active (standby type) | AVR_ACTIVE (INFO) |
| 4 = 1 | UPS failed | UPS_FAULT (CRITICAL) |
| 2 = 1 | Test in progress | TEST_STARTED (INFO) |
| 1 = 1 | Shutdown active | UPS_SHUTDOWN_ACTIVE (CRITICAL) |

2. QMOD and the QGS bits are UNVERIFIED. Record them as Measurement data. Do not make Events from them. Exception: a change of the QWS value makes UPS_WARNING_BITS_CHANGED (INFO) with the old and the new value.
3. Threshold Events (Settings with defaults):

| Event | Default condition |
|-------|-------------------|
| OVERLOAD | load.percent 100 or more |
| OVER_TEMPERATURE | ups.temperature 50 °C or more |
| RUNTIME_LOW | battery.runtime less than 5 minutes, when on battery |

4. Each threshold Event has a hysteresis of 5 % of the threshold value before it clears.
5. Debounce:
    1. ON_BATTERY and LOW_BATTERY: after 1 poll.
    2. All other Events and all clear conditions: after 2 polls in sequence.
6. COMM_LOST: WARNING. When the last known state is ON_BATTERY: CRITICAL. COMM_RESTORED: INFO.
7. Store each Event in the events table. Write each CRITICAL Event to the log.
8. Tests: each row of the tables above, debounce, hysteresis, COMM_LOST on battery.

### TASK 03-00-00-006 Make the Shutdown Policy and the Simulation

Branch: vlt-is-004/policy. Directory: internal/policy.

WARNING: An error in this package can turn off a computer at the wrong time or not turn it off before the battery is empty.

1. One Shutdown Policy in the MVP. Settings with defaults:

| Setting | Default |
|---------|---------|
| action | shutdown (other value: hibernate) |
| trigger.on_battery_seconds | 300 (0 = off) |
| trigger.runtime_below_minutes | 5 (0 = off) |
| trigger.charge_below_percent | 30 (0 = off) |
| trigger.low_battery_flag | on |
| trigger.comm_lost_on_battery_seconds | 180 (0 = off) |
| countdown_seconds | 60 (minimum 10, maximum 600) |
| cancel_when_mains_returns | on |
| force_close_applications (Windows) | on |

2. The first trigger that is true starts the countdown. During the countdown:
    1. If mains power returns and cancel_when_mains_returns is on, cancel the countdown.
    2. An operator can cancel it through IPC with the elevated state.
    3. If LOW_BATTERY is active, an operator cancel is not possible.
3. At the end of the countdown, the Policy Engine calls the shutdown executor (TASK 03-00-00-007).
4. Write an audit entry for: countdown start, cancel (with the reason), shutdown request.
5. Activation (SEC-055 and SEC-093):
    1. A policy can be active only after a passed Simulation of its current version.
    2. The policy version is a SHA-256 value of its Settings.
    3. A change of a Setting makes the policy inactive.
6. Simulation:
    1. Scenarios: power_fail_long, power_fail_short (mains returns during the countdown), low_battery, comm_lost_on_battery.
    2. Use a virtual clock. A scenario finishes in less than 5 seconds of real time.
    3. Use a dry-run executor. The Simulation does not send a command to the UPS and does not call the OS.
    4. The result is a list of steps with virtual times: Event, trigger, countdown start, cancel, shutdown request.
    5. The Simulation passes when power_fail_long and low_battery give a shutdown request, and power_fail_short gives a cancel.
    6. Store the result and the policy version.
7. Tests: each trigger, each cancel path, activation rules, all scenarios.

### TASK 03-00-00-007 Make the OS shutdown executor

Branch: vlt-is-004/shutdown. Directory: internal/shutdown.

1. Interface: Shutdown(reason) and Hibernate(reason). Fake executor for tests.
2. Windows:
    1. Enable SE_SHUTDOWN_NAME with AdjustTokenPrivileges.
    2. Shutdown: InitiateSystemShutdownEx with a message, a timeout of 0, the force_close_applications Setting, no restart, and a shutdown reason code for a power failure. Find the correct reason code in a Microsoft source.
    3. Hibernate: SetSuspendState with hibernate = TRUE. If hibernation is not available, use Shutdown and write an audit entry.
3. Linux:
    1. Use the system D-Bus (github.com/godbus/dbus/v5, if its license is in SEC-061).
    2. Before the call, list the active inhibitors (ListInhibitors). Write them in the audit entry.
    3. Call PowerOff(false) or Hibernate(false) on org.freedesktop.login1.Manager.
    4. If CanHibernate does not return "yes", use PowerOff and write an audit entry.
4. The executor is used only by the Policy Engine (SEC-092). Add a go/ast test like VLT-IS-003 TASK 03-00-00-004 step 8.
5. Tests: fake executor; Windows reason code constant; Linux D-Bus call with a fake bus connection.

### TASK 03-00-00-008 Add the IPC methods

Branch: vlt-is-004/ipc. Directories: internal/ipc, internal/ipc/wire, internal/ipc/client.

1. Push messages: add the frame type {"v":1,"push":text,"data":object}. A client gets push messages only after status.subscribe.
2. New methods:

| Method | Elevated state needed | Result |
|--------|------------------------|--------|
| device.status | No | Device profile (QID redacted), connection state |
| device.settings.set | Yes | Result only; restarts detection |
| status.subscribe | No | Push "status" every 2 s, push "event" for each Event, push "countdown" each second during a countdown |
| status.unsubscribe | No | Result only |
| measurements.query | No | Values for a name, a time range and a step (raw, 1 min, 1 h) |
| events.list | No | Events with paging |
| policy.get | No | Settings, version, active state, last Simulation result |
| policy.set | Yes | New version; policy is inactive |
| policy.simulate | Yes | Simulation result |
| policy.activate | Yes | Result; error if no passed Simulation |
| policy.deactivate | Yes | Result only |
| shutdown.status | No | Countdown state, remaining seconds, trigger |
| shutdown.cancel | Yes | Result; error if LOW_BATTERY is active |

3. No method starts a real shutdown (SEC-092).
4. Add the new methods to the no-Secret response test of VLT-IS-003 TASK 03-00-00-007 step 7.
5. Integration test: vltd with the simulator Transport. Subscribe, make a power failure in the simulator, receive the Events and the countdown, check the fake executor call.

### TASK 03-00-00-009 Write the Owner test plan

Branch: vlt-is-004/owner-tests. File: docs/user/owner-tests.md, in English and in Turkish.

Write each test with: purpose, preconditions, steps, expected result, what to send to the Instruction Author.

| No. | Test | Source |
|-----|------|--------|
| T-01 | vlt-probe Capture on the Reference UPS (probe, and watch with the mains test). On Linux if possible, because Windows has no raw report descriptor. | VLT-IS-002 TASK 03-00-00-008 |
| T-02 | vltd development install on Linux, with TPM2 and without TPM2 if possible | docs/user/dev-service.md |
| T-03 | vltd development install on Windows with vlt-setup | TASK 03-00-00-002 |
| T-04 | SEC-006 copy test (the data directory on a second computer) | VLT-RPT-003 7.5 |
| T-05 | Linux credential with one changed byte: vltd-fallback.service runs, Vault "locked" | TASK 03-00-00-001 step 3 |
| T-06 | Windows DPAPI canary after a service restart and after a computer restart | VLT-RPT-003 7.5 item 5 |
| T-07 | polkit: CanPowerOff and CanHibernate as the user vltd | VLT-RPT-003 7.4 item 7 |
| T-08 | Policy Simulation through the integration test client or a later GUI | TASK 03-00-00-006 |
| T-09 | Real shutdown with the Reference UPS | TASK 03-00-00-006 and 007 |

For T-09, write this WARNING before the steps:

"WARNING: This test turns off the computer. Save all work. Close all applications. Make sure that the UPS battery is fully charged. Set trigger.on_battery_seconds to 30. Disconnect the UPS input power. Connect the input power again after the computer is off."

### TASK 03-00-00-010 Update the documentation

Put each change in the pull request of the related task.

1. docs/architecture.md: Device Manager, Poller, poll groups, Measurements, Event Engine, Policy Engine, Simulation, shutdown executor, push messages.
2. docs/security.md: the changes in 01-30-00, SEC-092, SEC-093, the lockout per source, the pipe DACL.
3. docs/protocol.md: the status bit table of TASK 03-00-00-005 step 1, with the source.
4. docs/decisions.md: one row for each decision in 01-00-00.
5. docs/dependencies.md: each new module.

---

## 04-00-00 ACCEPTANCE CRITERIA

- CI passes on Linux and Windows for each pull request.
- CGO_ENABLED=0 go build ./... passes.
- The simulator recorder test proves that the Poller sends only READ commands from the registry.
- All Event table rows of TASK 03-00-00-005 have a passing test.
- The Simulation passes for the default policy. A changed policy is inactive until a new Simulation passes.
- The go/ast test of TASK 03-00-00-007 step 4 passes.
- The integration test of TASK 03-00-00-008 step 5 passes on Linux and Windows.
- No test starts a real shutdown. Give the evidence in Report 4.
- docs/user/owner-tests.md exists in English and Turkish.
- gitleaks finds no Secret in the full history.
- The token check of SEC-088 shows "absent" at the end of each task.

---

## 05-00-00 REPORT 4 REQUEST

### 05-00-01 General

1. Write Report 4 after all tasks.
2. Stop after Report 4. Do not start work that is not in this instruction.

### 05-10-00 Report format

1. Title: VLT-RPT-004, Revision 00.
2. Format: Markdown. Simplified Technical English.
3. Sections: the same 13 sections as VLT-RPT-003, in the same order. Section 7 "Research results" contains the source checks of TASK 03-00-00-002 step 2.4 and TASK 03-00-00-007 step 2.2.
4. Do not add other sections.

END OF MESSAGE VLT-IS-004
