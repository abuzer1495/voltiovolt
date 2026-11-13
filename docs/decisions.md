# Decisions

This file has one row for each decision in VLT-IS-002 01-00-00, VLT-IS-003 01-00-00, VLT-IS-004 01-00-00, VLT-IS-005 01-00-00, VLT-IS-006 01-00-00 and VLT-IS-007 01-00-00. The source texts are in `docs/instructions/`.

| No. | Date | Decision | Source document |
|-----|------|----------|-----------------|
| QA-01 | 2026-11-10 | Accepted. "Q" is a CONTROL command. "D" is in no class. VLT never sends "D". Detection and vlt-probe use only the Probe List (03-50-01). | VLT-IS-002 Rev 00, 01-10-00 |
| QA-02 | 2026-11-10 | Accepted. The installer (root) creates and wraps the DEK. The unit file uses LoadCredentialEncrypted=. vltd never starts systemd-creds. The installer also writes the key mode (host, tpm2 or host+tpm2) to a root-owned file. This file contains no Secret. vltd can read it for the SEC-005 notice. | VLT-IS-002 Rev 00, 01-10-00 |
| QA-03 | 2026-11-10 | Accepted. The SEC-020 account on Windows is the virtual account NT SERVICE\vltd. The SeShutdownPrivilege method stays UNVERIFIED. A later instruction covers it. | VLT-IS-002 Rev 00, 01-10-00 |
| QA-04 | 2026-11-10 | Accepted. Use the gitleaks binary with a pinned version and a SHA-256 check. Do not use gitleaks-action. | VLT-IS-002 Rev 00, 01-10-00 |
| QA-05 | 2026-11-10 | Accepted. Use the words "EN 50160-based estimate". Do not use the word "compliance". | VLT-IS-002 Rev 00, 01-10-00 |
| QA-06 | 2026-11-10 | Accepted. Use direct OS APIs for HID. If the direct method fails on the Reference UPS, write it in a report. Do not change to go-hid without approval. | VLT-IS-002 Rev 00, 01-10-00 |
| QA-07 | 2026-11-10 | Decided now. Implementation is in a later instruction. IPC messages are JSON (Go standard library). Each frame has a 4-byte length prefix. Maximum frame size: 1 MiB. The decoder rejects unknown fields. Each message has a protocol version field. | VLT-IS-002 Rev 00, 01-10-00 |
| QA-08 | 2026-11-10 | Accepted for v1.0. Each paid Channel has a monthly message limit Setting. Default: 300 messages each month. | VLT-IS-002 Rev 00, 01-10-00 |
| QA-09 | 2026-11-10 | Accepted. The report format in 05-10-00 has a "Command log" section and a "Deviations" section. | VLT-IS-002 Rev 00, 01-10-00 |
| OD-01 | 2026-11-10 | Project license: AGPL-3.0-or-later. Each source file has the SPDX identifier in its header. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-02 | 2026-11-10 | Dependency licenses: The SEC-061 list stays the rule for all Go modules in VLT binaries. GPL-3.0 programs are permitted only as separate programs that the user installs, after Owner approval. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-03 | 2026-11-10 | VLT GUI: Wails v3 is approved. cgo is approved for the VLT GUI on Linux only. Front-end rules come in a later instruction. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-04 | 2026-11-10 | Build tools: WiX Toolset v7 is approved as a build tool. The project does not generate revenue. kardianos/service, Qt and gitleaks-action are not used. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-05 | 2026-11-10 | Turkish voice (v1.0): Primary method: pre-recorded Turkish prompts. The Owner decides the source of the recordings before v1.0. The voice-message provider Channel uses the provider text-to-speech. Do not distribute the Piper tr_TR dfki voice. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-06 | 2026-11-10 | GitHub identity: The Builder uses only the login abuzer1495. Refer to 02-00-00. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-07 | 2026-11-10 | Code signing: Apply to SignPath Foundation before the first public release. Not in this instruction. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-08 | 2026-11-10 | Telegram CONTROL confirmation (v1.0): A TOTP code (RFC 6238, 6 digits, 30 seconds) replaces the Remote PIN. The Owner adds the TOTP key to an authenticator application with a QR code in the VLT GUI. SEC-044 stays: delete the message that contains the code. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-09 | 2026-11-10 | Loss of Secrets: Accepted for the MVP. After a Windows reinstall or a move to a new host, the Secrets are lost. The VLT GUI has a "Vault reset" flow. It asks for all Secrets again. SEC-015 stays in v1.0. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-10 | 2026-11-10 | SECURITY.md contact: GitHub private vulnerability reporting only. No e-mail address. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-11 | 2026-11-10 | Hardware facts: The Owner gives the battery data, the RS-232 status and the test computers with the first capture. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-12 | 2026-11-10 | Reports in the repository: Do not commit VLT-RPT files. They contain account data. Write the decisions in docs/decisions.md. Commit the VLT-IS files in docs/instructions/. | VLT-IS-002 Rev 00, 01-20-00 |
| OD-13 | 2026-11-10 | Commit sign-off: Use DCO sign-off (git commit -s) for each commit. | VLT-IS-002 Rev 00, 01-20-00 |
| CH-01 | 2026-11-10 | VLT-IS-001 00-10-00: "Remote PIN" is replaced by "Remote code": a TOTP code for CONTROL requests from Telegram. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-02 | 2026-11-10 | VLT-IS-001 00-10-00: Add the term "Probe List": the ordered READ commands for detection. Refer to 03-50-01 of VLT-IS-002. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-03 | 2026-11-10 | VLT-IS-001 00-10-00: Add the term "Capture": a file from vlt-probe with raw UPS requests and responses. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-04 | 2026-11-10 | VLT-IS-001 01-00-02: License: AGPL-3.0-or-later. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-05 | 2026-11-10 | VLT-IS-001 01-00-05 item 3: cgo is permitted for the VLT GUI on Linux only. Tests can use cgo for the race detector. Shipped binaries other than the VLT GUI use CGO_ENABLED=0. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-06 | 2026-11-10 | VLT-IS-001 01-00-05 item 6: vltd can read the path in $CREDENTIALS_DIRECTORY. This path is not a Setting and not a Secret. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-07 | 2026-11-10 | VLT-IS-001 01-00-05 item 6: vlt-probe and vlt-sim are diagnostic tools, not vltd. They can use command-line flags to select a Transport, a port and a mode. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-08 | 2026-11-10 | VLT-IS-001 02-10-01: Add S-11: TOTP key. S-08 becomes: PIN hash. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-09 | 2026-11-10 | VLT-IS-001 SEC-004: The installer (root) creates and wraps the DEK. vltd reads the DEK through LoadCredentialEncrypted=. vltd does not start systemd-creds. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-10 | 2026-11-10 | VLT-IS-001 SEC-037 (new): The Driver sends only commands in the command registry. It rejects all other strings. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-11 | 2026-11-10 | VLT-IS-001 SEC-038 (new): Do not send a CR alone. Do not send the remaining bytes of a partial request. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-12 | 2026-11-10 | VLT-IS-001 SEC-043: "Remote PIN" is replaced by "Remote code". | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-13 | 2026-11-10 | VLT-IS-001 SEC-063: Use the gitleaks binary, not gitleaks-action. If the API does not permit push protection, the Owner sets it in the GitHub UI. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-14 | 2026-11-10 | VLT-IS-001 03-20-00: Add "Q" to the CONTROL class. "D" is in no class. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-15 | 2026-11-10 | VLT-IS-001 03-40-00: Power quality is an "EN 50160-based estimate". | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| CH-16 | 2026-11-10 | VLT-IS-001 05-10-00: Replaced by 05-10-00 of VLT-IS-002. | VLT-IS-002 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 01) |
| QA-1 | 2026-11-10 | Accepted. New SEC-007 text: "Keep the PIN only as an Argon2id hash. The TOTP key (S-11) is a Secret in the Vault." | VLT-IS-003 Rev 00, 01-10-00 |
| QA-2 | 2026-11-10 | The minimum time of 500 ms between requests has priority. A watch cycle starts 2 seconds after the start of the previous cycle, or at the end of the previous cycle. Use the later time. D-07 is accepted. | VLT-IS-003 Rev 00, 01-10-00 |
| QA-3 | 2026-11-10 | Accepted. The CI guard of D-01 stays. | VLT-IS-003 Rev 00, 01-10-00 |
| OD-14 | 2026-11-10 | Author name in squash commits (QO-01): Accepted. The name is already public on the GitHub profile. The Owner can change the profile name. The Builder does no action. | VLT-IS-003 Rev 00, 01-20-00 |
| OD-15 | 2026-11-10 | CODE_OF_CONDUCT.md contact (QO-02): Keep the text "the repository Owner on GitHub". Keep the attribution section of the Contributor Covenant without changes. | VLT-IS-003 Rev 00, 01-20-00 |
| OD-16 | 2026-11-10 | Dependabot security updates (QO-03): On. The Builder sets it in TASK 03-00-00-001. The Builder does not merge Dependabot pull requests. The Builder lists them in each report. | VLT-IS-003 Rev 00, 01-20-00 |
| OD-17 | 2026-11-10 | Confirmation of settings in the GitHub UI (QO-04): The Owner does this. It is not a Builder task. | VLT-IS-003 Rev 00, 01-20-00 |
| OD-18 | 2026-11-10 | Deviations D-01 to D-14 of VLT-RPT-002: Accepted. D-13 makes the new rule SEC-088. | VLT-IS-003 Rev 00, 01-20-00 |
| OD-19 | 2026-11-10 | GitHub token: If the Owner installs a fine-grained token for abuzer1495/voltiovolt only, use that token for all work. Then do not use any other token. Write which token type you used in the Command log. | VLT-IS-003 Rev 00, 01-20-00 |
| CH-17 | 2026-11-10 | VLT-IS-001 SEC-007: New text: "Keep the PIN only as an Argon2id hash. The TOTP key (S-11) is a Secret in the Vault." | VLT-IS-003 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 02) |
| CH-18 | 2026-11-10 | SEC-088 (new): Do not export GH_TOKEN or any other token in the shell. Give a token only as a prefix of one command. At the start and at the end of each task, check that no token variable is in the environment. Report only "present" or "absent". | VLT-IS-003 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 02) |
| CH-19 | 2026-11-10 | SEC-089 (new): Do not read, clone, list or change a repository other than abuzer1495/voltiovolt. | VLT-IS-003 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 02) |
| CH-20 | 2026-11-10 | SEC-090 (new): vltd starts and runs when the Vault is not available. Then all functions that need a Secret are off. The VLT GUI shows the Vault state. | VLT-IS-003 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 02) |
| CH-21 | 2026-11-10 | SEC-091 (new): Test-only code (fake DEK provider, fake host binding) is only in _test.go files. A shipped binary MUST NOT contain it. | VLT-IS-003 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 02) |
| CH-22 | 2026-11-10 | VLT-IS-001 03-20-00: Add NOTE: vltd polls in two groups (later instruction). Fast group: 2 commands or fewer, every 2 seconds. Slow group: all other READ commands, every 30 seconds. | VLT-IS-003 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 02) |
| QI-1 | 2026-11-10 | Accepted with changes. The fallback unit starts only when vltd.service enters the "failed" state. The primary unit uses Restart=on-failure, RestartSec=5, StartLimitBurst=3, StartLimitIntervalSec=60 and RestartPreventExitStatus=243. The variable name is VLT_FALLBACK_MODE=1. The Vault reason text is "primary unit failed". | VLT-IS-004 Rev 00, 01-10-00 |
| QI-2 | 2026-11-10 | Per source. The lockout key is the actor and the peer identity: the Linux uid of the peer, the Windows user SID of the peer, and later the Telegram chat ID. There is no global block. If the failures of all sources are more than 100 in one hour, write a SECURITY_ALERT audit entry. | VLT-IS-004 Rev 00, 01-10-00 |
| QI-3 | 2026-11-10 | Accepted. The polkit rule includes power-off-ignore-inhibit and hibernate-ignore-inhibit. vltd records the active inhibitors in the audit entry before each shutdown. | VLT-IS-004 Rev 00, 01-10-00 |
| QI-4 | 2026-11-10 | Changed. Pipe DACL: SYSTEM GA, NT SERVICE\vltd GA, Administrators GRGW, VLT Operators GRGW. | VLT-IS-004 Rev 00, 01-10-00 |
| QI-5 | 2026-11-10 | Yes. In this instruction, the development setup tool makes the group and gives the privileges (TASK 03-00-00-002). The MVP installer uses the same code later. | VLT-IS-004 Rev 00, 01-10-00 |
| QI-6 | 2026-11-10 | Accepted. A Setting never contains a Secret. A test MUST prove that no Setting key has the Secret type. | VLT-IS-004 Rev 00, 01-10-00 |
| OD-20 | 2026-11-10 | GitHub token (QO-3): No change. OD-19 stays. | VLT-IS-004 Rev 00, 01-20-00 |
| OD-21 | 2026-11-10 | Owner tests (QO-1, QO-2, QO-4): The Owner does them with docs/user/owner-tests.md (TASK 03-00-00-009). | VLT-IS-004 Rev 00, 01-20-00 |
| CH-23 | 2026-11-10 | VLT-IS-001 01-00-05 item 6: vltd can read the variable VLT_FALLBACK_MODE. It is a state flag, not a Setting and not a Secret. | VLT-IS-004 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 03) |
| CH-24 | 2026-11-10 | VLT-IS-001 SEC-024: New text: "Count incorrect PIN entries per source (actor and peer identity). Block that source only." | VLT-IS-004 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 03) |
| CH-25 | 2026-11-10 | SEC-092 (new): vltd sends a shutdown request to the OS only from the Policy Engine. No IPC method starts a real shutdown directly. | VLT-IS-004 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 03) |
| CH-26 | 2026-11-10 | SEC-093 (new): Any change of a Shutdown Policy makes the policy inactive. A new Simulation is necessary before activation (SEC-055). | VLT-IS-004 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 03) |
| CH-27 | 2026-11-10 | VLT-IS-001 03-30-00: Add the Events AVR_ACTIVE (INFO), UPS_SHUTDOWN_ACTIVE (CRITICAL) and UPS_WARNING_BITS_CHANGED (INFO). | VLT-IS-004 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 03) |
| Q-IA-1 | 2026-11-10 | Changed. A safety function MUST NOT depend on the Vault. When the Vault state is "locked", vltd ignores the stored policy Settings and uses the Fallback Policy (TASK 03-00-00-001 step 1). When the Vault state is "not configured", there is no active policy. The first-run wizard sets the policy. | VLT-IS-005 Rev 00, 01-10-00 |
| Q-IA-2 | 2026-11-10 | Accepted. Clear Events have the same type, severity INFO and "active": false. | VLT-IS-005 Rev 00, 01-10-00 |
| Q-IA-3 | 2026-11-10 | Accepted. Voltronic fast group: QS, QGS. Slow group: QMOD, QBV, QWS, QFLAG. | VLT-IS-005 Rev 00, 01-10-00 |
| Q-IA-4 | 2026-11-10 | Accepted. Pass rule for comm_lost_on_battery: a shutdown request at a time equal to or less than (comm lost time + trigger.comm_lost_on_battery_seconds + countdown_seconds + 10 s). | VLT-IS-005 Rev 00, 01-10-00 |
| Q-IA-5 | 2026-11-10 | Accepted. Methods without elevation need a set PIN. The error codes unavailable, no_simulation, low_battery and not_found are accepted. | VLT-IS-005 Rev 00, 01-10-00 |
| OD-22 | 2026-11-10 | IPC client for T-09 (Q-O-2): No command-line client. The VLT GUI of this instruction is the client. | VLT-IS-005 Rev 00, 01-20-00 |
| OD-23 | 2026-11-10 | Owner tests (Q-O-1): No change. The Owner does T-01 to T-09, then T-10 (TASK 03-00-00-009). | VLT-IS-005 Rev 00, 01-20-00 |
| CH-28 | 2026-11-10 | VLT-IS-001 00-10-00: Add the term "Fallback Policy": a fixed Shutdown Policy in the code. vltd uses it only when the Vault state is "locked". | VLT-IS-005 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 04) |
| CH-29 | 2026-11-10 | VLT-IS-001 SEC-055: Add: "The Fallback Policy is an exception. Its Simulation runs in CI." | VLT-IS-005 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 04) |
| CH-30 | 2026-11-10 | SEC-094 (new): Do not add a dependency version (Go module or npm package) that was published less than 14 days before the day you add it. | VLT-IS-005 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 04) |
| CH-31 | 2026-11-10 | SEC-095 (new): The VLT GUI loads no remote content. All front-end files are in the binary. The Content Security Policy is "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'". No eval. No inline scripts. | VLT-IS-005 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 04) |
| CH-32 | 2026-11-10 | SEC-096 (new): The front end never receives a Secret value. A Secret goes from the input field to the Go side of the VLT GUI, then to vltd with secret.set. The Go side clears the buffer after the call. | VLT-IS-005 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 04) |
| CH-33 | 2026-11-10 | SEC-097 (new): Required status checks on main use the "strict" mode: a branch must be up to date with main before the merge. | VLT-IS-005 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 04) |
| Q-IA-1 | 2026-11-10 | Changed. Settings with the key prefix "ui." are not security Settings. A client can write them without the elevated state. A set PIN is necessary. Write an audit entry for each change. | VLT-IS-006 Rev 00, 01-10-00 |
| Q-IA-2 | 2026-11-10 | Accepted. The package internal/ipc/client/typed stays. | VLT-IS-006 Rev 00, 01-10-00 |
| Q-IA-3 | 2026-11-10 | Accepted. The Simulation discharge model (0.1 % each virtual second on battery) and the pass-rule limits of the Fallback Policy stay. | VLT-IS-006 Rev 00, 01-10-00 |
| Q-IA-4 | 2026-11-10 | Accepted with a condition: update cmd/vlt-gui to Wails v3.1.0 only if the work starts on 2026-11-15 or later. The work of VLT-IS-006 started on 2026-11-10: no update (VLT-RPT-006 section 7). | VLT-IS-006 Rev 00, 01-10-00 |
| Q-IA-5 | 2026-11-10 | Yes. VLT-IS-007 adds the Channel screens with Secret input fields. | VLT-IS-006 Rev 00, 01-10-00 |
| OD-24 | 2026-11-10 | VLT-REF-001 is an accepted source for SEC-060. It is the Owner interoperability analysis. It contains behavior only and synthetic examples. | VLT-IS-006 Rev 00, 01-20-00 |
| OD-25 | 2026-11-10 | The folder ANALIZ on the Builder computer is private. Do not read it. Do not commit it. Use only VLT-REF-001. | VLT-IS-006 Rev 00, 01-20-00 |
| OD-26 | 2026-11-10 | Primary path of the Reference UPS: serial Megatec (Q1, F, I) on a virtual COM port. HID VID 0665 PID 5161 is not the path of the Reference UPS. It stays supported for other units. | VLT-IS-006 Rev 00, 01-20-00 |
| OD-27 | 2026-11-10 | The user selects the serial port in the VLT GUI. vltd does not send bytes to a serial port that the user did not select or confirm. | VLT-IS-006 Rev 00, 01-20-00 |
| OD-28 | 2026-11-10 | UPS output off: the function exists in the MVP. The default is off. The Owner turns it on only after test T-12 passes. | VLT-IS-006 Rev 00, 01-20-00 |
| OD-29 | 2026-11-10 | The roadmap in VLT-IS-006 01-40-00 (docs/roadmap.md) replaces all earlier plans. | VLT-IS-006 Rev 00, 01-20-00 |
| CH-34 | 2026-11-10 | VLT-IS-001 01-00-04: new rows "Connection: USB cable. The OS shows a virtual COM port (FIELD, VLT-REF-001).", "Bridge chip VID and PID: UNVERIFIED.", "Working path: serial Megatec Q1, F, I (FIELD)." The row "USB identity 0665:5161" becomes "Not the path of the Reference UPS." | VLT-IS-006 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 05) |
| CH-35 | 2026-11-10 | VLT-IS-001 00-10-00: Dialect names megatec (Q1, F, I), voltronic-qs (QS family), voltronic (QGS family). | VLT-IS-006 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 05) |
| CH-36 | 2026-11-10 | VLT-IS-001 03-20-00: add "C" (cancel shutdown) to the CONTROL class. UNVERIFIED. VLT does not send it automatically. | VLT-IS-006 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 05) |
| CH-37 | 2026-11-10 | VLT-IS-001 SEC-060: add "VLT-REF-001 is an accepted source." | VLT-IS-006 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 05) |
| CH-38 | 2026-11-10 | SEC-098 (new): Only the package internal/control can make CONTROL request bytes. Only these callers can use internal/control: the IPC handlers for an elevated request with a confirmation, and the Policy Engine for the UPS output off step. | VLT-IS-006 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 05) |
| CH-39 | 2026-11-10 | SEC-099 (new): vltd does not send a byte to a serial port that the user did not select or confirm in the VLT GUI. | VLT-IS-006 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 05) |
| CH-40 | 2026-11-10 | SEC-100 (new): VLT never sends "S.0". The permitted shutdown delays are 0.2 to 0.9 minutes (12 to 54 seconds, steps of 6 seconds) and 1 to 10 minutes. | VLT-IS-006 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 05) |
| CH-41 | 2026-11-10 | SEC-101 (new): Do not commit a file or a folder of the Owner analysis. A CI check fails if the repository contains the path "ANALIZ/" or a Ghidra project file. | VLT-IS-006 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 05) |
| Q-IA-1 | 2026-11-13 | Accepted for now. On Windows, DTR and RTS are on after the open. Do not change the library. If T-01 shows a problem, a later instruction changes it. | VLT-IS-007 Rev 00, 01-10-00 |
| Q-IA-2 | 2026-11-13 | Accepted. | VLT-IS-007 Rev 00, 01-10-00 |
| Q-IA-3 | 2026-11-13 | In this instruction (TASK 03-00-00-002). SEC-094 is suspended (01-30-00). | VLT-IS-007 Rev 00, 01-10-00 |
| Q-IA-4 | 2026-11-13 | No VLT action. The computer is off at that time. The GUI text tells the user the behavior after T-12. A later instruction adds the result of T-12 to the text. | VLT-IS-007 Rev 00, 01-10-00 |
| OD-30 | 2026-11-13 | Test builds: GitHub Actions builds all binaries. The Builder does not build packages on the Builder computer for the Owner. | VLT-IS-007 Rev 00, 01-20-00 |
| OD-31 | 2026-11-13 | Test releases: The Owner approves a pre-release with the fixed tag "test-latest". The workflow replaces it at each merge to main. Real releases still need Owner approval (SEC-067). | VLT-IS-007 Rev 00, 01-20-00 |
| OD-32 | 2026-11-13 | Process: Less security process now. The security hardening step comes before release v1.0 (01-40-00). | VLT-IS-007 Rev 00, 01-20-00 |
| OD-33 | 2026-11-13 | E-mail in the MVP: SMTP with a password (Gmail: app password). Gmail OAuth moves to the security hardening step. | VLT-IS-007 Rev 00, 01-20-00 |
| CH-42 | 2026-11-13 | SEC-094: Suspended until the security hardening step. | VLT-IS-007 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 06) |
| CH-43 | 2026-11-13 | SEC-088: Keep: do not export a token in the shell. Remove: the token check log at the start and the end of each task. | VLT-IS-007 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 06) |
| CH-44 | 2026-11-13 | SEC-067: Add: "Exception: the pre-release test-latest (OD-31)." | VLT-IS-007 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 06) |
| CH-45 | 2026-11-13 | New code: New code needs normal unit tests. New go/ast tests and fuzz targets are not necessary. | VLT-IS-007 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 06) |
| CH-46 | 2026-11-13 | Report: The report format is shorter (05-10-00). | VLT-IS-007 Rev 00, 01-30-00 (makes VLT-IS-001 Revision 06) |
