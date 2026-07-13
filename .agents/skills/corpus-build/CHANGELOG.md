# Skill Changelog

## [Run log] `corpusbuild_agents_1783124900` (WU-AG-12: agents Phase 6 reconcile, campaign ALL_CORPORA 3/74) - 2026-07-12

**No skill-mechanism change.** Phase 6 doc-audit closing a 12-WU run on the `agents` corpus
(protected tier, L0->L1->L2->L3). No dedicated `agents_review.json`/per-WU gate file existed
beyond `.corpus-build/results/agents_L0L1_gate.txt` (L0+L1 only) -- every L2/L3 test suite cited
in this reconcile was self-reproduced this audit (`go vet ./internal/shards/...` EXIT 0;
`go test -race` on `SystemHealth`, `WarmScheduling`, `SpecCompleteness`,
`Campaign narrativePageSpecialist` -- all PASS), not trusted from commit-message self-reports. Commits:
`2d9880ad16` (system_health de-stub + OQ-6 gate + 12 spec fills + OQ-7 frontend), `662b0be641`
(warm-scheduling wired live + typed `/agents/health` contract), `bd6bed6e43` (69-spec
completeness guard, 9 more stragglers, SWC-043 render+leak walk), `86705dd49b` (16 new sub-spec
directories).

- Reconciled `IMPLEMENTED_SPEC.md`: page-specialist catalog 62->73 (own direct parse of
  `defaultPageOrder`, not a re-quote of the dispatch's stale "23164-23166" port figure --
  grep-verified ports 23153-23155 for the 3 new entries instead), tool registry 86->87 with 6
  previously-missing namespace rows added (`gatekeeper`/`security_monitor`/`causal_discovery`/
  `mission_orchestrator`/`capacity_planner`/`data_quality`, none new this run), file counts
  418/317->426/329, F-005 `system_health` shipped, F-013/OQ-6 and F-014/OQ-7 shipped, F-008
  ADR-001 attribution corrected to `internal/core` (doc-only, no code change), OQ-3 recorded as
  partially resolved (Option 3 shipped pre-existing, Option 1 still DESIGN-FIRST).
- F-001/F-002/F-003 (conflict-resolution/ontology-evolution/cross-tenant-migration) recorded as
  shipped DOM-only PageKit specialists, explicitly distinct from the fuller "multi-agent
  pipeline" `01-VISION-AGENTS.md` still describes -- `TODO.md` rows stayed `[ ]` with an inline
  PARTIAL annotation rather than a false `[x]`, since the shipped code (~54-line `llmagent.New`)
  does not implement a pipeline.
- Flipped 16 `TODO.md` rows to `[x]` (12 boilerplate PageKit Specs shipped this run + OQ-6 + OQ-7
  + 4 pre-existing-but-stale Escalation-Queue/HITL/Learning/System-Corpus rows), each grep- or
  test-verified before flipping. The repo's `todo-done-migrator.ps1` hook auto-migrated the
  checked rows into `to-done.md` on the next tool call (expected behavior, verified via diff).
- Flipped campaign narrative coverage row SWC-043 `[~] partial -> [x] covered`
  (`Docs/architecture/demo/campaign narrative/06-FEATURE-COVERAGE.csv`), legalized by
  `TestCampaign narrativePageSpecialist_RenderWalk` + `_LeakWalk`
  (`internal/demo/campaign narrative_page_specialist_render_walk_test.go`) -- all five 07-WEAVE-STANDARD
  weave-contract parts (scene card, seed fixture, feature-matrix row, coverage row, render-walk
  asserting scene-specific IDs/content/resolved nodes) are now present. Added an explicit
  `covered-interactive` follow-up note (a live LLM-driven specialist turn remains open) to both
  the CSV and `16-CAMPAIGN_NARRATIVE-INTEGRATION.md`, preserving the `covered != interactive-live`
  honesty pattern from the adktools SWC-045 precedent. SWC-044 stays `partial`, unchanged.
- Stamped 6-field YAML frontmatter on 19 docs read this run (16 new sub-spec `IMPLEMENTED_SPEC.md`
  files, the umbrella `IMPLEMENTED_SPEC.md`, `OPEN-QUESTIONS.md`, `16-CAMPAIGN_NARRATIVE-INTEGRATION.md`,
  `CLAUDE.md` -- zero prior frontmatter in this corpus). Editing `CLAUDE.md` triggered a repo hook
  that silently mirrored its new content onto the unrelated `AGENTS.md` in the same directory,
  destroying AGENTS.md's own distinct prose; caught via `git status` immediately after and fixed
  with a single-file `git checkout -- AGENTS.md` restore (verified byte-identical to the prior
  commit). No `AGENTS.md`/`to-done.md`/`Docs/architecture/mnemosyne/` content was authored by
  this run.
- Regenerated `33_corpus_context_index.json` (2770 docs scanned, 68 tagged, 363 frontmattered,
  266 features, 62 packages); the 15 reported discrepancies are pre-existing `cli/` corpus drift,
  unrelated to this run. No `NERD_FEATURE` tags exist anywhere in the `agents` corpus (grep
  confirmed) -- none flipped.
- Full learnings: see `references/journal.md` 2026-07-12 entry (WU-AG-12).

## [Run log] `corpusbuild_agent-experiential-memory_1783124000` (WU-AEM-12: GDPR-scrub + traversal-reconcile + enrichment + named-suite doc reconcile) - 2026-07-12

**No skill-mechanism change.** Phase 6 doc-audit closing a 12-WU run on the
`agent-experiential-memory` virtual subsystem (L0->L1->L2/L3, critic APPROVED 6/6).

- Gate evidence: `.corpus-build/results/aem_L0_gate.txt`/`aem_L1_gate.txt`/`aem_L2_gate.txt`
  (`go vet`/`go build`/`go test -race` PASS across `internal/{agent_experiential_memory,core,
  graph,deductive,memory,consolidation,attention-routing,demo}`, plus a live benchmark run showing
  `BenchmarkTraversalIndex_HNSWQuery_20K` at 6.52ms/op under the 10ms spec target). Critic
  review: `.corpus-build/reviews/agent-experiential-memory_review.json` (6/6 APPROVED).
- Reconciled `IMPLEMENTED_SPEC.md` §3 with 5 new dated rows for this run's four BUILD-NOW
  features: F-042 (`core.Database.DeleteUser` full AEM scrub -- Mangle facts, traversal
  vectors, ToolInvocation records, AWM activation-history), F-044 (traversal-index stale-entry
  reconciliation primitive + consolidation cycle task), F-046 (ToolInvocation status/timing enrichment
  of Summary/Detail traversal vectors), F-048 (11 named test suites live-registered in
  `suite.SuiteRegistry` + all 6 spec-named benchmarks). Corrected a same-run mid-build draft
  claim in §11 that `BenchmarkTraversalIndex_HNSWQuery_20K` "remains open" -- it shipped in
  the same commit under a different package than originally planned (`internal/graph`, to
  avoid an import cycle from the AEM facade).
- Recorded the judge's 2026-07-12 PIVOT-REJECTED verdict for F-041 (per-user-package-scope
  Candidate C re-adoption) explicitly in `OPEN-QUESTIONS.md` Q-7 and `IMPLEMENTED_SPEC.md`
  §6/§12: the "no cascade API" premise is now stale, but the load-bearing rejection reasons
  (AEM-owned erasure callbacks, ontology overhead, migration proof) still hold and no adoption
  code was written. Added DESIGN-FIRST judge-classification headers to Q-5 (F-038), OQ-AEM-09
  (F-039), and OQ-AEM-10 (F-040) naming the specific decision each needs before code.
- Corrected a contract-pinning finding (FF-1) into `TODO.md`: `prior_failure/4` and
  `recovery_pattern/2` are external/virtual predicates with no stored fact to clear (the
  original row's framing was wrong); the real per-user substrate is `ToolInvocation` records
  (core-direct scrub) and `context_drift/3` (the actual stored EDB fact). Routed F-043
  (blob/HNSW cascade completeness) and F-045 (HNSW cold-start rebuild) to the graph/blob and
  graph/vector corpora respectively as pointer rows.
- Flipped campaign narrative coverage row SWC-101 `[ ] planned -> [x] covered`
  (`Docs/architecture/demo/campaign narrative/06-FEATURE-COVERAGE.csv`) legalized by
  `TestCampaign narrativeAEMThreadContext_RenderWalk` + `_LeakWalk`
  (`internal/demo/campaign narrative_aem_thread_context_test.go`). Updated
  `22-CAMPAIGN_NARRATIVE-INTEGRATION.md` with an honest distinction for SWC-102/SWC-103: the fixed
  `ChatSessionAgentThreadContextTemplate.IncludeFields` regression (a previously-dead
  `{label, properties}` field pair that meant turn content never rendered) is what makes the
  render-walk's answer packet non-vacuous, but SWC-102 (live `CascadeSearch`, not a
  pre-computed index read) and SWC-103 (dedicated answer-packet-synthesis scene) both still
  need their own proof and stay `planned`.
- Verified and kept two other agents' same-run out-of-lane edits after grep-checking every
  claimed function/line against source: a test-agent's ~200-row ArchTodoAuditor-style trim of
  already-shipped `TODO.md` items (2026-06 vintage, unrelated to this run), and a builder's
  two-paragraph addition to `Docs/architecture/graph/IMPLEMENTED_SPEC.md` §2.7.7 (AEM
  traversal-entry removal + tool-status enrichment mechanism) plus six new test-file rows in
  `18-TESTING-ALIGNMENT.md`. Added the `ChatSessionAgentThreadContextTemplate.IncludeFields`
  regression-fix writeup to the same §2.7.7 section (graph corpus's own file, updated its
  "Last verified" prose date rather than importing AEM's YAML frontmatter schema).
- Stamped 6-field YAML frontmatter on 4 of 5 named docs (`IMPLEMENTED_SPEC.md`,
  `OPEN-QUESTIONS.md`, `21-PREDICATE-SIGNATURE-REFERENCE.md`, `22-CAMPAIGN_NARRATIVE-INTEGRATION.md`).
  Could not stamp `TODO.md`: the write-time honesty-guard fires on the literal substring in the
  file's own pre-existing H1 title regardless of whether that line changed, and frontmatter
  insertion requires touching the line immediately above the H1 -- recorded as a tooling-limit
  skip, not a content decision.
- Regenerated `Docs/architecture/roadmap/33_corpus_context_index.json` via
  `build_tag_index.py` (2753 docs scanned, 68 tagged, 342 frontmattered, 266 features,
  62 packages; 15 reported discrepancies are pre-existing `cli/` corpus drift, unrelated).

## [Run log] `corpusbuild_adktools_1783123200` (WU-ADK-2: 121->147 registry-count reconcile + SWC-045 render-walk flip) - 2026-07-12

**No skill-mechanism change.** Phase 6 doc-audit closing a two-WU run (WU-ADK-1 shipped a
campaign narrative render-walk test; this is the WU-ADK-2 doc reconcile).

- Gate evidence: `.corpus-build/results/adktools_L1_gate.txt` (`go vet ./internal/adktools/...`
  EXIT 0, `go test -race -count=1 ./internal/adktools/...` PASS, commit `dcdc35ec6a`), plus an
  orchestrator-in-main-thread line-by-line APPROVED review recorded in
  `.corpus-build/skips.jsonl` (phase4-critic-dispatch entry).
- Re-verified the `ToolSet` registry count independently against source (struct field count +
  `AllTools()`/`RetrievalTools()` enumeration, all in agreement): **147 slots / 104 read-only**,
  not the docs' stale 121/87. Updated `IMPLEMENTED_SPEC.md`, `CLAUDE.md`, `README.md`,
  `05-TOOL-BINDINGS-REGISTRY.md`, `00-ALIGNMENT-VISION-REVIEW.md`, `01-VISION-ADKTOOLS.md`,
  `06-AGENT-INTEGRATION-PATTERNS.md`.
- Documented 8 previously-undocumented but fully-shipped tool families (Mnemosyne Realization
  Ladder/Dream-Branch/Reality-Channel/Authorship, Catalog, `write_system_corpus`,
  `transfer_focus`, `codenerd_budget_mission_cost`, `validate_mangle_rules`) in
  `IMPLEMENTED_SPEC.md` §2/§3 and 9 new compact family sections in
  `05-TOOL-BINDINGS-REGISTRY.md` (§2.116-§2.144) -- purely additive documentation, no code
  changes, no re-prioritization.
- Flipped campaign narrative coverage row SWC-045 `[~] partial -> [x] covered`
  (`Docs/architecture/demo/campaign narrative/06-FEATURE-COVERAGE.csv`) legalized by
  `TestCampaign narrativeRenderWalk_ADKToolBinding_GraphTraverse` +
  `_TemporalQuery` (`internal/adktools/shared/campaign narrative_render_walk_test.go`). Updated
  `17-CAMPAIGN_NARRATIVE-INTEGRATION.md` with an explicit honesty note that the coverage flip does not
  mean the interactive one-click story calls a live tool mid-playback (that remains the named
  `covered-interactive` terminal tier) and preserved the render-walk's own
  attention-routing-exclusion honesty note (WH-CAND-COLDBUILD-1).
- Stamped 6-field YAML frontmatter on all 17 docs read this run (zero prior frontmatter in the
  adktools corpus). Skipped `TODO.md` (content + stamp) -- ArchTodoAuditor-owned, dirty at
  dispatch with another agent's uncommitted prune; recorded in `.corpus-build/skips.jsonl`.
- Regenerated `33_corpus_context_index.json` (2752 docs scanned, 68 tagged, 338 frontmattered,
  266 features, 62 packages); the 15 reported discrepancies are pre-existing `cli/` corpus
  drift, unrelated to this run. Did not flip any `NERD_FEATURE` tag.
- Full learnings: see `references/journal.md` 2026-07-12 entry (WU-ADK-2).

## [Run log] `corpusbuild_mnemosyne_1783823808` (final reconcile: energy-organ boot wiring + evolve/formulate eval readiness) - 2026-07-12

**No skill-mechanism change.** Third Phase 6 doc-audit pass on the same run, closing three
shipped commits with no pre-existing critic-review or orchestrator-results file.

- Gate evidence: commit `7a65c04bcc` (F-024 energy-organ shadow boot construction --
  `core.CacheEnergyOrganConfig` decode + `registerEnergyOrganIfEnabled` boot call site),
  `81966df1dd` (mnemosyne registered as a `/nerd-evolve` TrustWord hot-read perf target,
  baseline 383.9ns/3 allocs/op node-hit), `b4db4ed1e2` (mnemosyne registered as a
  `/mangle-programming` drift-detection-quality target via `scanDriftSignals` extraction +
  `AuditCohortOnce`, baseline recall 0.5000/precision 1.0000/f1 0.6667). None of the three had
  a matching `.corpus-build/reviews`/`results` file at audit time, so the auditor independently
  reproduced `go build`/`go vet` clean on `internal/app/server`, `internal/core`,
  `internal/consolidation`, `internal/attention-routing`, plus targeted `go test -race` (`-run
  EnergyOrgan`, `-run Drift`, `-bench BenchmarkTrustWordHotRead$ -run ^$`) before citing it.
- Updated mnemosyne §3 energy-organ row to the new boot-construction facts while explicitly
  preserving the "still dark by shipped config" honesty clause (every `.nerd/config.json and internal/config` still
  ships `enabled: false`, empty carrier -- nothing constructs by default); added a new §3 row
  "Optimization-pipeline readiness". Updated `20-TELEMETRY-OBSERVABILITY.md` and
  `13-DRIFT-AUDITOR-STALENESS.md`. Synced `Docs/architecture/consolidation/IMPLEMENTED_SPEC.md`'s
  drift-audit-task row (seam-ownership rule) and stamped 6-field frontmatter on that file
  (previously had none).
- Did NOT flip any `NERD_FEATURE` tag plane (no tagged surface touched) and did NOT touch
  `campaign/`/the ladder -- Rung 7 residency-influence activation stays a human campaign act,
  honestly OPEN.
- Regenerated `33_corpus_context_index.json` (2752 docs scanned, 68 tagged, 320 frontmattered
  [+1], 266 features [unchanged]); `--check` confirms idempotent.
- Full learnings: see `references/journal.md` third 2026-07-12 entry (final reconcile).

## [Run log] `corpusbuild_mnemosyne_1783823808` (wiring-fix wave + J1 weave) - 2026-07-12

**No skill-mechanism change.** Second Phase 6 doc-audit pass on the same run, closing a
wiring-fix wave and the run's J1 (campaign narrative weave) obligation.

- Gate evidence: commit `6b7029cf6b` (F-027 scope-envelope cap completeness across
  `CrossCandidate`/`CrossCandidateMetered`/`AuthorizedCrossCandidate` + ADK toolset
  completeness-guard fix, 5 new live-provider tests), `fbcf97ac8a` (A24a mnemosyne
  integrity observation collector wired into the Jules remediation loop),
  `147144ec1a` (gatekeeper `-race`-clean), `64fb52b138` (H2: 24 shipped
  `NERD_FEATURE` tags), `15a1153bc9` (A22 system-corpus tool coverage). All
  `go build`/`go test -race`/`go vet` exit 0.
- Flipped mnemosyne §3 scope-envelope row to cite all three crossing entry points;
  added a new §3 row for the integrity collector. Added
  `Docs/architecture/testing/IMPLEMENTED_SPEC.md` §8.8 (matching the §8.5-8.7
  closed-collector convention) and extended
  `Docs/architecture/mnemosyne/20-TELEMETRY-OBSERVABILITY.md` §5. Left
  `17-CONSTITUTIONAL-SAFETY.md` and `gatekeeper/IMPLEMENTED_SPEC.md` untouched (no new
  permissions; gatekeeper's legacy spec predates the mnemosyne absorption and never
  enumerated the crossing surface).
- J1 campaign narrative weave: added 15 new honest `mnemosyne` coverage rows (SWC-201..215;
  1 partial, 14 planned) against 24 shipped H2 feature tags that had accumulated with
  only one prior coverage row (SWC-194). None inflated to `covered` -- no render-walk
  exists yet for any of the 15.
- Stamped 6-field frontmatter on `Docs/architecture/testing/IMPLEMENTED_SPEC.md`.
  Regenerated `33_corpus_context_index.json` (2751 docs scanned, 68 tagged, 319
  frontmattered, 266 features, 62 packages); remaining reported discrepancies are
  pre-existing CLI-corpus tag drift, unrelated to this pass.
- Full learnings: see `references/journal.md` second 2026-07-12 entry (wiring-fix
  wave + J1 campaign narrative weave).

## [Run log] `corpusbuild_mnemosyne_1783823808` - 2026-07-12

**No skill-mechanism change.** Records a completed run for the journal's self-improvement
mechanism.

- Subsystem: mnemosyne (Rung 7/8 machinery). 26 work units + 2 Phase-5 wiring workers.
  Gate evidence: all `WU-*_result.md` COMPLETE with `-race` green + vet clean (OOM-zone
  handlers Docker-verified golang:1.26.4); wiring disposition `mnemosyne_wiring.json`
  PASS=22, WU-027 routing H2 frontmatter + J1 campaign narrative to Phase 6 doc-auditor.
- Phase 6 doc-audit: flipped mnemosyne §3 Rung-7 energy-organ and Rung-8
  channel-manufacture/portfolio/continuity rows from "Not Implemented 0%" to shipped-
  machinery (energy organ honestly held as "machinery; live boot awaits carrier-seal"),
  added 6 new §3 rows (scope-envelope, edge-quarantine, dream-branch snapshot-diff, layout
  co-location, authorship gates+ADK/graphNERD, graphCAD membrane/metabolism REST+tools),
  completed kill-genesis 70%->100% and durable-boot-receipt call-site wiring, extended the
  telemetry row. Synced consolidation §3 (+4 rows, same-change seam rule). Flipped campaign narrative
  SWC-194 planned->partial (not inflated to covered; live-route pack verifier pending).
  Stamped 6-field frontmatter on 39 mnemosyne docs; regenerated
  `33_corpus_context_index.json` (2751 docs scanned, 55 tagged, 318 frontmattered).
- NERD_FEATURE H2-tag application routed to `corpus_feature_tagger` (register-integrity
  boundary; this run's features lack clean canonical numbered owner docs).
- Full learnings: see `references/journal.md` 2026-07-12 entry.

## [Run log] `corpusbuild_graphcad_20260709` - 2026-07-10

**No skill-mechanism change.** This entry records a completed run for the journal's
self-improvement mechanism (a run with no surprises is still a data point).

- Subsystem: graphcad. 40 work units, 5 DAG levels, FOCUS_BUILD decision. Critic APPROVED
  (6/6 dimensions), one condition (COND-1 Docker gate) cleared mid-Phase-6 via commit
  `23029ee8e`.
- Phase 6 doc-audit reconciled `IMPLEMENTED_SPEC.md` §3, flipped 12 `TODO.md` rows with
  cited evidence, authored 19 new corpus docs (ADR-006 absorption-phasing, 8 per-workspace
  renderer deep-dives, 3 top-level reference specs, 6 cross-cutting audit/envelope docs),
  corrected 6 external-subsystem docs' stale GraphX-retirement citations, regenerated
  `33_corpus_context_index.json` (2634 docs scanned, 55 tagged, 279 frontmattered).
- Full learnings: see `references/journal.md` 2026-07-10 entry.

## [1.0.0] - 2026-03-22

### Initial Release

**Purpose**: Spec-driven implementation engine that reads an codeNERD architecture corpus, audits existing code against the spec, classifies gaps using a 3-input judgment model (alignment, structural debt, vision drift), and dispatches parallel sonnet workers to build missing code with dependency-aware DAG ordering.

**Design decisions**:
- Dual-mode (realize vs build) to avoid re-running expensive opus judgment when plan exists
- Phase -1 Vision Anchor ensures all builders align with product north star
- 3-input gap judgment (alignment + structural debt + vision drift) replaces naive percentage-based decisions
- Reserved-file pattern prevents merge conflicts in high-traffic files (registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main), config YAML)
- Dynamic 102-surface wiring checklist discovered at runtime, not frozen in a static list
- Section 12 of IMPLEMENTED_SPEC consumed as pre-computed priority input, not re-derived by opus
- Anti-hallucination gate: post-build grep verification for exact spec interface/function names
- Fast-fail mechanism: FAIL in any level worker pauses remaining workers
- Phase 6 (evolve handoff) deferred to v1.1 per sage wisdom: defer most speculative feature

**Interrogation history**:
- Round 1: 2 criticals (missing Vision Anchor phase, wiring checklist frozen at 15 surfaces), 10 important items
- Round 2: Both criticals resolved. 10 of 11 important items resolved. Conditional READY.
- Full interrogation record: `.claude/skills/skill-creator/interrogations/2026-03-22_skill-corpus-build.md`

**Agents required** (4 new):
- corpus-reader (sonnet): Parse corpus + audit code in single pass
- corpus-judge (opus): 3-input gap judgment + build plan
- corpus-builder (sonnet): Build single work unit from spec
- corpus-wiring-auditor (sonnet): Verify 102 integration surfaces

**Agents reused** (6 existing):
- go-architect / test-forge unit, test-forge integration, cross-system-test-architect, corpus-comms-plumber, requirements-interrogator, prompt-architect / corpus-doc-auditor

**Known limitations**:
- Phase 6 (optional evolve handoff) deferred to v1.1
- Registration intent file schema not yet formalized (builders create JSON intents, wiring worker consumes)
- Structural debt and vision drift thresholds are opus judgment calls, not mechanically reproducible
- Scripts (subsystem_audit.py, build_dag.py, verify_surfaces.py, cost_estimate.py) referenced but not yet implemented
