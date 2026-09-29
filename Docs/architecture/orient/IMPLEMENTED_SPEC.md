---
doc-class: shipped
subsystem: orient
implementation-status: shipped
last-verified: 2026-09-29
verified-against: 6597099c
supersedes: []
---

# IMPLEMENTED_SPEC — Shipped Orientation Truth

> **Authoritative Baseline Record**. Verified 2026-09-29 against commit `e056692c`. If any other document in this directory makes claims about current shipped functionality that contradict this file, **this file wins**.

---

## 1. What Has Shipped

As of commit `6597099c` plus the orientation-engine commit that carries this revision:
- **The orientation engine** (`internal/orient`): its own Mangle engine (`NewEngine`, `Assert`, `Evaluate`, `Query` in `internal/orient/engine.go`), one streaming `git log --name-status -M` pass with rename stitching and shallow-clone detection (`internal/orient/history.go`), document facts, resolved links and embedding similarity with a content-addressed vector cache (`internal/orient/docs.go`), and the policy (`internal/orient/schema.mg`, `timeline.mg`, `lineage.mg`): eras, generation, bursts, cohorts, evolution, supersession, liveness, origin sources, vision weights, and the bounded read-candidate set with every omitted document listed. `Inspect` (`internal/orient/report.go`) runs it on a workspace.
- **Config**: the `orient` block (`internal/config/orient.go`, `GetOrientConfig`), checked at load; every threshold reaches the policy as `config_param(/orient_<key>, N)`.
- **Evidence**: `go test ./internal/orient/` (policy fact sets; engine API; a temp git repository with a rename, a lull, a one-day burst and a shallow clone; embedding cache hit/miss; the report).
- **Not shipped**: init does not yet run orientation (GAP-ORIENT-23); the ecosystem ingest, role transduction, north-star derivation, discernment/questions and spec alignment are in flight (section 2).
- **By design, no `nerd orient` command** (Steve, 2026-09-29): orientation is the Orient step of the OODA loop, run by `nerd init` first and refreshed automatically.
- `internal/workspace` and non-interactive north-star derivation have not shipped.

## 2. Active Implementation Lanes In Flight

The target capabilities specified in this corpus are actively being implemented across ten coordinated development lanes:

| Lane ID | Subsystem Scope | Target Implementation Artifacts |
|---|---|---|
| **`I2a`** | Orientation Engine & History | `internal/orient/engine.go`, `history.go`, `docs.go`, `timeline.mg`, `lineage.mg`, `internal/config/orient.go`, `cmd/nerd/cmd_orient.go`. |
| **`I1`** | Ecosystem Ingest & Agents | `internal/orient/ecosystem.go`, `ecosystem_agents.mg`, `internal/init/phase_ecosystem.go`, deletion of `determineRequiredAgents` switch in `internal/init/agents.go`. |
| **`I2b`** | North Star Synthesis & Transduction | `internal/northstar/derive.go`, `internal/init/phase_orient.go`, JIT prompt atoms under `internal/prompt/atoms/northstar/`, `.nerd/orientation/README.md` and `orientation.mg` emission. |
| **`O1`** | Discernment, Privacy & Questions | `internal/orient/trees.go`, `trees.mg`, `deps.go`, `deps.mg`, `mcp.go`, `mcp.mg`, `questions.mg`, `internal/config/merge.go`, TUI first-boot clarification hook. |
| **`L1`** | Workspace Membership Authority | `internal/workspace/membership.go`, repointing of ~35 production walkers to ask `Membership.Includes`. |
| **`P1`** | Polyglot CodeDOM | `internal/world/codemodel/` element models and surgical edit tools for Python, TypeScript, TSX, and JS. |
| **`P2`** | Polyglot World Model | Tree-sitter AST, import resolution, JSX call edges, and test linking for Python and TS/JS in `internal/world/`. |
| **`P3`** | Polyglot Context & Test Runners | Holographic impact blocks and typed test runners (`pytest`, `vitest`) in `internal/world/holographic.go` and `internal/tools/codedom/`. |
| **`R1`** | Meta Search Grounding | Provider config and `/responses` search grounding integration in `internal/perception/client_openai_compat.go`. |
| **`R2`** | Native Browser Research | Rerouting all research web fetches through Rod/CDP native browser in `internal/tools/research/`. |

---

## 3. Shipped Baseline in Preexisting Subsystems

The codeNERD repository contains robust production foundations that the incoming orientation lanes build upon and integrate with:

### A. Initialization Pipeline (`internal/init`)
- `internal/init/initializer.go:460-621` executes 22 sequential phases with `ETATracker` progress reporting (`internal/init/eta_tracker.go:21-46`).
- Phase 1 opens `northstar.NewStore` for SQLite table migrations (`internal/init/initializer.go:796-804`).
- Phase 12 gathers documentation via `GatherProjectDocumentation` (`internal/init/strategic_knowledge.go:241-410`) using fixed filename scores (lines 246-260).
- Phase 9 executes `determineRequiredAgents` (`internal/init/agents.go:373-626`), evaluating language and dependency strings in an imperative Go switch.
- Specialist agents are provisioned via `createAgentKnowledgeBase` (`internal/init/agents_knowledge.go:33-164`) and registered in `.nerd/agents.json` (`internal/init/agents_registration.go:73`).

### B. North Star Domain Model (`internal/northstar`)
- `internal/northstar/types.go:27-38` defines the canonical relational model (`Vision`, `Mission`, `Problem`, `Personas`, `Capabilities`, `Risks`, `Requirements`, `Constraints`).
- `internal/northstar/bridge.go:88-100` defines `WizardDocument`; `ToVision` (`internal/northstar/bridge.go:148`) and `WizardDocumentFromVision` (`internal/northstar/bridge.go:221`) translate between it and `Vision`.
- `internal/northstar/store.go:96-108` provides durable SQLite persistence in `.nerd/northstar_knowledge.db`.
- `internal/northstar/bridge.go:436-534` (`SyncVisionAuthority`) maintains timestamp-based reconciliation between SQLite, `northstar.json`, and `northstar.mg`.

### C. Chat Clarification Seam (`cmd/nerd/chat`)
- `cmd/nerd/chat/process_dream_delegation.go:26` (`kernelClarification`) queries the kernel for pending clarification questions.
- `cmd/nerd/chat/model_handlers.go:417, 539` handles interactive option selection, text input, and formatted question rendering (`ClarificationState`).

### D. MCP Configuration Infrastructure (`internal/config`)
- `internal/config/integrations.go:12-45` defines `IntegrationsConfig` supporting arbitrary HTTP, stdio, and SSE servers with headers and endpoints.
- `internal/config/user_config.go:1430-1448` provides user configuration accessors and validation routines.

### E. Native Browser Subsystem (`internal/browser`)
- `internal/browser/session_manager.go:45-120` provides production Rod/CDP browser management, tab isolation, and DOM interaction primitives.
