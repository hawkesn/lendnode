---
template_version: 0.6.2
planner_model: claude-sonnet-5-5
builder_model: pending-run
target_repo: https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING
unit: lend-decision
created: 2026-11-28
---

# lend-decision: decide whether to lend a machine now Implementation Plan

**Target Repo:** https://github.com/hawkesn/moonlighter.git@START_SHA_PENDING

## Decisions parked for tier-0

- None: no choice here touches a protected path (CODEOWNERS protects only `/.github/workflows/`, which no task edits), a credential or auth, a migration, or a planner flag. The leave mode is a plain setting this card only parses.

## Intent

#### Goal

One pure function in moonlighter answers "lend now?" from the device's settings and signals, and the settings include the leave mode the owner picks on the device.

#### Operator intent

- I1: "i'd like to be able to toggle this behaviour from the worker node. A cut/kill could break a job, I'm open to letting it finish up if I only need to wait a little bit."
- I2: "this program should be able to run on any machine/any device (ie. pi, mac, linux, windows)"

#### Choices made

- Epic fit (hawkesn/embersuite#1948): this is the decision every later card acts on, so an owner who installs the app and joins is never lent out while present (I4, I6 of the epic) and the code is plain Go with the standard library only, so it builds on pi, mac, linux and windows (I1 of the epic).
- Files: the code goes in one new file `lend.go` (package `moonlighter`, next to `doc.go`); the tests go in one new file `lend_test.go`. The repo has no test file yet and no `test/` directory, and Go requires a test file to sit beside its package, so the `test/<module>-<topic>.test.ts` name does not apply. Alternative: split into `decide.go` and `settings.go`. Impact: none on behaviour; one file keeps the change within the 120-line limit on added plus deleted lines.
- Taken: the pure function is `Decide(s Settings, sig Signals, now time.Time) (lend bool, reason string)`. Alternative: a `Decision` struct. Impact: none; the pair is shorter.
- Taken: signals are plain values plus one `Err error` field (`Signals{Idle, OwnerInput, FreeRAM, Err}`); the caller joins the errors of its reads. Any non-nil `Err` means reclaim. Alternative: one error per signal. Impact: the same fail-closed answer. A zero `Signals` has zero idle time, so it reclaims.
- Taken: free RAM is carried in `Signals.FreeRAM` (bytes) but no setting gates on it. R2 lists no RAM setting. Alternative: add a `min_free_ram` setting. Impact: the gate waits for a later card that reads real RAM; adding it here would break the line limit.
- Taken: the settings file is plain `key=value` lines (`#` starts a comment) with keys `enabled`, `hours`, `idle_threshold`, `leave_mode`, `max_wait`. Env names are `MOONLIGHTER_` plus the upper-cased key. Flags are `-enabled`, `-hours`, `-idle-threshold`, `-leave-mode`, `-max-wait`, each taking a value (`-enabled=false`). Alternative: JSON. Impact: one text form for all three sources, no new dependency, `go.mod` unchanged.
- Taken: defaults are enabled true (R10), hours `22:00-06:00`, idle threshold 10m, leave mode finish (R3), max wait 5m. Alternative: no default hours. Impact: an empty default would never lend, so install-and-join would not lend; the hours still gate.
- Taken: hours are a comma list of `HH:MM-HH:MM`, start inclusive, end exclusive, a start after the end crosses midnight, an empty list means never lend (fail closed), start equal to end is rejected, and `24:00` is not accepted (use `00:00` as the end). Alternative: empty list means always. Impact: an owner who clears the hours takes the device out of lending.
- Taken: hours compare against the wall clock of the `now` the caller passes (`now.Hour()`, `now.Minute()`), so a daylight-saving change inside the window follows the clock on the wall. Alternative: elapsed time. Impact: a window `01:00-04:00` still holds at 03:30 on the spring-forward day.
- Taken: a failed `LoadSettings` returns the zero `Settings` and an error; its `Enabled` is false, so `Decide` answers reclaim. Alternative: return the defaults. Impact: a typo in the file cannot lend a machine.
- Taken: `max_wait` of zero is valid (finish mode stops new work and waits nothing); a negative value is not checked here because acting on the mode is a non-goal. `idle_threshold` must be above zero.
- Review class (not a contract field): prod-code.
- The `oracle_cmd` names the two Go files because `go test` takes files as arguments; `test_cmd` is a build and vet because the repo has no other test file.

#### Non-goals

- Reading any real signal from the operating system.
- Calling the cluster.
- A tray or menu-bar icon.
- A `pause` subcommand: `enabled` is the manual override.
- Acting on a leave mode: the mode is parsed and validated only.
- `.github/workflows/ci.yml`, `go.mod`, `README.md` and `doc.go` do not change.

## Constraints

#### Requirements

- R1: The decision is a pure function: settings, signals and the current time go in, lend or reclaim and a reason come out.
- R2: Settings are `enabled`, allowed hours, idle threshold, leave mode and the maximum wait used by finish mode.
- R3: Leave modes are finish, slow and now; the default is finish.
- R4: Free RAM is an input signal; any signal error means reclaim.
- R5: The owner changes the settings on the device (file, environment, flag) with no cluster change.
- R6: Words in code are lend, reclaim, hours, device.
- R7: Each setting resolves default, then settings file, then `MOONLIGHTER_` environment variable, then flag; each later source wins.
- R8: The settings are read again on each tick.
- R9: When `enabled` is off, the answer is reclaim.
- R10: `enabled` defaults to true; idle time and hours still gate lending.
- R11: Commits use the author address 1168175+hawkesn@users.noreply.github.com; past commits are not rewritten.
- R12: CI runs only on moonlighter-ci runners; this plan does not touch the workflow.
- R13: The code builds and vets for windows, darwin and linux on amd64 and arm64; tests run on Linux.

## Validation

#### Acceptance criteria

- Given a lent device, when a tick reports owner input, then the answer is reclaim on that tick.
- Given a time inside the allowed hours, when idle time is below the threshold, then the answer is reclaim; at or past the threshold it is lend.
- Given idle time past the threshold outside the allowed hours, then the answer is reclaim.
- Given any signal error, then the answer is reclaim.
- Given settings with no leave mode, when they load, then the mode is finish.
- Given an unknown leave mode, when settings load, then loading fails and the error names the value.
- Given `enabled` is off, then the answer is reclaim.
- Given settings with no `enabled` value, when they load, then `enabled` is true.
- Given a settings file changed between two ticks, when the next tick loads settings, then it uses the new value.
- Given one precedence table test, when each case sets a setting by default, file, environment and flag, then each case yields the value from the highest source present.

#### Edge cases

- Allowed hours that cross midnight (`22:00-06:00`).
- A daylight-saving change inside the allowed hours (America/New_York, 2026-03-08 spring forward; 2026-11-01 fall back).
- Empty list of hours: reclaim.
- A maximum wait of zero in finish mode: loads.

#### Tests

All tests are in the new `lend_test.go` and fail at the RED start because `Settings`, `Decide` and `LoadSettings` do not exist. The test imports `time/tzdata` so the daylight-saving cases need no system zone files. A test of a limit asserts the limit, never an exact observed value. This change touches a decision gate: the mutation probe is the eight wrong patches (`test/fixtures/lend_test-wrong-1.patch` to `-8.patch`), each breaking one guard (idle `<=`, dropped signal error case, default mode now, flags ignored, dropped enabled case, midnight `&&`, empty hours as always, unknown mode accepted); each fails the oracle.

- A1 [R1, R4, T1] TestDecide-signal-and-owner-input: a signal `Err` and owner input each answer reclaim, even with a long idle time.
- A2 [R1, R2, T2] TestDecide-hours-and-idle: threshold boundary (below, at), hours start inclusive and end exclusive, across midnight, two windows, empty hours, and the two daylight-saving days.
- A3 [R9, R5] TestDecide-and-TestEnabledOffReclaims: `enabled` off answers reclaim, set from the file.
- A4 [R3, R10, R2] TestDefaults: no leave mode loads as finish and no `enabled` loads as true.
- A5 [R3, T3] TestUnknownLeaveModeNamesValue: an unknown mode from file, env and flag fails naming `warp`, and the settings it returns reclaim.
- A6 [R7] TestPrecedence: the table over default, file, env over file, flag over env and file, plus a leave mode case.
- A7 [R8, R5] TestSettingsReadAgainEachTick: the file is rewritten between two loads and the second load sees the new value.
- A8 [R2, T4, T5] TestOtherSettings: max wait zero and empty hours load; bad hours, zero idle threshold, bad bool and unknown key fail.
- A9 [R6, R11, R12, R13, T6, T7] build-vet-and-cross-compile: not a unit test; `test_cmd` builds and vets on Linux, and the proof runs `GOOS`/`GOARCH` vet for the six targets; R11 is the commit author; R6 is read in review.

## Risk

#### Failure modes

- If a missing signal is read as zero idle or as false, a machine in use could be lent. A1 and the dropped-signal-error wrong patch guard it; an all-zero `Signals` has idle 0 and reclaims.
- If a bad settings file returned defaults, a typo would lend. A5 checks that the failed load reclaims.
- If the hours test used the host's zone, it would pass on one machine and fail on another. A2 builds every time in an explicit zone.

## Traps / gotchas

- T1: Contract: `Decide` fails closed. Order of checks: `!s.Enabled`, then `sig.Err != nil`, then `sig.OwnerInput`, then outside hours, then `sig.Idle < s.IdleThreshold`, else lend. Failing input: `Signals{Idle: time.Hour, Err: errors.New("boom")}` inside the hours must reclaim; idle exactly at the threshold must lend.
- T2: Trap: hours compare `now.Hour()*60 + now.Minute()` in the zone of the `now` the caller passes; do not convert to UTC. `Window{Start, End}` is in minutes, end exclusive, and `Start > End` crosses midnight: in range when `m >= Start || m < End`. Failing input: `22:00-06:00` at 05:59 lends and at 06:00 does not. No hours at all is never in range.
- T3: Contract: `LoadSettings(path string, lookup func(string) (string, bool), args []string) (Settings, error)`. Pass `os.LookupEnv` as `lookup` in production. A missing file is not an error; any parse error returns `Settings{}` (Enabled false) and a non-nil error. The error for a bad leave mode contains the value, e.g. `unknown leave mode "warp"`. Env name is `MOONLIGHTER_` plus the upper-cased key (`MOONLIGHTER_IDLE_THRESHOLD`); flags use dashes (`-idle-threshold`). Order of application: default, file, env, flags. Failing input: file `idle_threshold=1m`, env `2m`, flag `-idle-threshold=3m` must give 3m.
- T4: Trap: the flag set must use `flag.ContinueOnError`, never `ExitOnError`, and a flag that is not given must not override the env or file value (use `FlagSet.Func`, which runs only for flags that appear). Hours text is `HH:MM-HH:MM` parsed with `time.Parse("15:04", ...)`; an empty string gives an empty list, not an error; start equal to end is an error.
- T5: Trap: file lines are `key=value`; blank lines and lines starting with `#` are skipped; an unknown key is an error, so typos fail closed.
- T6: The line limit is tight: the reference is 119 added lines in `lend.go` (max_added 119 leaves 1 spare). Do not add comments or helpers beyond what the contract needs. Keep `go.mod` unchanged (standard library only).
- T7: No Postgres or Docker is involved: the repo has no `test/integration` directory and no database. `npm ci` does not apply, since the repo has no `package.json`.

## Hazards

Tables declared: none apply. shell tool list, regex or parser list, doc table list, concurrent DB writer list, runs-where list and port: `not-touched` (one pure Go library file; nothing runs yet in an entry point).

## External calls

- none: the function and the settings loader read only the local file, the environment and the flags; nothing calls outside the repo.

## Tasks

### Task 1: Add the lend decision and the settings loader

**Belief:** a-lent-node-capacity-premise-sums-whole-pod-requests-and-every-kind-that-can-land-there

**Intent:** I1, I2

**Goal.** Create `lend.go` in package `moonlighter` with `Settings`, `Window`, `LeaveMode` (constants `LeaveFinish`, `LeaveSlow`, `LeaveNow`), `Signals`, `LoadSettings` and `Decide`, so that `lend_test.go` passes.

**Leave alone.**
- Change no file except `lend.go`.
- Do not edit `lend_test.go`; it holds the tests you must make pass. If the mutant attack lists surviving mutants, add new test files to kill them and do not edit existing ones.
- Do not touch `.github/workflows/ci.yml`, `go.mod`, `doc.go` or `README.md`.
- Use the standard library only.

**Facts measured at the start commit.**
- `ls` shows `go.mod` (module `github.com/hawkesn/moonlighter`, `go 1.26.10`), `doc.go` (package `moonlighter`), `README.md`, `LICENSE`, `NOTICE`, `CODEOWNERS` and `.github/workflows/ci.yml`; there is no `.go` file other than `doc.go` and no `test/integration` directory.
- `CODEOWNERS` holds one line, `/.github/workflows/ @hawkesn`.
- `grep -n "go build\|go vet\|go test" .github/workflows/ci.yml` finds the build, vet and test steps; CI runs `go test ./...` on Linux and build plus vet for six targets.
- `lend_test.go` calls `LoadSettings(path, lookup, args)`, `Decide(s, sig, now)`, the `Signals` fields `Idle`, `OwnerInput`, `FreeRAM`, `Err`, the `Settings` fields `Enabled`, `Hours`, `IdleThreshold`, `LeaveMode`, `MaxWait`, and the constants `LeaveFinish`, `LeaveNow`.

**The failing test.** `go test lend.go lend_test.go` fails at the start commit: `lend.go` does not exist, so `Settings`, `Decide` and `LoadSettings` are undefined.

**The change.** In the new `lend.go`: define `Settings{Enabled bool; Hours []Window; IdleThreshold, MaxWait time.Duration; LeaveMode LeaveMode}`; defaults enabled true, hours `22:00-06:00`, idle 10m, max wait 5m, mode finish. Write `LoadSettings` as in T3 and T4 with one method `(*Settings).set(key, value string) error` that parses each key and is the only place a value is checked. Write `Decide` as in T1 and T2. Keep the whole file at 119 lines or fewer, gofmt-formatted.

**Done when.** `go test lend.go lend_test.go` exits 0, and `go build ./... && go vet ./...` exits 0.

**Task Contract:**

- item: lend-decision
- closes: hawkesn/embersuite#1949
- rung: 2
- risk_class: ordinary
- repo: https://github.com/hawkesn/moonlighter.git
- base_sha: START_SHA_PENDING
- branch: autoloop/lend-decision
- task_class: build-to-green-against-existing-tests
- oracle_cmd: `go test lend.go lend_test.go`
- test_cmd: `go build ./... && go vet ./...`
- max_added: 119
- max_deleted: 0
- test_files:
  - lend_test.go
- allowed_files:
  - lend.go
- release: batch

## RED start files

#### test edit

- bytes: sha256:2c2d884d06ca654f5198915cca2c789dfdb148baf9319e6db9424bfea6f8af51

#### reference patch

- bytes: sha256:cb85b87d3280fedb4781ac852a12eef0a34bef3d8b1dd8812b5fd0c170ea3c03
