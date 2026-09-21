# init INTERNALS — how the cold-start pipeline decides and stores

Companion to README.md (what init creates) and WIRING-AND-NOT-BUILT.md
(what is wired vs. dead). Everything below is read from the phase functions
and helpers cited; behaviors whose wiring is partial or absent are named in
the companion file, not here.

## 1. Detection: profile first, everything else follows

The scan (phase 2, `runPhase2Scanning`, `internal/init/initializer.go:790`)
feeds `ProjectProfile` (`internal/init/initializer.go:130`, with per-module
breakdown in `ModuleProfile`, `:167`, and `DependencyInfo`, `:187`), which
carries `Language`, `Framework`, and `Dependencies`. Dependency names come
from lockfile parsers in `internal/init/scanner_dependencies.go` (go.sum,
yarn/pnpm, cargo, pipfile/poetry, package-lock/package.json — each pinned by
its `TestParse*` case in `internal/init/init_coverage_test.go`). Every later
decision — which agents, which tools, which facts — reads the profile, never
the raw tree.

## 2. Agent recommendation is a fixed switch table

`determineRequiredAgents` (`internal/init/agents.go:380`) appends agents by
three exact matches:

- **Language** (`:384`): go→`GoExpert`, python→`PythonExpert`,
  typescript/javascript→`TSExpert`, rust→`RustExpert`, kotlin→`AndroidExpert`.
- **Framework** (`:443`): gin/echo/fiber→`WebAPIExpert`,
  react/nextjs/vue→`FrontendExpert`.
- **Dependency names** (`:467`): rod→`RodExpert`,
  chromedp/puppeteer/playwright→`BrowserAutomationExpert`, mangle→`MangleExpert`,
  openai/anthropic→`LLMIntegrationExpert`, bubbletea→`BubbleTeaExpert`,
  cobra→`CobraExpert`, gorm/sqlx/sql/prisma/typeorm→`DatabaseExpert`,
  arangodb→`ArangoExpert`, adk→`ADKExpert`, a2a→`A2AExpert`. The `sql` match is
  a literal dependency name, not SQL detection in general (`:548`).

If nothing matched, two generic agents are added (`SecurityAuditor`,
`TestArchitect`) — and only then (`:599`). Finally every agent gets
`Tools`/`ToolPreferences` from `GetToolsForAgentType`
(`internal/init/agents.go:625`, switch in `internal/init/tools.go:452`).

## 3. Curation happens in phase 6, before any KB is built

`runPhase6AnalyzeAgents` (`internal/init/initializer.go:881`) runs three
steps in order (`:885`): recommend, `mergeTypeUAgents`
(`internal/init/agents_curation.go:30`) — a `--define-agent` name that
collides with a detected agent **replaces** it (`:43`) because both would
fight over one KB file — then `curateAgents` (`:71`), then records the result
(`:897`). Curation prompts only when the run opted into `Interactive` **and**
a real terminal is present (`resolveInteractiveConfig`,
`internal/init/agents_curation.go:143`; both stdin and stdout must be
character devices, `stdioIsTerminal`, `:174`). Anything else — non-interactive
runs, no TTY, read failure, empty selection, a saved auto-accept — degrades
to the recommended set with a warning, never a failed init (`:72`, `:97`).
Kept/rejected names persist to `preferences.json` (`persistAgentSelection`,
`:115`) so a later `--force` run honors the same choices.

## 4. One agent KB holds three layers

`createAgentKnowledgeBase` (`internal/init/agents_knowledge.go:33`):

1. **Inheritance** (fresh KBs only): copies the shared pool via
   `InheritSharedKnowledge`, and a failed inherit only logs (`:60`).
   Hashes are re-fetched after inheriting so shared atoms dedup correctly
   (`:67`). Upgrade mode instead loads existing atoms for dedup
   (`buildAtomHashSet`, `:54`).
2. **Base atoms** from `generateBaseKnowledgeAtoms` (`:76`), appended with
   content-hash dedup (`appendKnowledgeAtom`, `:78`).
3. **Research** unless `--skip-research`: every topic runs through
   `context7_fetch` on a throwaway tool registry with a 2-minute deadline,
   results over 100 chars are chunked into atoms by `parseResearchResult`
   (`:95`). Research failures log and continue per topic (`:109`).

`QualityScore`/`QualityRating` are atom-count thresholds explicitly labeled a
"legacy population proxy" (`:132`); `SkipResearch` forces the bottom bucket
(`:143`).

Agents are built concurrently by a worker pool in `createAgentsParallel`
(`internal/init/agents.go:797`); existing registry entries are reused
(`loadExistingAgentRegistry`, `:637`), results are registered with the shard
manager (`registerAgentsWithShardManager`,
`internal/init/agents_registration.go:42`), and the registry is saved
(`saveAgentRegistry`, `:73`).

## 5. Prompts prefer curated LLM content, fall back to static

`generateAgentPromptsYAMLWithContext` (`internal/init/agents.go:78`) tries
LLM-generated methodology/domain content first (`:106`, `:115`, via
`generateAgentAtomContent`, `:299`), validates the YAML
(`validatePromptsYAML`, `:130`), and on any failure rebuilds from the static
template (`staticMethodologyContent`/`staticDomainContent`, `:132`) — so a nil
or hostile LLM still yields a parseable file. Generation honors the provider
and parent-context deadlines, and curated prompts survive even when no
knowledge DB exists (both pinned in
`internal/init/agent_generation_contract_test.go:29,69`).

## 6. The shared pool is base atoms only, by removal of research

`CreateSharedKnowledgePool` (`internal/init/shared_kb.go:105`) stores exactly
`BaseSharedAtoms` (`:128`) — shared principles, reusable agent configs,
prompting strategies. `SharedKnowledgeTopics` (`:18`) names what *would* be
researched, but each topic is only logged for a future JIT pass (`:136`).
Research lives in per-agent KB creation (section 4), not here.

## 7. The static tool catalog is language + framework only

Phase 10 (`runPhase10Tools`, `internal/init/initializer.go:1047`) passes
`GenerateToolsForProject` just the language plus the framework when known
(`:1051`) and saves the result to `tools/available_tools.json` (`:1056`).
The `ToolDefinition` shape (`IsMCPTool` in `internal/init/tools.go:43`)
supports dependency- and framework-specific entries, but on this path the
dependency list never arrives — see WIRING-AND-NOT-BUILT.md.

---
Verified 2026-09-21 against commit `34634770970153e78c1e250fdab7abd888dcce6f`.
