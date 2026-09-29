---
doc-class: deep-dive
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 09 — Mangle Surface — Relational Schema & Predicate Registry

This document defines the complete Mangle Datalog language surface for the `internal/orient` engine. It serves as the immutable schema contract across Lanes `I1`, `I2a`, `I2b`, and `O1`.

---

## 1. Engine Architecture & Policy File Ownership

The orientation engine is an independent Mangle deductive instance instantiated via `internal/orient.NewEngine(cfg *config.OrientConfig)`. It operates separately from the main session kernel (following the precedent of `internal/context/working_set.mg`).

Policy files are embedded into the Go binary via a `go:embed *.mg` glob, allowing additions and modifications to Datalog rules without modifying Go loader routines.

| Subsystem File | Owning Lane | Responsibility & Declarations |
|---|---|---|
| `internal/orient/timeline.mg` | Lane `I2a` | Computes commit eras, monthly aggregations, and repository span derivations. |
| `internal/orient/lineage.mg` | Lane `I2a` | Computes document generations, commit bursts, evolution links, supersession, and read candidate attention. |
| `internal/orient/ecosystem_agents.mg` | Lane `I1` | Computes cross-tool duplicate resolution, winning skills, and dynamic shard agent provisioning. |
| `internal/orient/trees.mg` | Lane `O1` | Computes tree roles and operational search treatments (`/index_and_parse`, `/index_names_only`, `/exclude`). |
| `internal/orient/deps.mg` | Lane `O1` | Computes private dependency boundaries and research topic suppression. |
| `internal/orient/mcp.mg` | Lane `O1` | Evaluates external MCP server candidates for consented configuration merge. |
| `internal/orient/questions.mg` | Lane `O1` | Derives first-boot clarification questions for low-confidence or ambiguous classifications. |

---

## 2. Extensional Database (EDB) Predicates — Asserted by Go

Go sensors assert raw facts into the orientation engine using `Engine.Assert(facts []types.Fact)`.

### Git History & Document Topography (Lane `I2a`)

```prolog
# Committer history for every member file from streaming git log
Decl repo_file_history(Path, FirstUnix, LastUnix, Commits, ActiveDays).

# Monthly development activity metrics
Decl repo_month(MonthIndex, Label, Commits, FilesAdded, DocsAdded).

# Global repository boundary and shallow clone indicator
Decl repo_span(FirstUnix, LastUnix, TotalCommits, Shallow).

# Discovered documentation files
Decl doc_file(Path, Dir, Bytes, Headings).

# Resolved relative markdown cross-links
Decl doc_link(FromPath, ToPath).

# Pairwise document semantic similarity from embedding centroids (scaled 0..1000)
Decl doc_similar(A, B, Permille).
```

### Foreign Agent Ecosystem (Lane `I1`)

```prolog
# Discovered coding-agent configurations, skills, subagents, and memories
Decl agent_source(ID, Tool, Kind, Name, Path, Tracked).
# Tool: /claude /codex /agents /gemini /antigravity /grok /roo /jules /cursor /copilot /nerd /other
# Kind: /skill /subagent /rule /memory /instructions /command /journal /mode
# Tracked: /yes (git-tracked) | /no (local-only or user-scoped)

# Content hash of normalized source body
Decl agent_source_digest(ID, Digest).

# Extracted topic tags or descriptions
Decl agent_source_topic(ID, Topic).

# Subtree confinement directory for scoped rules/instructions
Decl agent_source_scope(ID, Dir).
```

### Document Role Transduction (Lane `I2b`)

```prolog
# LLM-asserted document functional role claims
Decl doc_role_claim(Path, Role, ConfidencePct).
# Role: /vision /north_star_draft /origin_design /spec /plan /report /standard
#       /guide /journal /readme /instructions /reference /archive /generated

# Normalized architectural and semantic theme keywords
Decl doc_theme(Path, Theme).
```

### Tree Topography, Dependencies, and MCP (Lane `O1`)

```prolog
# Structural statistics for repository directories
Decl tree_stats(Dir, Files, Bytes, TrackedFiles, IgnoredEntriesBelow, LastUnix, Commits, DistinctAuthorsOrZero).
Decl tree_ext(Dir, Ext, Count).
Decl tree_name(Dir, Base).
Decl tree_role_claim(Dir, Role, ConfidencePct).

# Dependencies discovered from manifests and install parameters
Decl dependency_seen(Name, Ecosystem, Evidence, Path).
Decl dependency_source(Name, SourceKind, Detail).
# SourceKind: /registry | /path | /git | /private_index

# External MCP servers discovered from project and user configs
Decl mcp_server_seen(Name, Tool, Scope, Transport, Path).
# Scope: /project | /user; Transport: /stdio | /http | /sse
Decl mcp_server_credential(Name, CredKind, KeyName).
# CredKind: /header | /env (KeyName only; NEVER secret values)

# Persisted operator answers from previous clarification cycles
Decl orient_answer(QuestionID, SelectedChoice).
```

---

## 3. Intensional Database (IDB) Predicates — Derived by Mangle

Mangle rules compute deductive fixpoints over EDB facts.

### Timeline, Lineage, and Attention (Lane `I2a`)

```prolog
# Temporal development eras derived from monthly activity
Decl repo_era(EraIndex, StartMonth, EndMonth, Kind).
# Kind: /wave | /lull

# Document generational vintage relative to repository eras
Decl doc_generation(Path, Generation).
# Generation: /origin | /early | /middle | /recent

# Documents created or revised in a concentrated commit burst
Decl doc_burst(Path).

# Document evolution chains based on similarity, timestamps, and themes
Decl doc_evolved_into(OldPath, NewPath).

# Documents superseded by newer, live successors
Decl doc_superseded(OldPath, ByPath).

# Active documents touched in recent eras or central in link networks
Decl doc_live(Path).

# Authoritative origin documents establishing baseline design
Decl origin_source(Path, Why).

# Weighted vision sources informing the North Star
Decl vision_source(Path, WeightPct, Why).

# Attention selection: bounded documents that the model must read in full
Decl orient_read_candidate(Path, Reason).
```

### Agent Deduplication and Provisioning (Lane `I1`)

```prolog
# Detected duplicate skills or subagents across tools
Decl agent_source_duplicate(IDA, IDB).

# Authoritative winning source selected from evidence
Decl agent_source_winner(ID, Why).

# Derived specialist shard agents to materialize
Decl orient_agent(Name, Why).

# Knowledge sources routed to specific agent databases
Decl orient_agent_knowledge(AgentName, SourceID).

# Topics requiring external research due to lack of local coverage
Decl orient_research_topic(AgentName, Topic).
```

### Tree Discernment, Private Systems, and Dialogue (Lane `O1`)

```prolog
# Canonical classified role for a directory tree
Decl tree_role(Dir, Role, Why).

# Operational search and parsing treatment for a tree
Decl tree_treatment(Dir, Treatment).
# Treatment: /index_and_parse | /index_names_only | /exclude

# Dependencies identified as internal or proprietary
Decl dependency_private(Name, Why).

# Discovered MCP servers staged for operator import approval
Decl mcp_import_candidate(ServerName, Why).

# Questions formulated for operator clarification in first-boot TUI
Decl orient_question(QuestionID, Topic, Kind, BestGuess, WhyAsk).
```

---

## 4. Stratification and Safety Invariants

All Mangle rules in `internal/orient/*.mg` must satisfy codeNERD's Datalog execution safety constraints:
1. **Positive Variable Binding**: Negated literals (`!predicate(...)`) are permitted only when all variables in the literal have already been bound by preceding positive atoms in the rule body.
2. **Single-Argument Negation Projection**: In this engine version, negating a multi-argument predicate with wildcards (`!foo(X, _)`) does not safely exclude. Rules must project to a single-argument predicate first (`has_foo(X) :- foo(X, _).`) and negate the projection (`!has_foo(X)`).
3. **Pipeline Aggregation Syntax**: All aggregations must use pipe syntax:
   `|> do fn:group_by([Var1, ...]), let Result = fn:count()`.
4. **Configuration Parameter Decoupling**: Hardcoded numerical thresholds are prohibited in `.mg` rules. All thresholds must be loaded as `config_param(/orient_<key>, Value)` facts asserted from `config.OrientConfig`.
