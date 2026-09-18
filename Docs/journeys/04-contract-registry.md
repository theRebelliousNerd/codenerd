# 04 — Contract Registry: seams, the contracts across them, and the test that catches each break

## Status

- last updated: 2026-09-18
- done: sections 1-5 complete; registry entries C-01..C-32, each anchor re-read in the main
  checkout on `dogfood/c2-closure`
- open:
  - **C-26 is sourced from commit messages, not from the retry code.** `b8b0c637`'s
    `defaultMaxOutputTokensFor` was verified at `internal/perception/client_openai_compat.go:132-145,183,215`;
    `44f3f2ff`'s empty-completion retry shape was **not** re-read in the tree.
  - **C-30's numbers are not re-measured.** The mechanism (`live.Clone()` per compile, fresh
    fact store, shared strata) is verified; the "19 clones / ~40 s of 106 s" figure comes from
    `M1-mangle-corpus-usage.md` §9, whose Verification reports `checked: 0`.
  - **Two inputs are unverified in full.** `01-docs-audit.md` (`checked: 0`) and
    `M1-mangle-corpus-usage.md` (`checked: 0`). Where this document leans on them it says so
    inline (C-03, C-27, C-30); everything else was checked here.
  - **C-19 (CodeDOM) is the thinnest entry.** The absence of a reader in `internal/prompt` /
    `internal/articulation` is carried from `03-knowledge-delivery-and-window.md` §4 rather
    than re-grepped; the write side was not enumerated.
  - **No test was run.** Every "existing test" citation is a file:line read, not an execution;
    `go test ./...` was deliberately not run (the brief forbids building).

## 1. How to read this registry

A contract is a statement both sides of a seam must agree on. It is not a description of
either side; it is the sentence that is false the moment one side changes alone.

An entry exists here because the two sides are written in different places, by different
authors, in different languages (Go and Mangle, Go and SQL, Go and YAML), and **nothing
executable pins their agreement**. The package-keyed corpus under `Docs/architecture/`
cannot hold these: it is isomorphic to the package tree, and a seam is not a package
(`01-docs-audit.md` §4-5, unverified — that document's Verification section is empty).

Read an entry as: *Side A believes X. Side B believes Y. X and Y must be the same
statement. Here is what happened when they were not, and here is the test that would
have said so.* Where the break column says "latent", nobody has paid for the drift yet;
that is not evidence the contract holds, only that no turn has hit it.

The **Invariant test** column is the point of the document. A contract without a test is
a comment. Prefer a test that drives the real kernel and the real corpus: three of the
ten fixes below (`c1c50cf9`, `b3a9ae67`, `94e8f7ae`) were shipped over tests that passed
against the bug because the test asserted the buggy shape by name or ran against a stub.

Every entry names the S-id from the program of record that owns its fix, so this file can
be read as the test plan for that program rather than as a second program.

## 2. Registry

Thirty-two contracts. Line anchors are from the main checkout on
`dogfood/c2-closure`, read 2026-09-18; every quoted line was read here, not carried from an
input document.

### The verdict boundary (S1)

#### `C-01` — a turn's verdict must survive the boundary that reports it

- **Shared thing:** the outcome of one delegated turn.
- **Side A:** `internal/session/executor_memory.go:128-146` — `captureTurnOutcome` sets a
  four-valued verdict: `result.TurnOutcome = types.MangleAtom("/hollow")`, `"/failed"`,
  `"/done"` (only when `e.kernel.Query("turn_done")` returns rows) or `"/unverified"`.
- **Side B:** `internal/session/observed_return.go:39-105` — `observedReturn` builds the
  value that crosses the boundary and copies `Output`, `Duration`, `Failure`, `Changed`,
  `Build`, `Tests`, `Findings`, `Notes`. `observation.Return`
  (`internal/observation/subagent.go:97-115`) has no field for `TurnOutcome` and none for
  `ChangeStage`.
- **Contract:** every verdict the executor derives for a turn is representable in the value
  the parent receives, or it is not a verdict.
- **Break observed:** latent by construction — the verdict is computed and dropped at the
  same call boundary on every delegation, so there is no incident to point at, only the
  absence of one.
- **Invariant test:** TO WRITE — `TestObservedReturnCarriesTheTurnVerdict`
  (`internal/session`): run a real `ExecutionResult` whose `TurnOutcome` is `/hollow`
  through `observedReturn` and assert the projection can distinguish it from `/done`.
- **Seam:** S1. Landing now.

#### `C-02` — the parent-visible status vocabulary must not contradict the executor's

- **Shared thing:** the word the parent reads for "how did that go".
- **Side A:** `internal/observation/subagent.go:460-467` — `ProjectReturn` sets
  `result.Status = StatusFailed` when `Failure != ""`, `StatusEmpty` when the output trims
  to empty, and `StatusCompleted` otherwise. Constants at `:179-181`.
- **Side B:** `internal/session/executor_memory.go:133-145` (four-valued `TurnOutcome`) and
  `internal/session/verify_outcome.go:22-36` (five-valued `VerifyOutcome`, including
  `VerifyIndeterminate`).
- **Contract:** `completed` must not be derivable for a turn the executor called `/hollow`
  or `/unverified`.
- **Break observed:** latent. Today `StatusCompleted` means only "the subagent said
  something", which is exactly the hollow-success shape `coder_safety.mg:92-107` exists to
  refuse one layer down.
- **Invariant test:** TO WRITE — `TestProjectReturnStatusAgreesWithTurnOutcome`
  (`internal/observation`), table-driven over the four `TurnOutcome` atoms once C-01 lands.
- **Seam:** S1. Landing now.

#### `C-03` — `types.ShardResult.Result` is prose, and every caller must know it

- **Shared thing:** `types.ShardResult`.
- **Side A:** `internal/types/shard.go:100-105` — `ShardResult{ShardID, Result string,
  Error error, Timestamp time.Time}`. Nothing states what `Result` contains.
- **Side B:** `cmd/nerd/chat/process_continuation.go:167-197` — reads that prose and decides
  the kernel's status atom from it (C-04). Transport in between:
  `internal/session/task_executor.go:258-260` — `ExecuteWithContext` ends
  `observed, err := j.executeObserved(...)` / `return observed.Output, err`, i.e. the
  structured return is built and then narrowed to a string.
- **Contract:** either `Result` carries a declared vocabulary, or no caller may derive a
  status from it.
- **Break observed:** 2026-09-17, recorded as `01-docs-audit.md` §4 Seam 4 — that document
  is **unverified** (its `## Verification` reports `checked: 0`), but the two anchors were
  re-read here and hold verbatim.
- **Invariant test:** TO WRITE — the test is C-04's. That `ShardResult` itself cannot be
  tested is the finding.
- **Seam:** S1. Landing now.

#### `C-04` — the `shard_result` status atom set: writer ≡ consumer

- **Shared thing:** slot 2 of `shard_result/5`
  (`internal/core/defaults/schemas_misc.mg:188`,
  `bound [/string, /name, /name, /string, /string]`).
- **Side A (writer):** `cmd/nerd/chat/process_continuation.go:172-184` emits exactly four
  atoms — `/complete`, `/failed`, then
  `if strings.Contains(result, "TODO") || strings.Contains(result, "FIXME")` →
  `/incomplete`, and
  `if shardType == "coder" && !strings.Contains(strings.ToLower(result), "test")` →
  `/code_generated`.
- **Side B (consumer):** `internal/core/defaults/policy/codedom_continuation.mg:8,13,18,22,26`
  joins on `/code_generated`, `/incomplete`, **`/tests_needed`** and **`/review_needed`**.
- **Contract:** the set of status atoms written equals the set consumed.
- **Break observed:** latent and permanent — `codedom_continuation.mg:22` and `:26` can never
  fire; no Go writer emits either atom.
- **Invariant test:** TO WRITE — `TestShardResultStatusVocabularyIsClosed` (`internal/core`):
  scan the shipped corpus for every constant in slot 2 of a `shard_result` literal and compare
  it to an exported Go set the writer switches on. Fails when either side adds an atom alone.
- **Seam:** S1. Landing now.

#### `C-05` — `pending_test` means "this turn wrote no test", not "this prose lacks the word test"

- **Shared thing:** `pending_test/2` (`schemas_misc.mg:191`) and `pending_review/2` (`:194`).
- **Side A:** `cmd/nerd/chat/process_continuation.go:198-221` — asserts `pending_test` when
  `status == "/code_generated"`, and `pending_review` when
  `shardType == "reviewer" && strings.Contains(strings.ToLower(result), "issue")`.
- **Side B:** `internal/core/defaults/policy/codedom_continuation.mg:7-9` —
  `has_pending_subtask(TaskID, Description, /tester) :- shard_result(_, /code_generated,
  /coder, Task, _), pending_test(TaskID, Description).`
- **Contract:** the test-continuation obligation fires exactly when the turn produced code
  with no test.
- **Break observed:** live — a coder that wrote tests and said so suppresses the obligation;
  one that wrote none and never used the word triggers it. The executor already measures the
  truth: `UntestedPaths` / `TestCheck` (`internal/session/observed_return.go:57-95`).
- **Invariant test:** TO WRITE — `TestPendingTestFollowsTheWriteSetNotTheProse`
  (`cmd/nerd/chat`), driving a real kernel: a result whose write set names `_test.go` files
  must derive no `has_pending_subtask(_, _, /tester)`; one with writes and no test file must.
- **Seam:** S1 for the fact, S6 for the obligation it stands in for. S1 lands now.

### The shard task string (S2)

#### `C-06` — `TaskRequest.Task` has one shape

- **Shared thing:** the string built by `formatShardTask` and carried as
  `session.TaskRequest.Task`.
- **Side A:** `cmd/nerd/chat/delegation.go:362-366` — `func formatShardTask(verb, target,
  constraint, workspace string) string` opens `if target == "" || target == "none" { target
  = "codebase" }`: slot 2 is a **location**. The `/fix` arm renders `"fix issue in
  <target>"`, quoted back at `delegation.go:313` (*"observed 2026-09-17: the coder was started with fix issue in ..."*).
- **Side B (free text):** `cmd/nerd/chat/process.go:479` —
  `task := formatShardTaskWithContext(intent.Verb, intent.Target, intent.Constraint,
  m.workspace, m.lastShardResult)`; `intent.Target` is the classifier's extracted location.
- **Side B′ (slash `/fix`):** `cmd/nerd/chat/commands_handlers_evolution.go:19-21` —
  `target, constraint := splitSlashTarget(parts[1:])`. `splitSlashTarget`
  (`delegation.go:329-338`) returns the **whole joined description** as `target` whenever the
  first token fails `looksLikePath` (`:342-359`), so `/fix the auth token expires early`
  builds `"fix issue in the auth token expires early"`. This is narrower than
  `00-journey-map.md` S-6 states (it says the whole description is always the target); the
  path-shaped first token is now split out, and only the non-path case still breaks.
- **Side C:** `internal/system/factory.go:452-456` forwards it verbatim into
  `session.TaskRequest{IntentVerb: shardType, Task: task, Target: target}`;
  `internal/session/work_steps.go:131-135` records the drift in a comment — *"The bare task
  was sent before, and on the chat path the task is a one-line summary built from the
  intent (fix issue in <target>)"*.
- **Contract:** slot 2 of `formatShardTask` is a location; the user's words travel in their
  own slot, or the template is not applied.
- **Break observed:** 2026-09-17, commit `f896bc91`: *"The request carried only the task
  string, which on the chat path is a one-line summary built from the intent."* Step planning
  was handed a 48-character brief and returned empty twice.
- **Invariant test:** TO WRITE — `TestFormatShardTaskTargetIsALocation` (`cmd/nerd/chat`):
  for every verb arm, a `target` that fails `looksLikePath` must reach the shard as a
  described request, never as the object of "in".
- **Seam:** S2. Later.

### Forcing as facts (S3, S4)

#### `C-07` — `turn_done` must be reachable on every path that can complete a turn

- **Shared thing:** `turn_done/1`.
- **Side A:** `internal/core/defaults/policy/coder_safety.mg:114-115` —
  `turn_executed(Verb) :- turn_evidence(Verb, _, _, _, _, _), !has_hollow_success(),
  !build_state(/failing).` and `turn_done(Verb) :- turn_executed(Verb), turn_acceptance(Verb,
  _, _).`
- **Side B:** `internal/session/executor.go:2334-2336` — `if result.Acceptance != nil &&
  result.Acceptance.Status == "verified" {` … assert `turn_acceptance`. The only production
  writer of an acceptance contract is `cmd/nerd/cmd_direct_actions.go:313`, reachable only
  through `--acceptance`, registered only on `fixCmd` (`cmd/nerd/main.go:200`).
- **Contract:** the corpus's only positive completion predicate is derivable from at least
  one ordinary user path, or the system has no notion of done.
- **Break observed:** latent and total — every chat turn, every slash command and every CLI
  verb except `nerd fix --acceptance` ends `/unverified`; `executor.go:2563-2572`
  `consumeTurnDoneSignal` queries it and writes a Debug line either way.
- **Invariant test:** partial existing — `internal/session/turn_done_test.go:39-81` pins only
  the negative (`turn_done` must not derive while `build_state(/failing)` holds). TO WRITE:
  `TestTurnDoneIsReachableFromAChatTurn` (`internal/session`), driving `NewRealKernel` with
  the shipped corpus.
- **Seam:** S4. Later.

#### `C-08` — a predicate a verdict rule negates must have a producer

- **Shared thing:** `build_state/1` (`internal/core/defaults/schemas_shards.mg:238`,
  `bound [/name]`).
- **Side A:** `coder_safety.mg:114` negates it inside `turn_executed`; `coder_build.mg:17-21`
  and `coder_diagnostics.mg:19-23` join it for `block_commit` and `build_healthy`.
- **Side B:** `internal/session/executor.go:216-219` — *"perTurnBuildStateFacts tracks
  build_state facts asserted this turn via recordBuildState."* In the main checkout
  `recordBuildState` **does not exist**, `perTurnBuildStateFacts` has no reader and no
  writer, and the only `"build_state"` literals outside `.mg` files are that comment,
  `internal/shards/registration.go:140` (a shard-ownership list) and
  `internal/session/turn_done_test.go:39`.
- **Contract:** every predicate a verdict rule negates has at least one production producer,
  or the guard is vacuously true.
- **Break observed:** latent and structural — the "a red build is not done" guard has never
  been able to fire in production. A struct field naming a function that is not in the tree
  is the tell.
- **Invariant test:** TO WRITE — `TestBuildStateIsAssertedByTheTurn` (`internal/session`):
  run a turn whose build check failed against a real kernel and assert `build_state(/failing)`
  is queryable before the per-turn retraction. Generalise as
  `TestEveryNegatedVerdictPredicateHasAProducer` (`internal/core`) over the shipped corpus.
- **Seam:** S4 (and S1 supplies the evidence). Later.

#### `C-09` — the loop's report to the working policy: five numbers, in this order

- **Shared thing:** `working_progress/5` and `working_control/2`.
- **Side A:** `internal/context/working_set.mg:28,34` —
  `Decl working_control(Cycle, FailedRounds) bound [/name, /number].` and
  `Decl working_progress(Intent, Rounds, Writes, SinceWrite, SinceVerify) bound [/name,
  /number, /number, /number, /number].`
- **Side B:** `internal/context/working_set.go:179-182` asserts both plus
  `working_regime_now`, with `intent` ∈ {`/read`, `/write`} and `flag` ∈ {`/yes`, `/no`};
  the *values* come from `internal/session/tool_budget_controller.go:332-334`
  `workingProgress`, whose sole production caller is
  `internal/session/executor_tools.go:275`.
- **Contract:** the five slots mean what the Decl comment says, and the controller is the
  only thing that fills them.
- **Break observed:** latent. The exposure is that deleting the count ceiling (S3's aim)
  deletes the struct that produces these values, silencing five of the six policy stops —
  `working_stop`, `working_finalize`, `working_nudge`, `working_regime`, and the
  `/repeated_cycle` / `/tool_failures` controls — not merely the counting.
- **Invariant test:** TO WRITE — `TestWorkingProgressSlotsMatchTheDecl` (`internal/context`),
  asserting and reading back through the real private engine so an arity or order change
  fails; paired with `TestDeletingTheCountCeilingKeepsThePolicyStops` (`internal/session`).
- **Seam:** S3. Landing now.

#### `C-10` — exactly one ceiling decides when a tool loop ends

- **Shared thing:** the end of the tool loop.
- **Side A:** `internal/session/executor_tools.go:206` —
  `for iter := 0; openRounds || iter < budget.iterationLimit; iter++`, with
  `iterationLimit`/`hardLimit` reconciled against `MaxToolCalls` in
  `internal/session/tool_budget_controller.go:65,79-92`.
- **Side B:** `internal/context/working_set.mg:55-58` — four policy constants,
  `working_nudge_rounds(8).` `working_commit_rounds(16).` `working_finalize_rounds(16).`
  `working_stall_rounds(24).` — which reference neither Go ceiling.
- **Contract:** one authority ends the loop; the other reports.
- **Break observed:** acknowledged in source at `executor_tools.go:247-252` (a user who sets
  `max_tool_iterations` on a progress-driven loop gets both bounds).
- **Invariant test:** TO WRITE — `TestToolLoopEndsForExactlyOneReason` (`internal/session`):
  drive the loop to each terminator and assert the recorded reason is unique and is a fact,
  not a log line (`executor_tools.go:428-440` routes budget-exhausted and task-done into the
  same `forceFinalAnswer`).
- **Seam:** S3. Landing now.

### Knowledge delivery and the window (S12, S13, S14, S15, S16)

#### `C-11` — `context_budget/2` must be asserted every turn, by the turn

- **Shared thing:** `context_budget(ShardID, Budget)`
  (`internal/core/defaults/schemas_prompts.mg:136`, `bound [/string, /number]`).
- **Side A (consumers):** `internal/core/defaults/policy/prompt_context.mg:113-122` —
  `context_budget_constrained(ShardID) :- active_shard(ShardID, _), context_budget(ShardID,
  Budget), Budget < 5000.` and the `>= 5000` twin — feeding `final_injectable` at `:125-132`;
  and independently `internal/core/defaults/policy/stage_context.mg:285-300`, where
  `stage_shard_tool_allowed(ShardID, ToolName)` gates the **per-shard tool catalogue** on the
  same two predicates.
- **Side B (producer):** none. Repo-wide, the only assertions of `"context_budget"` are
  `internal/core/stage_context_test.go:66` and `:90`. `internal/campaign/recurse_plan.go:90`
  and `types.go:215` carry a Go `ContextBudget` field that feeds `ContextPager` arithmetic and
  never becomes a fact. `internal/shards/registration.go:320` lists the predicate as shared
  state, which reserves it rather than writing it.
- **Contract:** every compiling shard has a `context_budget` fact, or every rule that gates on
  the budget is dead.
- **Break observed:** latent and total. Both budget branches are unsatisfiable, so
  `final_injectable/2` is empty and `stage_shard_tool_allowed/2` is empty — the tool-shaping
  layer as well as the pruning layer. This is wider than
  `03-knowledge-delivery-and-window.md` §8 describes; its own Verification (refuted claim 5)
  already flags that `stage_context.mg` was never cited.
- **Invariant test:** TO WRITE — `TestEveryCompileAssertsAContextBudget` (`internal/prompt`
  or `internal/session`), driving a real compile and asserting the fact exists in the
  compilation scope; plus `TestStageShardToolAllowedDerivesForALiveShard` (`internal/core`)
  which must not hand-assert the budget.
- **Seam:** S12. Later.

#### `C-12` — `final_injectable/2` is the window's pruning decision, or it is deleted

- **Shared thing:** `final_injectable(ShardID, Atom)`
  (`internal/core/defaults/policy/prompt_context.mg:125-132`).
- **Side A:** the corpus derives it (all rows when the budget is sufficient, high-priority
  rows only when constrained) and boosts activation from it at `:136-138`.
- **Side B:** `internal/prompt/compiler.go:1076` queries the **unbudgeted**
  `injectable_context` directly (`ctxFacts, err := c.kernel.Query("injectable_context")`),
  and `internal/articulation/prompt_assembler.go:574-582` does the same on the legacy path.
  `final_injectable` has zero production callers.
- **Contract:** the prompt reads the budgeted set, or the budgeted set does not exist.
- **Break observed:** latent. The vision's "where a fact sits in the window is a decision"
  has a derived implementation that nothing calls.
- **Invariant test:** partial existing —
  `internal/core/mg_decl_body_literal_wiring_test.go:130` hand-asserts a `final_injectable`
  fact and exercises the derivation downstream of it, which pins the rule, not the wiring
  (its characterisation in `03-…` §8 as a blanket Decl check is refuted by that document's own
  Verification, row 4). TO WRITE: `TestCompilerReadsFinalInjectable` (`internal/prompt`).
- **Seam:** S12. Later.

#### `C-13` — slot 1 of `injectable_context` is a shard id or one of exactly two wildcards

- **Shared thing:** `injectable_context(ShardID, Atom)`.
- **Side A (writer):** `internal/core/defaults/policy/prompt_northstar.mg:142-152` states the
  contract in a comment and then obeys it — *"Slot 1 of injectable_context is ShardID
  (/string): a shard instance id, or the wildcard "*" / "/_all" meaning every shard …
  These rules used to tag slot 1 with a category name (/northstar_mission, /critical_cap,
  …), which is neither a shard id nor a wildcard, so no reader ever selected these rows and
  northstar context reached no prompt."* All five rules at `:153-185` now emit `"*"`.
- **Side B (readers):** `internal/prompt/compiler.go:1058-1071` — `matchesShard` treats only
  `"*"` and `"/_all"` as wildcards and otherwise compares against `cc.ShardInstanceID` then
  `cc.ShardID`; `internal/articulation/prompt_assembler.go:576-582` binds
  `injectable_context(%q, _)` plus the two wildcard literals.
- **Contract:** every rule that derives `injectable_context` puts a shard id or one of those
  two literals in slot 1.
- **Break observed:** past, and recorded in the corpus itself — category names in slot 1 meant
  northstar mission, capabilities, risks and constraints reached no prompt at all. The comment
  is the only thing preventing a recurrence.
- **Invariant test:** TO WRITE — `TestInjectableContextSlotOneIsSelectable` (`internal/core`):
  over the real corpus, derive every `injectable_context` row reachable from seeded facts and
  assert each slot-1 value satisfies the same predicate `matchesShard` implements. This is the
  one entry where the corpus wrote the contract down and still has no test.
- **Seam:** S12. Later.

#### `C-14` — the `prompt_atoms` column set across `internal/prompt` and `internal/store`

- **Shared thing:** the `prompt_atoms` table, created by two packages.
- **Side A:** `internal/prompt/loader.go:104-136` — `CREATE TABLE IF NOT EXISTS prompt_atoms
  (…)` including `embedding_model TEXT` at `:131`; `:154` lists
  `cols := []string{"description", "content_concise", "content_min", "embedding_model"}` for
  its own add-column pass.
- **Side B:** `internal/store/local_core.go:346-391` — a different `CREATE TABLE IF NOT
  EXISTS prompt_atoms (…)` with eleven selector-JSON columns the prompt side lacks
  (`operational_modes` … `world_states`) and **no `embedding_model`**;
  `internal/store/migrations.go:57-62` adds `description`, `content_concise`, `content_min`
  and `source_file` to an existing table and still does not add `embedding_model`.
- **Side C (reader):** `internal/prompt/vector_searcher.go:91-124` — selects
  `COALESCE(embedding_model, '')`, falls back to a legacy query when the column is missing,
  and skips every row where `storedModel != modelName`.
- **Contract:** a `prompt_atoms` table opened by either package has the column set the
  searcher requires.
- **Break observed:** 2026-09-17, commit `94e8f7ae`: *"all 914 atoms in
  .nerd/prompts/corpus.db NULL after a full reembed, and the vector tier had nothing to rank
  on the turn meant to validate the selector."*
- **Invariant test:** existing — `internal/store/prompt_reembed_stamp_test.go:16`
  `TestReembedAllPromptAtomsForce_StampsTheEngineOnEveryAtom` and `:58`
  `…_AddsTheStampColumnToALegacyDB`; `internal/prompt/vector_searcher_model_test.go`. TO
  WRITE: `TestPromptAtomsSchemaMatchesAcrossPackages` (`internal/store`), opening a DB created
  by each package and comparing the column sets — the missing test is the *schema* one, not
  the stamp one.
- **Seam:** S13 in spirit (knowledge must reach the window); the fix landed 2026-09-17.

#### `C-15` — the `embedding_model` stamp is the string the live engine reports

- **Shared thing:** the value in `prompt_atoms.embedding_model`.
- **Side A (writers):** `internal/store/prompt_reembed.go:181` — `UPDATE prompt_atoms SET
  embedding = ?, embedding_task = ?, embedding_model = ? WHERE atom_id = ?`, stamped with
  `engine.Name()`; `internal/prompt/loader_embedding.go:270-287` upserts the same column.
- **Side B (reader):** `internal/prompt/vector_searcher.go:118` — `if storedModel !=
  modelName { … skipped[key]++; continue }`, where an empty stamp is bucketed as
  `"unstamped"`.
- **Contract:** the writer's `engine.Name()` and the searcher's `modelName` are the same
  string for the same configured engine.
- **Break observed:** 2026-09-17 (`94e8f7ae`, the NULL case) — and the symptom was invisible
  until `781d032b` made the empty vector tier say so: *"With every stored vector unstamped …
  the log shows nothing to distinguish "kept 0 of N" from "never ran"."*
- **Invariant test:** TO WRITE — `TestStampedAtomsAreSearchableByTheSameEngine`
  (`internal/prompt`): re-embed with a real engine, then search with the same engine and
  assert `skipped["unstamped"] == 0`.
- **Seam:** S13. Landed for the writer; the round-trip is untested.

#### `C-16` — every string in the system prompt is a counted atom

- **Shared thing:** the compiled system prompt.
- **Side A:** `internal/prompt/compiler.go` budget accounting — atoms carry `token_count` and
  are fitted to `jit.token_budget`.
- **Side B:** `cmd/nerd/chat/process.go:831-832` — `// Inject the "Steven Moore Flare" persona` / `systemPrompt += "\n\n" + stevenMoorePersona`, appended unconditionally
  **after** compilation. The constant is at `cmd/nerd/chat/process_knowledge.go:271`, and the
  same persona is prepended again on two other paths (`process_knowledge.go:179`,
  `process_dream_delegation.go:421`). Sibling cases in the executor:
  `internal/session/executor.go:1058-1064` (`nerd.md` project instructions) and `:1065-1070`
  (file context).
- **Contract:** nothing reaches the model's system prompt outside the atom/budget system.
- **Break observed:** latent — the budget the selector fits to is not the budget the model
  receives, so a saturating compile (the measured 99.2 % incident at
  `internal/core/defaults/policy/jit_selection.mg:22-28`) overshoots by the persona's length
  with no accounting.
- **Invariant test:** TO WRITE — `TestSystemPromptIsExactlyTheCompiledAtoms`
  (`cmd/nerd/chat`): assert the string sent equals the compiler's output, or that every
  appended block is registered as an atom with a token count.
- **Seam:** S14. Landing now.

#### `C-17` — compression always leaves a marker

- **Shared thing:** "the model was given less than there was".
- **Side A:** `internal/prompt/limits.go:30-32` — `clampMarkerPrefix = "[codenerd:
  truncated"`, described as *"the visible truncation marker's stable prefix. Tests and log
  scrapers match on it"*, with a 2:1 head/tail split (`clampTailDivisor = 3`) and
  `TruncationNotice` for dropped rows.
- **Side B:** `internal/session/executor.go:2082-2093` — `priorTurnMessages` drops whole
  turns off the front of the history (`msgs = msgs[len(msgs)-window:]`, then pairs until the
  char budget fits) and leaves nothing behind. `cmd/nerd/chat/process_continuation.go:244-251`
  `truncateSummary` appends `"..."` with no label or count;
  `internal/core/shards/manager_spawn.go:675-677` appends `"... (truncated)"` at 4000 chars.
- **Contract:** every elision in a model-facing string is announced, with what was cut and how
  much.
- **Break observed:** latent for the history path; the pattern it violates is the one the same
  binary enforces two packages away.
- **Invariant test:** TO WRITE — `TestHistoryEvictionLeavesAMarker` (`internal/session`) and a
  package-level `TestEveryModelFacingTruncatorUsesClampMarker` (`internal/prompt`) that fails
  when a new truncation site ships without the prefix.
- **Seam:** S15. Later.

#### `C-18` — `ActivatedFacts` is populated with a cache-safe key, or it is deleted

- **Shared thing:** `CompilationContext.ActivatedFacts map[string]float64`
  (`internal/prompt/context.go:255`) and `ActivationThreshold` (`:258`).
- **Side A:** the field is typed, part of `Hash()` (the prompt cache key), deep-copied by
  `Clone()` (`:464-472`), and documented at `:226-254` as *"UNPOPULATED AND UNREAD, and not
  for want of wiring"*, with the two blockers named: no fact→atom relation exists in the
  corpus schema, and populating it with live scores turns the prompt cache off.
- **Side B:** `internal/context/activation.go` `SpreadFromSeeds` — the producer that would
  fill it has no production caller (two references repo-wide: its definition and its test).
- **Contract:** either both halves are wired with a quantised cache key, or both are deleted
  — the repo's own NO-SHIMS precedent (`jit_logic.mg:38-41`: delete the dead side, keep the
  reasoning).
- **Break observed:** latent; two near-misses already pinned by
  `TestActivatedFactsWouldDefeatThePromptCache` and `TestCloneDeepCopiesActivatedFacts`.
- **Invariant test:** existing tests pin the *hazards*. TO WRITE on wiring:
  `TestActivationScoresReachAtomSelection` (`internal/prompt`) — or, if the decision is
  deletion, the test is the compiler build failing on a removed field.
- **Seam:** S16. Later.

#### `C-19` — CodeDOM facts must be readable back into a prompt

- **Shared thing:** `code_element`, `code_defines`, `code_calls`, `code_implements` and the
  dataflow facts.
- **Side A:** `internal/core/virtual_store_codedom.go` and `internal/world` parse every
  element the model edits into these facts.
- **Side B:** no consumer in `internal/prompt` or `internal/articulation` queries them back
  into a compiled prompt; the only bridge is static YAML atoms teaching the model to call the
  CodeDOM tools itself.
- **Contract:** the structured knowledge the system builds about the code is deliverable, or
  the vision's "it never has to grep around" is unmet by construction.
- **Break observed:** latent; measured indirectly in
  `03-knowledge-delivery-and-window.md` §6 (89 delivered atoms vs 18 tool-driven reads in one
  real turn).
- **Invariant test:** TO WRITE — `TestCodeDOMFactsReachTheWindow` (`internal/prompt`): seed a
  kernel with `code_element` facts for the turn's target and assert the compiled prompt names
  them.
- **Seam:** S13. Later.

#### `C-20` — the tool-loop engine's corpus is a superset of the rules its policy joins

- **Shared thing:** which `.mg` files are loaded where.
- **Side A:** `internal/core/kernel_init.go:310-329` — the chat kernel walks
  `defaults/policy` and loads **every** `*.mg` file into one program.
- **Side B:** `internal/context/working_set.go:46-61` — the tool loop builds a **private**
  engine and loads exactly `core.DefaultCorpusText()` schemas +
  `core.GetDefaultContent("policy/context_compilation.mg")` + the embedded
  `workingSetPolicy`. `activation.mg`, `coder_safety.mg` and `prompt_context.mg` are absent.
- **Contract:** a rule loaded into an engine has every predicate its body joins derivable in
  that same engine.
- **Break observed:** latent — `context_compilation.mg`'s fallback tier
  (`context_relevant(Fact, /p60) :- context_atom(Fact)`) depends on `activation.mg`'s
  `context_atom/1`, so the tier is live for chat and structurally inert inside the tool loop.
  Neither `nerd query` nor `nerd logic` against the main kernel can see `working_stop`,
  `working_finalize`, `working_nudge` or `working_regime` at all.
- **Invariant test:** TO WRITE — `TestWorkingSetCorpusIsClosedUnderItsRuleBodies`
  (`internal/context`): for every rule in the private engine's program, assert every body
  predicate is declared *and* derivable there.
- **Seam:** S9. Later.

### The September defects, as contracts

Each of these is a seam that had already broken by the time it was found. The pattern is
uniform: both sides were individually correct, and the statement joining them was written
nowhere.

#### `C-21` — one boot guard, two structs, one untyped bridge

- **Shared thing:** the name `DisableBootGuard` / `IsBootGuardActive` / `bootGuardActive`.
- **Side A:** `internal/core/virtual_store.go:124` (`bootGuardActive bool`), `:209`
  (`bootGuardActive: true, // Prevent action execution until first user interaction`),
  `:395-408`; consumed at `internal/core/virtual_store_routing.go:41-43`.
- **Side B:** `internal/shards/system/executive.go:117`, `:154`
  (`bootGuardActive: true, // Prevent actions until first user interaction`), `:269-284`;
  consumed at `:326,337`.
- **Bridge:** `internal/core/shards/manager_tools.go:262-270` —
  `shard, ok := sm.GetRunningShardByConfigName("executive_policy")` then
  `if releaser, ok := shard.(interface{ DisableBootGuard() }); ok { releaser.DisableBootGuard() }`.
  The link is a **structural interface assertion on a method name**, so a rename on side B
  compiles and silently no-ops. Call site: `cmd/nerd/chat/process.go:150-155`, both guards
  released together on the first user message.
- **Contract:** the first user message releases *both* guards; the bridge releases the guard
  the running executive holds and changes nothing else.
- **Break observed:** 2026-09-17, commit `c1c50cf9`: the bridge added the running shard to the
  manager's disabled set instead of releasing its guard; *"the guard was still held thirteen
  minutes after boot and logged "suppressing 3 actions until user interaction" on the user's own turn."* The pre-existing test could not fail — it asserted the buggy behaviour by name.
- **Invariant test:** existing — `internal/core/shards/shards_coverage_test.go:486`
  `TestDisableExecutiveBootGuard_ReleasesTheRunningShardsGuard`, `:500`
  `…_DoesNotDisableTheShard`, `:513` `…_LeavesOtherRunningShardsAlone`.
- **Seam:** none of the S-ids owns it; it is the archetype the registry exists for. Landed.

#### `C-22` — a Go writer's argument shape matches the Decl it asserts into

- **Shared thing:** `learned_preference/2`, `learned_fact/2`, `learned_constraint/2`.
- **Side A:** `internal/core/defaults/schemas_memory.mg:93,96,99` —
  `Decl learned_preference(Predicate, Args) bound [/string, /string].` (and twins). The rules
  in `knowledge.mg` bind `Args` as one value.
- **Side B:** `internal/core/virtual_store_predicates.go:573` —
  `Args: []any{toAtomOrString(fact.Predicate), learnedArgsValue(fact.Args)}`, where
  `learnedArgsValue` (`:470-488`) returns the bare value for a one-argument row, `""` for
  none, and JSON otherwise. Before `b3a9ae67` it passed the stored `[]any` straight in.
- **Guard:** `internal/core/kernel_undeclared.go:109-125` `checkBoundAgainstValue` enforces
  only `/number` and `/string`.
- **Contract:** the Go value asserted into a `bound [/string]` slot is a string.
- **Break observed:** 2026-09-17, commit `b3a9ae67`: *"arg 1 declared /string, got
  []interface {}"* on every boot, 249 facts rejected — *"Nothing the user had taught the
  system reached the kernel, and the only test of the path used a stub kernel and asserted the
  rejected shape by name."*
- **Invariant test:** existing —
  `internal/core/mg_decl_body_literal_wiring_test.go:248 TestLearnedPreference_KeyIsAString`,
  which drives `NewRealKernel` with its real Decls. TO WRITE (generalisation):
  `TestEveryHydratorAssertsItsDeclaredShape` (`internal/core`) — the same class covers
  `modified_function` and `plan_edit` (`internal/mangle/agents.md:12-45`).
- **Seam:** none directly; it is the Decl-vs-writer archetype. Landed.

#### `C-23` — a periodic tick is throttled on exactly one side

- **Shared thing:** the `alignment_check` event.
- **Side A (emitter):** `internal/shards/observer_manager.go:358-385` — `periodicCheckLoop`
  emits `Type: EventAlignmentCheck` only when `periodicCheckDue()` is true, i.e.
  `atomic.SwapInt64(&m.eventsSinceCheck, 0) > 0`; the counter is incremented at `:284-286`
  for every event that is not itself a periodic check.
- **Side B (consumer):** `internal/northstar/observer.go:358-365` maps `"alignment_check"` to
  `TriggerPeriodic` and calls `h.guardian.CheckAlignment(ctx, trigger, subject, contextStr)`
  **directly** — bypassing `Guardian.ShouldCheckNow` (`internal/northstar/guardian.go:791-798`),
  which has its own throttle (`g.state.TasksSinceCheck >= g.config.PeriodicCheckInterval`) and
  is only reached from `guardian.go:836`.
- **Contract:** exactly one side decides whether a periodic alignment check is worth an LLM
  call.
- **Break observed:** 2026-09-17, commit `4e0aac61`: *"a check every five minutes from boot,
  each an LLM call of 13-43 s, each scoring the session 10-50/100 for having nothing to show
  — and the guardian's own recommendation on every one of them was to stop evaluating empty
  ticks."*
- **Invariant test:** existing —
  `internal/shards/observer_manager_periodic_test.go:40`
  `TestPeriodicCheck_FiresOnlyWhenSomethingHappenedSinceTheLastOne`, which runs the real loop
  at a 20 ms interval. TO WRITE: `TestObserverHandlerHonoursShouldCheckNow`
  (`internal/northstar`) — the second throttle is still bypassed.
- **Seam:** none; the observer tick is upstream of every S-id. Partly landed.

#### `C-24` — a re-embed writes only into `.nerd`

- **Shared thing:** which `*.db` files a force re-embed may rewrite.
- **Side A:** `internal/store/reembed_all.go:41-43` — `func ReembedSearchRoots(workspace
  string) []string { return []string{filepath.Join(workspace, ".nerd")} }`, the single source
  of the roots; `ReembedAllDBsForce` (`:48`) walks whatever it is handed.
- **Side B (callers):** `cmd/nerd/chat/reembed.go:62` and `cmd/nerd/embedding_cmd.go:159`,
  both of which must call the helper rather than assembling roots themselves.
- **Side C (the things that must not be touched):** `internal/core/defaults/*_corpus.db`
  (`go:embed`-ed into the binary) and `internal/shards/*_learnings.db` (tracked).
- **Contract:** a re-embed's roots come from one function, and that function names only the
  workspace `.nerd` tree.
- **Break observed:** 2026-09-17, commit `ce8f03ce`: *"one reembed dirtied five tracked
  databases and the next build shipped the rewritten copies."*
- **Invariant test:** existing — `internal/store/reembed_roots_test.go:48`
  `TestReembedSearchRoots_LeaveBuildInputsAlone`, which seeds a corpus under `.nerd` and one
  under `internal/core/defaults`, runs the real walker, and checks the seed is untouched.
- **Seam:** none. Landed.

#### `C-25` — a warning the user sees is durable

- **Shared thing:** a chat-turn warning string.
- **Side A:** 32 append sites across `cmd/nerd/chat` collect into one slice.
- **Side B:** `cmd/nerd/chat/system_warnings.go:23-34` — `renderSystemWarnings` now owns both
  the render and the log (`logging.Get(logging.CategorySession).Warn("user-visible warning:
  %s", w)` at `:30`), so a warning added later is durable by construction.
- **Contract:** every string shown to the user under "System Warnings" reaches the session
  log.
- **Break observed:** 2026-09-17, commit `3a7a7265`: three `[Kernel] Mangle update dropped`
  lines and a learnings-hydration failure were on screen and in none of the 26 log files that
  session wrote; a headless run lost them entirely.
- **Invariant test:** existing (the commit's test drives the real logging system against a
  real workspace and reads the session log back).
- **Seam:** none; it is the observability precondition for every other entry here — a contract
  break that is not logged cannot be dated.

#### `C-26` — an unconfigured LLM slot gets the model's output budget

- **Shared thing:** `max_output_tokens` for a slot.
- **Side A:** `internal/perception/client_openai_compat.go:132-145`
  `defaultMaxOutputTokensFor(vendor)`, applied at `:183` and `:215` only when the config value
  is absent.
- **Side B:** `.nerd/config.json`'s `worker` / `planner` / top-level slots — see
  `.claude/rules/nerd-config-schema.md`, "Two-tier routing". A slot that sets nothing shares
  the main client.
- **Contract:** an unset ceiling means the vendor's real ceiling for that model, never a flat
  constant, and an explicit config value is never overruled.
- **Break observed:** 2026-09-17, commit `b8b0c637`: every OpenAI-compatible slot without
  `max_output_tokens` fell back to 16384 while the project's own planner slot gave the same
  model 131072 — *"Shards ran at an eighth of what the model can emit."* The visible symptom
  was an **empty** reply with `finish_reason "stop"`, because the ceiling was consumed by
  reasoning before any content; that shape was then mis-handled again in `44f3f2ff` (the
  empty-completion retry fired only when the vendor returned reasoning text, which Meta does
  not).
- **Invariant test:** TO WRITE — `TestUnconfiguredSlotGetsTheVendorCeiling`
  (`internal/perception`), table-driven per vendor, plus
  `TestEmptyCompletionIsRetriedWhenTokensWereBilled` for the `44f3f2ff` shape.
- **Seam:** none of the S-ids; it is the substrate every shard runs on.

### The model-assertable surface

#### `C-27` — no predicate a verdict or budget rule reads is model-assertable

- **Shared thing:** the `mangle_updates` allowlists.
- **Side A:** `internal/core/mangle_updates.go:144-150` — `predicateAllowed` hard-denies
  `turn_acceptance, turn_evidence, turn_executed, turn_done, turn_cost` *"even with a
  permissive policy"*, then applies the caller's allowlist and prefixes;
  `validatePredicateDeclaration` (`:165-194`) additionally requires the predicate to be
  declared with matching arity.
- **Side B:** `internal/shards/system/planner.go:1071-1091` — five allowed predicates plus
  **ten prefixes**: `campaign_`, `phase_`, `task_`, `context_`, `plan_`, `replan_`, `build_`,
  `architectural_`, `suspicious_`, `eligible_`, at `MaxUpdates: 200`.
- **Contract:** the hard-deny list covers every predicate a verdict, budget or completion rule
  joins.
- **Break observed:** latent, and two instances are reachable today. `build_state/1` is
  declared (`schemas_shards.mg:238`) and matches the `build_` prefix, so a planner-asserted
  `build_state(/failing)` passes every filter and suppresses `turn_executed`
  (`coder_safety.mg:114`) — the model can veto its own completion. `context_budget/2` is
  declared (`schemas_prompts.mg:136`) and matches `context_`, so a planner-asserted
  `context_budget(...)` would become the **only** producer of the fact that gates
  `final_injectable` and `stage_shard_tool_allowed` (C-11), i.e. the model would decide its
  own tool catalogue. `M1-mangle-corpus-usage.md` §6 raises the prefix hazard in general terms;
  those two instances are verified here. That document is **unverified** overall
  (`checked: 0`).
- **Invariant test:** TO WRITE — `TestModelAssertablePredicatesDoNotFeedTheVerdict`
  (`internal/core`): for every predicate each shipped policy accepts (expanding prefixes
  against the live `programInfo` Decl set), assert it appears in no rule body reachable from
  `turn_done`, `turn_executed`, `hollow_success`, `final_injectable` or
  `stage_shard_tool_allowed`. This is the highest-value test in the registry: it is cheap, it
  runs against the real corpus, and it fails when either the allowlist or the corpus moves.
- **Seam:** S8 owns the surface; the deny list is policy the kernel should derive
  (`M1` §7 row 17: `model_may_assert(Surface, Pred)`).

#### `C-28` — the piggyback atom teaches exactly what the filter accepts

- **Shared thing:** the `mangle_updates` vocabulary.
- **Side A:** `internal/prompt/atoms/protocol/piggyback.yaml:84-104` — *"Only these
  predicates, at these arities"*, then a ten-line block (`observation`, `diagnostic`,
  `failing_test`, `test_state`, `review_finding`, `modified`, `modified_function`,
  `task_status`, `task_completed`, `missing_tool_for`).
- **Side B:** `internal/core/mangle_updates.go:30-53` accepts those ten **plus**
  `checkpoint_verdict`; `internal/shards/system/planner.go:1073-1091` accepts a different five
  plus ten prefixes; `internal/shards/system/perception.go:283-291` accepts two
  (`ambiguity_flag`, `clarification_needed`).
- **Contract:** every surface's accepted set is what the atom in that surface's window
  teaches.
- **Break observed:** live but low-cost — `mangle_updates.go:28-29` states the rule (*"Add a
  predicate here and there together"*) and the planner surface already violates it: nothing in
  its window describes the ten prefixes it may write. The atom's own comment records the cost
  of the inverse failure: while it was optional it reached *"0 of 150 recorded compiles, and
  every turn that filled mangle_updates did so blind"*.
- **Invariant test:** TO WRITE — `TestPiggybackAtomMatchesTheFilter` (`internal/prompt`):
  parse the predicate block out of the atom and compare it to the Go policy for that surface.
- **Seam:** S8 / S11. Later.

### The parallel executives and the cost of isolation

#### `C-29` — the command registry, the dispatcher and the tests describe one set

- **Shared thing:** the set of slash commands.
- **Side A:** `cmd/nerd/chat/command_categories.go:38` `var CommandRegistry = []CommandInfo{`
  — **69** entries (`grep -c 'Name:'` = 69), the source of truth for `/help`
  (`:553,588,603`).
- **Side B:** `cmd/nerd/chat/commands.go:54-266` — an independent `switch`. `/explain`
  (`:189`) and `/explain-off` (`:191`) are dispatched and **not** in the registry; `/recurse`
  is in both (`command_categories.go:281`, `commands.go:223`).
- **Side C:** `cmd/nerd/chat/commands_test.go:448` `TestCommand_Shards` and `:465`
  `TestCommand_Autopoiesis` call `handleCommand("/shards")` / `("/autopoiesis")` and assert
  `viewMode != ShardPage && len(result.history) == 0` is false — which the `default:` arm
  satisfies by printing `Unknown command`. `/shards`, `/autopoiesis` and `/facts` are in
  **neither** the registry nor the dispatcher; only the tests reference them.
- **Contract:** registry set = dispatcher set, and a test that names a command exercises that
  command.
- **Break observed:** live. Three commands exist only as test call sites, and the tests pass.
- **Invariant test:** partial existing —
  `cmd/nerd/chat/commands_handlers_features_test.go:74-82` pins one command's presence
  (*"/features is not in CommandRegistry, so /help will never list it"*). TO WRITE:
  `TestRegistryAndDispatcherAgree` (`cmd/nerd/chat`) — a set-equality test over the registry
  names and the `switch` cases, plus `TestNoTestNamesAnUndispatchedCommand`.
- **Seam:** S8. Later.

#### `C-30` — the price of a private compilation scope

- **Shared thing:** `KernelScopeProvider.NewCompilationScope`
  (`internal/prompt/compiler.go:58`).
- **Side A:** `internal/system/factory_adapters.go:79-101` — *"NewCompilationScope snapshots
  the production kernel for one JIT prompt compilation. Selector assertions and queries are
  therefore isolated across concurrent compiles and never mutate the live executive kernel."*
  It returns `NewKernelAdapter(live.Clone())`.
- **Side B:** `internal/core/kernel_eval.go:694-760` — `Clone()` gives the copy a
  `factstore.NewSimpleInMemoryStore()` (*"Fresh store"*) while sharing `programInfo`, `strata`
  and `predToStratum` as immutable. A fresh store plus a shared 900-stratum program means the
  first query on the clone re-derives everything.
- **Caller:** `internal/prompt/compiler.go:924-941` `acquireCompilationKernel` — one scope per
  compile, closed by a deferred `scope.Close()` (`factory_adapters.go:72-77`, which drops the
  adapter reference).
- **Contract:** isolation costs one full fixpoint over every stratum per prompt compile; that
  cost is accepted deliberately, or the scope is narrowed to the selector's strata.
- **Break observed:** measured, per `M1-mangle-corpus-usage.md` §9 (that document is
  **unverified**): 19 `Kernel cloned (facts=52767, policy=569969 bytes)` lines in one session,
  ~2 s each, ≈ 40 s of 106 s of evaluation. The mechanism is verified here; the numbers are
  not re-measured.
- **Invariant test:** TO WRITE — `BenchmarkCompilationScopeClone` (`internal/system`) with a
  regression guard: a compile must not re-derive strata whose predicates the selector never
  queries. The honest first step is a test that *records* which strata the selector touches.
- **Seam:** none of the S-ids; it is the standing cost of the JIT path.

#### `C-31` — a validated config key binds something

- **Shared thing:** `core_limits.max_facts_in_kernel`.
- **Side A:** `internal/config/limits.go:14` (`MaxFactsInKernel int \`yaml:"…"
  json:"max_facts_in_kernel"\` // EDB size limit`), validated at `:113-114`
  (*"max_facts_in_kernel must be >= 1000"*), defaulted to 250000 at `:145`; carried into
  `core.LimitsConfig` at `internal/system/factory.go:1743-1750`.
- **Side B:** `internal/core/limits.go:299-300` `GetMaxFactsInKernel()` has **zero**
  production callers (only `internal/core/coverage_boost_test.go:840` and
  `limits_coverage_test.go:45`). `internal/core/kernel_init.go:217-222` `SetMaxFacts` has zero
  production callers (five test call sites). The effective ceiling is
  `defaultMaxFacts = 250_000` (`kernel_init.go:215`), read at `kernel_facts.go:444-451` and
  `:775-780` whenever `k.maxFacts <= 0`.
- **Contract:** a config key that is validated and defaulted reaches the thing it names.
- **Break observed:** latent. The default happens to equal the constant, so lowering the key
  to 10000 in `.nerd/config.json` passes validation, is carried to an enforcer nobody asks,
  and changes nothing.
- **Invariant test:** TO WRITE — `TestMaxFactsInKernelBindsTheKernelCeiling`
  (`internal/system`): boot a real kernel with a low value and assert the (N+1)th assert
  fails. Generalise as `TestEveryValidatedCoreLimitHasAConsumer`.
- **Seam:** S17 of `00-journey-map.md` names the class; no program S-id owns it.

### Routing (S5)

#### `C-32` — the lane the kernel derives is the lane the turn takes

- **Shared thing:** `route_decision(Lane, Shard)` and the Go `RouteDecision`.
- **Side A:** `internal/core/defaults/policy/routing_arbitration.mg:106-112` —
  `route_decision(/clarify, /none) :- user_intent(/current_intent, /mutation, _, _, _),
  delegation_candidate(/current_intent, Shard, Conf), /none != Shard, Conf < 50,
  !wants_direct_answer().` The file's header states the rule for the Go side: *"Go … adds NO
  additional routing opinions"* (`:4-6`), and documents lane precedence at `:20-24`.
- **Side B:** `cmd/nerd/chat/delegation_routing.go:152-164` — a Go `switch` enforces the
  precedence the policy only documents, and maps `/clarify` to `RouteDecision{Kind:
  RouteClarify}` at `:161`. Repo-wide, `RouteClarify` appears only at its declaration (`:30`),
  its `String()` arm (`:43`), that assignment, and one test
  (`routing_arbitration_roundtrip_test.go:148`) — `cmd/nerd/chat/process.go` never reads it.
  Clarification is decided instead by `shouldAutoClarify`
  (`cmd/nerd/chat/process_dream_delegation.go:23-44`): `isBuildish && (looksLikeCampaign ||
  needsDetails)` over a seven-word substring bank, evaluated for **every** non-direct route,
  including one the kernel derived as `/delegate` or `/multi_step`.
- **Contract:** every lane the kernel can derive has a consumer, and no Go predicate can
  pre-empt a derived lane.
- **Break observed:** live — a derived `/clarify` is unreachable and an underived clarify is
  reachable, both at once.
- **Invariant test:** TO WRITE — `TestEveryDerivedLaneHasAConsumer` (`cmd/nerd/chat`): for
  each `route_decision` lane in the shipped corpus, assert `processInput` takes a distinct
  path; and `TestGoAddsNoRoutingOpinion`, asserting `shouldAutoClarify` cannot fire on a turn
  whose derived lane is `/delegate` or `/multi_step`.
- **Seam:** S5. Later.

## 3. Contracts by seam

`landing now` = S1, S3, S14 per the program of record. A contract with no S-id is one the
registry found that the program does not currently own.

| S-id | what it owns | contracts | tests to write | files touched |
|---|---|---|---|---|
| **S1** verdict from evidence | typed outcome across `types.ShardResult` / `observation.Return`; `injectShardResultFacts` substring guessing; the continuation summary; recovered tool errors | C-01, C-02, C-03, C-04, C-05 (+C-08, see §4) | `TestObservedReturnCarriesTheTurnVerdict`, `TestProjectReturnStatusAgreesWithTurnOutcome`, `TestShardResultStatusVocabularyIsClosed`, `TestPendingTestFollowsTheWriteSetNotTheProse` | `internal/observation/subagent.go`, `internal/session/observed_return.go`, `internal/session/executor_memory.go`, `internal/session/task_executor.go`, `internal/types/shard.go`, `cmd/nerd/chat/process_continuation.go`, `internal/core/defaults/policy/codedom_continuation.mg`, `internal/core/defaults/schemas_misc.mg` |
| **S2** task-string integrity | `formatShardTask` | C-06 | `TestFormatShardTaskTargetIsALocation` | `cmd/nerd/chat/delegation.go`, `cmd/nerd/chat/commands_handlers_evolution.go`, `cmd/nerd/chat/process.go`, `internal/system/factory.go`, `internal/session/work_steps.go` |
| **S3** forcing as facts, count ceilings deleted | the tool budget | C-09, C-10 | `TestWorkingProgressSlotsMatchTheDecl`, `TestDeletingTheCountCeilingKeepsThePolicyStops`, `TestToolLoopEndsForExactlyOneReason` | `internal/session/tool_budget_controller.go`, `internal/session/executor_tools.go`, `internal/context/working_set.go`, `internal/context/working_set.mg`, `internal/config/limits.go` |
| **S4** `turn_done` binding and reachable | `turn_acceptance` only from `--acceptance`; `build_state` never asserted | C-07, C-08 | `TestTurnDoneIsReachableFromAChatTurn`, `TestBuildStateIsAssertedByTheTurn`, `TestEveryNegatedVerdictPredicateHasAProducer` | `internal/core/defaults/policy/coder_safety.mg`, `internal/session/executor.go`, `cmd/nerd/cmd_direct_actions.go`, `cmd/nerd/main.go` |
| **S5** routing derived | `shouldAutoClarify` | C-32 | `TestEveryDerivedLaneHasAConsumer`, `TestGoAddsNoRoutingOpinion` | `internal/core/defaults/policy/routing_arbitration.mg`, `cmd/nerd/chat/delegation_routing.go`, `cmd/nerd/chat/process_dream_delegation.go`, `cmd/nerd/chat/process.go` |
| **S6** coverage obligation | what an uncovered file owes | C-05 (the fact it stands in for) | `TestPendingTestFollowsTheWriteSetNotTheProse`; then a producer test once the architect picks a consequence (§5) | `internal/core/defaults/policy/coder_safety.mg`, `internal/core/defaults/policy/delegation.mg`, `internal/core/defaults/policy/impact.mg`, `internal/core/defaults/tester.mg`, `internal/session/build_verify.go` |
| **S7** knowledge gate | "this agent must have X before acting" | **none** — one-sided today. No predicate states the obligation, so there is no second party to disagree with; C-11/C-12/C-13 are its preconditions | (deferred) | `internal/core/defaults/policy/prompt_context.mg`, `internal/prompt/selector.go` |
| **S8** slash commands and CLI verbs ask the kernel | `/shards`, `/autopoiesis`, `/facts` test-only | C-29, C-27, C-28 | `TestRegistryAndDispatcherAgree`, `TestNoTestNamesAnUndispatchedCommand`, `TestModelAssertablePredicatesDoNotFeedTheVerdict`, `TestPiggybackAtomMatchesTheFilter` | `cmd/nerd/chat/command_categories.go`, `cmd/nerd/chat/commands.go`, `cmd/nerd/chat/commands_test.go`, `internal/core/mangle_updates.go`, `internal/shards/system/planner.go`, `internal/shards/system/perception.go`, `internal/prompt/atoms/protocol/piggyback.yaml` |
| **S9** `working_set.mg` into the main kernel | two engines | C-20 (+C-09) | `TestWorkingSetCorpusIsClosedUnderItsRuleBodies` | `internal/context/working_set.go`, `internal/context/working_set.mg`, `internal/core/kernel_init.go`, `internal/core/defaults/policy/context_compilation.mg` |
| **S10** docs corpus | delivering `Docs/` as knowledge | **none** — one-sided; becomes two-sided the moment a delivery path exists, at which point it inherits C-13 and C-16 | (deferred) | `internal/prompt/atoms/`, `internal/prompt/loader.go` |
| **S11** Mangle skill | delivering Mangle knowledge | C-28 (the one place a skill's text and a Go filter already have to agree) | `TestPiggybackAtomMatchesTheFilter` | `internal/prompt/atoms/protocol/piggyback.yaml`, `internal/core/mangle_updates.go` |
| **S12** `context_budget` / `final_injectable` wired per turn | the pruning decision | C-11, C-12, C-13 | `TestEveryCompileAssertsAContextBudget`, `TestStageShardToolAllowedDerivesForALiveShard`, `TestCompilerReadsFinalInjectable`, `TestInjectableContextSlotOneIsSelectable` | `internal/core/defaults/policy/prompt_context.mg`, `internal/core/defaults/policy/stage_context.mg`, `internal/core/defaults/policy/prompt_northstar.mg`, `internal/prompt/compiler.go`, `internal/articulation/prompt_assembler.go`, `internal/session/executor.go` |
| **S13** CodeDOM facts reach the window; the tool-loop `WorkingSet` loads the full corpus | delivered knowledge | C-19, C-20, C-14, C-15 | `TestCodeDOMFactsReachTheWindow`, `TestStampedAtomsAreSearchableByTheSameEngine`, `TestPromptAtomsSchemaMatchesAcrossPackages` | `internal/core/virtual_store_codedom.go`, `internal/world/`, `internal/prompt/compiler.go`, `internal/prompt/loader.go`, `internal/store/local_core.go`, `internal/store/migrations.go`, `internal/prompt/vector_searcher.go` |
| **S14** the chat persona as a counted atom | prompt accounting | C-16 | `TestSystemPromptIsExactlyTheCompiledAtoms` | `cmd/nerd/chat/process.go`, `cmd/nerd/chat/process_knowledge.go`, `cmd/nerd/chat/process_dream_delegation.go`, `internal/prompt/compiler.go`, `internal/prompt/atoms/` |
| **S15** never silently truncate | elision policy | C-17 | `TestHistoryEvictionLeavesAMarker`, `TestEveryModelFacingTruncatorUsesClampMarker` | `internal/prompt/limits.go`, `internal/session/executor.go`, `cmd/nerd/chat/process_continuation.go`, `internal/core/shards/manager_spawn.go` |
| **S16** `ActivatedFacts` populated | the activation bridge | C-18 | `TestActivationScoresReachAtomSelection` (or deletion) | `internal/prompt/context.go`, `internal/context/activation.go`, `internal/prompt/selector.go` |
| *(no S-id)* | the September archetypes, already landed | C-21, C-22, C-23, C-24, C-25, C-26 | `TestObserverHandlerHonoursShouldCheckNow`, `TestEveryHydratorAssertsItsDeclaredShape`, `TestUnconfiguredSlotGetsTheVendorCeiling`, `TestEmptyCompletionIsRetriedWhenTokensWereBilled` | `internal/core/shards/manager_tools.go`, `internal/shards/system/executive.go`, `internal/core/virtual_store.go`, `internal/core/virtual_store_predicates.go`, `internal/shards/observer_manager.go`, `internal/northstar/observer.go`, `internal/store/reembed_all.go`, `cmd/nerd/chat/system_warnings.go`, `internal/perception/client_openai_compat.go` |
| *(no S-id)* | standing costs | C-30, C-31 | `BenchmarkCompilationScopeClone`, `TestMaxFactsInKernelBindsTheKernelCeiling`, `TestEveryValidatedCoreLimitHasAConsumer` | `internal/system/factory_adapters.go`, `internal/core/kernel_eval.go`, `internal/prompt/compiler.go`, `internal/core/limits.go`, `internal/core/kernel_init.go`, `internal/config/limits.go` |

## 4. Recommended order of landing

From the evidence in §2 only. Four orderings are forced by code, four contracts are
genuinely independent, and in four places the evidence disagrees with the program of record.

### Forced by the code

1. **C-27 before C-08 and C-11.** The moment `build_state` or `context_budget` gets a
   producer, the planner's prefix allowlist
   (`internal/shards/system/planner.go:1080-1086`: `build_`, `context_`) makes both
   **model-assertable** — `predicateAllowed` (`internal/core/mangle_updates.go:144-150`)
   hard-denies only the five `turn_*` predicates, and both candidates are declared, so
   `validatePredicateDeclaration` passes them. Today that is harmless because neither
   predicate does anything. Wiring them without narrowing the prefix list hands the model a
   veto over its own completion and over its own tool catalogue. C-27's test is cheap,
   corpus-driven and lands before any of the wiring.
2. **C-01 before C-02, C-04, C-05.** The chat layer guesses a status from prose
   (`process_continuation.go:172-184`) because prose is all that crosses
   `internal/session/task_executor.go:258-260`. The measurements already exist one frame
   earlier — `observedReturn` (`observed_return.go:39-105`) copies `Build`, `Tests`,
   `Changed`, `UntestedPaths`. Adding `TurnOutcome`/`ChangeStage` to `observation.Return` is a
   field; rewriting `injectShardResultFacts` without it is a second guess.
3. **C-09 before C-10.** The five numbers the policy stops on are asserted at
   `internal/context/working_set.go:179-182` from values produced by
   `internal/session/tool_budget_controller.go:332-334` — the same struct S3 intends to
   delete. Deleting the count ceiling first silences `working_stop`, `working_finalize`,
   `working_nudge`, `working_regime` and the `/repeated_cycle` + `/tool_failures` controls.
   Move the assertion site first, then delete.
4. **C-29 before the rest of S8.** "Slash commands ask the kernel" cannot be implemented for
   `/shards`, `/autopoiesis` and `/facts`, because they are in neither
   `command_categories.go:38` nor the `commands.go:54-266` switch — only in tests that pass
   against the unknown-command arm. The set-equality test is a prerequisite for knowing what
   the surface even is.

### Independent, cheap, and already testable

- **C-13** (`injectable_context` slot 1) — corpus-only, no Go change; the contract is already
  written at `prompt_northstar.mg:142-152` and merely untested.
- **C-31** (`max_facts_in_kernel`) — one test; the answer is either a call site or a deleted
  key.
- **C-16** (persona as an atom) — S14, already landing; no dependency on S1 or S12.
- **C-24**, **C-21**, **C-22**, **C-23**, **C-25** — landed; their tests exist and should be
  treated as the template for the ones still to write.

### Where the evidence disagrees with the program

1. **S4 is not "later" — C-08 belongs inside S1.** `turn_done` requires
   `!build_state(/failing)` (`coder_safety.mg:114`), and `build_state` has no producer; the
   Go struct field that would hold it names a function (`recordBuildState`) that is not in the
   main checkout (`internal/session/executor.go:216-219`). The fact that would supply it is
   exactly the mechanical build verdict S1's typed outcome carries
   (`observed_return.go:57-67`). Splitting these across two seams means S1 lands the evidence
   and S4 later discovers it still cannot use it.
2. **S3's "count ceilings deleted" is two changes, not one.** See ordering 3 above. The
   program states the destination; the evidence states that the assertion site must move
   first, and that `working_set.go:179-182` — not `tool_budget_controller.go` — is the line an
   architect has to relocate.
3. **S12 is wider than "wired per turn".** `context_budget_sufficient` /
   `context_budget_constrained` have a **second** consumer the study never cited:
   `internal/core/defaults/policy/stage_context.mg:285-300`, where they gate
   `stage_shard_tool_allowed/2`, the per-shard tool catalogue. Because `context_budget/2` has
   no producer, both branches are unsatisfiable and the tool-shaping layer is inert. Asserting
   the budget therefore does not only turn on pruning — it turns on a narrowing of every
   shard's tool list that nobody has evaluated. C-11 should land behind a measurement, not as
   a wiring task. (`03-knowledge-delivery-and-window.md`'s own Verification, refuted claim 5,
   flags the omission; the consequence is stated here.)
4. **S8's three "test-only" commands are worse than test-only.** They are neither registered
   nor dispatched; the tests assert against `Unknown command`. The program reads as though
   they need routing through the kernel; the evidence says they need to exist first, and that
   two *other* commands (`/explain`, `/explain-off`) are dispatched without being in the
   registry, so `/help` never lists them.

### What the in-flight seams already cover

S1 as scoped covers C-01 through C-05 and, if ordering 1 above is accepted, C-08. S3 covers
C-09 and C-10. S14 covers C-16. Nothing in flight covers C-27, which is the cheapest entry
in this document and the one that makes the others safe.

## 5. What the registry cannot pin

Five decisions where both sides have evidence and neither is wrong. A contract can state what
must hold *given* a choice; it cannot make the choice.

### One kernel or two engines

- **For two.** `internal/context/working_set.mg:1-2` states the reason in the file's first
  line: *"Loaded beside the canonical context_compilation.mg in a private evaluation scope;
  observations can never authorize an action."* A tool loop that asserts what it saw into the
  executive kernel would let a read become a permission. The JIT scope has the same rationale
  (`internal/system/factory_adapters.go:79-81`): selector facts never mutate the live kernel,
  and concurrent compiles cannot collide.
- **For one.** `working_stop`, `working_finalize`, `working_nudge` and `working_regime` — the
  only obligations in the system that force implement/verify/conclude — are invisible to
  `nerd query` and `nerd logic` against the main kernel, and `internal/core/defaults/policy/`
  contains no reference to them. `context_compilation.mg`'s fallback tier joins
  `activation.mg`'s `context_atom/1`, which the private engine never loads (C-20), so one
  rule means two different things depending on which engine evaluated it. And the JIT scope
  pays a full fixpoint over every stratum per compile (C-30).
- **The registry's position:** either answer is testable (C-20 for the subset, C-30 for the
  cost); which to pay is not derivable from the code.

### What an uncovered file owes, and for how long

- **Fail the turn.** `coder_safety.mg:104-107` — `hollow_success("new source was created
  without a test file")`, per turn, created source only.
- **Delegate.** `delegation.mg:91-93` — `delegate_task(/tester, "Generate tests for impacted
  code", /pending)`, any impacted file, blocks nothing.
- **Do nothing.** `impact.mg:16-22` — `unsafe_to_refactor/1` is consumed only by shadow mode;
  `block_refactor/2` has no consumer at all.
- **Nothing at all, most precisely.** `internal/session/build_verify.go:309-337` computes
  per-block coverage attributed to the turn's changed lines and leaves it as
  `observation.Return.Notes` (`observed_return.go:91-95`) — read by a parent, never a fact.
  `internal/core/defaults/tester.mg:109-124` declares `coverage_metric/2`,
  `coverage_goal/1`, `coverage_below_goal/1`, `needs_more_tests/1`, `coverage_warning/3` with
  no producer and no consumer.
- **The undecidable part** is whether coverage debt is a *turn* property or a *system*
  property. The vision says *"the agent cannot stop until the tests that actually harden the
  system exist"*, which is a system property; every mechanism above is a turn property.
  `02-forcing-and-completion.md` §6.5 proposes standing pressure that outlives the turn; that
  design leans on at least twelve predicates that exist in no file but that document (its own
  Verification, refuted claim 7). The registry can pin whichever is chosen. It cannot choose.

### Whether `turn_done` should be reachable without a host verifier

`coder_safety.mg:112-113` is a deliberate stance: *"Execution is weaker than completion. Only
the host verifier emits acceptance for an immutable caller contract with current, executed
behavioral witnesses."* Making `turn_done` derivable from a chat turn (C-07) means either
weakening that sentence or building a host verifier for chat turns. The evidence for keeping
it strict is `internal/core/mangle_updates.go:144-150`, which refuses to let a model assert
any of the five `turn_*` predicates even under a permissive policy — the system has already
decided completion is not self-reportable. The evidence for changing it is that, as shipped,
no ordinary path can ever be done, so the predicate the north star is written in terms of is
decorative.

### How far to narrow the planner's prefix allowlist

C-27's test will show `build_` and `context_` reaching verdict and budget rules. The obvious
fix — delete the prefixes — is not obviously safe: the campaign path relies on the planner
asserting `campaign_*`, `phase_*` and `task_*` facts that the kernel then reasons over
(`internal/shards/system/planner.go:1071-1091`, `MaxUpdates: 200`). Narrowing to an explicit
predicate list is a per-predicate audit of the campaign corpus, and `M1-mangle-corpus-usage.md`
§6 (unverified) reports that some campaign predicates have no other producer. The registry can
state the invariant; the size of the allowlist is an architect's call about how much the
planner is trusted to say.

### Whether `ActivatedFacts` and the persona are wired or deleted

Both are cases where the repo's NO-SHIMS rule points at deletion and the design intent points
at wiring. `internal/prompt/context.go:226-254` names the two blockers itself: there is no
fact→atom relation in the corpus schema, and the field is in the prompt cache key, so live
scores turn the cache off. `cmd/nerd/chat/process.go:832` is the mirror image: a string that
*is* in the window and is not in the accounting. The registry can pin either outcome — a
wiring test or a compile failure on a removed field — but "delivered knowledge is a decision"
and "every atom is counted" are the same sentence pointing in opposite directions for these
two, and which one the architect means decides both.
