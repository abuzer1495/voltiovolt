# Architecture

This file gives the component model of VLT-IS-001 03-10-00 (text only), with the portable mode of VLT-IS-008. Portable mode is the default mode (OD-35). Service mode stays in the code and is not in the default package (OD-36). Section 17 describes portable mode.

## 1. Processes

| Process | Function |
|---------|----------|
| vltd | The VLT service. It runs in the background without a user session. It is the only component that talks to the UPS. |
| VLT GUI | The desktop user interface. It shows data. It changes Settings only through vltd. It connects to vltd through local IPC only (named pipe on Windows, Unix domain socket on Linux). |
| VLT Agent | A small client on another computer. It receives shutdown signals from vltd. Release v2.0. |
| vlt-probe, vlt-sim | Diagnostic tools, not vltd (VLT-IS-002 01-30-00). vlt-probe is a READ-only capture tool. vlt-sim is the UPS simulator. |
| VoltioVolt (portable, default) | One application: VoltioVolt.exe (Windows) and voltiovolt (Linux). It contains the VLT GUI and the monitoring engine in one process (section 17, VLT-IS-008). |

The default package contains VoltioVolt and vlt-probe only. The processes vltd and VLT GUI stay in the repository and in CI. They are not in the default package.

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
| CONTROL | vltd | Makes and sends CONTROL commands (tests, beeper, shutdown and restore) with a Grant, and verifies them (section 14, SEC-098). | `internal/control` |
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

1. Only the monitoring engine talks to the UPS (vltd in service mode; the engine inside VoltioVolt in portable mode). vlt-probe is used only by the Owner for Captures; the engine and vlt-probe do not run at the same time on one port.
2. In the MVP, the engine has no network listener. Portable mode does not open a named pipe or a Unix socket (section 17).
3. The VLT GUI has no Secret and no UPS access. It uses IPC (JSON frames with a 4-byte length prefix, maximum 1 MiB, unknown fields rejected, protocol version field; QA-07). Section 5 gives the IPC. In portable mode the same IPC runs over an in-process connection (net.Pipe).

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
| Portable (default) | In-process: `ipc.PipeListener` and `net.Pipe`. Address: "in-process". No named pipe and no Unix socket. | `ipc.InProcessCheck`: the peer is the same process (`PeerInProcess`). No group and no DACL check. |

### 5.3 Limits

1. 8 connections or fewer. vltd closes a ninth connection at once.
2. 20 requests each second or fewer for each connection (token bucket). A request above the limit gives the error "rate_limited".
3. After the first byte of a frame, the full frame must arrive in 30 seconds. Otherwise vltd closes the connection. A connection without a partial frame can stay open.

### 5.4 Methods, protocol version 1

Before a PIN is set, only system.hello, system.status, auth.status and auth.set_pin are permitted. Other methods give "pin_not_set". The elevated state is for one connection only (no token). It ends after `auth.session_idle_minutes` without a request, after 8 hours, at auth.logout, at a PIN change and when the connection closes. In portable mode, if no PIN is set, the engine gives the elevated state to the in-process connection at all times (OD-37). Secret fields stay write-only.

| Method | Elevated state needed | Params | Result |
|--------|------------------------|--------|--------|
| system.hello | No | none | version, protocol |
| system.status | No | none | vault {state, reason, key_mode}, store {state, schema_version}, uptime_seconds |
| auth.status | No | none | pin_set, block_until, elevated |
| auth.set_pin | No at first start; Yes after | pin | ok |
| auth.login | No | pin | ok |
| auth.logout | No | none | ok |
| settings.list | No | none | settings [key, value, version, default, updated_at, updated_by] |
| settings.set | Yes; a key with the prefix "ui." needs a set PIN only (VLT-IS-006 Q-IA-1). Each call writes an audit entry. | key, value | version |
| secret.status | No | id | id, set, hint, updated_at |
| secret.set | Yes | id, value | ok |
| secret.delete | Yes | id | ok |
| vault.reset | Yes, and confirmation "RESET VAULT" | confirmation | ok |
| audit.list | Yes | after (seq), limit (1 to 500, default 100) | rows, next |
| device.status | No | none | state, since, config, profile (serial_number "[REDACTED]") |
| device.settings.set | Yes | settings {device.transport, device.serial_port, device.serial_dtr, device.serial_rts, device.serial.allowed_ports} | ok; detection starts again |
| device.ports | No | none | ports [name, is_usb, vid, pid, product]; opens no port (VLT-IS-006) |
| device.scan | Yes, and confirmation "SCAN PORTS" | ports (1 to 8 names of the OS list), confirmation | results [name, result, valid_q1]; sends only "Q1" CR to each marked port (VLT-IS-006) |
| ups.control.capabilities | No | none | connected, functions (test.10s, test.minutes, test.until_low, test.cancel, beeper), last_test (result TEST_PASSED or TEST_FAILED, start, end, ups_failed, battery_low, cancelled) (VLT-IS-006) |
| ups.test.start | Yes, and confirmation "START TEST" (type 10s or minutes) or "TEST UNTIL LOW" (type until_low) | type, minutes (1 to 99), confirmation | function, bytes, sends, verified. Refused when the UPS is on battery (on_battery), when a countdown runs (countdown_active) or when no UPS is connected (no_ups) (VLT-IS-006) |
| ups.test.cancel | Yes | none | function, bytes, sends, verified (VLT-IS-006) |
| ups.beeper.set | Yes | on | function, bytes, sends, verified, no_send (b0 was already the wanted state) (VLT-IS-006) |
| status.subscribe | No | none | ok; then push frames |
| status.unsubscribe | No | none | ok |
| measurements.query | No | name, from, to (RFC 3339), step (raw, 1m, 1h) | name, unit, step, rows [ts, min, max, mean, count, source] |
| events.list | No | before (id, 0 = newest), limit (1 to 500, default 100) | events [id, ts, type, severity, data], next |
| policy.get | No | none | settings, version, active, fallback, last_simulation |
| policy.set | Yes | settings {key: value} (short or full policy keys) | the new policy (inactive), or "vault_locked" |
| policy.simulate | Yes | none | Simulation result |
| policy.activate | Yes | none | ok, or "no_simulation", or "vault_locked" |
| policy.deactivate | Yes | none | ok, or "vault_locked" |
| shutdown.status | No | none | countdown {active, trigger, started, ends, remaining_seconds, action, cancelable, requested} |
| shutdown.cancel | Yes | none | ok, or "low_battery" (LOW_BATTERY active), or "not_found" (no countdown) |

"No" in the table means that a PIN must be set, but the elevated state is not necessary. A method of a vltd part that does not run gives "unavailable". No method calls the shutdown executor (SEC-092); shutdown.cancel can only stop a countdown.

No method returns a Secret value. The test TestAllMethodsNoSecretInResponses calls each method and checks all response bytes.

## 6. vltd start and stop (VLT-IS-003 TASK 03-00-00-008)

### 6.1 Start sequence

1. Logging (Linux: standard error to the journal; Windows: %ProgramData%\VoltioVolt\logs\vltd.log, 10 MiB, 5 files). All records go through the redaction handler.
2. Data directory and permission check (Linux /var/lib/vltd, Windows %ProgramData%\VoltioVolt). If the check fails, vltd stops and writes the reason to the log. This permission check applies to the service mode only (VLT-IS-008 01-20-00). Portable mode does not check or change the ACL of the data folder (section 17.2).
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
| available | DEK loaded, canary correct, key mode host+tpm2 or tpm2 (Linux service), dpapi (Windows, service or portable) or secret-service (Linux portable) | read and write | read and write (MAC checked) |
| reduced protection | as "available", but the key mode is host, or /etc/vltd/key-mode is missing or not valid (SEC-005, service mode), or the key mode is file (Linux portable: the DEK is in a 0600 file because the Secret Service is not available) | read and write | read and write |
| locked | DEK cannot be loaded or the canary is not correct (for example a data directory copied to another computer, SEC-006) | no access | default values, no writes |
| not configured | no DEK (Linux: no credential; Windows: never, vltd makes the DEK) | no access | default values, no writes |

Recovery from "locked": vault.reset with the confirmation "RESET VAULT" deletes all Secrets and, on Windows, makes a new DEK.

### 6.4 Command-line flags

vltd has one flag: --version. It prints the version and exits.


## 7. Monitoring core (VLT-IS-004)

### 7.1 Device Manager (internal/device, TASK 03-00-00-003)

1. Device Settings (changed by VLT-IS-006 TASK 03-00-00-004, OD-27, SEC-099): `device.transport` (not_configured, auto, hid or serial; default not_configured), `device.serial_port` (empty, `/dev/<name>` or `COM<n>`; the port that the user selected), `device.serial.allowed_ports` (list of up to 16 port names for "auto"; default empty), `device.serial_dtr` and `device.serial_rts` (unchanged, on or off; default unchanged). None of them is a Secret. A stored DTR or RTS value of the old type (true or false) fails the check, so vltd uses the default.
2. Candidates (`device.ListerFor`): "not_configured": none; the Device Manager does not call the lister and sends no byte (state "not configured"). "hid": the HID devices with VID 0665 and PID 5161. "serial": only `device.serial_port`; if it is empty, none. "auto": the HID devices with VID 0665 and PID 5161, then only the ports in `device.serial.allowed_ports`. The lister never takes the serial port list of the OS for a candidate.
3. On each candidate, the Device Manager sends the Probe List in order (VLT-IS-006 order: Q1, F, I, QS, QGS, QMOD, QBV, QWS, QFLAG, QRI, QID, QVFW, QMD), with 500 ms between requests and a response timeout of 1500 ms. Dialects: megatec (Q1 answers), voltronic-qs (QS answers), voltronic (QGS answers); the first match in the order voltronic, voltronic-qs, megatec is used. A command without a valid response is skipped; the next command follows.
4. Dialect selection: voltronic if QGS answers, voltronic-qs if QS answers, megatec if only Q1 answers. If no Dialect is found, the next candidate follows. If no candidate gives a Dialect, the state is "no device" and a new detection follows after 1, 2, 5, 10, 30, 30 ... seconds.
5. Device profile: Transport, port, Dialect, supported commands, nominal values (QRI, else F), model data (I, QMD, QVFW). The QID value is never kept: the profile shows `serial_number: "[REDACTED]"` if QID answers.
6. States: not configured, detecting, no device, port busy, connected, comm lost, stopped. "port busy": a serial port did not open because another program uses it (go.bug.st/serial PortBusy: Linux EBUSY, Windows ERROR_ACCESS_DENIED); the GUI shows help to stop UPSilon 2000 and the RupsMon service. `Restart` (device.settings.set) stops the Poller and starts a new detection with the current Settings.
7. Serial lines: with "unchanged", the Transport gives no initial line values to go.bug.st/serial and does not call SetDTR or SetRTS. On Linux the library then does not change the lines. On Windows the library v1.8.0 always sets DTR and RTS on at open (docs/protocol.md P-121).
8. Port scan (`device.Scanner`, device.ports and device.scan): device.ports lists the serial ports of the OS with the USB product, VID and PID (no USB serial number) and opens no port. device.scan (elevated, confirmation "SCAN PORTS") accepts 1 to 8 marked ports that are in the OS list, opens each one with the current DTR and RTS Settings, sends only "Q1" CR and returns "response" (with valid_q1), "no response", "busy" or "error". The port of the running Poller is not opened ("busy"). Each call writes the audit entry device_scan.

### 7.2 Poller (internal/poller, TASK 03-00-00-003)

| Dialect | Fast group (every 2 s) | Slow group (every 30 s) | Once (at start and every 6 h) |
|---------|------------------------|-------------------------|-------------------------------|
| voltronic | QS, QGS (VLT-IS-005) | QMOD, QBV, QWS, QFLAG (VLT-IS-005) | QRI, I, QVFW, QMD |
| voltronic-qs | QS, QMOD (if supported) | QBV, QWS (if supported) | F, I |
| megatec | Q1 | none | F, I |

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
| battery.charge_estimate | % | Q1 battery voltage and F rated battery voltage, megatec Dialect only (`measure.ChargeEstimate`; Settings battery.cell_empty_v, battery.cell_full_v, battery.block_empty_v, battery.block_full_v). Not used by the Shutdown Policy. The Dashboard shows "estimate". |
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
| policy.ups_off.enabled | off | on, off (VLT-IS-006) |
| policy.ups_off.delay_minutes | 2 | 1 to 10 whole minutes (SEC-100) |
| policy.ups_off.restore_minutes | 1 | 1 to 9999 |

### 10.2 Engine

1. Input after each poll cycle and each second: ON_BATTERY, LOW_BATTERY and COMM_LOST from the Event Engine (the last known state during comm lost), battery.charge and battery.runtime.
2. Triggers, in this order: on battery for `on_battery_seconds`; runtime below `runtime_below_minutes` on battery; charge below `charge_below_percent` on battery; LOW_BATTERY with `low_battery_flag` on; comm lost on battery for `comm_lost_on_battery_seconds`. Runtime and charge are not used during comm lost (old values).
3. The first true trigger starts the countdown (`countdown_seconds`). Audit: `policy_countdown_start`.
4. During the countdown: if the mains power returns (ON_BATTERY clear and no comm lost) and `cancel_when_mains_returns` is on, the countdown is cancelled. An operator with the elevated state can cancel (shutdown.cancel), but not while LOW_BATTERY is active. After an operator cancel, the triggers that were true do not start a new countdown until they are false again or the mains power returns; a new trigger starts a new countdown. Audit: `policy_countdown_cancel` with the reason (mains returned, operator, policy inactive), or `rejected`.
5. At the end of the countdown, the Engine calls the executor (Shutdown or Hibernate) one time. Audit: `policy_shutdown_request`. If the call fails, the Engine tries again every 30 seconds while a trigger is true.
6. The Policy Engine is the only caller of the executor (SEC-092). cmd/vltd gives the OS executor (internal/shutdown) to the Engine.
7. UPS output off step (VLT-IS-006 TASK 03-00-00-006). WARNING: An incorrect delay can turn off the power before the computer has shut down. Data can be lost. At the end of the countdown, when `ups_off.enabled` is on: (1) the Engine sends the shutdown and restore command through internal/control with the Policy Grant (delay `ups_off.delay_minutes`, restore `ups_off.restore_minutes`; example delay 2, restore 1: "S02R0001" CR); (2) the Controller verifies b1 = 1 (section 14.2); (3) the Engine calls the executor, also when the send or the verification fails. Audit: `policy_ups_off` (success or failure, with the bytes, the sends and the verification result), then `policy_shutdown_request`. In vltd the step and the executor call run in a new goroutine, because Tick runs also in the Poller goroutine and the verification waits for the next polls. The UPS turns off its output `delay` minutes after the command; the computer must shut down in less time. A retry after a failed executor call does not send the command again.

### 10.3 Version and activation (SEC-055, SEC-093)

1. Version: lowercase hex SHA-256 of the canonical JSON of the policy Settings (keys sorted, no spaces). The 3 ups_off Settings are in the JSON only when one of them is not the default (VLT-IS-006): the version of a VLT-IS-005 policy does not change at the upgrade, and a change of an ups_off Setting gives a new version (SEC-093).
2. The policy is active only if `policy_state.active_version` is the current version and a passed Simulation of that version is in `policy_simulations`. The Service reads the Settings at each tick, so a change through policy.set or settings.set makes the policy inactive at once. policy.set also deletes the active version.
3. An inactive policy starts no countdown and cancels a running countdown ("policy inactive").

### 10.4 Simulation

1. Scenarios: power_fail_long (mains fails at 10 s), power_fail_short (mains returns at half of the countdown), low_battery (charge 15 % at 30 s, bit 6; an operator cancel is tried and refused), comm_lost_on_battery (comm lost at 20 s).
2. Each scenario uses the simulator model (no Transport), the real Event Engine without a database, a new Engine and the dry-run executor. A virtual clock moves in steps of 2 seconds. No command goes to a UPS and no OS function is called.
3. Result: for each scenario the steps with virtual times (event, trigger, countdown_start, cancel, cancel_refused, ups_off, ups_off_failed, shutdown_request), the outcome, the dry-run calls and the real time. With `ups_off.enabled` on, the step `ups_off` has the virtual time of the send, the command bytes (detail) and `output_off_after_seconds`. The dry run sends no command: the Simulation uses `control.DryShutdownRestore`, which only makes the bytes. A failed ups_off step fails the scenario.
4. Pass: power_fail_long and low_battery give a shutdown request, and power_fail_short gives a cancel. comm_lost_on_battery (VLT-IS-005 Q-IA-4): a shutdown request at a virtual time equal to or less than comm lost time (20 s) + trigger.comm_lost_on_battery_seconds + countdown_seconds + 10 s. If trigger.comm_lost_on_battery_seconds is 0 (off), comm_lost_on_battery is reported only.
4a. On battery, the charge of the model decreases by 0.1 % for each virtual second (VLT-IS-005). Without it, a policy without an on-battery time trigger (the Fallback Policy) never starts a countdown in power_fail_short. The default policy results do not change.
5. The result and the version are stored in `policy_simulations` (migration 0003).
5a. Fallback Policy result (CI test `TestFallbackSimulation`): power_fail_long: request at 838 s (no pass rule). power_fail_short: LOW_BATTERY at 810 s, cancel at 826 s. low_battery: request at 60 s. comm_lost_on_battery: request at 350 s (limit 360 s).
6. Default policy result: power_fail_long: ON_BATTERY at 10 s, trigger at 310 s, request at 370 s. power_fail_short: cancel at 342 s. low_battery: trigger charge_below_percent at 30 s, request at 90 s. comm_lost_on_battery: trigger at 200 s, request at 260 s.

### 10.5 Fallback Policy (VLT-IS-005 TASK 03-00-00-001, Q-IA-1)

1. The Fallback Policy has fixed values in the code (`policy.FallbackConfig`): action shutdown; trigger.low_battery_flag on; trigger.runtime_below_minutes 2; trigger.comm_lost_on_battery_seconds 300; all other triggers off (0); countdown_seconds 30; cancel_when_mains_returns on; force_close_applications on; ups_off.enabled off (VLT-IS-006, test TestFallbackUPSOffOff).
2. vltd uses the Fallback Policy only when the Vault state is "locked". It is active without a stored Simulation; its Simulation runs in CI.
3. When vltd starts to use it, vltd writes the audit entry `policy_fallback_start` (actor system) and a WARN log entry. When the Vault state is not "locked" again, vltd writes `policy_fallback_end` and uses the stored policy with its active state.
4. policy.get returns the Fallback Policy with `"fallback": true`. policy.set, policy.activate and policy.deactivate return the error code `vault_locked`. policy.simulate runs the Simulation of the Fallback Policy.
5. In the Vault state "not configured", no policy is active.

## 11. Shutdown executor (internal/shutdown, VLT-IS-004 TASK 03-00-00-007)

1. `shutdown.Executor` implements `policy.Executor`: `Shutdown(ctx, reason)` and `Hibernate(ctx, reason)`. cmd/vltd makes it with the Setting policy.force_close_applications and the Audit Log, and gives it to the Policy Engine. The Simulation and all tests use dry-run or fake executors.
2. Windows:
    1. Enable SE_SHUTDOWN_NAME (SeShutdownPrivilege) in the process token with AdjustTokenPrivileges. A nonzero return with GetLastError = ERROR_NOT_ALL_ASSIGNED means that the token does not have the privilege; the executor then gives an error and does not call InitiateSystemShutdownEx. vlt-setup gives the privilege to the service (security.md section 10).
    2. Shutdown: `InitiateSystemShutdownExW(NULL, message, 0, force_close_applications, FALSE, 0x0006000C)`. Timeout 0: no dialog box. The reason code is SHTDN_REASON_MAJOR_POWER (0x00060000) | SHTDN_REASON_MINOR_ENVIRONMENT (0x0000000C), "Power Failure: Environment", unplanned (source: Microsoft Learn, System Shutdown Reason Codes).
    3. Hibernate: if `IsPwrHibernateAllowed` is false or `SetSuspendState(TRUE, FALSE, FALSE)` fails, the executor writes the audit entry `os_hibernate_fallback` and does a Shutdown.
3. Linux (github.com/godbus/dbus/v5, system bus, one connection for each request):
    1. `ListInhibitors` on org.freedesktop.login1.Manager. The list (what, who, why, mode, uid, pid) goes in the audit entry.
    2. `PowerOff(false)` or `Hibernate(false)`. interactive = false: polkit does not ask; 50-vltd.rules gives the permission to the user vltd.
    3. Hibernate: if `CanHibernate` does not return "yes", the executor writes `os_hibernate_fallback` and calls PowerOff.
4. Audit actions: os_shutdown_request, os_hibernate_request (success or failure, with the error) and os_hibernate_fallback.
5. Other OS: ErrNotSupported.
6. SEC-092 test (`TestOnlyPolicyCallsExecutor`, go/ast): only cmd/vltd imports internal/shutdown; only internal/policy calls `Shutdown(ctx, reason)` or `Hibernate(ctx, reason)`; only internal/shutdown uses godbus, the login1 names and the Windows shutdown functions.

## 12. Push messages and the vltd wiring (VLT-IS-004 TASK 03-00-00-008)

### 12.1 Push messages

1. Frame: `{"v":1,"push":text,"data":object}`, with no id. A connection gets push frames only after status.subscribe, until status.unsubscribe or the end of the connection.
2. Types:
    1. "status", every 2 seconds: ts, device (device.status), ups (on_battery, low_battery, comm_lost, active Events), measurements (the last value of each name), countdown, policy_active.
    2. "event", one for each Event (ts, type, severity, data).
    3. "countdown", every second during a countdown (shutdown.status data).
    4. "notification", one for each desktop Notification of the Notification Router (VLT-IS-007, section 15.1).
3. The server keeps 64 push frames for each connection. If a client is slow and the queue is full, new push frames for that client are dropped; requests and other clients are not blocked. Responses and push frames are written one at a time on the connection.
4. The client (internal/ipc/client) reads all frames in one goroutine. Responses go to the waiting call; push frames go to `Pushes()` (256 frames; more are dropped and counted).

### 12.2 vltd wiring

1. Start step 5a: Event Engine (thresholds from the Settings), Policy Service (with the OS executor from main, SEC-092), Device Manager with device.OSLister, the hourly Measurement job, and a 1-second tick.
2. Each poll cycle: Measurements (one transaction), Event Engine, then a Policy tick. A connection state change (comm lost, connected) goes to the Event Engine (COMM_LOST, COMM_RESTORED) and to the Policy Engine.
3. The 1-second tick runs the Policy Engine (on battery time, countdown end) and sends the push messages.
4. Stop: IPC, then the Device Manager, the Poller and the tick, then the Store.
5. Integration test TestPowerFailureIntegration (cmd/vltd): vltd with the simulator Transport (Q1 profile) and a fake executor. The test sets the PIN, sets countdown_seconds to 10, runs the Simulation, activates the policy, subscribes, and sets mains off and charge 15 % in the simulator. It checks the ON_BATTERY and LOW_BATTERY Events, the countdown and status push frames, the refused cancel, one fake executor call, the stored Events and Measurements, and that the simulator got only READ commands. It runs on Linux and Windows. No real shutdown.

## 13. VLT GUI (VLT-IS-005)

### 13.1 Parts

| Part | Directory | Notes |
|------|-----------|-------|
| Wails application | cmd/vlt-gui | Separate Go module (cmd/vlt-gui/go.mod) with Wails v3.1.0 (VLT-IS-007). Linux: cgo, GTK 4 and WebKitGTK 6.0. Windows: WebView2, no cgo. Build tag `production` (no dev server, no devtools). Embeds the built front end (cmd/vlt-gui/frontend/dist). Service mode only. Not in the default package. |
| Portable application | cmd/voltiovolt | Separate Go module. The same Wails front end (copied to cmd/voltiovolt/frontend/dist). It starts the monitoring engine in the same process and connects through net.Pipe (section 17). |
| Go side | internal/gui | Root module, CGO_ENABLED=0. Imports internal/ipc/client and internal/ipc/wire only (no Store, Vault, SQLite or policy package; TestNoStoreImports). Manager: connection, reconnect every 5 s, push messages, lock timer, tray state. API: the bound methods. |
| Front end | web/ | TypeScript, Vite, Preact, uPlot. No other runtime npm package. |

### 13.2 Bindings and events

1. The front end calls `internal/gui.API` methods with the Wails runtime (`Call.ByName("github.com/abuzer1495/voltiovolt/internal/gui.API.<Method>")`). The runtime is served by the VLT GUI at /wails/runtime.js. The front end has no IPC access.
2. A method error is a code (for example `not_elevated`, `vault_locked`, `not_connected`). The front end shows the text of the code from the i18n files.
3. vltd push messages go to the front end as Wails events: `vlt:status`, `vlt:event`, `vlt:countdown`. GUI state events: `vlt:conn`, `vlt:auth`, `vlt:tray`.
4. Service mode: a close of the VLT GUI (window close or tray Quit) closes the IPC connection only. vltd continues. Portable mode: a close of the window hides it and the engine continues. The first time, the text is: "VoltioVolt continues to monitor the UPS in the tray." The tray menu "Quit" stops the engine after the confirmation: "If you quit, VoltioVolt stops the monitoring and the automatic shutdown."
5. Each bound method has a test on a fake IPC server (internal/gui/methods_test.go, TestEachMethodWithFakeServer). The test fails if a new method has no test case.

| Method (internal/gui.API) | vltd method | Elevated |
|---------------------------|-------------|----------|
| Connection, About, Last, Rules, SetTrayTexts, OpenSource | none | no |
| SystemStatus | system.status | no |
| AuthStatus, SetPIN, Login, Logout | auth.status, auth.set_pin, auth.login, auth.logout | SetPIN for a change only |
| DeviceStatus, SetDeviceSettings, DevicePorts, DeviceScan | device.status, device.settings.set, device.ports, device.scan | set and scan only |
| UPSControlCapabilities, UPSTestStart, UPSTestCancel, UPSBeeperSet | ups.control.capabilities, ups.test.start, ups.test.cancel, ups.beeper.set | test, cancel and beeper only; no shutdown function (SEC-098) |
| Measurements | measurements.query (in parts of 9000 rows) | no |
| Events | events.list | no |
| Policy, SetPolicy, Simulate, ActivatePolicy, DeactivatePolicy | policy.get, policy.set, policy.simulate, policy.activate, policy.deactivate | all except Policy |
| ShutdownStatus, CancelShutdown | shutdown.status, shutdown.cancel | cancel only |
| SecretStatus, SetSecret | secret.status, secret.set | set only |
| ResetVault, Audit | vault.reset, audit.list | yes |
| Settings, SetLanguage | settings.list, settings.set (ui.language) | no (ui.language needs a set PIN only) |

### 13.3 Push events and the lock timer

1. The Manager subscribes (status.subscribe) after each connect and sends each push message to the front end as a Wails event. A new Event goes to the top of the Event list of the Dashboard. The front end does not ask vltd again.
2. Each vltd request in the elevated state starts the idle time of vltd again. For this reason the VLT GUI sends no background requests. The Manager records the time of the last request. It sets a timer to that time plus auth.session_idle_minutes plus 2 s (or to the 8-hour limit), and then calls auth.status. When the elevated state ends, the GUI shows the locked state. A locked GUI still shows the Dashboard.
3. Connection: system.hello on each connect. If the connection fails, the GUI shows the reason and tries again every 5 s. Access denied: the GUI shows help (Linux: group vlt; Windows: group "VLT Operators"; then log out and log in).
4. Tray: the icon has a different shape for each state (mains, battery, countdown, fault, no connection), so the state is not shown by color only. The menu texts come from the front end (i18n).

### 13.4 Screens

| Screen | Contents |
|--------|----------|
| Set PIN / Login | First start: only "Set PIN" with the rules. Login shows the block time. |
| Dashboard | Mode with icon and text, values, countdown banner with Cancel, Fallback banner, last Events. |
| Device | device.status; port list with the USB description, "Scan" (WARNING, typed text "SCAN PORTS", elevated), "Use this port" (elevated, confirmation); port busy help; device settings (elevated). |
| UPS Control | ups.control.capabilities; buttons Test 10 seconds, Test for minutes (1 to 99), Test until battery low (WARNING, typed text "TEST UNTIL LOW"), Cancel test, Beeper on, Beeper off. Each button needs the elevated state and the function in the capabilities. The tests ask for a confirmation and send "START TEST". The last test result (TEST_PASSED or TEST_FAILED) with its time. No shutdown button (VLT-IS-006). |
| History | uPlot charts for 1 h, 24 h, 7 d, 30 d, 1 y; a text table for each chart. |
| Events | events.list with paging, filter by severity and type. |
| Shutdown Policy | policy.get, edit with default and range per field, Simulation with steps, Activate only after a passed Simulation of the current version, Deactivate, shutdown.status. VLT-IS-006: the 3 ups_off Settings with their ranges, the WARNING "An incorrect delay can turn off the power before the computer has shut down. Data can be lost." and the text with the delay and restore values. |
| Security | Vault state and reason, SEC-005 notice for "reduced protection", Vault reset (type "RESET VAULT"), Change PIN, audit log with paging. |
| General | Language (tr/en, Setting ui.language), theme follows the OS, About. |

### 13.5 Fallback Policy in the VLT GUI

1. When policy.get gives `"fallback": true`, the GUI shows a WARNING banner on all screens and the Fallback Policy values on the Shutdown Policy screen. Edit and Activate are off. The vltd error `vault_locked` has its own text.
2. The Fallback Policy is defined in section 10.5. Its Simulation runs in CI (`TestFallbackSimulation`).

### 13.6 Security of the VLT GUI

1. SEC-095: the front end is embedded in the binary. The VLT GUI sends the CSP as an HTTP header for each asset; index.html also has the CSP meta element. Tests: web/test/csp.test.ts (built index.html), cmd/vlt-gui/main_test.go (header). The link to the source code opens in the OS web browser, never in the GUI window.
2. SEC-096: see docs/security.md section 13.
3. Language: all user text is in web/i18n/en.json and web/i18n/tr.json. Tests: web/test/i18n.test.ts.
4. Accessibility: web/test/a11y.test.tsx (contrast, focus, names, keyboard).

## 14. CONTROL commands (internal/control, VLT-IS-006 TASK 03-00-00-005)

WARNING: CONTROL commands can cut power to the protected equipment.

### 14.1 Encoders

| Function | Bytes | Grant | Status |
|----------|-------|-------|--------|
| test.10s | T CR | IPC, confirmation "START TEST" | STATIC, PUBLIC |
| test.minutes | "T" + nn (01 to 99) + CR | IPC, confirmation "START TEST" | STATIC, PUBLIC |
| test.until_low | TL CR | IPC, confirmation "TEST UNTIL LOW" | STATIC, PUBLIC |
| test.cancel | CT CR | IPC, no confirmation | STATIC, PUBLIC |
| beeper | Q CR (toggle) | IPC, no confirmation | STATIC, PUBLIC |
| shutdown.restore | delay less than 60 s: "S." + digit (delay / 6 s) + "R" + restore with 4 digits + CR; delay 1 to 10 min: "S" + minutes with 2 digits + "R" + restore with 4 digits + CR | Policy Engine | STATIC, PUBLIC (VLT-REF-001 5.1) |
| shutdown.cancel | C CR | Policy Engine; never sent | PUBLIC only, UNVERIFIED |

1. The bytes are the same in the Dialects megatec, voltronic-qs and voltronic.
2. SEC-100: the delay is 12 to 54 s in steps of 6 s (".2" to ".9") or 1 to 10 whole minutes. The restore time is 1 to 9999 minutes. There is no encoder for a shutdown without restore, and no input gives "S.0". `control.Check` refuses all other bytes; `transport.Stream.Control` calls it again before the write. Tests: TestNeverSendsS0, TestNoShutdownWithoutRestore, TestShutdownNegative, TestEncoderVectors, TestShutdownDelaySteps, TestCheck.
3. SEC-098: only internal/control makes CONTROL bytes. Each encoder needs a `control.Grant`. `control.IPCGrant` (elevated state and the confirmation text; test, cancel test and beeper only) is used only in internal/ipc. `control.PolicyGrant` (shutdown and restore) is used only in internal/policy. The go/ast test TestOnlyControlMakesControlBytes fails for a Grant outside these packages, an encoder call or a Grant or Command literal outside internal/control, or CONTROL bytes in a non-test file outside internal/control. TestNoIPCShutdown fails if internal/ipc or internal/gui uses the shutdown functions, or if a package outside internal/control uses the cancel shutdown function.
4. `ups.control.capabilities` gives `control.Capabilities(dialect)`: the test, cancel test and beeper functions. The shutdown functions are never in the list. No IPC method sends the shutdown and restore command or the cancel shutdown command: `ipc.UPSControl` has no shutdown function, and only the Policy Engine sends the shutdown and restore command (SEC-098). The IPC handlers (internal/ipc/ups.go) make the IPC Grant in the elevated state with the confirmation text and write the audit entries `ups_test_start`, `ups_test_cancel` and `ups_beeper_set` (success, failure or rejected). Error codes: on_battery, countdown_active, not_verified, not_supported, no_ups, control_busy. Tests: TestUPSControlIntegration, TestUPSControlAccess, TestUPSTestRejectedDuringCountdown (internal/ipc), TestUPSControlVltd (cmd/vltd); the methods are in TestAllMethodsNoSecretInResponses.

### 14.2 Send and verification

1. The Poller implements `control.Link`. It sends a CONTROL command in place of the next fast-group poll; the next fast-group poll follows one period (2 s) later. The minimum time between two requests stays 500 ms. With more than one request, the order is: shutdown, cancel test, test, beeper (VLT-REF-001 4.4). `transport.Stream.Control` reads a response until CR or for a maximum of 300 ms; the response is not used. `transport.Stream.Exchange` still accepts only READ commands (SEC-037).
2. The vltd Poller sink gives each fast-group status (Q1 or QS) to `control.Controller.Observe`.
3. Verification with the statuses of the polls after the send:

| Function | Expected status | Sends |
|----------|-----------------|-------|
| Test (all types) | b2 = 1 | Up to 3. One poll between two sends (2 s apart); 2 polls after the last send |
| Cancel test | b2 = 0 within 2 polls | 1 |
| Shutdown and restore | b1 = 1 | Up to 3, as the test |
| Beeper | b0 changes within 2 polls | 1, then 1 retry |

4. Beeper: `SetBeeper(on)` reads b0 of the last status. It sends Q only if b0 is not the wanted state.
5. A test or beeper request is refused while another one runs. The shutdown and restore command does not wait. A test is refused when b7 = 1 (on battery).
6. Test end: b2 changes from 1 to 0. The result is TEST_PASSED if b4 and b6 stayed 0 while b2 was 1, else TEST_FAILED.
7. Audit entries: `ups_control` (result "sent" for each send, "failure" with the error, "no send" for the beeper), `ups_control_verify` ("verified" or "not verified"), `ups_test_end` (TEST_PASSED or TEST_FAILED).
8. Simulator: the model reacts to T, Tnn, TL, CT, Q, the shutdown and restore command and C. It gives no response to a CONTROL command. Test: b2 = 1 for the test time (TL: until the charge is less than 20 %). Shutdown: b1 = 1, output off after the delay, output on after the restore time if mains power is present (without mains the model waits for mains; the real behavior is UNVERIFIED, VLT-REF-001 5.1 item 4). Beeper: b0 changes. Tests: internal/control/sim_test.go (TestSimTestAndBeeper, TestSimTestFailed, TestSimCancelTest, TestSimShutdownRestore, TestSimNoRestoreWithoutMains); each checks that the simulator received only READ commands and the expected CONTROL commands, with 500 ms or more between two requests.

## 15. Notification Router (internal/notify, VLT-IS-007 TASK 03-00-00-003)

- Input: each Event of the Event Engine (`events.Subscribe` in `cmd/vltd/live.go`) and the countdown state of the Policy Engine (one call each tick; the Router sends a message only when the state changes).
- Settings for each Channel (`desktop`, `telegram`, `email`): `notify.<channel>.enabled` and `notify.<channel>.min_severity` (INFO, WARNING, CRITICAL; default WARNING). Default: desktop on, Telegram and e-mail off.
- Flood control: the first Event of a type goes at once. More Events of the same type in the next 60 s go in one combined message at the end of the 60 s window.
- Queue: one queue for each Channel. A failed send is tried again after 10 s, 30 s, 1 min, 5 min, then every 5 min. After 1 hour the message is dropped (log entry). A message sent after a failed send has the text "Late message" and the original time.
- Templates: English and Turkish (`templates.go`). The language is the Setting `ui.language`. The content is the Event, the time, the mode, input voltage, load, battery charge and runtime when available. No Secret and no serial number.
- Channels implement `notify.Channel` (`Name`, `Send`). The Channels are added in TASK 03-00-00-004 to 006.

### 15.1 Desktop Channel (VLT-IS-007 TASK 03-00-00-004)

1. vltd sends each desktop Notification to the connected VLT GUI as the push message "notification" (data: subject, text, severity, type, time). Only connections after status.subscribe get it.
2. If no VLT GUI is connected, the Notification is not shown and not queued. The Event stays in the Events screen.
3. The VLT GUI shows it as an OS notification with the Wails notifications service: Windows toast notifications, Linux org.freedesktop.Notifications (D-Bus). If the service does not start (for example no D-Bus session), the VLT GUI continues without OS notifications and writes a log entry.

### 15.2 Telegram Channel (internal/notify/telegram, VLT-IS-007 TASK 03-00-00-005)

1. The Telegram Bot API over HTTPS (certificate verified) with long polling: getUpdates with a timeout of 25 s, allowed update "message". No webhook. vltd polls when the Telegram Channel is on, or when a pairing code is active.
2. The bot token is the Secret S-01 (Vault ID `telegram.token`). vltd reads it from the Vault for each request (Vault.Use; the go/ast allow list of internal/vault has internal/notify/telegram and internal/notify/email). vltd refuses a token that does not have the format `<digits>:<20 to 100 of A-Z a-z 0-9 _ ->`. The token is in the request URL; errors and log entries have `/bot[REDACTED]` instead.
3. Pairing: the GUI asks for a code (telegram.pair.start, TASK 03-00-00-007). The code has 6 digits (crypto/rand) and expires after 10 minutes, after one correct use, or after 5 wrong codes. The user sends `/start <code>` to the bot. vltd stores the chat ID, the chat name (title, or first and last name, or @username; 64 characters maximum) and the time in the table telegram_chats (migration 0004). Audit: actor telegram, action telegram_pair, target the chat ID, result success or failure.
4. Messages of a chat that is not paired get no answer. vltd writes the audit entry telegram_ignored (result denied), at most once in 10 minutes for each chat.
5. Commands of paired chats: /status and /durum (also with the suffix @botname) give the same text: mode, input and output voltage, load, battery charge, runtime, Shutdown Policy (active and action), last Event. The language is the Setting ui.language. Other text gives the list of commands.
6. Send: the Notification text goes to each paired chat (sendMessage). The Router tries again only when no chat got the message.
7. Tests: a fake Telegram HTTPS server (httptest.NewTLSServer). No test connects to api.telegram.org.

### 15.3 e-mail Channel (internal/notify/email, VLT-IS-007 TASK 03-00-00-006)

1. Settings: `email.smtp.server` (host name), `email.smtp.port` (default 465), `email.smtp.security` (`tls`: implicit TLS, port 465; `starttls`: port 587), `email.smtp.user`, `email.from`, `email.to` (1 to 5 plain addresses). The Channel sends only when the server, the sender and at least one receiver are set.
2. The password is the Secret S-04 (Vault ID `smtp.password`). vltd reads it from the Vault only for SMTP AUTH PLAIN, after TLS is active.
3. Preset "Gmail": smtp.gmail.com, port 465, TLS. The GUI shows the help text about the Google app password (TASK 03-00-00-007).
4. TLS 1.2 or later; the server certificate is verified with the system CAs. In the `starttls` mode, if the server does not offer STARTTLS or the STARTTLS handshake fails, vltd sends nothing and does not use the password.
5. Message: subject "[VLT][severity] Event" (one line, Q-encoded), UTF-8 text, quoted-printable. Addresses and the subject cannot add header lines.
6. Tests: a fake SMTP server (implicit TLS and STARTTLS, a test CA). No test connects to a real server.

### 15.4 Channel screens and IPC methods (VLT-IS-007 TASK 03-00-00-007)

1. IPC methods (elevated state for all changes; list and telegram.chats.list are read-only): `channels.list`, `channels.settings.set`, `channels.test`, `telegram.pair.start`, `telegram.chats.list`, `telegram.chats.remove`. Secret values go only through `secret.set`. `channels.list` shows only the set state and the hint, never the value. The pairing code is in the result of `telegram.pair.start` only, not in `channels.list` and not in the audit log.
2. The VLT GUI screen "Notifications": one section for each Channel (on or off, minimum severity, "Send test message"). Telegram: write-only bot token, "Pair a chat", the code and the steps, the list of paired chats with "Remove". E-mail: SMTP Settings, the preset "Gmail" (smtp.gmail.com, port 465, TLS) with the help text "Use a Google app password, not your normal password. Google account, Security, 2-Step Verification, App passwords.", write-only SMTP password.
3. All user text is in en.json and tr.json.

## 16. First-run wizard (VLT-IS-007 TASK 03-00-00-008)

The wizard starts when no PIN is set. In portable mode it also starts when the wizard-completed marker is not there (the user can Skip the PIN). The user can also start it from the General screen (then the PIN step is omitted). Steps, in order:

1. Set the PIN. Service mode: no Back, no Skip. Portable mode (OD-37): the step is optional, with a "Skip" button. The text is: "A PIN prevents other people at this computer from changing the shutdown settings."
2. Select the UPS port (the Device screen functions, with Scan).
3. Show the live values. The user confirms that they are correct.
4. Shutdown Policy: show the defaults, run the Simulation, activate it. The Simulation includes the step "OS shutdown". If that step fails, the error is "This user cannot shut down the computer." and the policy cannot be activated (section 17.4).
5. Notifications: Telegram, then e-mail (each step can be skipped).
6. Question: "Are the modem and the router connected to the UPS?" If the answer is No, show: "During a power failure, the internet stops. Telegram and e-mail cannot send messages."
7. Summary.

Each step has "Back" and "Skip" (except step 1 in service mode).

## 17. Portable mode (VLT-IS-008)

Portable mode is the default mode (OD-35). One application, VoltioVolt, contains the VLT GUI and the monitoring engine. The default package and its install scripts make no change to the operating system (SEC-102, OD-34, OD-38). Service mode (vltd, vlt-setup, vlt-gui) stays in the source code. It is not in the default package. It becomes an optional advanced mode in a later release (OD-36).

### 17.1 Process and in-process connection

1. Binary: VoltioVolt.exe (Windows) and voltiovolt (Linux). Directory: `cmd/voltiovolt`.
2. The engine runs the same packages as vltd: Device Manager, Poller, Measurements, Event Engine, Policy Engine, Notification Router, Channels, Store, Vault, audit.
3. The VLT GUI connects to the engine through an in-process connection: `ipc.PipeListener` makes a `net.Pipe` for each dial (`internal/ipc/pipe.go`). The peer check is `ipc.InProcessCheck` (peer identity `in-process`). Portable mode does not open a named pipe or a Unix socket.
4. Single instance: only one VoltioVolt process for each user. A second start shows the window of the first process and stops.
5. Start at logon (OD-39): a switch on the General screen. On: make a shortcut in the startup folder of the user (Windows: shell:startup; Linux: ~/.config/autostart/voltiovolt.desktop). Off: delete it. Default: off. This is the only exception in SEC-102.

### 17.2 Data folder

1. Default: the folder "data" next to the binary, if it is writable.
2. If that folder is not writable, the profile folder of the user: Windows %LOCALAPPDATA%\VoltioVolt, Linux ~/.local/share/voltiovolt.
3. The General screen shows the data folder path.
4. Portable mode does not check or change the ACL of the folder. The data folder permission check of VLT-IS-003 TASK 03-00-00-003 step 6 applies to the service mode only.

### 17.3 Vault in portable mode

1. Windows: DPAPI, current user scope (no CRYPTPROTECT_LOCAL_MACHINE), entropy "VoltioVolt-DEK-v1" and the install ID. The canary and the Vault states stay. The key mode is `dpapi`. `NewPortableProvider` uses the same algorithm as `NewHostProvider`. The process runs as the current user, not as NT SERVICE\vltd (SEC-003, VLT-IS-008 01-20-00).
2. Linux (SEC-004 and SEC-005, VLT-IS-008 01-20-00, TASK 03-00-00-002 step 3):
    1. If the Secret Service (D-Bus org.freedesktop.secrets) is available, the DEK is there. The key mode is `secret-service`.
    2. If not, the DEK is in the file `vault.dek` with mode 0600 in the data folder. The Vault state is "reduced protection". The key mode is `file`.
    3. The code talks to the Secret Service with github.com/godbus/dbus/v5 (already a dependency).
3. The service mode keeps its old Vault code without changes.

### 17.4 Access and OS shutdown

1. PIN optional (OD-37): if no PIN is set, the engine gives the elevated state to the in-process connection at all times. Secret fields stay write-only. If a PIN is set, the rules of VLT-IS-003 TASK 03-00-00-005 apply. The General screen has "Set PIN" and "Remove PIN" (Remove needs the current PIN). SEC-022 applies only when a PIN is set.
2. Windows shutdown: the process enables SeShutdownPrivilege in its token with AdjustTokenPrivileges. A normal user has this right on Windows 10 and 11 by default. Portable mode does not ask for administrator rights. If the privilege is not available, the Simulation step "OS shutdown" shows the error "This user cannot shut down the computer." The policy cannot be activated.
3. Linux shutdown: the process calls logind `CanPowerOff` and `PowerOff` or `Hibernate` on the system D-Bus as the user. The default polkit rules permit this for the active local session. Portable mode does not install a polkit rule. If logind refuses, the same error as step 2 is shown.
4. The Simulation and the first-run wizard (step "Shutdown Policy") include this check.

### 17.5 Port access (no change to the system)

1. Windows: if the COM port is busy, the existing "port busy" help text is shown (UPSilon 2000 and RupsMon).
2. Linux: if the open of /dev/ttyUSB* or /dev/ttyACM* fails with "permission denied", the GUI shows: "Your user cannot open this port. Run this command one time, then log out and log in: sudo usermod -aG dialout <user name>". The application does not run the command.
3. Linux HID (hidraw) with "permission denied": a similar text with a udev rule as an example. The application does not install the rule.

### 17.6 Default packages

The pre-release test-latest has VoltioVolt-test-windows-amd64.zip (VoltioVolt.exe, vlt-probe.exe, install.cmd, uninstall.cmd, remove-old-test-service.cmd, OKUBENI-README.txt) and VoltioVolt-test-linux-amd64.tar.gz (voltiovolt, vlt-probe, install.sh, uninstall.sh, OKUBENI-README.txt). The install scripts copy files into the folder of the user only, with no administrator rights. A Start menu or desktop shortcut is made only when the user agrees (OD-38). The script remove-old-test-service.cmd is only for computers with the VLT-IS-007 test installation. The binaries vltd, vlt-gui and vlt-setup are not in the default package.
