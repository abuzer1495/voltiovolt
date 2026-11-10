VOLTIOVOLT (VLT UPS MANAGEMENT)
INSTRUCTION SET – MESSAGE 2
DECISIONS AND FIRST BUILD: REPOSITORY, CI, PROTOCOL LAYER, SIMULATOR, CAPTURE TOOL

DOCUMENT IDENTIFICATION
- Document number: VLT-IS-002
- Revision: 00
- Date: 2026-11-10
- Reference documents: VLT-IS-001 Revision 00. VLT-RPT-001 Revision 00.
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: Repository, CI, Dialect layer, Transport layer, simulator, vlt-probe. Release: MVP.
- Type: Build instruction.

====================================================================
00-00-00 GENERAL
====================================================================

00-00-01 Purpose
1. This instruction gives the decisions on VLT-RPT-001.
2. This instruction starts the build.
3. Do only the tasks in 03-00-00. Then write Report 2. Refer to 05-00-00.

00-00-02 Not in this instruction
- vltd service, Vault, IPC, Store, Event Engine, Policy Engine, Channels, VLT GUI.
- Encoders for CONTROL commands and CONFIG commands. Do not write code that can send them.
- Releases and tags.

00-00-03 Failure rule
CAUTION: If a task fails, do not use a workaround that changes the scope.
1. Stop the failed task.
2. Continue with the tasks that do not depend on it.
3. Write the failure in Report 2.

====================================================================
01-00-00 DECISIONS
====================================================================

01-10-00 Answers to the questions for the Instruction Author
| No.   | Decision |
|-------|----------|
| QA-01 | Accepted. "Q" is a CONTROL command. "D" is in no class. VLT never sends "D". Detection and vlt-probe use only the Probe List (03-50-01). |
| QA-02 | Accepted. The installer (root) creates and wraps the DEK. The unit file uses LoadCredentialEncrypted=. vltd never starts systemd-creds. The installer also writes the key mode (host, tpm2 or host+tpm2) to a root-owned file. This file contains no Secret. vltd can read it for the SEC-005 notice. |
| QA-03 | Accepted. The SEC-020 account on Windows is the virtual account NT SERVICE\vltd. The SeShutdownPrivilege method stays UNVERIFIED. A later instruction covers it. |
| QA-04 | Accepted. Use the gitleaks binary with a pinned version and a SHA-256 check. Do not use gitleaks-action. |
| QA-05 | Accepted. Use the words "EN 50160-based estimate". Do not use the word "compliance". |
| QA-06 | Accepted. Use direct OS APIs for HID. If the direct method fails on the Reference UPS, write it in a report. Do not change to go-hid without approval. |
| QA-07 | Decided now. Implementation is in a later instruction. IPC messages are JSON (Go standard library). Each frame has a 4-byte length prefix. Maximum frame size: 1 MiB. The decoder rejects unknown fields. Each message has a protocol version field. |
| QA-08 | Accepted for v1.0. Each paid Channel has a monthly message limit Setting. Default: 300 messages each month. |
| QA-09 | Accepted. The report format in 05-10-00 has a "Command log" section and a "Deviations" section. |

01-20-00 Owner decisions
| No.   | Subject | Decision |
|-------|---------|----------|
| OD-01 | Project license | AGPL-3.0-or-later. Each source file has the SPDX identifier in its header. |
| OD-02 | Dependency licenses | The SEC-061 list stays the rule for all Go modules in VLT binaries. GPL-3.0 programs are permitted only as separate programs that the user installs, after Owner approval. |
| OD-03 | VLT GUI | Wails v3 is approved. cgo is approved for the VLT GUI on Linux only. Front-end rules come in a later instruction. |
| OD-04 | Build tools | WiX Toolset v7 is approved as a build tool. The project does not generate revenue. kardianos/service, Qt and gitleaks-action are not used. |
| OD-05 | Turkish voice (v1.0) | Primary method: pre-recorded Turkish prompts. The Owner decides the source of the recordings before v1.0. The voice-message provider Channel uses the provider text-to-speech. Do not distribute the Piper tr_TR dfki voice. |
| OD-06 | GitHub identity | The Builder uses only the login abuzer1495. Refer to 02-00-00. |
| OD-07 | Code signing | Apply to SignPath Foundation before the first public release. Not in this instruction. |
| OD-08 | Telegram CONTROL confirmation (v1.0) | A TOTP code (RFC 6238, 6 digits, 30 seconds) replaces the Remote PIN. The Owner adds the TOTP key to an authenticator application with a QR code in the VLT GUI. SEC-044 stays: delete the message that contains the code. |
| OD-09 | Loss of Secrets | Accepted for the MVP. After a Windows reinstall or a move to a new host, the Secrets are lost. The VLT GUI has a "Vault reset" flow. It asks for all Secrets again. SEC-015 stays in v1.0. |
| OD-10 | SECURITY.md contact | GitHub private vulnerability reporting only. No e-mail address. |
| OD-11 | Hardware facts | The Owner gives the battery data, the RS-232 status and the test computers with the first capture. |
| OD-12 | Reports in the repository | Do not commit VLT-RPT files. They contain account data. Write the decisions in docs/decisions.md. Commit the VLT-IS files in docs/instructions/. |
| OD-13 | Commit sign-off | Use DCO sign-off (git commit -s) for each commit. |

01-30-00 Changes to VLT-IS-001
These changes make VLT-IS-001 Revision 01.
| Item | Change |
|------|--------|
| 00-10-00 | "Remote PIN" is replaced by "Remote code": a TOTP code for CONTROL requests from Telegram. |
| 00-10-00 | Add the term "Probe List": the ordered READ commands for detection. Refer to 03-50-01 of VLT-IS-002. |
| 00-10-00 | Add the term "Capture": a file from vlt-probe with raw UPS requests and responses. |
| 01-00-02 | License: AGPL-3.0-or-later. |
| 01-00-05 item 3 | cgo is permitted for the VLT GUI on Linux only. Tests can use cgo for the race detector. Shipped binaries other than the VLT GUI use CGO_ENABLED=0. |
| 01-00-05 item 6 | vltd can read the path in $CREDENTIALS_DIRECTORY. This path is not a Setting and not a Secret. |
| 01-00-05 item 6 | vlt-probe and vlt-sim are diagnostic tools, not vltd. They can use command-line flags to select a Transport, a port and a mode. |
| 02-10-01 | Add S-11: TOTP key. S-08 becomes: PIN hash. |
| SEC-004 | The installer (root) creates and wraps the DEK. vltd reads the DEK through LoadCredentialEncrypted=. vltd does not start systemd-creds. |
| SEC-037 (new) | The Driver sends only commands in the command registry. It rejects all other strings. |
| SEC-038 (new) | Do not send a CR alone. Do not send the remaining bytes of a partial request. |
| SEC-043 | "Remote PIN" is replaced by "Remote code". |
| SEC-063 | Use the gitleaks binary, not gitleaks-action. If the API does not permit push protection, the Owner sets it in the GitHub UI. |
| 03-20-00 | Add "Q" to the CONTROL class. "D" is in no class. |
| 03-40-00 | Power quality is an "EN 50160-based estimate". |
| 05-10-00 | Replaced by 05-10-00 of VLT-IS-002. |

====================================================================
02-00-00 GITHUB RULES
====================================================================
WARNING: The Builder box has more than one GitHub login. A push with the wrong login publishes work under a different identity.

SEC-080 Use only the login abuzer1495 for all GitHub API calls and all git pushes.
SEC-081 Before the first push of each task, confirm the login with the GitHub API. Write the result in the Command log.
SEC-082 Set the git author and committer in the local configuration of this repository only. Name: abuzer1495. E-mail: the GitHub no-reply address of abuzer1495 (format: <id>+abuzer1495@users.noreply.github.com). Get the id from the GitHub API.
SEC-083 Do not change the global git configuration. Use a token for each command when possible. If you must change the active gh login, change it back after the task. Write it in the report.
SEC-084 Do not write the name of any other GitHub login in the repository.
SEC-085 One task, one branch, one pull request. Branch name: vlt-is-002/<task-name>.
SEC-086 This instruction gives Owner approval to merge each pull request of this instruction. Conditions: all required checks pass, and the pull request contains only the work of its task. Use squash merge.
SEC-087 Do not commit binaries. Do not commit Capture files.

====================================================================
03-00-00 TASKS
====================================================================

TASK 03-00-00-001 Confirm the identity
1. Confirm the login abuzer1495 with the GitHub API.
2. Get the numeric user id. Make the no-reply e-mail address.
3. Write the login and the e-mail address in the Command log. Do not write tokens.

TASK 03-00-00-002 Create the repository
1. Create the public repository abuzer1495/voltiovolt.
   Description: "VoltioVolt (VLT UPS Management): open-source UPS monitoring and safe shutdown for Windows and Linux."
2. Make the initial commit on main. It contains only these files:
| File | Content |
|------|---------|
| LICENSE | Full AGPL-3.0 text without changes. Source: gnu.org. |
| README.md | Purpose. Status "pre-alpha". Platforms. Reference UPS. Safety WARNING about CONTROL and CONFIG commands. Links to docs. |
| SECURITY.md | Supported versions: none (pre-alpha). Report method: GitHub private vulnerability reporting only. Scope: Vault, IPC, Channels, UPS commands. |
| CONTRIBUTING.md | Clean-room rule (SEC-060). Dependency rule (SEC-061). Branch and pull request rules. DCO sign-off. Test and fuzz rules. |
| CODE_OF_CONDUCT.md | Contributor Covenant 2.1. Verify the text source and its license before use. |
| CHANGELOG.md | One section: "Unreleased". |
| .gitignore | Go build output, editor files, Capture files (*.vltcap.json). |
| .gitattributes | LF line ends for text files. |
| docs/instructions/VLT-IS-001.md | Text of VLT-IS-001 Revision 00. |
| docs/instructions/VLT-IS-002.md | Text of this instruction. |
| docs/decisions.md | One row for each decision in 01-00-00: number, date, decision, source document. |
| docs/security.md | Secret list, SEC list and threat model summary from VLT-IS-001, with the changes in 01-30-00. |
3. Do not put code in the initial commit.

TASK 03-00-00-003 Set the repository settings
Make these settings with the GitHub API. If the API refuses a setting, do not try a different method. Write it in Report 2. The Owner then sets it in the GitHub UI.
1. Merge methods: squash merge only. Delete head branches after merge: on.
2. Wiki: off. Projects: off. Issues: on.
3. Private vulnerability reporting: on.
4. Secret scanning: on. Push protection: on.
5. Actions: default GITHUB_TOKEN permission is read-only. Actions cannot approve pull requests.
6. Dependabot alerts: on.

TASK 03-00-00-004 Make the CI
Branch: vlt-is-002/ci.
1. go.mod: module github.com/abuzer1495/voltiovolt. Lines: "go 1.27" and "toolchain go1.27.3".
2. File .github/workflows/ci.yml. Triggers: pull_request and push to main.
3. Jobs on ubuntu-latest and windows-latest:
   a. gofmt check.
   b. go vet.
   c. go build ./... with CGO_ENABLED=0.
   d. go test ./... on Windows. go test -race ./... on Linux.
   NOTE: The race detector needs cgo. This is a test-only exception.
4. Jobs on ubuntu-latest only:
   a. Fuzz smoke test: each fuzz target for 30 seconds.
   b. govulncheck.
   c. gitleaks binary: pinned version, SHA-256 check before it runs. Scan the full history.
   d. go-licenses check. The allow list is the SEC-061 list. Fail on all other licenses.
5. Pin each action with its full commit SHA. Write the version in a comment.
6. Set "permissions: contents: read" at workflow level.
7. File .github/dependabot.yml: gomod and github-actions, weekly.
8. File .gitleaks.toml: default rules. Confirm that the rules detect a Telegram bot token. If not, add a rule. The allow list contains only files in testdata/ with fake values.
9. Artifacts: vlt-probe binaries for windows-amd64 and linux-amd64, each with a SHA-256 file. Retention: 14 days. This is not a release.

TASK 03-00-00-005 Make the Dialect package
Branch: vlt-is-002/dialect. Directory: internal/dialect.
1. Command registry: one table. Each row has the command, the class (READ, CONTROL, CONFIG), the Dialects and the response type. CONTROL and CONFIG rows contain only the name and the class.
2. Encoder: makes request bytes for READ commands only. Output: ASCII command, then CR. Reject all strings that are not READ commands in the registry.
3. Parsers: Q1, QS, QGS, QBV, QMOD, QWS, QFLAG, QRI, F, I, QID, QVFW, QMD.
4. Parser rules:
   a. Maximum response length: 256 bytes.
   b. Check the start character ("(", or "#" for F and I), the field count, the field width, the number format and the CR at the end.
   c. A field with only "-", "@" or "#" characters has no value. Do not change it to zero.
   d. Detect "NAK", "(NAK" and an echo of the request. Return the result "not supported".
   e. Keep status bits as raw bits. Add named flags only for the Q1 and QS bits in the public Megatec document. QGS bits stay raw (meaning UNVERIFIED).
   f. Keep the Q1 battery voltage as received. Do not change a per-cell value to a total value in the parser.
   g. Each value has a unit in its type: V, Hz, A, %, °C or minutes.
   h. A parser never panics. It returns an error with the field index.
5. Tests: table tests with the vectors in 03-50-02. One fuzz target for each parser.
6. A test proves that the encoder rejects these strings: "Q", "D", "T", "TL", "S01", "S01R0001", "C", "CS", "SOFF", "BZOFF", "PEE", "PDB", "", "\r", "Q1 ", "q1".
7. Each protocol fact in the code has a source line in docs/protocol.md.

TASK 03-00-00-006 Make the Transport package
Branch: vlt-is-002/transport. Directory: internal/transport.
1. Interface: Open, Close and Exchange. Exchange sends one request and returns one response. It uses a context with a timeout. Only one Exchange runs at a time.
2. Exchange steps:
   a. Remove old input data. Read for a maximum of 100 ms.
   b. Write the full request. Confirm the written length.
   c. If the write is partial, do not write the remaining bytes. Close the Transport. Return an error (SEC-038).
   d. Read until CR or until the timeout.
   e. Return the raw bytes and the time in milliseconds.
3. Default timeout: 3 seconds. Minimum time between requests: 500 ms.
4. Serial Transport: go.bug.st/serial. 2400 bit/s, 8 data bits, no parity, 1 stop bit. Default: DTR on, RTS off.
5. HID Transport, Linux: hidraw with golang.org/x/sys/unix. Find the device by VID 0665 and PID 5161 through sysfs. Read the report descriptor. Use the report lengths from the descriptor. Do not use fixed lengths.
6. HID Transport, Windows: SetupAPI and hid.dll through golang.org/x/sys/windows. Use the report lengths from HidP_GetCaps.
7. If the device has no report IDs, start each report with byte 0. Split the request into reports. Fill the remaining bytes with zero.
   NOTE: The report type (output report or feature report) is UNVERIFIED. vlt-probe records the descriptor. The Owner Capture will confirm it.
8. Make a fake Transport for tests.
9. Build all of this package with CGO_ENABLED=0.

TASK 03-00-00-007 Make the simulator
Branch: vlt-is-002/sim. Directories: internal/sim and cmd/vlt-sim.
1. Model: mains on or off, battery charge, load, temperature, faults.
2. Dialect profiles: Q1 only, QS, Voltronic (QGS family). Use the vectors in 03-50-02 as base values.
3. Fake Transport that uses the model.
4. Fault injection: old bytes in the buffer, split responses, broken reports with missing bytes, NAK, echo, no response, wrong length.
5. The simulator records each received command. A test MUST fail if a tool sends a command that is not in the Probe List.
6. Linux only: a pty endpoint (creack/pty) for integration tests of the serial Transport.
7. cmd/vlt-sim starts the pty endpoint on Linux and shows the pty path.

TASK 03-00-00-008 Make vlt-probe
Branch: vlt-is-002/probe. Directory: cmd/vlt-probe.
WARNING: vlt-probe connects to the Reference UPS. It MUST send only the commands in the Probe List.
WARNING: The Builder runs vlt-probe only with the simulator. Only the Owner runs it with real hardware.
1. Modes:
   a. scan: list HID devices with VID 0665 and PID 5161, and list the serial ports. Send no command.
   b. probe: send each Probe List command one time, in the order of 03-50-01.
   c. watch: send the Watch List (03-50-01) every 2 seconds for 1 to 30 minutes. If a command returns "not supported" two times, do not send it again in this session.
2. Flags: --transport (hid or serial), --port (serial only), --minutes (watch only), --dtr and --rts (serial only). No other flags.
3. There is no flag for a raw command. The command lists are constants in the code.
4. Wait 500 ms between commands. Timeout: 3 seconds.
5. Capture file: JSON. Name: vlt-capture-<UTC date and time>.vltcap.json. Contents:
   a. Tool version, OS, Transport.
   b. Device data: VID, PID, product string, report descriptor in hex, report lengths.
   c. For each command: request in hex, response in hex, response as ASCII, time in ms, result (data, not supported, timeout, error).
6. Privacy: replace the QID data and the USB serial number string with "REDACTED" and the length. Do not record the host name or the user name.
7. Show a short summary in English at the end.
8. Build with CGO_ENABLED=0. CI makes the artifacts (TASK 03-00-00-004, step 9).
9. File docs/user/probe.md, in English and in Turkish. Contents:
   a. How to download the artifact and check the SHA-256.
   b. How to run scan, probe and watch.
   c. Linux: hidraw needs root. Use sudo for vlt-probe only.
   d. The mains test in watch mode, with this WARNING:
      "WARNING: Make sure that the battery is fully charged. Disconnect the UPS input power for 60 seconds or less. The protected equipment runs on battery during this time. Then connect the input power again."
   e. How to send the Capture file to the Owner channel. Tell the user to check the file before sending.

TASK 03-00-00-009 Protect the main branch
Do this task after the CI pull request is merged and the CI has run one time on main.
1. Make a ruleset for main: pull request required, 0 approvals, required status checks (the job names in ci.yml), linear history, no force push, no deletion.
2. If the API refuses, write it in Report 2. The Owner sets it in the GitHub UI.

TASK 03-00-00-010 Write the documentation
Put each file in the pull request of the related task.
1. docs/protocol.md: each Dialect fact with its source and its status (VERIFIED or UNVERIFIED).
2. docs/references.md: the SEC-060 source register.
3. docs/dependencies.md: the SEC-061 register (name, version, license, reason).
4. docs/architecture.md: the component model of VLT-IS-001 03-10-00. Text only.

====================================================================
03-50-00 REFERENCE DATA
====================================================================

03-50-01 Command lists
Probe List (fixed order):
Q1, QS, QGS, QMOD, QBV, QWS, QFLAG, QRI, F, I, QID, QVFW, QMD

Watch List:
Q1, QS, QGS, QMOD, QBV, QWS

03-50-02 Test vectors
NOTE: The CR at the end of each response is not shown.

Source A: Tunçmatik Newtech Pro 1 kVA, first generation (2010). Status: family sample. UNVERIFIED for the Reference UPS.
URL: https://lists.alioth.debian.org/pipermail/nut-upsuser/2010-February/005856.html
| Command | Response |
|---------|----------|
| Q1    | (209.7 209.7 219.7 019 50.0 2.32 23.3 00000001 |
| QS    | (209.7 209.7 219.7 020 50.0 41.8 23.3 00000001 |
| QGS   | (209.7 50.0 219.7 50.0 000.8 019 374.9 374.3 041.8 ---.- 023.3 100000000001 |
| QBV   | (041.8 03 01 100 058 |
| QRI   | (220.0 004 036.0 50.0 |
| F     | #220.0 004 036.0 50.0 |
| QMOD  | (L |
| QFLAG | (EpbshcDrovegfjl |
| QVFW  | (VERFW:00019.03 |
| QWS   | (0000000000000000000000000000000000000000000000000000000000000000 |
| QMD   | (############G1K ###1000 80 1/1 220 220 03 12.0 |
| QID   | (00000000000000 |
| Q     | (NAK |
NOTE: The QID value is fake. The source value is a real serial number. Do not use it.
NOTE: The exact spaces of the "I" response are not known. There is no "I" vector. Use the field widths in the public Megatec document.

Source B: standby UPS, Q1 over USB.
URL: https://gist.github.com/ao-kenji/7070940
| Command | Response |
|---------|----------|
| Q1 | (103.6 102.2 103.9 024 59.9 13.6 40.1 00101000 |
NOTE: Bit 3 is 1 (standby type). The battery value is the total voltage.

====================================================================
04-00-00 ACCEPTANCE CRITERIA
====================================================================
- The repository abuzer1495/voltiovolt is public and has the AGPL-3.0 LICENSE.
- Each commit has the author abuzer1495, the no-reply e-mail address and a DCO sign-off.
- CI passes on Linux and Windows.
- CGO_ENABLED=0 go build ./... passes.
- All parsers pass all test vectors. The fuzz smoke test passes.
- The encoder test of TASK 03-00-00-005 step 6 passes.
- A simulator test proves that vlt-probe sends only Probe List and Watch List commands.
- CI makes vlt-probe artifacts for Windows and Linux with SHA-256 files.
- docs/user/probe.md exists in English and Turkish.
- gitleaks finds no Secret, token or real personal data in the full history.

====================================================================
05-00-00 REPORT 2 REQUEST
====================================================================

05-00-01 General
1. Write Report 2 after all tasks.
2. Stop after Report 2. Do not start work that is not in this instruction.

05-10-00 Report format
1. Title: VLT-RPT-002, Revision 00.
2. Format: Markdown. Simplified Technical English.
3. Sections, in this order:
   1. Summary (5 sentences or fewer)
   2. Tasks (table: task number, status DONE, PARTIAL or NOT DONE, pull request link, notes)
   3. Command log (command and output summary; no tokens)
   4. Tests (command and result summary)
   5. Repository settings (table: setting, result, Owner action needed)
   6. Dependencies added (table: name, version, license, reason)
   7. Deviations (each difference from this instruction, with the reason)
   8. UNVERIFIED items
   9. Risks (new or changed only)
   10. Questions (for the Owner; for the Instruction Author)
   11. Sources
4. Do not add other sections.

END OF MESSAGE VLT-IS-002
