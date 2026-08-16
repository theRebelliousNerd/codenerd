# TODO — Context package backlog

> Last verified against codebase: 2026-08-16  
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

- [~] Validate target compression ratio on real multi-hour sessions (campaign assault artifacts).  
      The assault artifacts in-repo (`.nerd/campaigns/*/assault/triage/*.json`) carry
      no turn or window data, so the item as written is not reproducible here. A
      deterministic 300-turn gate stands in: `long_session_test.go`. Running it found
      two defects, now fixed: compression never engaged on long sessions (window
      overflow was truncated, not compressed — 290 of 300 turns silently deleted),
      and the rolling summary grew without bound past the window, which made
      `BuildContext` refuse outright. Post-fix: 300 turns → 6 segments, 623:1, 56%
      window usage. Validation against a genuine multi-hour session is still open.
- [x] Optional provider-aligned tokenizer adapter behind `TokenCounter`.  
      `TokenEstimator` interface + `NewTokenCounterWithEstimator`; the default
      chars-per-token heuristic is exposed as `CharsPerTokenEstimator` so an adapter
      can wrap or compare against it. No provider tokenizer is vendored — the seam
      exists, the implementation is a caller's choice.
- [x] Ensure `LoadState` + `RefreshBudget` always paired on session rehydrate.  
      `LoadState` now recalculates the budget itself, so the unpaired call is
      impossible rather than merely discouraged. `RefreshBudget` stays exported and
      idempotent.

## P3 — learning & JIT

- [x] Wire audit: confirm prompt JIT actually calls `GetActivationScores` each turn when expected.
      **Audit result: it does not.** Re-audit 2026-08-16 re-confirms the original finding and adds why the wiring has never been done. (1) The audit itself is complete and re-confirmed: `Compressor.GetActivationScores` and `GetHighActivationFactKeys` are defined in `internal/context/compressor_metrics.go:356` and `:395` and have no callers outside that package — `GetHighActivationFactKeys` is the only caller of `GetActivationScores`; `prompt.CompilationContext.ActivatedFacts` (`internal/prompt/context.go:194`) is never populated or read in production; the identically named `ActivatedFacts` in `internal/testing/context_harness` is a different type and unrelated. (2) `ActivationThreshold` IS threaded, but only into the cache key: `internal/prompt/context.go:576` folds it into the sha256 canonical-content hash, so the field affects cache identity while the scores it thresholds affect nothing. (3) Blocker — no atom-to-fact relation exists to boost along: the documented intent at `context.go:191-193` is to "boost atoms related to highly-activated facts", but `PromptAtom` (`internal/prompt/atoms.go:135`) carries contextual selectors only — `OperationalModes`, `CampaignPhases`, `BuildLayers`, `InitPhases`, `NorthstarPhases`, `OuroborosStages`, `IntentVerbs`, `ShardTypes`, `Languages`, `Frameworks` — and no predicate or fact linkage; `ScoredAtom` (`internal/prompt/selector.go:459`) carries only `LogicScore`, `VectorScore` and `Combined`. (4) Consequence: this is not mechanical wiring. Closing it first requires DESIGNING the atom-to-fact relation — for example a `RelatedPredicates` field on `PromptAtom` that would then have to be populated across the whole atom library under `internal/prompt/atoms/`, or a derived relatedness via embeddings — and then deciding how much a hot fact should move an atom's `Combined` score. Both are architecture decisions that belong in an arch-propose pass, not an incremental fix, and guessing at them would silently change prompt selection quality on every turn. The audit itself is what this checkbox asked for and it is complete; the work it surfaced is tracked immediately below.
- Needs design - Making the activation->JIT edge live is blocked on a design decision, not on wiring. The documented intent at internal/prompt/context.go:191-193 is to boost atoms related to highly-activated facts, but no atom-to-fact relation exists to boost along: PromptAtom (internal/prompt/atoms.go:135) carries contextual selectors only - operational modes, campaign phases, build layers, init phases, northstar phases, ouroboros stages, intent verbs, shard types, languages, frameworks - and ScoredAtom (internal/prompt/selector.go:459) carries only LogicScore, VectorScore and Combined. Closing it requires first choosing how an atom relates to a fact - for example a RelatedPredicates field on PromptAtom, which would then have to be populated across the entire atom library under internal/prompt/atoms/, or a derived relatedness via embeddings - and then choosing how far a hot fact should move an atom's Combined score. Both choices change prompt selection on every turn, so guessing at them would silently alter model behaviour repo-wide with no way to attribute a regression. This belongs in an arch-propose pass with a golden-scenario comparison, not an incremental fix.
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
