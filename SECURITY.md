# Security policy

## Supported versions

None. VLT is pre-alpha. The only download is the pre-release "Test build" (tag `test-latest`) for tests. It is not supported for production use.

## Report a vulnerability

Use GitHub private vulnerability reporting only:

1. Go to the **Security** tab of this repository.
2. Select **Report a vulnerability**.
3. Write the description, the steps to reproduce and the impact.

Do not report a vulnerability in a public issue, a pull request or a discussion. There is no e-mail address for reports.

## Scope

| Area | Examples |
|------|----------|
| Vault | Secret storage, DEK, host binding (DPAPI, systemd credentials), Secret leaks in logs |
| IPC | Local named pipe or Unix domain socket, access control, message decoding |
| Channels | Telegram, e-mail and other Notification paths, token handling, Remote code |
| UPS commands | Any path that can send a CONTROL or CONFIG command, or a command that is not in the command registry |

## Out of scope

- Physical access to the UPS or to the computer.
- Third-party services (for example Telegram or Gmail) themselves.
