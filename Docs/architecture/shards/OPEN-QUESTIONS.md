# Open questions: shards

These choices are not implementation claims. Each names the consequence of
leaving the decision open.

## Q1. Where does the single descriptor live?

Should the typed shard/domain descriptor live in `internal/shards`, a neutral
types package, or core? `internal/shards` owns factories but imports core manager
types; core owns KernelShard construction and must not import concrete system
shards. The choice determines whether production consumes data, callbacks, or a
generated artifact.

## Q2. What are observer overflow and snapshot semantics?

Restartability is now pinned and tested with fresh run contexts. The remaining
choice is whether a full event channel drops, coalesces, backpressures, or
persists events, and whether last-assessment access returns a defensive copy.
Without this decision, absence of an assessment is still ambiguous.

## Q3. What is a batch consultation result?

Should partial success return responses plus a typed per-target error set, or an
aggregate error with partial responses? The answer must preserve useful dissent
without letting total failure become success.

## Q4. Which system shards are required for an effect generation?

Constitution and the actual effect validator are clearly required. Is tactile
router required only for Mangle-derived actions, while session tool calls use a
separate VirtualStore path? A readiness design cannot be correct until these
effect classes are pinned.

## Q5. Which perception path owns interactive intent?

Shared transducer and perception firewall can both participate. Pin whether the
firewall is the authoritative ingress, an asynchronous verifier, or a degraded
fallback so one user turn cannot create duplicate current intents.

## Q6. How should runtime enrichers compose?

Should browser, ToolStore, Glass Box, learning candidates, and campaign manager
use descriptor-declared optional dependencies, post-spawn hooks, or explicit
environment decorators? The choice must keep base factory parity inspectable.

## Q7. Is a Northstar block advisory or executive?

Observer AssessmentLevel includes block, but observers do not own constitutional
authority. If a block can halt work, it needs a declared fact, producer trust
boundary, appeal, expiry, and Mangle rule. Otherwise UI must label it advisory.

## Q8. What becomes durable?

Learning patterns, consultation responses, assessments, and lifecycle receipts
have different sensitivity and value. Pin retention, redaction, size, workspace
scope, versioning, and deletion before adding unified persistence.

## Q9. How does semantic specialist retrieval stay subordinate to logic?

Embeddings may nominate experts, but readiness, execution mode, knowledge tier,
budget, permissions, and required evidence need typed gates. Pin the fallback and
evaluation dataset before replacing deterministic heuristics.

## Q10. Which operation outcome becomes the shared lifecycle receipt?

Consultations now preserve partial errors, observer generations restart, and
unmapped routes terminate exactly once. Should a common receipt envelope wrap
these existing APIs, or should each subsystem expose its own typed result plus a
normalized observability adapter? The choice affects versioning and compatibility.
