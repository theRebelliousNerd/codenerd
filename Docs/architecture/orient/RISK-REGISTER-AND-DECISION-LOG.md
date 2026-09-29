---
doc-class: governance
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# RISK-REGISTER-AND-DECISION-LOG — Orientation Risk Governance

This document records the operational risks, failure probabilities, architectural consequences, and retirement criteria for the `internal/orient` subsystem and related spec alignment capabilities, accompanied by the authoritative decision log.

---

## 1. Operational Risk Register

| Risk ID | Description & Context | Likelihood | Consequence | Mitigation & Architectural Tripwire | Retirement Criteria |
|---|---|---|---|---|---|
| `RISK-ORIENT-01` | **Shallow Git Clones**<br>CI pipelines or developers clone with `git clone --depth 1`. Whole-history streaming sees only 1 commit; eras cannot be derived. | Medium | High | Go sensor checks `git rev-parse --is-shallow-repository`. Asserts `repo_span(..., /yes)`. Mangle policy refuses to partition eras on shallow repositories, treating history as a single recent baseline with diagnostic notice. | Test `TestShallowCloneRefusal` passes; verified refusal to derive false eras on shallow clone fixture. |
| `RISK-ORIENT-02` | **One-Day Commit Bursts**<br>A major architectural thesis or complete subsystem specification is committed in a single burst on one day, risking misclassification as an ephemeral lull. | High | High | `doc_burst(Path)` derives when commits are concentrated on few active days (`Commits >= 5`, `ActiveDays <= 2`). `vision_source` policy boosts the weight of burst documents rather than penalizing them. | Test `TestBurstDocumentWeighting` proves a 1-day burst specification outranks an older, slowly edited changelog. |
| `RISK-ORIENT-03` | **Embedding Cost & Latency on Massive Doc Sets**<br>A repository containing 16,000 markdown files (e.g. generated API docs or release notes) exhausts embedding budgets or takes hours. | High | High | Documents are pre-filtered by size, link graph in-degree, and directory role. Only candidate documents undergo chunked embedding. Vectors are cached under `.nerd/cache/embeddings/` keyed by SHA-256 hash. | Benchmarking 16,000 files completes in under 30 seconds by skipping generated/excluded trees. |
| `RISK-ORIENT-04` | **LLM Inference Cost of Document Role Transduction**<br>Invoking LLM classification (`ClassifyDocuments`) across thousands of documentation files causes extreme token consumption. | High | High | Strict structural attention: Mangle derives `orient_read_candidate` first based on topology and history, capping candidates at `config_param(/orient_read_candidate_budget, 24)`. LLM classifies only the candidate set. | Test verifies that in a 1,000-doc repository, exactly $\le 24$ documents are submitted to the LLM client. |
| `RISK-ORIENT-05` | **Private-Registry False Negatives**<br>A proprietary internal package is misclassified as public, causing research tools to issue external search queries that leak proprietary names. | Medium | Critical | Multi-signal detection: checks manifest install sources (`file:`, `git:`, `--extra-index-url`), vendored client packages, and local agent skills. If ANY local signal matches, `dependency_private` derives immediately without querying public registries. | Test verifies that an internal package declared in `.env.example` and local skills derives `dependency_private` and executes 0 network calls. |
| `RISK-ORIENT-06` | **MCP Credential Exposure**<br>Importing external MCP server configurations exposes sensitive API tokens or bearer headers into git-tracked files or fact logs. | Medium | Critical | EDB facts record only credential key names (`mcp_server_credential(..., /header, "X-API-Key")`), never values. Secret values are read only upon explicit operator confirmation and written directly into protected config. | Test asserts that secret strings never appear in Mangle facts, logs, or `.nerd/orientation/README.md`. |
| `RISK-ORIENT-07` | **Orientation Drift Over Time**<br>Repository architecture evolves across weeks of active development while `.nerd/orientation/` retains stale initial baseline facts. | High | Medium | `nerd orient --refresh` evaluates git commit delta since orientation timestamp. If significant commit waves are detected, the system recommends re-orientation, preserving durable operator answers from `answers.json`. | Test verifies that re-running orientation incorporates new commit eras while preserving existing operator answers. |
| `RISK-ORIENT-08` | **Question Fatigue on Large Repositories**<br>An operator opening an enterprise repository with hundreds of undocumented modules is confronted with dozens of orphan clarification prompts, inducing prompt fatigue and blind dismissals. | High | High | Group questions strictly by package directory and enforce a hard session cap via `config_param(/orient_orphan_question_cap, 5)`. Transduction pre-selects intelligent best guesses with evidence quotes. | Test `TestOrphanClarification_SessionCapEnforced` verifies that at most 5 questions are presented per session regardless of total orphan count. |
| `RISK-ORIENT-09` | **False Orphan Calls on Reflection/DI Code**<br>Handlers, controllers, and services registered via reflection, runtime dependency injection, or string lookups have zero static AST call sites, risking false orphan detection and accidental deletion. | Medium | Critical | Static AST sensors inspect dependency injection configuration annotations and registration manifests; operator dialogue is mandatory before deletion; post-commit test verification gates catch regressions and trigger immediate rollback. | Test `TestOrphanDetection_ReflectionRegistrationAware` proves registered classes are exempted from orphan classification. |
| `RISK-ORIENT-10` | **Link-Claim Hallucination by Model Transduction**<br>Model transduction asserts spurious realization links between conceptually similar but functionally unrelated code and specification units. | Medium | High | Dual-gate verification: candidate links must exceed a strict vector embedding similarity floor before reaching the model; model claims require explicit confidence scores $\ge 85\%$ with verbatim evidence snippets; low-confidence claims route to operator clarification. | Test `TestLinkTransduction_HallucinationRejection` verifies that ungrounded link claims below the confidence threshold are refused by Mangle policy. |
| `RISK-ORIENT-11` | **Computational Cost of Symbol-Level Realization Links**<br>In repositories with $>50\text{k}$ symbols, computing pairwise Cartesian similarity between all symbols and specification requirements causes extreme embedding overhead and memory exhaustion. | High | High | Two-tier linking: status evaluation and search scoping occur at the package unit grain; symbol-level links are evaluated lazily only within the package's local dependency cone and cached by SHA-256 content hashes. | Benchmark `BenchmarkSymbolLinking_50kSymbols` completes in $<15$ seconds with $<100\text{MB}$ memory overhead. |

---

## 2. Architectural Decision Log

### DEC-001: Independent Mangle Engine for Orientation
- **Context**: Evaluating whether orientation reasoning should execute inside the main session kernel or in a dedicated engine instance.
- **Decision**: Construct an independent `Engine` instance in `internal/orient` with embedded `*.mg` policies.
- **Rationale**: Orientation executes before session shards exist. Isolating the orientation engine prevents fact contamination in the main session kernel and ensures zero runtime performance overhead during normal coding turns.

### DEC-002: Committer Timestamp as the Sole Chronological Standard
- **Context**: Filesystem `mtime` vs. git author date vs. git committer date.
- **Decision**: Ground all temporal reasoning exclusively in git committer epoch timestamp (`%ct`).
- **Rationale**: `mtime` is volatile and resets across checkouts. Author dates can be arbitrarily backdated or distorted by rebase operations. Committer timestamps accurately reflect the repository's integration sequence.

### DEC-003: Pure Sensory Role for Go; All Decisions in Mangle
- **Context**: Choosing between imperative Go procedural logic and declarative Datalog for agent provisioning and document lineage.
- **Decision**: Go code functions strictly as a sensor, asserting EDB facts. All executive decisions (eras, generations, winners, agents, tree roles, questions) are derived in Mangle.
- **Rationale**: Fulfills the foundational codeNERD vision ("Clean fixpoint, not clean loop"). Eliminates complex Go branching and renders all orientation decisions inspectable and explainable via Mangle query provenance.

### DEC-004: Non-Interactive North Star Synthesis
- **Context**: The existing North Star wizard is tightly coupled to interactive Bubbletea TUI turns.
- **Decision**: Extract North Star synthesis into a non-interactive library (`internal/northstar/derive.go`) executed during `nerd init`, refactoring the TUI wizard into a thin wrapper.
- **Rationale**: Ensures `nerd init` produces a complete, functional world model in headless CI environments without requiring human chat interaction.

### DEC-005: Unified Bipartite Unit Graph for Code and Specifications (ADR-008)
- **Context**: Code dependency graphs and North Star specification graphs were completely disjoint, preventing derivation of codebase readiness and architectural alignment.
- **Decision**: Construct a unified unit graph with status grain at the package directory level and realization links at the symbol/file level, joined by `unit_realizes` facts.
- **Rationale**: Unifies code structure and architectural intent into a single queryable model; enables mathematical derivations of `/aligned`, `/behind`, `/ahead`, `/missing`, `/stale`, and `/not_trending`.

### DEC-006: Derived Foundations-Up What-Next Ordering (ADR-009)
- **Context**: Models hallucinate suggestions or grep for TODO comments when asked what to work on next, lacking awareness of dependency readiness.
- **Decision**: Extend Kahn topological ordering from `recurse.mg` across the unified unit graph. Allow pluggable starting directions via `start_from(Kind)` and rank ready units with parametric configuration weights, deriving verbal justification chains.
- **Rationale**: Inverts control from ungrounded model suggestions to deterministic logic derivations; ensures that work progresses from stable foundations upward.

### DEC-007: Spec-First Completion Obligation on Coding Turns (ADR-010)
- **Context**: Code changes routinely drift ahead of specifications because session turn verification gates evaluate only compiler, test, coverage, and vet outputs.
- **Decision**: Mandate that any coding turn modifying public surfaces or behavior of a covered unit must update the realizing specification in the same change, refusing turn completion with `/spec_misaligned` missing evidence if omitted.
- **Rationale**: Eliminates specification decay by making spec maintenance an unbypassable completion invariant enforced by constitutional Mangle policy.

### DEC-008: Durable, Team-Shared Orphan Classification (ADR-011)
- **Context**: Un-specced code is either deleted aggressively or ignored indefinitely, and user decisions are lost between sessions.
- **Decision**: Present orphan code units to the operator through the `kernelClarification` dialogue, capped per session, and persist choices (`/feature`, `/experiment_develop`, `/experiment_park`, `/failed_experiment`, `/trash`) in the git-committed file `.nerd/orientation/answers.json`.
- **Rationale**: Preserves developer intent across team members and clone boundaries; eliminates repetitive questioning; grounds automated cleanup in human decisions.
