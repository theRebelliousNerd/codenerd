---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# ADR-006: Init Writes the North Star Non-Interactively and Never Overwrites

## Context

Today, `nerd init` executes 22 sequential phases, but completely fails to initialize a North Star:
- `internal/init/initializer.go:796-804` opens `northstar.NewStore(nerdDir)` during Phase 1 purely to run SQLite schema migrations, and closes it immediately without writing a vision row.
- `internal/init/initializer.go:1447` outputs the final diagnostic instruction: `Use '/northstar' to define your project vision`.
- The only LLM drafting path lives inside the interactive Bubbletea chat model (`cmd/nerd/chat/northstar_llm.go:185-242`), which is completely unreachable from headless commands or scripted pipelines.

Consequently, projects initialized with `nerd init` start with an empty North Star. Background observers and the Guardian shard cannot evaluate project alignment, and the agent operates without its primary measuring stick until a human manually walks through an interactive TUI wizard.

## Decision

1. **Non-Interactive Synthesis in Library Code**:
   Extract document classification and North Star synthesis into a reusable, non-interactive library (`internal/northstar/derive.go`).
2. **Execution During `nerd init`**:
   Introduce a dedicated initialization phase (`internal/init/phase_orient.go`) that executes `ClassifyDocuments` and `DraftVision` over derived read candidates, saving the resulting `WizardDocument` directly to `.nerd/northstar_knowledge.db` via `Store.SaveVision`.
3. **Strict Invariant: Never Overwrite Existing Vision**:
   If `northstar.NewStore` discovers an existing vision row with a valid `Mission` statement (indicating prior user configuration or earlier initialization):
   - The existing database row is **never overwritten**.
   - The newly derived vision is written to `.nerd/northstar.derived.json` for reference.
   - The system logs an advisory note informing the operator that the existing vision was preserved.
4. **Diagnostic Cleanup**:
   Remove the instruction `Use '/northstar' to define your project vision` from `internal/init/initializer.go:1447` when a vision was successfully derived.

## Consequences

- **Positive**: Projects are fully oriented and aligned upon completion of `nerd init`; headless and automated CI initialization flows generate complete world models; existing operator commitments are strictly protected.
- **Negative**: Adds 1–2 LLM inference turns to the initialization pipeline.
- **Risks**: Repositories with minimal documentation may produce sparse or generic North Star drafts. In such cases, the draft is explicitly marked as a work-in-progress in its `VisionStmt`.

## Witness

**Witness:** `test:TestPhaseOrient_WritesNorthStarNonInteractively`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Init closes store empty today | `northstarStore.Close()` in `internal/init/initializer.go:796-804`. |
| Manual instruction printed today | `Use '/northstar' to define your project vision` in `internal/init/initializer.go:1447`. |
| Target phase implementation | `internal/init/phase_orient.go`. |
| Proving regression test | `TestPhaseOrient_WritesNorthStarNonInteractively` asserting that running `init` on a repository with design documents results in a non-empty `northstar_knowledge.db`. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The decision is approved, but `internal/init/phase_orient.go` has not yet landed in commit `e056692c`. Status flips to `implemented` once the witness test passes.
