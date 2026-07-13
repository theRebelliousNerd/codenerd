# Plan: corpus-build v2 — Intelligent Multi-Agent Spec-to-Code Ecosystem

**Date**: 2026-03-22 (v1) · **Uplifted in place**: 2026-07-08 (v2)
**Status**: Design-of-record for the corpus-build v2 ecosystem build-out
**Type**: Skill-ecosystem uplift — orchestrator + specialist fleet + micro-skills + mechanical hook enforcement

> **corpus-realize decides WHAT to build. corpus-build is HOW it gets built.**
> Pipeline position: arch-templates / arch-propose → [corpus exists] → corpus-build → nerd-evolve

This v2 rewrite supersedes the v1 text of this document in full. It is grounded in
(a) a full audit of the shipped v1 pipeline and its 5 real runs (causal 2026-03-23,
agent-experiential-memory 2026-04, attention-routing-structural 2026-06-10, gravity 2026-07-04,
mnemosyne wiring 2026-07-03), (b) the NeuroLog `spec-to-code` v9.9.2 skill (the reference
standard we are matching and exceeding), and (c) the Claude Code harness surface as of
2026-07: subagent frontmatter `hooks:`, `skills:` preloading, `memory:` persistence,
`disallowedTools:`, and all hook events available per-agent.

---

## 1. What v1 got right, and what reality showed

**Right (keep):** the reader→judge→builder→wiring-auditor spine; the 3-input gap judgment
(alignment / structural debt / vision drift); DAG-leveled parallel builds with the
reserved-file/intent pattern; the vision anchor; mandatory human checkpoints; the
102/106-surface wiring checklist concept; all four scripts (`build_dag.py`,
`subsystem_audit.py`, `verify_surfaces.py` — `cost_estimate.py` is deleted, see §9).

**What reality showed (fix in v2):**

1. **Three lineages hide under the `corpus-*` prefix.** Only 4 agents belong to this
   pipeline (`corpus-reader`, `corpus-judge`, `corpus-builder`, `corpus-wiring-auditor`).
   Six others (`corpus-foundation-worker`, `corpus-integration-worker`,
   `corpus-surface-worker`, `corpus-wiring-worker`, `corpus-packetizer`,
   `corpus-governance-reconciler`) are Codex packet-backbone ports belonging to the
   mission-outcomes / subagent-system-workbench world, yet all six falsely declare
   `skills: [corpus-build]` and carry generic template descriptions, no tool
   restrictions, and no modularization guard. `corpus-feature-tagger` is a third,
   correctly-wired lineage (doc tagging).
2. **The self-improvement protocol was never executed.** `journal.md` and `CHANGELOG.md`
   are frozen at the 2026-03-22 creation entry despite 5 real runs. Prose mandates don't
   fire; hooks do.
3. **Agent routing drifted undocumented.** Reference docs route tests to `test-forge integration`;
   real plans dispatch `go-test-grinder` and the newer `test-forge-*` family. Build-plan
   JSON schema evolved (added `agent`, `type: "execution (non-builder)"`) with no doc
   reconciliation.
4. **The wiring checklist is prose, so it costs a grepping agent to evaluate.** 106
   surfaces evaluated by an agent reading markdown tables is expensive and unreliable.
   The checklist must become a machine-readable registry with mechanical
   detection/verification, leaving agents only the judgment calls (see the sibling doc
   `PLAN-corpus-build-wiring-checklist.md`).
5. **Codegen, client consumables, and Jules handoff were checklist rows, not phases.**
   Rows get skipped; phases with gates don't.

---

## 2. Design pillars (what we adopt from spec-to-code, and what codeNERD adds)

**Adopted from NeuroLog spec-to-code v9.9.2** (proven over dozens of runs):

- **Micro-skill per agent** — orchestrator SKILL.md stays a lean routing layer; domain
  knowledge lives in each agent's companion skill, preloaded via frontmatter `skills:`.
- **Mechanical enforcement over prose** — per-agent frontmatter hooks block forbidden
  actions at tool-call time (compile blocking, write-scope guards, post-write checks).
- **Paired accountability** — a critic gate between build and integration; workers never
  grade their own homework.
- **Pinned interface contracts** — interrogation pins shared types/signatures/seam
  ownership BEFORE parallel workers fan out; workers author against the contract.
- **Concurrency pre-check** — before dispatching any work unit: `git log --oneline -5 --
  <files>`, existence tests for declared-new files, and symbol/gap-id grep for
  modify-targets. Multiple agents share this checkout and this repo has a documented
  history of duplicate parallel builds.
- **Serial gates owned by the orchestrator** — one entity runs the authoritative
  compile/test/codegen passes and routes errors back as targeted fix cycles.
- **Persistent agent memory** — `memory: project` on every fleet agent so learnings
  compound across runs.
- **Skip records** — every intentionally skipped phase/surface writes an auditable
  reason (`scripts/record_skip.py` → `.corpus-build/skips.jsonl`), never a silent pass.

**codeNERD-specific (where we exceed spec-to-code):**

- **Machine-readable context system** — the `NERD_FEATURE` tag corpus + doc
  frontmatter + roadmap CSVs compile into one index that hooks use for *dynamic spec
  injection*: context follows the worker's actual reads/writes instead of being
  front-loaded (§6).
- **Registry-driven wiring** — `surfaces.yaml` gives every integration surface an
  applicability predicate, detection greps, and a verification command; the wiring phase
  becomes mostly mechanical (§7 and sibling doc).
- **Self-fixing handoff** — failures that exhaust their fix budget become
  `internal/testing/remediation/` FailureEvents dispatched to Jules, closing the loop
  with the live remediation subsystem instead of dying in a report (§10).
- **Measured token economics** — a SubagentStop token-ledger hook replaces speculative
  cost tables with per-run, per-agent, per-phase measured truth (§9).
- **OOM-aware verification** — the handlers-package host-build freeze (froze this
  128 GB machine twice) is hook-blocked for every agent; Docker is the only compile
  path for that package.

---

## 3. Agent roster v2

### 3.1 Namespace cleanup (precondition)

The six packet-backbone agents are renamed out of the corpus namespace to `packet-*`
(`packet-foundation-worker`, `packet-integration-worker`, `packet-surface-worker`,
`packet-wiring-worker`, `packet-packetizer`, `packet-governance-reconciler`), their
`skills:` pointers corrected away from `corpus-build` to their real parent
(subagent-system-workbench / mission-outcomes), and every consumer reference updated
(`.claude/skills/mission-outcomes-architecture-workbench/references/02-agent-selection-matrix.md`,
`.claude/skills/subagent-system-workbench/`). Acceptance gate: `grep -rl "corpus-foundation-worker\|corpus-packetizer\|corpus-surface-worker\|corpus-integration-worker\|corpus-wiring-worker\|corpus-governance-reconciler" .claude/` returns only files that are themselves being renamed.

### 3.2 The fleet

| Agent | Model / effort | Role | Status |
|---|---|---|---|
| `corpus-reader` | opus / xhigh | Corpus → feature manifest + reconciliation matrix; consumes the tag index first, prose second; stamps frontmatter on untagged docs it reads (tag-as-you-go) | uplift existing |
| `corpus-judge` | fable / high | 3-input gap judgment → classified build plan (DAG) | uplift existing |
| `corpus-builder` | fable / high | One work unit of Go code from spec, anti-hallucination gated | uplift existing |
| `corpus-critic` | fable / high (opus on mechanical WUs) | Post-build review gate: stub detection, invariant conformance, spec-intent vs impl-behavior test relevance, package-scope-isolation and read-before-write (persistent store) conformance | **new** |
| `corpus-comms-plumber` | opus / high | Protocol wiring specialist: REST handler+route, gRPC service+proto, MCP tool, A2A capability, ADK tool declarations — the full route→handler→bind trace discipline | **new** |
| `corpus-defense-auditor` | opus / xhigh | Defense-in-depth: constitutional safety (permitted) permission definitions, input validation at boundaries, telemetry spans + Prometheus collectors, **observation-collector registration** (the Jules self-fixing entry point), gatekeeper policies, backup scope | **new** |
| `corpus-consumables-keeper` | sonnet / high | internal/ sync: customer skills (the 5th ship dimension), 7-language clients, Python SDK integrations, CLI commands; runs and extends the parity scripts | **new** |
| `corpus-wiring-auditor` | fable / high | Adjudicates the registry verifier's AMBIGUOUS results; applies registration intents; owns the wiring verdict | uplift existing |
| `corpus-doc-auditor` | sonnet / high | Post-build reality reconcile: IMPLEMENTED_SPEC §Implementation Status, `NERD_FEATURE` tag updates, frontmatter stamps, tag-index + roadmap-register regen, journal + CHANGELOG entries | **new** |
| `corpus-jules-dispatcher` | sonnet / high | Packages fix-budget-exhausted failures as remediation FailureEvents / Jules sessions with forensic context packets; tracks attempt IDs into the run report | **new** |
| `corpus-feature-tagger` | opus / xhigh | Bulk/backlog tagging campaigns (distinct from tag-as-you-go, which every agent does inline) | keep as-is |

**Reused, not duplicated** (routing table reconciled to name the current generation):
`test-forge-unit-test-grinder`, `test-forge-integration-test-builder`,
`test-forge-cross-system-test-architect`, `go-test-grinder` (legacy alias),
`corpus-comms-plumber` (subsumed by comms-plumber for new work; kept for graphcad pipeline),
`requirements-interrogator` (contract-pinning interrogation), `go-architect / test-forge unit`.

### 3.3 Standard frontmatter contract (every fleet agent)

```yaml
---
name: corpus-<role>
description: <specific, ends with "Called by corpus-build skill.">
model: <per table>            # dispatch-time override allowed
effort: <per table>
memory: project               # learnings compound across runs
tools: [Read, Write, Edit, Glob, Grep, Bash]   # minimum viable set per role
disallowedTools: [Agent]      # no nested spawn — hard rule from fleet history
skills:
  - corpus-<role>             # companion micro-skill, preloaded in full
hooks:
  PreToolUse:
    - matcher: "Bash|PowerShell"
      hooks:
        - type: command
          command: "powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/block-oom-build.ps1"
          timeout: 10
    - matcher: "Write|Edit"
      hooks:
        - type: command
          command: "powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/write-scope-guard.ps1"
          timeout: 10
  PostToolUse:
    - matcher: "Read"
      hooks:
        - type: command
          command: "powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/spec-context-injector.ps1"
          timeout: 15
---
```

Role-specific deltas: `corpus-judge`/`corpus-critic`/auditors drop `Edit`+`Write` where
they only emit JSON/markdown artifacts to `.corpus-build/`; `corpus-builder` adds the
docs-architecture write-block (builders never modify the architecture corpus);
`corpus-doc-auditor` is the ONLY fleet agent allowed to write under `Docs/architecture/`
(scoped by write-scope-guard to the run's subsystem directory + roadmap registers).

Session-global guards (`modularization-guard.ps1`, `anti-defer-guard.ps1`,
`disk-hygiene.sh`) already fire for subagent tool calls via `.claude/settings.json` —
frontmatter hooks ADD to them, never replace them.

---

## 4. Micro-skills

Each new agent gets a companion skill at `.claude/skills/corpus-<role>/` following the
spec-to-code pattern: `SKILL.md` (domain knowledge, patterns, refusal boundaries) +
`references/` + `scripts/` (agent-runnable spot-check tools).

**Agent-runnable scripts (the "spot check" toolbox):**

- `corpus-builder/scripts/spot_check_coverage.py` — per-package `go test -cover` on
  host-safe packages, diffed against the WU's coverage expectation; cheap self-audit
  before reporting complete.
- `corpus-comms-plumber/scripts/trace_route.py` — given a route, verifies
  route-registration → handler → bind-struct → OpenAPI-contract chain and prints the
  gap (encodes the route→handler→bind trace rule).
- `corpus-defense-auditor/scripts/check_rbac_coverage.py` — diffs new endpoints against
  permission definitions in `internal/core/defaults/policy/`.
- `corpus-consumables-keeper/scripts/consumables_parity.py` — extends
  `scripts/check-client-parity.sh` coverage to all 7 client languages + skills-sync
  check (endpoint mentions in `.agents/skills/codenerd-*/references/` vs live route list).
- `corpus-doc-auditor/scripts/build_tag_index.py` — see §6.

**Shared common docs — sync-script, not symlinks.** Common references (attribution
format, anti-hallucination gate text, package-scope-isolation and read-before-write (persistent store) rules,
reporting format) live once in `.agents/skills/corpus-build/references/common/` and are
mirrored into each micro-skill by `scripts/sync_common_refs.py` (checksum-stamped header,
CI-checkable). Rationale: Windows symlinks need Developer Mode/admin and materialize as
plain text when `core.symlinks=false` (the default here) — a silent-breakage class we
don't accept for load-bearing rule text. Junctions work for directories but don't
survive git. The sync script is boring and verifiable: `--check` mode fails when a
mirror drifts from canon.

---

## 5. Hook architecture (three layers)

**Layer 1 — session-global (exists today, unchanged):** modularization-guard,
anti-defer-guard on Edit/Write; disk-hygiene on Bash. Apply to every agent including
fleet workers.

**Layer 2 — per-agent frontmatter (new, scoped to the agent's lifetime):**

| Hook | Event/matcher | What it enforces |
|---|---|---|
| `block-oom-build.ps1` | PreToolUse Bash/PowerShell | Blocks host `go build`/`go test` targeting `cmd/nerd` (and `./...` from repo root, which transitively compiles it); message points to the Docker compile path. Grant-marker escape hatch: `.corpus-build/.compile-grant-<agent>` created/deleted only by the orchestrator |
| `write-scope-guard.ps1` | PreToolUse Write/Edit | Worker may only touch files listed in its WU slice manifest `.corpus-build/slices/<WU-NNN>.json` (+ its own `.corpus-build/results/` artifact). Blocks reserved files (registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main), .nerd/config.json and internal/config, MCP/tool schemas) for builders — those go through intents. Blocks `Docs/architecture/**` for everyone except corpus-doc-auditor |
| `spec-context-injector.ps1` | PostToolUse Read | Dynamic spec injection — see §6 |
| `spec-attribution-check.ps1` | PostToolUse Write/Edit (builders) | New/changed Go files must carry a `// SPEC: <doc>#<section>` attribution near the package or symbol they implement; warn-level (additionalContext), not block-level, for the first iteration |

**Layer 3 — fleet telemetry (settings.json additions, matcher `corpus-.*`):**

- `SubagentStart` → `corpus-fleet-start.ps1`: appends dispatch row (agent_type, WU id,
  phase) to `.corpus-build/ledger/fleet_events.jsonl` (same pattern as the
  roadmap-grinder fleet hooks already in settings.json).
- `SubagentStop` → `corpus-token-meter.ps1`: port of NeuroLog's
  `skill-token-subagent.sh` with its hard-won fixes intact (bounded-transcript
  resolution via `<transcript-dir>/<session>/subagents/agent-<id>.jsonl`, recency
  fallback for named agents, hard guard against summing the main transcript,
  stem-dedupe against idle-ping re-fires). Writes
  `.corpus-build/ledger/token_runs.csv` with schema
  `ts,run_id,phase,agent_type,agent_id,output,input,cache_creation,cache_read,billable_total`.
- The active-run state (`run_id`, current `phase`) is stamped to
  `.corpus-build/ledger/<session>.active` by the orchestrator at Phase 0 and updated at
  each phase transition — giving per-phase token attribution for free.

---

## 6. Machine-readable context system + dynamic spec injection

**Three metadata systems exist today; v2 unifies at the index, not the source:**

1. `NERD_FEATURE` HTML-comment tags (corpus-feature-tagging schema; 57 files tagged;
   feeds `roadmap/31_feature_tag_register.csv` + `32_..._by_corpus.csv`) — the
   feature-granular layer. KEEP as the feature tagging mechanism.
2. graphcad's 6-field YAML frontmatter + `make doc-inventory-check` — the doc-class
   governance layer, currently graphcad-only. GENERALIZE its core fields corpus-wide:
   `doc-class` (shipped | shipped-with-future | north-star | governance), `subsystem`,
   `source-paths` (for virtual subsystems), `last-verified`. Applied
   **tag-as-you-go**: no backlog grind — any fleet agent (or main-thread session) that
   reads an untagged `Docs/architecture/` doc stamps it before finishing its task; the
   Read hook injects the reminder mechanically.
3. Roadmap CSVs (`21_feature_inventory.csv`, `22_feature_dependencies.csv`, 31/32) —
   the machine-owned registers. KEEP.

**The unification artifact:** `corpus-doc-auditor/scripts/build_tag_index.py` compiles
all three into `Docs/architecture/roadmap/33_corpus_context_index.json` (machine-owned,
regenerated, committed):

```json
{ "docs":     { "<path>": {"subsystem", "doc-class", "features": [], "last-verified"} },
  "features": { "<id>":   {"topic", "plane", "status", "owner_doc", "source_paths": []} },
  "packages": { "internal/<pkg>/": {"subsystem", "features": [], "surfaces": []} } }
```

Partial coverage is a design property, not a failure: untagged docs simply don't appear;
a `coverage` block reports the climb. The index is rebuilt by the doc-audit phase of
every run and on demand.

**Dynamic spec injection (`spec-context-injector.ps1`, PostToolUse Read):**

- Read of `Docs/architecture/**/*.md` → if the doc has no frontmatter/tags, inject:
  *"untagged doc — stamp schema-conformant frontmatter before completing this task
  (schema: Docs/architecture/roadmap/FEATURE_TAGGING_SCHEMA.md)"*. If tagged, inject its
  feature ids + status planes so the agent knows what is shipped vs target without
  re-deriving it.
- Read of `internal/<pkg>/**/*.go` → look up the package in the index; inject the owning
  subsystem, its feature tags, and the applicable wiring-surface ids (from
  `surfaces.yaml`) — e.g. *"this package serves features WH-012, WH-014; its declared
  surfaces: rest, mcp, config, rbac, telemetry, observation-collector"*. This is the
  answer to "boxes that won't take an agent grepping around to find" — the boxes find
  the agent.
- Bounded: injector caches per-session, emits each injection once per (agent, target),
  and stays silent on misses. Silence is always safe — degradation = today's behavior.

---

## 7. Orchestration loop v2

```
-1  VISION ANCHOR      cached 3-5 sentence summary (regenerate when vision docs change)
 0  INIT               dirs, agents-exist check, virtual-subsystem source_paths[],
                       run_id + ledger .active stamp
 0.5 CONCURRENCY PRE-CHECK   git log sweep + new-file existence + symbol/gap-id grep
                       per planned WU; STOP and reconcile on evidence of a parallel or
                       prior landing (multi-agent-on-main is normal here)
 1  INGEST             corpus-reader: tag-index first, prose fallback; feature manifest
                       + reconciliation matrix; stamps frontmatter as it reads
 1.5 INTERROGATE       requirements-interrogator on the plan; PINS interface contracts
                       (shared types, signatures, seam ownership) to
                       .corpus-build/contracts/<subsystem>.md — workers author against it
 2  GAP JUDGMENT       corpus-judge; 3-input scoring; DAG plan
     ── CHECKPOINT 1: proceed / focus / pivot / abort ──
 3  BUILD              DAG-leveled parallel corpus-builder workers (≤3 worktrees/level);
                       write-scope + OOM hooks live; workers self-verify HOST-SAFE
                       packages only; reserved files via intents
 3.6 SERIAL GATE       orchestrator-owned: go vet + go build (scoped package path for the
                       handlers package and full-tree builds), go test -race on touched
                       packages; error list routed back as targeted parallel fix cycles
                       (max 3 gate cycles, then escalate)
 3.5 TEST              test-forge routing per reconciled table; five-case discipline;
                       orchestrator runs suites, bounces failures to owning builder
                       (max 2-3 cycles)
 4  REVIEW             corpus-critic gate: stubs, invariants, package/fact-space isolation,
                       read-before-write (persistent store), test relevance (spec intent, not impl echo)
     ── CHECKPOINT 2: proceed to wiring / fix WU / abort ──
 5  WIRING             verify_surfaces.py --registry surfaces.yaml → PASS/FAIL/N-A/
                       AMBIGUOUS with evidence; corpus-wiring-auditor adjudicates
                       AMBIGUOUS; FAILs dispatched to owners: comms-plumber (protocol),
                       defense-auditor (rbac/telemetry/observation), consumables-keeper
                       (internal/), builder (engine)
 5.5 CODEGEN GATE      orchestrator-owned, serial, ORDERED: protoc (if proto touched) →
                       route snapshot → openapi spec (container path — handlers.test
                       OOMs on host) → orval client → ws-client → ts-constants → mocks →
                       parity checks (check-api-client, client-parity, consumables_parity)
 6  DOC AUDIT          corpus-doc-auditor: IMPLEMENTED_SPEC status reconcile, tag stamps,
                       33_corpus_context_index.json + registers regen, journal +
                       CHANGELOG entries (mechanically this run, not aspirationally)
 6.5 GIT PUBLISH       explicit-path commits per slice, push after every verified slice;
                       safety-net reconcile at run end (stash sweep + unpushed check)
 7  JULES HANDOFF      corpus-jules-dispatcher: any failure that exhausted its fix
                       budget → FailureEvent → remediation orchestrator → Jules session;
                       attempt IDs recorded in the final report
     ── CHECKPOINT 3: accept / reject / follow-up ──
 8  SELF-IMPROVEMENT   journal entry + CHANGELOG bump + token-ledger rollup appended to
                       the run report; skill/agent gaps found during the run get fixed
                       or filed as explicit work items
```

Phases 3.6/3.5 keep spec-to-code's key insight — **the orchestrator is the single
authority for gate verdicts** — while keeping codeNERD's cheaper reality: Go builds are
fast and cache-shared, so workers still self-verify host-safe packages during authoring;
the serial gate is the *authoritative* pass, not the only compile ever run. The hard
mechanical line is the OOM package: no agent, worker or orchestrator, host-builds
`cmd/nerd` — scoped package tests; full-tree with care, hook-enforced.

---

## 8. Verification protocol

Per level: authoritative serial gate (vet, build, race on touched packages). After all
levels: full suite for the subsystem + adjacent integration tests, wiring registry
verdict, codegen parity green, reconciliation re-run vs Phase 1 baseline, critic verdict,
final report with before/after matrix + ledger totals. A WU is DONE only with gate
evidence recorded in `.corpus-build/results/` — "no claiming implemented without
go build evidence" survives from v1, now with the gate's output attached.

---

## 9. Token economics (cost model DELETED)

The v1 estimated-cost table and `cost_estimate.py` are removed — speculative
dollar/time estimates violate the repo's no-estimates rule and anchor planning on
fiction. Replacement: the Layer-3 token ledger measures actual spend per run/phase/agent.
The pre-build checkpoint cites **measured medians from prior runs** of comparable WU
count once history exists; before that, it presents WU count + fan-out only. The ledger
CSV schema is designed for later ingestion into a `corpus-build-telemetry` package-scope
(package-scope-scoped, per the isolation mandate) so fleet economics become queryable
substrate data.

---

## 10. Jules self-fixing handoff (coverage expansion)

Ground truth today: `internal/testing/remediation/` is real and boot-wired — ~30 runtime
observation collectors → aggregator → forensic packet builder → `dispatch_jules.go` →
Jules sessions, with the `testing_remediation` ADK agent polling attempts. The gap:
**test-run failures never enter it** (`internal/testing/suite/` constructs no
FailureEvents; doc 19's Stage-1 claim is aspirational).

corpus-build v2 closes its slice of that gap from the consumer side:

1. `corpus-jules-dispatcher` packages fix-budget-exhausted build/test/wiring failures
   into FailureEvents carrying the WU's spec refs, contract path, gate error output, and
   verification command — feeding the EXISTING forensics/dispatch machinery, not a
   parallel path.
2. The wiring registry gains an `observation-collector` surface (defense-auditor owned):
   any new subsystem with runtime failure modes registers a collector in
   `internal/testing/remediation/observation/collectors/` — every corpus-build run
   therefore *widens* the self-fixing system's sensory field as a side effect.
3. Explicit follow-on work item (engine-side, separate from this skill): wire
   `internal/testing/suite/` failure results to FailureEvent construction so CI/test
   failures dispatch autonomously. corpus-build's dispatcher is the manual-trigger
   precursor and proof of the packet shape.

---

## 11. Relationship to other skills

Unchanged in role: arch-templates / arch-propose author corpora upstream;
nerd-evolve optimizes downstream; ralph-codenerd / integration-auditor audit without
building; roadmap-grinder grinds the feature CSV (different granularity — feature rows,
not whole-subsystem realizations; shares the test-forge fleet and telemetry patterns).
The packet-* backbone (renamed) belongs to mission-outcomes-architecture-workbench and
is no longer confusable with this pipeline.

---

## 12. Build order (dependencies + acceptance gates, no durations)

| # | Work item | Depends on | Acceptance gate |
|---|---|---|---|
| W1 | Rename 6 packet agents out of corpus-*; fix skills pointers; update consumers | — | grep gate in §3.1 clean |
| W2 | Hook scripts: block-oom-build, write-scope-guard, spec-context-injector, spec-attribution-check, corpus-fleet-start, corpus-token-meter | — | each script has a dry-run self-test; injector silent on misses |
| W3 | Uplift core-4 agent frontmatter to §3.3 contract | W2 | agents parse; hooks fire in a scratch dispatch |
| W4 | surfaces.yaml registry + verify_surfaces.py --registry upgrade | — | verifier emits PASS/FAIL/N-A/AMBIGUOUS with evidence on a shipped subsystem (causal) with 0 false FAILs on its known-wired surfaces |
| W5 | build_tag_index.py + 33_corpus_context_index.json | — | index builds from current 57 tagged files + graphcad frontmatter + CSVs; coverage block present |
| W6 | New agents + micro-skills: critic, comms-plumber, defense-auditor, consumables-keeper, doc-auditor, jules-dispatcher | W2, W4 | each agent file parses, preloads its micro-skill, passes a scoped smoke dispatch |
| W7 | sync_common_refs.py + common refs dir | — | --check mode green |
| W8 | SKILL.md v2 rewrite + references reconcile (work-unit routing table to test-forge generation; journal + CHANGELOG catch-up entries for the 5 historical runs) | W1–W7 | SKILL.md ≤ ~250 lines; references match dispatched reality |
| W9 | settings.json Layer-3 hook registration (corpus-.* matchers) | W2 | fleet_events + token_runs rows appear in a scratch run |
| W10 | Pilot run on one small subsystem; fix what the run surfaces | all | full loop executes; ledger + journal + index artifacts produced |
| W11 | Follow-on (engine-side): testing/suite → FailureEvent wiring; proto + ts-constants CI gates; 4-language client parity extension | after pilot | CI jobs red on injected drift, green on clean tree |

Slicing discipline: each W-item is one or more explicit-path commits, pushed when its
gate is green. Workers share this working tree — partition strictly by file ownership.
