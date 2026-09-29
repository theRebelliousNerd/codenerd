---
doc-class: deep-dive
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# 09 — Mangle Surface — Relational Schema & Predicate Registry

This document defines the complete Mangle Datalog language surface for the `internal/orient` engine and related runtime policy extensions. It serves as the immutable schema contract across Lanes `I1`, `I2a`, `I2b`, `O1`, and the spec alignment and what-next engines.

---

## 1. Engine Architecture & Policy File Ownership

The orientation engine operates as an independent Mangle deductive instance instantiated via `internal/orient.NewEngine(cfg *config.OrientConfig)`. In addition, spec alignment, what-next ordering, and turn safety obligations integrate into the main session kernel.

Policy files are embedded into the Go binary via `go:embed *.mg` globs, allowing additions and modifications to Datalog rules without altering Go loader routines.

| Subsystem File | Owning Lane | Responsibility & Declarations |
|---|---|---|
| `internal/orient/timeline.mg` | Lane `I2a` | Computes commit eras, monthly aggregations, and repository span derivations. |
| `internal/orient/lineage.mg` | Lane `I2a` | Computes document generations, commit bursts, evolution links, supersession, and read candidate attention. |
| `internal/orient/ecosystem_agents.mg` | Lane `I1` | Computes cross-tool duplicate resolution, winning skills, and dynamic shard agent provisioning. |
| `internal/orient/trees.mg` | Lane `O1` | Computes tree roles and operational search treatments (`/index_and_parse`, `/index_names_only`, `/exclude`). |
| `internal/orient/deps.mg` | Lane `O1` | Computes private dependency boundaries and research topic suppression. |
| `internal/orient/mcp.mg` | Lane `O1` | Evaluates external MCP server candidates for consented configuration merge. |
| `internal/orient/questions.mg` | Lane `O1` | Derives first-boot clarification questions for low-confidence or ambiguous classifications. |
| `internal/orient/spec_status.mg` | Lane TBD (Spec Alignment) | Evaluates the unified unit graph and derives status atoms: `/aligned`, `/behind`, `/ahead`, `/missing`, `/stale`, `/not_trending`. |
| `internal/orient/what_next.mg` | Lane TBD (What-Next Engine) | Computes foundations-up Kahn ordering, direction filtering (`start_from`), and parametric ranking with verbal derivations (`what_next`). |
| `internal/orient/debt_orphans.mg` | Lane TBD (Debt & Orphans) | Evaluates technical debt, forwarding shims, dark fields, and identifies orphan code candidates for operator dialogue. |
| `internal/core/defaults/policy/coder_safety.mg` | Runtime Kernel (Coder Safety) | Enforces the spec-first completion obligation on coding turns, deriving `turn_missing_evidence(Turn, /spec_misaligned)`. |

---

## 2. Extensional Database (EDB) Predicates — Asserted by Go

Go sensors assert raw facts into the orientation and runtime engines using `Engine.Assert(facts []types.Fact)` and `kernel.Assert(...)`.

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

### Unified Unit Graph & Realization Links (Lanes TBD, Decisions D1, D2)

```prolog
# Code Units: declared packages and symbols
Decl code_unit(UnitID, UnitKind, FilePath, Signature).
# UnitKind: /package /file /type /interface /function /endpoint

# Inferred architectural kind of unit
Decl code_unit_kind(UnitID, Kind).
# Kind: /types /ingress /egress /api /persistence /ui /tooling

# Declared symbols inside code units
Decl code_declares(UnitID, Symbol, SymbolType, FilePath).

# Code-to-spec citations found in comments, docstrings, or test names
Decl code_cites(UnitID, Symbol, CapID).

# Structural dependencies between code units
Decl code_depends(FromUnit, ToUnit).

# Last git committer timestamp for code unit (%ct epoch)
Decl code_mtime(UnitID, LastCommitterUnix).

# Spec Units: specification sections, North Star capabilities, and gap rows
Decl spec_unit(SpecID, SpecKind, Title, NorthstarRef).
# SpecKind: /section /capability /requirement /gap_row

# Capabilities declared by a specification unit
Decl spec_declares(SpecID, CapID, Statement, ExitCriterion).

# Targets cited by a specification unit (/symbol, /file, /test, /endpoint)
Decl spec_cites(SpecID, CapID, TargetType, Target).

# Hierarchical dependencies between specification units
Decl spec_depends(ChildSpec, ParentSpec).

# Last git committer timestamp for spec unit (%ct epoch)
Decl spec_mtime(SpecID, LastCommitterUnix).

# Semantic realization link between code unit and spec unit
Decl unit_realizes(CodeUnit, SpecUnit).

# Model link transduction claim with confidence and evidence
Decl spec_link_claim(CodeUnit, SpecUnit, ConfidencePct, EvidenceSnippet).

# Behavioral test witness execution status
Decl unit_tested_by(CodeUnit, TestUnit, GateStatus).
# GateStatus: /passing | /failing | /skipped
```

### Native Spec Templates & Runtime Quality (Lane TBD, Decision D8)

```prolog
# Learned specification template format for foreign repository
Decl repo_spec_template(Format, PathPattern, HeaderList).

# Runtime architectural documentation problem emitted by generalized docscheck
Decl doc_problem(Pkg, File, Code, Message).
```

### Technical Debt, Shims, and Dark Fields (Lane TBD, Decision D6)

```prolog
# Detected forwarding shim (wrapper function, alias, re-export)
Decl code_is_shim(SymbolRef, TargetSymbolRef, ShimKind).
# ShimKind: /forwarder | /alias | /reexport

# Deprecation marker citing replacement target
Decl code_is_deprecated(SymbolRef, ReplacementRef).

# Dark struct field read by production code but never written
Decl code_dark_field(StructName, FieldName, FilePath).

# Static debt findings (unreferenced symbol, dead unreachable code)
Decl code_debt(SymbolRef, DebtKind, FilePath).
# DebtKind: /unreferenced | /unreachable

# Near-duplicate element body based on embedding similarity
Decl code_near_duplicate(ElementA, ElementB, SimilarityPermille).
```

### Turn Evidence & Operator Decisions (Lanes TBD, Decisions D4, D5, D7)

```prolog
# Files written during current turn
Decl turn_written_file(Turn, FilePath).

# Spec unit updated during current turn
Decl turn_spec_updated(Turn, SpecID).

# Pluggable seed direction for what-next foundations ordering
Decl start_from(Kind).
# Kind: /types | /ingress | /egress | /api | /persistence | /ui | /tooling

# Durable operator classification of orphan code units
Decl code_classified(UnitID, Classification).
# Classification: /feature | /experiment_develop | /experiment_park | /failed_experiment | /trash
```

---

## 3. Intensional Database (IDB) Predicates — Derived by Mangle

Mangle rules compute deductive fixpoints over extensional facts.

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

### Unit Status Derivation (Lane TBD, Decision D3)

```prolog
# Comprehensive derived status for a unit
Decl unit_status(UnitID, Status).
# Status: /aligned | /behind | /ahead | /missing | /stale | /not_trending

# Specific decomposed status projections
Decl unit_spec_ahead(UnitID).
Decl unit_spec_behind(UnitID).
Decl unit_spec_missing(UnitID).
Decl unit_spec_aligned(UnitID).
Decl unit_spec_stale(UnitID).
Decl unit_not_trending(UnitID).

# Orphan code unit lacking specification and classification
Decl unit_is_orphan(UnitID).
```

### Foundations-Up What-Next Derivation (Lane TBD, Decision D4)

```prolog
# Units matching active seed direction filter
Decl foundations_candidate(UnitID).

# Unmet prerequisite dependency check
Decl unit_dep_unmet(UnitID).

# Unit whose prerequisites are fully satisfied (ready for implementation)
Decl unit_ready(UnitID).

# Parametric readiness score accumulated from config weights
Decl what_next_score(UnitID, Score).

# Ranked, verbally explained recommendation output
Decl what_next(UnitID, Rank, Why).
```

### Spec-First Turn Safety Gates (Runtime Kernel, Decision D5)

```prolog
# Code unit touched by current turn's writes
Decl turn_touches_unit(Turn, UnitID).

# Violation derived when write turn touches code without updating spec
Decl turn_has_spec_misalignment(Turn).

# Failure reason asserted when turn cannot verify due to spec misalignment
Decl turn_missing_evidence(Turn, Reason).
# Reason: /spec_misaligned (alongside existing /build_not_green, /tests_not_green, etc.)
```

---

## 4. Stratification and Safety Invariants

All Mangle rules across orientation and runtime policies must satisfy codeNERD's Datalog execution safety constraints:
1. **Positive Variable Binding**: Negated literals (`!predicate(...)`) are permitted only when all variables in the literal have already been bound by preceding positive atoms in the rule body.
2. **Single-Argument Negation Projection**: In this engine version, negating a multi-argument predicate with wildcards (`!foo(X, _)`) does not safely exclude. Rules must project to a single-argument predicate first (`has_foo(X) :- foo(X, _).`) and negate the projection (`!has_foo(X)`).
3. **Pipeline Aggregation Syntax**: All aggregations must use pipe syntax:
   `|> do fn:group_by([Var1, ...]), let Result = fn:count()`.
4. **Configuration Parameter Decoupling**: Hardcoded numerical thresholds or ranking weights are prohibited in `.mg` rules. All thresholds must be loaded as `config_param(Key, Value)` facts asserted from Go configuration.
