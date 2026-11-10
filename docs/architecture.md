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
| Vault | vltd | Keeps Secrets. | not yet |
| IPC Server | vltd | Serves the VLT GUI. Checks access. | not yet |
| Audit Log | vltd | Records security-relevant actions. | not yet |
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
3. The VLT GUI has no Secret and no UPS access. It uses IPC (JSON frames with a 4-byte length prefix, maximum 1 MiB, unknown fields rejected, protocol version field; QA-07, implementation in a later instruction).
