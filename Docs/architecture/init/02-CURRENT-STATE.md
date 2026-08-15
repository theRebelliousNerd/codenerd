# init — Current State

> Last verified: 2026-08-15
> Package: `internal/init/`

## Inventory summary

| Kind | Count | Notes |
|------|------:|-------|
| Non-test `.go` | **16** | Core implementation |
| Test `.go` | **8** | Unit + coverage suites, including initializer truth coverage |
| `.mg` in package | **0** | Kernel fault dumps go to `.nerd/debug/` and are gitignored; a test pins that the package tree stays free of them |
| Package docs (`README.md` / `agents.md`) | **1** | `internal/init/agents.md` carries scoped implementation invariants |

### Non-test sources (approx line counts)

| Path | ~Lines | Role |
|------|------:|------|
| `initializer.go` | 1128 | Types, `NewInitializer`, `Initialize` 22-phase orchestration, summary/validation |
| `scanner.go` | 1034 | Language/deps/entry/build/project-type detection, directory structure, default config |
| `profile.go` | 956 | Profile/facts/prefs/session I/O, project atoms, codebase/campaign KBs, prompt DB |
| `strategic_knowledge.go` | 742 | LLM strategic “soul” analysis + persist |
| `agents.go` | 728 | Agent recommendation, parallel Type-3 creation, prompts.yaml |
| `strategic_documents.go` | 600 | Doc discovery, relevance, atom extraction, synthesis campaign state |
| `tools.go` | 566 | Static tool catalog by language/framework/deps; save/load JSON |
| `agents_registration.go` | 521 | ShardManager registration, core shard KBs, tool-gen stub |
| `scanner_dependencies.go` | 504 | go.mod / lockfile parsers (npm, yarn, pnpm, cargo, pipfile, poetry) |
| `interactive.go` | 446 | Interactive agent selection UX |
| `validation.go` | 376 | Post-init agent DB validation + backup cleanup |
| `agents_knowledge.go` | 369 | Agent KB create/upgrade, Context7 research parse, base atoms |
| `jit_integration.go` | 261 | Init-phase JIT compiler + fallback prompts |
| `shared_kb.go` | 195 | Shared `core_concepts.db` pool + inherit |
| `typeu_agents.go` | 178 | `--define-agent` parse/validate (Type U) |
| `agents_curation.go` | 170 | Type U merge, interactive curation gate, selection persistence |
| `eta_tracker.go` | 158 | Phase ETA tracking + progress helpers |

### Tests

| Path | Focus |
|------|-------|
| `init_test.go` | Integration-ish init paths |
| `init_coverage_test.go` | Broad pure-function matrix (sanitize, parsers, prefs, sessions, tools, ETA, facts) |
| `scanner_test.go` | Scanner detection |
| `scanner_dependencies_test.go` | Lockfile parsers |
| `agents_knowledge_helpers_test.go` | Atom hashing, research parse, base atoms |
| `interactive_display_test.go` | Display helpers, defaults |
| `typeu_coverage_test.go` | Type U flags + directory structure |
| `initializer_truth_test.go` | Overlay preservation, timeout, config, enrichment metrics, result truth |

## What works today (behavioral)

1. **Full `Initialize` pipeline** with 22 named phases and non-blocking progress channel.
2. **`.nerd/` tree** including shards, sessions, mangle overlays, prompts, tools subdirs, agents, campaigns.
3. **World scan** via `world.NewScanner().ScanDirectory` + batch `kernel.LoadFacts(scanResult.ToFacts())`.
4. **File-based profile** (language, deps, build system details, project type, entry points).
5. **`profile.mg` generation** with sanitized name constants and escaped strings.
6. **Shared + agent + core-shard + codebase + campaign knowledge bases** via `store.LocalStore`.
7. **Strategic knowledge** when LLM client present (optional Gemini grounding).
8. **Agent registry** JSON + `ShardManager.DefineProfile` for created agents.
9. **Prompt corpus DB** init + `prompt.ReloadAllPrompts` sync.
10. **Validation** of agent DBs (tables, schema version, hashes, min atoms).
11. **Session persistence helpers** used heavily by chat (`SessionState`, history).
12. **Static tool definitions** written to `.nerd/tools/available_tools.json`.
13. **Machine-readable completion truth** via required failures and per-provider LLM outcome metrics.
14. **Force-init overlay preservation** via atomic create-if-absent templates.

## What is intentionally partial

| Area | State in code |
|------|----------------|
| Phase 3 “analysis” | Stubbed: message that JIT session handles research |
| Researcher domain shard | Removed; comments point to JIT clean loop |
| `generateProjectTools` | Stub: logs would-be tools, returns empty |
| Interactive selection | Wired: phase 6 `curateAgents`, terminal-gated, `--no-interactive` opts out |
| Type U agents | Wired: `nerd init --define-agent` parses into `InitConfig.TypeUAgents`, merged in phase 6 |
| Deep research semantics | Legacy scores are labeled atom-count population proxies, not formal evaluation |

## Artifacts written under `.nerd/`

| Artifact | Producer |
|----------|----------|
| `config.json` | `createDefaultConfig` if missing |
| `profile.json` | `saveProfile` |
| `profile.mg` | `generateFactsFile` |
| `preferences.json` | `savePreferences` (merge-preserving; never truncates ux/agent-selection blocks) |
| `session.json` | `initSessionState` / chat updates |
| `sessions/*.json` | session history APIs |
| `knowledge.db` | LocalStore + strategic atoms |
| `northstar_knowledge.db` | `northstar.NewStore` schema create |
| `agents.json` | agent registry |
| `agents/{name}/prompts.yaml` | Type-3 prompt templates |
| `shards/*.db` | agent/core/shared KBs |
| `prompts/corpus.db` | prompt database init |
| `tools/available_tools.json` | static catalog |
| `mangle/extensions.mg`, `policy_overrides.mg` | create-if-absent user-owned templates |
| `doc_ingestion_state.json` | strategic doc campaign (when used) |

## Hotspots

- **`Initialize` length**: single method owns phase sequencing — high change risk.
- **Detection tables**: duplicated conceptual knowledge across `determineRequiredAgents`, `GetLanguageTools`, dep maps.
- **Embedding hard requirement** for KB path: `ensureEmbeddingEngine` failure aborts init after DB create.
- **Parallel agent creation** (`createAgentsParallel`): concurrency + per-agent DB files.
