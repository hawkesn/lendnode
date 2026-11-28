---
template_version: 0.6.2
planner_model: claude-sonnet-5-5
builder_model: pending-run
target_repo: https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING
unit: reclaim-node-actions
created: 2026-11-28
---

# reclaim-node-actions: lend and reclaim the node Implementation Plan

**Target Repo:** https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING

## Decisions parked for tier-0

- None: no choice here touches a protected path (CODEOWNERS protects only `/.github/workflows/`, which no task edits), a credential or auth, a migration or a planner flag. The kubeconfig default of R4 is only named for the later adapter; this plan reads no credential.

## Seams

#### (a) Stage table

| Order | Piece | Content | What keeps main green |
|---|---|---|---|
| 1 | Task 1 (this plan) | `Agent` with `Tick`, `Lend`, `Reclaim`, and the `Cluster` interface, in plain Go | new file `reclaim.go` beside `lend.go`; no existing file changes |
| 2 | Windows signals | `GetLastInputInfo` with tick-wrap fix, one game signal, free RAM, the `check` subcommand | a `_windows.go` file and a portable stub; cross-compile job |
| 3 | Cluster and WSL adapters, loop | a `Cluster` over `wsl.exe --exec kubectl` with the kubeconfig path, the CPU priority call, the no-terminal loop command | needs stage 1; tested with fakes on Linux |

#### (b) Shared contracts

- `Cluster` has `Node`, `Cordon(ctx, now)`, `Uncordon`, `Pods` and `Evict(ctx, Pod)`. `Evict` has no grace argument, so a caller cannot set a shorter grace; the stage-3 adapter must send no grace override.
- `NodeState{Unschedulable, Ours bool; Since time.Time}` is read from the Node. `Cordon` records `Since` always and sets `Ours` only when it flips the node from schedulable.
- `Agent{Cluster, LowerPriority}`; `Tick(ctx, Settings, Signals, now)` calls `Decide` from `lend.go`.

#### (c) Stitch record

| Piece | Plan file | PR | Merge sha | Deviation |
|---|---|---|---|---|
| Task 1 | `specs/self-feed/reclaim-node-actions.md` | | | none |

## Intent

#### Goal

moonlighter can lend its node (uncordon) and hand it back (cordon first, then the owner's leave mode), as plain Go over a small `Cluster` interface, with every acceptance row of the card that needs no Windows or cluster tested against a fake.

#### Operator intent

- I1: "i'd like to be able to toggle this behaviour from the worker node. A cut/kill could break a job, I'm open to letting it finish up if I only need to wait a little bit."

#### Choices made

- Epic fit (hawkesn/embersuite#1948): this is the part that gives the machine back when its owner returns, and a finish mode that waits before evicting protects the owner's wish not to break a job (I4 of the epic); it is stdlib Go, so it builds on pi, mac, linux and windows (I1).
- Files: new `reclaim.go` (package `moonlighter`, beside `lend.go`) and new `reclaim_test.go`. The repo's Go tests sit beside the package, so the `test/<module>-<topic>.test.ts` name does not apply (same choice as `lend.go`). The existing `lend_test.go` is not edited; `lend.go` is not edited.
- Taken: a `Cluster` interface instead of `k8s.io/client-go`. Alternative: add client-go and use its fake clientset. Impact: client-go means a `go.mod`/`go.sum` change (security-or-trust) and far more than 120 lines; the interface is faked in the same way and the stage-3 adapter implements it.
- Taken: this plan is stage 1 of 3 (see Seams); the card's Windows signals, `check` subcommand, WSL adapter and loop are later stages, so the last contract says relates.
- Taken: the game signal reaches `Decide` through `Signals.OwnerInput` (set by stage 2), so a game cordons first via the existing owner-input path. Alternative: a new `Signals.Game` field. Impact: none now, and `lend.go` stays unchanged.
- Taken (R5): moonlighter's own cordon is told apart by `NodeState.Ours`, set by `Cordon` only when the node was schedulable; the reclaim start `Since` is recorded even on a foreign cordon. Alternative: one annotation for both. Impact: an owner or admin cordon is never undone, and a finish wait still counts from the reclaim start.
- Taken (restart): all reclaim state is in `NodeState`, read each tick; a new `Agent` continues from it. No file state.
- Taken (T2): `Reclaim` never calls `Uncordon`; an eviction error is returned (all pods are still tried, errors joined) and the node stays cordoned. Only `Lend` uncordons.
- Taken: an unknown or empty leave mode cordons and then does nothing more (fail closed; a failed settings load gives `Settings{}`).
- Taken: finish mode evicts after `MaxWait` from `Since`; "the running unit" is read as the non-DaemonSet pods on the node, evicted with `Evict` and the pod's own grace.
- Review class (not a contract field): prod-code.
- Risk class `ordinary`: no workflow, manifest, `go.mod` or credential changes.

#### Non-goals

- Reading any Windows signal, the `check` subcommand, or the run loop (stages 2 and 3).
- A real `Cluster` (kubectl or client-go), the kubeconfig path (R4), and the real CPU priority call (stage 3).
- Sleep, reboot or logoff as an unclean hand-back; deleting the Node; editing taints.
- `.github/workflows/ci.yml`, `go.mod`, `lend.go`, `lend_test.go`, `README.md` do not change.

## Constraints

#### Requirements

- R1: Lend is uncordon; reclaim is cordon first, then the leave mode; the Node is never deleted.
- R2: finish waits up to `MaxWait` from the reclaim start, then evicts; slow lowers CPU priority and never evicts; now lowers CPU priority, then evicts.
- R3: No eviction sets a grace; the pod keeps its own.
- R4: moonlighter only uncordons a cordon it made itself.
- R5: A restart mid reclaim continues from the node's state.
- R6: Commits use the author address 1168175+hawkesn@users.noreply.github.com; past commits are not rewritten.
- R7: The code builds and vets for windows, darwin and linux on amd64 and arm64; tests run on Linux.

## Validation

#### Acceptance criteria

- Given a lent node, when a game or owner input signal arrives, then the first action is a cordon.
- Given finish mode and a running unit, when the maximum wait has not passed, then no eviction; when it passes, then one.
- Given slow mode, when reclaim runs, then cordon, then CPU priority lowered, and no eviction.
- Given now mode, when reclaim runs, then cordon, then CPU priority, then an eviction with no grace.
- Given a node an owner or admin cordoned, when the decision says lend, then it stays cordoned.
- Given a restart in the middle of a reclaim, when the program starts again, then it continues from the node's state.

#### Edge cases

- An eviction refused by the API: the error is returned, other pods are still tried, the node is not uncordoned.
- The cluster API is unreachable: `Node` fails, nothing else is called, the error is returned.
- A DaemonSet pod is never evicted.
- Zero `Settings` (failed load) cordons and evicts nothing.

#### Tests

All tests are in the new `reclaim_test.go` and fail at the RED start because `Agent`, `NodeState`, `Pod` and `Cluster` do not exist. The fake records calls in order, so ordering is asserted. A test of a limit asserts the limit, not an exact observed value. The mutation probe is the eight wrong patches `test/fixtures/reclaim_test-wrong-1.patch` to `-8.patch` (finish without wait, now with evict before lower, lend of a foreign cordon, DaemonSet eviction, slow evicting, uncordon on eviction error, re-cordon on restart, unknown mode evicting); each fails the oracle.

- A1 [R1, T1] TestNodeGameSignalCordonsFirst: owner input on a lent node makes the first call `cordon`.
- A2 [R2, T2] TestNodeFinishWaitsForMaxWait: no eviction at 0 and 4 minutes, one at 5 minutes, one cordon in all.
- A3 [R2] TestNodeSlowLowersPriorityAndNeverEvicts: calls are exactly cordon, lower.
- A4 [R1, R2, R3, T3] TestNodeNowOrdersCordonLowerEvict: calls are exactly cordon, lower, evict of the non-DaemonSet pod.
- A5 [R4, T4] TestNodeLendUncordonsOnlyItsOwnCordon: a foreign cordon is kept, an own cordon is uncordoned once, a foreign cordon stays after a reclaim began on it.
- A6 [R5, T4] TestNodeRestartMidReclaimContinuesFromNodeState: a new agent on a cordoned, started state does not cordon again and evicts after the wait.
- A7 [T5] TestNodeFailuresNeverUncordon: a refused eviction is returned, the next pod is still tried, no uncordon; an unreachable node returns the error with no action.
- A8 [T6] TestNodeUnknownLeaveModeNeverEvicts: zero `Settings` evicts nothing.
- A9 [R6, R7, T7] build-vet-and-cross-compile: not a unit test; the proof runs `go vet` for the six targets; R6 is the commit author.

## Risk

#### Failure modes

- If finish ignored the wait, a running job would be evicted at once. A2 and wrong patch 1 guard it.
- If a slow eviction led to an uncordon (nodenap's give-up), the machine would take work while the owner plays. A7 and wrong patch 6 guard it.
- If a foreign cordon were undone, an admin's hold would be lost. A5 and wrong patch 3 guard it.

## Traps / gotchas

- T1: Contract: `Reclaim` and `Tick` call `Cordon` before anything else when the node is schedulable or has no `Since`. Failing input: owner input on a zero `NodeState` must give `calls[0] == "cordon"`.
- T2: Trap: the finish wait counts from `NodeState.Since`, not from `now` of each tick; counting from `now` never reaches `MaxWait`. `now.Sub(Since) < MaxWait` waits; equal to `MaxWait` evicts. Failing input: `Since = t0`, now `t0+5m`, `MaxWait` 5m must evict.
- T3: Trap: `Evict(ctx, Pod)` has no grace parameter by design (card T1: nodenap sets none; a shorter grace risks a kill). Skip pods with `DaemonSet` true. Order for now mode: cordon, lower, evict.
- T4: Contract: `Lend` returns without a call when `Unschedulable && !Ours`, and when `!Unschedulable && Since.IsZero()`; otherwise it calls `Uncordon` (which clears `Ours` and `Since`). `Cordon` always records `Since` but sets `Ours` only if it flips the node. Failing input: `NodeState{Unschedulable: true}` on lend must make no `uncordon` call.
- T5: Trap: `Reclaim` must never call `Uncordon`, even when `Evict` or `Pods` fails (nodenap's `abortDrain` re-enables the node). Collect eviction errors with `errors.Join` and keep going; a `Node` error returns at once with no other call.
- T6: Trap: only `LeaveSlow`, `LeaveNow` and `LeaveFinish` act; any other value (including empty from a failed load) returns after the cordon. Do not let it fall through to the eviction.
- T7: Tests need no Postgres or Docker, the repo has no `test/integration` and no `package.json`, so `npm ci` does not apply; Go 1.26.10 is installed and was used. Keep `go.mod` unchanged, stdlib only. The reference is 95 lines in `reclaim.go`; do not add helpers beyond it.

## Hazards

Tables declared: none apply. shell tool list, regex or parser list, doc table list, concurrent DB writer list, runs-where list and port: `not-touched` (one library file; nothing is bound in an entry point yet).

## External calls

- none: the agent calls only the `Cluster` interface, which the tests fake; no call leaves the repo in this plan (the real kubectl and `wsl.exe` calls come in stage 3).

## Tasks

### Task 1: Add the node lend and reclaim agent

**Belief:** a-lent-node-capacity-premise-sums-whole-pod-requests-and-every-kind-that-can-land-there

**Intent:** I1

**Goal.** Create `reclaim.go` in package `moonlighter` with `Pod`, `NodeState`, `Cluster`, `Agent` and the methods `Tick`, `Lend` and `Reclaim`, so that `reclaim_test.go` passes.

**Leave alone.**
- Change no file except `reclaim.go`.
- Do not edit `reclaim_test.go`; it holds the tests you must make pass. If the mutant attack lists surviving mutants, add new test files to kill them and do not edit existing ones.
- Do not touch `lend.go`, `go.mod`, the workflow or any existing test.
- Standard library only.

**Facts measured at the start commit.**
- `grep -n "func Decide" lend.go` finds `Decide(s Settings, sig Signals, now time.Time) (bool, string)`; `Settings` has `LeaveMode` and `MaxWait`; `LeaveFinish`, `LeaveSlow`, `LeaveNow` are constants.
- `ls *.go` shows no `reclaim.go`; `go.mod` has no requires.
- Existing wrong patches are `lend_test-wrong-1..8` and `preflight_test-wrong-1..6`; this plan's are `reclaim_test-wrong-1..8`.

**The failing test.** `reclaim_test.go` does not compile at the start commit: `NodeState`, `Pod`, `Agent` are undefined.

**The change.** Write `reclaim.go` as the Traps T1 to T6 describe: `Tick` calls `Decide` and then `Lend` or `Reclaim`; `Lend` per T4; `Reclaim` reads `Node`, cordons when needed (T1, T4), then switches on the leave mode (T6), then evicts every non-DaemonSet pod and returns the joined errors (T5). `LowerPriority` is optional (nil does nothing).

**Done when.** `go test reclaim_test.go reclaim.go lend.go` exits 0.

**Task Contract:**

- item: reclaim-node-actions
- relates: hawkesn/embersuite#1951
- rung: 1
- risk_class: ordinary
- repo: https://github.com/hawkesn/moonlighter.git
- base_sha: START_SHA_PENDING
- branch: autoloop/reclaim-node-actions
- task_class: build-to-green-against-existing-tests
- oracle_cmd: `go test reclaim_test.go reclaim.go lend.go`
- test_cmd: `go test lend_test.go lend.go`
- max_added: 100
- max_deleted: 0
- test_files:
  - "reclaim_test.go"
- allowed_files:
  - "reclaim.go"
- release: batch

## RED start files

#### test edit

- bytes: sha256:31ad87feb24bac0ac27411acb12f35463ab28dc9265d5e71c844b530e7387309

#### reference patch

- bytes: sha256:50962c955534ea72148815af7e35fb9dd9f4cb544132fde5b4834d95069eb28e

## Surviving mutants: the tests must kill these

- reclaim.go:60: || -> && | if !st.Unschedulable || st.Since.IsZero() {

Kill each swap where the line is behaviour with a test in a new test file named like the repo's tests (`<pkg>/<name>_test.go` beside the code); you may add new test files but must not edit existing ones. For each swap you do not test, write one commit-message line `mutant-skip: <path>:<line> <comment|doc|message|type|equivalent>`. Use `equivalent` only for a swap that cannot change behaviour or a line no test can reach (an error path of an os/exec pipe); a plain behaviour line a test can reach, such as a failing writer or a bad input, is killed with a test, never skipped as `equivalent`.
