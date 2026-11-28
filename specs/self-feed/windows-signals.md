---
template_version: 0.6.2
planner_model: claude-sonnet-5-5
builder_model: pending-run
target_repo: https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING
unit: windows-signals
created: 2026-11-28
---

# windows-signals: read the owner's Windows signals and add the check subcommand Implementation Plan

**Target Repo:** https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING

## Decisions parked for tier-0

- None: no choice here touches a protected path (CODEOWNERS at the repo root protects only `/.github/workflows/`, which no task edits), a credential or auth, a migration or a planner flag. The kubeconfig default of R4 is a later stage and this plan reads no credential.

## Seams

#### (a) Stage table

| Order | Piece | Content | What keeps main green |
|---|---|---|---|
| 1 | Lend decision and node actions | `Decide`, `Settings` in `lend.go`; `Agent`, `Cluster` in `reclaim.go` | already at the start commit, with tests |
| 2 | Task 1 (this plan) | the signals reader behind `ReadSignals`, tick-wrap fix, the game signal, free RAM, the `check` subcommand | new files only; a Windows file and a stub for every other OS; the cross-compile job |
| 3 | Cluster and WSL adapters, loop | a `Cluster` over `wsl.exe --exec kubectl` with the kubeconfig path (R4), the CPU priority call, the pod's own grace on eviction (R3), the no-terminal run command (R6) | needs stages 1 and 2; tested with fakes on Linux |

#### (b) Shared contracts

- `ReadSignals() Signals` is the one function that reads signals (R1). Windows defines it in `signals_windows.go`; every other OS in `signals_other.go`, which returns `Signals{Err: ...}`.
- `Signals.OwnerInput` carries the game signal; `Signals.Idle` carries the idle time; `Signals.FreeRAM` is the host's free RAM in bytes; `Signals.Err` is any failed read. `Decide` in `lend.go` already refuses to lend on `Err`, on `OwnerInput`, or on idle below the threshold.
- `Cluster.Evict(ctx, Pod)` has no grace argument; stage 3 must send no grace override.

#### (c) Stitch record

| Piece | Plan file | PR | Merge sha | Deviation |
|---|---|---|---|---|
| Stage 1 | `specs/self-feed/lend-decision.md`, `specs/self-feed/reclaim-node-actions.md` | | | none |
| Task 1 | `specs/self-feed/windows-signals.md` | | | none |

## Intent

#### Goal

On Windows, moonlighter reads the owner's idle time, one game signal and the host's free RAM, and a `check` subcommand prints each signal. On every other OS the same function fails closed, so nothing is lent. The cluster adapter and the run loop are the next stage.

#### Operator intent

- I1: "i'd like to be able to toggle this behaviour from the worker node. A cut/kill could break a job, I'm open to letting it finish up if I only need to wait a little bit."

#### Choices made

- Epic fit (hawkesn/embersuite#1948): this is the part that notices the owner is back so the machine can be handed back before a job is broken (I4 of the epic); the Windows file uses only the standard library `syscall`, so the repo still builds for pi, mac, linux and windows (I1).
- Start commit read: `lend.go` (`Decide`, `Signals`, `Settings`), `reclaim.go` (`Agent`, `Cluster`) and their tests already exist, so card rows for decision, cordon-first, finish, slow, now, foreign cordon and restart are already built and tested (`TestNodeGameSignalCordonsFirst`, `TestNodeFinishWaitsForMaxWait` and others in `reclaim_test.go`). Only signals, the `check` subcommand, the WSL adapter and the loop remain; this plan takes the signals and `check`.
- Taken: this plan is stage 2 of 3 (see Seams), so the last contract says relates. Alternative: one plan for the whole card. Impact: the whole card cannot fit the 120-line limit of a task and the adapter needs a real cluster to prove.
- Taken (R7 game signal): full-screen state through `SHQueryUserNotificationState`, states 2, 3 and 4. Alternative: a process list. Impact: no list of game names to keep; a full-screen video also counts as the owner being present, which errs towards handing the machine back.
- Taken: the game signal goes in `Signals.OwnerInput`, so `lend.go` stays unchanged and `Decide` already refuses to lend on it.
- Taken (T4): idle time is `uint32` tick count minus `uint32` last-input tick, so the wrap is handled by unsigned arithmetic, in a pure function `IdleSince` that Linux tests reach.
- Taken (R9): any failed Windows call returns `Signals{Err: ...}` and `Decide` never lends on an error; a call with no interactive session fails the same way.
- Taken: the new tests are in a new `signals_test.go` beside the code; the repo's Go tests sit beside the package, so the `test/<module>-<topic>.test.ts` name does not apply, and there is no existing test file for a signals module. `lend_test.go` is not edited.
- Taken: `check` is a new `cmd/moonlighter/main.go` that prints `FormatSignals` and exits 1 on an error, 2 on any other argument. Alternative: the run command in the same file. Impact: the run loop needs the stage-3 adapter, so it waits.
- Taken: no `go.mod` change; `golang.org/x/sys` is not added.
- Review class (not a contract field): prod-code.
- Risk class `ordinary`: no workflow, manifest, `go.mod`, lockfile or credential changes.

#### Non-goals

- The `Cluster` adapter over `wsl.exe --exec`, the kubeconfig path (R4), the CPU priority call, the eviction grace and the run command (stage 3).
- The logon task, the distro and the k3s agent (hawkesn/embersuite#1955); cluster permissions (hawkesn/embersuite#1952).
- Linux or macOS signals; editing taints; deleting the Node.
- `lend.go`, `reclaim.go`, every existing test, `go.mod` and `.github/workflows/ci.yml` do not change.

## Constraints

#### Requirements

- R1: Signals sit behind one function, `ReadSignals`, so another operating system adds a signals file.
- R2: The signals are idle time from `GetLastInputInfo`, one game signal (full-screen state) and the host's free RAM.
- R3: The tick-count wrap is handled in the idle time.
- R4: A failed read, and any OS but Windows, returns an error in `Signals.Err`, so nothing is lent.
- R5: `moonlighter check` prints each signal.
- R6: Commits use the author address 1168175+hawkesn@users.noreply.github.com; past commits are not rewritten.
- R7: The code builds and vets for windows, darwin and linux on amd64 and arm64; tests run on Linux.

## Validation

#### Acceptance criteria

- Given the tick count has wrapped since the last input, when idle is computed, then it is the small true gap, not a huge or negative one.
- Given a full-screen game and an hour of idle, when `Decide` runs, then it does not lend.
- Given an OS that is not Windows, when signals are read, then `Err` is set and `Decide` does not lend.
- Given `moonlighter check`, when it runs, then it prints idle, game and free RAM, and the error line when a read failed.

#### Edge cases

- A game played only with a controller: idle grows, so the game signal alone must stop the lend (A5).
- No user logged on or session 0: `GetLastInputInfo` returns 0 and the read returns an error.
- The tick count wraps between the last input and now.

#### Tests

All tests are in the new `signals_test.go` and fail at the RED start because `IdleSince`, `FullScreenGame`, `FormatSignals` and `ReadSignals` do not exist. Windows rows are proven only on orion-mk4 (hawkesn/embersuite#1953, hawkesn/embersuite#1954); CI tests run on Linux. The mutation probe is the seven wrong patches `test/fixtures/signals_test-wrong-1.patch` to `-7.patch` (no wrap handling, game state 3 only, game state 2 and above, free RAM in bytes, a stub with no error, the error line never printed, idle in seconds); each fails the oracle.

- A1 [R3, T1] TestIdleSinceHandlesTickWrap: plain gap, equal ticks, and tick 100 after last input 0xFFFFFF00 gives 356 ms.
- A2 [R2, T2] TestFullScreenGameStates: states 2, 3 and 4 are a game; 0, 1, 5, 6 and 7 are not.
- A3 [R5, T3] TestFormatSignalsPrintsEachSignal: idle, game and free RAM in MiB lines; an error line only when `Err` is set.
- A4 [R1, R4, T4] TestReadSignalsFailsClosedOffWindows: off Windows `ReadSignals` sets `Err` and `Decide` does not lend.
- A5 [R2, T2] TestControllerGameDoesNotLendOnIdleAlone: an hour idle with the game signal does not lend; the same without it lends.
- A6 [R6, R7, T5, T6] build-vet-and-cross-compile: not a unit test; the proof runs `go vet ./...` for the six targets; R6 is the commit author.

## Risk

#### Failure modes

- If idle ignored the wrap, a machine used right after a 49.7-day wrap would look idle for weeks and be lent under the owner. A1 and wrong patch 1 guard it.
- If the game signal were dropped, a controller game would lend the machine. A2, A5 and wrong patches 2 and 3 guard it.
- If the non-Windows stub returned no error, a Linux build would see zero idle and zero game, and an idle threshold of zero would lend. A4 and wrong patch 5 guard it.
- The Windows reader cannot run on CI; a wrong struct size or DLL name shows only on orion-mk4 as an error line in `moonlighter.exe check`.

## Traps / gotchas

- T1: Contract: `IdleSince(tick, last uint32) time.Duration` is `time.Duration(tick-last) * time.Millisecond` with the subtraction in `uint32`. Converting to `int64` or `time.Duration` before subtracting breaks the wrap. Failing input: tick 100, last 0xFFFFFF00 must give 356 ms.
- T2: Contract: `FullScreenGame(state uint32) bool` is true for 2, 3 and 4 only (`SHQueryUserNotificationState` values for busy, Direct3D full screen and presentation mode). State 5, 6 and 7 (accepts notifications, quiet time, app) are false. Failing input: state 5 must be false, state 4 must be true.
- T3: Contract: `FormatSignals` writes `idle: <Duration>`, `game: <bool>`, `free_ram: <n> MiB` (bytes shifted right by 20), each ending in a newline, then `error: <text>` only when `Err` is set. Failing input: 3 GiB must print `free_ram: 3072 MiB`.
- T4: Trap: `signals_other.go` needs the line `//go:build !windows` first, then a blank line; `signals_windows.go` is selected by its name and needs no tag. Both define `ReadSignals() Signals`; defining it in `signals.go` too would not compile. The Windows file calls `GetLastInputInfo` (user32), `GetTickCount` and `GlobalMemoryStatusEx` (kernel32) and `SHQueryUserNotificationState` (shell32) through `syscall.NewLazyDLL`; `LASTINPUTINFO` is two `uint32` (size 8, `cbSize` = 8) and `MEMORYSTATUSEX` has `dwLength` = 64. A call that returns 0 (or a nonzero HRESULT for the shell call) returns `Signals{Err: ...}`.
- T5: Trap: the oracle names files, so `go test signals_test.go signals.go signals_other.go lend.go` builds only the Linux files; `go vet ./...` with `GOOS=windows` is what compiles `signals_windows.go` and `cmd/moonlighter/main.go`. Run it for all six targets before you finish.
- T6: Tests need no Postgres or Docker; the repo has no `test/integration` and no `package.json`, so `npm ci` does not apply. Go 1.26.10 is installed and was used. Keep `go.mod` unchanged and use only the standard library. The reference adds 97 lines in four files; add no helpers beyond it.

## Hazards

Tables declared: none apply. shell tool list, regex or parser list, doc table list, concurrent DB writer list, runs-where list and port: `not-touched` (the signals reader is not bound to a run loop yet; `check` runs by hand on orion-mk4).

## External calls

- observe-first: `GetLastInputInfo` in user32.dll (Microsoft Learn, winuser.h); request is a `LASTINPUTINFO` with `cbSize` 8, success is a nonzero return with `dwTime` set, error is 0; run `moonlighter.exe check` in the owner's session on orion-mk4 and compare idle with the real last input.
- observe-first: `GetTickCount` and `GlobalMemoryStatusEx` in kernel32.dll; the second takes a `MEMORYSTATUSEX` with `dwLength` 64, success is nonzero with `ullAvailPhys` set; run `moonlighter.exe check` on orion-mk4 and compare `free_ram` with Task Manager.
- observe-first: `SHQueryUserNotificationState` in shell32.dll; takes a pointer to a state value, success is `S_OK` (0) with state 1 to 7; run `moonlighter.exe check` on orion-mk4 once with a full-screen game and once without, and read `game:`.

## Tasks

### Task 1: Add the Windows signals reader and the check subcommand

**Belief:** a-lent-node-capacity-premise-sums-whole-pod-requests-and-every-kind-that-can-land-there

**Intent:** I1

**Goal.** Create `signals.go`, `signals_other.go`, `signals_windows.go` (package `moonlighter`) and `cmd/moonlighter/main.go`, so that `signals_test.go` passes and `moonlighter check` prints each signal.

**Leave alone.**
- Change no file except the four new files above.
- Do not edit `signals_test.go`; it holds the tests you must make pass. If the mutant attack lists surviving mutants, add new test files to kill them and do not edit existing ones.
- Do not touch `lend.go`, `reclaim.go`, `go.mod`, the workflow or any existing test.
- Standard library only.

**Facts measured at the start commit.**
- `grep -n "type Signals" lend.go` finds `Signals{Idle time.Duration; OwnerInput bool; FreeRAM uint64; Err error}`; `Decide` in `lend.go` returns false when `sig.Err != nil` or `sig.OwnerInput`.
- `ls *.go` shows no `signals*.go`; `go.mod` has no requires; there is no `cmd` directory.
- Existing wrong patches are `lend_test-wrong-1..8`, `reclaim_test-wrong-1..8`, `preflight_test-wrong-1..6` and `install_test-wrong-1..6`; this plan's are `signals_test-wrong-1..7`.

**The failing test.** `signals_test.go` does not compile at the start commit: `IdleSince`, `FullScreenGame`, `FormatSignals` and `ReadSignals` are undefined.

**The change.** Write `signals.go` with `IdleSince`, `FullScreenGame` and `FormatSignals` as T1 to T3 describe. Write `signals_other.go` and `signals_windows.go` as T4 describes: `ReadSignals` reads the last-input info, the tick count, the notification state and the memory status, and returns `Signals{Idle: IdleSince(tick, last), OwnerInput: FullScreenGame(state), FreeRAM: ullAvailPhys}`. Write `cmd/moonlighter/main.go`: with the single argument `check` print `FormatSignals(ReadSignals())` and exit 1 when `Err` is set; with any other arguments print `usage: moonlighter check` to stderr and exit 2.

**Done when.** `go test signals_test.go signals.go signals_other.go lend.go` exits 0 and `GOOS=windows go vet ./...` exits 0.

**Task Contract:**

- item: windows-signals
- relates: hawkesn/embersuite#1951
- rung: 1
- risk_class: ordinary
- repo: https://github.com/hawkesn/moonlighter.git
- base_sha: START_SHA_PENDING
- branch: autoloop/windows-signals
- task_class: build-to-green-against-existing-tests
- oracle_cmd: `go test signals_test.go signals.go signals_other.go lend.go`
- test_cmd: `go test lend_test.go lend.go`
- max_added: 110
- max_deleted: 0
- test_files:
  - "signals_test.go"
- allowed_files:
  - "signals.go"
  - "signals_other.go"
  - "signals_windows.go"
  - "cmd/moonlighter/main.go"
- release: batch

## RED start files

#### test edit

- bytes: sha256:a76acc26cf0209a68c74f134b163cb6919534931104c2a9cf998078d4da7ea1b

#### reference patch

- bytes: sha256:76282555f5ab1e030a55c898f538f31cb8ba4645a00da1d12463afb06b829064

## Surviving mutants: the tests must kill these

- cmd/moonlighter/main.go:12: || -> && | if len(os.Args) != 2 || os.Args[1] != "check" {
- cmd/moonlighter/main.go:14: 2 -> 3 | os.Exit(2)
- cmd/moonlighter/main.go:19: 1 -> 2 | os.Exit(1)
- signals_windows.go:19: 8 -> 9 | input := struct{ Size, Time uint32 }{Size: 8}
- signals_windows.go:20: 0 -> 1 | if r, _, err := user32.NewProc("GetLastInputInfo").Call(uintptr(unsafe.Pointer(&input))); r == 0 {
- signals_windows.go:25: 0 -> 1 | if hr, _, _ := shell32.NewProc("SHQueryUserNotificationState").Call(uintptr(unsafe.Pointer(&state))); hr != 0 {
- signals_windows.go:34: 64 -> 65 | }{Length: 64}
- signals_windows.go:35: 0 -> 1 | if r, _, err := kernel32.NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&mem))); r == 0 {

Kill each swap where the line is behaviour with a test in a new test file named like the repo's tests (`<pkg>/<name>_test.go` beside the code); you may add new test files but must not edit existing ones. For each swap you do not test, write one commit-message line `mutant-skip: <path>:<line> <comment|doc|message|type|equivalent>`. Use `equivalent` only for a swap that cannot change behaviour or a line no test can reach (an error path of an os/exec pipe); a plain behaviour line a test can reach, such as a failing writer or a bad input, is killed with a test, never skipped as `equivalent`.
