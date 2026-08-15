# TODO — Context package backlog

> Last verified against codebase: 2026-08-15  
> Prioritized engineering backlog for `internal/context` and its wires  
> Docs-only corpus does not implement these items.

## P0 — correctness / safety

- [x] Audit every chat path that injects history for `IsCompressionActive` parity (perception + articulation + session context).  
      Result: parity holds. `process.go` gates both the perception and articulation
      paths; `model_session_context.go` injects the compressed block only and never
      raw `m.history`, which is why it is unconditional. Enforced by
      `internal/context/chat_history_parity_test.go` (static scan of `cmd/nerd/chat`),
      which fails if a new path mixes raw history with the compressed block without
      the gate, and fails vacuously-passing scans too.
- [x] Keep race coverage green: `go test -race ./internal/context/...` on activation changes.  
      `compressor_race_test.go` now also drives ProcessTurn / RefreshBudget /
      LoadState / BuildContext / GetState concurrently. This caught a real race:
      `RefreshBudget` used to drop `c.mu` before `recalcBudget`.
- [x] Preserve issue weight clamp + score caps when editing `activation_scoring.go`.  
      Encoded in `activation_caps_test.go`: dependency ≤ 40, campaign ≤ 60,
      issue ≤ 100, back-reference ≤ 70, feedback ∈ [−20, 20].

## P1 — kernel/Go hybrid

- [x] Measure frequency of Go fallback vs kernel inclusion in production logs; reduce dual-path drift.  
      `SelectionStats` (`GetSelectionStats`, and `kernel_selections` /
      `go_fallbacks` / `kernel_inclusion_rate` / `last_selection_reason` in
      `GetMetrics`) makes the split assertable instead of a debug line. Drift was
      also *fixed*, not just measured — see the entity-resolution note below.
- [x] Expand tests with loaded `context_compilation.mg` so `should_include_context` path is first-class.  
      `kernel_context_test.go` builds compressors over a real `RealKernel` (which
      loads the embedded policy) and drives C1 relevance, C3 masking, and the
      fallback path through `BuildContext`.
- [x] Finish C3: consume `should_mask_observation` in Go when building summaries (assert path already present).  
      The assert path was present but broken: it appended a clause terminator that
      `ParseFactString` adds itself, so every `turn_age_category` assertion failed
      to parse and the error was discarded. Masking had never fired. Fixed, and the
      kernel's decision is now consumed by `generateObservationMaskedSummary`.

## P2 — quality of compression

- [ ] Validate target compression ratio on real multi-hour sessions (campaign assault artifacts).  
      Needs recorded long-session artifacts; not reproducible from the repo alone.
- [ ] Optional provider-aligned tokenizer adapter behind `TokenCounter`.
- [x] Ensure `LoadState` + `RefreshBudget` always paired on session rehydrate.  
      `LoadState` now recalculates the budget itself, so the unpaired call is
      impossible rather than merely discouraged. `RefreshBudget` stays exported and
      idempotent.

## P3 — learning & JIT

- [ ] Wire audit: confirm prompt JIT actually calls `GetActivationScores` each turn when expected.  
      **Audit result: it does not.** Nothing in the tree calls
      `Compressor.GetActivationScores` or `GetHighActivationFactKeys`, and
      `prompt.CompilationContext.ActivatedFacts` is never populated or read. The
      activation→JIT feedback edge is dead. Wiring it requires changes in
      `internal/prompt` and `cmd/nerd/chat`.
- [x] Surface feedback store stats in glass-box / transparency UI.  
      `Compressor.GetFeedbackStats` / `context.CollectFeedbackStats` expose the
      snapshot, and `nerd context-stats` renders helpful vs noise predicates.
      The transparency UI itself still needs to consume `FeedbackStats`.
- [x] Document operator workflow for inspecting helpful vs noise predicates.  
      See [11-OBSERVABILITY.md](11-OBSERVABILITY.md) §9.

## P4 — docs hygiene

- [x] Align `internal/context/README.md` defaults (200k, current date, file list including feedback_store).
- [x] Remove or relocate crash-dump `debug_program_ERROR.mg` from package tree if not intentional.  
      Already relocated: `writeFailedProgramDump` writes to `.nerd/debug/`, the name
      is gitignored, and no `.mg` file remains under `internal/context/`. The corpus
      references to `internal/context/debug_program_ERROR.mg` were stale and are fixed.

## Done recently (do not re-open without evidence)

- Concurrent map fix on ActivationEngine.  
- Turn age calculation fix (turnNumber − turn id).  
- Core facts Query error logging (no silent empty safety set).  
- Issue keyword weight clamping.  
- `turn_age_category` assertion parse fix (C3 masking was dead).  
- Kernel-derived context entity resolution + missing Go fallback when kernel
  entities resolve to nothing (a live session shipped an empty ACTIVE CONTEXT block).  
- `GetOverallStats` NULL average on an empty feedback DB.
