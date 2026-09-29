---
doc-class: governance
subsystem: orient
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 00 — Index — Docs/architecture/orient

_Governance map for this directory. Per `Docs/journeys/09-architecture-doc-standard.md:50-52`: read order, one line per file stating what it answers and when to read it, followed by the grounded-vs-hypothesized map._

- Last-verified: 2026-09-29
- Verified-against: `e056692c`
- ID ownership: `03-GAP-ANALYSIS.md` owns `GAP-ORIENT-01..13`.

## 1. Read Order (One Line Per File)

| # | File | What it answers — when to read it |
|---|------|-----------------------------------|
| 1 | `README.md` | What this directory is, why orientation exists, and where truth lives — read first. |
| 2 | `00-INDEX.md` (this file) | In what order to read the corpus and which claims are grounded versus hypothesized — read second; defines governance boundaries. |
| 3 | `01-VISION.md` | The north-star dream: how `nerd init` autonomously discovers repository reality, timeline, lineage, ecosystem, and north star without guessing — read before current state. |
| 4 | `02-CURRENT-STATE.md` + `IMPLEMENTED_SPEC.md` | What runs today in `init`, `world`, `research`, and `northstar`, cited to exact source lines — read together as the baseline of today's behavior. |
| 5 | `WIRING-AND-NOT-BUILT.md` | Integration seams, uncalled helper paths, and dormant assumptions in existing initialization and scanning passes — read before capability specs. |
| 6 | `03-GAP-ANALYSIS.md` | The complete gap matrix (`GAP-ORIENT-01..13`) defining severity, blocking lanes, phases, and command-checkable exit criteria — read after current state. |
| 7 | `04-PRINCIPLES-AND-CONSTRAINTS.md` | Non-negotiable architectural laws and rulings (recency is not quality, no name-keyed decisions, paging over truncation, blind acceptance) — read before implementation. |
| 8 | `05-TIMELINE-AND-LINEAGE.md` | Capability spec for git history streaming, commit era derivation, document generation, and supersession chains (`I2a`). |
| 9 | `06-ECOSYSTEM-INGEST-AND-AGENTS.md` | Capability spec for foreign agent CLI ingest, cross-tool duplicate resolution, and dynamic shard agent materialization (`I1`). |
| 10 | `07-NORTH-STAR-DERIVATION.md` | Capability spec for document role classification, non-interactive vision derivation, and orientation persistence (`I2b`). |
| 11 | `08-DISCERNMENT-AND-QUESTIONS.md` | Capability spec for tree role discernment, private system detection, MCP server discovery, and first-boot TUI clarification (`O1`). |
| 12 | `09-MANGLE-SURFACE.md` | Exhaustive declaration index of all EDB and IDB predicates in the orientation contract, stratified rules, and lane ownership. |
| 13 | `RISK-REGISTER-AND-DECISION-LOG.md` | Evaluation of operational risks (shallow clones, burst commits, embedding costs, credential leakage) and mitigation tripwires. |
| 14 | `OPEN-QUESTIONS.md` | Unresolved design edges and standing architectural invariants across the orientation boundary. |
| 15 | `TODO.md` | Concrete leaf execution queue directly traceable to `GAP-ORIENT` identifiers. |
| 16 | `corpus.toml` | Machine-readable corpus metadata and root declarations. |
| 17 | `adr/ADR-001..007` | Seven governed Architectural Decision Records establishing witnesses and derivations for orientation invariants. |

## 2. Grounded vs Hypothesized

### 2a. GROUNDED — Code-Verified Claims (Cite Freely)

The following areas reflect current repository source code re-verified against commit `e056692c`:

- **Initialization Phase Pipeline**: `internal/init/initializer.go:460-621` orchestrates 22 phases. Phase 1 opens and closes `northstar.NewStore` without writing vision data (`internal/init/initializer.go:796-804`). Next steps instruct user to run `/northstar` manually (`internal/init/initializer.go:1447`).
- **Strategic Knowledge Disconnection**: `internal/init/initializer.go:968-985` invokes `generateStrategicKnowledge` (`internal/init/strategic_knowledge.go:25, 68-120`), which stores vision solely in `.nerd/knowledge.db` as atom `strategic/vision` (`internal/init/strategic_knowledge.go:616-663`), completely bypassing `internal/northstar`.
- **Hardcoded Documentation Scoring**: `internal/init/strategic_knowledge.go:241-260` scores files by fixed name (`CLAUDE.md` priority 0, `README.md`/`VISION.md` priority 1). Lines 305-308 prune all hidden directories except `.github` and `.claude`, ignoring `.agents`, `.codex`, `.gemini`, `.grok`, `.jules`, and `.cursor`.
- **Imperative Agent Selection Switch**: `internal/init/agents.go:373-626` (`determineRequiredAgents`) selects shard agents via hardcoded Go `switch` statements over language and dependencies.
- **Scanner Hidden Directory Pruning**: `internal/world/fs.go:226-263` prunes dot-directories except `.github`, `.vscode`, `.circleci`, `.config`, causing complete omission of foreign agent configuration trees.
- **North Star Interactive Truncation**: `cmd/nerd/chat/northstar_llm.go:185-242` truncates document input to 10,000 characters (`northstar_llm.go:201-203`) and requires Bubbletea TUI message handling.
- **Clarification UI Seam**: `cmd/nerd/chat/process_dream_delegation.go:26` (`kernelClarification`) and `cmd/nerd/chat/model_handlers.go:417, 539` implement the working TUI clarification flow.
- **MCP Configuration Model**: `internal/config/integrations.go:12-45` defines `IntegrationsConfig` and `MCPServerIntegration`.
- **Git Tracking Precedent**: `internal/docscheck/docscheck.go:723-740` demonstrates safe zero-copy execution of `git -C <abs> ls-files -z`.

### 2b. HYPOTHESIZED / TARGET-STATE — Planned Design (Mark Planned)

The following areas describe target architecture specified in implementation briefs (`I1`, `I2a`, `I2b`, `O1`, `L1`, `P1`, `P2`, `P3`, `R1`, `R2`) and orientation contract `_orient_contract.txt`. None of these have landed in production source code as of commit `e056692c`:

- **Orientation Engine**: `internal/orient/engine.go` and embedded Datalog files (`*.mg`).
- **Git Streaming History Pass**: Whole-repo streaming parser extracting `repo_file_history`, `repo_month`, `repo_span`.
- **Deductive Lineage and Eras**: Derivation of `repo_era`, `doc_generation`, `doc_burst`, `doc_evolved_into`, `doc_superseded`, `doc_live`.
- **Ecosystem Ingest**: Multi-tool parsing of `.claude`, `.codex`, `.agents`, `.gemini`, `.grok`, `.jules`, `.cursor`, `.roomodes`, and user-scoped memory into `agent_source*` facts.
- **Declarative Agent Deduction**: `ecosystem_agents.mg` deriving `agent_source_duplicate`, `agent_source_winner`, `orient_agent`, and `orient_agent_knowledge`.
- **Non-Interactive North Star Derivation**: `internal/northstar/derive.go` with `ClassifyDocuments` and `DraftVision` emitting `.nerd/orientation/README.md` and `.nerd/orientation/orientation.mg`.
- **Tree Role Discernment**: `internal/orient/trees.go` and `trees.mg` classifying directory roles and treatments (`/index_and_parse`, `/index_names_only`, `/exclude`).
- **Private Dependency Protection**: Detection of unlisted dependencies via manifests and local skills, preventing web research and enforcing internal knowledge derivation.
- **Consented MCP Discovery and Guarded Config Merge**: Scanning multi-agent MCP configurations and applying byte-preserving targeted merges to `.nerd/config.json`.
- **First-Boot Clarification**: Automated derivation of `orient_question` and persistence of `orient_answer`.

## 3. How to Cite This Directory

- **Shipped claims**: Cite `02-CURRENT-STATE.md`, `IMPLEMENTED_SPEC.md`, or the verified source `path:line` directly.
- **Planned claims**: Cite capability specs `05` through `08`, `09-MANGLE-SURFACE.md`, or specific `GAP-ORIENT-NN` rows from `03-GAP-ANALYSIS.md`.
- **Decisions**: Cite `adr/ADR-001..007` including status (`accepted-not-implemented` until witness verification resolves).
