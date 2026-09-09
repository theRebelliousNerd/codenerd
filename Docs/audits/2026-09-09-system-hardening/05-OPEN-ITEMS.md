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

## O7 — `tests/e2e`: ten failures, one shared shape

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
visible and precise — 29 failures at that point, 5 after the fixture below.

### The rest of the suite: 29 failures down to 5

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

**Result: 29 → 5, identical across consecutive runs.** Twenty-four tests fixed,
none broken.

### Still failing — 5

| Test | File | Error |
|---|---|---|
| `TestE2E_CrossBoundary_Executor_MultiTurn_ConversationDrift` | `cross_boundary_integration_test.go:507` | `/fix ... attempted=0` |
| `TestE2E_PiggybackExecutor_ControlPacket_EndToEnd_HardBoundary` | `piggyback_executor_full_boundary_test.go:312` | `write-oriented intent /fix completed without a recognized write-mutation tool (tool_calls=2)` |
| `TestE2E_Boundary_Session_Kernel_StringAllocationPressure` | `Session_Kernel_Boundary_integration_test.go:358` | `Failed large string allocation test` |
| `TestE2E_SessionKernelVStore_State_ExecutorIndependence` | `session_kernel_vstore_integration_test.go:912` | `Expected 100 facts, got 0` |
| `TestE2E_Session_VirtualStore_Unavailable_AgentGracefulFail` | `session_spawner_config_integration_test.go:414` | expected an error loading a non-existent specialist; also panics on a nil `spawnerMockKernel` receiver via `executor.go:871` |

Each needs bespoke work rather than the shared fixture:

- **ConversationDrift** cycles `/explain`, `/fix`, `/test`, `/review` across 20
  turns and passes a **nil** VirtualStore, so there is no executive gate at all.
  It needs a real store plus a `/test`-satisfying tool (`run_tests`,
  `EffectExecute`), which is a second fixture, not a reuse of this one. The verb
  mix is deliberate — it is what "conversation drift" means here — so it should
  not be narrowed.
- **Piggyback ControlPacket** now reaches `tool_calls=2`: its tools run, but
  `e2e_safe_tool` is not a write-mutation tool. Its subject is an adversarial
  piggyback envelope, so the right fix is a write-mutation tool in *that*
  fixture, not a verb change.
- The last three are unrelated to hollow success and are each their own
  investigation. The spawner one panics on a nil mock receiver, which is a
  defect in the mock rather than in production code.

**Recommendation.** Add the `-tags integration` CI job only once these five are
green: a job that fails on its first run is worse than no job, because it
teaches everyone to ignore it.

### A measurement trap worth recording

Two earlier readings of this suite — "10 failures", then "15" — were both wrong,
and the recorded conclusion drawn from them ("the write fixture is net negative:
−7 here, +12 there") was exactly backwards. The cause was mundane: both runs
were piped through `tail -40` / `tail -50`, so the counts were *the tail of the
output*, not the number of failures. The fixture that reading rejected is the
one that took the suite from 29 to 5.

Two habits follow. Count failures with `grep -c '^--- FAIL'` over the whole
output, never a tail. And compare only whole-package runs: `tools.Global()` is
one registry shared by every test in the package, so a `-run` subset sees
different global state than a full run — `TestE2E_OrchestratorExecutor_Smoke`
failed alone and passed in a full run for exactly that reason. Per-test registry
isolation (a `t.Cleanup`-scoped registry, or `tools.NewRegistry()` injected
instead of the global) would remove the hazard.

### Related: O5 is only half-closed

The `Formatting` CI gate added in this pass covers `internal` and `cmd`.
`gofmt -l tests` still reports **13** unformatted files. Left out so this diff
stays reviewable; widening the gate to `tests` should be its own mechanical
commit, exactly as O5 recommends for the original 55.
