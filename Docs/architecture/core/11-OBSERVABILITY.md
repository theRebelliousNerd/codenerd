# core observability

Verified 2026-09-21 against commit `3463477` (`main`). This file answers one
question: what observability does `internal/core` expose, and where does each
signal go? Mechanisms of the kernel itself live in `INTERNALS.md`.

## Failure forensics

- A failed program rebuild is dumped to `debug_program_ERROR.mg` by
  `writeFailedProgramDump` (`internal/core/kernel_eval.go:543-553`). The dump
  concatenates schemas, policy, and learned rules as staged, so it can include
  user overrides — treat it as a support artifact, not a public paste.
- Load-time validation reports through `GetStartupValidationResult`
  (`internal/core/kernel_validation.go:730-740`): rejected vs. healed
  (`healLearnedRules`, `internal/core/kernel_validation.go:557-688`) vs.
  loop-risk (`checkInfiniteLoopRisk`,
  `internal/core/kernel_validation.go:365-478`).

## Derivation inspection

- `Explain` (`internal/core/kernel_provenance.go:72-112`) returns the
  derivation tree for a fact, but only when provenance was enabled up front
  (`EnableProvenance`, `internal/core/kernel_provenance.go:29-36`) and scoped
  (`SetProvenanceContext`, `internal/core/kernel_provenance.go:23-27`).
  There is no retroactive explanation: questions asked after the fact, without
  recording turned on, have no data.

## Live state reads

- `Query` / `QueryWithBindings` / `QueryCallback`
  (`internal/core/kernel_query.go:24-362`) inspect what holds right now;
  `QueryAll` (`internal/core/kernel_facts.go:791-851`) and `GetDerivedFacts`
  (`internal/core/kernel_facts.go:1083-1134`) cover bulk and derived reads.
- `IsBootGuardActive` (`internal/core/virtual_store.go:395-408`) answers
  whether routing is still globally denied by the boot guard — the first thing
  to check when every action is denied and policy looks correct.
- `UpdateSystemFacts` (`internal/core/kernel_sysfacts.go:24-107`) refreshes
  environment-derived facts, so re-reading after it shows the outside world
  as the kernel currently sees it.

## What is NOT here

- No external metrics exporter exists in the `kernel_*.go` surface; subsystem
  counters stay in-process (see `WIRING-AND-NOT-BUILT.md`).
- Deny paths do not say which stage denied: boot guard, Dreamer block,
  constitution, allowlist, and validator denials are not distinguished in the
  signal the caller gets back.
