# Perception feature cards

> Sole authoritative `NERD_FEATURE` surface for this corpus. A proposed card is
> not current behavior; `verified` requires code, wiring, regression, and receipt.

## P0: Fail closed on absent provider configuration

<!-- NERD_FEATURE
id: perception-nil-provider-config-v1
owner: perception
status: verified
kind: truth-gap
depends_on: []
affects: [perception, config, session]
-->

**Value.** A missing construction input produces an actionable boot error instead
of a nil-pointer panic before the agent can explain the configuration problem.

**Evidence and observed gap.** `internal/perception/client_factory.go#NewClientFromConfig`
previously dereferenced `config.Engine` unconditionally. The function now rejects
nil, and `internal/perception/client_factory_test.go#TestNewClientFromConfig_NilConfig`
proves both the nil client and explicit error.

**Desired behavior.** All client factory entrypoints fail explicitly for absent
required configuration and preserve nil-as-optional only where documented, such
as the classification-client fallback.

**Non-goals.** Do not invent defaults, consult ambient credentials after an
explicit config failure, or convert optional worker/classification clients into
required clients.

**Affected contracts.** System boot, config error reporting, provider factory,
session construction.

**Positive acceptance.** Nil configuration returns `(nil, error)` without panic;
the full perception package passes.

**Negative acceptance.** Valid API and subscription-engine configs preserve their
existing constructors; `NewClassificationClientFromConfig(nil)` remains its
documented optional `(nil, nil)` result.

**Rollback.** Revert the guard and regression together only if every caller gains
a stronger typed non-nil constructor contract.

## P0: Normalize provider failure semantics

<!-- NERD_FEATURE
id: perception-provider-failure-contract-v1
owner: perception
status: proposed
kind: truth-gap
depends_on: []
affects: [perception, session, observability]
-->

**Value.** A user sees the same truthful degraded response whether the active
model is Gemini, Anthropic, OpenAI-compatible, a CLI engine, or OAuth-backed.

**Evidence and observed gap.** `internal/perception/client_gemini.go#ErrLLMUnavailable`
and `internal/perception/understanding_adapter.go#UnderstandingTransducer.ParseIntentWithContext`
carry transient identity through the nil-error adapter contract. Equivalent typed
exhaustion behavior is not proven across every provider.

**Desired behavior.** Define typed failure classes for transient service failure,
rate limit, authentication, invalid request, cancellation, and permanent provider
failure. Require every adapter and retry loop to preserve the class and safe,
redacted operator detail.

**Non-goals.** Do not retry authentication or invalid-request failures, expose raw
provider bodies/secrets, or make perception responsible for session retry policy.

**Affected contracts.** Client interfaces, retry helpers, transient intent flag,
session firewall, telemetry.

**Positive acceptance.** A shared conformance suite drives representative 429,
401, 400, 500/503, timeout, and cancellation responses through every adapter and
asserts the same error taxonomy and retryability.

**Negative acceptance.** Context cancellation returns promptly; non-idempotent
tool calls are not retried by classification transport; redaction tests reject
keys/tokens/provider bodies in surfaced messages.

**Rollback.** Retain provider-specific errors behind an adapter while consumers
migrate; remove the compatibility layer only after all session paths use typed
classification.

## P1: Publish a typed capability and ownership receipt

<!-- NERD_FEATURE
id: perception-capability-ownership-receipt-v1
owner: perception
status: proposed
kind: leverage
depends_on: [perception-provider-failure-contract-v1]
affects: [perception, prompt, jit, session, system]
-->

**Value.** Boot and JIT selection know exactly which model can stream, call tools,
enforce schemas, think, upload files, or classify cheaply, and which workspace
owns its caches/taxonomy/learning state.

**Evidence and observed gap.** `internal/perception/client_types.go#LLMClient` is
deliberately small while concrete clients expose optional methods. Consumers use
type assertions; `SharedTaxonomy` and `SharedSemanticClassifier` are process-wide.

**Desired behavior.** At construction, emit an immutable versioned receipt with
provider/engine identity, supported capabilities, bounded model limits, semantic
state owner, lifecycle hooks, and degradation reasons. JIT may describe these
capabilities; session policy still authorizes actual calls.

**Non-goals.** A capability is not `permitted/3`; do not centralize provider HTTP
implementation or expose API keys/model secrets in the receipt.

**Affected contracts.** Provider factory, prompt context, system boot, workspace
lifecycle, tracing, session tool resolution.

**Positive acceptance.** Every constructor's receipt matches runnable behavior;
boot rejects contradictory claims; concurrent workspace tests prove taxonomy and
semantic state isolation and deterministic teardown.

**Negative acceptance.** A client cannot claim tools or schema support without a
conformance test; receipt mutation after construction is impossible; teardown of
one workspace cannot close another's stores.

**Rollback.** Keep optional interface assertions as a compatibility adapter until
receipt parity is proven, then remove the duplicate discovery path.

## P2: Persist a perception decision receipt

<!-- NERD_FEATURE
id: perception-decision-receipt-v1
owner: perception
status: proposed
kind: north-star
depends_on: [perception-provider-failure-contract-v1, perception-capability-ownership-receipt-v1]
affects: [perception, prompt, core, observability, transparency]
-->

**Value.** An operator can answer “why did codeNERD interpret this turn as a
mutation?” without storing hidden reasoning or trusting an uncorrelated log line.

**Evidence and observed gap.** `internal/perception/tracing_client.go#ReasoningTrace`
and process metrics observe model calls, while `Understanding`, semantic matches,
routing, and final `Intent` exist in memory. No durable receipt joins those stages
to the downstream permission and outcome.

**Desired behavior.** Emit a redacted, versioned receipt containing correlation
ID, input digest, classifier/model/capability receipt IDs, semantic candidate IDs
and scores, normalized Understanding fields, routing derivation provenance,
degradation class, final Intent/fact digest, and later permission/outcome links.

**Non-goals.** Do not persist full prompts, user secrets, chain-of-thought, raw
provider responses, or use the receipt as authorization.

**Affected contracts.** Perception tracing, prompt manifest, kernel decision
receipts, session correlation, transparency storage and retention.

**Positive acceptance.** Every turn produces exactly one schema-valid receipt or
an explicit receipt-write failure signal; operators can trace perception -> prompt
-> permission -> effect -> response; redaction and retention tests pass.

**Negative acceptance.** Persistence failure cannot grant or execute an action;
high-cardinality fields do not enter metrics; identical digests do not deduplicate
distinct turns.

**Rollback.** Disable durable storage while retaining ephemeral correlation and
existing tracing; receipt absence remains observable.

## P3: Side-effect-free shadow transduction lab

<!-- NERD_FEATURE
id: perception-shadow-transduction-lab-v1
owner: perception
status: deferred
kind: moonshot
depends_on: [perception-decision-receipt-v1]
affects: [perception, testing, autopoiesis]
-->

**Value.** Maintainers can compare prompts, models, taxonomy rules, and semantic
corpora on realistic redacted turns before changing live routing.

**Evidence and observed gap.** Benchmarks, torture tests, and gated live-provider
tests exist, but there is no receipt-based shadow comparison or promotion gate.

**Desired behavior.** Replay immutable redacted perception inputs through current
and candidate classifiers, diff decision receipts, score contract adherence and
human-reviewed outcomes, and require explicit promotion/rollback evidence.

**Non-goals.** Never execute tools, replay secrets, auto-promote from a model judge,
or allow a shadow result to assert live `user_intent` facts.

**Affected contracts.** Test fixtures, receipt schema, provider budgets, taxonomy
promotion, autopoiesis governance.

**Positive acceptance.** Repeated runs produce deterministic structural diffs;
the candidate cannot reach live kernels or stores; promotion cites a fixed corpus
and negative safety cases.

**Negative acceptance.** Network use is explicitly budgeted and optional; shadow
failures do not alter production routing; redaction runs before persistence.

**Rollback.** Delete lab artifacts and disable shadow dispatch; production
transduction remains unchanged.
