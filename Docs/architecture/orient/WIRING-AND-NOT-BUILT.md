---
doc-class: shipped
subsystem: orient
implementation-status: shipped
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# WIRING-AND-NOT-BUILT — Orientation Wiring Audit

> Verified 2026-09-29 against `e056692c` (`main`).

This document records the exact state of wiring between initialization, documentation ingestion, world scanning, and North Star synchronization. It catalogues what is wired and reachable, what exists but is disconnected, and what the high-level architecture assumes that the codebase does not currently perform.

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
