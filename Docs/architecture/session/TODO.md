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
status: proposed
kind: leverage
depends_on: [session-effective-capability-envelope-v1]
affects: [session, articulation, perception, tools]
-->

**Value.** Providers using Piggyback tools can observe results and continue with
the same safety, budget, cancellation, and completion behavior as native function
calling.

**Evidence and observed gap.** `Executor.runToolLoop` supports native tool result
feedback; the Piggyback route parses/executes requests but is not a fully symmetric
multi-iteration feedback loop.

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
