---
doc-class: north-star
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 08 — Discernment and Questions — Tree Roles, Private Systems, MCP, and First-Boot Dialogue

This capability specification details the discernment of messy repository directory trees, identification of private and proprietary systems, discovery of external MCP tools, and first-boot operator clarification, owned by Lane `O1`.

---

## 1. The Real-World Repository Dilemma

A real-world repository is rarely a clean collection of production source files and documentation. In enterprise and polyglot codebases:
1. **The Nebulous Mess**: Massive directory trees containing test fixtures, golden answer keys, machine-generated stubs, or gigabyte-scale seed datasets sit alongside application source code. If an agent naively indexes, parses, and embeds these files, its CodeDOM memory exhausts its budget, and its code search returns irrelevant fixture data.
2. **Private Dependencies**: Codebases often depend on proprietary enterprise packages, internal microservices, or custom database engines (e.g. internal SDKs or proprietary graph engines). Today, research tools attempt to look these up on DuckDuckGo and GitHub (`internal/tools/research/web_search.go:138`), leaking internal names to the public internet and hallucinating documentation when the lookup fails.
3. **Hidden MCP Tooling**: Multi-agent users maintain active MCP server configurations in `.claude/settings.json`, `.cursor/mcp.json`, or user-scoped `~/.claude.json`. Today, codeNERD operates completely disconnected from these tools unless an operator manually hand-edits `.nerd/config.json`.
4. **Interactive Paralysis vs. Silent Guessing**: Asking dozens of terminal prompts during `nerd init` frustrates developers and breaks headless CI environments. Conversely, guessing silently without operator confirmation leads to subtle, persistent agent failures.

The Discernment engine resolves this by establishing git membership as the floor, deriving tree roles and private boundaries deductively, formulating best guesses, and presenting a concise, high-value clarification dialogue on the first TUI boot.

---

## 2. Tree Roles and Treatment Overlay

The Go sensor (`internal/orient/trees.go`) gathers structural statistics for every member directory (bounded by configurable depth; always including all top-level directories):

### Ground EDB Facts
- `tree_stats(Dir, Files, Bytes, TrackedFiles, IgnoredEntriesBelow, LastUnix, Commits, DistinctAuthorsOrZero)`:
  - `IgnoredEntriesBelow` is computed via `git ls-files -o -i --exclude-standard --directory`. A directory containing a few tracked files surrounded by thousands of ignored files provides a strong heuristic signal for build artifacts or seed environments.
- `tree_ext(Dir, Ext, Count)`:
  - Histogram of file extensions within the tree.
- `tree_name(Dir, Base)`:
  - Normalized directory base name.

### LLM Transduction for Ambiguous Trees
For trees where deterministic heuristics yield low confidence, the LLM inspects directory names, extension distributions, top-level file listings, and the unclipped headers of representative files, asserting:
- `tree_role_claim(Dir, Role, ConfidencePct)` with one line of explanatory evidence.
- Supported Roles: `/source`, `/tests`, `/fixtures`, `/seed_data`, `/answer_key`, `/generated`, `/build_output`, `/vendored`, `/third_party_client`, `/docs`, `/archive`, `/experiments`, `/tooling`, `/agent_config`, `/infra`, `/data`, `/assets`.

### Deductive Treatment Policies in Mangle (`trees.mg`)
Mangle evaluates role claims and structural facts to derive:
- `tree_role(Dir, Role, Why)`: the canonical classified role.
- `tree_treatment(Dir, Treatment)`:
  - `/index_and_parse`: Full CodeDOM parsing, symbol indexing, and call graph construction (standard production source and test files).
  - `/index_names_only`: File paths and names are indexed so the agent knows they exist, but content is neither parsed, embedded, nor offered as code context (seed data, massive JSON/CSV datasets, answer keys).
  - `/exclude`: Directory is skipped entirely during normal walks (disposable experiments, dead archives, build outputs).

### Integration with `internal/workspace`
Derived treatments are exported to `.nerd/orientation/membership.json`. `internal/workspace` loads this artifact as an active overlay on top of git membership truth, exposing a `Treatment(rel string) string` accessor that guides search and indexing tools.

---

## 3. Dependencies and Private Systems Protection

The Go sensor (`internal/orient/deps.go`) identifies all external dependencies:
1. **Manifest Ingestion**: Parses package manifests (`pyproject.toml`, `requirements.txt`, `setup.cfg`, `package.json`, `go.mod`, `Cargo.toml`), emitting `dependency_seen(Name, Ecosystem, Evidence, Path)`.
2. **Install Source Detection**: Inspects install patterns, emitting `dependency_source(Name, Source, Detail)` where `Source` is `/registry`, `/path` (local path installs), `/git` (direct git URLs), or `/private_index` (`--extra-index-url` or private registry configurations).
3. **Local Knowledge Correlation**: Correlates dependencies with vendored client directories, local agent skills (`agent_source_topic`), and environment variable templates (`.env.example`).
4. **Registry Presence Verification**: For external candidates, queries public registry metadata (PyPI JSON or npm registry) using codeNERD's native browser. Public lookup results are cached under `.nerd/cache/registry/`. If a dependency was already derived as local-only, public lookups are completely bypassed.

### Deductive Policy (`deps.mg`)
- `dependency_private(Name, Why)`: Derives when a dependency is absent from public registries, installed from a private path or git repository, or described exclusively by internal skills and vendored client packages.
- **Enforcement Rule**: `orient_research_topic(Name, Topic)` is **strictly forbidden** from emitting public web research queries for any dependency flagged as `dependency_private`. Its domain knowledge must be synthesized exclusively from the repository's own internal skills, documentation, and client source code.

---

## 4. MCP Server Discovery & Consented Import

The Go sensor (`internal/orient/mcp.go`) discovers external Model Context Protocol server configurations:
- **Project Scope**: `.mcp.json`, `.claude/settings.json`, `.claude/settings.local.json`, `.codex/config.toml`, `.cursor/mcp.json`, `.vscode/mcp.json`.
- **User Scope**: `~/.claude.json` (top-level and project-specific blocks), `~/.codex/config.toml`, `~/.gemini/settings.json`.

### Credential Safety Invariant
The engine asserts only keys and transports:
- `mcp_server_seen(Name, Tool, Scope, Transport, Path)` (`Scope` $\in \{/\text{project}, /\text{user}\}$).
- `mcp_server_credential(Name, Kind, Key)` (`Kind` $\in \{/\text{header}, /\text{env}\}$).
- **CRITICAL**: Secret values are **never asserted as facts** and never written to logs.

### Policy & Import Staging (`mcp.mg`)
- `mcp_import_candidate(Name, Why)`: Derives when an external MCP server directly matches an active dependency or skill topic (e.g. an MCP server providing database or vector operations for a detected library).
- For user-scope servers (whose credentials reside outside the repository), the policy mandates explicit operator confirmation.

---

## 5. First-Boot Clarification Dialogue

Orientation avoids blocking `nerd init` with terminal prompts. Instead, it defers questions to the first interactive TUI session.

### Deductive Question Generation (`questions.mg`)
Policy derives `orient_question(ID, Topic, Kind, BestGuess, WhyAsk)` whenever:
1. Tree role classification remains ambiguous or conflicting.
2. A private dependency call falls below a high confidence threshold.
3. An MCP import candidate is discovered.
4. North Star vision derivation detects conflicting architectural claims.

`nerd init` serializes pending questions to `.nerd/orientation/questions.json`. Question prose is authored via JIT prompt atoms (`internal/prompt/atoms/orient/question_prompt.yaml`), not hardcoded Go strings.

### TUI Presentation and Durable Answers
On the first interactive launch of `nerd chat`:
1. The kernel loads pending questions from `.nerd/orientation/questions.json`.
2. The existing clarification loop (`cmd/nerd/chat/process_dream_delegation.go:26` `kernelClarification`, `cmd/nerd/chat/model_handlers.go:417, 539`) presents each question with its evidence and pre-selected `BestGuess`.
3. The operator can accept the default, select an alternate option, or skip.
4. Answers are permanently persisted to `.nerd/orientation/answers.json` and asserted as `orient_answer(ID, Choice)`. In Mangle policies, `orient_answer` immediately overrides all prior automated guesses.
5. If the operator approves an MCP import, codeNERD executes a **guarded config merge** against `.nerd/config.json`, inserting the server definition while preserving all surrounding formatting and comments, verified via `config.LoadUserConfig` and `Check`.

---

## 6. Verification Seams & Tests

1. `TestTreesPolicy_SeedDataClassification`: Asserts stats for a directory with 10,000 `.json` files and 50 commits; verifies that `tree_role` derives `/seed_data` and `tree_treatment` derives `/index_names_only`.
2. `TestPrivateDeps_BlocksWebResearch`: Configures an internal package installed from a local path; verifies that `dependency_private` derives and `orient_research_topic` emits zero web queries.
3. `TestGuardedConfigMerge_PreservesUserFormatting`: Executes a targeted MCP server insertion into an existing `.nerd/config.json` containing custom formatting and comments; asserts byte-level preservation outside the inserted object.
4. `TestTUIClarification_PresentsOrientationQuestions`: Exercises the chat model clarification handler with pending orientation questions, verifying correct display, choice recording, and persistence to `answers.json`.
