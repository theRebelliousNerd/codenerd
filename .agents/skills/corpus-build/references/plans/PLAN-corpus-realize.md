# Plan: corpus-realize — the outer loop (WHAT to build)

**Date**: 2026-03-22 (v1) · **Uplifted in place**: 2026-07-08 (v2)
**Status**: Design-of-record for the realize-mode outer loop (shipped today as Phase -1..2 of the corpus-build skill; kept as a distinct design layer)

## The Problem (unchanged, still true)

Each pipeline solves one piece: **nerd-evolve** optimizes existing code (needs code +
benchmarks); **integration-auditor** finds emergent compositions (doesn't build to spec);
**ralph-codenerd** reconciles docs ↔ code (doesn't implement); **agentic-evolve** improves
agent prompts. None handle: *"Here's an architecture corpus describing what a subsystem
should be. Make it real."*

## What corpus-realize Does

**Input**: a path to an architecture corpus (`Docs/architecture/<subsystem>/`)
**Output**: working, tested, wired code matching the spec — or an informed pivot proposal

### Core Loop

```
Vision Anchor → Concurrency Pre-check → Corpus Ingest (tag-index first) →
Code Audit → Gap Judgment (build/evolve/pivot) → [hand plan to corpus-build] →
Verify Against Spec → Doc Audit (reality reconcile) → offer nerd-evolve
```

corpus-realize is the architect/PM; corpus-build (see PLAN-corpus-build.md v2) is the
construction crew. They remain separable: `realize <subsystem>` runs the full pipeline;
`corpus-build --plan <path>` executes an existing plan.

### Decision Logic (Gap-Size Judgment) — unchanged

| Gap Size | Action |
|----------|--------|
| **Small** (<20% specced features missing) | Build remaining to spec, then evolve |
| **Medium** (20–60%) | Evolve what exists toward spec, build high-priority gaps |
| **Large** (>60%) | Focus on what IS implemented, evolve it, propose pivot if spec unrealistic |
| **Pivot** | Spec intent achievable via a different path than specced |

User feedback steers priority within any gap size. The judgment is produced by
`corpus-judge` from the 3-input score (alignment / structural debt / vision drift) —
see corpus-build Phase 2.

## v2 changes to the outer loop

1. **Ingest is index-first.** The corpus-reader consumes
   `Docs/architecture/roadmap/33_corpus_context_index.json` (features, planes, owner
   docs, source paths) before falling back to prose parsing. Prose parsing is the
   bootstrap path and shrinks as tag coverage climbs — **tag-as-you-go**: every doc the
   reader touches gets frontmatter stamped and `NERD_FEATURE` tags verified as a side
   effect of reading it. No backlog grind, ever.
2. **Concurrency pre-check before any judgment** (adopted from spec-to-code v9.9.1 +
   this repo's own duplicate-build history): per candidate feature, `git log --oneline
   -5 -- <paths>`, existence checks for declared-new files, symbol/gap-id greps for
   modify-targets. Recent commits usually mean already-shipped — the reconciliation
   matrix must be built against HEAD reality, not against the spec's assumption of
   absence. Stale-queue drift is the documented norm in this repo, not the exception.
3. **Virtual subsystems are first-class.** The Classification line of IMPLEMENTED_SPEC
   drives a `source_paths[]` list; every audit greps ALL paths (causal's predicates
   live in `internal/mangle/`, not `internal/causal/`).
4. **The spec-currency direction is now explicit** (was Open Question 2): when the audit
   finds code AHEAD of spec, that is drift to reconcile, not to build — the run routes
   it to `corpus-doc-auditor` (flip IMPLEMENTED_SPEC status + tag planes), never to a
   builder. Most 03-GAP-ANALYSIS deltas in this repo have historically been exactly this
   drift class.

## Open Questions — answered (v1 §Open Questions resolved)

1. **Pivot detection** — resolved by the 3-input judgment: high structural-debt score
   with justified invariant-contradiction counts distinguishes "can't build without
   breaking invariants" (→ REFACTOR/PIVOT) from "not built yet" (→ BUILD). Vision-drift
   >50% forces the PIVOT verdict to a human checkpoint with the judge's justification.
2. **Spec currency** — resolved: code-ahead-of-spec routes to doc-audit (see above);
   docs-ahead-of-code is the normal build input. The doc-auditor is the single writer
   of record for architecture-corpus corrections.
3. **Parallel worker coordination** — resolved by two mechanisms from corpus-build v2:
   interrogation **pins interface contracts** before fan-out (workers author against
   the pinned contract, not their own guesses), and the **reserved-file/intent pattern**
   serializes contested files (registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main), configs, proto) into the wiring phase.
4. **Test quality bar** — resolved: `corpus-critic` gates test RELEVANCE (tests assert
   spec intent, not implementation echo); the test-forge fleet authors to the five-case
   table; coverage is spot-checked per WU and profiled at the serial gate.
5. **Cost estimation** — deleted as a concept. The token ledger measures actual spend
   per run/phase/agent (corpus-build §9); checkpoints cite measured history, never
   estimates. `cost_estimate.py` is removed.

## Relationship to Existing Skills (refs reconciled)

- **NOT a mode of nerd-evolve** — evolve optimizes existing code; this builds from spec.
- **Complements ralph-codenerd** (v1 said "ralph-wiggum") — ralph finds doc-code drift;
  realize fixes the code side, doc-auditor fixes the doc side.
- **Uses nerd-evolve as the optional final phase** — after building, offer evolution
  on the newly-built code (benchmarks now exist from the test phase).
- **Distinct from roadmap-grinder** — grinder consumes feature-CSV rows; realize consumes
  whole-subsystem corpora. Both share the test-forge fleet, fleet-telemetry hooks, and
  the doc-audit reality-reconcile discipline.

## User Interaction Model (unchanged in shape)

```
User: "realize the causal subsystem"
→ ingest + audit → "45% implemented; 12 specced, 5 built, 3 partial, 4 missing.
   Key gaps: X, Y, Z. Build plan: N work units, M workers. [proceed/focus/pivot]"
User: "focus on X and Y"
→ corpus-build executes (contracts pinned, workers fan out, gates run)
→ wiring registry verdicts + codegen gate + doc audit
→ "X: DONE (gate evidence attached). Y: DONE. Subsystem now 65% implemented.
   Ledger: <measured tokens by phase>. Offer to evolve X's scoring?"
```

Three human checkpoints survive every automation pass: gap judgment, post-build review,
final acceptance. Everything between them is fleet + hooks + registries.
