# Architecture

This file gives the component model of VLT-IS-001 03-10-00 (text only). Status: pre-alpha. Most components do not exist yet.

## 1. Processes

| Process | Function |
|---------|----------|
| vltd | The VLT service. It runs in the background without a user session. It is the only component that talks to the UPS. |
| VLT GUI | The desktop user interface. It shows data. It changes Settings only through vltd. It connects to vltd through local IPC only (named pipe on Windows, Unix domain socket on Linux). |
| VLT Agent | A small client on another computer. It receives shutdown signals from vltd. Release v2.0. |
| vlt-probe, vlt-sim | Diagnostic tools, not vltd (VLT-IS-002 01-30-00). vlt-probe is a READ-only capture tool. vlt-sim is the UPS simulator. |

## 2. Components

| Component | Part of | Function | Package (this repository) |
|-----------|---------|----------|---------------------------|
| Device Manager | vltd | Finds the UPS. Selects the Transport and the Dialect. | not yet |
| Poller | vltd | Sends READ commands on a schedule. | not yet |
| Measurement Store | vltd | Records Measurements. Keeps summaries per minute and per hour. | not yet |
| Event Engine | vltd | Makes Events from state changes. | not yet |
| Analyzer | vltd | Calculates runtime, battery health, power quality (EN 50160-based estimate) and energy. | not yet |
| Policy Engine | vltd | Runs Shutdown Policies. | not yet |
| Notification Router | vltd | Sends Notifications. Runs Escalations. Receives Acknowledgements. | not yet |
| Vault | vltd | Keeps Secrets. | `internal/vault` |
| IPC Server | vltd | Serves the VLT GUI. Checks access. | `internal/ipc`, `internal/ipc/wire`, `internal/ipc/client` |
| Audit Log | vltd | Records security-relevant actions. | `internal/audit` |
| VLT GUI | Desktop | Screens: Dashboard, History, Events, Analysis, Channels, Policies, UPS Settings, Security. | not yet |

Protocol layer (made by VLT-IS-002):

| Part | Function | Package |
|------|----------|---------|
| Transport | The physical link to the UPS: USB HID or RS-232 serial. Sends one request and returns one response. Accepts only READ commands from the command registry. | `internal/transport` |
| Dialect | Command registry, READ encoder and response parsers for the Megatec Q family. | `internal/dialect` |
| Simulator | UPS model, Dialect profiles, fault injection, command recorder, pty endpoint (Linux). For development and CI only (SEC-036). | `internal/sim`, `cmd/vlt-sim` |
| Capture tool | Sends the Probe List or the Watch List and writes a Capture file. | `cmd/vlt-probe` |

## 3. Data flow

UPS -> Transport -> Dialect parser -> Measurement -> Measurement Store and Event Engine -> Policy Engine and Notification Router -> Channels.

## 4. Rules

1. Only vltd talks to the UPS. vlt-probe is used only by the Owner for Captures; vltd and vlt-probe do not run at the same time on one port.
2. In the MVP, vltd has no network listener.
3. The VLT GUI has no Secret and no UPS access. It uses IPC (JSON frames with a 4-byte length prefix, maximum 1 MiB, unknown fields rejected, protocol version field; QA-07). Section 5 gives the IPC.

## 5. IPC (VLT-IS-003 TASK 03-00-00-007)

### 5.1 Frames and messages

1. Frame: a 4-byte big-endian length, then JSON. The length is 1 to 1 MiB. A larger frame gives the error "frame_too_large" and vltd closes the connection.
2. The decoder rejects unknown fields in the request and in the params.
3. Request: `{"v":1,"id":number,"method":text,"params":object}`. Response: `{"v":1,"id":number,"result":object}` or `{"v":1,"id":number,"error":{"code":text,"message":text}}`.
4. Error codes: bad_request, unknown_method, pin_not_set, not_elevated, pin_incorrect, blocked, vault_unavailable, not_found, invalid, rate_limited, frame_too_large, internal.
5. Package `internal/ipc/wire` has the frame code and the message types. The GUI uses `internal/ipc/client` (one Go function for each method). The client does not import the server packages.

### 5.2 Endpoints

| Platform | Endpoint | Access |
|----------|----------|--------|
| Linux | Unix socket /run/vltd/vltd.sock, mode 0660, owner vltd, group vlt. Directory /run/vltd, mode 0750, group vlt. | SO_PEERCRED: uid 0, or a member of the group vlt (primary or supplementary group). |
| Windows | Named pipe \\.\pipe\vltd (github.com/Microsoft/go-winio). | Protected DACL (VLT-IS-004 QI-4): SYSTEM and NT SERVICE\vltd GA; Administrators and the local group "VLT Operators" (if it exists) GRGW. Peer identity: the user SID of the client process token. Remote clients are rejected (PIPE_REJECT_REMOTE_CLIENTS). |

### 5.3 Limits

1. 8 connections or fewer. vltd closes a ninth connection at once.
2. 20 requests each second or fewer for each connection (token bucket). A request above the limit gives the error "rate_limited".
3. After the first byte of a frame, the full frame must arrive in 30 seconds. Otherwise vltd closes the connection. A connection without a partial frame can stay open.

### 5.4 Methods, protocol version 1

Before a PIN is set, only system.hello, system.status, auth.status and auth.set_pin are permitted. Other methods give "pin_not_set". The elevated state is for one connection only (no token). It ends after `auth.session_idle_minutes` without a request, after 8 hours, at auth.logout, at a PIN change and when the connection closes.

| Method | Elevated state needed | Params | Result |
|--------|------------------------|--------|--------|
| system.hello | No | none | version, protocol |
| system.status | No | none | vault {state, reason, key_mode}, store {state, schema_version}, uptime_seconds |
| auth.status | No | none | pin_set, block_until, elevated |
| auth.set_pin | No at first start; Yes after | pin | ok |
| auth.login | No | pin | ok |
| auth.logout | No | none | ok |
| settings.list | No | none | settings [key, value, version, default, updated_at, updated_by] |
| settings.set | Yes | key, value | version |
| secret.status | No | id | id, set, hint, updated_at |
| secret.set | Yes | id, value | ok |
| secret.delete | Yes | id | ok |
| vault.reset | Yes, and confirmation "RESET VAULT" | confirmation | ok |
| audit.list | Yes | after (seq), limit (1 to 500, default 100) | rows, next |

No method returns a Secret value. The test TestAllMethodsNoSecretInResponses calls each method and checks all response bytes.

## 6. vltd start and stop (VLT-IS-003 TASK 03-00-00-008)

### 6.1 Start sequence

1. Logging (Linux: standard error to the journal; Windows: %ProgramData%\VoltioVolt\logs\vltd.log, 10 MiB, 5 files). All records go through the redaction handler.
2. Data directory and permission check (Linux /var/lib/vltd, Windows %ProgramData%\VoltioVolt). If the check fails, vltd stops and writes the reason to the log.
3. Store (SQLite, migrations). Then the permission check of the database files.
4. Vault. If the host binding fails, vltd continues (SEC-090). The Vault state is then "locked" or "not configured".
5. Audit chain verification. A broken chain gives the audit entry "audit_chain_broken". vltd continues.
6. IPC server (section 5).
7. Ready signal: Linux sd_notify READY=1 (Type=notify, github.com/coreos/go-systemd/v22/daemon); Windows service state "running" (golang.org/x/sys/windows/svc).

### 6.2 Stop sequence

The reverse order, 10 seconds maximum: Linux STOPPING=1 or Windows "stop pending", IPC server (all connections closed), Store, logging.

### 6.3 Vault states

| State | Cause | Secrets | Settings |
|-------|-------|---------|----------|
| available | DEK loaded, canary correct, key mode host+tpm2 or tpm2 (Linux) or dpapi (Windows) | read and write | read and write (MAC checked) |
| reduced protection | as "available", but the key mode is host, or /etc/vltd/key-mode is missing or not valid (SEC-005) | read and write | read and write |
| locked | DEK cannot be loaded or the canary is not correct (for example a data directory copied to another computer, SEC-006) | no access | default values, no writes |
| not configured | no DEK (Linux: no credential; Windows: never, vltd makes the DEK) | no access | default values, no writes |

Recovery from "locked": vault.reset with the confirmation "RESET VAULT" deletes all Secrets and, on Windows, makes a new DEK.

### 6.4 Command-line flags

vltd has one flag: --version. It prints the version and exits.


## 7. Monitoring core (VLT-IS-004)

### 7.1 Device Manager (internal/device, TASK 03-00-00-003)

1. Device Settings: `device.transport` (auto, hid or serial; default auto), `device.serial_port` (empty, `/dev/<name>` or `COM<n>`; empty means all serial ports), `device.serial_dtr` (default on), `device.serial_rts` (default off). None of them is a Secret.
2. Candidates with "auto": first the HID devices with VID 0665 and PID 5161, then the serial ports. With "hid": HID only. With "serial": serial only. If `device.serial_port` has a name, only that serial port.
3. On each candidate, the Device Manager sends the Probe List (VLT-IS-002 03-50-01) in order, with 500 ms between requests. A command without a valid response is skipped; the next command follows.
4. Dialect selection: Voltronic if QGS answers, QS if QS answers, Q1 if only Q1 answers. If no Dialect is found, the next candidate follows. If no candidate gives a Dialect, the state is "no device" and a new detection follows after 1, 2, 5, 10, 30, 30 ... seconds.
5. Device profile: Transport, port, Dialect, supported commands, nominal values (QRI, else F), model data (I, QMD, QVFW). The QID value is never kept: the profile shows `serial_number: "[REDACTED]"` if QID answers.
6. States: detecting, no device, connected, comm lost, stopped. `Restart` (device.settings.set) stops the Poller and starts a new detection with the current Settings.

### 7.2 Poller (internal/poller, TASK 03-00-00-003)

| Dialect | Fast group (every 2 s) | Slow group (every 30 s) | Once (at start and every 6 h) |
|---------|------------------------|-------------------------|-------------------------------|
| Voltronic | QGS, QMOD | QS, QBV, QWS, QFLAG | QRI, I, QVFW, QMD |
| QS | QS, QMOD (if supported) | QBV, QWS (if supported) | F, I |
| Q1 | Q1 | none | F, I |

1. A command that did not answer at detection is not polled.
2. The minimum time between two requests is 500 ms. The Poller sends one command at a time. A due fast cycle goes before the next command of a running slow or once cycle.
3. A cycle starts one period after the start of the previous cycle of its group, or at the end of the previous cycle (the later time, VLT-IS-003 QA-2).
4. A fast poll fails if no command of the fast group gives a valid response. After 3 failed fast polls in sequence, the state is "comm lost". The Poller closes the Transport and opens it again after 1, 2, 5, 10, 30, 30 ... seconds. After each open, it sends Q1, QS and QGS and selects the Dialect. If the Dialect is the same, the state is "connected" and all groups start again (the once group too). If the Dialect is different, the Poller stops and the Device Manager starts a new detection.
5. Each cycle goes to the Sink (Measurements, Event Engine) with the time of each request and the parsed value or the error.
6. Only READ commands of the registry: requests come only from dialect.EncodeRead, and transport.Stream refuses all other bytes (SEC-037). The simulator recorder tests check each received command.
7. Time: the Poller and the Device Manager use a Clock interface. Tests use a virtual clock, so 6 hours of polling run in seconds.

## 8. Measurements (internal/measure, VLT-IS-004 TASK 03-00-00-004)

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

1. Tables (migration 0002): `measurements` (raw: ts, name, value, unit, source), `measurements_1m` and `measurements_1h` (ts, name, unit, min, max, mean, count). ts is Unix time in milliseconds (UTC), the start of the period for summaries.
2. A field with no value (the parser gives Valid=false, for example "---") is not stored. vltd never stores zero in its place. NaN and infinite values are refused.
3. All Measurements of one poll cycle are written in one transaction. If one row fails, no row of the cycle is stored.
4. The hourly job (at start and every hour) makes the 1-minute values of all complete minutes from the raw values, and the 1-hour values of all complete hours from the 1-minute values (mean weighted by count). It makes the last summary period again, so a second run gives the same result. Then it deletes raw values older than 7 days and 1-minute values older than 400 days. 1-hour values have no limit.
5. Query (measurements.query): name, range [from, to), step raw, 1m or 1h. Maximum 10000 rows.

## 9. Event Engine (internal/event, VLT-IS-004 TASK 03-00-00-005)

1. Input: one Observation for each poll cycle: the Q1 or QS status bits, the QWS bits, load.percent, ups.temperature and battery.runtime (only the fields in the cycle). The status bit table is in docs/protocol.md section 8.
2. Threshold Events (Settings): OVERLOAD (WARNING) when `event.overload_percent` (default 100) or more; OVER_TEMPERATURE (WARNING) when `event.over_temperature_c` (default 50) or more; RUNTIME_LOW (WARNING) when battery.runtime is less than `event.runtime_low_minutes` (default 5) and ON_BATTERY is active.
3. Hysteresis: an active threshold Event clears only 5 % of the threshold value past the threshold (OVERLOAD below 95 %, OVER_TEMPERATURE below 47.5 °C, RUNTIME_LOW at 5.25 minutes or more, or when not on battery).
4. Debounce: ON_BATTERY and LOW_BATTERY after 1 poll. All other Events and all clear conditions after 2 polls in sequence. A cycle without the input of a condition does not change its counters.
5. Clear: ON_BATTERY clears with the Event ONLINE (INFO). The other conditions clear with an Event of the same type, severity INFO and `"active": false`.
6. COMM_LOST: WARNING, or CRITICAL when ON_BATTERY is active. COMM_RESTORED: INFO. The Poller state gives them (3 failed fast polls).
7. Each Event is stored in the events table (ts in Unix milliseconds, type, severity, data JSON). Each CRITICAL Event is written to the log with level ERROR. Subscribers (IPC push, Policy Engine) get each Event.
8. events.list: newest first, paging by ID (rowid), 1 to 500 rows.

## 10. Policy Engine and Simulation (internal/policy, VLT-IS-004 TASK 03-00-00-006)

WARNING: An error in the Policy Engine can turn off a computer at the wrong time or not turn it off before the battery is empty.

### 10.1 Settings (one Shutdown Policy in the MVP)

| Setting | Default | Range |
|---------|---------|-------|
| policy.action | shutdown | shutdown, hibernate |
| policy.trigger.on_battery_seconds | 300 | 0 (off) to 86400 |
| policy.trigger.runtime_below_minutes | 5 | 0 (off) to 600 |
| policy.trigger.charge_below_percent | 30 | 0 (off) to 100 |
| policy.trigger.low_battery_flag | on | on, off |
| policy.trigger.comm_lost_on_battery_seconds | 180 | 0 (off) to 86400 |
| policy.countdown_seconds | 60 | 10 to 600 |
| policy.cancel_when_mains_returns | on | on, off |
| policy.force_close_applications | on (Windows) | on, off |

### 10.2 Engine

1. Input after each poll cycle and each second: ON_BATTERY, LOW_BATTERY and COMM_LOST from the Event Engine (the last known state during comm lost), battery.charge and battery.runtime.
2. Triggers, in this order: on battery for `on_battery_seconds`; runtime below `runtime_below_minutes` on battery; charge below `charge_below_percent` on battery; LOW_BATTERY with `low_battery_flag` on; comm lost on battery for `comm_lost_on_battery_seconds`. Runtime and charge are not used during comm lost (old values).
3. The first true trigger starts the countdown (`countdown_seconds`). Audit: `policy_countdown_start`.
4. During the countdown: if the mains power returns (ON_BATTERY clear and no comm lost) and `cancel_when_mains_returns` is on, the countdown is cancelled. An operator with the elevated state can cancel (shutdown.cancel), but not while LOW_BATTERY is active. After an operator cancel, the triggers that were true do not start a new countdown until they are false again or the mains power returns; a new trigger starts a new countdown. Audit: `policy_countdown_cancel` with the reason (mains returned, operator, policy inactive), or `rejected`.
5. At the end of the countdown, the Engine calls the executor (Shutdown or Hibernate) one time. Audit: `policy_shutdown_request`. If the call fails, the Engine tries again every 30 seconds while a trigger is true.
6. The Policy Engine is the only caller of the executor (SEC-092). cmd/vltd gives the OS executor (internal/shutdown) to the Engine.

### 10.3 Version and activation (SEC-055, SEC-093)

1. Version: lowercase hex SHA-256 of the canonical JSON of the 9 policy Settings (keys sorted, no spaces).
2. The policy is active only if `policy_state.active_version` is the current version and a passed Simulation of that version is in `policy_simulations`. The Service reads the Settings at each tick, so a change through policy.set or settings.set makes the policy inactive at once. policy.set also deletes the active version.
3. An inactive policy starts no countdown and cancels a running countdown ("policy inactive").

### 10.4 Simulation

1. Scenarios: power_fail_long (mains fails at 10 s), power_fail_short (mains returns at half of the countdown), low_battery (charge 15 % at 30 s, bit 6; an operator cancel is tried and refused), comm_lost_on_battery (comm lost at 20 s).
2. Each scenario uses the simulator model (no Transport), the real Event Engine without a database, a new Engine and the dry-run executor. A virtual clock moves in steps of 2 seconds. No command goes to a UPS and no OS function is called.
3. Result: for each scenario the steps with virtual times (event, trigger, countdown_start, cancel, cancel_refused, shutdown_request), the outcome, the dry-run calls and the real time.
4. Pass: power_fail_long and low_battery give a shutdown request, and power_fail_short gives a cancel. comm_lost_on_battery is reported only.
5. The result and the version are stored in `policy_simulations` (migration 0003).
6. Default policy result: power_fail_long: ON_BATTERY at 10 s, trigger at 310 s, request at 370 s. power_fail_short: cancel at 342 s. low_battery: trigger charge_below_percent at 30 s, request at 90 s. comm_lost_on_battery: trigger at 200 s, request at 260 s.

## 11. Shutdown executor (internal/shutdown, VLT-IS-004 TASK 03-00-00-007)

1. `shutdown.Executor` implements `policy.Executor`: `Shutdown(ctx, reason)` and `Hibernate(ctx, reason)`. cmd/vltd makes it with the Setting policy.force_close_applications and the Audit Log, and gives it to the Policy Engine. The Simulation and all tests use dry-run or fake executors.
2. Windows:
    1. Enable SE_SHUTDOWN_NAME (SeShutdownPrivilege) in the process token with AdjustTokenPrivileges. vlt-setup gives the privilege to the service (security.md section 10).
    2. Shutdown: `InitiateSystemShutdownExW(NULL, message, 0, force_close_applications, FALSE, 0x0006000C)`. Timeout 0: no dialog box. The reason code is SHTDN_REASON_MAJOR_POWER (0x00060000) | SHTDN_REASON_MINOR_ENVIRONMENT (0x0000000C), "Power Failure: Environment", unplanned (source: Microsoft Learn, System Shutdown Reason Codes).
    3. Hibernate: if `IsPwrHibernateAllowed` is false or `SetSuspendState(TRUE, FALSE, FALSE)` fails, the executor writes the audit entry `os_hibernate_fallback` and does a Shutdown.
3. Linux (github.com/godbus/dbus/v5, system bus, one connection for each request):
    1. `ListInhibitors` on org.freedesktop.login1.Manager. The list (what, who, why, mode, uid, pid) goes in the audit entry.
    2. `PowerOff(false)` or `Hibernate(false)`. interactive = false: polkit does not ask; 50-vltd.rules gives the permission to the user vltd.
    3. Hibernate: if `CanHibernate` does not return "yes", the executor writes `os_hibernate_fallback` and calls PowerOff.
4. Audit actions: os_shutdown_request, os_hibernate_request (success or failure, with the error) and os_hibernate_fallback.
5. Other OS: ErrNotSupported.
6. SEC-092 test (`TestOnlyPolicyCallsExecutor`, go/ast): only cmd/vltd imports internal/shutdown; only internal/policy calls `Shutdown(ctx, reason)` or `Hibernate(ctx, reason)`; only internal/shutdown uses godbus, the login1 names and the Windows shutdown functions.
