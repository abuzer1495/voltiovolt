# Contributing to VoltioVolt (VLT)

VLT is in pre-alpha. The Owner approves all decisions. Read these rules before you open a pull request.

## 1. Clean-room rule (SEC-060)

1. Do not copy code, comments or text from other UPS software (for example Network UPS Tools, apcupsd or vendor software).
2. Do not reverse engineer vendor software.
3. You can use protocol facts from public documents and public captures. Write each fact with its source in `docs/protocol.md`. Write each source in `docs/references.md`.
4. Mark each fact without a confirmed source as UNVERIFIED.

## 2. Dependency rule (SEC-061)

1. A Go module in a VLT binary must have one of these licenses: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, MPL-2.0.
2. Each new dependency needs a row in `docs/dependencies.md`: name, version, license, reason.
3. CI runs `go-licenses check` with this allow list. CI fails on all other licenses.
4. GPL-3.0 programs are permitted only as separate programs that the user installs, after Owner approval (OD-02).

## 3. Branch and pull request rules

1. One task, one branch, one pull request.
2. Branch name: `vlt-is-<nnn>/<task-name>`. Example: `vlt-is-002/dialect`.
3. The pull request title starts with the instruction number and the task number.
4. A pull request contains only the work of its task.
5. Merge method: squash merge only.
6. Do not push directly to `main`. Do not force push. Do not rewrite history.
7. Do not commit binaries, Capture files (`*.vltcap.json`) or reports (`VLT-RPT-*`).
8. All required CI checks must pass before a merge.

## 4. DCO sign-off

Each commit has a Developer Certificate of Origin sign-off (<https://developercertificate.org/>). Use:

```
git commit -s
```

The sign-off line is `Signed-off-by: Name <e-mail>`. It must agree with the commit author.

## 5. Tests and fuzz tests

1. Each parser has table tests with the test vectors and one fuzz target.
2. A parser never panics. It returns an error with the field index.
3. Use only the simulator (`internal/sim`, `cmd/vlt-sim`) for tests. Do not run tests against a real UPS.
4. Test data in `testdata/` contains only fake values. Do not add real serial numbers, tokens or personal data.
5. Run these commands before a pull request:

```
gofmt -l .
go vet ./...
CGO_ENABLED=0 go build ./...
go test -race ./...
```

6. Fuzz smoke test (each target for 30 seconds), for example:

```
go test -run='^$' -fuzz='^FuzzParseQ1$' -fuzztime=30s ./internal/dialect
```

## 6. Source files

1. Each source file starts with `SPDX-License-Identifier: AGPL-3.0-or-later` (OD-01).
2. Write all repository text and code comments in English.
3. Use the terms of the VLT definitions (refer to `docs/instructions/`).
