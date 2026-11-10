# Security

This file is a summary of the security requirements of VLT-IS-001 Revision 00 (section 02-00-00), with the changes of VLT-IS-002 01-30-00 (VLT-IS-001 Revision 01) and VLT-IS-003 01-30-00 (VLT-IS-001 Revision 02). It also has the GitHub rules of VLT-IS-002 02-00-00. The full source text is in `docs/instructions/`. If this file and the instructions do not agree, the instructions apply.

## 1. Priority

1. Security requirements have priority over all other requirements.
2. If a task conflicts with a security requirement, stop the task. Write the conflict in the report.
3. Do not make a security requirement weaker to make a task easier.

## 2. Secret list

| ID | Secret | Change |
|----|--------|--------|
| S-01 | Telegram bot token |  |
| S-02 | Gmail OAuth refresh token and access token |  |
| S-03 | Google OAuth client secret (created by the user) |  |
| S-04 | Gmail app password (SMTP fallback) |  |
| S-05 | WhatsApp Cloud API access token |  |
| S-06 | SIP account password |  |
| S-07 | Voice-message provider credentials |  |
| S-08 | PIN hash | Changed by VLT-IS-002 01-30-00 (was: PIN hash and Remote PIN hash) |
| S-09 | DEK |  |
| S-10 | VLT Agent keys (v2.0) |  |
| S-11 | TOTP key | New: VLT-IS-002 01-30-00, OD-08 |

## 3. SEC list


### 02-10-02 Storage

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-001 | Encrypt each Secret with AES-256-GCM and the DEK. Use a new random nonce for each encryption. |  |
| SEC-002 | Do not write the DEK to disk in plain form. |  |
| SEC-003 | Windows: wrap the DEK with DPAPI in the user scope of the vltd service account. Add application-specific entropy. Do not use the LocalMachine scope. |  |
| SEC-004 | Linux: the installer (root) creates and wraps the DEK with systemd-creds (TPM2 plus the host key when a TPM2 chip is available). vltd reads the DEK through LoadCredentialEncrypted=. vltd does not start systemd-creds. | VLT-IS-002 01-30-00 (SEC-004), QA-02 |
| SEC-005 | Linux without TPM2: systemd-creds uses the host key only. Show a "reduced protection" notice in the VLT GUI. |  |
| SEC-006 | A copy of the database on a different host MUST NOT decrypt. Write a test for this. |  |
| SEC-007 | Keep the PIN only as an Argon2id hash. The TOTP key (S-11) is a Secret in the Vault. | VLT-IS-003 01-30-00 (answer QA-1 to VLT-RPT-002) |

### 02-10-03 Exposure

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-010 | Secret fields in the VLT GUI are write-only. After save, show only the last 4 characters. |  |
| SEC-011 | No IPC message, export, report or log contains a Secret. |  |
| SEC-012 | Do not put Secrets in environment variables, command-line arguments, crash dumps or child processes. |  |
| SEC-013 | Remove Secrets from all log output. Write tests for this redaction. |  |
| SEC-014 | The Settings export does not contain Secrets. |  |
| SEC-015 | v1.0: an encrypted backup can contain Secrets. It uses a passphrase with Argon2id. It needs the PIN. |  |
| SEC-016 | Keep decrypted Secrets in memory only for the time of use. Keep the DEK in memory only inside vltd. |  |

### 02-20-00 Access control

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-020 | vltd runs as a dedicated service account. Give it only the rights for UPS access and system shutdown. |  |
| SEC-021 | IPC access: Windows: Administrators and the local group "VLT Operators". Linux: root and the group "vlt". |  |
| SEC-022 | The VLT GUI asks for the PIN before these actions: Settings change, Secret entry, CONTROL command, CONFIG command, Shutdown Policy activation. |  |
| SEC-023 | The VLT GUI locks after 10 minutes without input. The Owner can change this value. |  |
| SEC-024 | After 5 incorrect PIN entries, block PIN entry for 5 minutes. Double the block time after each new failure. |  |
| SEC-025 | Write an audit entry for each Settings change, each CONTROL or CONFIG command, each incorrect PIN and each rejected remote request. |  |

### 02-30-00 UPS command safety

WARNING: CONTROL commands and CONFIG commands can stop power to the protected equipment. CONFIG commands change the UPS permanently. Do not send CONTROL or CONFIG commands to real hardware. Only the Owner does tests on real hardware.

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-030 | Automatic detection sends READ commands only. |  |
| SEC-031 | vltd sends a CONTROL command only for one of these: an operator request with the PIN, a Telegram request with the Remote code, a Shutdown Policy trigger. | Term change "Remote PIN" -> "Remote code" (VLT-IS-002 01-30-00, 00-10-00; OD-08). Refer to question in VLT-RPT-002. |
| SEC-032 | vltd sends a CONFIG command only after an operator request with the PIN and a second confirmation. |  |
| SEC-033 | Parse each UPS response strictly. Check the start character, the field count, the field format and the maximum length. Reject all other data. |  |
| SEC-034 | Fuzz-test each parser. |  |
| SEC-035 | Before each command on USB, remove old data from the input buffer. Match each response to its request. |  |
| SEC-036 | Development and CI use a UPS simulator only. |  |
| SEC-037 | The Driver sends only commands in the command registry. It rejects all other strings. | New: VLT-IS-002 01-30-00 |
| SEC-038 | Do not send a CR alone. Do not send the remaining bytes of a partial request. | New: VLT-IS-002 01-30-00 |

### 02-40-00 Channels

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-040 | Telegram: use long polling (getUpdates). Do not use a webhook. Do not open an inbound port. |  |
| SEC-041 | Telegram: pair a chat with a one-time code. The code expires after 10 minutes. |  |
| SEC-042 | Telegram: accept requests only from paired chat IDs. Ignore all other chats. Write an audit entry for each. |  |
| SEC-043 | Telegram: use two roles, Viewer and Admin. A CONTROL request needs the Admin role, an inline confirmation and the Remote code. The confirmation expires after 60 seconds. | VLT-IS-002 01-30-00 (SEC-043) |
| SEC-044 | Telegram: delete the message that contains the Remote code after you read it. | Term change "Remote PIN" -> "Remote code" (VLT-IS-002 01-30-00, 00-10-00; OD-08). Refer to question in VLT-RPT-002. |
| SEC-045 | Gmail: use OAuth 2.0 with PKCE and a loopback redirect to 127.0.0.1. Request only the scope https://www.googleapis.com/auth/gmail.send. |  |
| SEC-046 | Gmail: the "Disconnect" action revokes the token at Google. Then it deletes the token from the Vault. |  |
| SEC-047 | SMTP fallback: use TLS on port 465 or STARTTLS on port 587. If STARTTLS fails, stop the send. |  |
| SEC-048 | All outbound TLS connections: verify the certificate. Use TLS 1.2 or later. Do not add an option to disable verification. |  |
| SEC-049 | WhatsApp: use the official Cloud API with approved utility templates only. |  |
| SEC-050 | SIP: use TLS and SRTP when the provider supports them. If not, show a warning in the VLT GUI. |  |
| SEC-051 | Notification text does not contain Secrets. It does not contain serial numbers or IP addresses unless the Owner enables them. |  |

### 02-50-00 Shutdown and scripts

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-055 | A Shutdown Policy cannot be active before one successful Simulation. |  |
| SEC-056 | Show a countdown with a cancel button before each automatic shutdown. Default: 60 seconds. |  |
| SEC-057 | User scripts run only from a protected directory. vltd records the SHA-256 hash of each script. If the hash changes, vltd does not run the script and sends an Alarm. |  |
| SEC-058 | Do not build shell command strings. Start each process with an argument list. |  |

### 02-60-00 Source code and supply chain

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-060 | Clean-room rule: do not copy code or text from NUT, UPSilon 2000, ViewPower or other GPL or proprietary sources. Use only protocol facts from public documents. Record each source in docs/references.md. |  |
| SEC-061 | For each dependency, record the name, version, license and reason. Permitted licenses: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, MPL-2.0. Other licenses need Owner approval. |  |
| SEC-062 | Pin all dependency versions. Run govulncheck in CI. |  |
| SEC-063 | Enable GitHub secret scanning and push protection. Run the gitleaks binary in CI, not gitleaks-action. If the API does not permit push protection, the Owner sets it in the GitHub UI. | VLT-IS-002 01-30-00 (SEC-063), QA-04 |
| SEC-064 | Do not commit Secrets, real tokens, real phone numbers or real e-mail addresses. In tests, use example.com addresses and clearly fake numbers. |  |
| SEC-065 | After the first commit, do not push to the main branch directly. Use one branch and one pull request for each task. Merge only after Owner approval. |  |
| SEC-066 | Do not force-push. Do not rewrite history. Do not change GitHub account settings. Do not create or delete other repositories. |  |
| SEC-067 | Each release has SHA-256 checksums and an SBOM (SPDX or CycloneDX). Do not publish a release without Owner approval. |  |
| SEC-068 | Add SECURITY.md. Enable GitHub private vulnerability reporting. |  |

### 02-70-00 Builder integrity

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-070 | Do not invent facts. If you do not know a fact, write UNVERIFIED. |  |
| SEC-071 | Do not invent library names, versions or functions. Give a source URL for each. |  |
| SEC-072 | Do not report a test as passed if you did not run it. Give the command and a summary of the output. |  |
| SEC-073 | Do not change the scope. Write scope proposals in your report. |  |
| SEC-074 | If an instruction is not clear, do not guess. Write a question in your report. |  |

### GitHub and build rules (VLT-IS-002 02-00-00, VLT-IS-003 01-30-00)

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-080 | Use only the login abuzer1495 for all GitHub API calls and all git pushes. | VLT-IS-002 02-00-00 |
| SEC-081 | Before the first push of each task, confirm the login with the GitHub API. Write the result in the Command log. | VLT-IS-002 02-00-00 |
| SEC-082 | Set the git author and committer in the local configuration of this repository only. Name: abuzer1495. E-mail: the GitHub no-reply address of abuzer1495. | VLT-IS-002 02-00-00 |
| SEC-083 | Do not change the global git configuration. Use a token for each command when possible. If you must change the active gh login, change it back after the task. Write it in the report. | VLT-IS-002 02-00-00 |
| SEC-084 | Do not write the name of any other GitHub login in the repository. | VLT-IS-002 02-00-00 |
| SEC-085 | One task, one branch, one pull request. Branch name: vlt-is-00N/<task-name>. | VLT-IS-002 02-00-00, VLT-IS-003 02-00-00 |
| SEC-086 | The instruction gives Owner approval to merge each pull request of the instruction. Conditions: all required checks pass, and the pull request contains only the work of its task. Use squash merge. | VLT-IS-002 02-00-00 |
| SEC-087 | Do not commit binaries. Do not commit Capture files. | VLT-IS-002 02-00-00 |
| SEC-088 | Do not export GH_TOKEN or any other token in the shell. Give a token only as a prefix of one command. At the start and at the end of each task, check that no token variable is in the environment. Report only "present" or "absent". | New: VLT-IS-003 01-30-00 |
| SEC-089 | Do not read, clone, list or change a repository other than abuzer1495/voltiovolt. | New: VLT-IS-003 01-30-00 |

### vltd rules (VLT-IS-003 01-30-00)

| ID | Requirement | Change |
|----|-------------|--------|
| SEC-090 | vltd starts and runs when the Vault is not available. Then all functions that need a Secret are off. The VLT GUI shows the Vault state. | New: VLT-IS-003 01-30-00 |
| SEC-091 | Test-only code (fake DEK provider, fake host binding) is only in _test.go files. A shipped binary MUST NOT contain it. | New: VLT-IS-003 01-30-00 |

NOTE: VLT-IS-001 has no SEC-008, SEC-009, SEC-017 to SEC-019, SEC-026 to SEC-029, SEC-039, SEC-052 to SEC-054, SEC-059 and SEC-069.

## 4. Threat model summary

VLT-IS-001 has no separate threat model section. This summary is made from VLT-IS-001 02-00-00 and 03-50-00. It adds no new requirement.

| No. | Asset | Threat | Controls |
|-----|-------|--------|----------|
| T-01 | Secrets S-01 to S-11 (messaging accounts, keys) | Theft from disk, from a copied database, from logs, IPC messages, exports, environment variables or child processes | SEC-001 to SEC-007, SEC-010 to SEC-016, SEC-051 |
| T-02 | Power to the protected equipment | A CONTROL or CONFIG command sent by error, by a broken frame or by an attacker | SEC-022, SEC-030 to SEC-038, SEC-055, SEC-056 |
| T-03 | UPS settings (EEPROM) | A permanent change by a CONFIG command | SEC-032, SEC-037 |
| T-04 | vltd and local IPC | A local user without rights sends requests to vltd | SEC-020, SEC-021, SEC-022, SEC-024, SEC-025 |
| T-05 | Telegram Channel | A stranger or a stolen chat sends commands; a Remote code stays in the chat history | SEC-040 to SEC-044 |
| T-06 | E-mail, WhatsApp and SIP Channels | Token theft, downgrade of TLS, interception | SEC-045 to SEC-050 |
| T-07 | Shutdown Policies and user scripts | Wrong or changed script, shell injection, unwanted shutdown | SEC-055 to SEC-058 |
| T-08 | Source code and supply chain | Copied GPL or proprietary code, bad licenses, vulnerable or malicious dependencies, leaked tokens, history rewrite | SEC-060 to SEC-068 |
| T-09 | Builder output | Invented facts, versions or test results | SEC-070 to SEC-074 |
| T-10 | Alarm delivery during a power failure | Internet Channels fail when the modem and router are not on UPS power | VLT-IS-001 03-50-00 item 1 (first-run wizard warning) |

## 5. Redaction rules (VLT-IS-003 TASK 03-00-00-002)

Package `internal/redact`:

1. The type `redact.Secret` holds Secret bytes behind a pointer.
2. These methods return the text "[REDACTED]": String, GoString, Format (all verbs), MarshalJSON, MarshalText and LogValue. A Secret in an unexported struct field shows only a pointer address.
3. Only the method `Expose` gives the bytes. Use it only in the Vault and in Channel code.
4. The log/slog handler `redact.Handler` replaces each attribute of the Secret type with "[REDACTED]". It also applies `redact.Scrub` to the message, to attribute keys, to text values, to error values and to the text form of all other values.
5. `redact.Scrub` removes:
   - the path segment "/bot" with a token (result: "/bot[REDACTED]"),
   - bare Telegram bot tokens (8 to 10 digits, ":", 35 characters),
   - "Authorization:" header values (to the end of the line),
   - "Bearer" tokens,
   - URL query values with the names token, key, secret, password and access_token.
6. Package `internal/logging` makes the vltd logger. All output goes through `redact.Handler`. Linux: standard error (journal). Windows: `<data directory>\logs\vltd.log`, 5 files of 10 MiB each.
7. Tests use fake token values made at run time. No token text is in the repository.

## 6. Vault design (VLT-IS-003 TASK 03-00-00-004)

WARNING: An error in the Vault can expose all user Secrets.

1. DEK: 32 bytes from crypto/rand.
2. Encryption: AES-256-GCM. A new 96-bit random nonce for each encryption. Additional data: "vlt-secret-v1|" + Secret ID + "|" + version. A ciphertext moved to a different Secret ID or version does not decrypt.
3. Linux host binding: systemd decrypts the credential `vlt-dek` (LoadCredentialEncrypted=) before vltd starts. vltd reads `$CREDENTIALS_DIRECTORY/vlt-dek` (exactly 32 bytes). vltd never starts systemd-creds. vltd reads the key mode from `/etc/vltd/key-mode` (root-owned, not writable by group or others): host, tpm2 or host+tpm2. "host" gives the state "reduced protection" (SEC-005). A missing or bad key mode file also gives "reduced protection" (reason "key mode unknown").
4. Windows host binding: at the first start, vltd makes the DEK and wraps it with CryptProtectData (user scope of NT SERVICE\vltd, not CRYPTPROTECT_LOCAL_MACHINE, flag CRYPTPROTECT_UI_FORBIDDEN, no prompt structure). Entropy: "VoltioVolt-DEK-v1" + the install ID (meta table). File: `vault.dek` in the data directory.
5. Canary: when the Vault is made, vltd stores the Secret `vlt.canary` with a known value. At each start, vltd decrypts it. If this fails, the state is "locked" (reason "host binding failed"); vltd writes an audit entry and a log entry, without the canary value.
6. States: available, reduced protection, locked, not configured. In "locked" and "not configured", Set, Delete, Use and settings writes return an error; vltd continues (SEC-090). Secret status (set or not set, hint, updated_at) works in all states.
7. Functions: Set (version + 1), Delete, Status (never the value), Use (buffer cleared after the function returns), Reset (confirmation text "RESET VAULT"; deletes all Secrets; Windows: new DEK; Linux: same DEK, new canary; valid settings get a new MAC).
8. A go/ast test fails if a package outside internal/vault calls Use. The allow list is empty.
9. Settings integrity: the Store keeps an HMAC-SHA256 of key, value and version for each settings row. The MAC key comes from the DEK through HKDF-SHA256 (no salt, info "vlt-settings-mac-v1", 32 bytes). A row with a MAC that is not correct is not used: vltd uses the default value and writes an audit entry and a log entry.
10. Secret IDs: letters, digits, ".", "_" and "-", 64 characters or fewer. The prefix "vlt." is reserved.
11. Hint: the last 4 characters of a Secret of 16 characters or more. No hint for a shorter Secret.
12. Test-only DEK providers are only in _test.go files (SEC-091).

