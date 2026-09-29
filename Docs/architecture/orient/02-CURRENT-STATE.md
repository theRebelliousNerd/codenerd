---
doc-class: shipped
subsystem: orient
implementation-status: partial
last-verified: 2026-09-29
verified-against: working-tree-C2a
supersedes: []
---

# Current orientation source state

Source review only: C2a is an author-only lane. No build or test was executed.
The acceptance status is partial; the authoritative matrix is
[IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md).

- Init runs one orientation phase after scanning and before profile creation
  (`internal/init/initializer.go:469`, `Initialize`). Its phase inventory and ETA
  include orientation (`internal/init/initializer.go:712`, `newPhaseRunner`).
- Measurements come from one history/document/ecosystem census
  (`internal/orient/snapshot.go:77`, `Measure`). The duplicate init git, document,
  similarity and engine files are removed.
- The phase queries the Mangle read set, passes full bodies to the northstar
  classification API, asserts role claims, and invokes vision drafting
  (`internal/init/phase_orient.go:38`, `runOrientation`). Optional LLM failures
  are warnings; engine and required artifact failures fail initialization.
- Agents and their source/knowledge/prompt assignments use the retained engine
  (`internal/init/phase_ecosystem.go:115`, `integrateEcosystem`). Prompt source
  choice is derived (`internal/orient/ecosystem_agents.mg:109`, `orient_agent_prompt`).
- Strategic identity, capabilities, constraints and limitations retain their
  existing knowledge categories, using the stored vision when an existing
  authority prevents a draft installation (`internal/init/phase_orient.go:258`, `persistOrientationKnowledge`). The old filename-priority/relevance path is removed.
- Init timeout applies to one model request (`internal/init/jit_integration.go:399`, `withJITPrompt`); caller cancellation governs the run.
- The durable snapshot stores measured HEAD, document bodies/digests and EDB;
  its projection exports public measurements and judgments
  (`internal/orient/snapshot.go:181`, `Projection`; `internal/orient/snapshot.go:228`, `Save`).
- Boot invokes refresh before loading the projection beside northstar
  (`internal/core/kernel_init.go:309`, `loadMangleFiles`). Freshness is a Mangle
  conclusion (`internal/orient/freshness.mg:8`, `orient_stale`). Refresh removes
  changed role claims and preserves unchanged classifications and operator answers
  (`internal/orient/snapshot.go:281`, `Refresh`).

The existing incremental scan does not yet call the new refresh API. Changed
roles remain pending until an LLM-capable pass transduces them. The typed
orientation section is not registered on the live `UserConfig` in this tree;
its scoped reader is `LoadOrientConfig` (`internal/config/orient.go:97`).
