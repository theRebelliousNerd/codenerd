# Open items — found, verified, deliberately not fixed

Date: 2026-09-09
Companion to [00-FINDINGS.md](00-FINDINGS.md).

Each item below was verified with file:line evidence during the hardening pass
and left alone on purpose. Every one states why, so the next person does not
re-derive the analysis.

---

## O1 — Per-shard permissions are declared and enforced by nothing

**Severity: medium. Defence-in-depth that is not in depth.**

Ten shard profiles declare a `Permissions` list
(`internal/core/shards/config.go:15,34,53,72`,
`internal/shards/registration.go:624,674,692,711,728`,
`internal/shards/requirements_interrogator.go:29`), drawn from a vocabulary that
maps directly onto tool names: `read_file`, `write_file`, `exec_cmd`, `network`,
`browser`, `code_graph`, `ask_user`, `research`
(`internal/types/shard.go:36-43`).

The only reader is `BaseShardAgent.HasPermission`
(`internal/core/shards/agents.go:92`), which has **no non-test caller**. The
values never reach the kernel either — no `.mg` file mentions `ShardPermission`
or any equivalent predicate.

There is a second, also-unwired mechanism: `stage_shard_tool_allowed(ShardID,
ToolName)` (`stage_context.mg:59,285,294`) derives a per-shard tool allow-list
in Mangle and has **no Go consumer** — only `internal/core/stage_context_test.go`
queries it.

So a researcher shard that declares it may not write files is not prevented from
writing files by either mechanism. The actual gates are the constitution's
`permitted(ActionType, Target, Payload)` — which takes **no actor argument**
(`schemas_safety.mg:15`), so it cannot distinguish shards by construction — and
the session's `AllowedTools`.

**Why not fixed here.** Wiring either mechanism is a behaviour change that would
begin denying tools shards currently use, and validating that needs interactive
runs against real workloads, not a test suite. Choosing between them is also an
architectural decision, not a wiring repair.

**Recommendation.** Prefer the Mangle path: assert `shard_permission(ShardID,
Permission)` at spawn, join it into `stage_shard_tool_allowed`, and have the
session intersect that with `AllowedTools`. That keeps enforcement in the
executive rather than in a Go helper, which is the project's stated inversion of
control. Ship it behind a feature flag and log denials for a release before
enforcing.

## O2 — `atom_context_boost` has no producer, so half of atom scoring is inert

`jit_logic.mg:90-92` derives `atom_matches_context(AtomID, FinalScore)` from
`prompt_atom` joined with `atom_context_boost`, described in the schema as a
virtual predicate ("Go-computed boost", `schemas_prompts.mg:448`).

`RegisterVirtualPredicate` (`internal/mangle/differential.go:847`) has **no call
site anywhere in the repo**. The virtual-predicate mechanism is entirely unused,
so nothing produces `atom_context_boost` and the non-mandatory branch of
`atom_matches_context` never fires in production. The mandatory branch
(`:94-95`, score 100 for `is_mandatory` atoms) does.

The thirteen `atom_has_*_match` rules that fed the same area **were** removed in
this pass — they were dead at both ends (see F12). This one is different: it has
a live consumer chain and a missing producer, so removing it would delete
working logic, and implementing it means deciding what the boost should be.

**Recommendation.** Either implement the boost as a real Go-side scorer and
register it, or replace the virtual predicate with ordinary derived rules over
facts the selector already emits (`atom_priority`, `atom_tag`,
`current_context`). Do not leave it as a third unused mechanism.

## O3 — `internal/context` never reaches three of four entry points

The spreading-activation and semantic-compression subsystem
(`internal/context/`, ~5.3k non-test lines) is imported **only** by
`cmd/nerd/chat/**` and `cmd/nerd/cmd_context_stats.go`. `internal/session` does
not import it at all.

So every `nerd <verb>` CLI run, every spawned subagent and every campaign shard
gets no compression — their only history control is the fixed 6-message /
24,000-character window at `internal/session/executor.go:1688`. Only the
interactive TUI benefits.

**Why not fixed here.** Threading a compressor through the executor is a
substantial change to the session lifecycle, not a wiring gap, and the two paths
have genuinely different lifetimes: a TUI session is long-lived and a CLI verb
is not. It may be correct as designed. It is not *documented* as designed, which
is the actual defect.

Several exported `ActivationEngine` methods have no non-test callers as a
consequence: `ScoreFacts`, `SelectWithinBudget`, `SpreadFromSeeds`,
`SetCorpusPriorities`, `AddDependency` (`internal/context/activation.go:266,
411, 573, 229, 493`).

## O4 — Dead exported surface not worth touching yet

Verified zero non-test callers, left in place because each is the visible half of
a capability whose other half is also unwired — the repo's `audit-before-delete`
rule says find the gap before deleting, and the gap is a design decision in each
case:

| Symbol | File | Note |
|---|---|---|
| `SelfHealer` (whole type) | `internal/core/self_healing.go:71` | `internal/session/build_verify.go:35-38` documents that it supersedes this. Two of its strategies are also stubs. Wire or delete — a maintainer's call. |
| `LimitedExecutorInterface`, `SandboxedExecutorInterface`, `CompositeExecutorInterface` | `internal/tactile/executor_interface.go:31,39,50` | Whole file unreferenced. |
| `NewPersistentDockerExecutor`, `DefaultContainerPoolConfig` | `internal/tactile/persistent_docker.go:92,149` | Container pool never constructed. |
| `GetUserJourneyState`, `RecordSessionStart`, `CheckJourneyTransition`, `GetExperienceLevelFromPreferences`, `GetDisclosureLevel` | `internal/ux/migration.go:211,220,238,262`, `user_state.go:74` | Progressive disclosure: journey state is never recorded and disclosure level never computed. `ShouldShowOnboarding` in the same package **is** live. |
| `SetGlobalAllowlist`, `SetGlobalFactSink` | `internal/tools/registry.go:594,630` | Instance-level equivalents are wired at `internal/core/virtual_store_tools.go:65,69`. |
| `ReconcileEmbeddedCorpus` | `internal/prompt/reconciler.go:301` | Sibling `ReconcilePromptCorpus` is live. |

## O5 — Repo-wide `gofmt` drift

`gofmt -l internal cmd` reports **55 files** unformatted at the branch point, all
pre-existing. Files touched by this pass were formatted; the rest were left
alone so the diff stays readable.

**Recommendation.** One mechanical `gofmt -w` commit on its own, then a CI gate.
Mixing it into feature work is what let it reach 55 files.

## O6 — Current-turn user input is uncapped into generation

Classification caps user input at 50,000 characters; the generation path does
not. This is the one remaining uncapped path into a live prompt.

Left alone deliberately: truncating the user's actual request is a semantic
decision, not a hygiene one, and the failure mode is a clean provider 413 rather
than the model reasoning on a silently shortened request. It is bounded on the
replay side now, so it can affect at most one turn.

**Recommendation.** A configurable cap with a visible CLI warning, so the user
learns their input was shortened rather than the model quietly working from
half of it.

## O7 — `tests/e2e`: from not compiling to green (RESOLVED)

The integration suite had not compiled since `8e9507d`. Once it compiled it did
not finish: it hung for the full 12-minute package timeout, so the run reported
three failures and silence — the silence was 30-odd tests that never got to
run.

### What was actually wrong

Nine tools across three e2e files were registered without `Tool.Effect`:

```go
tools.Global().Register(&tools.Tool{Name: "race_tool", Execute: ...})
//                                                     ^ no Effect
```

`executeToolCall` resolves an effect before it dispatches
(`internal/session/executor_tools.go:2127`) and refuses the call when none
resolves — the executive gate will not run what it cannot classify. That is
correct and deliberately fail-closed. The problem is how quietly it fails:
`Register` validated `Name` and `Execute` and never `Effect`, so the tool
registered, was catalogued, was offered to the model, cleared the JIT
allowlist, and then every invocation returned an error from a call site two
thousand lines away while the `Execute` closure was never entered.

The visible symptom is a nil error, a plausible response, `ToolCallsExecuted`
counting the attempt, and only `SuccessfulToolCalls` staying at zero. The tests
asserted on counters their tool bodies incremented, saw `0`, and reported
`"Race corruption: Expected 50 executions, got 0"` and `"OOM limit bypassed or
execution dropped"` — confident, specific, plausible diagnoses of a bug that
did not exist. `TestE2E_TemporalFailure_GoroutineLeakPrevention` blocked on an
unbuffered channel that its tool body was supposed to close, which is what
timed out the package and hid everything after it.

**This corrects `5837328`.** That commit's message attributed the failures to
`buildToolDefinitions` producing nothing against the executor's config
contract. That was wrong. Tool definitions were built correctly; the refusal
happened later, at the effect lookup. The evidence is a one-field diff:
adding `Effect: tools.EffectRead` moved a probe from
`SuccessfulToolCalls:0, called=0` to `SuccessfulToolCalls:1, called=1` with
nothing else changed.

### Fixed in this pass

- `Effect` declared at all nine registration sites, plus the eight in
  `internal/tools`' own tests.
- `Registry.Register` now rejects a tool whose effect does not resolve, so the
  author finds out at the registration site instead of at dispatch.
  `DeclaredEffect`, not `tool.Effect`: built-ins take their effect from the
  reviewed `BuiltinEffect` manifest by name and set no field.
- `TestEffects_WhenToolRegistered_ShouldDeclareAResolvableEffect` asserts the
  property for every tool the production registrars install (45 today, all
  clean — this was only ever a test-scaffolding defect), with a non-vacuity
  floor so it cannot silently become a check over an empty set.
- The unguarded `<-started` receive is now bounded, so a future regression
  fails in five seconds naming the cause instead of timing out the package.
- `tests/e2e/write_turn_fixture_test.go`, the shared write-turn fixture the
  orchestrator files needed — see the next section for what it does and why the
  cheaper fix was the wrong one.

Result: the suite completes instead of hanging at 720s, and the failure list is
visible and precise — 29 failures at that point, zero after the work below.

### The rest of the suite: 29 failures down to zero

`8e9507d` added `checkHollowSuccess`: a write-oriented intent that finishes with
no successful write-mutation tool call is refused. The guard is right. Every
orchestrator fixture in the package predated it and drove `/fix` against mocks
that called nothing, so once the suite could run at all, 29 tests failed.

**The obvious fix is wrong here.** The sibling piggyback/race file took a
one-line verb change (`/fix` → `/explain`) because the verb there was incidental
to the mechanics under test. That does not transfer: in the orchestrator files
the verb *is* the mechanism — `"/fix" is treated as inline` and `Use "/research"
to force Subagent isolation`. Swapping it would change which execution path each
test exercises while turning the bar green — the precise failure this audit
exists to find.

So `tests/e2e/write_turn_fixture_test.go` makes the fixtures complete a **real
write turn** instead. Three things were missing, each a genuine gap:

1. **The VirtualStore never received the kernel.** Every fixture built a real
   `RealKernel` and a real `VirtualStore` side by side and never connected them.
   `getDreamer` derives the Dreamer lazily from `v.kernel`, so with a nil kernel
   there is no Dreamer, and `PreflightDestructiveToolCall` is fail-closed on
   exactly that — *"permission and speculative safety are independent gates; an
   allow decision from checkSafety must never compensate for a missing
   simulation engine."* Every destructive call was blocked before reaching a
   tool. One `SetKernel` call fixes it.
2. **No write-mutation tool.** The name `write_file` matters twice:
   `projectdoc.IsWriteMutationTool` recognises it, and `BuiltinEffect` resolves
   its `EffectWrite` with no `Tool.Effect` field needed.
3. **Nothing in `AllowedTools`.** The config factories returned an empty
   `EffectiveAgentRuntimeConfig`, so the JIT allowlist authorised nothing.

The stub must *really* create the file — the post-action validator checks the
side effect landed, so a stub returning `"wrote"` without writing is correctly
judged hollow. And it must write **atomically** (temp file + rename): several
tests drive ten or fifty goroutines through one executor sharing a single
fixture path, and a plain `os.WriteFile` truncates before writing, so a
validator racing a concurrent writer sees an empty file and fails the call. That
single detail was the difference between a suite that varied run to run and one
that does not.

**Result: 29 → 0**, stable across three consecutive whole-package runs. The
`-tags integration` CI job is added in the same commit, and not one commit
earlier: a job that fails on its first run teaches everyone to ignore it, which
is worse than no job because it also hides the day it starts failing for a real
reason.

Five of those came from outside the shared fixture, and each was a test
asserting something the code deliberately does not do:

- `StringAllocationPressure` built a hundred one-megabyte targets distinguished
  by an appended `_%d`. `Intent.ToFact` runs Target through `sanitizeFactArg`,
  which truncates at 2048 bytes, so all hundred truncated to the same 2048 "A"s
  and the kernel correctly stored one fact. The truncation is a deliberate
  injection-and-size guard on a field carrying user input; the test had put its
  only distinguishing information past the cut. Prefixing the index fixes it.
- `State_ExecutorIndependence` asserted a hundred `concurrent_load` facts
  concurrently and read back zero, reporting `"Concurrency lost data"`.
  `concurrent_load` is declared nowhere — see the finding below. Switched to
  `critical_file`, and to a membership check rather than a raw count, since the
  corpus asserts `critical_file` facts of its own at boot.
- `VirtualStore_Unavailable_AgentGracefulFail` asserted that `SpawnSpecialist`
  errors for a name with no on-disk config. It does not, by design:
  `loadSpecialistConfig` falls back to JIT generation with a documented reason.
  The test pinned the opposite of the intended behaviour, which costs more than
  a red bar — it would have blocked anyone relying on the fallback. It now
  asserts the degradation *and* that a path-traversing name is still rejected.
- `MultiTurn_ConversationDrift` cycles `/explain`, `/fix`, `/test`, `/review`
  across twenty turns against a **nil** VirtualStore, so there was no executive
  gate at all. A real store plus the write fixture fixes it without narrowing
  the verb mix, which is what "conversation drift" means here.
- `PiggybackExecutor_ControlPacket` needed a legitimate write beside its
  adversarial requests. Its JIT allowlist now admits `e2e_safe_tool` and
  `write_file` and still refuses `e2e_forbidden_tool`, so the boundary under
  test is unchanged; the scenario is simply realistic, since a real `/fix` turn
  writes something. It got a real `core.VirtualStore` rather than a no-op gate,
  because a rubber-stamped gate in the one test whose premise is
  `EnableSafetyGate = true` would have been worse than the red bar.

### Concurrency: it was never the file

Two orchestrator tests stayed intermittently red after all of the above, one or
the other failing per run with `attempted=1`. Both drive ten or fifty goroutines
through a single executor.

The first cause was the file write: `os.WriteFile` truncates before it writes,
so a post-action validator racing a concurrent writer sees an empty file and
judges the side effect absent. Writing to a temp file and renaming fixed that —
rename is atomic, so every observer sees a complete file.

The second was subtler and is the interesting one. `executeToolCall` asserts
`pending_edit(FilePath, Content)` before a write-mutation tool and retracts it
on every exit path. Ten goroutines writing the *same* path with the *same*
content assert an identical fact; the kernel dedupes it to one; the first
goroutine to finish retracts it out from under the nine still running. The
fixture now mints a distinct path per call, which is both what the lifecycle
assumes and what a real turn looks like — nothing writes one file from ten
goroutines.

Worth stating plainly, because two tests spent this whole audit claiming
otherwise: neither failure was a concurrency defect in the executor. One was a
non-atomic write in a test stub, the other a shared-fact lifetime in a fixture.

### A new finding: `Assert` accepts undeclared predicates and drops them

Measured directly against a fresh `RealKernel` with 1839 Decls loaded:

| Predicate | `Assert` returns | `Query` returns |
|---|---|---|
| `critical_file` (declared) | `nil` | the facts |
| `concurrent_load` (undeclared) | **`nil`** | **nothing** |

`Fact.ToAtom` is Decl-blind, so a fact whose predicate was never declared
converts cleanly, lands in the EDB, and returns `nil` — every signal the caller
has says it worked. The fixpoint only derives what the program declares, so the
fact is written into a space nothing reads.

A static scan of non-test Go against the corpus found **82 distinct predicates
asserted from production code with no declaration anywhere**, including whole
state machines — `campaign_paused`, `tdd_phase`, `ouroboros_phase`,
`python_snapshot` — whose facts have never been visible to a rule.

**What was done.** `RealKernel.warnIfUndeclaredLocked` now names the predicate
and its arity, once per predicate per process, so the silence is gone. Arity is
part of the identity on purpose: a fact asserted at an arity the `Decl` does not
declare is invisible for exactly the same reason and reads as a far more
confusing bug. `TestUndeclaredAssertBudget` pins the count at 82 and fails in
either direction, the mirror of `TestStarvedPredicateBudget` — starved is
*declared and read, produced by nothing*; undeclared is *produced by Go,
declared by nothing*.

**What was deliberately not done.** `Assert` does not start returning an error.
Eighty-two live call sites would begin failing at once, and the honest fix for
each is a per-predicate decision — declare it and wire a consumer, or drop the
assert. Declaring all 82 without consumers would only move them onto the
starved list, trading one silent failure for another. That is a program of work
with a measurement attached to it now, which is the point.

### A measurement trap worth recording

Two earlier readings of this suite — "10 failures", then "15" — were both wrong,
and the recorded conclusion drawn from them ("the write fixture is net negative:
−7 here, +12 there") was exactly backwards. The cause was mundane: both runs
were piped through `tail -40` / `tail -50`, so the counts were *the tail of the
output*, not the number of failures. The fixture that reading rejected is the
one that took the suite from 29 to 5, on the way to zero.

Two habits follow. Count failures with `grep -c '^--- FAIL'` over the whole
output, never a tail. And compare only whole-package runs: `tools.Global()` is
one registry shared by every test in the package, so a `-run` subset sees
different global state than a full run — `TestE2E_OrchestratorExecutor_Smoke`
failed alone and passed in a full run for exactly that reason. Per-test registry
isolation (a `t.Cleanup`-scoped registry, or `tools.NewRegistry()` injected
instead of the global) would remove the hazard.

### The gates themselves were the defect, twice

Both new CI gates failed on their first real run, and both failed in the exact
way this whole branch is about — a check that looks like it is running and is
not.

The dead-code budget compared a Linux-recorded baseline against a Windows run.
Reachability is computed per build configuration, so a file excluded by build
tag is not analysed at all and its functions are absent from the report — not
"reachable", just invisible. Thirty-eight `platform_linux` / `platform_unix`
functions read as "no longer unreachable" with nothing changed. Pinning GOOS
inside the script cannot fix it from a Windows host, because cross-compiling
disables cgo and this module needs it (go-tree-sitter), so the analysis fails to
typecheck instead of answering differently. The job moved to `ubuntu-latest`,
where its baseline was taken, and the pin stays as a guard that makes any future
mismatch loud.

The gofmt gate expanded 2049 paths into one argv, died with `Argument list too
long` at exit 126 before checking a single file, and then — once that was fixed
with `xargs -0` — flagged the entire tree, because `.gitattributes` leaves `.go`
files CRLF on Windows while gofmt normalises to LF. Pinning `*.go text eol=lf`
would fix the gate and also change what every Windows contributor gets on
checkout, which is not a formatting gate's call to make, so it moved to Linux
too.

Each of these failed loudly enough to look like a real finding. That is the most
expensive way for a gate to be wrong: it costs a reviewer a full investigation
and then teaches them to skip the job.

### Related: O5 is only half-closed

The `Formatting` CI gate added in this pass covers `internal` and `cmd`.
`gofmt -l tests` still reports **13** unformatted files. Left out so this diff
stays reviewable; widening the gate to `tests` should be its own mechanical
commit, exactly as O5 recommends for the original 55.
