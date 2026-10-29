---
doc-class: north-star
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# 11 — What Next Foundations-Up — Unit Graph Traversal, Pluggable Directions, and Explanations

This capability specification details the design of codeNERD's foundations-up work recommendation engine. It realizes Steve's 2026-09-29 mandate: when an operator asks what to work on next, the system does not ask an ungrounded model to invent suggestions; the Mangle kernel derives a mathematically ordered, explained, and ranked queue from the difference between the code graph and the spec graph, progressing from foundations upward along any chosen architectural direction.

---

## 1. The Core Invariant: Derived, Not Suggested (Decision D4, ADR-009)

In conventional coding assistants, asking "what should I work on next" prompts the LLM to inspect recent chat messages or grep for "TODO" comments, hallucinating tasks disconnected from architectural prerequisites. 

codeNERD enforces an absolute inversion of control:
- The Mangle deductive kernel is the executive that derives what is ready, what blocks what, and what ranks highest.
- The unit graph (connecting code units and specification units, established in [10-SPEC-ALIGNMENT.md](10-SPEC-ALIGNMENT.md)) is the single source of truth.
- Every recommendation is a derived fact: `what_next(Unit, Rank, Why)`, where `Why` is a deterministic, human-readable derivation chain explaining the exact logical justification.

---

## 2. Foundations-Up Kahn Ordering on the Unified Unit Graph

Today, bottom-up topological sorting is implemented in Mangle for package-level compiler and linter sweep passes (`internal/core/defaults/policy/recurse.mg:377-386` and `internal/campaign/recurse_plan.go:108-133`). In that engine, `recurse_node_ready(ID, Ord)` derives when all dependencies of a subsystem node have already been visited.

The "What Next" engine extends this Kahn topological ordering across the entire unified unit graph:

### Readiness Derivation Rules

1. **Target Population**: Only units whose derived status is not `/aligned` are candidates for work. Units already marked `/aligned` serve as satisfied foundations.
2. **Dependency Satisfaction**: A unit is blocked if any unit it depends upon (via `code_depends` or `spec_depends`) is unaligned and has not been classified as an intentional deletion or leave-alone target.
3. **Foundations-Ready Derivation**: A unit is derived as `unit_ready(Unit)` when:
   - It is not `/aligned`.
   - Every prerequisite unit it depends upon is either `/aligned` or explicitly marked satisfied by operator override.
4. **Bottom-Up Progression**: Leaves (units with zero dependencies, such as base domain types or primitives) become ready first. As they are implemented, verified, and aligned, their immediate consumers become ready in subsequent cycles. Entry points and public facades become ready only after their supporting substrates are fully aligned.

---

## 3. Pluggable Starting Directions (`start_from(Kind)`)

Architectural work rarely begins from an arbitrary leaf; developers often choose to stabilize a specific architectural layer before proceeding to higher levels. The engine provides pluggable starting directions via `start_from(Kind)`.

### Selecting the Seed Set

- When `start_from(Kind)` is asserted, candidate selection is filtered to units matching the specified kind fact:
  - `start_from(/types)`: Seeds from fundamental domain types, data models, and schemas.
  - `start_from(/ingress)`: Seeds from input parsers, decoders, event consumers, and request validators.
  - `start_from(/persistence)`: Seeds from database repositories, storage engines, and WAL implementations.
  - `start_from(/api)`: Seeds from HTTP/gRPC handlers, RPC dispatchers, and public gateways.
  - `start_from(/tooling)`: Seeds from CLI commands, developer scripts, and operational utilities.
- When `start_from` is omitted, the default candidate set encompasses all non-aligned units across the entire repository.
- Unit kinds are never looked up in brittle filename tables; they are asserted as ground facts from CodeDOM element structural mixes and orientation tree-role classifications.

---

## 4. Parametric Ranking and Verbal Explanations

Units that are foundations-ready are ranked by a deterministic scoring model. Crucially, **no constants or magic numbers exist in `.mg` policy files**. All ranking coefficients are loaded from configuration parameters via `config_param(Key, Value)`.

### Ranking Factor Hierarchy

The total score for a ready unit is computed by accumulating weighted factors:

| Ranking Dimension | Configuration Parameter Key | Default Weight | Architectural Rationale |
|---|---|---|---|
| **Critical North Star Capability** | `/orient_weight_northstar_critical` | 100 | Units realizing must-have North Star requirements (`critical_capability` in `prompt_northstar.mg:10`) outrank secondary tasks. |
| **Failing Behavioral Witness** | `/orient_weight_failing_witness` | 80 | A unit specified with failing or regressing tests (`/behind` with test failure) must be repaired before new features are authored. |
| **Blocking Shims & Debt** | `/orient_weight_blocking_debt` | 60 | Unused forwarding shims, deprecated adapters, or dark fields that block downstream units are elevated for removal. |
| **Chronological Spec Staleness** | `/orient_weight_stale_spec` | 40 | Units where code has drifted ahead of older specifications are scheduled for spec-first synchronization. |
| **High Fan-In Blocker** | `/orient_weight_fan_in_multiplier` | 10 (per dependent) | Units that unblock multiple downstream consumers receive priority to widen parallelism. |

### Verbal Explanation Derivation

The third argument of `what_next(Unit, Rank, Why)` produces an audit-grade explanation string by conjoining human-readable clause templates:
- For North Star items: "Foundations ready. Realizes critical capability: [CapabilityTitle]. Blocks [N] downstream units."
- For failing items: "Foundations ready. Implementation is behind specification with failing witness tests in [TestPath]."
- For technical debt: "Foundations ready. Forwarding shim or dark field blocking alignment of [ConsumerUnit]."

---

## 5. The Three Consumption Surfaces

The derived `what_next` recommendations are exposed through three native interaction surfaces:

### 1. The Interactive Chat Command (`/next`)

In `cmd/nerd/chat/commands.go:55-265`, a new slash command `/next [direction]` is registered. It queries `what_next(Unit, Rank, Why)` from the kernel, rendering a formatted Charm TUI table displaying the top 5 ready units, their priority ranks, and their derivation justifications.

### 2. Natural Language Intent Transduction

When the operator types natural language queries such as "what should I work on next?", "where should we start?", or "what is unblocked?", the perception transducers map the request to intent `/what_next` with optional direction parameters (e.g. "what types should I build next?" $\rightarrow$ `/what_next` with `direction: /types`). The virtual store executes the kernel query and formats the recommendations directly into the articulation stream.

### 3. CLI Command (`nerd next`)

A dedicated CLI command is added under `cmd/nerd/cmd_next.go`:
- Plain execution: `nerd next` displays the highest-priority ready tasks with full verbal derivations.
- Direction flag: `nerd next --from <kind>` filters the sweep to a specific architectural layer.
- JSON output: `nerd next --json` outputs structured JSON containing unit identifiers, ranks, status atoms, dependencies, and reasons, designed for IDE integrations and external automation.

---

## 6. Integration with Campaign Recursion (`nerd campaign recurse`)

Today, `nerd campaign recurse` (`cmd/nerd/cmd_campaign_recurse.go:42-80` and `internal/campaign/recurse_policy.go:66-106`) operates in a vacuum, executing an import sweep that derives `recurse_next` solely from compiler and linter gate findings (`gates.Finding`). It is completely blind to missing specifications, unbuilt capabilities, and architectural gaps.

Under this specification, campaign recursion is unified with the unit graph:
1. `recurse_policy.go` queries both gate findings and spec alignment findings (`unit_status` and `what_next`).
2. When a package is visited during a recursive sweep, unbuilt spec capabilities (`/behind`) and un-specced public surfaces (`/ahead`) are treated as first-class findings alongside compiler errors.
3. The campaign cannot declare a subsystem completed until its unit status evaluates to `/aligned`.

---

## 7. Verification Tests That Prove This Capability

1. **`TestWhatNext_KahnFoundationsUpOrdering`**: Constructs a 5-node dependency graph where leaf node A is unaligned, intermediate node B depends on A, and root node C depends on B. Verifies that `what_next` derives A as ready, while B and C are withheld. Once A becomes `/aligned`, verifies that B immediately becomes ready.
2. **`TestWhatNext_PluggableDirectionSeeding`**: Evaluates a mixed repository with unaligned `/types` and unaligned `/api` units. Verifies that asserting `start_from(/types)` yields only type units in the candidate list.
3. **`TestWhatNext_ConfigurableParametricRanking`**: Injects custom `config_param` weights into the kernel. Verifies that changing the weight for `/orient_weight_blocking_debt` causes technical debt removal to leapfrog unbuilt capabilities in the ranked output.
4. **`TestWhatNext_ExplanationDerivation`**: Asserts a unit realizing a critical North Star capability. Verifies that the derived `Why` string contains the exact title of the capability and the count of blocked consumers.
5. **`TestWhatNext_CLIJsonContract`**: Executes `nerd next --json` against a test fixture, asserting that stdout unmarshals into the valid `WhatNextResponse` JSON schema with non-empty units and explanation strings.

---

## 8. Failure Modes and Guardrails

- **Dependency Cycles**: If circular dependencies exist in the code graph, naive topological sort deadlocks. Mitigation: `DeriveWorkspaceDAG` collapses strongly connected components using Tarjan's SCC algorithm (`internal/campaign/recurse_workspace.go:139-226`) into composite nodes, preventing cycles from halting readiness derivation.
- **Starved Recommendations**: If every leaf unit is blocked by an unaligned external dependency that the repository cannot edit, the queue could empty. Mitigation: dependencies outside the workspace boundary are classified as external providers and marked implicitly satisfied unless explicitly flagged as missing.
- **Overwhelming Output on Large Codebases**: In a monorepo with 500 unaligned units, presenting an unranked or un-capped list causes operator paralysis. Mitigation: the engine derives the full order but caps presentation surfaces to the top $N$ items (governed by `config_param(/orient_what_next_display_limit, 5)`).
