# core — Architecture Uplifts

> Sole authority for core `NERD_FEATURE` cards. Claim truth remains in
> [02-CURRENT-STATE.md](02-CURRENT-STATE.md); target rationale lives in
> [01-VISION.md](01-VISION.md).

## Verified truth-gap repair

<!-- NERD_FEATURE
id: core-sandbox-counterfactuals-v1
owner: core
status: verified
kind: truth-gap
depends_on: []
affects: [mangle, transparency]
-->

### Keep Dreamer counterfactuals out of executive truth

- **Value:** a safety check cannot change later executive decisions merely by
  asking “what if?”.
- **Current evidence:** `internal/core/dreamer.go#projectEffects` creates the
  `hypothetical/1` fact in the projected set;
  `internal/core/dreamer_test.go#TestDreamer_SimulateAction_DoesNotMutateLiveHypotheticals`
  proves the live EDB remains unchanged.
- **Observed gap:** the earlier call to `RealKernel.Assert` persisted each
  counterfactual before sandbox evaluation.
- **Desired behavior:** counterfactual facts exist only in the cloned evaluation.
- **Non-goals:** redesigning Dreamer projections or policy semantics.
- **Affected contracts:** Mangle `hypothetical/1`, Dreamer state isolation,
  transparency of safety evidence.
- **Positive acceptance:** the projected facts contain `hypothetical/1` and the
  checked sandbox accepts valid projections.
- **Negative acceptance:** the live kernel's `hypothetical/1` count must not grow;
  invalid or over-limit projected facts must return unsafe, and destructive routes
  without a Dreamer must deny.
- **Rollback:** disable/reject destructive simulation while investigating. Never
  restore live counterfactual mutation or convert staging failure into allow.

<!-- NERD_FEATURE
id: core-executive-envelope-hardening-v1
owner: core
status: verified
kind: truth-gap
depends_on: [core-sandbox-counterfactuals-v1]
affects: [mangle, system, shards, testing, transparency]
-->

### Preserve one exact executive safety envelope

- **Value:** authorization, execution, validation, and telemetry describe the
  same action instead of drifting across arity, shard, ID, or timestamp seams.
- **Current evidence:** `TestPermissionCacheIsClassificationOnly`,
  `TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard`,
  `TestRouteActionFailsClosedWithoutKernel`,
  `TestRouteActionFailureFactsMatchDeclaredContracts`,
  `TestMaybePruneActionLogsUsesExecutionResultTimestamp`,
  `TestRouteActionReturnsPostValidationFailure`, and
  `TestPendingActionPipelineProducesRoutingResult` pass.
- **Observed gap:** cached classification could bypass exact permission; Cortex
  split the join; missing gates could fail open; failure facts, prune timestamp,
  and action correlation disagreed with their declarations.
- **Desired behavior:** exact canonical `permitted/3`, policy-shard ownership,
  fail-closed missing dependencies, declared failure shapes, executive action ID
  correlation, and validation failure propagation.
- **Non-goals:** eliminating direct `Exec`, making Cortex mandatory, or changing
  the public action vocabulary.
- **Affected contracts:** `pending_action/3`, `safe_action/1`, `permitted/3`,
  `security_violation/3`, `execution_error/2`, `execution_result/6`.
- **Positive acceptance:** a matching target and canonical payload permits and the
  original action ID reaches the result fact.
- **Negative acceptance:** nil kernel/Dreamer, target or payload mismatch, invalid
  postcondition, or malformed projection denies with schema-valid evidence.
- **Rollback:** revert only to the last exact fail-closed envelope. If routing or
  policy ownership cannot be restored safely, disable the affected action class;
  never reinstate cache-only allow or nil-dependency allow.

## Safe leverage uplift

<!-- NERD_FEATURE
id: core-action-contract-registry-v1
owner: core
status: proposed
kind: leverage
depends_on: []
affects: [mangle, testing, tactile]
-->

### Generate one action-contract registry

- **Value:** adding an action cannot silently omit a handler, destructive
  classification, policy classification, permission test, or observability name.
- **Current evidence:** action identity is defined in
  `internal/core/virtual_store_types.go#ActionType`, destructive routing in
  `internal/core/virtual_store_constitution.go#isDestructiveAction`, dispatch in
  `internal/core/virtual_store_routing.go#RouteAction`, and policy in
  `internal/core/defaults/policy/constitution.mg#permitted/3`.
- **Observed gap:** the same contract is maintained across Go switches and Mangle
  facts without one parity proof.
- **Desired behavior:** one declarative registry generates or verifies all
  required projections and fails tests on an unclassified action.
- **Non-goals:** moving constitutional decisions out of Mangle or making unknown
  actions safe by default.
- **Affected contracts:** action enum, handler dispatch, `safe_action/1`,
  `dangerous_action/1`, Dreamer projection, audit events, generated tests.
- **Positive acceptance:** every registered action maps to exactly one handler and
  required policy/safety/telemetry attributes.
- **Negative acceptance:** a seeded new action with no destructive classification
  or handler makes the parity test fail.
- **Rollback:** keep generated checks read-only first; remove generation while
  retaining the last verified parity test if schema ergonomics are poor.

## North-star uplift

<!-- NERD_FEATURE
id: core-executive-decision-receipt-v1
owner: core
status: proposed
kind: north-star
depends_on: [core-action-contract-registry-v1]
affects: [session, transparency, verification, observability]
-->

### Emit a bounded executive decision receipt

- **Value:** an operator can answer which intent, rule, permission, and effect
  produced an outcome without reconstructing it from loose logs.
- **Current evidence:** `internal/core/virtual_store_routing.go#RouteAction` has
  separate route, permission, execution, validation, fact-injection, and audit
  steps; `internal/core/kernel_provenance.go#Explain` can provide optional proof
  data.
- **Observed gap:** those signals lack one stable, redacted correlation envelope.
- **Desired behavior:** emit an immutable bounded receipt containing identities
  and evidence references, not raw secrets or mutable executive truth.
- **Non-goals:** deterministic replay, full prompt retention, or making receipts
  an authorization source.
- **Affected contracts:** session/request identity, permission reason, effect and
  verification result, redaction, retention, telemetry loss indication.
- **Positive acceptance:** an allowlisted end-to-end test correlates intent through
  permission and result while sensitive payload values are absent.
- **Negative acceptance:** a receipt containing a seeded secret, lacking a deny
  reason, or authorizing an effect fails validation.
- **Rollback:** disable receipt emission behind configuration while retaining
  existing audit signals and schema compatibility.

## Bounded moonshot

<!-- NERD_FEATURE
id: core-shadow-policy-comparison-v1
owner: core
status: deferred
kind: moonshot
depends_on: [core-executive-decision-receipt-v1]
affects: [campaign, verification, transparency]
-->

### Compare candidate policy in a no-effect shadow

- **Value:** policy authors can see how a proposed constitution would change
  decisions before it can touch a real effect.
- **Current evidence:** `internal/core/dreamer.go#evaluateProjection` already clones
  the kernel, and `internal/core/shadow_mode.go#ShadowMode` models simulations.
- **Observed gap:** neither is a redacted mission-level policy comparison contract.
- **Desired behavior:** replay normalized receipts through a candidate policy with
  all effect adapters disabled and report decision deltas.
- **Non-goals:** automatic policy adoption, live shadow effects, or replaying raw
  prompts and secrets.
- **Affected contracts:** snapshot identity, nondeterminism markers, redaction,
  effect isolation, result comparison, resource limits.
- **Positive acceptance:** a seeded policy change produces the expected decision
  delta with zero effect-adapter calls.
- **Negative acceptance:** any outbound effect, unbounded retention, or replay of
  a redacted field rejects the experiment.
- **Rollback:** delete shadow artifacts and disable the evaluator; runtime decision
  and effect paths remain unchanged.
