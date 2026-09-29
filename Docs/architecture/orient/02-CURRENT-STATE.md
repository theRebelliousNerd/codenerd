---
doc-class: shipped
subsystem: orient
implementation-status: shipped
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# 02 — Current State — Initialization, Scanning, and Documentation Today

> Verified 2026-09-29 against `bb7bafac` (`main`). Every claim in this document reflects source code inspected directly in the repository.

This document describes the exact behavior of codeNERD's initialization (`internal/init`), world scanning (`internal/world`), strategic documentation ingest, agent selection, North Star synchronization, specification checking, dependency graphing, turn safety gates, and technical debt detection as they exist today.

As of commit `bb7bafac`, package `internal/orient` has **not shipped**. There is no dedicated orientation engine, no git history streaming pass, no multi-agent ecosystem parser, and no non-interactive North Star derivation engine in production. The systems currently performing repository onboarding and codebase analysis are distributed across `internal/init`, `internal/world`, `cmd/nerd`, `internal/campaign`, `internal/session`, and `internal/northstar`.

---

## 1. The `nerd init` Phase Sequence Today

The initialization lifecycle is driven imperatively by `Initialize(ctx context.Context)` in `internal/init/initializer.go:460-621`. Execution proceeds through 22 sequential phases managed by `phaseRunner` (`internal/init/initializer.go:692-731`) and tracked by `ETATracker` (`internal/init/eta_tracker.go:21-46`):

1. **Phase 0 (`setup`)** — Starts system background shards via `i.shardMgr.StartSystemShards(ctx)` (`internal/init/initializer.go:486-494`).
2. **Phase 1 (`migration`)** — Executes SQLite table migrations across existing agent databases via `store.MigrateAllAgentDBs` (`internal/init/initializer.go:733-755`).
3. **Phase 2 (`directory`)** — Creates `.nerd/` directory hierarchy, copies default Mangle templates, creates `knowledge.db` and `northstar_knowledge.db` (`internal/init/initializer.go:757-808`).
4. **Phase 3 (`scanning`)** — Calls `world.Scanner.ScanDirectory(ctx, workspace)` (`internal/world/fs.go:124`), computing SHA-256 hashes and extracting preliminary topology facts (`internal/init/initializer.go:810-831`).
5. **Phase 4 (`analysis`)** — Stubbed notice indicating that deep semantic analysis is handled on-demand by the JIT loop (`internal/init/initializer.go:833-838`).
6. **Phase 5 (`profile`)** — Executes `buildProjectProfile()` (`internal/init/profile.go:27`), inspecting manifest files to detect languages, frameworks, and entry points, writing `.nerd/profile.json` (`internal/init/initializer.go:840-860`).
7. **Phase 6 (`facts`)** — Emits project identity facts into `.nerd/profile.mg` (`internal/init/profile.go:83`, called at `internal/init/initializer.go:862-876`).
8. **Phase 7 (`prompt_atoms`)** — Synthesizes hardcoded language and framework prompt atoms into `.nerd/knowledge.db` (`internal/init/profile.go:271`, called at `internal/init/initializer.go:878-886`).
9. **Phase 8 (`prompt_db`)** — Materializes `.nerd/prompts/corpus.db`, reconciling baked defaults with project atoms (`internal/init/profile.go:983`, called at `internal/init/initializer.go:888-899`).
10. **Phase 9 (`agents`)** — Executes `determineRequiredAgents` (`internal/init/agents.go:373-626`), merges CLI `--define-agent` definitions, runs interactive terminal curation on a TTY (`internal/init/agents_curation.go:71-109`), and records selections (`internal/init/initializer.go:901-921`).
11. **Phase 10 (`shared_kb`)** — Populates `.nerd/shards/core_concepts.db` with 13 baseline architecture atoms via `CreateSharedKnowledgePool` (`internal/init/shared_kb.go:105`, called at `internal/init/initializer.go:930-941`).
12. **Phase 11 (`kb_creation`)** — Invokes `createType3Agents` (`internal/init/agents.go:670`), creating `.nerd/shards/{agent}_knowledge.db`, dispatching Context7 web research queries, generating `.nerd/agents/{agent}/prompts.yaml`, and registering shards with the manager (`internal/init/initializer.go:943-954`).
13. **Phase 12 (`codebase_kb`)** — Creates `.nerd/shards/codebase_knowledge.db` (`internal/init/profile.go:791`), collects documentation via `GatherProjectDocumentation` (`internal/init/strategic_knowledge.go:241`), filters documents through batch LLM relevance queries, and synthesizes `StrategicKnowledge` (`internal/init/initializer.go:956-986`).
14. **Phase 13 (`core_shards_kb`)** — Generates SQLite databases for core shards (`coder_knowledge.db`, `reviewer_knowledge.db`, `tester_knowledge.db`) containing safety and protocol rules (`internal/init/agents_registration.go:89`, called at `internal/init/initializer.go:988-1001`).
15. **Phase 14 (`campaign_kb`)** — Seeds `.nerd/shards/campaign_knowledge.db` with orchestration concept atoms (`internal/init/profile.go:869`, called at `internal/init/initializer.go:1003-1015`).
16. **Phase 15 (`tool_generation`)** — Records `missing_tool_for` facts for on-demand synthesis by autopoiesis; compiles no binaries at initialization time (`internal/init/tools.go:429`, called at `internal/init/initializer.go:1024-1037`).
17. **Phase 16 (`preferences`)** — Merges default coding, testing, and safety preferences into `.nerd/preferences.json` (`internal/init/profile.go:219`, called at `internal/init/initializer.go:1039-1056`).
18. **Phase 17 (`session`)** — Writes initial state to `.nerd/session.json` (`internal/init/profile.go:718`, called at `internal/init/initializer.go:1058-1068`).
19. **Phase 18 (`tools`)** — Generates tool configuration entries into `.nerd/tools/available_tools.json` (`internal/init/tools.go:37`, called at `internal/init/initializer.go:1070-1096`).
20. **Phase 19 (`registry`)** — Serializes active agent descriptors into `.nerd/agents.json` (`internal/init/agents_registration.go:73`, called at `internal/init/initializer.go:1098-1108`).
21. **Phase 20 (`prompt_sync`)** — Reloads prompts into SQLite prompt tables via `initializeDiscoveredAgentStores` (`internal/init/validation.go:26`) and `prompt.ReloadAllPrompts` (`internal/prompt/loader.go:541`, called at `internal/init/initializer.go:1110-1130`).
22. **Phase 21 (`complete`)** — Executes database validation (`ValidateAllAgentDBs`) and outputs the final completion diagnostic (`internal/init/initializer.go:1132-1182`).

---

## 2. World Topology and Hidden Directory Pruning

The filesystem walk during Phase 3 (`internal/world/fs.go:124-370`) traverses regular files to assert `file_topology`, `file_dir`, and `is_test_file` into the kernel. It enforces three exclusion layers:

1. **Hardcoded Directory Names** — Lines 226-238 prune `node_modules`, `vendor`, `dist`, `build`, `.git`, and `.nerd` via `filepath.SkipDir`.
2. **Hidden Directory Allowlist** — Lines 241-263 prune any directory starting with `.` unless it matches an explicit allowlist:
   - Permitted: `.github`, `.vscode`, `.circleci`, `.config`.
   - Explicitly dropped: all other hidden directories.
3. **Consequence**: Preexisting agent configurations in `.claude/`, `.codex/`, `.agents/`, `.gemini/`, `.grok/`, `.jules/`, and `.cursor/` are **completely skipped by `ScanDirectory`**. The world model never records their existence.

The incremental scan counterpart (`internal/world/incremental_scan.go:128-182`) duplicates this hidden directory allowlist (`incremental_scan.go:137-153`), but omits the hardcoded `node_modules` map, creating inconsistent directory behavior between full and incremental scans.

---

## 3. Strategic Documentation Ingest and LLM Filtering

Phase 12 gathers documentation via `GatherProjectDocumentation` in `internal/init/strategic_knowledge.go:241-410`. Its behavior exhibits three major constraints:

1. **Hardcoded Filename Scoring** (`internal/init/strategic_knowledge.go:246-260`):
   - Priority 0: `CLAUDE.md`.
   - Priority 1: `README.md`, `ARCHITECTURE.md`, `DESIGN.md`, `VISION.md`, `PHILOSOPHY.md`.
   - Priority 2: `CONTRIBUTING.md`, `CHANGELOG.md`, `ROADMAP.md`, `GOALS.md`, `STRATEGY.md`, `API.md`, and any file inside `docs/`, `spec/`, `planning/`, `.github/`, or `.claude/`.
   - Priority 3: All other discovered documentation files.
   - Files named `AGENTS.md` or `GEMINI.md` are not in the priority map; they fall through to Priority 3 unless keyword heuristics detect phrases like `# Vision` in the first 2,000 characters.
2. **Selective Hidden Walking** (`internal/init/strategic_knowledge.go:305-308`):
   The walker skips hidden directories with one exception:
   `if strings.HasPrefix(name, ".") && name != "." && name != ".github" && name != ".claude" { return filepath.SkipDir }`.
   Consequently, `.claude/` is examined, but `.codex/`, `.agents/`, `.gemini/`, `.grok/`, and `.cursor/` are ignored.
3. **Expensive LLM Batch Relevance Calls** (`internal/init/strategic_knowledge.go:414-450`):
   Documentation files are batched into groups of 10 and sent to the LLM to classify relevance. On a repository with 1,000 documentation files, this issues ~100 LLM calls solely to filter documents.
4. **Disconnected Synthesis**:
   Relevant documents are synthesized into a single `StrategicKnowledge` object (`ProjectVision`, `CorePhilosophy`, `DesignPrinciples`) via `generateStrategicKnowledge` (`internal/init/strategic_knowledge.go:25, 68-120`). However, `PersistStrategicKnowledge` (`strategic_knowledge.go:616-663`) stores the vision solely as atom `strategic/vision` in `.nerd/knowledge.db`. It does not write to `internal/northstar`.

---

## 4. Shard Agent Selection Today

Selection of specialist shard agents during Phase 9 is entirely imperative. In `internal/init/agents.go:373-626` (`determineRequiredAgents`):

1. **Go Language Switch** (`internal/init/agents.go:377-434`):
   - `"go"`, `"golang"` $\rightarrow$ `GoExpert`
   - `"python"` $\rightarrow$ `PythonExpert`
   - `"typescript"`, `"javascript"` $\rightarrow$ `TSExpert`
   - `"rust"` $\rightarrow$ `RustExpert`
   - `"kotlin"` $\rightarrow$ `AndroidExpert`
2. **Framework & Dependency Matcher** (`internal/init/agents.go:436-580`):
   - `"gin"`, `"echo"`, `"fiber"` $\rightarrow$ `WebAPIExpert`
   - `"react"`, `"nextjs"`, `"vue"` $\rightarrow$ `FrontendExpert`
   - `"rod"` $\rightarrow$ `RodExpert`
   - `"bubbletea"` $\rightarrow$ `BubbleTeaExpert`
   - `"cobra"` $\rightarrow$ `CobraExpert`
   - `"gorm"`, `"sqlx"`, `"prisma"` $\rightarrow$ `DatabaseExpert`
   - `"adk"`, `"a2a"` $\rightarrow$ `ADKExpert`, `A2AExpert`
3. **Fallback** (`internal/init/agents.go:582-625`):
   - If no agents match, defaults to `SecurityAuditor` and `TestArchitect`.
4. **Ecosystem Ingest Deficit**:
   codeNERD parses zero foreign subagent definitions (`.claude/agents/*.md`, `.codex/agents/*.toml`, `.roomodes`). Preexisting skills in `.claude/skills/*/SKILL.md` or `.agents/skills/*/SKILL.md` are treated as generic markdown files, discarding their YAML frontmatter, tool assignments, tags, and execution instructions.

---

## 5. North Star Authority and Creation Paths Today

The canonical North Star domain model lives in `internal/northstar/types.go:27-38` (`Vision` with `Mission`, `Problem`, `VisionStmt`, `Personas`, `Capabilities`, `Risks`, `Requirements`, `Constraints`), with disk representation `WizardDocument` (`internal/northstar/bridge.go:88-100`) and SQLite store `Store` (`internal/northstar/store.go:96-108`).

### The Complete Disconnection from `init`

1. In `internal/init/initializer.go:796-804`, Phase 1 executes:
   `northstarStore, err := northstar.NewStore(nerdDir)`
   The store is opened purely to run SQLite schema migrations, and is closed immediately. No vision record is created, no `northstar.json` is exported, and no `northstar.mg` is emitted.
2. In `internal/init/initializer.go:1447`, the final output explicitly commands:
   `Use '/northstar' to define your project vision`.
3. In `cmd/nerd/cmd_init_scan.go:126-242`, `runInit` provides no arguments, flags, or configuration to derive a North Star non-interactively.

### Existing Creation Paths

Only three creation mechanisms exist today:

1. **Interactive TUI Chat Wizard** (`cmd/nerd/chat/northstar_wizard.go:41-97`):
   Steps through 10 interactive phases. In Phase 1 (`northstar_llm.go:185-242`), the operator enters file paths manually. `analyzeNorthstarDocs` reads each file and **truncates any file larger than 10,000 characters** (`northstar_llm.go:201-203`). In Phase 8, typing "auto" runs `generateRequirementsWithLLM` (`northstar_llm.go:22-102`) to draft requirements.
2. **CLI JSON Load** (`cmd/nerd/cmd_northstar.go:625-684`):
   `nerd northstar load <file.json>` unmarshals an existing, pre-authored JSON file into SQLite and renders `.mg` and `.json`.
3. **Reconciliation Sync** (`internal/northstar/bridge.go:436-534`):
   `SyncVisionAuthority` checks timestamps between `.nerd/northstar.json` and `.nerd/northstar_knowledge.db` on boot, synchronizing changes between disk and SQLite.

There is **zero non-interactive path** to derive a North Star directly from repository documents without human chat interaction or pre-authored JSON files.

---

## 6. Git History, Recency, and Quality Machinery Today

1. **Git Scanner** — `internal/world/git_scanner.go:18-101` executes:
   `git log -n<depth> --pretty=format:COMMIT:%H|%an|%ct|%s --numstat`.
   It parses committer Unix timestamp (`%ct`) and calculates churn per file, asserting `git_history/5` and `churn_rate/2`. This scanner runs only up to a fixed depth, does not stream the entire commit graph, does not follow file renames (`-M`), and does not compute monthly aggregate buckets or development eras.
2. **Filesystem Modification Time Awareness** — `internal/world/content_stamp.go:18-207` acknowledges that filesystem modification timestamps (`mtime`) are volatile across checkouts, clones, and branch switches. However, repository document ranking in `GatherProjectDocumentation` ignores git committer dates entirely.
3. **Git Blame** — `internal/projectdoc/tool_gate.go:547-548` includes `git blame` in the permitted read-only tool list, but it is never executed by any scanner or analysis function in the codebase.
4. **Mangle Policy Scope** — While `internal/context/working_set.mg:183-205` derives `working_stale` and `working_superseded` for model context observations during active sessions, and `internal/core/defaults/jit_compiler.mg:184-264` derives prompt atom supersession, **no Mangle policy exists to rank, supersede, or detect lineages across repository documentation, code trees, or external agent skills**.

---

## 7. Specification Checking and Project Contracts Today

1. **Static Documentation Linter (`docscheck`)**:
   - `internal/docscheck/docscheck.go:66-150` implements `Checker.CheckPackage`, validating front-matter schemas, package slots, gap matrices, and ADR witnesses.
   - It is strictly hardcoded to codeNERD's internal layout: `ArchDir()` resolves to `Docs/architecture/<pkg>` (`docscheck.go:85`), and witness scans restrict file extensions to `.go` and `.mg` (`docscheck.go:674`).
   - Execution is purely CLI-driven via `nerd docs check` (`cmd/nerd/cmd_docs.go:85`).
   - `internal/docscheck/problem.go:55-101` maps findings to `doc_problem(Pkg, File, Code, Message)` facts, declared in `internal/core/defaults/schemas_reviewer.mg:83`. However, these facts are never asserted into the active session kernel during live development.
2. **Project Policy Surface (`nerd.md`)**:
   - `internal/projectdoc/nerdmd.go:48-128` parses `nerd.md` front-matter schema `nerd/v1`, loaded via `LoadAll` (`internal/system/factory.go:1339`).
   - Emits operational facts: `project_doc`, `project_command`, `project_forbidden_path`, `project_requirement`, `project_convention`, `module_northstar`, `module_requirement` (`internal/projectdoc/facts.go:61-120`).
   - `nerd.md` acts as a project configuration and write-safety boundary (`internal/projectdoc/tool_gate.go:37-80`), not an architectural or behavioral specification.

---

## 8. Campaign Decomposition and Recursion Today

1. **Unstructured Document Decomposition**:
   - When campaigns ingest documents (`internal/campaign/decomposer_documents.go:22-78`), files matching `.md` and `.txt` are embedded into SQLite vector tables (`decomposer_documents.go:265-307`).
   - Requirements are extracted via a single LLM prompt capped at 30 snippets (`internal/campaign/decomposer_requirements.go:19-150`).
   - Zero parsing of structured Markdown tables, exit criteria, or formal specification matrices occurs.
2. **Blind Campaign Recursion**:
   - `nerd campaign recurse` (`cmd/nerd/cmd_campaign_recurse.go:42-80` and `internal/campaign/recurse_policy.go:66-106`) sweeps package import DAGs bottom-up.
   - It selects the next task via `recurse_next` (`recurse_policy.go:87-95`), prioritized strictly by compiler and linter gate findings (`gates.Finding`, lines 66-85).
   - Recurse cycles completely ignore `Docs/architecture` gap tables, specifications, and North Star requirements.

---

## 9. Dependency Graphs, Kahn Ordering, and Graph Disjointness Today

1. **Code Workspace DAG**:
   - `DeriveWorkspaceDAG` (`internal/campaign/recurse_workspace.go:59-92`) parses imports across Go (`go list`), Python, JS/TS, and Rust into `workspaceGraph` nodes (`recurse_workspace.go:94-105`).
   - Collapses cycles into strongly connected components via Tarjan's algorithm (`recurse_workspace.go:139-226`) and closes the DAG with `wiring`, `review`, and `bench` nodes (`recurse_workspace.go:228-249`).
2. **Kahn Topological Ordering in Mangle**:
   - `internal/campaign/recurse_plan.go:108-133` asserts `subsystem_node(ID)`, `subsystem_node_ord(ID, Ord)`, and `subsystem_depends(ID, Dep)`.
   - `internal/core/defaults/policy/recurse.mg:377-386` computes Kahn ordering: `recurse_node_ready(ID, Ord) :- subsystem_node(ID), subsystem_node_ord(ID, Ord), !recurse_visited_id(ID), !recurse_dep_unvisited(ID).`
   - This ordering is consumed exclusively by `nerd campaign recurse` for compiler sweep loops.
3. **World Model Dependency Facts**:
   - Cartographer emits `dependency_link` (`internal/world/dependency_links.go:12-51`), `file_imports`, and `code_calls` (`internal/world/cartographer.go:273-279`).
   - Bounded demand-driven reachability in Mangle (`internal/core/defaults/policy/system_world.mg:61-88`) bounds search to length $\le 15$ to avoid kernel exhaustion.
4. **Complete Disconnection from Specifications**:
   - The Code DAG and North Star specification schema (`schemas_misc.mg:44-96`) are entirely disjoint.
   - Zero predicates or Go data structures join code units to specification units; no realization edges exist.

---

## 10. Turn Completion Gates and Missing Evidence Today

1. **Session Turn Verification**:
   - `internal/core/defaults/policy/coder_safety.mg:632-633` defines `turn_done(Turn) :- turn_executed(Turn), turn_verified(Turn)`.
   - Lines 674-676 derive `turn_verified` based on write evidence, build gates, test gates, coverage debt, and vet findings.
   - `consumeTurnDoneSignal` (`internal/session/executor.go:2879-2915`) consumes `turn_done` and `turn_missing_evidence`.
2. **Zero Spec Obligations**:
   - All evaluated gates (`/build`, `/test`, `/check`, `/vet`, `/test_run`, `/pinned`) are mechanical toolchain checks (`coder_safety.mg:500-580`).
   - There is no gate verifying whether a specification was updated, whether a gap was closed, or whether new public surfaces possess architectural documentation.

---

## 11. Technical Debt Detection and CodeDOM Refactoring Today

1. **Unreferenced Symbols**:
   - `StructureIndex.Unreferenced` (`internal/world/structure_index.go:759-793`) identifies unreferenced declarations in Go code by comparing usage counts to declaration counts.
   - Output is rendered as human-readable text; no facts are asserted into Mangle.
2. **Dark Struct Fields**:
   - `cmd/tools/audit_dark_fields/main.go:1-77` audits Go struct fields that production code reads but never writes, checked against a baseline text file.
   - It runs as an offline developer script with zero runtime kernel integration.
3. **CodeDOM Repoint and Delete**:
   - `repointAndDelete` (`internal/tools/codedom/repoint.go:310-342` and `element_edit.go:566-568`) deletes an element and rewrites call sites across specified paths in one atomic write.
   - It is strictly restricted to Go package-level names; methods are refused (`repoint.go:312`), cross-file moves are unsupported, and non-Go languages are excluded. It checks syntax but does not execute compiler or test gates inside the transaction.

---

## 12. Clarification Dialogues and Persistence Today

1. **Kernel Clarification UI Seam**:
   - `kernelClarification` (`cmd/nerd/chat/process_dream_delegation.go:26-62`) queries `clarification_question(/current_intent, Question)` and `clarification_option(/current_intent, Verb, Label)` from `internal/core/defaults/policy/clarification.mg:42-88`.
   - It is triggered solely when a user prompt cannot be mapped to a known verb (`intent_unmapped`).
2. **Zero Orphan Classification or Shared Persistence**:
   - Zero code queries orphaned or un-specced functions to prompt the user.
   - No schema or persistence mechanism exists to record whether un-specced code is a feature, an experiment, or trash.
   - No committed team-shared answers file exists in version control.
