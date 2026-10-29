---
doc-class: shipped
subsystem: context
implementation-status: shipped
last-verified: 2026-09-25
verified-against: 76afac9
supersedes: []
---

# context: wiring and what is NOT built

> Re-verified 2026-09-25 against `76afac9` by reading the code and running
> the tests named below. This supersedes the 2026-09-21 pass, which
> described `WorkingSet.Select` (removed; the context ledger replaced it)
> and called `ProcessTurn` and `GetContextString` uncalled (both have chat
> callers). Line anchors are to the tree at `76afac9`.

## Wired and reachable

- **Session compressor, built at boot.** `newSessionCompressor`
  (`cmd/nerd/chat/session_shared_boot.go:325`, called at `:131`) builds the
  `Compressor` on the domain Cortex's catch-all shard (`primary`), where the
  facts it asserts itself land, and sets the Cortex as its `KernelReader`
  (`SetKernelReader`, `internal/context/compressor.go:99`), so the
  retention and relevance decisions read the session's whole kernel. The
  context feedback store is attached at `session_shared_boot.go:146`.
- **Turn path.** `ProcessTurn` (`internal/context/compressor_turns.go:30`)
  is driven from chat (`cmd/nerd/chat/process.go:956`). It asserts the
  control packet's atoms, runs memory operations
  (`processMemoryOperation`, `:233`: `promote_to_long_term`, `forget`,
  `store_vector`, and `note` → `recordSessionNote`, `:287`), recalculates
  the token budget, compresses when `TokenBudget.ShouldCompress` holds, and
  persists (`persistTurnLocked`, `:185`), warning on and counting a failed
  write (`GetMetrics` `persist_failures`).
- **Context build.** `BuildContext` (`internal/context/compressor.go:706`),
  reached through `GetContextString` from `cmd/nerd/chat/process.go:783`
  and `cmd/nerd/chat/model_session_context.go:92`:
  - asks `should_include_context` through the reader; a kernel answer that
    resolves to facts is selected within `AtomReserve` without the Go
    threshold (`buildKernelDerivedContext`,
    `internal/context/compressor_metrics.go:553`, budget at `:663`);
    otherwise the Go activation engine selects, with the reason recorded
    in `SelectionStats`;
  - retains every fact of every predicate `context_must_retain` names
    (`getCoreFacts`, `compressor.go:859`, deciding with
    `retainedPredicates`, `:824`), falling back to `constitutionalFloor`
    (`:817`) only when the kernel derives none — warned once, counted as
    `RetentionFloorUsed`;
  - renders one line under ACTIVE CONTEXT saying whose order the block is:
    `selected and ordered by the kernel` or `heuristic_ordered: … (reason)`
    (`internal/context/serializer.go:279-281`).
- **Kernel policy for the window** (`internal/core/defaults/policy/context_compilation.mg`,
  Decls in `internal/core/defaults/schemas_context.mg`): relevance
  (`context_relevant` → `should_include_context`, including
  `context_relevant(Key, /p90) :- session_note(Key, _)`), retention
  (`context_must_retain`, `:85`), and observation masking
  (`should_mask_observation` / `should_preserve_reasoning`, obeyed by
  `maskedObservationTurns`, `compressor_metrics.go:722`).
- **Learned usefulness.** `process.go:859` stores the model's context
  feedback; the activation engine reads it in `computeFeedbackScore`
  (`internal/context/activation_scoring.go:462-467`). It affects the Go
  activation path only; the kernel path does not read it.
- **Per-task working set.** `working_set.mg` is embedded
  (`internal/context/working_set.go:24`) and loaded into the working set's
  private engine (`:62`). The tool loop calls `Continue` (`:325`, from
  `internal/session/executor_tools.go:297`, `build_verify.go:618`),
  `RepeatThreshold` (`:405`, from `executor_tools.go:209`,
  `build_verify.go:547`) and `Ledger` (`:222`, from
  `internal/session/working_context.go:556`). Eviction moves results behind
  recall handles; `RecallContextTool` recalls them (`WorkingSet.Recall`,
  `:108`).

## Exists but uncalled

- `ActivationEngine.ScoreFactsWithKernelOverride`
  (`internal/context/activation_scoring.go:653`): test-only, and not to be
  wired as it stands — it scores kernel priorities (≤100) and Go scores
  (100–250) on one scale, mixing derived and heuristic order silently.
  `BuildContext` takes one path per build and says which.
- `Compressor.GetSelectionStats` has no production reader; the same counts
  reach `GetMetrics`, and the chosen path is now rendered in the block.

## Assumed by the design, not done by the code

- **The Cortex answers `should_include_context` from the catch-all shard
  only** (open, `internal/core`). The world shard derives the
  modified-file row (`context_relevant(File, /p85) :- modified(File)`), and
  `core.BuildDerivationMap`'s `queryTargets` routes the predicate to the
  catch-all because its body's presence is `All` — a union of rules, not
  identical facts in every shard. So a modified file does not enter the
  window through the kernel path even with the reader wired. Retention is
  not affected: `block_commit` fans out correctly
  (`TestSessionCompressor_RetainsFactsOtherShardsDerive`).
- **Entity resolution sees the catch-all's facts only.**
  `buildKernelDerivedContext` resolves a kernel-named entity against the
  compressor's own kernel's fact snapshot, so a fact another shard owns is
  not pulled in by name.
- **`UnmarshalCompressedState`** (`internal/context/serializer.go:609`)
  validates JSON shape only; there is no state version to check.
- **Token counts are broker estimates**; `Confidence()` / `Ratio()` let a
  caller check calibration, and none in this package does.
- **Section order** (core → atoms → history → recent) is fixed in Go; the
  kernel orders facts within the active block only (TODO-CTX-06B).
