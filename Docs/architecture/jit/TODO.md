# jit — architecture uplifts

> Sole authority for JIT `NERD_FEATURE` cards. Current truth remains in
> [02-CURRENT-STATE.md](02-CURRENT-STATE.md).

## Truth-gap repair

<!-- NERD_FEATURE
id: jit-validated-capability-envelope-v1
owner: jit
status: in_progress
kind: truth-gap
depends_on: []
affects: [session, prompt, tools, testing]
-->

### Validate and fail close every capability envelope

- **Value:** an invalid or missing agent config cannot silently become an
  unrestricted execution capability.
- **Current evidence:** specialist YAML now invokes
  `internal/jit/config/types.go#Validate`; nil/empty/unlisted capability checks
  deny across modular and Ouroboros execution and catalog paths. See
  `internal/session/spawner_config_test.go#TestLoadSpecialistConfigRejectsInvalidRuntimeConfig`
  and `internal/session/executor_capability_test.go#TestExecutorOuroborosRegistryDoesNotGrantCapability`.
- **Verified slices (2026-07-13):** path-qualified specialist validation, hostile
  path/size preservation, deny-all nil/empty lists, modular/Ouroboros allowlist
  intersection, Piggyback catalog filtering, and focused race coverage.
- **Observed residual:** factory/generated fallback configs do not uniformly
  validate, and zero-value degradation has no typed reason or explicit mode.
- **Desired behavior:** every boundary validates; invalid values fail; a separately
  typed degraded mode grants only an explicit bounded read-only set; every tool
  registry is intersected with the effective grants.
- **Non-goals:** weakening constitutional permission, removing compiler fallback,
  or forcing tools onto pure-reasoning turns.
- **Affected contracts:** specialist YAML, ConfigFactory, compiler fallback,
  executor injection, modular/Ouroboros tool routing, recovery errors.
- **Positive acceptance:** valid specialist configs are accepted, and separately
  listed modular/Ouroboros capabilities cross their handler gates (**verified**);
  one loader-to-effect integration oracle, valid factory configs, and any
  explicit read-only degradation with a typed reason remain **open**.
- **Negative acceptance:** blank specialist identity, missing specialist policies,
  nil/empty normal mode, and an unlisted modular or Ouroboros tool are rejected
  before handler execution (**verified**); equivalent generated/fallback config
  validation remains open.
- **Rollback:** revert boundary validation and allowlist intersection together
  with their focused tests; retain the old behavior only behind a named temporary
  compatibility flag that logs every use.

## Safe leverage uplift

<!-- NERD_FEATURE
id: jit-schema-consumer-policy-parity-v1
owner: jit
status: in_progress
kind: leverage
depends_on: [jit-validated-capability-envelope-v1]
affects: [prompt, mangle, session, verification]
-->

### Generate schema-consumer and policy-set parity

- **Value:** every field and policy reference either changes a proven runtime gate
  or is visibly rejected as decorative configuration.
- **Current evidence:** `internal/prompt/config_factory.go#Generate` populates
  `ToolLoop` and `Safety`; `internal/session/executor_tools.go#runToolLoop` uses
  `ExecutorConfig`. `internal/core/policy_inventory.go` now owns stable set IDs,
  canonical boot-inventory paths, and resolution; both prompt providers share it,
  and JIT validation rejects noncanonical or duplicate references.
- **Observed gap:** reviewers must manually compare schema, factories, consumers,
  and tests for non-policy fields. Policy paths do not carry set identity/version
  into a turn or alter a selectively loaded per-agent kernel.
- **Verified slice (2026-07-13):** stable default set IDs map only to embedded
  boot-corpus members; kernel root-module inventory is shared; both default
  providers have parity; aliases, traversal, missing, whitespace, and duplicate
  policy references fail validation.
- **Desired behavior:** preserve the registry, map every field to owner,
  precedence, and negative test, and carry explicit policy-set identity/version
  with pinned global-versus-selective semantics.
- **Non-goals:** generating Mangle semantics, loading arbitrary YAML paths, or
  activating Model/Workspace merely to satisfy the parity check.
- **Affected contracts:** config schema, prompt config atoms, core policy embed,
  executor limits, safety flags, docs and generated verification.
- **Positive acceptance:** every default policy-set ID resolves to loaded module
  members (**verified**); every populated field has exactly one authoritative
  consumer and set identity reaches a tested turn (**open**).
- **Negative acceptance:** missing, aliased, traversal-shaped, whitespace, and
  duplicate policy references fail (**verified**); a seeded field with no
  consumer or duplicate owner must fail the future parity gate (**open**).
- **Rollback:** keep the registry and tests read-only first; if generation is too
  rigid, retain the last parity report and remove only generated projections.

## North-star uplift

<!-- NERD_FEATURE
id: jit-effective-capability-receipt-v1
owner: jit
status: proposed
kind: north-star
depends_on: [jit-schema-consumer-policy-parity-v1]
affects: [session, observability, transparency, recovery]
-->

### Emit a redacted effective-capability receipt

- **Value:** an operator can prove which identity/config version, policy set,
  grants, bounds, and fallback state governed a turn.
- **Current evidence:** `internal/prompt/compiler.go#CompilationStats` records
  compilation metrics and `internal/session/executor.go#ProcessWithIntent` owns
  the turn, but no stable envelope correlates config production to effects.
- **Observed gap:** JIT and session logs cannot distinguish all producer,
  validation, degradation, and enforcement outcomes without reconstruction.
- **Desired behavior:** emit immutable IDs, counts, resolved policy-set version,
  active limits, reason codes, and enforcement results with bounded retention.
- **Non-goals:** storing prompt bodies, user content, file paths, tool arguments,
  secrets, or using the receipt as permission truth.
- **Affected contracts:** compilation manifest, session/request correlation,
  capability validation, effect audit, redaction, retention, restart behavior.
- **Positive acceptance:** an integration test correlates compiled and degraded
  turns through effect allow/deny outcomes while seeded secrets are absent.
- **Negative acceptance:** an unversioned policy, missing fallback reason,
  unbounded field, secret-bearing receipt, or receipt-authorized effect fails.
- **Rollback:** disable receipt emission and preserve existing JIT/session logs;
  config validation and capability gates remain active.

## Bounded moonshot

<!-- NERD_FEATURE
id: jit-capability-shadow-lab-v1
owner: jit
status: deferred
kind: moonshot
depends_on: [jit-effective-capability-receipt-v1]
affects: [campaign, verification, observability]
-->

### Compare candidate capability envelopes in a no-effect shadow

- **Value:** maintainers can measure whether a proposed persona/tool/policy mix
  improves task utility or denial quality before it can touch a workspace.
- **Current evidence:** compilation contexts and manifests already identify atom
  selection, while Dreamer supplies a separate core pattern for no-effect
  projection; no capability-level comparator exists.
- **Observed gap:** capability changes are evaluated through live or handcrafted
  tests, without representative decision deltas or resource budgets.
- **Desired behavior:** replay normalized, redacted receipt inputs through a
  candidate envelope with every effect adapter disabled; compare grants, denial
  reasons, token use, and task-quality proxies.
- **Non-goals:** replaying raw prompts, automatic rollout, model self-authorization,
  live shadow effects, or retaining user artifacts.
- **Affected contracts:** receipt normalization, snapshot identity, effect
  isolation, nondeterminism labels, campaign sampling, storage bounds.
- **Positive acceptance:** a seeded candidate produces expected decision and
  budget deltas with zero registry/VirtualStore effect calls.
- **Negative acceptance:** missing redaction, any outbound effect, unbounded
  sample retention, or automatic policy adoption rejects the experiment.
- **Rollback:** delete shadow artifacts and disable the lab; production config,
  policy, and effect paths remain unchanged.
