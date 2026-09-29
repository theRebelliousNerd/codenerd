---
doc-class: governance
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 04 — Principles and Constraints — The Laws of Orientation

Every modification, extension, or implementation of `internal/orient` and its associated initialization pipelines must adhere strictly to these nine architectural principles. Each principle is tied to a specific architectural ruling and operational invariant.

---

### Principle 1: Recency Is Not Quality
*Tied to: Orientation Contract (`_orient_contract.txt:8-9`), Ruling on Chronological Truth*

A document committed yesterday to update an operational dependency is not inherently more important than a foundational architectural specification committed eighteen months ago. A large, cohesive document set committed in a single burst on one day is often the definitive design of the entire project.
- **Invariant**: Filesystem modification times (`mtime`) are volatile and reset across clones, checkouts, and archive extractions (`internal/world/content_stamp.go:18-207`). All chronological calculations must be grounded in immutable git committer timestamps (`%ct`).
- **Invariant**: Age is relative to the repository's span and development eras (`repo_era`), not absolute calendar time. A document untouched for two years in an active repository may be an active foundational origin if no successor has superseded it.

---

### Principle 2: No Name-Keyed Decisions
*Tied to: Orientation Contract (`_orient_contract.txt:8-10`), Ruling on General-Purpose Invariance*

Orientation logic must never contain hardcoded repository names, project-specific folder structures, or arbitrary filename priority tables.
- **Invariant**: Hardcoded priority tables such as `priorityFiles["CLAUDE.md"] = 0` (`internal/init/strategic_knowledge.go:246-260`) are strictly prohibited.
- **Invariant**: Attention selection (`orient_read_candidate`) must derive candidates purely from structural properties: origin generation, graph centrality in the link network, embedding cluster centroids, commit bursts, and evolution tips.

---

### Principle 3: Decisions Derived in Mangle; Go Is the Sensor
*Tied to: The Vision (`CLAUDE.md:49-54`), Ruling on Clean Fixpoint Architecture*

The executive decisions of orientation — what an era is, which document evolved into which, which agent to spawn, what tree role applies, and what question to ask — must be computed as the fixpoint of Mangle Datalog rules over asserted facts.
- **Invariant**: Go code performs sensing and mechanical duties only: executing streamed git logs, parsing ASTs, calculating content hashes, and executing token math.
- **Invariant**: Go code must never implement decision switches over business domain concepts (such as the language-to-agent switch in `internal/init/agents.go:373-626`). Such tables must be represented as declarative Datalog facts in `.mg` policy files.

---

### Principle 4: Never Truncate — Page Instead
*Tied to: North Star Brief (`I2b_northstar_derive.txt:14-15`), Ruling on Zero Cognitive Clipping*

Truncating documents mid-file silently destroys context, amputates requirements, and induces hallucinations in downstream reasoning.
- **Invariant**: The hardcoded 10,000-character truncation in `cmd/nerd/chat/northstar_llm.go:201-203` is forbidden.
- **Invariant**: Any document exceeding the model request budget must be paged sequentially in its entirety, carrying a structured running summary across chunk boundaries. Small documents must be batched together without clipping.

---

### Principle 5: Nothing Repository-Specific in Core
*Tied to: Ecosystem Brief (`I1_ecosystem.txt:28-32`), Ruling on Format Generalization*

All parsers and transducers in `internal/orient` must operate against documented tool specifications and standard serialization formats, never ad-hoc project artifacts.
- **Invariant**: Ecosystem discovery walks only registered, well-known coding-agent directory roots (`.claude/`, `.codex/`, `.agents/`, `.gemini/`, `.grok/`, `.jules/`, `.cursor/`, `.roomodes`). It does not search for project-specific custom folders.
- **Invariant**: Manifest parsing supports canonical package definitions across languages (`go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`).

---

### Principle 6: Credentials Only After Explicit Consent
*Tied to: Discernment Brief (`O1_discern_and_ask.txt:21-27`), Ruling on Credential Isolation*

Discovered external tools, integrations, and MCP servers often contain sensitive access tokens, headers, and environment variables.
- **Invariant**: The orientation engine asserts only credential keys and header names (`mcp_server_credential(Name, /header, "X-API-Key")`), never secret values.
- **Invariant**: Secret values are read only at the exact moment of explicit operator approval and injected directly into target configurations without entering the fact graph or logs.

---

### Principle 7: User-Owned Config Is Merged, Never Rewritten
*Tied to: Discernment Brief (`O1_discern_and_ask.txt:23-27`), Ruling on Configuration Preservation*

`.nerd/config.json` belongs to the operator. Automated tools must never rewrite or reformat the entire configuration file using generic JSON unmarshal/marshal cycles, which discard whitespace, reorder keys, and strip comments.
- **Invariant**: Configuration modifications must use targeted, surgical JSON patches that modify only the intended object path (e.g. `integrations.servers.<name>`).
- **Invariant**: Every proposed patch must be validated via `config.LoadUserConfig` and `Check` before writing to disk, failing closed if validation fails.

---

### Principle 8: Docs Are the Spec
*Tied to: Architecture Standard (`Docs/journeys/09-architecture-doc-standard.md:1-24`), Ruling on Spec Primacy*

`Docs/architecture/` is the single source of truth for codeNERD's design and intent.
- **Invariant**: Implementation must conform to the architecture corpus. Discrepancies between code and spec are defects in the code unless an approved ADR updates the spec.
- **Invariant**: Shipped documentation must cite verified repo-relative source lines (`path:line`), distinguishing verified reality from planned aspiration.

---

### Principle 9: The Blind-Orientation Acceptance Principle
*Tied to: Acceptance Ruling (`ADR-007`), Ruling on True Generalization*

The orientation subsystem must be validated on genuine, complex foreign repositories whose directory structures, document names, and project semantics have never been exposed to the model, briefs, prompts, or test fixtures.
- **Invariant**: If orientation requires hardcoding an exception or keyword to pass a specific repository test, the design has failed.
- **Invariant**: Acceptance consists of running `nerd init` on an unprimed foreign repository and verifying that the derived North Star, timeline eras, tree roles, and agent rosters accurately reflect reality without manual correction.
