---
doc-class: governance
subsystem: context
implementation-status: accepted-not-implemented
last-verified: 2026-09-21
verified-against: 456e521
supersedes: []
---

# ADR-001: Mangle kernel manages working context; Go keeps mechanics

## Context

`internal/context` computes relevance, retention, eviction, retrieval, and
ordering in Go today. Relevance is a nine-component Go sum in `computeScore`
(`internal/context/activation_scoring.go:39-51` over `scoreComponents`
`internal/context/activation_scoring.go:17-27`); the only kernel path is the
`ScoreFacts` fallback inside `ScoreFactsWithKernelOverride`
(`internal/context/activation_scoring.go:653-698`), which returns the Go-only
`ScoreFacts` result verbatim when the kernel map is empty
(`internal/context/activation_scoring.go:654-656`). Retention is token math in
`recalcBudget` (`internal/context/compressor_turns.go:247-294`) with a
hardcoded safety list in `getCoreFacts`
(`internal/context/compressor.go:745-771`, predicates at `:759`). Retrieval
funnels through `WorkingSet.Select` (`internal/context/working_set.go:308-512`)
with `should_include_context` consulted, not gating. The `note` memory op is
accepted by the protocol schema but Warn-dropped
(`internal/context/compressor_turns.go:226-231`).

This is the Go→kernel drift named in `Docs/architecture/context/01-VISION.md:42-44`.
It serves `agents.md:20` (Mangle manages the active working context) and
`agents.md:47-50` ("clean fixpoint, not clean loop").

## Decision

Move each of the five verbs — relevance, retention, eviction, retrieval,
ordering — from Go heuristics to kernel derivation. Go retains only mechanical
duties: stable fact keys (`factKey`, `internal/context/activation.go:533-535`),
stable sort (`sortScoredFactsDesc`, `internal/context/activation.go:539-546`,
same-sort contract at `internal/context/activation_scoring.go:693-696`), and
token math as the constraint policy reasons within (`TokenBudget`,
`internal/context/tokens.go:184-200`). Specifically:

1. **Relevance:** kernel derives `context_score/2` + `should_include_context/2`;
   `ScoreFactsWithKernelOverride` stays the substitution point (kernel hit takes
   precedence at `:668-673`, Go fallback with component breakdown on miss at
   `:674-690`).
2. **Retention:** a `must_retain/1` derivation replaces the hardcoded safety
   list at `internal/context/compressor.go:759`; reserves stay kernel-gated.
3. **Eviction:** kernel-gated masking/eviction with reason; merges preserve
   coverage and account every shed atom (generalize the
   `assertTurnAgeCategories` / `maskedObservationTurns` exemplar,
   `internal/context/compressor_metrics.go:671-772`).
4. **Retrieval:** the `Select` funnel gates inclusion on
   `should_include_context/2`, joins `Revision`
   (`internal/context/working_set.go:116-134`) before inclusion, and every omit
   carries a ref resolvable through `WorkingStore.Read`
   (`internal/context/working_store.go:219-239`) / `RecallContextTool`.
5. **Ordering:** kernel derives `context_position/2`; Go keeps the stable sort.

Build queue and proving tests: `GAP-CTX-01..07` per
`.nerd/campaigns/aab9612b/artifacts/task_aab9612b_3_0.md`
(`T-REL-01/02`, `T-RET-01/02/03`, `T-RTN-01/02`, `T-EVC-01/02/03`,
`T-ORD-01/02`, plus the `note` round-trip-or-reject).

## Consequences

- Positive: relevance/retention/eviction/retrieval/ordering become inspectable
  kernel rules with machine-checkable exits, instead of Go sums with inferred
  weights; fixes the accept-and-drop contract violation on `note` either by
  implementing it or narrowing the schema.
- Negative: requires core-defaults predicate definitions (call sites live in
  this package, defs live in core defaults); the `working_set.mg` load site in
  compilation scope is still unverified
  (`Docs/architecture/context/WIRING-AND-NOT-BUILT.md:68-73`).
- Risks carried, not resolved here: `ProcessTurn` is exists-but-uncalled
  (zero production callers) so turn-path exits stay uncredited until a live
  driver is traced; kernel-derived facts currently bypass budget selection
  (`buildKernelDerivedContext` "never filters or reorders",
  `internal/context/compressor_metrics.go:549-551`); persistence is best-effort
  with discarded errors (`internal/context/compressor_turns.go:168-181`).

## Witness
**Witness:** test:TestEveryProtocolMemoryOpIsHandled

| Claim | Witness (body-verified 2026-09-21 unless noted) |
|---|---|
| Go-only relevance is live | `ScoreFactsWithKernelOverride` empty-map branch returns `ae.ScoreFacts` (`internal/context/activation_scoring.go:654-656`); `BuildContext` falls back to `GetHighActivationFacts` when kernel yields nothing (`internal/context/compressor.go:707-712`) |
| Kernel-first selection seam exists | `BuildContext` queries `should_include_context` (`internal/context/compressor.go:688`), substitutes via `buildKernelDerivedContext` (`internal/context/compressor.go:695`), records kernel-vs-Go split (`internal/context/compressor.go:713`) |
| Safety retention is a Go literal | predicate list `permitted, dangerous_action, admin_override, security_violation, block_commit` (`internal/context/compressor.go:759`) with Warn+continue (`internal/context/compressor.go:761-766`) |
| `note` is accept-and-drop | `case "note"` Warn-drops (`internal/context/compressor_turns.go:226-231`); coverage pinned by `TestEveryProtocolMemoryOpIsHandled` (`internal/context/memory_op_coverage_test.go:27-62`) |
| Recall path exists (recovery leg) | `WorkingStore.Search` returns handles with `body_chars`, never bodies (`internal/context/working_store.go:106-134`); `WorkingStore.Read` pages bodies by id+offset (`internal/context/working_store.go:219-239`) |
| Funnel shape (outline-verified + durable re-verification artifact) | `Select` at `internal/context/working_set.go:308-512`; control facts, 64-entity two-hop cap, per-entity `working_revision`, `Candidates(ctx,entities,256)`, per-round eviction with `recall_context` ref — see `.nerd/campaigns/aab9612b/artifacts/task_aab9612b_2_0.md` §1 |

## Status (derived, not asserted)

**`accepted-not-implemented`.** Derivation: the direction is accepted (this ADR,
serving the North Star sentences above), but every witness above shows the Go
path live and the kernel path a consulted-not-gating seam — no `must_retain/1`,
`should_evict/2`, or `context_position/2` derivation exists, and `note` still
drops. Status flips to `implemented` only when the gap-matrix exits
(`T-REL`/`T-RET`/`T-RTN`/`T-EVC`/`T-ORD` plus the `note` round-trip-or-reject)
pass; it flips to `superseded` only by a later ADR that names this one.
