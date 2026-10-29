---
doc-class: governance
subsystem: orient
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: working-tree-C2a
supersedes: []
---

# 00 — Index — Docs/architecture/orient

_Governance map for this directory. Per `Docs/journeys/09-architecture-doc-standard.md:50-52`: read order, one line per file stating what it answers and when to read it, followed by the grounded-vs-hypothesized map._

- Last-verified: 2026-09-29
- Verified-against: `bb7bafac`
- ID ownership: `03-GAP-ANALYSIS.md` owns `GAP-ORIENT-01..23`.

## 1. Read Order (One Line Per File)

| # | File | What it answers — when to read it |
|---|------|-----------------------------------|
| 1 | `README.md` | What this directory is, why orientation exists, and where truth lives — read first. |
| 2 | `00-INDEX.md` (this file) | In what order to read the corpus and which claims are grounded versus hypothesized — read second; defines governance boundaries. |
| 3 | `01-VISION.md` | The north-star dream: how `nerd init` autonomously discovers repository reality, timeline, lineage, ecosystem, and north star without guessing — read before current state. |
| 4 | `02-CURRENT-STATE.md` + `IMPLEMENTED_SPEC.md` | What runs today in `init`, `world`, `research`, `northstar`, `docscheck`, `campaign`, and `session`, cited to exact source lines — read together as the baseline of today's behavior. |
| 5 | `WIRING-AND-NOT-BUILT.md` | Integration seams, uncalled helper paths, disjoint Code/Spec DAGs, and dormant assumptions in existing passes — read before capability specs. |
| 6 | `03-GAP-ANALYSIS.md` | The complete gap matrix (`GAP-ORIENT-01..23`) defining severity, blocking lanes, phases, and command-checkable exit criteria — read after current state. |
| 7 | `04-PRINCIPLES-AND-CONSTRAINTS.md` | Non-negotiable architectural laws and rulings (recency is not quality, no name-keyed decisions, paging over truncation, blind acceptance) — read before implementation. |
| 8 | `05-TIMELINE-AND-LINEAGE.md` | Capability spec for git history streaming, commit era derivation, document generation, and supersession chains (`I2a`). |
| 9 | `06-ECOSYSTEM-INGEST-AND-AGENTS.md` | Capability spec for foreign agent CLI ingest, cross-tool duplicate resolution, and dynamic shard agent materialization (`I1`). |
| 10 | `07-NORTH-STAR-DERIVATION.md` | Capability spec for document role classification, non-interactive vision derivation, and orientation persistence (`I2b`). |
| 11 | `08-DISCERNMENT-AND-QUESTIONS.md` | Capability spec for tree role discernment, private system detection, MCP server discovery, and first-boot TUI clarification (`O1`). |
| 12 | `10-SPEC-ALIGNMENT.md` | Capability spec for unified unit graph, link sensors, status derivation (`/aligned`, `/behind`, `/ahead`, `/missing`, `/stale`, `/not_trending`), spec-first turn gates, and native templates. |
| 13 | `11-WHAT-NEXT-FOUNDATIONS-UP.md` | Capability spec for foundations-up Kahn ordering over unit graphs, pluggable directions (`start_from`), parametric ranking, verbal justifications (`what_next`), and `/next`. |
| 14 | `12-DEBT-ORPHANS-AND-CLEANUP.md` | Capability spec for technical debt sensors, orphan code identification, durable operator clarification, `.nerd/orientation/answers.json`, and atomic CodeDOM cleanup transactions. |
| 15 | `09-MANGLE-SURFACE.md` | Exhaustive declaration index of all EDB and IDB predicates in orientation, spec alignment, what-next, and turn safety contracts. |
| 16 | `RISK-REGISTER-AND-DECISION-LOG.md` | Evaluation of operational risks (question fatigue, reflection false orphans, link hallucination, symbol link cost) and mitigation tripwires. |
| 17 | `OPEN-QUESTIONS.md` | Unresolved design edges and standing architectural invariants across orientation and spec alignment. |
| 18 | `TODO.md` | Concrete leaf execution queue directly traceable to `GAP-ORIENT` identifiers across Phases 1–7. |
| 19 | `corpus.toml` | Machine-readable corpus metadata and root declarations. |
| 20 | `adr/ADR-001..011` | Eleven governed Architectural Decision Records establishing witnesses and derivations for orientation and alignment invariants. |

## 2. Grounded vs Hypothesized

### 2a. Source-reviewed C2a integration

- Init orientation precedes profile and agent materialization
  (`internal/init/initializer.go:469`, `Initialize`).
- Strategic consumers receive the oriented vision and candidate evidence
  (`internal/init/phase_orient.go:258`, `persistOrientationKnowledge`).
- Agents consume the retained orientation engine
  (`internal/init/phase_ecosystem.go:115`, `integrateEcosystem`).
- Runtime evidence is pending; the incremental scan hook is outside C2a scope.

### 2b. HYPOTHESIZED / TARGET-STATE — Planned Design (Mark Planned)

The following areas describe target architecture specified in capability specifications (`05`, `06`, `07`, `08`, `10`, `11`, `12`) and orientation contracts. None of these have landed in production source code as of commit `bb7bafac`:

- **Orientation Engine**: `internal/orient/engine.go` and embedded Datalog files (`*.mg`).
- **Git Streaming History Pass**: Whole-repo streaming parser extracting `repo_file_history`, `repo_month`, `repo_span`.
- **Deductive Lineage and Eras**: Derivation of `repo_era`, `doc_generation`, `doc_burst`, `doc_evolved_into`, `doc_superseded`, `doc_live`.
- **Ecosystem Ingest**: Multi-tool parsing of `.claude`, `.codex`, `.agents`, `.gemini`, `.grok`, `.jules`, `.cursor`, `.roomodes`, and user-scoped memory into `agent_source*` facts.
- **Declarative Agent Deduction**: `ecosystem_agents.mg` deriving `agent_source_duplicate`, `agent_source_winner`, `orient_agent`, and `orient_agent_knowledge`.
- **Non-Interactive North Star Derivation**: `internal/northstar/derive.go` with `ClassifyDocuments` and `DraftVision` emitting `.nerd/orientation/README.md` and `.nerd/orientation/orientation.mg`.
- **Tree Role Discernment**: `internal/orient/trees.go` and `trees.mg` classifying directory roles and treatments (`/index_and_parse`, `/index_names_only`, `/exclude`).
- **Private Dependency Protection**: Detection of unlisted dependencies via manifests and local skills, preventing web research and enforcing internal knowledge derivation.
- **Consented MCP Discovery and Guarded Config Merge**: Scanning multi-agent MCP configurations and applying byte-preserving targeted merges to `.nerd/config.json`.
- **Unified Unit Graph**: Bipartite graph joining code units and spec units via `code_depends`, `spec_depends`, and `unit_realizes`.
- **Mangle Spec Status Derivation**: Datalog derivation of `/aligned`, `/behind`, `/ahead`, `/missing`, `/stale`, and `/not_trending` using git timestamps.
- **Spec-First Turn Gate**: Mandatory completion obligation in `coder_safety.mg` requiring spec updates alongside code changes, deriving `turn_missing_evidence(Turn, /spec_misaligned)`.
- **Learned Native Spec Templates**: Engine discovering foreign heading and table schemas (`repo_spec_template`) to generate and repair native specs.
- **Foundations-Up Work Planning**: Kahn topological sort over the unit graph filtered by `start_from(Kind)` and ranked parametrically (`what_next(Unit, Rank, Why)`), consumed by `/next` and `nerd next`.
- **Technical Debt & Orphan Sensors**: Go sensors emitting `code_is_shim`, `code_dark_field`, `code_debt`, and `code_is_deprecated` into Mangle.
- **Durable Orphan Dialogue**: Operator classification of un-specced code into `/feature`, `/experiment_develop`, `/experiment_park`, `/failed_experiment`, or `/trash`, persisted in git-committed `.nerd/orientation/answers.json`.
- **Polyglot Cleanup Transaction**: Multi-file CodeDOM refactoring engine supporting method repointing, cross-file moves, AST syntax checks, post-commit build/test verification, and automated rollback.

## 3. How to Cite This Directory

- **Shipped claims**: Cite `02-CURRENT-STATE.md`, `IMPLEMENTED_SPEC.md`, or the verified source `path:line` directly.
- **Planned claims**: Cite capability specs `05` through `08`, `10` through `12`, `09-MANGLE-SURFACE.md`, or specific `GAP-ORIENT-NN` rows from `03-GAP-ANALYSIS.md`.
- **Decisions**: Cite `adr/ADR-001..011` including status (`accepted-not-implemented` until witness verification resolves).

## C2a integration record

[13-INIT-AND-REORIENTATION.md](13-INIT-AND-REORIENTATION.md) specifies the single init orientation and boot refresh. Its source exists; acceptance commands and the incremental hook remain pending.

