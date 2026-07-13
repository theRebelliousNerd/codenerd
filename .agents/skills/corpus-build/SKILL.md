---
name: corpus-build
description: >
  Spec-driven implementation engine for codeNERD. Reads an architecture corpus
  (Docs/architecture/<subsystem>/ or Docs/Spec/...), audits code against the spec,
  judges the gap (build/evolve/pivot), and dispatches a specialist fleet with
  dependency-aware DAG ordering, surface-registry wiring checks, serial compile/test
  gates, and auditable skips. Use for "realize", "build from spec", "corpus-build",
  "implement the architecture", "make real", "spec to code". Do NOT use for routine
  bug fixes, writing architecture docs (arch-propose / spec-doc-sprint), pure evolution
  (nerd-evolve), or doc-only drift audits (integration-auditor).
metadata:
  version: 1.0.0
  author: codeNERD (ported from Vectryx corpus-build)
  last-verified: 2026-07-13
  spawned-agents:
    - corpus-reader
    - corpus-judge
    - corpus-builder
    - corpus-critic
    - corpus-wiring-auditor
    - corpus-doc-auditor
---

# Corpus Build — Spec → Code (codeNERD)

Spec-driven implementation. Reads architecture corpus, audits live code, produces a
gap-classified build plan, dispatches specialists in DAG order.

> **arch-propose decides WHAT to design. corpus-build is HOW it gets built.**

**Pipeline**: arch-propose / existing corpus → corpus-build → tests green → optional
spec-doc-sprint reconcile → nerd-evolve

## DELEGATION MANDATE

Orchestrator only: run state, phase transitions, serial gates (build/test), ledger,
checkpoints, commits. Workers author code; orchestrator verifies. Prefer
`spawn_subagent` with `isolation: worktree` for builders. Workers must not spawn
further subagents.

## Mode Selection

| User phrasing | Mode |
|---------------|------|
| realize X / build from spec X | Full pipeline Phase -1 → end |
| corpus-build --plan path | Load plan; start Phase 3 |
| Ambiguous "implement the spec" | Ask which corpus path |

## Phase -1: Vision Anchor

Inject codeNERD north star into every worker prompt:

- LLM = creative center; Mangle kernel = executive
- Fact flow: user_intent → next_action → VirtualStore → articulation
- Constitutional safety: `permitted(...)`; default deny
- JIT-first for new LLM-facing behavior
- Cached summary: `references/vision-summary.md` (regenerate from root AGENTS.md if stale)

## Phase 0: Initialization

0.1 Prefer `Docs/architecture/<subsystem>/IMPLEMENTED_SPEC.md`; else
    `Docs/Spec/internal/<subsystem>/` north-star + gap-analysis as corpus surrogate.
0.2 Check `.corpus-build/plans/` for existing plans.
0.3 Ensure dirs: `.corpus-build/{plans,results,matrices,manifests,intents,journal,contracts,reviews,slices/current,ledger}/`
0.4 Detect multi-path / virtual subsystem from corpus `source_paths[]`.
0.5 Write run state `.corpus-build/ledger/<session_id>.active` with phase transitions.
0.6 Confirm fleet agents under `.grok/agents/corpus-*.md`.

## Phase 0.5: Concurrency Pre-Check

Per work unit: `git log` / existence tests / symbol greps. Evidence of parallel land →
STOP and reconcile with user (never rebuild shipped rows blindly).

## Phase 1: Corpus Ingest + Code Audit

Dispatch **corpus-reader**:

1. Parse corpus → feature manifest
2. Grep all `source_paths[]` → reconciliation matrix
3. Anti-hallucination: verify every extracted symbol exists or mark UNVERIFIED

Outputs: `.corpus-build/manifests/`, `.corpus-build/matrices/`.

## Phase 1.5: Interrogate + Pin Contracts

Pin shared interfaces to `.corpus-build/contracts/<subsystem>.md`. Skip only for
purely-additive single-WU runs via `scripts/record_skip.py`.

## Phase 2: Gap Judgment

Dispatch **corpus-judge**. Classifications: NONE, PARTIAL, MISSING, UNWIRED, DIVERGENT.

Decision guidance:

| Alignment | Structural debt | Vision drift | Action |
|-----------|-----------------|--------------|--------|
| high | low | low | BUILD |
| high | high | any | REFACTOR |
| mid | mid | low | EVOLVE_AND_BUILD |
| low | any | low | FOCUS_BUILD |
| any | any | high | PIVOT |

Code-ahead-of-spec → Phase 6 doc-audit, not builders.

Output: `.corpus-build/plans/<subsystem>_build_plan.json`.

### CHECKPOINT 1 (mandatory)

Present gap judgment + plan + measured ledger stats if any (never invented estimates).
User: proceed / focus / pivot / abort.

## Phase 3: Build (batch-parallel)

DAG levels per `references/03-dag-ordering-rules.md`. Up to 3 concurrent worktrees per
level. Slice manifests in `.corpus-build/slices/current/`.

Reserved shared files use **intent files**
(`.corpus-build/intents/<WU>_intents.json`) — builders do not race-edit registration hubs.

codeNERD reserved hubs (default):

- `internal/shards/registration.go`
- `internal/core/virtual_store.go` / routing split files
- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/**/*.mg`
- CLI command registration under `cmd/nerd/`
- prompt atom loaders if shared indexes exist

Workers self-verify HOST-SAFE packages only.

## Phase 3.6: Serial Gate (orchestrator)

After each level:

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go vet ./<touched>/...
go test -race ./<touched>/...
# binary when needed:
go build -o nerd.exe ./cmd/nerd
```

Capture errors to `.corpus-build/results/<level>_gate.txt`. Max 3 fix cycles then escalate.

## Phase 3.5: Tests

Route unit / integration / cross-system work units to `go-architect` patterns or
test-forge agents when available. Five-case discipline: happy, nil/empty, error,
boundary, concurrency.

## Phase 4: Review

Dispatch **corpus-critic**: stubs, invariants, Mangle safety, subgraph-equivalent
isolation (package boundaries), test relevance. NEEDS_FIX re-spins only cited WUs.

### CHECKPOINT 2

User: proceed to wiring / fix WU / abort.

## Phase 5: Wiring (registry-driven)

```powershell
python .agents/skills/corpus-build/scripts/verify_surfaces.py `
  --registry .agents/skills/corpus-build/references/surfaces.yaml `
  --manifest .corpus-build/manifests/<subsystem>.json `
  --json .corpus-build/results/<run>_wiring.json
```

Verdicts: PASS / FAIL / N-A / AMBIGUOUS. Dispatch **corpus-wiring-auditor** for
AMBIGUOUS + intent incorporation. FAIL owners from registry `fix_owner`.

Intentional skips: `scripts/record_skip.py` or it did not happen.

## Phase 6: Doc Audit

Dispatch **corpus-doc-auditor** (only agent allowed to update architecture status rows):

- Reconcile IMPLEMENTED_SPEC status from gate evidence
- Update `Docs/architecture/<feature>/_progress.md`
- Optionally note Spec drift for later `spec-doc-sprint`
- Append journal economics (measured only)

## Phase 6.5: Git Publish

Explicit-path commits per slice (never `git add -A` on shared trees). Push only when
user policy / AGENTS.md regular-push applies and user has not forbidden it.

## Phase 7: Final Report + Optional Jules / follow-up

If fix budget exhausted, package FailureEvent for external remediation (Jules when
available). Final report: matrix, WU outcomes, coverage, wiring table, ledger, skips.

### CHECKPOINT 3

User: accept / reject / follow-up.

## Phase 8: Self-Improvement

Journal gaps in skill/agents/hooks; roll measured ledger into `references/journal.md`.

---

## Fleet Roster

| Agent | Role |
|-------|------|
| `corpus-reader` | Corpus parse + code matrix |
| `corpus-judge` | Gap classes + build plan |
| `corpus-builder` | Implement WUs in worktree |
| `corpus-critic` | Review change set |
| `corpus-wiring-auditor` | Surfaces + intents |
| `corpus-doc-auditor` | Status / progress docs only |

Reuse: `go-architect`, `mangle-logic-architect`, `prompt-jit`, `wiring-auditor`,
`explore`, `plan`.

---

## Safety Boundaries

1. No auto-merge without explicit user request
2. Never delete passing tests to go green
3. Builders do not write `Docs/architecture/` status (doc-auditor only)
4. Max 3 concurrent worktrees
5. Never claim implemented without gate evidence
6. Never skip gap-judgment checkpoint
7. Silent skips forbidden
8. No time/cost estimates — measured ledger only
9. Mangle changes need Decl + safety; prefer mangle-logic-architect for non-trivial rules
10. Prompt behavior via atoms, not ad-hoc shard prose dumps

---

## Scripts

| Script | Purpose |
|--------|---------|
| `scripts/build_dag.py` | Plan → levels + cycle check |
| `scripts/record_skip.py` | Auditable skips |
| `scripts/verify_surfaces.py` | Registry verdicts |

## References

| File | When |
|------|------|
| `references/01-work-unit-types.md` | Phase 3 routing |
| `references/02-integration-surface-checklist.md` | Phase 5 human checklist |
| `references/surfaces.yaml` | Phase 5 machine registry |
| `references/03-dag-ordering-rules.md` | Phase 3 levels |
| `references/vision-summary.md` | Phase -1 inject |
| `references/journal.md` | Living learnings |
| `references/common/*` | Worker shared docs |
