# Prompt JIT feature cards

> Sole authoritative `NERD_FEATURE` surface for this corpus. Cards describe
> desired work; none is current behavior unless its status becomes `verified`
> with code, wiring, regression, and a receipt.

## P0: Repair the no-tool retry cache identity

<!-- NERD_FEATURE
id: prompt-no-tool-retry-cache-key-v1
owner: prompt
status: verified
kind: truth-gap
depends_on: []
affects: [prompt, session]
-->

**Value.** Mutation requests that produce planning prose get the intended one-time
tool-call correction instead of receiving the original cached prompt again.

**Evidence and closed gap.**
`internal/session/executor_tools.go#Executor.retryWithNoToolNudge` changes
`PreviousAttemptNoToolCall` and `AvailableTools`. The versioned
`internal/prompt/context.go#CompilationContext.Hash` now includes both and all
other prompt-affecting fields.

**Implemented behavior.** Schema `compilation-context-v2` length-prefixes every
affecting value and canonicalizes set-like tools/frameworks without mutating the
caller. `internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_RetryContextBypassesPreRetryCache`
proves the retry atom and exact current tool surface. Cached-result immutability
remains separate G6 hardening rather than part of this verified collision repair.

**Non-goals.** Do not disable caching globally, hardcode retry prose in Go, or make
tool capability text an authorization gate.

**Affected contracts.** Compilation identity, world-state selection,
`{{available_tools}}` rendering, session hollow-success recovery, cache telemetry.

**Positive acceptance.** `internal/prompt/context_hash_test.go#TestCompilationContext_Hash`
and the production retry regression pass, including distinct retry/tools hashes
and output containing `system/tool_nudge/no_tool_call_retry` behavior.

**Negative acceptance.** Reordering set-like tools/frameworks does not create a
different key after canonicalization; non-affecting opaque pointers do not enter
the key; two identical contexts still singleflight/cache-hit.

**Rollback.** Revert the key schema and tests together, increment/decrement the
schema version deliberately, and clear in-process caches on rollback.

## P0: Isolate Mangle facts per compilation

<!-- NERD_FEATURE
id: prompt-compile-fact-scope-v1
owner: prompt
status: verified
kind: truth-gap
depends_on: []
affects: [prompt, mangle, system, session]
-->

**Value.** Concurrent users, shards, and retries cannot select atoms from one
another's context, and failures leave no prompt-selection residue in the executive
kernel.

**Evidence and closed gap.** `internal/prompt/compiler.go#acquireCompilationKernel`
now acquires a compilation-owned evaluator. Production
`internal/system/factory_adapters.go#KernelAdapter.NewCompilationScope` clones the
primary `RealKernel`; selector facts never enter the live kernel.

**Implemented behavior.** JIT evaluates in a disposable cloned kernel projection.
Deferred scope close discards every ephemeral predicate on success, error,
cancellation, and panic while immutable policy/corpus facts survive in the live
kernel.

**Non-goals.** Do not replace Mangle selection with a parallel Go authority, hold
a global compile lock as the final architecture, or retract another live compile's
facts by predicate name.

**Affected contracts.** Kernel adapter, prompt selector predicates, compiler
concurrency, cancellation, system boot, shard/session isolation.

**Positive acceptance.** `internal/system/prompt_kernel_scope_test.go` runs mixed
language/retry/tool compiles concurrently and covers budget failure, cancellation,
and live-kernel residue; `internal/prompt/compiler_scope_test.go` covers panic.
The focused production gate passes under `-race`.

**Negative acceptance.** Immutable policy and long-lived corpus facts survive;
one compile cannot clean another's in-flight state; the race profile is clean.

**Rollback.** Guard the scoped evaluator behind a temporary rollout flag, retain
old behavior only for comparison, and remove the fallback after parity evidence.

## P1: Make atom authoring and runtime share one strict contract

<!-- NERD_FEATURE
id: prompt-atom-contract-gate-v1
owner: prompt
status: verified
kind: leverage
depends_on: [prompt-compile-fact-scope-v1]
affects: [prompt, init, shards, system]
-->

**Value.** An atom accepted in CI is the atom loaded in production, with the same
selectors and ID count. Authors get precise errors instead of silent selector loss.

**Evidence and closed gap.** `internal/prompt/atom_schema.go#AtomDefinition` and
`ParsePromptAtomYAML` now own filesystem, embedded, synchronization, and validator
decoding. The checked-in validator and runtime routes agree on 333 files and 888
ordered atoms with zero issues or built-in migrations.

**Implemented behavior.** Schema v1 fails whole documents on unknown/invalid data,
uses typed category/world-state vocabulary, validates set-wide uniqueness and
dependencies, and exposes bounded observable legacy migrations expiring
2027-01-01. Built-ins must be canonical.

**Non-goals.** Do not silently accept arbitrary metadata, erase migration history,
or treat YAML validation as proof that selection/wiring is correct.

**Affected contracts.** Built-in atom embed, per-agent sync, prompt builder,
validator CLI, selector vocabulary, init boot.

**Positive acceptance.** `cmd/tools/validate_prompt_atoms#TestCheckedInCorpusOrderedParity`
pins validator/filesystem/embedded order, 888 count, and digest. The CLI passes
with `-fail-on-warn`; strict parser and fail-closed synchronizer regressions pass.

**Negative acceptance.** A typo such as `agent_type` fails before binary build;
deprecated aliases warn for one bounded migration window and then fail; no invalid
file is silently skipped.

**Rollback.** Retain the previous schema reader as an explicit version adapter,
not a permissive fallback, and preserve the last generated corpus artifact.

## P1: Persist a prompt decision receipt

<!-- NERD_FEATURE
id: prompt-decision-receipt-v1
owner: prompt
status: proposed
kind: north-star
depends_on: [prompt-no-tool-retry-cache-key-v1, prompt-compile-fact-scope-v1, prompt-atom-contract-gate-v1]
affects: [prompt, observability, transparency, session]
-->

**Value.** An operator can explain one model turn without exposing prompt secrets:
what context was considered, why each atom was selected/dropped, what budget mode
was used, which capabilities were described, and how the later permission/outcome
related to that decision.

**Evidence and observed gap.** `internal/prompt/manifest.go#PromptManifest` is an
in-memory compiler snapshot. `internal/prompt/compiler.go#JITPromptCompiler.GetLastResult`
holds only the latest result; neither is durably correlated to session, permission,
tool effect, or response.

**Desired behavior.** Emit a versioned, redacted receipt with correlation ID,
context-key schema version, atom ID/version/hash/source, Mangle reasons, conflicts,
dependencies, render modes, budget drops, degradation flags, described tools, and
downstream permission/outcome links. Add inspector “why?” and turn-to-turn diff.

**Non-goals.** Do not persist full user input, atom bodies, secrets, raw tool
payloads, or hidden model reasoning by default. Do not make receipts an approval
mechanism.

**Affected contracts.** Compiler manifest, logging/retention, session correlation,
transparency UI, incident response.

**Positive acceptance.** Every model call has exactly one schema-valid receipt;
success and degraded paths name their reasons; retention/redaction tests prove
secrets and full prompt bodies are absent; an operator can trace receipt ->
permission -> tool result.

**Negative acceptance.** Cache hits still emit a turn receipt identifying the
source compilation; receipt persistence failure cannot authorize or execute an
action; high-cardinality labels do not enter process metrics.

**Rollback.** Disable durable persistence while retaining the existing in-memory
manifest and compilation logs.

## P3: Counterfactual prompt replay lab

<!-- NERD_FEATURE
id: prompt-counterfactual-replay-lab-v1
owner: prompt
status: deferred
kind: moonshot
depends_on: [prompt-decision-receipt-v1]
affects: [prompt, testing, autopoiesis]
-->

**Value.** Maintainers can compare selector, budget, and atom changes against a
redacted historical context set before exposing production turns to them.

**Evidence and observed gap.** Evolved atom loading exists in
`internal/prompt/evolved_atoms.go#EvolvedAtomManager`, but no shadow comparison,
causal rubric, promotion receipt, or side-effect-safe replay contract exists.

**Desired behavior.** Replay immutable redacted compilation contexts through
production and candidate compilers, diff receipts, optionally evaluate model
responses in a no-tools sandbox, and require manual promotion with rollback.

**Non-goals.** Never replay real tools, treat model-judge scores as truth, auto-edit
constitutional atoms, or promote from a single session.

**Affected contracts.** Prompt evolution, testing artifacts, receipt schema,
redaction, promotion governance.

**Positive acceptance.** A fixed corpus produces reproducible receipt diffs and
quality/safety gates without external effects; promotions name evidence and can be
rolled back to a known atom manifest.

**Negative acceptance.** Tool registries are absent/deny-all in replay; secrets are
redacted before persistence; candidate output cannot mutate production atom stores.

**Rollback.** Delete replay outputs and disable the lab; production compilation and
atom promotion remain unchanged.
