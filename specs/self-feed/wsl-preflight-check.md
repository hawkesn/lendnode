---
template_version: 0.6.2
planner_model: claude-sonnet-5-5
builder_model: pending-run
target_repo: https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING
unit: wsl-preflight-check
created: 2026-11-28
---

# Run the agent in a Linux distro that moonlighter manages on Windows: preflight check Implementation Plan

**Target Repo:** https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING

## Decisions parked for tier-0

- None: the change adds two new files in the repo root. It touches no protected path (CODEOWNERS protects only `/.github/workflows/`), no credential or auth, no migration and no planner flag.

## Seams

#### (a) Stage table

| Order | Piece | Content | What keeps main green |
|---|---|---|---|
| 1 | Task 1 | preflight check: WSL present, WSL memory cap at least 12 GiB (R14, R15 of the card) | new pure code with its own tests; nothing calls it yet |
| 2 | install step | rootfs sha256 check and import, `.wslconfig` merge, `wsl.conf`, firewall rule, tasks, port-reserve unit, post-join check | later plan; calls `Preflight` first |
| 3 | stop and size | stop order, allowable size through system-reserved | later plan |

#### (b) Shared contracts

- `Preflight(h Host) error` returns nil, or a `*PreflightError` with `Reason` and `Command`.
- `Host` holds `WSLInstalled bool`, `HostRAMBytes int64` and `WSLConfig string`; the caller gathers them, so the check runs on Linux in tests.
- `WSLInstallCommand` is exactly `wsl --install --no-distribution`.

#### (c) Stitch record

| Piece | Plan file | PR | Merge sha | Deviation |
|---|---|---|---|---|
| Task 1 | `specs/2026-11-28-wsl-preflight-check.md` | | | none |

## Intent

#### Goal

On a Windows machine, a pure preflight check says whether the machine can host the moonlighter-managed WSL2 distro. It fails when WSL is absent (naming the one install command) and when the WSL memory cap is below about 12 GiB. This is the first piece of the card; the install step, the stop path and the size setting follow in later plans.

#### Operator intent

- I1: "Another thought is that it opens up a virtual machine on the host machine?"
- I2: "it's a simple application (windows or linux or macos), where you install it, and you give it a connection url and a token. It should just join."
- I3: "this program should be able to run on any machine/any device (ie. pi, mac, linux, windows)"

#### Choices made

- Epic service: the check refuses early and says the one command to run, so a Windows owner who pastes the connection string either joins or is told exactly what is missing (I2). The input is plain data, so the code builds and tests on every OS (I3).
- Taken: this plan covers only the preflight check (the card's R2 first piece, R14, R15). Alternative: plan the whole card at once. Impact: the whole card cannot fit one task of at most 120 changed lines; the later pieces are named in Seams stage rows 2 and 3, so this plan says "relates" and leaves the issue open.
- Taken: the code lives in `preflight.go` in the root package `moonlighter`, tests in `preflight_test.go`. Alternative: a subpackage. Impact: the repo has only `doc.go` in the root package, so the root is the smallest change. The `test/<module>-<topic>.test.ts` naming is a TypeScript rule; Go requires `_test.go` beside the code.
- Taken: `Preflight` takes a `Host` value and does not read the machine. Alternative: call `wsl.exe` and read `%USERPROFILE%\.wslconfig` inside the check. Impact: those reads are Windows only and the card's R12 says CI tests run on Linux only; the install plan gathers the Host on Windows.
- Taken: WSL's `GB` means GiB (1024 based), and an absent or unparseable `memory=` key means half of host RAM, as the card says for the default. Alternative: refuse on an unparseable value. Impact: WSL itself ignores an invalid value, so the cap it really applies is the default.
- Taken: review class is `prod-code`; the task `risk_class` is `ordinary` because no workflow, manifest or auth changes.
- Taken: `preflight.go` and `preflight_test.go` are the only new files; `go.mod`, `doc.go` and `.github/workflows/ci.yml` stay as they are.
- Taken: there is no `package.json` in the repo, so no `npm ci` applies; the proof ran with the Go toolchain.

#### Non-goals

- No install step, no rootfs import, no `.wslconfig` write (this check only reads text it is given).
- No Windows-only code, no call of `wsl.exe`, no change to CI.
- No change to the hand installs.

## Constraints

#### Requirements

- R1: `Preflight` fails when `Host.WSLInstalled` is false, with a `*PreflightError` whose `Command` is `wsl --install --no-distribution` (card R14); it does not look at memory in that case.
- R2: `Preflight` fails when the WSL memory cap is below 12 GiB (card R15, T3 of the card).
- R3: `WSLMemoryCap` reads only `memory=` in the `[wsl2]` section of the given text, and returns half of host RAM when the key is absent or unparseable.

## Validation

#### Acceptance criteria

- Given a machine without WSL, when `Preflight` runs, then it returns a `*PreflightError` with `Command` equal to `wsl --install --no-distribution`.
- Given a WSL memory cap below 12 GiB, when `Preflight` runs, then it returns an error; at 12 GiB it returns nil.
- Given no `memory=` key, when `Preflight` runs, then the cap is half of host RAM.

#### Edge cases

- `memory=` in another section such as `[experimental]` is ignored.
- Key and section names match in any letter case, spaces around `=` are allowed and a `;` comment line is skipped.
- `memory=lots` is unparseable: the default applies.

#### Tests

The tests live in the new `preflight_test.go`. The oracle is `go test doc.go preflight.go preflight_test.go`. No guard or gate changes, so no mutation probe; six wrong patches under `test/fixtures` stand in for one. The sandbox has no Postgres or integration directory, and none is needed.

- A1 [R1, T1] wsl-absent-names-install-command: refuses with the exact command.
- A2 [R2, R3, T2] memory-cap-table: explicit 11GB and 8GB refuse, 12GB and 12288MB pass, the default half of 16 GiB refuses, half of 32 GiB passes.
- A3 [R3, T3] section-and-key-parsing: another section is ignored, case, spaces and comments are handled, an unparseable value falls back.
- A4 [R1, R2] pass-case: WSL present and enough memory returns nil.

## Risk

#### Failure modes

- If the real `.wslconfig` uses a form this parser misreads, the check passes or refuses wrongly; A3 covers the forms the WSL documentation shows. The install plan's post-join check (card R19) still proves the node works.

## Traps / gotchas

- T1: Contract: a missing WSL is checked first and returns `Command` set; a memory refusal has an empty `Command`. Failing input: `Host{WSLInstalled: false, HostRAMBytes: 64<<30}` must refuse.
- T2: Contract: 12 GiB passes exactly (`<` not `<=`). WSL `GB` is 1<<30, not 1e9. Failing input: `memory=12GB` must pass.
- T3: Trap: a `memory=` line under `[experimental]` must not count. Failing input: `[experimental]\nmemory=2GB\n[wsl2]\nprocessors=4\n` with a 32 GiB host passes.

## Hazards

Tables declared: none. Every table is `not-touched`: the change is two Go files with no shell, regex, SQL or doc table.

## External calls

- none: the check reads only the values handed to it and calls nothing outside the repo.

## Tasks

### Task 1: Add the WSL preflight check

**Belief:** orion-mk4-gaming-pc-specs

**Intent:** I1, I2, I3

**Goal.** Add `preflight.go` with `Host`, `PreflightError`, `Preflight`, `WSLMemoryCap`, `WSLInstallCommand` and `MinWSLMemoryBytes`, in package `moonlighter`.

**Leave alone.**
- Change no file except `preflight.go`.
- Leave `preflight_test.go` as it is. It holds the tests you must make pass.
- Do not touch `go.mod`, `doc.go` or `.github/`.

**Facts measured at the start commit.**
- `doc.go` is the only Go file, package `moonlighter`; `go.mod` says `go 1.26.10`.
- There is no `test` directory yet; `preflight_test.go` is the only test file.
- `.github/workflows/ci.yml` runs `go build`, `go vet` and `go test ./...`, so the file must pass `gofmt` and `go vet` on every OS target.

**The failing test.** `go test doc.go preflight.go preflight_test.go` fails to compile at the start commit: `Preflight`, `Host` and `PreflightError` are undefined.

**The change.** `Preflight(h Host) error`: if `!h.WSLInstalled`, return `&PreflightError{Reason, Command: WSLInstallCommand}`. Else compute `WSLMemoryCap(h.WSLConfig, h.HostRAMBytes)`; if below `MinWSLMemoryBytes` (12<<30) return a `*PreflightError` with no `Command`. Else nil. `WSLMemoryCap` walks the lines of the text, tracks whether the current section is `[wsl2]` (any letter case), and for a `memory` key (any case, trimmed) parses an integer with unit TB, GB, MB, KB or B (1024 based, upper-cased); a bad value means the default, half of host RAM. `PreflightError.Error()` includes the command when set.

**Done when.** `go test doc.go preflight.go preflight_test.go` exits 0.

**Task Contract:**

- item: wsl-preflight-check
- relates: hawkesn/embersuite#1955
- rung: 1
- risk_class: ordinary
- repo: https://github.com/hawkesn/moonlighter.git
- base_sha: START_SHA_PENDING
- branch: autoloop/wsl-preflight-check
- task_class: build-to-green-against-existing-tests
- oracle_cmd: `go test doc.go preflight.go preflight_test.go`
- test_cmd: `go build ./...`
- max_added: 100
- max_deleted: 0
- test_files:
  - "preflight_test.go"
- allowed_files:
  - "preflight.go"
- release: batch

## RED start files

#### test edit

- bytes: sha256:959fd424eae440da6002bbc612d1a495c8791beb961dd59f346476a8e6fb9922

#### reference patch

- bytes: sha256:aa0b1100bafab4fe7681166ca5627b168d92c4bef7b20af8634bac8c27e00d9f

## Surviving mutants: the tests must kill these

- preflight.go:49: false -> true | def, in := hostRAM/2, false
- preflight.go:64: 1 -> 2 | }{{"TB", 1 << 40}, {"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1}} {

Kill each swap where the line is behaviour with a test in a new test file named like the repo's tests (`<pkg>/<name>_test.go` beside the code); you may add new test files but must not edit existing ones. For each swap you do not test, write one commit-message line `mutant-skip: <path>:<line> <comment|doc|message|type|equivalent>`. Use `equivalent` only for a swap that cannot change behaviour or a line no test can reach (an error path of an os/exec pipe); a plain behaviour line a test can reach, such as a failing writer or a bad input, is killed with a test, never skipped as `equivalent`.
