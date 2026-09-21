# init WIRING-AND-NOT-BUILT — what is reachable, what is dead, what is assumed

The old 19-file corpus described this package from stale claims. This file
replaces that entire dimension: every entry cites the code that proves it, at
commit `34634770970153e78c1e250fdab7abd888dcce6f` (2026-09-21).

## Wired and reachable from production

- `cmd/nerd/cmd_init_scan.go:214` and `cmd/nerd/chat/helpers_scan.go:65,244`
  build the `Initializer`; `Initialize` (`internal/init/initializer.go:451`)
  runs all 16 phases in order.
- Type-U merge and interactive curation both run in phase 6
  (`mergeTypeUAgents` then `curateAgents`,
  `internal/init/initializer.go:895`) — they are not library-only helpers.
- Per-agent research executes through `tools.NewRegistry` +
  `research.RegisterAll` with a 2-minute timeout
  (`internal/init/agents_knowledge.go:99`); shared-pool inheritance runs on
  every fresh agent KB (`:63`).
- `NewInitializer` always installs a non-nil `ShardManager`
  (`internal/init/initializer.go:353`), so the nil guard in
  `registerAgentsWithShardManager`
  (`internal/init/agents_registration.go:42`) never fires on this path —
  registration is live.
- `LoadToolsFromFile` (`internal/init/tools.go:415`) is read by
  `internal/system/factory.go:1441`; the phase-10 catalog is consumed, not
  write-only.
- `determineRequiredTools` (`internal/init/agents_registration.go:330`) runs
  twice: into phase-5 facts (`internal/init/profile.go:131`) and into phase 7e
  (`internal/init/agents_registration.go:260`).

## Exists but nothing calls

- `filterTopicsNeedingResearch` (`internal/init/agents_knowledge.go:225`):
  the only call sites are its own tests
  (`internal/init/agents_knowledge_helpers_test.go:67,81`).
  `createAgentKnowledgeBase` researches **every** topic in a loop
  (`internal/init/agents_knowledge.go:106`) without consulting it — the
  "skip already-covered topics" optimization is written, tested, and bypassed.
- `convertStoreAtomsToInitAtoms` (`internal/init/agents_knowledge.go:292`):
  likewise test-only (`internal/init/agents_knowledge_helpers_test.go:91`).
- `ProcessDocumentsWithTracking`
  (`internal/init/strategic_documents.go:128`),
  `extractAndStoreDocKnowledge` (`:245`), and `SynthesizeFromStoredAtoms`
  (`:471`) have no call sites anywhere in `internal/init`. Phase 7b builds
  strategic knowledge through `generateStrategicKnowledge` /
  `PersistStrategicKnowledge` (`internal/init/strategic_knowledge.go:69`,
  `:639`) instead — the whole document-ingestion path in
  `strategic_documents.go` is unreachable from `Initialize`.

## Named like it builds, but does not

- **Phase 7e "Generating Project-Specific Tools" generates nothing.**
  `generateProjectTools` (`internal/init/agents_registration.go:259`)
  converts needs to `missing_tool_for` facts (`projectToolNeedFacts`, `:280`)
  and returns empty, so phase 7e always lands on the "No tools generated"
  branch (`internal/init/initializer.go:1011`). The phase title and its
  "Generated %d tools" branch (`:1005`) describe behavior no input can
  trigger. Recording needs-as-facts for later on-demand generation is the
  actual design; the name is the lie.
- **Phase 10 drops dependencies.** `detectedTech` is language plus framework
  only (`internal/init/initializer.go:1051`), so `GetDependencyTools`
  (`internal/init/tools.go:346`) results never reach
  `tools/available_tools.json` even though the catalog supports them.
- **Tool coverage lags the agent table.** Agent names come from
  `determineRequiredAgents` (`internal/init/agents.go:380`) while tools come
  from the separate switch in `GetToolsForAgentType`
  (`internal/init/tools.go:452`); a recommended name with no case keeps the
  empty values assigned at `internal/init/agents.go:626`. The newer
  dependency-driven experts (Arango, ADK, A2A, BrowserAutomation, LLM,
  Mangle, BubbleTea, Cobra, Android) are the names at risk — check the switch
  before trusting any of their tool lists.

## What the design assumes that the code does not do

- The TTY gate assumes "character devices on both ends" means "a human is
  present". The code itself admits `/dev/null` passes the gate and that
  pty-allocating wrappers (`docker run -t`, `script -c`) pass it while never
  sending EOF — which used to hang init forever
  (`internal/init/agents_curation.go:161`). The safety property now lives on
  the read timeout, not the gate; nothing downstream may treat the gate as
  proof of interactivity.
- `QualityScore`/`QualityRating` assume atom count measures knowledge
  quality. The code labels them a legacy population proxy
  (`internal/init/agents_knowledge.go:132`) — read them as "did research
  return text", not as evaluation.
- Phase 5b/5c and phase 12 assume a prompt-atom pipeline
  (`runPhase5bPromptAtoms`, `runPhase5cPromptDB`, `runPhase12PromptSync`,
  `internal/init/initializer.go:858`, `:868`, `:1087`) whose store and sync
  semantics are defined in `internal/init/profile.go:1007` and
  `internal/init/jit_integration.go`; this corpus pins their existence and
  order, not their effect — that handoff is JIT territory.

## What the old corpus got wrong (why this rewrite exists)

- It described research as key-gated and shared-pool-level
  (old `01-VISION.md`). The code runs per-agent Context7 research gated only
  on `--skip-research` (`internal/init/agents_knowledge.go:95`) and stores
  base atoms only in the shared pool (`internal/init/shared_kb.go:128`).
- It listed interactive curation and Type-U handling as partially wired (old
  `00-ALIGNMENT-VISION-REVIEW.md`). Both run unconditionally in phase 6
  (`internal/init/initializer.go:895`).
- Its INDEX row claimed "16 go / 7 tests"; the package holds 17 non-test
  `.go` files and 20 `_test.go` files. The INDEX row is corrected with this
  rewrite.

---
Verified 2026-09-21 against commit `34634770970153e78c1e250fdab7abd888dcce6f`.
