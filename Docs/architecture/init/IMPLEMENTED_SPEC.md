# init — Implemented Spec (Deep-Dive)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go  
> Package: `internal/init`  
> CLI: `nerd init`, `nerd scan` (`cmd/nerd/cmd_init_scan.go`)  
> Scale: **16** non-test Go files ≈ **8.7k** lines; **7** test files; **1** debug `.mg` artifact  

## 1. Overview

`internal/init` implements **codeNERD cold-start**: the first-time (and force-upgrade) materialization of a workspace’s `.nerd/` control plane. It sits **before** the normal OODA fact-flow. Its job is to leave behind durable structure so that chat boot, kernel loads, specialist profiles, and prompt JIT have something real to bind.

Unlike a thin “mkdir .nerd” scaffold, init:

1. Scans the tree with `world.Scanner` and batch-loads topology facts into a temporary kernel.
2. Detects language, dependencies (direct + lockfile transitive), build system, project type, entry points.
3. Writes `profile.json` + Mangle `profile.mg`.
4. Seeds knowledge databases (shared, Type-3 agents, core coder/reviewer/tester, codebase, campaign).
5. Optionally runs LLM strategic analysis (with Gemini grounding when available).
6. Registers agent profiles on `ShardManager`, syncs prompt YAML into KBs, validates DBs, prints operator guidance.

### Key characteristics

| Property | Value |
|----------|-------|
| Orchestrator | `Initializer.Initialize` — **22 phases** |
| Detection | File/lock heuristics (monorepo globs 2 levels) |
| Research | Modular `tools/research` (Context7); no domain Researcher shard |
| LLM role | Strategic knowledge + optional JIT phase prompts; not executive |
| Progress | Optional `ProgressChan` + `ETATracker` |
| Default timeout (config) | 30 minutes (`DefaultInitConfig`) |
| CLI timeout | Global CLI timeout context |
| Architecture slogan applied | Describe project into facts/atoms; logic later decides actions |

### High-level control flow

```
nerd init
   │
   ├─ cleanup-backups only? ──► CleanupBackups ──► exit
   ├─ already init && !force ──► message ──► exit
   └─ NewInitializer ──► Initialize (22 phases) ──► Validate ──► summary
```

Post-init runtime fact-flow (elsewhere):

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards (profiles from agents.json / DefineProfile)
  → articulation
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Directory + default config | **Implemented** | `scanner.go` `createDirectoryStructure` / `createDefaultConfig` |
| Schema migration | **Implemented** | `store.MigrateAllAgentDBs` on existing `.nerd` |
| World scan + LoadFacts | **Implemented** | Batch path critical for large trees |
| File detectors | **Implemented** | Language, deps, entry, build, project type |
| Profile + profile.mg | **Implemented** | Escape/sanitize hygiene |
| Analysis phase content | **Stub** | JIT session owns deep research |
| Prompt atoms + corpus.db | **Implemented** | Dual-store caveat for JIT visibility |
| Type-3 recommendation | **Implemented** | Heuristic switches, not LLM |
| Shared KB | **Implemented** | Base atoms only (research deferred) |
| Parallel agent KB create | **Implemented** | Upgrade mode + inheritance |
| Agent prompts.yaml | **Implemented** | Template with identity/methodology/domain |
| ShardManager registration | **Implemented** | `DefineProfile` |
| Codebase / campaign KBs | **Implemented** | `profile.go` |
| Core shard KBs | **Implemented** | coder/reviewer/tester |
| Strategic knowledge | **Implemented** | Requires LLM; soft-fail |
| Doc ingestion campaign | **Implemented** | `strategic_documents.go` helpers |
| Static tools JSON | **Implemented** | Language/framework catalogs |
| Dynamic tool generation | **Stub** | JIT/Ouroboros message |
| Preferences + session seed | **Implemented** | Hint-driven prefs |
| Prompt reload sync | **Implemented** | `prompt.ReloadAllPrompts` |
| Validation + backups | **Implemented** | Schema v4, RequiredTables |
| Interactive selection | **Library** | Not wired in `runInit` |
| Type U agents | **Library** | Parse/validate; CLI merge incomplete |
| ETA / progress | **Implemented** | Channel optional |
| Gemini grounding | **Implemented** | When client is Gemini |

**Overall:** production cold-start package with intentional JIT-era stubs — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/init/
  initializer.go           # types, NewInitializer, Initialize, summary
  scanner.go               # detection + directories + default config
  scanner_dependencies.go  # lockfile / version parsers
  profile.go               # profile, facts, prefs, sessions, KBs, prompt DB
  agents.go                # recommend + create Type-3 + prompts.yaml
  agents_knowledge.go      # per-agent KB + research parse
  agents_registration.go   # shard profiles, core KBs, tool-gen stub
  shared_kb.go             # core_concepts.db
  tools.go                 # tool definition catalog
  interactive.go           # agent curation UX
  typeu_agents.go          # user-defined agents
  jit_integration.go       # init-phase JIT prompts
  strategic_knowledge.go   # LLM strategic analysis
  strategic_documents.go   # doc pipeline + mangle status facts
  validation.go            # post-init DB validation
  eta_tracker.go           # phase ETA
  *_test.go                # unit/coverage suites
  debug_program_ERROR.mg   # crash dump artifact (not product schema)
```

### 3.2 Largest non-test sources

| Path | ~Lines | Purpose |
|------|------:|---------|
| `initializer.go` | 1128 | Orchestration + public types |
| `scanner.go` | 1034 | Detection + FS scaffold |
| `profile.go` | 956 | Durable identity + KBs |
| `strategic_knowledge.go` | 742 | Strategic LLM path |
| `agents.go` | 728 | Agent portfolio |
| `strategic_documents.go` | 600 | Doc campaign |
| `tools.go` | 566 | Tool catalog |
| `agents_registration.go` | 521 | Registration + core KBs |
| `scanner_dependencies.go` | 504 | Transitive deps |
| `interactive.go` | 446 | Interactive curation |
| `validation.go` | 376 | Validate / cleanup |
| `agents_knowledge.go` | 369 | KB hydration |
| `jit_integration.go` | 261 | JIT atoms |
| `shared_kb.go` | 195 | Shared pool |
| `typeu_agents.go` | 178 | Type U |
| `eta_tracker.go` | 158 | ETA |

---

## 4. Public surface (summary)

Full tables: [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md).

**Must-know exports for integrators:**

- `NewInitializer` / `Initialize` / `Close`
- `DefaultInitConfig`
- `IsInitialized`, `LoadProjectProfile`, session load/save helpers
- `ValidateAllAgentDBs`, `CleanupBackups`
- `InteractiveAgentSelection`, Type U parsers
- `GenerateToolsForProject` / `SaveToolsToFile`
- `CreateSharedKnowledgePool`

---

## 5. Deep dive — `Initialize` phases

### 5.1 Construction (`NewInitializer`)

```
AutoDetectContext7APIKey if empty
core.NewRealKernel + SetWorkspace
world.NewScanner
ETATracker(22)
ShardManager new or injected
if LLMClient: SetLLMClient + research.NewGroundingHelper + EnableGoogleSearch when available
```

Embedding engine is **lazy** via `ensureEmbeddingEngine` (reads workspace or global `config.json`).

### 5.2 Phase table (behavioral)

| # | Name | Criticality | Failure mode |
|---|------|-------------|--------------|
| 1 | setup | Low | Warning if system shards fail |
| 2 | migration | Medium | Warning |
| 3 | directory | **Hard** | Error on mkdir; error on embedding after DB |
| 4 | scanning | Medium | Warning |
| 5 | analysis | None | Stub only |
| 6 | profile | High | Warning if save fails (marker missing) |
| 7 | facts | Medium | Warning |
| 8 | prompt_atoms | Low | Warning |
| 9 | prompt_db | Medium | Warning |
| 10 | agents | Low | Always produces list (maybe generic) |
| 11 | shared_kb | Medium | Warning |
| 12 | kb_creation | High | Per-agent soft fail |
| 13 | codebase_kb | Medium | Warning; strategic soft |
| 14 | core_shards_kb | Medium | Warning |
| 15 | campaign_kb | Low | Warning |
| 16 | tool_generation | None | Stub |
| 17 | preferences | Medium | Warning |
| 18 | session | Medium | Warning |
| 19 | tools | Low | Warning |
| 20 | registry | Medium | Warning |
| 21 | prompt_sync | Medium | Warning |
| 22 | complete | — | Summary + validation |

### 5.3 Directory tree created

```
.nerd/
  shards/          # agent + shared + core knowledge DBs
  sessions/        # conversation history
  cache/
  mangle/          # extensions.mg, policy_overrides.mg
  prompts/         # corpus.db
  tools/           # available_tools.json + autopoiesis subdirs
  agents/          # {name}/prompts.yaml
  campaigns/
  config.json      # if missing
  .gitignore
  knowledge.db     # after LocalStore open
  northstar_knowledge.db
  profile.json
  profile.mg
  preferences.json
  session.json
  agents.json
```

### 5.4 Detection details

**Language files** (root then `*` then `*/*`): go.mod, Cargo.toml, package.json→typescript, pyproject/requirements/setup.py, pom/gradle, kotlin gradle, csproj, mix.exs, Gemfile.

**Go dep map** includes rod, chromedp, playwright, mangle, openai, anthropic, bubbletea, cobra, gin/echo/fiber, gorm/sqlx/sql, arangodb, adk, a2a, etc.

**Node key deps** via package.json + lock parsers; **Rust/Python** lock parsers for notable crates/packages.

**Entry points**: language-specific path heuristics + content checks (`func main`, etc.).

**Project type**: application vs library vs hybrid (`detectProjectType`).

### 5.5 Agent recommendation policy

1. Language specialist (GoExpert, PythonExpert, TSExpert, RustExpert, AndroidExpert for Kotlin).
2. Framework specialists (WebAPIExpert, FrontendExpert).
3. Dependency specialists (Rod, BrowserAutomation, Mangle, LLM, BubbleTea, Cobra, Database, Arango, ADK, A2A, …).
4. **Only if empty:** SecurityAuditor + TestArchitect fallbacks.
5. Attach tools via `GetToolsForAgentType`.

### 5.6 Knowledge hydration

```
CreateSharedKnowledgePool → BaseSharedAtoms → core_concepts.db
createType3Agents:
  load registry → upgrade flags
  parallel workers:
    createAgentKnowledgeBase:
      inherit shared (new only)
      base identity/topics atoms
      context7_fetch per topic (unless SkipResearch)
    generateAgentPromptsYAML
registerAgentsWithShardManager
```

Quality scores: thresholds on new atom counts (80/65/50) — heuristic only.

### 5.7 Strategic knowledge

Requires `LLMClient`. Builds codebase context from profile + scan, gathers docs (CLAUDE.md/README/docs prioritized), LLM filters relevance, requests structured JSON `StrategicKnowledge`, optionally Gemini grounded completion with tech doc URLs, persists atoms into `knowledge.db`.

### 5.8 Mangle outputs

`profile.mg` example predicates:

- `project_profile("id", "name", "desc").`
- `project_language(/go).`
- `project_framework(/…).`
- `project_architecture(/…).`
- `build_system(/…).`
- `architectural_pattern(/…).`
- `entry_point("path").`

Templates invite user Decls/rules under `.nerd/mangle/` without editing core policy packages.

### 5.9 Validation

`ValidateAllAgentDBs`:

- Required tables: knowledge_atoms, cold_storage, vectors, knowledge_graph
- Schema version vs `CurrentSchemaVersion = 4`
- content_hash population warnings
- MinAtomCount = 5 warning threshold
- Backup file discovery → `--cleanup-backups`

---

## 6. Integration map

### 6.1 CLI

See [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md). Primary entry `runInit` / `runScan` in `cmd/nerd/cmd_init_scan.go`.

### 6.2 Chat

Session persistence reuses `SessionState` / `ChatMessage`. Status commands report `IsInitialized`. Boot does **not** re-enter full Initialize.

### 6.3 Upstream packages

core, core/shards, world, store, prompt, embedding, config, tools/research, northstar, types, logging — detailed in [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md).

### 6.4 Fact-flow position

```
┌────────────┐     ┌──────────────┐     ┌─────────────────────┐
│  nerd init │────►│  .nerd/*     │────►│ chat boot / Cortex  │
└────────────┘     │ profile.mg   │     │ kernel + shards     │
                   │ agents.json  │     └──────────┬──────────┘
                   │ *knowledge.db│                │
                   └──────────────┘                ▼
                                        OODA + VirtualStore
```

---

## 7. JIT and north-star alignment

| North-star item | Init stance |
|-----------------|-------------|
| LLM creative center | Strategic docs + research atoms describe the project |
| Mangle executive | Facts generated for later; no action permission loop here |
| JIT atoms | `jit_integration` init phases; YAML agent prompts; corpus.db |
| Wiring before delete | Researcher/tool_generator removed with stubs/comments; interactive/Type U remain as APIs |

---

## 8. Testing posture

Broad pure-function coverage in `init_coverage_test.go` and focused suites. Full Initialize E2E depends on embedding + optional network — treat as integration. Commands: `go test ./internal/init/...`. Details: [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md).

---

## 9. Observability and failure modes

- Boot/store logs + stdout phases + optional progress/ETA.
- Soft-fail enrichment vs hard-fail substrate (directory/embedding).
- See [11-OBSERVABILITY.md](11-OBSERVABILITY.md), [12-FAILURE-MODES.md](12-FAILURE-MODES.md).

---

## 10. Gaps pointer

Authoritative gap matrix: [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

Top residual issues:

1. Interactive + Type U CLI wiring incomplete.
2. Tool generation stub.
3. Project prompt atoms may not reach JIT corpus.db.
4. Framework field often under-populated.
5. Quality scores naive.

---

## 11. Related corpora

| Area | Path |
|------|------|
| CLI surface | `Docs/architecture/cli/` |
| World scanner | `Docs/architecture/world/` |
| Store / migrations | `Docs/architecture/store/` |
| Prompt JIT | `Docs/architecture/prompt/` |
| Core kernel | `Docs/architecture/core/` |
| Config defaults | `Docs/architecture/config/` |
| Northstar | `Docs/architecture/northstar/` |

---

## 12. Operator quick reference

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
./nerd.exe init
./nerd.exe init --force
./nerd.exe scan
./nerd.exe init --cleanup-backups
go test ./internal/init/...
```

Environment:

| Var / file | Effect |
|------------|--------|
| `ZAI_API_KEY` / `--api-key` | LLM client for enrichment |
| `CONTEXT7_API_KEY` | Context7 research |
| `.nerd/config.json` | Provider, embedding, Gemini flags |

---

## 13. Changelog of understanding (corpus)

| Date | Note |
|------|------|
| 2026-07-13 | Full deep corpus from source; supersedes thin inventory stubs |

This document is the **flagship living spec** for `internal/init`. Prefer updating it when phase lists, artifacts, or wiring change.
