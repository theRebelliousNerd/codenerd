---
doc-class: governance
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# TODO — Orientation & Spec Alignment Build Queue

This document specifies the leaf implementation tasks for `internal/orient`, spec alignment, what-next planning, and refactoring transactions. Every item is traceable directly to an identifier in [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

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

---

## Phase 5: Spec Alignment, Native Templates & Technical Debt Sensors

- [ ] `TODO-ORIENT-29` (`GAP-ORIENT-14`, Lane TBD): Implement `internal/orient/spec_units.go` extracting code units, spec units, and dependencies into bipartite unit graph facts.
- [ ] `TODO-ORIENT-30` (`GAP-ORIENT-14`, Lane TBD): Implement link transduction client in `internal/orient/spec_links.go` proposing candidate pairs and recording model confidence claims.
- [ ] `TODO-ORIENT-31` (`GAP-ORIENT-15`, Lane TBD): Implement `internal/orient/spec_status.mg` deriving status atoms (`/aligned`, `/behind`, `/ahead`, `/missing`, `/stale`, `/not_trending`) using git committer timestamps (`%ct`).
- [ ] `TODO-ORIENT-32` (`GAP-ORIENT-17`, Lane TBD): Implement `internal/orient/spec_templates.go` discovering foreign repository spec schemas into `repo_spec_template` facts.
- [ ] `TODO-ORIENT-33` (`GAP-ORIENT-17`, Lane TBD): Generalize `internal/docscheck/docscheck.go` into runtime sensor asserting `doc_problem` facts into the live kernel.
- [ ] `TODO-ORIENT-34` (`GAP-ORIENT-20`, Lane TBD): Implement `internal/orient/debt_sensors.go` extracting unreferenced symbols, forwarding shims, deprecated tags, and dark fields into Mangle debt facts.

---

## Phase 6: Foundations-Up Planning, Spec-First Gate & Orphan Dialogue

- [ ] `TODO-ORIENT-35` (`GAP-ORIENT-18`, Lane TBD): Implement `internal/orient/what_next.mg` extending Kahn topological sort over the unit graph to derive `unit_ready(Unit)` and `what_next(Unit, Rank, Why)`.
- [ ] `TODO-ORIENT-36` (`GAP-ORIENT-19`, Lane TBD): Implement `start_from(Kind)` seed direction filtering and parametric weight loading from `config_param`.
- [ ] `TODO-ORIENT-37` (`GAP-ORIENT-18`, Lane TBD): Register `/next` chat command in `cmd/nerd/chat/commands.go` and add CLI entry point `nerd next [--from <kind>] [--json]` in `cmd/nerd/cmd_next.go`.
- [ ] `TODO-ORIENT-38` (`GAP-ORIENT-18`, Lane TBD): Wire unit status and `what_next` findings into `internal/campaign/recurse_policy.go` for campaign recursion sweeps.
- [ ] `TODO-ORIENT-39` (`GAP-ORIENT-16`, Lane TBD): Conjoin `!turn_has_spec_misalignment(Turn)` into `coder_safety.mg:675` and wire `turn_missing_evidence(Turn, /spec_misaligned)` into `internal/session/executor.go`.
- [ ] `TODO-ORIENT-40` (`GAP-ORIENT-21`, Lane TBD): Implement orphan code identification in `internal/orient/orphans.mg`, excluding entry points, tests, and excluded tree treatments.
- [ ] `TODO-ORIENT-41` (`GAP-ORIENT-21`, Lane TBD): Wire orphan questions through `cmd/nerd/chat/process_dream_delegation.go` (`kernelClarification`) with package grouping and session capping.
- [ ] `TODO-ORIENT-42` (`GAP-ORIENT-21`, Lane TBD): Implement `.nerd/orientation/answers.json` persistence and boot re-assertion in `internal/orient/answers.go`.

---

## Phase 7: Atomic Transactional Refactoring

- [ ] `TODO-ORIENT-43` (`GAP-ORIENT-22`, Lane TBD): Generalize `repointAndDelete` (`internal/tools/codedom/repoint.go:310`) to support methods, cross-file moves, and polyglot files in `internal/tools/codedom/cleanup_transaction.go`.
- [ ] `TODO-ORIENT-44` (`GAP-ORIENT-22`, Lane TBD): Implement pre-commit in-memory AST syntax validation across all staged files in `cleanup_transaction.go`.
- [ ] `TODO-ORIENT-45` (`GAP-ORIENT-22`, Lane TBD): Wire post-commit compiler (`verifyBuild`) and impacted test execution (`run_impacted_tests.go`) with automated `git checkout` rollback on failure.
