# internal/core/defaults/policy - Executive Policy Rules (IDB)

Mangle policy files implementing the Intensional Database (IDB) rules that derive `next_action`, `permitted`, and other executive decisions. These rules form the deterministic logic layer that orchestrates the OODA loop.

## File Index

| File | Description |
|------|-------------|
| `activation.mg` | Spreading activation rules for context selection (Section 1). Computes activation scores based on recency, dependency, and intent to determine which facts enter working memory via `context_atom/1`. |
| `autopoiesis.mg` | Self-learning and tool generation rules (Sections 12, 12B, 12C). Detects repeated rejection patterns, triggers Ouroboros tool generation, manages tool lifecycle states, and tracks quality for refinement. |
| `browser.mg` | Browser physics and spatial reasoning rules (Section 9). Derives element positions (`left_of`, `above`), detects honeypot elements via CSS properties, and identifies safe interactive elements for browser automation. |
| `campaign.mg` | Campaign orchestration state machine (Section 19). Manages multi-phase campaign execution including phase eligibility, task selection, context paging, checkpoint verification, replanning triggers, and phase-aware tool permissions. |
| `capabilities.mg` | Future capability mappings for VirtualStore actions (Section 51). Placeholder rules ensuring all registered actions (Code DOM, Python, SWE-bench) are reachable via policy to satisfy the action linter. |
| `clarification.mg` | Focus resolution and clarification rules (Sections 4, 11, 16). Triggers `interrogative_mode` when confidence is low, detects ambiguity, implements abductive reasoning for missing hypotheses, and resumes from clarification. |
| `code_dom.mg` | Code DOM and semantic editing rules (Section 22). Manages file scope, element accessibility, transitive containment, edit safety/risk assessment, breaking change detection, mock generation hints, and continuation protocol. |
| `commit_gate.mg` | Commit barrier and diff approval rules (Sections 6, 15). Blocks commits on build errors, failing tests, or Nemesis gauntlet failures. Requires approval for dangerous mutations via Chesterton's Fence warnings. |
| `constitution.mg` | Constitutional safety logic (Sections 7, 7B, 7C). Implements default-deny permission system, defines safe vs dangerous actions, domain allowlists, stratified trust for autopoiesis, and appeal mechanism for blocked actions. |
| `data_flow.mg` | Data flow safety and taint analysis rules (Section 47). Derives guard patterns (nil checks, ok checks), detects unsafe nil dereferences, and identifies unchecked error variables using Go idiom patterns. |
| `delegation.mg` | Shard delegation and tool mapping rules (Sections 8, 10). Routes intents to appropriate shards (researcher, coder, tester, reviewer), maps tool capabilities, and derives `next_action` from intent verb mappings. |
| `dreamer.mg` | Speculative Dreamer precognition rules (Section 26). Enumerates critical files, derives `panic_state` for actions that would remove critical files or tested symbols, and blocks via `dream_block` for safety. |
| `git_safety.mg` | Git-aware safety and Chesterton's Fence rules (Section 13). Detects recent changes by other authors, warns before deleting recently-modified code, flags high-churn files, and triggers clarification for risky refactors. |
| `impact.mg` | Impact analysis and refactoring guard rules (Section 5, 48). Computes transitive impact closure from modified files, blocks refactoring of uncovered dependencies, and derives bounded impact graphs for reviewer context. |
| `jit_config.mg` | JIT prompt compiler configuration. Defines static category ordering (identity → safety → methodology → context) and budget allocation percentages for prompt assembly token distribution. |
| `jit_logic.mg` | JIT prompt compiler context matching rules. Implements selector matching for shard type, mode, phase, language, framework, and world state. Computes `atom_matches_context` scores and dependency resolution. |
| `jit_selection.mg` | JIT prompt atom selection algorithm. Implements three-phase selection: candidate filtering, conflict/exclusion detection, and final ordering. Validates compilation for skeleton categories and learns from effective atoms. |
| `knowledge.mg` | Knowledge atom integration and retrieval rules (Sections 17, 17B, 25). Activates domain expert strategy on high-confidence knowledge, applies learned preferences/constraints, implements holographic retrieval via code graph. |
| `prioritization.mg` | Hypothesis prioritization rules (Section 50). Assigns base priorities by issue type (SQL injection > unsafe deref > resource leak), applies file-based boosts for uncovered or buggy files, computes final priority scores. |
| `prompt_composition.mg` | Dynamic prompt composition rules (Sections 41, 42). Derives shard-specific context relevance, manages context budgets, detects staleness, integrates with spreading activation, and implements Northstar vision reasoning. |
| `shards.mg` | Shard type classification and trace analysis rules (Sections 18, 24). Defines shard types (system/ephemeral/persistent/user), tracks reasoning trace quality, detects struggling/performing shards, enables cross-shard learning. |
| `shadow_mode.mg` | Shadow mode counterfactual reasoning rules (Section 14). Derives implications from hypothetical changes, validates safe projections, detects projection violations (test failures, security issues), blocks unsafe mutations. |
| `strategy.mg` | Strategy selection rules (Section 2, 17, 20). Activates appropriate strategies based on intent: TDD repair loop for fixes, breadth-first survey for exploration, refactor guard for modifications, campaign planning for large tasks. |
| `system.mg` | System shard coordination rules (Section 21). Implements OODA loop phases (observe → orient → decide → act), manages permission/routing flow, monitors shard health, handles safety violations, and coordinates on-demand activation. |
| `tdd_logic.mg` | TDD loop commit barrier rule. Simple rule blocking commits when error-severity diagnostics exist, implementing Cortex 1.5.0 §2.2 "The Barrier". |
| `tdd_loop.mg` | TDD repair loop state machine rules (Section 3). Implements state transitions: failing → read_error_log → analyze_root_cause → generate_patch → run_tests → passing. Escalates after 3 retries. |
| `tool_routing.mg` | Intelligent tool routing rules (Section 40). Defines shard-capability affinities (coder↔generation, tester↔validation), intent-capability mappings, and derives tool relevance scores for context-aware tool selection. |
| `trace_logic.mg` | Tracing metadata for debugging and visualization. Maps IDB rule predicates to human-readable names (e.g., `next_action` → "strategy_selector") and identifies EDB predicates for trace analysis. |
| `verification.mg` | Verification loop and quality gate rules (Section 23). Manages verification attempts, triggers corrective actions for mock code/placeholders/hallucinated APIs, blocks commits on quality violations, learns from corrections. |

## Architecture

The policy files implement the **Intensional Database (IDB)** - derived rules that produce actions from facts:

```text
EDB Facts (World Model) → IDB Rules (Policy) → next_action/1 → VirtualStore
```

Key derived predicates:
- `next_action/1` - The single action to execute next
- `permitted/3` - Constitutional safety gate
- `activation/2` - Spreading activation scores
- `delegate_task/3` - Shard spawning signals
- `block_commit/1` - Commit barrier reasons

## Section Mapping

| Sections | Focus Area |
|----------|------------|
| 1-3 | Core loop: Activation, Strategy, TDD |
| 4-6 | Safety: Clarification, Impact, Commit |
| 7-9 | Constitution, Delegation, Browser |
| 10-14 | Tools, Abduction, Learning, Git, Shadow |
| 15-21 | Approval, State, Knowledge, Shards, Campaign, System |
| 22-26 | Code DOM, Verification, Traces, Holographic, Dreamer |
| 40-51 | Advanced: Tool Routing, Prompts, Data Flow, Priorities |

## Loading Order

All `.mg` files should be loaded together. The Mangle engine handles stratification automatically, but conceptually:
1. Constitution rules (permission foundations)
2. Strategy/activation rules (context selection)
3. Action derivation rules (next_action)
4. Specialized domain rules (campaign, verification, etc.)

---

**Remember: Push to GitHub regularly!**


> *[Archived & Reviewed by The Librarian on 2026-01-25]*