---
doc-class: governance
subsystem: orient
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# OPEN-QUESTIONS — Orientation Invariants and Open Edges

This document catalogues the standing architectural tripwires that must never be breached, alongside open architectural questions identified during system audits.

---

## 1. Standing Architectural Tripwires

A future author must preserve these four non-negotiable invariants:

### Tripwire 1: Zero Repository-Specific Names in Code or Policy
- **Constraint**: No string literal matching a specific company name, proprietary product name, or internal folder path may appear in Go source code or `.mg` policy files.
- **Verification**: A repository-wide regex scan of `internal/orient/` for project-specific terms must return zero hits. All discernment must emerge from general structural and statistical properties.

### Tripwire 2: Zero Secret Credential Emission into the Fact Graph
- **Constraint**: Discovered API keys, bearer tokens, or password strings from external MCP configurations or environment files must never be asserted into Mangle facts, written to `.nerd/orientation/README.md`, or emitted to logs.
- **Verification**: `mcp_server_credential` facts carry only key names (`X-API-Key`, `OPENAI_API_KEY`), never values.

### Tripwire 3: Zero Document Truncation
- **Constraint**: Code reading documentation or agent skills must never clip content at arbitrary character boundaries (such as the legacy 10,000-character truncation in `cmd/nerd/chat/northstar_llm.go:201-203`).
- **Verification**: Paging algorithms must stream oversized files sequentially with running summaries; small documents must be batched without clipping.

### Tripwire 4: Non-Destructive North Star Authority
- **Constraint**: `nerd init` must never overwrite an existing `.nerd/northstar_knowledge.db` containing user-curated or previously established vision data.
- **Verification**: If an existing vision record is present, the derived vision must be written to `.nerd/northstar.derived.json` and logged as an advisory.

---

## 2. Open Architectural Questions

### Question 1: Subtree North Stars in Polyglot Monorepos
- **Problem**: In a monorepo containing distinct applications (e.g. a Python machine learning backend, a TypeScript web application, and a Go systems service), should orientation synthesize a single unified project North Star, or should it support hierarchical, subtree-scoped North Stars?
- **Current Position**: `internal/northstar` supports a single global vision per workspace. Monorepo capabilities are represented as distinct `Capability` entries linked to shared personas.
- **Open Edge**: Evaluating whether future iterations should allow `northstar_vision(Scope, ...)` partitioned by sub-workspace roots.

### Question 2: Graceful Degradation in Non-Git Workspaces
- **Problem**: When codeNERD is executed in an unpacked zip archive or a Docker container lacking a `.git/` directory, git history streaming cannot run.
- **Current Position**: The Go sensor detects the absence of `.git/`, asserts `repo_span(0, 0, 0, /no_git)`, and falls back to filesystem traversal. Document generations default to `/origin` or `/untracked`.
- **Open Edge**: How should evolution policies behave when committer timestamps are completely absent? Can document cross-links and heading analysis alone compensate for missing temporal signals?

### Question 3: Submodule and Worktree Boundaries
- **Problem**: How should `internal/orient` treat git submodules and secondary git worktrees?
- **Current Position**: `git ls-files` at the top level treats submodules as directory entries. The orientation sensor does not recursively stream independent git logs for nested submodules in Phase 1.
- **Open Edge**: Should submodules receive their own independent timeline passes, or remain treated as vendored dependencies?

### Question 4: Automated Re-Orientation Triggers
- **Problem**: As a repository undergoes months of active development, commit eras change and initial orientation facts become stale.
- **Current Position**: `nerd orient --refresh` is an explicit manual CLI command.
- **Open Edge**: Should background observers (`internal/shards/observer_manager.go`) automatically detect when commit volume exceeds an era threshold and nudge the operator to trigger re-orientation?
