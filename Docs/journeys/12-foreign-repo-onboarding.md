# Journey 12 — Foreign Repository Onboarding & First-Boot TUI Orientation

A horizontal end-to-end journey tracking what happens when a developer runs `nerd init` in an arbitrary foreign repository, followed by launching `nerd chat` (or bare `nerd`) for the first interactive coding session.

Every stage below contrasts **Shipped Today** (with exact `path:line` citations verified against commit `e056692c`) against the **Target State** (the incoming architecture governed by `Docs/architecture/orient/` and `Docs/architecture/workspace/`).

---

## 1. Journey Overview & Measuring Stick

In standard coding agent workflows, onboarding a foreign codebase is fraught with friction: the agent greps blindly across irrelevant virtual environments, truncates critical architecture documents at arbitrary byte limits, ignores existing skills authored for other agent CLIs, forces the user to manually configure the project vision through multi-step chat wizards, and queries public search engines for private internal libraries.

Journey 12 realizes codeNERD's core vision:
- **"Never has to grep around"**: Anchoring membership in git truth and `.gitignore` ensures the search space contains only genuine repository code.
- **"The harness decides, not the model's discretion"**: Mangle Datalog fixpoints select read candidates, resolve duplicate agent skills, and derive tree roles.
- **"Domain knowledge is pushed into the window when needed"**: Foreign skills and living documentation are digested into SQLite knowledge bases and discrete JIT prompt atoms.
- **"Clean fixpoint, not clean loop"**: Go acts as the sensor; Mangle computes eras, lineages, agent rosters, and clarification questions.

---

## 2. Stage-by-Stage Execution Table

| # | Stage | Component (`path:func`) | Target State (Planned) | Shipped Today (`e056692c`) | Who Decides | Knowledge Entering the Model Window | What Forces Continuation | Tools Executed | Verdict / Output |
|---|---|---|---|---|---|---|---|---|---|
| **0** | Invocation | `cmd/nerd/cmd_init_scan.go:126` (`runInit`) | Operator runs `nerd init` in foreign repo root. Default configuration loaded. | Identical: `runInit` builds `DefaultInitConfig(cwd)` and calls `initializer.Initialize(ctx)`. | **User** / Go CLI | None. | None. | None. | CLI start. |
| **1** | Workspace Membership Floor | `internal/workspace/membership.go` (`For`) [Seam: `workspace <-> world`] | Executes `git ls-files -z -co --exclude-standard`. Creates member files snapshot; ancestors become member dirs. Admits tracked `.claude`, `.codex`. | **Absent**. `internal/world/fs.go:226-263` hardcodes 6-name `ignoredDirs` map and dot allowlist (`.github`, `.vscode`, `.circleci`, `.config`), skipping `.claude`, `.codex`. | **Git Truth** (Target) vs **Go Hardcoding** (Shipped) | None. | `git ls-files` (Target) vs disk `os.Stat` (Shipped). | Member snapshot created. |
| **2** | Whole-History Git Chronology | `internal/orient/history.go` (`StreamGitHistory`) | Executes single streaming `git log --name-status -M` with `GIT_OPTIONAL_LOCKS=0`. Extracts committer epoch (`%ct`), file active days, monthly metrics. Asserts `repo_file_history`, `repo_month`, `repo_span`. | **Partial**. `internal/world/git_scanner.go:18-101` runs `git log -n<depth> --numstat`, parsing committer time for churn rate only. No whole-history streaming; no rename tracking; no monthly buckets. | **Go Sensor** | None. | `git log` streaming process. | Temporal EDB facts asserted. |
| **3** | Ecosystem Discovery & Ingest | `internal/orient/ecosystem.go` (`Discover`) | Walks format registry (`.claude/`, `.codex/`, `.agents/`, `.gemini/`, `.grok/`, `.jules/`, `.cursor/`, `.roomodes`, user-scoped memory). Extracts frontmatter and unclipped bodies. Asserts `agent_source*`. | **Absent**. `internal/world/fs.go:241-263` prunes dot-dirs. `.claude/skills` only read in Phase 12 as unstructured markdown (`internal/init/strategic_knowledge.go:305-308`). Non-Claude agent trees completely skipped. | **Go Sensor** | None. | Filesystem reads over registered agent dirs. | `agent_source` EDB facts asserted. |
| **4** | Engine Fixpoint 1: Lineage & Winners | `internal/orient/engine.go` (`Evaluate`) | Orientation engine runs to fixpoint over `timeline.mg`, `lineage.mg`, `ecosystem_agents.mg`. Derives `repo_era` (`/wave`, `/lull`), `doc_generation`, `doc_burst`, `doc_evolved_into`, `doc_superseded`, `agent_source_winner`. | **Absent**. Zero Mangle policy evaluates git history, eras, document lineages, or agent skill deduplication. | **Mangle Kernel** | None. | Mangle fixpoint evaluation. | Chronological and lineage IDB facts derived. |
| **5** | Attention Selection | `internal/orient/lineage.mg` | Evaluates structural attention rules: selects origin specs, link graph hubs, cluster centroids, burst docs, and instruction files. Derives `orient_read_candidate/2` capped at budget. | **Heuristic Go Table**. `internal/init/strategic_knowledge.go:246-260` ranks docs by fixed name (`CLAUDE.md` priority 0, `README/VISION` priority 1). Lines 414-450 issue batch LLM relevance queries (1 call per 10 docs). | **Mangle Kernel** (Target) vs **Go Hardcoding + LLM Calls** (Shipped) | None. | Mangle query (Target) vs 100+ batch LLM calls (Shipped). | `orient_read_candidate` facts ready. |
| **6** | Document Role Transduction | `internal/northstar/derive.go` (`ClassifyDocuments`) | LLM reads each candidate document IN FULL via sequential lossless paging. Emits `doc_role_claim(Path, Role, Conf)` with evidence quote, plus `doc_theme`. | **Absent in Init**. Only exists in chat TUI (`cmd/nerd/chat/northstar_llm.go:185-242`), which **truncates documents exceeding 10,000 chars** (`northstar_llm.go:201-203`). | **Model** (guided by JIT atom) | Full candidate document bodies, paged sequentially with running summary. | LLM client `CompleteWithSystem`. | `doc_role_claim` facts asserted into engine. |
| **7** | Engine Fixpoint 2: Vision & Agents | `internal/orient/engine.go` (`Evaluate`) | Engine evaluates role claims. `lineage.mg` derives `vision_source(Path, Weight, Why)`. `ecosystem_agents.mg` derives `orient_agent(Name, Why)` and `orient_agent_knowledge(Name, SourceID)`. | **Imperative Go Switch**. `internal/init/agents.go:373-626` (`determineRequiredAgents`) switches on `profile.Language` and dependency strings. Preexisting agent skills ignored. | **Mangle Kernel** (Target) vs **Go Switch** (Shipped) | None. | Mangle fixpoint evaluation. | Winning vision sources and agent rosters derived. |
| **8** | Shard Agent Materialization | `internal/init/phase_ecosystem.go` [Seam: `orient <-> init`] | Dynamic creation of `.nerd/shards/{agent}_knowledge.db` seeded with winning skill atoms; writes `.nerd/agents/{agent}/prompts.yaml`; emits `.nerd/orientation/agents.md`. Feeds interactive curation. | **Partial**. `internal/init/initializer.go:560` runs `createType3Agents` (`agents.go:670`) on the Go-switch agent list. Context7 web research run unconditionally (`agents_knowledge.go:33-164`). | **Go Execution** (driven by Mangle query) | None. | SQLite DDL, disk file writes, optional Context7 research. | Shard agent knowledge bases materialized. |
| **9** | Non-Interactive North Star Synthesis | `internal/northstar/derive.go` (`DraftVision`) [Seam: `orient <-> northstar`] | LLM synthesizes `WizardDocument` (`Mission`, `Problem`, `VisionStmt`, `Personas`, `Capabilities`, `Risks`, `Requirements`, `Constraints`) citing source paths. Saves to `Store.SaveVision` (`store.go:197`). Existing vision preserved. | **Disconnected & Missing**. Phase 1 opens/closes store empty (`internal/init/initializer.go:796-804`). Phase 7b derives vision string but stores solely as atom `strategic/vision` in `knowledge.db` (`strategic_knowledge.go:655`). | **Model** (synthesis) + **Go Store** (authority) | Staged context: timeline eras, origin docs, evolution chains, winning vision text in weight order. | `Store.SaveVision`, `WriteVisionJSON`, `WriteVisionMangle`. | North Star authority populated in `.nerd/northstar_knowledge.db`. |
| **10** | Discernment & Private Boundary | `internal/orient/trees.go`, `deps.go` | Asserts `tree_stats` and manifest `dependency_seen`. Transduces ambiguous trees. Derives `tree_role`, `tree_treatment` (`membership.json`), and `dependency_private`. Suppresses web research for private deps. | **Absent**. All unignored files parsed as code. Research tools blindly query DuckDuckGo (`internal/tools/research/web_search.go:138`) and GitHub (`context7.go:293`) for private package names. | **Go Sensor** + **Mangle Policy** | Tree statistics, directory heads, package manifests. | Registry metadata lookup (cached). | `membership.json` emitted; private deps protected. |
| **11** | MCP Discovery & Staging | `internal/orient/mcp.go` | Scans project and user MCP configs. Asserts `mcp_server_seen` and key names (`mcp_server_credential`). Derives `mcp_import_candidate`. Stages for user consent. | **Absent**. No discovery of external MCP configs. Manual editing of `.nerd/config.json` required. | **Go Sensor** + **Mangle Policy** | None. | File scanning over `.mcp.json`, `settings.json`. | Candidates staged; secrets isolated. |
| **12** | Orientation Report & Fact Emission | `internal/init/phase_orient.go` | Emits human-readable `.nerd/orientation/README.md`. Emits ground EDB facts to `.nerd/orientation/orientation.mg` (loaded at boot). Emits pending questions to `questions.json`. | **Absent**. Zero orientation report produced. Only terminal logs and disconnected knowledge atoms emitted. | **Go Execution** | Complete orientation fact graph. | Disk file writes. | Persistent orientation artifacts created. |
| **13** | Init Completion Diagnostic | `internal/init/initializer.go:1132` | Diagnostic summary confirms fully oriented repository. Does NOT ask user to run `/northstar`. | **Manual Instruction**. `internal/init/initializer.go:1447` prints: `Use '/northstar' to define your project vision`. | **Go CLI** | None. | None. | Init command exits 0. |
| **14** | First-Boot TUI Launch | `cmd/nerd/cmd_chat.go:24` (`runChat`) | Developer executes `nerd chat` (or `nerd` bare). Guardian boots; loads `northstar_knowledge.db` and `.nerd/orientation/orientation.mg`. | `runChat` boots Guardian and chat TUI. `northstar_knowledge.db` is empty unless manually configured. | **User** / Go CLI | Orientation facts loaded into RealKernel. | SQLite connection, Mangle fact assertion. | TUI interactive loop begins. |
| **15** | First-Boot Clarification Dialogue | `cmd/nerd/chat/process_dream_delegation.go:26`, `model_handlers.go:417` [Seam: `orient <-> chat`] | Kernel queries pending questions from `questions.json`. Clarification UI presents questions one by one with pre-selected `BestGuess` and evidence lines. Operator accepts, selects, or skips. | **Unused on Boot**. `kernelClarification` only executes when a dream delegation routes to `/clarify`. First-boot prompt is empty chat prompt. | **Kernel** (queries) + **User** (decides) | Question text, options, and evidence formatted via `formatClarificationRequest`. | TUI Bubbletea rendering. | Operator choices captured. |
| **16** | Guarded MCP Config Merge | `internal/config/merge.go` (`GuardedMerge`) | If operator answered "yes" to MCP import: surgical JSON patch inserts `integrations.servers.<name>` into `.nerd/config.json`. Validates via `LoadUserConfig` + `Check`. Comments preserved. | **Absent**. Manual hand-editing of `.nerd/config.json` only. | **Go Execution** (gated by operator consent) | Staged MCP server definition. | Atomic disk file write with memory validation. | `.nerd/config.json` updated safely. |
| **17** | Durable Answer Persistence | `internal/orient/questions.go` (`SaveAnswers`) | Persists operator answers to `.nerd/orientation/answers.json`. Asserts `orient_answer(ID, Choice)`. Policies re-derive, immediately overriding automated guesses. | **Absent**. No persistent clarification answers recorded. | **Go Execution** + **Mangle Fixpoint** | None. | Disk file write. | Answers permanently stored. |
| **18** | Steady-State Transition | `cmd/nerd/chat/process.go:48` (`processInput`) | Operator begins normal interactive coding turns. Shards operate with grounded domain knowledge, accurate CodeDOM scope, and full North Star alignment. | Normal chat turn executes with ungrounded world model and empty North Star. | **Kernel Executive** | Working set facts selected via spreading activation. | Active session tool execution. | Normal OODA coding turn. |

---

## 3. Deep-Dive on Subsystem Seams

### Seam 1: Workspace Authority $\longleftrightarrow$ World Model (`workspace <-> world`)
- **Target Contract**: `internal/world/fs.go` and `incremental_scan.go` do not maintain private ignore lists. They construct or receive `*workspace.Membership` and invoke `Membership.IncludesDir(rel)`. If `false`, the walker immediately returns `filepath.SkipDir`.
- **Shipped Reality (`e056692c`)**: `internal/world/fs.go:226-263` hardcodes a 6-directory `ignoredDirs` map and a dot-directory allowlist, silently skipping `.claude`, `.codex`, `.agents`. `factory.go:2239` is the only site that copies user ignore patterns; `nerd init` uses compiled defaults.

### Seam 2: Orientation Engine $\longleftrightarrow$ Initialization Pipeline (`orient <-> init`)
- **Target Contract**: `internal/init/initializer.go` registers two new phases: `phase_ecosystem` (Phase 9 replacement) and `phase_orient` (Phase 12 replacement). The orientation engine runs to fixpoint, emitting agent rosters and North Star records directly into init's build context.
- **Shipped Reality (`e056692c`)**: Init runs 22 hardcoded phases. Phase 9 calls imperative Go function `determineRequiredAgents` (`internal/init/agents.go:373-626`). Phase 12 runs batch LLM relevance queries and stores a vision string in `.nerd/knowledge.db` that nothing in `internal/northstar` ever queries.

### Seam 3: Orientation Engine $\longleftrightarrow$ North Star Guardian (`orient <-> northstar`)
- **Target Contract**: `internal/northstar/derive.go` non-interactively synthesizes `WizardDocument` and writes directly to `.nerd/northstar_knowledge.db` via `Store.SaveVision`. It emits `.nerd/orientation/orientation.mg`. On Guardian boot, `SyncVisionAuthority` (`internal/northstar/bridge.go:436-534`) completes as a clean no-op, and `refreshKernelFacts` loads orientation facts beside `northstar.mg`.
- **Shipped Reality (`e056692c`)**: `internal/init/initializer.go:796-804` opens and closes `northstar.NewStore` empty. `SyncVisionAuthority` runs on boot, finds an empty database, and leaves the kernel with zero North Star facts.

### Seam 4: Orientation Questions $\longleftrightarrow$ TUI Chat Clarification (`orient <-> chat clarification`)
- **Target Contract**: On first TUI boot, `cmd/nerd/chat/model_handlers.go` checks for pending questions in `.nerd/orientation/questions.json`. It routes through the existing `kernelClarification` seam (`cmd/nerd/chat/process_dream_delegation.go:26`) and renders questions using `formatClarificationRequest` (`model_handlers.go:539`). Answers persist to `.nerd/orientation/answers.json`.
- **Shipped Reality (`e056692c`)**: `kernelClarification` is wired only to the `/dream` delegation path. First-boot chat displays an empty prompt with no awareness of initialization ambiguities.

### Seam 5: Research Tools $\longleftrightarrow$ Native Browser (`research <-> browser`)
- **Target Contract**: All research fetch operations (`web_fetch`, `web_search`, `context7_fetch`) route through codeNERD's native Rod/CDP browser (`internal/browser/session_manager.go`), executing full DOM rendering, JS execution, and quiescence waits.
- **Shipped Reality (`e056692c`)**: `internal/tools/research/web_fetch.go:100`, `web_search.go:138`, and `context7.go:293` read the network via `http.DefaultClient.Do`. Native browser automation is isolated in `browser_*` tools called only when explicitly invoked by the model.

### Seam 6: Perception & Meta Grounding $\longleftrightarrow$ Research (`perception/Meta grounding <-> research`)
- **Target Contract**: `OpenAICompatClient` for vendor `meta` routes requests through `/responses` with `{"type":"web_search"}` and `include:["web_search_call.results"]`. The broker reports `SupportsGrounding == true` for Meta, matching Gemini's capabilities.
- **Shipped Reality (`e056692c`)**: Meta plain completions and first tool turns go to `/chat/completions` without search tools (`client_openai_compat.go:5-18`). `GroundedWebSearch` skips `web_search_call` results and returns only URL citations; broker reports `SupportsGrounding == false`.

---

## 4. What Enters the Model's Window and Why

Across the onboarding journey, knowledge injection is strictly governed by the harness:

1. **Document Classification (Stage 6)**:
   - **What Enters**: Complete candidate documentation files paged sequentially.
   - **Why**: The model must classify the functional role (`/vision`, `/spec`, `/guide`) and extract core themes without suffering lossy truncation.
   - **Mechanism**: Lossless paging with running summaries carried across chunk boundaries.
2. **North Star Synthesis (Stage 9)**:
   - **What Enters**: (1) Chronological era timeline summary, (2) Foundational origin documents, (3) Evolution and supersession chains, (4) Full text of winning vision sources in descending weight order.
   - **Why**: The model must understand the full arc of the project's development — what was built in the origin era, what changed over time, and what current human drafts intend — to synthesize an authentic North Star.
   - **Mechanism**: Assembly of Mangle query results into a structured JIT synthesis prompt.
3. **First-Boot Clarification (Stage 15)**:
   - **What Enters**: Exactly one structured clarification question at a time, containing the target topic, pre-selected best guess, and supporting evidence quotes.
   - **Why**: Minimizes operator cognitive load while resolving residual ambiguities in tree roles or private dependencies.
   - **Mechanism**: Injected via `formatClarificationRequest` into the Bubbletea chat view.

---

## 5. What Persists After Onboarding

Following completion of Journey 12, the repository contains a durable, complete world model:

1. **SQLite Knowledge Bases** (`.nerd/shards/`):
   - `core_concepts.db`: Shared architecture and system concepts.
   - `codebase_knowledge.db`: Synthesized codebase architecture and conventions.
   - `{agent}_knowledge.db`: Specialized domain knowledge for each derived shard agent, seeded with winning foreign skill atoms.
   - `northstar_knowledge.db`: Canonical North Star specification (`Mission`, `Problem`, `Personas`, `Capabilities`, `Risks`, `Requirements`, `Constraints`).
2. **Declarative Mangle Facts**:
   - `.nerd/orientation/orientation.mg`: Ground EDB facts establishing repository eras, document lineages, origin sources, and private system boundaries (loaded on every boot).
   - `.nerd/northstar.mg`: Relational facts projecting the North Star into the kernel.
3. **Human-Readable Documentation**:
   - `.nerd/orientation/README.md`: Complete orientation report documenting development eras, document lineages, and the derived North Star summary.
   - `.nerd/orientation/agents.md`: Comprehensive agent roster detailing each specialist's purpose, sources, and superseded duplicates.
4. **Configuration & Operational Overlays**:
   - `.nerd/orientation/membership.json`: Operational directory treatments (`/index_and_parse`, `/index_names_only`, `/exclude`) consumed by `internal/workspace`.
   - `.nerd/orientation/answers.json`: Durable operator answers to clarification questions.
   - `.nerd/config.json`: Updated with approved external MCP servers via surgical, byte-preserving configuration merge.
