# VOLTIOVOLT (VLT UPS MANAGEMENT)
## INSTRUCTION SET – MESSAGE 3
## DECISIONS ON REPORT 2 AND vltd FOUNDATION: REDACTION, STORE, VAULT, AUTH, AUDIT, IPC, SERVICE

### DOCUMENT IDENTIFICATION

- Document number: VLT-IS-003
- Revision: 00
- Date: 2026-11-10
- Reference documents: VLT-IS-001 Revision 01 (Revision 00 with the changes in VLT-IS-002 01-30-00). VLT-IS-002 Revision 00. VLT-RPT-002 Revision 00.
- Writing standard: ATA iSpec 2200 structure. ASD-STE100 Simplified Technical English.
- Effectivity: vltd foundation. Release: MVP.
- Type: Build instruction.

---

## 00-00-00 GENERAL

### 00-00-01 Purpose

1. This instruction gives the decisions on VLT-RPT-002.
2. This instruction builds the vltd foundation: redaction, logging, Store, Vault, PIN authentication, audit log, IPC and the service skeleton.
3. Do only the tasks in 03-00-00. Then write Report 3. Refer to 05-00-00.

### 00-00-02 Not in this instruction

- Poller, Driver selection inside vltd, Event Engine, Analyzer, Policy Engine, Channels, VLT GUI.
- Installers (MSI, deb, rpm). This instruction has development install files only.
- TOTP (v1.0).
- Encoders for CONTROL commands and CONFIG commands.
- Releases and tags.

### 00-00-03 Failure rule

CAUTION: If a task fails, do not use a workaround that changes the scope.

1. Stop the failed task.
2. Continue with the tasks that do not depend on it.
3. Write the failure in Report 3.

---

## 01-00-00 DECISIONS

### 01-10-00 Answers to the questions for the Instruction Author

| No. | Decision |
|-----|----------|
| QA-1 | Accepted. New SEC-007 text: "Keep the PIN only as an Argon2id hash. The TOTP key (S-11) is a Secret in the Vault." |
| QA-2 | The minimum time of 500 ms between requests has priority. A watch cycle starts 2 seconds after the start of the previous cycle, or at the end of the previous cycle. Use the later time. D-07 is accepted. |
| QA-3 | Accepted. The CI guard of D-01 stays. |

### 01-20-00 Owner decisions

| No. | Subject | Decision |
|-----|---------|----------|
| OD-14 | Author name in squash commits (QO-01) | Accepted. The name is already public on the GitHub profile. The Owner can change the profile name. The Builder does no action. |
| OD-15 | CODE_OF_CONDUCT.md contact (QO-02) | Keep the text "the repository Owner on GitHub". Keep the attribution section of the Contributor Covenant without changes. |
| OD-16 | Dependabot security updates (QO-03) | On. The Builder sets it in TASK 03-00-00-001. The Builder does not merge Dependabot pull requests. The Builder lists them in each report. |
| OD-17 | Confirmation of settings in the GitHub UI (QO-04) | The Owner does this. It is not a Builder task. |
| OD-18 | Deviations D-01 to D-14 of VLT-RPT-002 | Accepted. D-13 makes the new rule SEC-088. |
| OD-19 | GitHub token | If the Owner installs a fine-grained token for abuzer1495/voltiovolt only, use that token for all work. Then do not use any other token. Write which token type you used in the Command log. |

### 01-30-00 Changes to VLT-IS-001

These changes make VLT-IS-001 Revision 02.

| Item | Change |
|------|--------|
| SEC-007 | New text: "Keep the PIN only as an Argon2id hash. The TOTP key (S-11) is a Secret in the Vault." |
| SEC-088 (new) | Do not export GH_TOKEN or any other token in the shell. Give a token only as a prefix of one command. At the start and at the end of each task, check that no token variable is in the environment. Report only "present" or "absent". |
| SEC-089 (new) | Do not read, clone, list or change a repository other than abuzer1495/voltiovolt. |
| SEC-090 (new) | vltd starts and runs when the Vault is not available. Then all functions that need a Secret are off. The VLT GUI shows the Vault state. |
| SEC-091 (new) | Test-only code (fake DEK provider, fake host binding) is only in _test.go files. A shipped binary MUST NOT contain it. |
| 03-20-00 | Add NOTE: vltd polls in two groups (later instruction). Fast group: 2 commands or fewer, every 2 seconds. Slow group: all other READ commands, every 30 seconds. |

---

## 02-00-00 GENERAL RULES

1. The GitHub rules SEC-080 to SEC-087 of VLT-IS-002 stay. Branch name: vlt-is-003/task-name.
2. SEC-086 gives Owner approval to merge each pull request of this instruction. The conditions of SEC-086 stay.
3. Build all code with CGO_ENABLED=0.
4. Record each new dependency in docs/dependencies.md. The license check of CI MUST pass.
5. The Builder does not run vltd on real hardware. Only the Owner does tests on real hardware.

---

## 03-00-00 TASKS

### TASK 03-00-00-001 Set Dependabot security updates

1. Check the environment for token variables (SEC-088). Write "present" or "absent".
2. Turn on Dependabot security updates with the GitHub API.
3. Read the setting back. Write the result in Report 3.

### TASK 03-00-00-002 Make the redaction and logging packages

Branch: vlt-is-003/redact. Directories: internal/redact and internal/logging.

1. Make a Secret type that holds bytes.
2. These methods of the Secret type return the text "[REDACTED]": String, GoString, Format, MarshalJSON, MarshalText and LogValue.
3. Only one method gives the bytes. Name it Expose. Write a comment: "Use only in the Vault and in Channel code."
4. Make a log/slog handler. It replaces each attribute of the Secret type with "[REDACTED]".
5. The handler also removes these patterns from all text: Telegram bot tokens, the path segment "/bot" with a token, "Authorization:" header values, "Bearer" tokens and URL query values with the names "token", "key", "secret", "password" or "access_token".
6. Logging output:
    1. Linux: standard error, for the journal.
    2. Windows: a log file in the data directory. Keep 5 files of 10 MiB each.
7. Tests:
    1. fmt verbs %v, %+v, %#v, %s and %q on a Secret and on a struct that contains a Secret.
    2. encoding/json on the same values.
    3. slog text handler and slog JSON handler.
    4. An error from net/http for a request URL that contains a fake bot token. The log output MUST NOT contain the token.

### TASK 03-00-00-003 Make the Store

Branch: vlt-is-003/store. Directory: internal/store.

1. Use modernc.org/sqlite. Set WAL mode, foreign keys on and a busy timeout of 5 seconds.
2. Migrations: SQL files embedded in the binary, numbered, forward only. Record the schema version in a table.
3. Tables:

| Table | Columns (minimum) | Use |
|-------|-------------------|-----|
| meta | key, value | Install ID (16 random bytes), schema data |
| settings | key, value (JSON), version, updated_at, updated_by, mac | Settings |
| secrets | id, version, nonce, ciphertext, hint, updated_at | Vault data |
| measurements | ts, name, value, unit, source | Later instruction. Schema only. |
| events | ts, type, severity, data (JSON) | Later instruction. Schema only. |
| audit | refer to TASK 03-00-00-006 | Audit log |
| auth_state | key, value | PIN hash, failed-attempt count, block end time |

4. Hint rule: the hint is the last 4 characters of a Secret. Store a hint only for a Secret of 16 characters or more. For a shorter Secret, store no hint.
5. Settings integrity:
    1. Each settings row has an HMAC-SHA256 value in the column "mac".
    2. The MAC covers key, value and version.
    3. The MAC key comes from the DEK through HKDF-SHA256 with the info text "vlt-settings-mac-v1".
    4. If a MAC is not correct, do not use the value. Use the default value. Write an audit entry and a log entry.
    5. If the Vault is not available, settings writes are not possible. Settings reads use the default values. Write this state to the log.
6. Data directory and permissions:
    1. Linux: /var/lib/vltd, mode 0700, owner vltd. Database file mode 0600.
    2. Windows: %ProgramData%\VoltioVolt. ACL: SYSTEM full control, Administrators full control, NT SERVICE\vltd modify. No other entries. No inheritance from the parent.
    3. If the permissions are less strict than these values at start, vltd stops. It writes the reason to the log.
7. Tests: migration from an empty database, a database that is not valid, a changed settings row (MAC test), the permission check on Linux.

### TASK 03-00-00-004 Make the Vault

Branch: vlt-is-003/vault. Directory: internal/vault.

WARNING: An error in this package can expose all user Secrets. Do each step. Do not make a simpler method.

1. The DEK has 32 bytes from crypto/rand.
2. Encryption: AES-256-GCM. A new 96-bit random nonce for each encryption. Additional data: the text "vlt-secret-v1|", the Secret ID, the text "|" and the version.
3. Linux host binding:
    1. Read the DEK from the file vlt-dek in the directory in $CREDENTIALS_DIRECTORY.
    2. Read the key mode from the file /etc/vltd/key-mode (root-owned, mode 0644, no Secret). Values: host, tpm2, host+tpm2.
    3. If the key mode is "host", the Vault state shows "reduced protection" (SEC-005).
    4. vltd never starts systemd-creds.
4. Windows host binding:
    1. At the first start, vltd makes the DEK.
    2. vltd wraps the DEK with CryptProtectData. Do not use CRYPTPROTECT_LOCAL_MACHINE. Use CRYPTPROTECT_UI_FORBIDDEN. Pass NULL for the prompt structure.
    3. Entropy: the text "VoltioVolt-DEK-v1" and the install ID from the meta table.
    4. vltd writes the wrapped DEK to the file vault.dek in the data directory.
5. Canary:
    1. When the Vault is made, store a canary Secret with a known value.
    2. At each start, decrypt the canary.
    3. If this fails, set the Vault state "locked: host binding failed". Write an audit entry and a log entry. Do not write the canary value.
6. Vault states: available, reduced protection, locked, not configured.
7. Vault functions:

| Function | Result |
|----------|--------|
| Set(id, value) | Encrypts and stores the Secret. Increases the version. |
| Delete(id) | Removes the Secret. |
| Status(id) | Set or not set, hint, updated_at. Never the value. |
| Use(id, function) | Gives the bytes to the function. Clears the buffer after the function returns. |
| Reset(confirmation text) | Deletes all Secrets. On Windows, makes a new DEK. On Linux, keeps the DEK and makes a new canary. |

8. Add a test that reads the source code with go/ast. The test fails if a package outside internal/vault calls Use. The allow list is empty in this instruction.
9. Tests:
    1. Encrypt and decrypt.
    2. 10,000 encryptions give 10,000 different nonces.
    3. A ciphertext moved to a different Secret ID does not decrypt.
    4. A different DEK does not decrypt (simulation of SEC-006).
    5. Canary failure gives the state "locked".
    6. Windows CI: a DPAPI round trip in the CI user account.
    NOTE: The test with the virtual account NT SERVICE\vltd is an Owner test. Refer to TASK 03-00-00-008, step 6.

### TASK 03-00-00-005 Make the PIN authentication

Branch: vlt-is-003/auth. Directory: internal/auth.

1. PIN length: 6 to 64 characters. Permitted characters: digits and letters.
2. Reject these PINs: the same character only (for example "000000"), a simple sequence (for example "123456", "654321"), and a list of 20 common PINs in the code.
3. Argon2id parameters: time 3, memory 64 MiB, threads 4, salt 16 bytes, key 32 bytes. Source: RFC 9106, section 4, second recommended option.
4. Store the hash in the PHC string format, with the parameters.
5. Compare in constant time.
6. Lockout (SEC-024): after 5 incorrect entries, block for 5 minutes. Double the block time after each new failure. Maximum block time: 24 hours. Keep the counter in the Store. A restart does not reset it.
7. Session:
    1. A correct PIN gives an elevated state to the IPC connection only. Do not make a token.
    2. The elevated state ends after 10 minutes without a request (Setting), or after 8 hours, or when the connection closes.
8. First start: if no PIN is set, permit only "set PIN" and read-only status requests.
9. Write an audit entry for each PIN set, each correct entry, each incorrect entry and each block.

### TASK 03-00-00-006 Make the audit log

Branch: vlt-is-003/audit. Directory: internal/audit.

1. The audit table is append-only. There is no function to change or delete a row.
2. Columns: seq, ts (UTC), actor (gui, telegram or system), action, target, result, details (JSON, no Secret), prev_hash, hash.
3. hash = SHA-256 of prev_hash and the canonical JSON of the row without the hash.
4. At each start, vltd verifies the full chain. If the chain is broken, write a log entry and an audit entry with the action "audit_chain_broken". vltd continues.
5. Tests: a correct chain, a changed row, a deleted row.

### TASK 03-00-00-007 Make the IPC

Branch: vlt-is-003/ipc. Directories: internal/ipc (server) and internal/ipc/client.

1. Frame: a 4-byte big-endian length, then JSON. Maximum frame size: 1 MiB. The decoder rejects unknown fields.
2. Request: {"v":1,"id":number,"method":text,"params":object}. Response: {"v":1,"id":number,"result":object} or {"v":1,"id":number,"error":{"code":text,"message":text}}.
3. Windows endpoint:
    1. Named pipe \\.\pipe\vltd.
    2. Security descriptor: SYSTEM, Administrators, NT SERVICE\vltd, and the local group "VLT Operators" if it exists.
    3. Reject remote clients (PIPE_REJECT_REMOTE_CLIENTS).
    4. Library: github.com/Microsoft/go-winio (MIT) or own code with golang.org/x/sys/windows. Write your choice and the reason in Report 3.
4. Linux endpoint:
    1. Unix socket /run/vltd/vltd.sock, mode 0660, owner vltd, group vlt. Directory /run/vltd, mode 0750.
    2. Check the peer with SO_PEERCRED. Accept uid 0 and members of the group vlt only.
5. Limits: 8 connections or fewer. 20 requests each second or fewer for each connection. Close a connection with a partial frame after 30 seconds.
6. Methods, protocol version 1:

| Method | Elevated state needed | Result |
|--------|------------------------|--------|
| system.hello | No | vltd version, protocol version |
| system.status | No | Vault state, Store state, uptime |
| auth.status | No | PIN set or not, block end time, elevated state |
| auth.set_pin | No at first start; Yes after | Result only |
| auth.login | No | Result only |
| auth.logout | No | Result only |
| settings.list | No | Non-secret Settings |
| settings.set | Yes | New version |
| secret.status | No | Set or not set, hint, updated_at |
| secret.set | Yes | Result only |
| secret.delete | Yes | Result only |
| vault.reset | Yes, and the confirmation text "RESET VAULT" | Result only |
| audit.list | Yes | Audit rows, with paging |

7. No method returns a Secret value. Add a test that calls each method and checks that no response contains a test Secret value.
8. Fuzz target for the frame decoder.
9. The client package gives one Go function for each method. A later instruction uses it in the VLT GUI.

### TASK 03-00-00-008 Make the vltd service skeleton

Branch: vlt-is-003/vltd. Directories: cmd/vltd, internal/platform, packaging/linux, packaging/windows.

1. Start sequence:
    1. Logging.
    2. Data directory and permission check.
    3. Store.
    4. Vault. If the Vault fails, continue (SEC-090).
    5. Audit chain verification.
    6. IPC server.
    7. Ready signal: sd_notify READY=1 on Linux, service state "running" on Windows.
2. Stop sequence: the reverse order. Maximum 10 seconds.
3. Windows service: golang.org/x/sys/windows/svc. Linux: Type=notify with github.com/coreos/go-systemd/v22/daemon.
4. Command-line flag: --version only.
5. Development install files. These are not packages.

| File | Content |
|------|---------|
| packaging/linux/vltd.service | User=vltd, Group=vltd, Type=notify, LoadCredentialEncrypted=vlt-dek:/etc/credstore.encrypted/vlt-dek, StateDirectory=vltd, RuntimeDirectory=vltd, UMask=0077, NoNewPrivileges=yes, ProtectSystem=strict, ProtectHome=yes, PrivateTmp=yes, ProtectKernelTunables=yes, ProtectKernelModules=yes, ProtectControlGroups=yes, CapabilityBoundingSet= (empty), MemoryDenyWriteExecute=yes, LockPersonality=yes, SystemCallArchitectures=native, RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6. |
| packaging/linux/vlt-dev-init.sh | Runs as root. Makes the users and groups vltd and vlt. Makes 32 random bytes and encrypts them with "systemd-creds encrypt --name=vlt-dek --with-key=auto" to /etc/credstore.encrypted/vlt-dek. Writes /etc/vltd/key-mode. Never uses --with-key=null. Does not write the plain DEK to disk. |
| packaging/windows/dev-install.ps1 | Runs as Administrator. Makes the service vltd with the virtual account NT SERVICE\vltd. Makes the data directory with the ACL of TASK 03-00-00-003. Starts the service. |
| packaging/windows/dev-uninstall.ps1 | Stops and removes the service. Asks before it deletes the data directory. |

6. File docs/user/dev-service.md, in English and in Turkish. Owner test steps:
    1. Install on Linux and on Windows.
    2. Check the service status.
    3. Find the Vault state and the canary result in the log.
    4. SEC-006 test: copy the data directory to a second computer. Start vltd there. The Vault state MUST be "locked".
    5. Uninstall.
7. Integration tests:
    1. Start vltd in the test process with a temporary directory and a test DEK provider (SEC-091).
    2. Use the IPC client to test: first PIN, login, lockout, settings write and read, Secret write and status, Vault reset, audit list.
    3. Start without a DEK: vltd runs, the Vault state is "not configured" or "locked", and secret.set returns an error.

### TASK 03-00-00-009 Update the documentation

Put each change in the pull request of the related task.

1. docs/architecture.md: start sequence, Vault states, IPC methods.
2. docs/security.md: the changes in 01-30-00, the Vault design and the redaction rules.
3. docs/decisions.md: one row for each decision in 01-00-00.
4. docs/dependencies.md: each new module.

### TASK 03-00-00-010 Research (no code)

Find the facts. Give a source URL for each fact. Mark each fact without a source UNVERIFIED.

1. systemd: the unit behavior when a LoadCredentialEncrypted= credential cannot be decrypted. Propose a method that lets vltd start without the Vault (SEC-090).
2. Windows: the method to give SeShutdownPrivilege to NT SERVICE\vltd.
3. Windows: the method for an installer to make the local group "VLT Operators" and to use it in the pipe security descriptor.
4. Linux: the method for a non-root service to power off or hibernate the computer through logind and polkit.
5. Windows: confirm that user-scope DPAPI works for a virtual account. Give a Microsoft source if one exists.

---

## 04-00-00 ACCEPTANCE CRITERIA

- CI passes on Linux and Windows for each pull request.
- CGO_ENABLED=0 go build ./... passes.
- All redaction tests pass. The token of TASK 03-00-00-002, step 7.4 is not in any output.
- The Vault tests of TASK 03-00-00-004, step 9 pass. The Windows DPAPI round trip passes in CI.
- The go/ast test of TASK 03-00-00-004, step 8 passes.
- The IPC test of TASK 03-00-00-007, step 7 passes.
- The integration tests of TASK 03-00-00-008, step 7 pass.
- No test-only DEK provider is in a shipped binary (SEC-091). Give the check command in Report 3.
- gitleaks finds no Secret in the full history.
- The token check of SEC-088 shows "absent" at the end of each task.

---

## 05-00-00 REPORT 3 REQUEST

### 05-00-01 General

1. Write Report 3 after all tasks.
2. Stop after Report 3. Do not start work that is not in this instruction.

### 05-10-00 Report format

1. Title: VLT-RPT-003, Revision 00.
2. Format: Markdown. Simplified Technical English.
3. Sections, in this order:
    1. Summary (5 sentences or fewer)
    2. Tasks (table: task number, status DONE, PARTIAL or NOT DONE, pull request link, notes)
    3. Command log (command and output summary; no tokens; token check results of SEC-088)
    4. Tests (command and result summary)
    5. Repository settings (only changed settings)
    6. Dependencies added (table: name, version, license, reason)
    7. Research results (TASK 03-00-00-010)
    8. Open Dependabot pull requests (table: number, module, versions, CI result)
    9. Deviations (each difference from this instruction, with the reason)
    10. UNVERIFIED items
    11. Risks (new or changed only)
    12. Questions (for the Owner; for the Instruction Author)
    13. Sources
4. Do not add other sections.

END OF MESSAGE VLT-IS-003
