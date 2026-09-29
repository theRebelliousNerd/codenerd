---
doc-class: governance
subsystem: orient
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# orient

> Verified 2026-09-29 against `e056692c` (`main`).

This directory is the governed architecture corpus for `internal/orient` — the subsystem that enables `nerd init` to fully set up codeNERD on ANY foreign repository by deriving, rather than being told, repository reality. It establishes the north-star vision, current shipped baseline, gap matrix, and capability specifications for foreign repository orientation. Start with [00-INDEX.md](00-INDEX.md) for the recommended read order and the grounded-versus-hypothesized map.

Package `codenerd/internal/orient`: a dedicated deductive Mangle engine and transducer pipeline designed to derive repository structure, development chronology, document lineage, agent ecosystem ingest, and project north star without hardcoded repository-specific rules or file-name heuristics.

1. **Timeline and Lineage Engine** — Evaluates streamed whole-history `git log` metrics into development eras, wave versus lull classifications, and document evolution chains (`repo_file_history`, `doc_generation`, `doc_evolved_into`, `doc_superseded`, `doc_live`). Replaces volatile filesystem modification timestamps with true committer times.
2. **Read-Candidate Attention** — Uses structural graph centrality, document generation, link density, and commit bursts to derive `orient_read_candidate` facts. This bounds model reading to high-signal documents, replacing arbitrary file-name priority tables and 10,000-character truncations with lossless document paging.
3. **Ecosystem Ingest and Derived Agents** — Parses multi-agent configuration and skill formats (`.claude/`, `.codex/`, `.agents/`, `.gemini/`, `.grok/`, `.jules/`, `.cursor/`, `.roomodes`), detects cross-tool duplicates, derives winning knowledge sources, and dynamically materializes shard agents and SQLite knowledge bases (`.nerd/shards/<name>_knowledge.db`). Replaces hardcoded imperative Go agent switches with declarative Mangle data.
4. **Non-Interactive North Star Derivation** — Classifies read-candidate documents into functional roles (`doc_role_claim`), weights vision sources, and synthesizes a complete `WizardDocument` (`Mission`, `Problem`, `VisionStmt`, `Personas`, `Capabilities`, `Risks`, `Requirements`, `Constraints`) directly into `internal/northstar`. Syncs authority at init time without requiring interactive chat turns, while emitting human-readable documentation (`.nerd/orientation/README.md`) and runtime EDB facts (`.nerd/orientation/orientation.mg`).
5. **Discernment and Clarification Overlay** — Analyzes tree structural statistics to classify tree roles (`/source`, `/tests`, `/fixtures`, `/seed_data`, `/generated`, `/vendored`, `/archive`) and treatments (`/index_and_parse`, `/index_names_only`, `/exclude`), identifies private systems and dependencies that must not undergo web research, discovers MCP integrations, and formulates first-boot questions asked via the TUI clarification loop, persisting answers to `.nerd/orientation/answers.json`.

What runs today across existing initialization and world scanner subsystems is detailed in [02-CURRENT-STATE.md](02-CURRENT-STATE.md) and [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md). The integration seams, uncalled routines, and assumed behaviors are documented in [WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md).
