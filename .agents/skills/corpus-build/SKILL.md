---
name: corpus-build
description: >
  Spec-driven implementation engine that reads an codeNERD architecture corpus
  (Docs/architecture/<subsystem>/), audits existing code against the spec, judges
  the gap (build/evolve/pivot), and dispatches a hook-governed specialist fleet to
  build missing code with dependency-aware DAG ordering, registry-driven wiring,
  serial codegen gates, and Jules self-fixing handoff. Use when user says "realize",
  "build from spec", "corpus-build", "build from architecture", "make real",
  "implement the spec", "realize the [subsystem]", "build [subsystem] from docs",
  or "spec to code". Do NOT use for routine bug fixes, writing architecture docs
  (use arch-templates), optimizing existing code (use nerd-evolve), discovering
  new compositions (use integration-auditor), improving agent prompts (use
  agentic-evolve), or doc-code drift audits without implementation (use
  ralph-codenerd or integration-auditor).
metadata:
  version: 2.0.0
  author: codeNERD (full ecosystem port from Vectryx; mutated for codeNERD)
  last-verified: 2026-07-13
  spawned-agents:
    - corpus-reader
    - corpus-judge
    - corpus-builder
    - corpus-critic
    - corpus-comms-plumber
    - corpus-defense-auditor
    - corpus-consumables-keeper
    - corpus-wiring-auditor
    - corpus-doc-auditor
    - corpus-jules-dispatcher
    - corpus-feature-tagger
---

# Corpus Build v2

> **codeNERD full ecosystem port.** Entire skill tree copied from Vectryx then mutated file-by-file. Agent fleet lives under `.grok/agents/` and `.claude/agents/` with bound `skills:` frontmatter. Architecture corpora: `Docs/architecture/`. North star: LLM creative / Mangle executive; `permitted(...)` default deny; JIT prompt atoms.


Spec-driven implementation engine. Reads an architecture corpus, audits code against
spec, produces a gap-classified build plan, and dispatches a specialist fleet in
dependency order — with mechanical hook enforcement, registry-driven wiring, measured
token economics, and self-fixing handoff.

> **corpus-realize decides WHAT to build. corpus-build is HOW it gets built.**
> Design-of-record: `.agents/skills/corpus-build/references/plans/PLAN-corpus-build.md`
> (+ wiring-checklist and realize siblings). Load those for rationale; this file is the routing layer.

Pipeline position: arch-propose / living Docs/architecture corpus → corpus-build → nerd-evolve

---

## DELEGATION MANDATE

You are the orchestrator — dispatch agents, never author subsystem code yourself. You
own: run state, phase transitions, serial gates (compile/test/codegen), the ledger,
checkpoints, commits, and pushes. Workers author; you verify. Dispatch with
`run_in_background: true`; workers must never spawn subagents (`disallowedTools:
[Agent]` is stamped in their frontmatter).

---

## Mode Selection

"realize [subsystem]" / "build from spec" → full pipeline from Phase -1.
"corpus-build --plan [path]" → build mode, load existing plan, start at Phase 3.
Ambiguous "implement the spec" → ask which input source.

---

## Phase -1: Vision Anchor

Read the cached vision summary at `references/vision-summary.md` (codeNERD north star).
If missing/stale, regenerate from root `AGENTS.md` + `Docs/architecture/INDEX.md` +
`Docs/architecture/DARK-FACTORY-JOURNAL.md`.

Inject as `## Vision Context` in every downstream agent prompt.

---

## Phase 0: Initialization

0.1 Validate `Docs/architecture/<subsystem>/IMPLEMENTED_SPEC.md` exists.
0.2 Check `.corpus-build/plans/` for existing plans.
0.3 Ensure `.corpus-build/{plans,results,matrices,manifests,intents,journal,contracts,reviews,slices/current,ledger,jules}/` exist (gitignored).
0.4 Read `Docs/architecture/INDEX.md` for tier / doc count / source location.
0.5 Verify fleet agents exist (see roster below).
0.6 **Virtual subsystem detection**: IMPLEMENTED_SPEC line 5 Classification — extract ALL
    `source_paths[]`; never assume `internal/<subsystem>/` exists standalone.
0.7 **Run state**: write `.corpus-build/ledger/<session_id>.active` as
    `{"run_id":"corpusbuild_<subsystem>_<epoch>","phase":"init","skill":"corpus-build"}`.
    Update `phase` at every transition — the token meter attributes spend per phase.
    `<session_id>` is the FULL session UUID from hook events (the transcript filename
    stem under `~/.claude/projects/<project>/`), never a shortened prefix — the
    telemetry hooks gate on an exact `"$sessionId.active"` filename match and skip
    silently when it's absent. Keep phase strings ASCII (telemetry CSV friendliness).

## Phase 0.5: Concurrency Pre-Check (MANDATORY — multi-agent-on-main is normal here)

Per candidate feature/WU: `git log --oneline -5 -- <paths>`; `git show HEAD:<path>`
existence tests for declared-NEW files; symbol/gap-id greps for MODIFY targets
(`git log -S<symbol>`, `git log --grep=<feature-id>`). Evidence of a prior or parallel
landing → STOP, surface to the user, reconcile (preserve unique adds; route
already-shipped rows to doc-audit, never rebuild).

---

## Phase 1: Corpus Ingest + Code Audit

Dispatch **corpus-reader**. Index-first: consume
`Docs/architecture/INDEX.md` and any `*_context_index.json` if present; fall back to prose. Tasks:
(1) parse corpus into feature manifest, (2) grep ALL `source_paths[]` for the
reconciliation matrix. Tag-as-you-go: stamp frontmatter on untagged docs read along the
way (the Read hook reminds mechanically).

Outputs: `.corpus-build/manifests/`, `.corpus-build/matrices/`.
Anti-hallucination: grep-verify every extracted name; flag UNVERIFIED.

## Phase 1.5: Interrogate + Pin Contracts

Dispatch **requirements-interrogator** on the draft plan. Beyond stress-testing, it PINS
the interface contract (shared types, signatures, seam ownership) to
`.corpus-build/contracts/<subsystem>.md`. Every builder dispatch cites the contract path.
Skip rule: purely-additive single-WU runs; record skip via
`scripts/record_skip.py --target 1.5 --reason "..."`.

---

## Phase 2: Gap Judgment

Dispatch **corpus-judge**. 3-input score — Alignment %, Structural Debt % (justified by
invariant-contradiction counts), Vision Drift % (justified). Classifications: NONE,
PARTIAL, MISSING, UNWIRED, DIVERGENT. Decision table: >80%/<20%/<20% BUILD ·
>80%/>40%/any REFACTOR · 40-80%/<40%/<30% EVOLVE_AND_BUILD · <40%/any/<30% FOCUS_BUILD ·
any/any/>50% PIVOT. Code-ahead-of-spec rows route to Phase 6 doc-audit, not to builders.

Output: `.corpus-build/plans/<subsystem>_build_plan.json`.

### MANDATORY CHECKPOINT 1
Present gap judgment + plan + measured token medians from prior comparable runs (ledger;
if no history: WU count + fan-out only — never estimates). User: [proceed / focus /
pivot / abort].

---

## Phase 3: Build (batch-parallel)

Up to 3 concurrent worktrees per DAG level (`references/03-dag-ordering-rules.md`;
routing: `references/01-work-unit-types.md`). Before each level: write per-worker slice
manifests to `.corpus-build/slices/current/<agent_type>.json` — the write-scope hook
enforces them. Every dispatch carries: Vision Context, spec refs, contract path,
dependency outputs, anti-hallucination gate.

Reserved files (registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main), .nerd/config.json and internal/config, MCP/tool schemas) are hook-blocked for builders — they
write `.corpus-build/intents/<WU>_intents.json`; Phase 5 incorporates.

Workers self-verify **scoped packages only** (never accidental whole-repo thrash without intent).

## Phase 3.6: Serial Gate (orchestrator-run)

After each level:
```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go vet ./<touched>/...
go test -race ./<touched>/...
# binary only when CLI surface requires it:
go build -o nerd.exe ./cmd/nerd
```
Capture the full error list to `.corpus-build/results/<level>_gate.txt`; route errors to
owning workers as parallel fix cycles (max 3 gate cycles, then escalate to Phase 7).
Coverage check after test level (<70% new-code coverage → more tests). Gate verdicts
certify COMMITTED state.

## Phase 3.5: Test

Route per the reconciled table: unit → `test-forge-unit-test-grinder` /
`go-architect / test-forge unit`; integration → `test-forge-integration-test-builder`; cross-system
(3+ subsystems) → `test-forge-cross-system-test-architect`. Five-case discipline (happy,
nil/empty, error, boundary, concurrency). Orchestrator runs the suites; failing output
bounces to the owning builder (max 2-3 cycles).

## Phase 4: Review

Dispatch **corpus-critic** with the level's unified change set: stubs, invariant
conformance, package/fact-space isolation, read-before-write (persistent store), test relevance (spec intent, not
impl echo). NEEDS_FIX cites WUs; only those re-spin.

### MANDATORY CHECKPOINT 2
User: [proceed to wiring / fix WU / abort]. Abort tags `corpus-build-abort-<ts>`.

---

## Phase 5: Wiring (registry-driven)

Run `scripts/verify_surfaces.py --registry references/surfaces.yaml --manifest
<manifest> --json .corpus-build/results/<run>_wiring.json`. Verdicts: PASS / FAIL /
N-A / AMBIGUOUS with evidence. Dispatch **corpus-wiring-auditor** to adjudicate
AMBIGUOUS and incorporate intents. Route FAILs by `fix_owner`: **corpus-comms-plumber**
(protocol B*), **corpus-defense-auditor** (constitutional safety (permitted)/telemetry/observation-collector A24a),
**corpus-consumables-keeper** (internal/ D*), **corpus-builder** (engine A*), frontend →
graphcad/viz pipeline. Intentional skips of applicable surfaces require
`record_skip.py` rows.

## Phase 5.5: Codegen / Generate Gate (orchestrator-run, serial)

If the WU touches generated surfaces (MCP schemas, embed assets, prompt corpora builders):
run the repo's existing generators (`go generate`, `cmd/tools/*`, corpus builders) only for
touched paths. Skip foreign frontend/Orval-style pipelines that do not exist here. Any
diff in generated artifacts belongs to this run's commits.

---

## Phase 6: Doc Audit

Dispatch **corpus-doc-auditor** (the ONLY fleet agent with Docs/architecture write
access): IMPLEMENTED_SPEC §Implementation Status reconcile from gate evidence,
NERD_FEATURE plane flips, frontmatter stamps, regenerate
`33_corpus_context_index.json` + registers via `scripts/build_tag_index.py`, and write
THIS RUN's `references/journal.md` + CHANGELOG entries — the self-improvement protocol
executes as a phase, not as an aspiration.

**Campaign / cross-package impact reconcile (standing duty, same rank as IMPLEMENTED_SPEC):**
every feature the run shipped updates `Docs/architecture/<subsystem>/IMPLEMENTED_SPEC.md`
status from gate evidence and notes campaign/CLI/shard wiring impact in the corpus journal.
Do not invent demo coverage ledgers that do not exist in this repo.

## Phase 6.5: Git Publish

Explicit-path commits per slice (never `git add -A` — workers share the tree); push
after every verified slice. Run-end safety net: `git stash list` sweep + unpushed-commit
check.

## Phase 7: Jules Handoff

Dispatch **corpus-jules-dispatcher** for every failure that exhausted its fix budget:
FailureEvent packet (spec refs, contract, gate output, verification command) into the
existing remediation machinery (`internal/testing/remediation/`); attempt IDs recorded
in the final report.

### MANDATORY CHECKPOINT 3
Final report: before/after matrix, WU outcomes, coverage, wiring verdict table, codegen
parity, ledger totals by phase/agent, Jules attempt IDs, skips. User: [accept / reject /
follow-up].

## Phase 8: Self-Improvement

Journal + CHANGELOG entries land in Phase 6 (mechanical). Here: fix any skill/agent/hook
gap the run surfaced (or file it as an explicit work item), and roll the run's ledger
into `references/journal.md`'s economics table.

---

## Fleet Roster (models are dispatch-time assignments)

reader high/xhigh · judge high/high · builder high/high · critic high/high (opus on
mechanical WUs) · comms-plumber high/high · defense-auditor high/xhigh ·
consumables-keeper medium/high · wiring-auditor high/high · doc-auditor medium/high ·
jules-dispatcher medium/high · feature-tagger high/xhigh. Test fleet: test-forge-*.
All: `memory: project`, `disallowedTools: [Agent]`, micro-skill preload, standard hooks
(`.claude/hooks/corpus-build/`: block-oom-build, write-scope-guard,
spec-context-injector; builders + attribution check). Fleet telemetry:
SubagentStart/Stop hooks (matcher `corpus-.*`) append
`.corpus-build/ledger/fleet_events.jsonl` + `token_runs.csv`.

---

## Safety Boundaries

Forbidden: (1) auto-merge to main, (2) deleting passing tests, (3) builders modifying
`Docs/architecture/` (doc-auditor only, scoped), (4) >3 worktrees, (5) claiming
implemented without gate evidence, (6) skipping the gap-judgment checkpoint, (7) host
builds of the handlers package, (8) silent skips (record_skip.py or it didn't happen),
(9) estimates of time/cost — measured ledger numbers only.

| # | Phase | Gate | Options |
|---|-------|------|---------|
| 1 | 2 | Gap judgment + plan | proceed/focus/pivot/abort |
| 2 | 4 | Build + review results | proceed/fix WU/abort |
| 3 | 7 | Final report | accept/reject/follow-up |

---

## Scripts

`scripts/`: `build_dag.py` (plan → levels + cycle check), `subsystem_audit.py` (quick
readiness verdict), `verify_surfaces.py` (registry verdicts), `record_skip.py`
(auditable skips), `build_tag_index.py` (context index regen), `sync_common_refs.py`
(micro-skill common-doc mirror; `--check` in verification). `cost_estimate.py` was
deleted in v2 (measured ledger replaces estimates).

## References

| File | When | Contents |
|------|------|---------|
| `references/01-work-unit-types.md` | Phase 3 | WU types + agent routing (test-forge generation) |
| `references/02-integration-surface-checklist.md` | Phase 5 | Pointer to registry + human-readable source |
| `references/surfaces.yaml` | Phase 5 | THE machine-authoritative surface registry |
| `references/03-dag-ordering-rules.md` | Phase 3 | Level rules, reserved-file pattern |
| `references/common/` | dispatch | Canonical shared worker docs (synced to micro-skills) |
| `references/journal.md` | Phase 0/6 | Living learning journal + run economics |
| `references/agent-skill-quality-rubric.md` | evaluation | Quality scorecard |
