# JIT capability envelope — target architecture

> This document is **PROPOSED UPLIFT** unless a row explicitly says otherwise.
> Current truth remains in [02-CURRENT-STATE.md](02-CURRENT-STATE.md).

## Vision statement

Every dynamic agent receives a validated, versioned capability envelope in which
creative identity is compiled just in time while executable authority, limits,
and recovery behavior are deterministically resolved and observable.

## Strategic goals

### Goal J1 — One valid boundary value

Every producer and loader must cross the session boundary through the same
validation contract. Nil, empty, invalid, and deliberately degraded states must
be distinct types or explicit modes rather than interpretations hidden in
individual consumers. Success means specialist YAML, factory output, cached
output, and fallback output have identical positive and negative invariants. This
closes gaps J1 and J2 without growing orchestration inside `internal/jit`.

### Goal J2 — Fields that mean what they say

Every config field must map to exactly one authoritative runtime consumer and a
negative test. Tool-loop bounds need documented precedence; safety and policy
flags need a real fail-closed gate; model and workspace hints must remain dormant
until their ownership is explicit. Success is a generated parity check that fails
when a field is populated but has no consumer, or when two consumers silently
compete. This addresses gaps J3 and J4.

### Goal J3 — Logic-derived capability identity

The shipped core registry now resolves stable persona set IDs into canonical
embedded policy paths, and JIT validation rejects aliases or missing paths.
Complete the contract by carrying a stable set identity/version into the turn and
defining whether those members are selectively loaded per agent or are evidence
about the already-global corpus. Capability selection may use JIT context and
prompt atoms, but effect authority must remain with declared executive rules and
`permitted/3`. No model-authored text can widen either. This advances the
logic-first north star and closes the residual of gap J3.

### Goal J4 — Explainable degradation and recovery

An operator should be able to distinguish a normal compiled config from baseline
prompt fallback, reduced-budget retry, missing factory, invalid specialist, and a
deliberate read-only degraded mode. A bounded receipt should expose identities,
counts, policy-set/version, limits, and reason codes without retaining prompts,
tool arguments, secrets, or user content. Success is a correlated turn-level
explanation and deterministic recovery test. This addresses gaps J2, J5, and J6.

## Guiding principles

1. **Creative identity is not authority.** Prompt text may shape behavior; only
   deterministic policy and effect gates authorize actions.
2. **Invalid is not degraded.** Validation failure is rejected; degradation is a
   separately named, narrowly capable contract.
3. **No decorative fields.** A populated field without an enforced consumer and
   negative test is removed or remains explicitly proposed.
4. **Policy IDs, not hopeful paths.** The core policy registry resolves stable IDs
   to loaded rule sets; callers do not invent filenames.
5. **Receipts observe but never authorize.** Telemetry may explain a capability
   decision but cannot become mutable executive truth.

## Non-goals

- Moving ConfigFactory, JIT compiler, session loops, or Mangle source into
  `internal/jit/config`.
- Replacing the constitutional safety and postcondition gates with an allowlist.
- Loading arbitrary user-supplied Mangle files because they appear in YAML.
- Capturing full identity prompts, user inputs, or tool payloads in receipts.
- Activating per-agent model/workspace overrides without explicit precedence and
  containment designs.

## Success metrics

| Metric | Current | Target | Measurement |
|---|---|---|---|
| Production config boundaries that call validation | specialist YAML enforces it; generated/fallback paths do not uniformly enforce it | 100% | generated caller inventory plus negative integration tests |
| Populated schema fields with a named runtime consumer and negative test | `AllowedTools` partial; several fields have no hot-path consumer | 100% or field removed | schema-to-consumer parity test |
| Default policy references resolving to the embedded boot inventory | 100% for both default providers | 100% | `TestDefaultAgentPolicySetsResolveToEmbeddedPolicyInventory` plus provider parity |
| Turns with explicit policy-set identity/version and selective/global semantics | no turn-level identity; canonical member paths refer to the global corpus | 100% | session integration and capability receipt |
| Nil/empty capability paths that can request an unlisted tool | 0 in the verified session execution and Piggyback catalog paths | 0 | modular and Ouroboros negative execution tests |
| Degraded turns with a machine-readable reason | no stable envelope receipt | 100% | receipt schema and turn-level integration test |
| Corpus claim lanes with evidence or bounded N-A | nine documented here and in README | nine | strict corpus validation plus semantic audit |

## Relationship to codeNERD

The capability envelope operationalizes the creative-center/executive split at
the agent boundary. The model receives enough identity and context to solve the
problem creatively; Mangle and the effect pipeline determine which capabilities
actually exist and which actions are permitted.

This design also protects JIT-first evolution. A new specialist should be prompt
atoms, policy-set membership, and typed capability data—not another hard-coded
runtime class. The small schema remains stable while the declarative ecosystem
evolves around it.

## Uplift roadmap

### Phase 1 — Close the boundary (J1, J2)

Keep the verified specialist validation and nil/empty modular/Ouroboros
fail-closed gates; add uniform factory/fallback validation and define an explicit
degraded mode rather than a zero-value convention.

### Phase 2 — Make the schema honest (J3, J4)

Preserve the verified canonical policy-set registry, map every field to its owner,
pin selective-versus-global policy semantics, choose tool-loop precedence, and
remove or defer fields that do not yet alter runtime.

### Phase 3 — Explain operation (J5, J6)

Create a redacted effective-capability receipt with config and policy versions,
grant counts, enforced limits, fallback reason, and correlation identity. Add
bounded metrics and restart/retry tests.

### Phase 4 — Compare safely (J7)

Only after receipts and fail-closed boundaries are proven, run candidate
capability envelopes in a no-effect shadow and report allow/deny and token/quality
deltas without applying effects.

## North-star alignment matrix

| Goal | Supporting gaps | Roadmap phase | Priority |
|---|---|---|---|
| J1 — One valid boundary value | J1, J2 | Phase 1 | Critical |
| J2 — Fields that mean what they say | J3, J4 | Phase 2 | High |
| J3 — Logic-derived capability identity | J3 | Phase 2 | Critical |
| J4 — Explainable degradation and recovery | J2, J5, J6 | Phase 3 | High |

J7 is deliberately not a standalone goal: it is a deferred experiment enabled by
the four operational goals, not a reason to postpone boundary repair.
