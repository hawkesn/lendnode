---
template_version: 0.6.2
planner_model: claude-sonnet-5-5
builder_model: pending-run
target_repo: https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING
unit: wsl-install-pure-pieces
created: 2026-11-28
---

# Run the agent in a Linux distro that moonlighter manages on Windows: install step pure pieces Implementation Plan

**Target Repo:** https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING

## Decisions parked for tier-0

- None: the change adds one new Go file in the repo root. It touches no protected path (the root `CODEOWNERS` protects only `/.github/workflows/`), no credential or auth, no migration and no planner flag. The pinned sha256 value itself is not written here; the later install plan carries it.

## Seams

#### (a) Stage table

| Order | Piece | Content | What keeps main green |
|---|---|---|---|
| 1 | preflight check | `Preflight`, WSL present and memory cap at least 12 GiB (R14, R15) | merged; pure code with its own tests |
| 2 | Task 1 of this plan | `MergeWSLConfig` (R3) and `VerifyRootfs` (R2 sha256 check) | new pure code with its own tests; nothing calls it yet |
| 3 | install step | rootfs import, `wsl.conf`, firewall rule, scheduled tasks, port-reserve unit, post-join check, run on Windows | later plan; calls the pieces above and `Preflight` |
| 4 | stop and size | stop order, allocatable size through system-reserved | later plan |

#### (b) Shared contracts

- `Preflight(h Host) error` returns nil, or a `*PreflightError` with `Reason` and `Command`.
- `MergeWSLConfig(config string) string` returns the text with `networkingMode=mirrored` set in `[wsl2]`.
- `VerifyRootfs(r io.Reader, pin string) error` returns nil only when the sha256 of the bytes read equals the pin.

#### (c) Stitch record

| Piece | Plan file | PR | Merge sha | Deviation |
|---|---|---|---|---|
| preflight check | `specs/self-feed/wsl-preflight-check.md` | | | none |
| Task 1 | `specs/self-feed/wsl-install-pure-pieces.md` | | | none |

## Intent

#### Goal

Two pure helpers for the install step: one merges `networkingMode=mirrored` into the owner's `.wslconfig` text and keeps every other line, and one checks the rootfs sha256 against a pin before anything is imported. Later plans call them from the Windows install.

#### Operator intent

- I1: "Another thought is that it opens up a virtual machine on the host machine?"
- I2: "it's a simple application (windows or linux or macos), where you install it, and you give it a connection url and a token. It should just join."
- I3: "this program should be able to run on any machine/any device (ie. pi, mac, linux, windows)"

#### Choices made

- Epic service: the install must not disturb the owner's WSL settings and must refuse a wrong rootfs, so pasting one connection string joins the node without surprises (I2). Both helpers take plain data, so they build and test on every OS (I3).
- The change lives in `install.go` in the root package `moonlighter`; the card names no file. The repo root already holds `lend.go` and `preflight.go`, so a subpackage would be more than needed.
- Taken: this plan covers only the two pure helpers (card R2 sha256 check, R3). Alternative: plan the whole install step. Impact: the whole step cannot fit one task of at most 120 changed lines and needs Windows to prove; Seams rows 3 and 4 name the later pieces, so this plan says "relates" and the issue stays open.
- Taken: tests go in a new `install_test.go`, because the module has no `install` test file yet and Go needs `_test.go` beside the code. Alternative: add to `preflight_test.go`. Impact: that edits an existing file the preflight plan owns.
- Taken: when no `[wsl2]` section exists, one is appended; when `networkingMode` exists in `[wsl2]` it is replaced in place in any letter case; otherwise the key is inserted directly under the `[wsl2]` header. Alternative: rewrite the file. Impact: the card says never overwrite and keep every other key.
- Taken: a CRLF file stays CRLF, the way Windows editors write it. Alternative: always write LF. Impact: LF-only lines inside a CRLF file would be a mixed file.
- Taken: an empty pin always fails, and the pin compares in any letter case. Alternative: skip the check on an empty pin. Impact: a missing pin must not turn into a silent pass.
- Taken: review class `prod-code`; `risk_class` is `ordinary` because no workflow, manifest or auth changes.
- Taken: there is no `package.json`, so `npm ci` does not apply; the proof ran with the Go toolchain. The repo has no `test/integration` directory and no Postgres is needed.

#### Non-goals

- No rootfs download or import, no write of `.wslconfig` to disk, no call of `wsl.exe`.
- No change to `preflight.go`, `lend.go`, `go.mod` or `.github/`.
- No change to the hand installs.

## Constraints

#### Requirements

- R1: `MergeWSLConfig` sets `networkingMode=mirrored` in `[wsl2]` and leaves every other line byte for byte as it was (card R3).
- R2: `MergeWSLConfig` is idempotent: merging its own output returns the same text.
- R3: `VerifyRootfs` returns nil only when the sha256 of the bytes equals the pin, and an error otherwise, including an empty pin (card R2).

## Validation

#### Acceptance criteria

- Given a `.wslconfig` with memory, processors and swap keys, when the merge runs, then only `networkingMode=mirrored` is added and the other lines are unchanged.
- Given `networkingMode=NAT` in `[wsl2]`, when the merge runs, then that line becomes `networkingMode=mirrored` and no second key appears.
- Given a rootfs whose sha256 differs from the pin, when `VerifyRootfs` runs, then it returns an error.

#### Edge cases

- Empty file: the result is a `[wsl2]` section with the one key.
- A `networkingMode` line under `[experimental]` is not the mode; a `;` comment line is kept as it is.
- A CRLF file keeps CRLF line ends.
- A pin in upper case matches.

#### Tests

The tests live in the new `install_test.go`. The oracle is `go test doc.go install.go install_test.go`. `VerifyRootfs` is a guard, and six wrong patches under `test/fixtures` stand in for a mutation probe: each breaks one rule (case-sensitive pin, overwrite, always insert, any section, no check, lost CR).

- A1 [R1, T1] merge-table: empty file, owner keys kept, other mode replaced, missing section appended, other section ignored, case and comment, CRLF.
- A2 [R2, T2] merge-idempotent: merging the output again changes nothing, checked on every case of the table.
- A3 [R3, T3] verify-rootfs: matching pin passes, upper-case pin passes, changed bytes fail, empty pin fails.

## Risk

#### Failure modes

- If a real `.wslconfig` uses a form this merge misreads, the file could gain a second key; WSL then reads the last one. The post-join cross-node check (card R19) in a later plan still proves the node works.

## Traps / gotchas

- T1: Contract: the key is written exactly `networkingMode=mirrored`, with no spaces. Failing input: `[wsl2]\nmemory=24GB\n` gives `[wsl2]\nnetworkingMode=mirrored\nmemory=24GB\n`. A `networkingMode` under `[experimental]` must not be replaced.
- T2: Trap: replacing the line when it exists and inserting when it does not must be exclusive, or the key appears twice. Failing input: `[wsl2]\nnetworkingMode=NAT\n`.
- T3: Contract: the pin compare ignores letter case and an empty pin fails. `VerifyRootfs` reads the whole reader, and a read error is returned. Failing input: the right bytes with an upper-case pin pass; `""` fails.

## Hazards

Tables declared: none. Every table is `not-touched`: the change is one Go file with no shell, regex, SQL or doc table.

## External calls

- none: both helpers read only the values handed to them and call nothing outside the repo.

## Tasks

### Task 1: Add MergeWSLConfig and VerifyRootfs

**Belief:** orion-mk4-gaming-pc-specs

**Intent:** I1, I2, I3

**Goal.** Add `install.go` in package `moonlighter` with `MergeWSLConfig(config string) string` and `VerifyRootfs(r io.Reader, pin string) error`.

**Leave alone.**
- Change no file except `install.go`.
- Leave `install_test.go` as it is. It holds the tests you must make pass.
- Do not touch `go.mod`, `preflight.go`, `lend.go` or `.github/`.

**Facts measured at the start commit.**
- `ls *.go` shows `doc.go`, `lend.go`, `preflight.go` and their tests; `go.mod` says `go 1.26.10`.
- `grep -n "func " preflight.go` shows no merge or hash helper, so nothing is reused.
- There is no `test/integration` directory.

**The failing test.** `go test doc.go install.go install_test.go` fails: `install.go` does not exist, and `MergeWSLConfig` and `VerifyRootfs` are undefined.

**The change.** `MergeWSLConfig`: split the text on `\n`; track whether the current section is `[wsl2]` (any letter case); in `[wsl2]`, a line whose key (trimmed, any case) is `networkingMode` is replaced by `networkingMode=mirrored` plus its own trailing `\r`, if any. If none was replaced and a `[wsl2]` header exists, insert `networkingMode=mirrored` directly under the first header. If there is no header, append a newline if the text lacks one, then `[wsl2]` and the key, each ending in `\n`. When the text holds `\r\n`, the added lines end in `\r\n`. `VerifyRootfs`: hash the reader with sha256, return the error from the read if any, and return an error unless the pin is non-empty and equals the hex digest ignoring letter case.

**Done when.** `go test doc.go install.go install_test.go` exits 0, and `gofmt -l .` and `go vet ./...` print nothing.

**Task Contract:**

- item: wsl-install-pure-pieces
- relates: hawkesn/embersuite#1955
- rung: 1
- risk_class: ordinary
- repo: https://github.com/hawkesn/moonlighter.git
- base_sha: START_SHA_PENDING
- branch: autoloop/wsl-install-pure-pieces
- task_class: build-to-green-against-existing-tests
- oracle_cmd: `go test doc.go install.go install_test.go`
- test_cmd: `go test doc.go preflight.go preflight_test.go preflight_units_test.go`
- max_added: 70
- max_deleted: 0
- test_files:
  - "install_test.go"
- allowed_files:
  - "install.go"
- release: batch

## RED start files

#### test edit

- bytes: sha256:894f21cdfc7bf2936fcda9bd9461ec193d5860994a4579e8cd7809d355f4d055

#### reference patch

- bytes: sha256:e107df5b85c475bb185488edf5be571bb59e4a8802ac77102132e459051ea203

## Surviving mutants: the tests must kill these

- install.go:22: false -> true | in, header, replaced := false, -1, false
- install.go:27: < -> <= | if in && header < 0 {

Kill each swap where the line is behaviour with a test in a new test file named like the repo's tests (`<pkg>/<name>_test.go` beside the code); you may add new test files but must not edit existing ones. For each swap you do not test, write one commit-message line `mutant-skip: <path>:<line> <comment|doc|message|type|equivalent>`. Use `equivalent` only for a swap that cannot change behaviour or a line no test can reach (an error path of an os/exec pipe); a plain behaviour line a test can reach, such as a failing writer or a bad input, is killed with a test, never skipped as `equivalent`.
