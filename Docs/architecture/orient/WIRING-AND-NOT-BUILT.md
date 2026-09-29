---
doc-class: shipped
subsystem: orient
implementation-status: shipped
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# WIRING-AND-NOT-BUILT — Orientation & Spec Alignment Wiring Audit

> Verified 2026-09-29 against `bb7bafac` (`main`).

This document records the exact state of wiring between initialization, documentation ingestion, world scanning, North Star synchronization, dependency graphing, turn safety gates, and technical debt detection. It catalogues what is wired and reachable, what exists but is disconnected, and what the high-level architecture assumes that the codebase does not currently perform.

---

## 1. Wired and Reachable Today

The following paths are actively executed during normal CLI invocations:

1. **`nerd init` Orchestration Pipeline**:
   - `cmd/nerd/cmd_init_scan.go:126-242` invokes `initializer.Initialize(ctx)`.
   - `internal/init/initializer.go:460-621` executes 22 sequential phases tracked by `phaseRunner` (lines 692-731) and `ETATracker` (`internal/init/eta_tracker.go:21-46`).
2. **Manifest and Dependency Ingestion**:
   - `internal/init/scanner.go:46-687` and `internal/init/scanner_dependencies.go:47-85` parse `go.mod`, `package.json`, `Cargo.toml`, and lockfiles into `ProjectProfile`.
3. **World Scanner Traversal**:
   - `internal/init/initializer.go:814` invokes `world.Scanner.ScanDirectory(ctx, workspace)` (`internal/world/fs.go:124-370`), asserting file topology and SHA-256 hashes into the active session kernel.
4. **Strategic Knowledge Filtering & Atom Storage**:
   - `internal/init/initializer.go:968-985` invokes `generateStrategicKnowledge` (`internal/init/strategic_knowledge.go:68-120`) using documents gathered by `GatherProjectDocumentation` (lines 241-410).
   - Knowledge is persisted into `.nerd/knowledge.db` as atom `strategic/vision` (`strategic_knowledge.go:616-663`).
5. **Specialist Agent Creation**:
   - `internal/init/agents.go:373-626` (`determineRequiredAgents`) evaluates profile fields and triggers `createAgentKnowledgeBase` (`internal/init/agents_knowledge.go:33-164`), populating `.nerd/shards/{agent}_knowledge.db` and registering in `.nerd/agents.json` (`internal/init/agents_registration.go:73`).
6. **North Star Interactive Chat Wizard**:
   - `/northstar` in interactive TUI chat triggers `cmd/nerd/chat/northstar_wizard.go:41-97`, calling `analyzeNorthstarDocs` (`cmd/nerd/chat/northstar_llm.go:185-242`) and `generateRequirementsWithLLM` (lines 22-102), saving to SQLite via `Store.SaveVision` (`internal/northstar/store.go:197`).
7. **North Star Store Reconciliation**:
   - Guardian boot calls `SyncVisionAuthority` (`internal/northstar/bridge.go:436-534`), synchronizing `.nerd/northstar.json` and `.nerd/northstar_knowledge.db`.
8. **Interactive Clarification UI Seam**:
   - `cmd/nerd/chat/process_dream_delegation.go:26` (`kernelClarification`) and `cmd/nerd/chat/model_handlers.go:417, 539` render interactive clarification requests and handle option selection.
9. **Workspace DAG Generation**:
   - `internal/campaign/recurse_workspace.go:59-92` (`DeriveWorkspaceDAG`) scans package directories across Go, Python, JavaScript/TypeScript, and Rust, collapsing circular dependencies via Tarjan's SCC algorithm (`recurse_workspace.go:139-226`).
10. **Kahn Topological Sort for Sweep Sweeps**:
    - `internal/campaign/recurse_plan.go:108-133` asserts sweep graph facts, and `internal/core/defaults/policy/recurse.mg:377-386` computes `recurse_node_ready` for bottom-up package ordering in `nerd campaign recurse` (`cmd/nerd/cmd_campaign_recurse.go:42-80`).
11. **Static AST Unreferenced Symbol Scanner**:
    - `internal/world/structure_index.go:759-793` (`StructureIndex.Unreferenced`) computes unreferenced declarations in Go code by comparing usage counts to declaration counts.
12. **CodeDOM Repoint and Delete**:
    - `internal/tools/codedom/repoint.go:310-342` (`repointAndDelete`) rewrites uses and deletes Go package-level declarations in one atomic file commit.

---

## 2. Exists But Nothing Calls (or Test-Only)

1. **`northstar.NewStore` in `init`**:
   `internal/init/initializer.go:796-804` instantiates `northstar.NewStore(nerdDir)` during Phase 1 to execute database table migrations, and immediately calls `northstarStore.Close()`. No vision records, personas, or capabilities are ever inserted.
2. **`strategic/vision` Knowledge Atom**:
   `internal/init/strategic_knowledge.go:655` writes `storeAtom("strategic/vision", knowledge.ProjectVision, 1.0)` into `knowledge.db`. However, `internal/northstar` never queries this atom, and `Guardian` never loads it into the North Star fact family.
3. **Dataflow Directory Walk**:
   `internal/world/dataflow.go:607` (`ExtractDataFlowForDirectory`) has zero production callers outside `dataflow_test.go`.
4. **`git blame` Tooling**:
   `git blame` is permitted by tool gates (`internal/projectdoc/tool_gate.go:547-548`), but is never called by any scanner, analyzer, or initialization routine.
5. **`docscheck` Problem Facts in Runtime Kernel**:
   `internal/docscheck/problem.go:55-101` generates `doc_problem(Pkg, File, Code, Message)` facts, declared in `internal/core/defaults/schemas_reviewer.mg:83`. However, no production Go code asserts these facts into the kernel during live execution; `docscheck` runs exclusively as a standalone CLI tool (`cmd/nerd/cmd_docs.go:85`).
6. **Dark Field Static Scanner**:
   `cmd/tools/audit_dark_fields/main.go:1-77` audits unwritten exported struct fields as an offline developer script. It asserts zero facts into Mangle and is uncalled by session executors or scanners.

---

## 3. What the Design Assumes That the Code Does NOT Do

| Architectural Assumption | Code Reality (Verified Citations) | Architectural Consequence |
|---|---|---|
| `nerd init` autonomously configures the project North Star. | `internal/init/initializer.go:796-804` closes the store empty; line 1447 tells user to run `/northstar`. | Initialized projects start with an empty North Star; Guardian cannot evaluate alignment. |
| codeNERD ingests skills from preexisting agent CLIs. | `internal/world/fs.go:241-263` prunes all dot-directories except `.github`, `.vscode`, `.circleci`, `.config`. | Skills in `.claude/`, `.codex/`, `.agents/`, `.gemini/` are invisible to the world model. |
| Specialist agents are selected based on project reality. | `internal/init/agents.go:373-626` uses a fixed Go `switch` statement over language strings. | Projects receive generic language experts; custom domain skills and subagent configurations are ignored. |
| Chronology distinguishes living designs from obsolete specs. | `internal/init/strategic_knowledge.go:246-260` ranks documents by hardcoded filename strings. | `CLAUDE.md` is always Priority 0; an active architectural spec not named in the table is ranked Priority 3. |
| Document ingestion is comprehensive and lossless. | `cmd/nerd/chat/northstar_llm.go:201-203` truncates documents exceeding 10,000 characters. | Specifications lose requirements, architectures lose components, inducing hallucinations. |
| Scanners discern source code from seed data and fixtures. | All unignored files are ingested into topology (`internal/world/fs.go:330-348`). | Megabytes of seed data, answer keys, and test fixtures pollute code search and CodeDOM memory. |
| Research tools respect internal enterprise boundaries. | `internal/tools/research/web_search.go:138` and `context7.go:293` blindly fetch external URLs. | Internal private packages trigger public search engine queries, leaking names and failing lookups. |
| **Code DAG and Spec DAG are unified to determine codebase readiness.** | **The two graphs are completely disjoint.** The Code DAG (`internal/campaign/recurse_workspace.go:59-92`) and North Star specification schema (`schemas_misc.mg:44-96`) share zero relations. `recurse_cycle.go:21-40` evaluates only compiler/linter gate findings (`gates.Finding`), completely blind to specifications and gap matrices. | codeNERD cannot derive whether code is aligned with specs, cannot suggest what to work on next from spec gaps, and cannot trace requirements to AST symbols. |
| **A change that leaves specs misaligned is refused.** | `internal/core/defaults/policy/coder_safety.mg:674-676` and `internal/session/executor.go:2879-2915` verify turns based on compiler/test/vet gates only. Spec files owe zero updates. | Specifications drift behind implementation immediately upon code mutation, rendering specs obsolete. |
| **codeNERD detects orphaned code and asks the operator to classify it.** | Zero orphan detection exists. `kernelClarification` (`cmd/nerd/chat/process_dream_delegation.go:26-62`) handles only ambiguous intent strings; no durable classification store exists. | Un-specced code accumulates indefinitely or is destroyed arbitrarily by ungrounded agents. |
| **Next work item is derived from the DAG of code vs spec.** | No `/next` command exists (`cmd/nerd/chat/commands.go:55-265`). `next_action` (`capabilities.mg:20-51`) is an internal agent dispatcher, not a user suggestion. | Users receive no foundations-up guidance on what to build next. |
