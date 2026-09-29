---
doc-class: governance
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# TODO — Orientation Build Queue

This document specifies the leaf implementation tasks for `internal/orient` and its initialization hooks. Every item is traceable directly to an identifier in [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

---

## Phase 1: Foundations & Prerequisites

- [ ] `TODO-ORIENT-01` (`GAP-ORIENT-01`, Lane `L1`): Complete `internal/workspace` package providing single gitignore-aware membership authority.
- [ ] `TODO-ORIENT-02` (`GAP-ORIENT-01`, Lane `L1`): Repoint `world.Scanner.ScanDirectory` (`internal/world/fs.go:124`) to ask `Membership.Includes`, admitting git-tracked `.claude`, `.codex`, and `.agents` trees.
- [ ] `TODO-ORIENT-03` (`GAP-ORIENT-13`, Lane `P1`): Implement Python, TypeScript, TSX, and JS element parsing in `internal/world/codemodel/`.
- [ ] `TODO-ORIENT-04` (`GAP-ORIENT-13`, Lane `P2`): Link tree-sitter TSX grammar in `go.mod` and emit JSX component call edges in `internal/world/`.
- [ ] `TODO-ORIENT-05` (`GAP-ORIENT-13`, Lane `P3`): Implement multi-language holographic context blocks and typed test runners in `internal/tools/codedom/`.

---

## Phase 2: Sensors & Mangle Policy Substrate

- [ ] `TODO-ORIENT-06` (`GAP-ORIENT-02`, Lane `I2a`): Implement `internal/orient/history.go` whole-history streaming git log parser emitting `repo_file_history`, `repo_month`, and `repo_span`.
- [ ] `TODO-ORIENT-07` (`GAP-ORIENT-02`, Lane `I2a`): Implement `internal/orient/timeline.mg` deriving `repo_era` (`/wave` vs `/lull`).
- [ ] `TODO-ORIENT-08` (`GAP-ORIENT-02`, Lane `I2a`): Implement `internal/orient/docs.go` extracting headings, cross-links, and chunked embedding centroids.
- [ ] `TODO-ORIENT-09` (`GAP-ORIENT-03`, Lane `I2a`): Implement `internal/orient/lineage.mg` deriving `doc_generation`, `doc_burst`, `doc_evolved_into`, `doc_superseded`, and `orient_read_candidate`.
- [ ] `TODO-ORIENT-10` (`GAP-ORIENT-02`, Lane `I2a`): Implement `cmd/nerd/cmd_orient.go` exposing `nerd orient [--json]` for read-only inspection.
- [ ] `TODO-ORIENT-11` (`GAP-ORIENT-04`, Lane `I1`): Implement `internal/orient/ecosystem.go` parsing multi-agent configurations across `.claude`, `.codex`, `.agents`, `.gemini`, `.grok`, `.jules`, `.cursor`, `.roomodes`, and user memory.
- [ ] `TODO-ORIENT-12` (`GAP-ORIENT-04`, Lane `I1`): Implement `internal/orient/ecosystem_agents.mg` deriving `agent_source_duplicate` and evidence-based `agent_source_winner`.

---

## Phase 3: Derivation, Synthesis & Init Integration

- [ ] `TODO-ORIENT-13` (`GAP-ORIENT-05`, Lane `I1`): Complete `orient_agent` and `orient_agent_knowledge` rules in `ecosystem_agents.mg`.
- [ ] `TODO-ORIENT-14` (`GAP-ORIENT-05`, Lane `I1`): Delete imperative `determineRequiredAgents` switch in `internal/init/agents.go:373-626` and repoint caller to Mangle derivation.
- [ ] `TODO-ORIENT-15` (`GAP-ORIENT-05`, Lane `I1`): Implement `internal/init/phase_ecosystem.go` materializing shard databases in `.nerd/shards/` and writing `.nerd/orientation/agents.md`.
- [ ] `TODO-ORIENT-16` (`GAP-ORIENT-06`, Lane `I2b`): Implement `internal/northstar/derive.go` `ClassifyDocuments` with lossless multi-page reading, emitting `doc_role_claim` and `doc_theme`.
- [ ] `TODO-ORIENT-17` (`GAP-ORIENT-06`, Lane `I2b`): Author JIT prompt atoms `internal/prompt/atoms/northstar/derive_classify.yaml` and `derive_vision.yaml`.
- [ ] `TODO-ORIENT-18` (`GAP-ORIENT-07`, Lane `I2b`): Implement `internal/northstar/derive.go` `DraftVision` synthesizing complete `WizardDocument` and saving directly to `Store.SaveVision`.
- [ ] `TODO-ORIENT-19` (`GAP-ORIENT-07`, Lane `I2b`): Refactor TUI chat wizard (`cmd/nerd/chat/northstar_llm.go`) into thin wrapper around `derive.go`, deleting 10,000-char truncation.
- [ ] `TODO-ORIENT-20` (`GAP-ORIENT-08`, Lane `I2b`): Implement `internal/init/phase_orient.go` orchestrating classification, vision derivation, and writing `.nerd/orientation/README.md` and `orientation.mg`.
- [ ] `TODO-ORIENT-21` (`GAP-ORIENT-08`, Lane `I2b`): Remove `Use '/northstar' to define your project vision` message from `internal/init/initializer.go:1447`.

---

## Phase 4: Discernment, Privacy & Operator Dialogue

- [ ] `TODO-ORIENT-22` (`GAP-ORIENT-09`, Lane `O1`): Implement `internal/orient/trees.go` gathering structural directory metrics (`tree_stats`, `tree_ext`, `tree_name`).
- [ ] `TODO-ORIENT-23` (`GAP-ORIENT-09`, Lane `O1`): Implement `internal/orient/trees.mg` deriving `tree_role` and `tree_treatment` (`/index_and_parse`, `/index_names_only`, `/exclude`), generating `.nerd/orientation/membership.json`.
- [ ] `TODO-ORIENT-24` (`GAP-ORIENT-10`, Lane `O1`): Implement `internal/orient/deps.go` and `deps.mg` identifying private dependencies and suppressing public web search in `orient_research_topic`.
- [ ] `TODO-ORIENT-25` (`GAP-ORIENT-11`, Lane `O1`): Implement `internal/orient/mcp.go` and `mcp.mg` discovering MCP configurations and staging candidates for operator consent.
- [ ] `TODO-ORIENT-26` (`GAP-ORIENT-11`, Lane `O1`): Implement `internal/config/merge.go` executing surgical, byte-preserving JSON patches on `.nerd/config.json`.
- [ ] `TODO-ORIENT-27` (`GAP-ORIENT-12`, Lane `O1`): Implement `internal/orient/questions.mg` deriving `orient_question` and serializing `.nerd/orientation/questions.json`.
- [ ] `TODO-ORIENT-28` (`GAP-ORIENT-12`, Lane `O1`): Wire first-boot TUI clarification loop in `cmd/nerd/chat` to present orientation questions and persist answers to `.nerd/orientation/answers.json`.
