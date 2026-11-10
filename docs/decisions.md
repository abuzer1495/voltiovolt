# Decisions

This file has one row for each decision in VLT-IS-002 01-00-00. The source text is in `docs/instructions/VLT-IS-002.md`.

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
