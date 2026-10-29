---
doc-class: shipped-with-future
subsystem: orient
implementation-status: partial
last-verified: 2026-09-29
verified-against: working-tree-C2a
supersedes: []
---

# IMPLEMENTED_SPEC — Shipped Orientation Truth

> **Authoritative Baseline Record**. Verified 2026-09-29 against commit `e056692c`. If any other document in this directory makes claims about current shipped functionality that contradict this file, **this file wins**.

---

## 1. Source state and verification boundary

The committed engine remains the baseline. C2a has authored the following
wiring; compilation and runtime gates were deliberately not run by this lane.
This record describes inspected source, not passing acceptance evidence.

| Capability | Source witness | Gate status |
|---|---|---|
| One init orientation before profile, prompts, agents and KB generation | `internal/init/initializer.go:469` (`Initialize`); `internal/init/initializer.go:712` (`newPhaseRunner`) | `TestOrientationPhaseOrder`, authored; not run |
| Shared history, document and ecosystem census | `internal/orient/snapshot.go:77` (`Measure`); `internal/init/phase_orient.go:38` (`runOrientation`) | `TestOrientationInitArtifacts`, authored; not run |
| Role transduction and north-star API calls | `internal/init/phase_orient.go:38` (`runOrientation`) calls the existing northstar APIs | C2b owns the derivation implementation; integration unverified |
| Ecosystem policy loaded with every embedded policy file | `internal/orient/engine.go:111` (`policySource`) | Existing engine gate updated; not run |
| Typed topic overlap and skill cluster thresholds | `internal/config/orient.go:182` (`WithDefaults`), `internal/config/orient.go:400` (`Params`) | Config test authored; `UserConfig` registration remains outside this lane |
| Agent roster, KB seeds and imported prompts consume the same fixpoint | `internal/init/phase_ecosystem.go:115` (`integrateEcosystem`); `internal/orient/ecosystem_agents.mg:109` (`orient_agent_prompt`) | Existing materialization test adapted; not run |
| Strategic consumers keep their existing categories | `internal/init/phase_orient.go:258` (`persistOrientationKnowledge`) | End-to-end artifact test checks `strategic/vision`; not run |
| Init timeout bounds one model request | `internal/init/jit_integration.go:399` (`withJITPrompt`) | `TestInitTimeoutBoundsOneModelRequest`, authored; not run |
| Durable snapshot and main-kernel projection | `internal/orient/snapshot.go:181` (`Projection`), `internal/orient/snapshot.go:228` (`Save`) | Author-only; no compiled evidence |
| Boot derives staleness and refreshes measurements | `internal/orient/freshness.mg:8` (`orient_stale`); `internal/orient/snapshot.go:281` (`Refresh`); `internal/core/kernel_init.go:309` (`loadMangleFiles`) | `TestRefreshNewCommitAnswersSurvive`, authored; not run |

There is no orientation command. Changed documents lose their old role/theme
claims and become `orient_role_pending`; boot cannot call an LLM. Unchanged
classifications survive. Fast-forward history uses a delta; rewritten history,
renames, and snapshots without day witnesses require a full history pass.
Operator answers are outside the snapshot writer and remain untouched.

GAP-ORIENT-23 remains **partial**: the incremental scan must invoke `Refresh`,
and all acceptance commands remain pending. See [13-INIT-AND-REORIENTATION.md](13-INIT-AND-REORIENTATION.md).

## 2. Active Implementation Lanes In Flight

The target capabilities specified in this corpus are actively being implemented across ten coordinated development lanes:

| Lane ID | Subsystem Scope | Target Implementation Artifacts |
|---|---|---|
| **`I2a`** | Orientation Engine & History | `internal/orient/engine.go`, `history.go`, `docs.go`, `timeline.mg`, `lineage.mg`, `internal/config/orient.go`. |
| **`I1`** | Ecosystem Ingest & Agents | `internal/orient/ecosystem.go`, `ecosystem_agents.mg`, `internal/init/phase_ecosystem.go`, deletion of `determineRequiredAgents` switch in `internal/init/agents.go`. |
| **`C2b / C2a`** | North Star Synthesis & Transduction | C2b: library `ClassifyDocuments` (`internal/northstar/derive.go:215`), `DraftVision` (`internal/northstar/derive.go:343`), `DeriveRequirements` (`internal/northstar/derive.go:381`), insert-only `InstallDerivedVision` (`internal/northstar/derive.go:545`) and TUI callers (`cmd/nerd/chat/northstar_llm.go:15`, `cmd/nerd/chat/northstar_llm.go:39`). Authored, runtime validation pending. C2a owns init, orientation reporting and fact persistence. |
| **`O1`** | Discernment, Privacy & Questions | `internal/orient/trees.go`, `trees.mg`, `deps.go`, `deps.mg`, `mcp.go`, `mcp.mg`, `questions.mg`, `internal/config/merge.go`, TUI first-boot clarification hook. |
| **`L1`** | Workspace Membership Authority | `internal/workspace/membership.go`, repointing of ~35 production walkers to ask `Membership.Includes`. |
| **`P1`** | Polyglot CodeDOM | `internal/world/codemodel/` element models and surgical edit tools for Python, TypeScript, TSX, and JS. |
| **`P2`** | Polyglot World Model | Tree-sitter AST, import resolution, JSX call edges, and test linking for Python and TS/JS in `internal/world/`. |
| **`P3`** | Polyglot Context & Test Runners | Holographic impact blocks and typed test runners (`pytest`, `vitest`) in `internal/world/holographic.go` and `internal/tools/codedom/`. |
| **`R1`** | Meta Search Grounding | Provider config and `/responses` search grounding integration in `internal/perception/client_openai_compat.go`. |
| **`R2`** | Native Browser Research | Rerouting all research web fetches through Rod/CDP native browser in `internal/tools/research/`. |

---

## 3. Historical baseline in preexisting subsystems (not reverified by C2a)

### C2b library source supplement (not runtime-verified)

The north-star library now contains the full-reading and synthesis implementation described in [07-NORTH-STAR-DERIVATION.md](07-NORTH-STAR-DERIVATION.md). Exact source and authored test witnesses are listed there. `TestDerive_LosslessPaging` (`internal/northstar/derive_test.go:95`), `TestDerive_WizardDocumentLinkIntegrity` (`internal/northstar/derive_test.go:214`), `TestInstallDerivedVision_PreservesExistingAuthority` (`internal/northstar/derive_test.go:316`) and the TUI witnesses (`cmd/nerd/chat/northstar_llm_test.go:50`, `cmd/nerd/chat/northstar_llm_test.go:84`) have not been executed by this author-only lane. They do not close `GAP-ORIENT-06` or `GAP-ORIENT-07` until the validation owner runs their exit commands. Shared `OrientConfig` registration of the typed derivation section is still outside C2b scope (`internal/config/northstar_derive.go:13`, `NorthstarDeriveConfig`).

The codeNERD repository contains robust production foundations that the incoming orientation lanes build upon and integrate with:

### A. Initialization Pipeline (`internal/init`)

`Initialize` inserts orientation after the membership scan and before profile
creation (`internal/init/initializer.go:536`). Agent materialization consumes
that engine (`internal/init/phase_ecosystem.go:115`, `integrateEcosystem`). The
independent phase-7b document relevance pass and filename score table were
removed; strategic storage is fed from orientation instead
(`internal/init/phase_orient.go:258`, `persistOrientationKnowledge`).

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
