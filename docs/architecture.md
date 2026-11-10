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
| Windows | Named pipe \\.\pipe\vltd (github.com/Microsoft/go-winio). | Protected DACL: SYSTEM, Administrators and NT SERVICE\vltd full access; local group "VLT Operators" (if it exists) read and write. Remote clients are rejected (PIPE_REJECT_REMOTE_CLIENTS). |

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

