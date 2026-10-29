# Session feature cards

> Sole authoritative `NERD_FEATURE` surface for this corpus.

## P0: Enforce exact executive permission

<!-- NERD_FEATURE
id: session-exact-executive-permission-v1
owner: session
status: verified
kind: truth-gap
depends_on: []
affects: [session, core, system, tools]
-->

**Value.** A model-proposed tool call executes only when the kernel permits that
exact action, target, and canonical payload.

**Evidence and observed gap.** Session previously retained a `safe_action/1`
authorization fallback. `internal/session/executor_tools.go#Executor.checkSafety`
now requires exact `permitted/3`; tests cover missing, mismatched, and wrong-arity
facts, nil kernel, canonical nil args, and large payloads.

**Desired behavior.** Treat action classification as input to policy, never the
decision. Keep session and VirtualStore gates aligned on the same envelope.

**Non-goals.** Do not move permission into Go allowlists, prompt instructions, or
tool registration.

**Affected contracts.** Kernel predicates, payload canonicalization, session tool
loop, VirtualStore routing, specialist execution.

**Positive acceptance.** An exactly matching `permitted(Action,Target,Payload)`
allows the call and all absent/mismatched facts deny; real-kernel focused tests pass.

**Negative acceptance.** `safe_action/1`, `permitted/1`, wrong target, `null` vs
`{}`, truncated payload, and stale prior-turn facts cannot authorize.

**Rollback.** Revert only to another exact default-deny decision contract; never
restore classification-as-authorization.

## P0: Fail closed on missing effective capabilities

<!-- NERD_FEATURE
id: session-effective-capability-envelope-v1
owner: session
status: verified
kind: truth-gap
depends_on: [session-exact-executive-permission-v1]
affects: [session, jit, prompt, tools, autopoiesis]
-->

**Value.** JIT/config failure or an empty specialist config cannot grant every
registered tool by accident.

**Evidence and observed gap.** `internal/session/executor_tools.go#isToolAllowed`
now rejects nil/empty configs; `buildToolDefinitions` and Piggyback catalogs expose
only allowlisted modular/Ouroboros tools. Regressions live in
`internal/session/executor_capability_test.go`; integration tests under
`tests/e2e/tool_safety_fallback_config_test.go` now assert the repaired behavior.

**Desired behavior.** Capability is an immutable per-turn envelope derived from
validated config. Registration proves availability, not permission or capability.

**Non-goals.** Capability does not replace `permitted/3`; do not silently recover
missing config from a global registry.

**Affected contracts.** ConfigFactory fallback, tool definitions, native and
Piggyback paths, Ouroboros registry, no-tool retry.

**Positive acceptance.** Nil/empty/failing config exposes and executes zero tools;
an explicit allowlist exposes exactly its live subset; race tests pass.

**Negative acceptance.** Registry membership, stale definitions, or dynamically
generated tools cannot bypass the envelope; removing a tool mid-turn fails safely.

**Rollback.** Preserve fail closed; rollback may disable a broken JIT path but must
use an explicit minimal capability envelope.

## P0: Validate specialist runtime configuration

<!-- NERD_FEATURE
id: session-specialist-config-validation-v1
owner: session
status: verified
kind: truth-gap
depends_on: [session-effective-capability-envelope-v1]
affects: [session, jit, config, shards]
-->

**Value.** A malformed specialist cannot start without identity and constitutional
policy anchoring.

**Evidence and observed gap.** `internal/session/spawner.go#loadSpecialistConfig`
now calls `EffectiveAgentRuntimeConfig.Validate` after strict path/size/YAML
loading and returns a path-qualified error. `spawner_config_test.go` covers blank
identity, missing policies, traversal, maximum size, and valid config.

**Desired behavior.** Every persisted/generated specialist config passes one
versioned validation contract before registry or execution.

**Non-goals.** Do not validate by merely checking file presence, silently fill
identity/policies, or make YAML policy names executable paths.

**Affected contracts.** Specialist filesystem layout, config schema, spawner,
prompt config factory, lifecycle diagnostics.

**Positive acceptance.** Valid configs load; blank identity, empty policy set,
unknown/invalid shape, traversal, and oversize fail before spawn with exact path.

**Negative acceptance.** A failed config creates no registered/running SubAgent;
errors contain no secret config contents.

**Rollback.** Keep the validator and reject specialists while repairing migration;
never return to permissive unvalidated loading.

## P1: Complete protocol-neutral multi-turn tools

<!-- NERD_FEATURE
id: session-protocol-neutral-tool-loop-v1
owner: session
status: verified
kind: leverage
depends_on: [session-effective-capability-envelope-v1]
affects: [session, articulation, perception, tools]
-->

**Value.** Providers using Piggyback tools can observe results and continue with
the same safety, budget, cancellation, and completion behavior as native function
calling.

**Verified 2026-09-25 (commit 799c5a3).** `internal/session/piggyback_channel.go`
(`piggybackChannel`, `Executor.toolResultsChannel`) is the envelope client's
`ToolResultsProvider`: it renders the working request (the round's offered
catalog, the conversation with each call id and result) onto the text channel
and promotes the envelope back to tool calls, so `runToolLoopPass` -- working
ledger, working-policy stops and finalize, forced final answer, every repair
round, planned steps -- is one loop for both protocols. Reused envelope ids are
made loop-unique (`workingLoop.claimCallIDs`). A constitutional-override notice
on a round that goes on to call tools is carried to the response
(`withSafetyNotices`). It also took a wiring fix: the broker, the tracer and the
session adapter did not forward `ShouldUsePiggybackTools`, so in production no
client was ever treated as Piggyback and the CLI engines failed their first
continuation ("does not implement ToolResultsProvider"). Proof:
`TestPiggybackClientSeesItsToolResultsAndContinues`,
`TestPiggybackRepeatedRequestIsEndedByTheWorkingPolicy`,
`TestPiggybackSafetyNoticeOutlivesTheRoundThatRaisedIt` (internal/session),
`TestWrapAnswersThePiggybackQuestionForTheUnderlyingClient` (internal/broker),
`TestSessionAdapterReportsAnEnvelopeOnlyEngineThroughTheProductionChain`
(internal/system), and the integration tests
`TestE2E_PiggybackExecutor_ControlPacket_EndToEnd_HardBoundary` and
`TestE2E_SchedulerSession_Semantic_PiggybackFallback`.

**Evidence and observed gap (historical).** `Executor.runToolLoop` supported native tool result
feedback; the Piggyback route parsed/executed requests but was not a multi-iteration
feedback loop.

**Desired behavior.** Normalize native and Piggyback requests into one typed turn
state machine: resolve capability, exact permission, execute, record bounded
result, feed back via provider adapter, detect repetition/hollow success, stop.

**Non-goals.** Do not emulate unsupported provider calls with untrusted prose,
re-execute completed non-idempotent calls, or hide protocol degradation.

**Affected contracts.** LLM clients, articulation controls, tool loop limits,
timeouts, idempotency, response generation.

**Positive acceptance.** Cross-provider fixtures complete a read-edit-test chain
with identical effect receipts; cancellation, denial, error, timeout, duplicate,
and max-loop paths stop deterministically.

**Negative acceptance.** Tool result text cannot grant a later capability; fallback
surface contains no executable control; retry cannot duplicate an effect.

**Rollback.** Disable Piggyback continuation and return an explicit partial result;
native loop remains unchanged.

## P1: Build one owned session stack

<!-- NERD_FEATURE
id: session-stack-lifecycle-manifest-v1
owner: session
status: proposed
kind: truth-gap
depends_on: [session-specialist-config-validation-v1]
affects: [session, system, campaign, cli]
-->

**Value.** Cortex, campaign, CLI, and tests run the same executor/spawner/kernel/
registry/persistence composition with explicit ownership and teardown.

**Evidence and observed gap.** `internal/system/factory.go` builds the main stack;
campaign command paths also construct session components. Optional persister,
Ouroboros registry, timeouts, budgets, and teardown can drift.

**Desired behavior.** Provide one versioned stack builder that emits a lifecycle
manifest: components, owners, config digests, capability/policy registry versions,
start order, dependencies, and reverse close order.

**Non-goals.** Do not create a service locator, hide dependencies in globals, or
make tests require the whole binary.

**Affected contracts.** System factory, campaign boot, command wiring, test
fixtures, shutdown.

**Positive acceptance.** All production routes use the builder or a typed subset;
parity tests compare manifests; partial boot closes exactly constructed resources.

**Negative acceptance.** Double close and concurrent shutdown are safe; one stack
cannot close shared resources it does not own; defaults cannot drift silently.

**Rollback.** Keep old constructors as manifest-producing adapters until every
consumer migrates, then remove duplication.

## P2: Persist a turn execution receipt

<!-- NERD_FEATURE
id: session-turn-execution-receipt-v1
owner: session
status: proposed
kind: north-star
depends_on: [session-protocol-neutral-tool-loop-v1, session-stack-lifecycle-manifest-v1]
affects: [session, prompt, core, articulation, observability, transparency]
-->

**Value.** An operator can answer why a turn compiled, executed, blocked, retried,
degraded, persisted, or stopped without reconstructing interleaved logs.

**Evidence and observed gap.** Session logs every stage and can persist turns, but
`persistTurn` does not store compiled atom identity and no durable object joins
JIT manifest, capability, exact permission, VirtualStore effect, tool loop,
Piggyback controls, and response.

**Desired behavior.** Emit a redacted versioned receipt with correlation/attempt,
input and manifest digests, capability/policy IDs, each exact decision/effect,
bounded result digest, retry/stop reasons, persistence state, and response digest.

**Non-goals.** Do not store full prompt/input, secrets, hidden reasoning, raw tool
payloads/results, or use observability as authority.

**Affected contracts.** Executor, prompt manifest, kernel decision receipt,
VirtualStore, articulation, SessionPersister, transparency.

**Positive acceptance.** Success, denial, timeout, cancellation, partial failure,
retry, fallback, persistence failure, and restart each yield a schema-valid trace;
operators can traverse perception -> prompt -> permission -> effect -> response.

**Negative acceptance.** Receipt failure cannot execute a tool; retries have
distinct attempt IDs and shared effect idempotency; redaction/retention tests pass.

**Rollback.** Disable durable storage while retaining in-memory correlation and
structured logs; absence is observable.

## P3: Deny-all counterfactual session replay

<!-- NERD_FEATURE
id: session-counterfactual-replay-v1
owner: session
status: deferred
kind: moonshot
depends_on: [session-turn-execution-receipt-v1]
affects: [session, testing, campaign, autopoiesis]
-->

**Value.** Candidate prompts, policies, capabilities, and loop strategies can be
compared on realistic turns before they can affect a workspace.

**Evidence and observed gap.** Task cloning and Dreamer provide isolation slices,
but no receipt-replay environment combines deny-all tools, immutable inputs,
candidate stack manifests, and promotion gates.

**Desired behavior.** Replay redacted receipts through production and candidate
session stacks with disposable kernels/stores and dry-run tools, diff decisions,
budgets, stop reasons, and proposed effects, then require explicit promotion.

**Non-goals.** Never execute real tools, reuse live credentials, auto-promote from
model scores, or treat replay success as production permission.

**Affected contracts.** Receipt schema, stack manifests, Dreamer/sandbox,
campaign evaluation, autopoiesis promotion.

**Positive acceptance.** Replays are deterministic for fixed model fixtures,
bounded, cancellable, side-effect-free, and produce reviewable diffs.

**Negative acceptance.** No live VirtualStore/kernel/persistence handle is present;
candidate panic/timeout is contained; secrets are redacted before capture.

**Rollback.** Disable replay and delete artifacts; live session behavior is
unchanged.

## Wave 2 reconciliation (2026-09-25, verified against the code)

Each open item in this corpus (this file, `03-GAP-ANALYSIS.md`,
`08-WIRING-AND-INTEGRATION.md`) re-checked against the tree; commits are on
the lane-A wave-2 branch.

| Item | Classification | Evidence |
|---|---|---|
| `session-protocol-neutral-tool-loop-v1` | built | commit 799c5a3; card above |
| `session-stack-lifecycle-manifest-v1` | partly stale, rest declined for now | The dual assembly is gone: `session.NewExecutor`/`NewSpawner`/`NewJITExecutor` are constructed only in `internal/system/factory.go` (`initFinalExecutors`); `cmd/nerd/cmd_campaign.go` no longer builds a stack. A versioned lifecycle manifest has no reader yet; building one without a consumer is the unwired-feature pattern. Revisit when teardown ordering is an observed defect. |
| `session-turn-execution-receipt-v1` | open (north star) | Its premise "persistTurn does not store compiled atom identity" is stale: `persistTurn` passes `compilationAtomsJSON(telemetry.compileResult)` to `StoreSessionTurn`, and `recordTurn` carries `AtomIDs`. The joined, redacted, versioned receipt is not built; it depends on a receipt schema decision. |
| `session-counterfactual-replay-v1` | declined | Deferred moonshot; depends on the receipt above. |
| Evicted history is recoverable (lead from wave 1: `Executor.recoverHistoryEviction` unreachable) | built | commit 478bfbd. The window's notice named no way back and the reader had no caller. A working loop now serves the evicted turns behind `recall_context` under an `obs:hist:` handle the notice names (`history_recall.go`); the executor-wide record and its reader are deleted, since the loop holds its own snapshot. `TestHistoryEviction_RecallContextReturnsTheEvictedTurns`. |
| Piggyback memory ops to cold storage (03 P2) | built | commit e0df714. The executor asserted undeclared `memory_operation/3`; it now calls `context.ApplyMemoryOperation` (note -> `session_note`, promotion/vector -> the knowledge store). `forget` no longer calls `Retract(key)` on an arbitrary predicate. `TestPiggybackMemoryOperationsLandInKernelAndStore`, `TestForgetMemoryOp_CannotRetractAPredicateItNames`. |
| `atomsJSON` on `StoreSessionTurn` (03 P2) | already done | `compilationAtomsJSON` in `persistTurn` (`executor.go`). |
| `StoreCompressedState` when a SubAgent compresses (03 P2) | open | `SubAgent` compresses through `SemanticCompressor` and persists nothing; only the chat compressor (`internal/context/compressor_turns.go`) and chat persistence call `StoreCompressedState`. A SubAgent has no persister today; wiring one is a spawner-configuration change with no current reader of subagent compressed state. |
| Production VS implements `InteractiveExecutiveGate` (03 P0.7) | already done | `var _ session.InteractiveExecutiveGate = (*sessionVirtualStoreAdapter)(nil)` (`internal/system/factory_adapters.go`); a missing gate fails closed for every non-read effect (`executeToolCall`: "mandatory executive gate unavailable"). |
| Task-integrity incident (03 §7: shell effects, dirty/untracked work) | already done by fail-closed denial | `checkShellEffect` (`executor_tools.go`) runs `projectdoc.ValidateShellToolInvocation` before any handler and denies mutating or ambiguous shell; `TestExecuteToolCall_ShellEffectGateStopsIncidentBeforeExecution` runs both incident commands (`git checkout -- <tracked>`, a recursive delete) and proves neither reaches the handler. With no accepted shell mutation there is nothing to attribute or refresh; a task-baseline that would *permit* scoped shell mutation is a relaxation for a maintainer to design, not a gap. |
| Canonical prompt-atom precedence (03 P0.6) | out of lane | `internal/prompt` corpus reconciliation. |
| Spawn vs SpawnSpecialist start semantics (03 P3) | already done | Documented contract in `internal/session/README.md` ("`Spawn` constructs/registers; higher-level helpers decide when execution starts"). |
| Replace Wait polling (03 P3) | declined | Conditional item ("if contention becomes hot"); no measured contention. `SubAgent.WaitWithContext` and `JITExecutor.waitObserved` still poll at 100 ms. |
| Package README accuracy (03 P3) | open, not edited here | `internal/session/README.md` still says tool calls are "bounded by call count, loop count"; the loop has no count bound (`runToolLoopPass`: "No count bounds this loop"). Left for the README owner. |
| Ouroboros registry wiring (08 §8) | verified live | `SetOuroborosRegistry` is called on the executor and the spawner by `internal/system/factory.go` (`initFinalExecutors`) and on the chat executor by `cmd/nerd/chat/session_shared_boot.go`; the spawner forwards it to each SubAgent (`spawner.go`). |
