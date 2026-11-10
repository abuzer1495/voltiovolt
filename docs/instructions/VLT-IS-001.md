VOLTIOVOLT (VLT UPS MANAGEMENT)
INSTRUCTION SET – MESSAGE 1
SECURITY, SCOPE AND CONCEPTS

DOCUMENT IDENTIFICATION
- Document number: VLT-IS-001
- Revision: 00
- Date: 2026-11-10
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: All VLT components. All platforms. All releases.
- Type: Information and report request. This is not a build instruction.

====================================================================
00-00-00 GENERAL
====================================================================

00-00-01 Purpose
1. This message gives the security rules, the scope and the concepts of the VLT project.
2. Do not write application code for this message.
3. Do not create, change or delete files, branches or repositories for this message.
4. Read all of this message. Then write Report 1. Refer to 05-00-00.

00-00-02 Work method
1. The project uses a cycle of instructions and reports.
2. The Owner sends you one instruction.
3. You do only the tasks in that instruction.
4. You write one report.
5. The Owner sends your report to the Instruction Author.
6. The Instruction Author writes the next instruction.
NOTE: Each instruction has a document number and a revision. Obey the latest revision only.

00-00-03 Parties
| Party              | Identity                  | Responsibility                                          |
|--------------------|---------------------------|---------------------------------------------------------|
| Owner              | GitHub account abuzer1495 | Approves all decisions. Does all tests on the real UPS. |
| Instruction Author | Not applicable            | Writes the instructions. Reviews the reports.           |
| Builder            | You                       | Does the tasks in each instruction. Writes the reports. |

00-00-04 Warnings, cautions and notes
- WARNING: Risk of a security breach, loss of user Secrets, or loss of power to the protected equipment.
- CAUTION: Risk of data loss or incorrect operation.
- NOTE: Information that helps you.
A WARNING or a CAUTION comes before the text it applies to. Obey it before you continue.

00-00-05 Requirement words
- MUST: The requirement is mandatory.
- MUST NOT: The action is prohibited.
- CAN: The action is permitted. It is not mandatory.
Do not use "should" or "may" for requirements in your reports.

====================================================================
00-10-00 DEFINITIONS
====================================================================
Use these terms only with these meanings. Do not use other words for these items.

| Term                | Meaning |
|---------------------|---------|
| VLT                 | The product VoltioVolt (VLT UPS Management). |
| UPS                 | Uninterruptible power supply. |
| Reference UPS       | Tunçmatik Newtech Pro II 1 kVA, online type. The Owner tests VLT on this unit. |
| vltd                | The VLT service. It runs in the background without a user session. It is the only component that talks to the UPS. |
| VLT GUI             | The desktop user interface. It shows data. It changes Settings only through vltd. |
| VLT Agent           | A small client on another computer. It receives shutdown signals from vltd. Release v2.0. |
| Transport           | The physical link to the UPS: USB HID or RS-232 serial. |
| Dialect             | One variant of the Megatec Q protocol family. Examples: Q1, Voltronic-QS, Voltronic. |
| Driver              | One Transport, one Dialect and one response parser for one UPS model. |
| Poll                | One request to the UPS and one response. |
| Measurement         | One value from a Poll, with a unit and a UTC time stamp. |
| Event               | A change of the UPS state. Example: ON_BATTERY. |
| Alarm               | An Event that needs an action from a person. |
| Channel             | One path for Notifications: desktop, Telegram, e-mail, WhatsApp or voice call. |
| Notification        | One message on one Channel. |
| Acknowledgement     | A person confirms that they saw an Alarm. It stops the Escalation. |
| Escalation          | A timed sequence of Notifications. It continues until an Acknowledgement. |
| Shutdown Policy     | The rules that start the safe shutdown of computers and of the UPS output. |
| Simulation          | A test of Shutdown Policies and Channels. It does not shut down a computer. It does not send commands to the UPS. |
| Settings            | All configuration that is not a Secret. vltd keeps Settings in its database. |
| Secret              | A value that gives access to an account or to a key. Refer to 02-10-01. |
| Vault               | The encrypted Secret store inside vltd. |
| DEK                 | Data Encryption Key. vltd uses it to encrypt each Secret. |
| Host binding        | The operating system wraps the DEK. Only the same host can unwrap it. |
| PIN                 | The administrator code for the VLT GUI. |
| Remote PIN          | A different administrator code for CONTROL requests from Telegram. |
| READ command        | A UPS command that only gets data. |
| CONTROL command     | A UPS command that changes the output, a test or the beeper. |
| CONFIG command      | A UPS command that writes to the UPS memory (EEPROM). |
| UNVERIFIED          | A statement without a source or without a test on the Reference UPS. |

====================================================================
01-00-00 SCOPE
====================================================================

01-00-01 Product
VLT is an open-source replacement for UPSilon 2000. VLT monitors a UPS and records the data.
VLT analyzes the data and sends Notifications. VLT shuts down computers safely during a long power failure.

01-00-02 Identity
| Item                     | Value |
|--------------------------|-------|
| Product name             | VoltioVolt |
| Full name                | VLT UPS Management |
| Short name               | VLT |
| Repository               | github.com/abuzer1495/voltiovolt |
| Visibility               | Public |
| License                  | AGPL-3.0 |
| Repository language      | English |
| User interface languages | Turkish and English |

01-00-03 Platforms
| Platform | Minimum version                                                 | Architecture | Release |
|----------|-----------------------------------------------------------------|--------------|---------|
| Windows  | Windows 10 22H2, Windows 11                                     | x86_64       | MVP     |
| Linux    | systemd 250 or later. Examples: Ubuntu 24.04, Debian 12, Fedora 40 | x86_64    | MVP     |
| Linux    | Same as above                                                   | arm64        | v2.0    |
NOTE: macOS is out of scope.

01-00-04 Reference hardware
| Item                  | Value                                                        | Status |
|-----------------------|--------------------------------------------------------------|--------|
| Model                 | Tunçmatik Newtech Pro II 1 kVA, online                       | Known |
| USB identity          | VID 0665, PID 5161, product string "USB to Serial"           | Known for the product family |
| USB class             | HID with vendor-defined reports                              | Known for the product family |
| Serial link           | RS-232, 2400 bit/s, 8 data bits, no parity, 1 stop bit, CR (0x0D) at end of line | Known for the protocol family |
| Protocol              | Megatec Q family, Voltronic dialects                         | UNVERIFIED on the Reference UPS |
| Battery configuration | Number of 12 V blocks                                        | UNVERIFIED |

01-00-05 Fixed technical decisions
1. Write vltd, VLT Agent and all shared code in Go.
2. Use the current stable Go release. Record it with the toolchain directive in go.mod.
3. Use pure Go libraries when possible. Use cgo only with a written reason in a report.
4. vltd keeps Settings, Measurements, Events and the audit log in one SQLite database.
5. All configuration is done in the VLT GUI. Users do not edit files.
6. vltd does not read Settings from configuration files, environment variables or command-line flags.
   NOTE: Read-only diagnostic flags are permitted. Example: --version.
7. In the MVP, vltd has no network listener. Each later network listener is off by default.
8. The VLT GUI connects to vltd through local IPC only: a named pipe on Windows, a Unix domain socket on Linux.
9. The VLT GUI technology is not decided. Propose it in Report 1.

01-00-06 Release content
MVP:
- USB and serial Transports. Automatic Dialect detection with READ commands only.
- Live dashboard: input and output voltage and frequency, load, battery, temperature, mode, warning bits.
- Event Engine, local Event log and Measurement recording.
- Safe shutdown or hibernation of the local computer.
- UPS output off, with automatic restart when mains power returns.
- Simulation mode.
- Telegram Channel: Notifications, status command, pairing code, chat ID allow list.
- E-mail Channel: Gmail API with OAuth 2.0. SMTP with an app password as fallback.
- Vault with host binding. Write-only Secret fields.
- First-run wizard. Tray icon. Turkish and English user interface.
v1.0:
- Battery analysis: learned runtime estimate, state of health, internal resistance trend, battery life estimate.
- Power quality and interruption reports, based on EN 50160 limits.
- Energy, cost (TRY per kWh) and CO2 estimate.
- Scheduled battery test. Scheduled UPS on and off.
- UPS advanced settings screen for CONFIG commands.
- Telegram administrator commands with confirmation and Remote PIN.
- WhatsApp Channel: official Cloud API, utility templates, outbound only.
- Voice call Channel: SIP, Turkish text-to-speech, DTMF Acknowledgement, Escalation chain.
- Voice-message provider Channel. Example: Netgsm.
- Weekly and monthly PDF reports. CSV export without Secrets.
- NUT-compatible network server (RFC 9271). Off by default.
v2.0:
- VLT Agent for other computers. Wake-on-LAN.
- More than one UPS.
- Web interface with TLS and TOTP. Off by default.
- MQTT, Home Assistant and Prometheus integration.
- USB 4G modem Channel for SMS and calls without internet.
- Virtual machine shutdown (Proxmox, Hyper-V).

01-00-07 Out of scope
- macOS.
- A cloud service operated by the project.
- Telemetry, crash upload or usage statistics of any type.
- Unofficial WhatsApp libraries in the MVP and v1.0.
- UPS firmware update.
- Reverse engineering of proprietary software. Examples: UPSilon 2000, ViewPower.

====================================================================
02-00-00 SECURITY
====================================================================
WARNING: VLT has access to messaging accounts. VLT can turn off computers and the UPS output.
A defect can cause a security breach or a loss of power to equipment.

02-00-01 Priority
1. Security requirements have priority over all other requirements.
2. If a task conflicts with a security requirement, stop the task. Write the conflict in your report.
3. Do not make a security requirement weaker to make a task easier.

02-10-00 SECRETS

02-10-01 Secret list
| ID   | Secret |
|------|--------|
| S-01 | Telegram bot token |
| S-02 | Gmail OAuth refresh token and access token |
| S-03 | Google OAuth client secret (created by the user) |
| S-04 | Gmail app password (SMTP fallback) |
| S-05 | WhatsApp Cloud API access token |
| S-06 | SIP account password |
| S-07 | Voice-message provider credentials |
| S-08 | PIN hash and Remote PIN hash |
| S-09 | DEK |
| S-10 | VLT Agent keys (v2.0) |

02-10-02 Storage
SEC-001 Encrypt each Secret with AES-256-GCM and the DEK. Use a new random nonce for each encryption.
SEC-002 Do not write the DEK to disk in plain form.
SEC-003 Windows: wrap the DEK with DPAPI in the user scope of the vltd service account. Add application-specific entropy. Do not use the LocalMachine scope.
SEC-004 Linux: wrap the DEK with systemd-creds. Use TPM2 plus the host key when a TPM2 chip is available.
SEC-005 Linux without TPM2: systemd-creds uses the host key only. Show a "reduced protection" notice in the VLT GUI.
SEC-006 A copy of the database on a different host MUST NOT decrypt. Write a test for this.
SEC-007 Keep the PIN and the Remote PIN only as Argon2id hashes.

02-10-03 Exposure
SEC-010 Secret fields in the VLT GUI are write-only. After save, show only the last 4 characters.
SEC-011 No IPC message, export, report or log contains a Secret.
SEC-012 Do not put Secrets in environment variables, command-line arguments, crash dumps or child processes.
SEC-013 Remove Secrets from all log output. Write tests for this redaction.
SEC-014 The Settings export does not contain Secrets.
SEC-015 v1.0: an encrypted backup can contain Secrets. It uses a passphrase with Argon2id. It needs the PIN.
SEC-016 Keep decrypted Secrets in memory only for the time of use. Keep the DEK in memory only inside vltd.

02-20-00 ACCESS CONTROL
SEC-020 vltd runs as a dedicated service account. Give it only the rights for UPS access and system shutdown.
SEC-021 IPC access: Windows: Administrators and the local group "VLT Operators". Linux: root and the group "vlt".
SEC-022 The VLT GUI asks for the PIN before these actions: Settings change, Secret entry, CONTROL command, CONFIG command, Shutdown Policy activation.
SEC-023 The VLT GUI locks after 10 minutes without input. The Owner can change this value.
SEC-024 After 5 incorrect PIN entries, block PIN entry for 5 minutes. Double the block time after each new failure.
SEC-025 Write an audit entry for each Settings change, each CONTROL or CONFIG command, each incorrect PIN and each rejected remote request.

02-30-00 UPS COMMAND SAFETY
WARNING: CONTROL commands and CONFIG commands can stop power to the protected equipment.
CONFIG commands change the UPS permanently.
SEC-030 Automatic detection sends READ commands only.
SEC-031 vltd sends a CONTROL command only for one of these: an operator request with the PIN, a Telegram request with the Remote PIN, a Shutdown Policy trigger.
SEC-032 vltd sends a CONFIG command only after an operator request with the PIN and a second confirmation.
SEC-033 Parse each UPS response strictly. Check the start character, the field count, the field format and the maximum length. Reject all other data.
SEC-034 Fuzz-test each parser.
SEC-035 Before each command on USB, remove old data from the input buffer. Match each response to its request.
SEC-036 Development and CI use a UPS simulator only.
WARNING: Do not send CONTROL or CONFIG commands to real hardware. Only the Owner does tests on real hardware.

02-40-00 CHANNELS
SEC-040 Telegram: use long polling (getUpdates). Do not use a webhook. Do not open an inbound port.
SEC-041 Telegram: pair a chat with a one-time code. The code expires after 10 minutes.
SEC-042 Telegram: accept requests only from paired chat IDs. Ignore all other chats. Write an audit entry for each.
SEC-043 Telegram: use two roles, Viewer and Admin. A CONTROL request needs the Admin role, an inline confirmation and the Remote PIN. The confirmation expires after 60 seconds.
SEC-044 Telegram: delete the message that contains the Remote PIN after you read it.
SEC-045 Gmail: use OAuth 2.0 with PKCE and a loopback redirect to 127.0.0.1. Request only the scope https://www.googleapis.com/auth/gmail.send.
SEC-046 Gmail: the "Disconnect" action revokes the token at Google. Then it deletes the token from the Vault.
SEC-047 SMTP fallback: use TLS on port 465 or STARTTLS on port 587. If STARTTLS fails, stop the send.
SEC-048 All outbound TLS connections: verify the certificate. Use TLS 1.2 or later. Do not add an option to disable verification.
SEC-049 WhatsApp: use the official Cloud API with approved utility templates only.
SEC-050 SIP: use TLS and SRTP when the provider supports them. If not, show a warning in the VLT GUI.
SEC-051 Notification text does not contain Secrets. It does not contain serial numbers or IP addresses unless the Owner enables them.

02-50-00 SHUTDOWN AND SCRIPTS
SEC-055 A Shutdown Policy cannot be active before one successful Simulation.
SEC-056 Show a countdown with a cancel button before each automatic shutdown. Default: 60 seconds.
SEC-057 User scripts run only from a protected directory. vltd records the SHA-256 hash of each script. If the hash changes, vltd does not run the script and sends an Alarm.
SEC-058 Do not build shell command strings. Start each process with an argument list.

02-60-00 SOURCE CODE AND SUPPLY CHAIN
SEC-060 Clean-room rule: do not copy code or text from NUT, UPSilon 2000, ViewPower or other GPL or proprietary sources. Use only protocol facts from public documents. Record each source in docs/references.md.
SEC-061 For each dependency, record the name, version, license and reason. Permitted licenses: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, MPL-2.0. Other licenses need Owner approval.
SEC-062 Pin all dependency versions. Run govulncheck in CI.
SEC-063 Enable GitHub secret scanning and push protection. Run gitleaks in CI.
SEC-064 Do not commit Secrets, real tokens, real phone numbers or real e-mail addresses. In tests, use example.com addresses and clearly fake numbers.
SEC-065 After the first commit, do not push to the main branch directly. Use one branch and one pull request for each task. Merge only after Owner approval.
SEC-066 Do not force-push. Do not rewrite history. Do not change GitHub account settings. Do not create or delete other repositories.
SEC-067 Each release has SHA-256 checksums and an SBOM (SPDX or CycloneDX). Do not publish a release without Owner approval.
SEC-068 Add SECURITY.md. Enable GitHub private vulnerability reporting.

02-70-00 BUILDER INTEGRITY
SEC-070 Do not invent facts. If you do not know a fact, write UNVERIFIED.
SEC-071 Do not invent library names, versions or functions. Give a source URL for each.
SEC-072 Do not report a test as passed if you did not run it. Give the command and a summary of the output.
SEC-073 Do not change the scope. Write scope proposals in your report.
SEC-074 If an instruction is not clear, do not guess. Write a question in your report.

====================================================================
03-00-00 CONCEPTS
====================================================================

03-10-00 Components
| Component           | Part of | Function |
|---------------------|---------|----------|
| Device Manager      | vltd    | Finds the UPS. Selects the Transport and the Dialect. |
| Poller              | vltd    | Sends READ commands on a schedule. |
| Measurement Store   | vltd    | Records Measurements. Keeps summaries per minute and per hour. |
| Event Engine        | vltd    | Makes Events from state changes. |
| Analyzer            | vltd    | Calculates runtime, battery health, power quality and energy. |
| Policy Engine       | vltd    | Runs Shutdown Policies. |
| Notification Router | vltd    | Sends Notifications. Runs Escalations. Receives Acknowledgements. |
| Vault               | vltd    | Keeps Secrets. |
| IPC Server          | vltd    | Serves the VLT GUI. Checks access. |
| Audit Log           | vltd    | Records security-relevant actions. |
| VLT GUI             | Desktop | Screens: Dashboard, History, Events, Analysis, Channels, Policies, UPS Settings, Security. |

03-10-01 Data flow
UPS -> Transport -> Dialect parser -> Measurement -> Measurement Store and Event Engine
-> Policy Engine and Notification Router -> Channels.

03-20-00 UPS protocol
1. The computer is the master. The UPS sends data only as a response to a request.
2. A request is ASCII text. It ends with CR (0x0D).
3. A data response starts with "(" and ends with CR.
4. For an unsupported command, the UPS can return "NAK" or an echo of the request. Treat both as "not supported".
5. On USB, the UPS sends each response in 8-byte reports. Some units keep old bytes in the buffer.

Command classes. All commands are UNVERIFIED on the Reference UPS.
| Class   | Commands                                                      | Use |
|---------|---------------------------------------------------------------|-----|
| READ    | Q1, QS, QGS, QBV, QMOD, QWS, QFLAG, QRI, F, I, QID, QVFW, QMD | Detection and polling |
| CONTROL | T, Tnn, TL, CT, Snn, SnnRmmmm, C, CS, SOFF, SON, BZON, BZOFF  | Tests, shutdown, beeper |
| CONFIG  | PEx, PDx and other setting commands                           | UPS settings in EEPROM |
NOTE: QWS returns a 64-bit warning field. Each bit has a fixed meaning. Translate each bit to an Event.

03-30-00 Events
| Event                | Severity |
|----------------------|----------|
| ONLINE               | INFO |
| ON_BATTERY           | WARNING |
| LOW_BATTERY          | CRITICAL |
| RUNTIME_LOW          | CRITICAL |
| OVERLOAD             | CRITICAL |
| OVER_TEMPERATURE     | CRITICAL |
| UPS_FAULT            | CRITICAL |
| BATTERY_DISCONNECTED | CRITICAL |
| BYPASS_ACTIVE        | WARNING |
| ECO_ACTIVE           | INFO |
| BATTERY_REPLACE      | WARNING |
| COMM_LOST            | WARNING. CRITICAL when the UPS is on battery. |
| COMM_RESTORED        | INFO |
| TEST_STARTED         | INFO |
| TEST_PASSED          | INFO |
| TEST_FAILED          | WARNING |
| SHUTDOWN_PENDING     | CRITICAL |
| SHUTDOWN_CANCELLED   | INFO |
| SHUTDOWN_EXECUTED    | CRITICAL |
| SECURITY_ALERT       | WARNING |

03-40-00 Analysis
| Analysis              | Inputs                                       | Method summary |
|-----------------------|----------------------------------------------|----------------|
| Runtime estimate      | Load, battery voltage, discharge history     | Learn a load-to-runtime curve from real discharges. |
| State of charge       | Battery voltage, load                        | Voltage at rest. Integration under load. |
| State of health       | Runtime at the same load over time           | Ratio to the first calibrated runtime. |
| Internal resistance   | Voltage drop and load at transfer to battery | Delta V divided by delta I. Trend over time. |
| Battery life estimate | Temperature history, installation date       | Life is halved for each 8 °C above 25 °C. This is a configurable assumption. |
| Power quality         | Input voltage and frequency                  | EN 50160 limits: nominal voltage ±10 %, 50 Hz ±1 %. |
| Interruptions         | Mode changes                                 | Short: 3 minutes or less. Long: more than 3 minutes. |
| Energy and cost       | Load or output current, rated power, tariff  | kWh per day, month and year. TRY per kWh from Settings. |
NOTE: A poll interval of 2 seconds cannot detect short voltage dips. Show this limit in the VLT GUI.

03-50-00 External constraints
1. Internet Channels need the modem and the router on UPS power. The first-run wizard asks about this. If they are not on UPS power, the VLT GUI shows a warning.
2. WhatsApp Cloud API: each message has a cost. Inbound messages need a public webhook. For this reason, the WhatsApp Channel is outbound only.
3. Gmail: gmail.send is a sensitive scope. A shared project OAuth client needs Google verification. For this reason, the MVP uses an OAuth client that the user creates. The VLT GUI has a wizard for this.
4. A Google OAuth client in "Testing" status loses its tokens after approximately 7 days. The wizard tells the user to set the status "In production".

====================================================================
04-00-00 WORK RULES
====================================================================
1. Do one instruction at a time.
2. Do only the tasks in the instruction.
3. Write all repository text and all code comments in English.
4. Use the terms in 00-10-00.
5. Write reports in Simplified Technical English.
6. Use the same report structure each time.

====================================================================
05-00-00 REPORT 1 REQUEST
====================================================================
CAUTION: Do not write application code. Do not create, change or delete repositories, branches or files.

TASK 05-00-00-001 Confirm understanding
1. Write the scope in your own words. Use 10 sentences or fewer.
2. List each item in this message that you do not agree with. Give the reason for each item.

TASK 05-00-00-002 Check access
1. Confirm read access to the GitHub account abuzer1495.
2. List the existing repositories with names that contain "volt" or "vlt".
3. Confirm that the name "voltiovolt" is available in this account.
4. Do not make changes.

TASK 05-00-00-003 Propose the toolchain
1. For each item below, give 1 to 3 candidates.
2. For each candidate, give: name, URL, license, latest version, release date, cgo use, Windows support, Linux support, risks.
3. Recommend one candidate for each item. Give the reason.
4. Mark each fact without a source as UNVERIFIED.
Items:
a. VLT GUI technology. Requirements: Windows and Linux, tray icon, Turkish text, charts, accessibility, separate process from vltd.
b. USB HID access for VID 0665, PID 5161.
c. Serial port access.
d. SQLite driver.
e. Windows service and Linux systemd integration.
f. DPAPI from a Windows service account. Identify the account type that supports user-scope DPAPI.
g. systemd-creds integration.
h. Telegram Bot API client, or direct HTTPS.
i. Google OAuth 2.0 and Gmail API.
j. SIP and RTP with DTMF (RFC 4733), TLS and SRTP.
k. Offline Turkish text-to-speech.
l. Argon2id and AES-GCM.
m. Packaging and code signing: Windows MSI, Linux deb and rpm.
n. CI: GitHub Actions, govulncheck, gitleaks, license check, SBOM.
o. UPS simulator for tests.

TASK 05-00-00-004 Propose the repository structure
1. Propose the directory layout.
2. Propose the governance files: README.md, LICENSE, SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md, docs/.
3. Propose the branch rules and the pull request rules.
4. Do not create these items.

TASK 05-00-00-005 List the risks
1. List the 10 highest risks.
2. For each risk, give: description, probability (low, medium, high), impact, mitigation.

TASK 05-00-00-006 List your questions
1. Write your questions for the Owner.
2. Write your questions for the Instruction Author.

05-10-00 Report format
1. Title: VLT-RPT-001, Revision 00.
2. Format: Markdown.
3. Sections, in this order:
   1. Understanding
   2. Disagreements
   3. Access
   4. Toolchain (one table for each item)
   5. Repository plan
   6. Risks (one table)
   7. Questions
   8. Sources (list of URLs)
4. Do not add other sections.

END OF MESSAGE VLT-IS-001
