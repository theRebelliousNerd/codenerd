# REVIEW — Wave 1 adversarial review (S1, S4, S14, S15)

## Status

- Reviewer: adversarial reviewer, wave 1. Read-and-test only; no seam code was changed.
- Range reviewed: `a287131f..7a86da2f` (19 commits, 74 files, ~6.5k lines).
- **The branch moved during the review.** `dogfood/c2-closure` advanced past `7a86da2f` to
  `49a12113` (S3, the tool-count nuke: `340dfd31`, `cb6dab78`; plus the S12/S19 studies) while
  this review was in progress. Every diff and line number below is pinned to `7a86da2f`, not to
  the branch tip. One consequence worth flagging: `internal/session/executor_tools.go` was
  rewritten by S3 after S15 and S4 both edited it, so the wave-1 line numbers in those logs no
  longer resolve on the tip.
- 8 findings: **6 confirmed, 2 unconfirmed.** 3 dismissed after checking.
- `go build ./...` clean; `go vet` clean on every package touched.

## Scope

| Seam | Claim under review |
|---|---|
| S1 | Verdict from evidence: typed outcome across the shard boundary; `injectShardResultFacts` no longer guesses from prose; `build_state`/`test_state` asserted; recovered tool errors. |
| S4 | `turn_done` derives from evidence via `turn_verified`; the outcome is read once from the kernel and is binding; the model cannot assert `build_state`/`test_state`/`turn_*`; the mandatory piggyback atom that taught the model to write `test_state` was fixed. |
| S14 | The architect persona is delivered from one seam at offset 0 with a byte-for-byte golden. |
| S15 | No silent truncation: twelve cut sites carry the `[codenerd: truncated` marker, moved to `internal/types/elide.go`; `tactile` output truncation is visible to the model. |

Paths each seam must hold on: chat free-text, slash `/fix`, `nerd fix`, planned steps, campaign
task, observer.

## Findings

| # | Sev | Status | File:line | Scenario |
|---|---|---|---|---|
| F1 | High | CONFIRMED | `internal/observation/subagent.go:500-507`, `:968` | `ProjectReturn` sets `Status` to `completed` whenever `Failure == ""` and the output is non-empty. It ignores the `Return.Outcome` S1 added 370 lines above it, and `ReturnResult` has no field for it. On the delegate/observer path (`internal/core/virtual_store_actions.go:879` → `encodeDelegationReturn` → `ReturnResult.Text`) a subagent turn the kernel recorded `/unverified`, `/hollow` or `/failed` is announced to the parent model as `coder returned completed`, and asserted into the kernel as `delegation_result/2` carrying that word. This is contract **C-02**, whose invariant test the registry lists as TO WRITE and which S1 did not write. |
| F2 | High | CONFIRMED | `internal/core/defaults/policy/coder_safety.mg:155` | `turn_verified` arm 2 reads `test_state(/passing)` as "the gate this turn ran". `test_state/1` is a session-global with three other producers, none per-turn: `virtual_store_actions.go:460` (`handleRunTests` — the `run_tests` tool **the model invokes**, injected at `virtual_store_routing.go:176`), `virtual_store_workflows.go:70` (`/log_read`), and `tdd_loop.go:254`/`:958` (the TDD state machine's own vocabulary). Only `recordBuildState`'s own fact is tracked in `perTurnBuildStateFacts` and retracted. Two live holes: **(a)** a `test_state(/passing)` left by a `run_tests` call in turn N verifies a later write turn whose own `TestCheck` was skipped (`build_verify.go:406` returns `VerifySkipped` for tag-gated packages, and `recordBuildState` deliberately asserts nothing for a skipped gate) — stale evidence surviving a source change as current truth; **(b)** the arm is positive where `turn_executed` is negative (`!build_state(/failing)`), so a coexisting `test_state(/failing)` does not block verification. |
| F3 | High | CONFIRMED | `internal/core/defaults/prompt_corpus.db` vs `internal/prompt/atoms/` | S4's Design F item 3 (delete `test_state(/passing).` from the mandatory piggyback atom) is inert, and the reason is much larger than the edit. Atom YAML is not a runtime source: `cmd/tools/prompt_builder` compiles it into the `go:embed`ed `prompt_corpus.db`, which **wins every duplicate atom ID** in the compiler's merge, and there is no `go:generate`. Measured: the shipped corpus holds **878** atoms, the source tree **914**; **42 atoms in the tree are absent from the corpus entirely** — including `safety/honesty/no_unverified_claims`, `system/change_evidence`, `methodology/tdd/coverage_mandate`, `identity/base/core`, `methodology/working_context`, all five `northstar/guardian/*` — and **2 disagree on `is_mandatory`**, one of them being `protocol/piggyback/mangle_updates` itself (source `true`, corpus `false`, and the corpus revision is 707 bytes against the source's current text). So S4's premise — "the **mandatory** prompt atom ... taught the model to write it" — is not true of the corpus the runtime serves, and neither the `is_mandatory: true` fix from an earlier wave nor this one has ever reached a compiled prompt. Every atom edit in this repo is currently a no-op. |
| F8 | Medium-High | CONFIRMED | `cmd/nerd/chat/process_continuation.go:236-252` vs `internal/core/defaults/policy/codedom_continuation.mg:7-26` | S1 says "the `shard_result/5` shape does not change, so the Decl and every rule stand unedited". The shape did not change; the **vocabulary** did. `shardResultStatus` now writes `/unverified`, which no rule joins, and a verified turn is written `/complete`. `pendingTestOwed` is evidence-driven and independent of the status, so a `/done` coder turn that wrote production Go with nothing beside it asserts `pending_test` — and `has_pending_subtask` joins that obligation against `shard_result(_, /code_generated, /coder, _, _)`, which a `/complete` turn is not. The obligation is asserted into the kernel with no rule that can reach it. Before S1 that same turn *did* derive a tester step (the substring heuristic made it `/code_generated`). `/tests_needed` and `/review_needed` remain joined by rules no producer writes. This is contract **C-03**. |
| F4 | Medium | CONFIRMED | `internal/core/mangle_updates.go:168`; `internal/shards/registration.go:140`, `:327` | `predicateAllowed` now hard-blocks `build_state`, `test_state` and `turn_created_source`. S4 deleted the now-dead `ModelObservationPolicy` entry for `test_state` on the stated ground that "leaving a dead entry behind a hard block would be two truths coexisting" — and left the same shape one file away: `registration.go:140` still owns `"turn_evidence", "turn_acceptance", "turn_created_source", "build_state"` under a comment justifying them by the `turn_done` derivation, and `:327` still lists `"test_state"` in the executive fact set. Ownership routing is a different surface from an update allowlist, so this is a decision to take rather than an automatic deletion — but it is the same class the seam named. |
| F5 | Low | CONFIRMED | `internal/session/executor.go:1113-1117`; claim at `internal/session/executor_tools.go:1957-1960` | The comment says `captureTurnOutcome` "runs on EVERY path". It does not: `checkHollowSuccess`, which calls it, is guarded by `if result.Error == nil`, and `surfaceToolErrors` at `:1103` — the function S1 rewrote — is what sets `result.Error`. On an unrecovered tool error the kernel is never asked and `resolveTurnOutcome`'s error classification decides. Behaviour is right (`/failed` either way); the "one read, binding" claim has a documented universal that is false, and a reader who trusts it will look for the verdict in the wrong place. |
| F6 | Low | UNCONFIRMED | `internal/session/change_evidence.go:100-104` | `turnRecoveredFromToolErrors` treats *any* passed gate as recovery for *any* tool error. Constructed the four cases the brief names — build failed then passed (recovered ✓), tests failed no rerun (not recovered ✓), gates skipped + empty response (not recovered ✓), gates skipped + non-empty response (recovered ✓) — all match the stated rule. The fifth case is the gap: a failed `run_tests` tool call, `BuildCheck` passed, `TestCheck` **skipped** is reported recovered on the strength of a build that says nothing about tests. Consistent with the seam's written rule, so possibly intended; would be confirmed by an architect's call that a build cannot answer for a test. |
| F7 | Low | UNCONFIRMED | `internal/projectdoc/facts.go:326`, `:336` | Hand-rolled copies of the marker string rather than `types.DroppedNotice`/`ClampInline`. S15 inventoried projectdoc as "already compliant, left alone", and the prefix does match so an audit finds it — but it is a second spelling of a convention whose whole argument is that there should be one. |

### Dismissed after checking

- **S14 `.gitattributes` under `core.autocrlf=true`.** This worktree *is* a fresh checkout with
  autocrlf on (git warned "LF will be replaced by CRLF" when staging this very file).
  `git check-attr text eol` reports `eol: lf` for both `cmd/nerd/chat/persona.go` and
  `cmd/nerd/chat/testdata/architect_persona.golden`, and both contain **0 CR bytes** on disk
  (golden: 6,461 bytes, 147 LF). The golden gate survives a fresh clone. Not a defect.
- **S15 marker move, leftover callers / import cycle.** `internal/prompt/limits.go` is deleted,
  not forwarded; `grep` for `prompt.ClampText|ClampInline|IsClamped|TruncationMarker|DroppedNotice`
  returns nothing; the 12 call sites all read `types.*`. `internal/types` imports no repo package,
  so no cycle is possible. Not a defect.
- **S15 `tactile` reaching the model.** Every `internal/core` consumer of a
  `tactile.ExecutionResult` goes through `Output()` or the new `MarkTruncated`
  (`virtual_store_actions.go:105,108,208,216,219,221,254,441,492,558`), and
  `internal/tools/shell/execute.go` has no byte ceiling of its own, so there is no second
  unmarked cut on the shell path. Not a defect.
- **Mangle Decl / stratification of the new `coder_safety.mg` block.** Every negation is over a
  bound single-argument projection (`!turn_wrote(Verb)`, `!turn_verified(Verb)`) or a ground
  literal (`!build_state(/passing)`, `!test_state(/passing)`); `has_turn_acceptance/1` is the
  correct projection given the fork's wildcard-negation trap; the chain is acyclic. Verified by
  booting `core.NewRealKernel()` with the shipped corpus in three new tests and querying the new
  predicates with facts asserted in production's shapes — including the Go-string `"/passing"`
  that `handleRunTests` uses rather than a `types.MangleAtom`, which the engine does accept.

## Tests added

All four files are new; none modifies an existing test. Each fails on `7a86da2f`.

| Test | Package | Pins | Fails with |
|---|---|---|---|
| `TestProjectReturnStatusAgreesWithTurnOutcome` | `internal/observation` | F1 / **C-02** (named by the registry, written here) | `a /unverified turn is projected Status="completed"`; `the parent is told "coder returned completed for \"add the retry\""` — same for `/hollow` and `/failed` |
| `TestProjectReturn_AbsentVerdictKeepsTheOldReading` | `internal/observation` | the converse — a producer with no verdict is not newly demoted | passes (guard) |
| `TestProjectReturn_FailureStillOutranksTheVerdict` | `internal/observation` | a failure outranks any atom | passes (guard) |
| `TestTurnVerified_DoesNotReuseAnEarlierTurnsTestState` | `internal/session` | F2(a), through `assertTurnEvidence`/`captureTurnOutcome` on a real kernel | `turn_verified = 1: this turn ran no test gate`; `TurnOutcome = /done on a turn whose tests were never run` |
| `TestTurnVerified_ARedTestStateBlocksVerification` | `internal/session` | F2(b) | `TurnOutcome = /done with test_state(/failing) live in the kernel` |
| `TestShippedPromptCorpus_AgreesWithTheAtomSourceOnMandatory` | `internal/prompt` | F3 | `the shipped prompt corpus (878 atoms) disagrees with internal/prompt/atoms (914 atoms) on 44 atom(s)` — full list in the failure |
| `TestShippedPromptCorpus_ProtocolAtomsTeachNoHardBlockedPredicate` | `internal/prompt` | the regression net for S4's atom edit; probes the hard block through `core.FilterMangleUpdates` rather than restating it | passes (guard) |
| `TestPendingTestOnAVerifiedTurnStillReachesTheTester` | `cmd/nerd/chat` | F8, through the real kernel and the shipped corpus | `pending_test was asserted and no subtask derives from it` |
| `TestShardResultStatusVocabularyIsClosed` | `cmd/nerd/chat` | **C-03** (named by the registry, written here) | `shardResultStatus writes [/unverified], which no rule joins`; `codedom_continuation.mg joins [/tests_needed /review_needed], which no producer writes` |

No fix is included: F1, F2, F3 and F8 are all more than one line and belong to whoever owns the
next pass.

## Verdict per seam

**S1 — verdict from evidence: partially landed.** The derivation inside
`injectShardResultFacts` is correct and the substring heuristics are genuinely gone; the typed
outcome does reach `observation.Return`, `types.ShardResult`, the chat continuation, `nerd spawn`
and `nerd fix`. It does not reach the two consumers that matter most on the paths the implementer
did not test: the subagent projection every delegation and observer run reads (F1) and the
continuation rules the obligation has to pass through (F8). Of the four invariant tests the
registry assigns S1, C-01/C-04/C-05 are covered in substance under different names; **C-02 and
C-03 were not written and both fail.** The campaign path is untouched, as S4's Open #4 says.

**S4 — turn_done binding: landed, on evidence that is not yet this turn's.** The policy block is
well-formed and stratifies; the binding read is genuinely single and genuinely happens before
retraction; the acceptance guard in front of the kernel read is gone; `resolveTurnOutcome`'s
re-derivation is deleted rather than kept. The hard block is real and correctly ordered ahead of
the caller allowlist. But `turn_verified` is only as good as `test_state`, and `test_state` is a
shared global with a second vocabulary and no per-turn scope (F2) — which means the seam can, on
an ordinary session, hand out `/done` for a turn whose tests were last run before the edit
existed. And the third of its three surfaces (the atom) never shipped (F3).

**S14 — persona: landed.** One delivery seam, offset 0, byte-for-byte golden, structural test
against a second delivery path, eol pinned and verified on a fresh autocrlf checkout. The study's
central finding (`final_system_prompt` has no producer, so the main chat turn compiles no JIT
prompt at all) is the largest single fact in this wave and is correctly recorded as out of scope.
No defects found.

**S15 — no silent truncation: landed.** The move is clean, the old file is deleted with all 12
callers repointed, there is no cycle, the marker reaches the model-facing projection on the shell
path, and the invariant test is a real net rather than a per-site assertion. The two pre-existing
tests it changed are honest contract changes. Only F7 (a second spelling in `projectdoc`) is
outstanding, and it is cosmetic.

## Open

1. **`test_state/1` needs a producer contract, not a patch.** Options, in the order they preserve
   the seam's own argument: give the predicate a turn identity (`test_state(Turn, State)`) so the
   corpus can join this turn's; or have `recordBuildState` retract every `test_state` before
   asserting, which makes the turn's gate the sole authority and breaks the TDD loop's use of the
   same name; or split the TDD loop's state machine onto its own predicate (`tdd_state/1`), which
   is what it actually is. The third is the only one that leaves two systems with two names.
   Whichever is chosen, `turn_verified` should also gain `!test_state(/failing)` so the arm is
   symmetric with `turn_executed`'s guard on `build_state`.
2. **`prompt_corpus.db` is 42 atoms behind the source tree** and there is no `go generate`, no CI
   check and no test that compares them (until this review's). Regenerating is one command with a
   Gemini key; the standing fix is a gate, because the failure mode is silent and has already
   swallowed at least two waves of edits. `TestShippedPromptCorpus_AgreesWithTheAtomSourceOnMandatory`
   is the floor — it compares only `is_mandatory` and atom presence, because everything else is
   transformed on the way in. The ceiling is a content-hash comparison, which needs the builder's
   transformation to be exposed as a function.
3. **C-02 and C-03 now have failing tests and no fix.** Both are single-consumer changes:
   `ProjectReturn` must read `Return.Outcome`, and `codedom_continuation.mg` must join the status
   set the producer can actually write.
4. **The campaign path still returns a string.** `Orchestrator.spawnTask`
   (`internal/campaign/orchestrator_task_handlers.go:28`) calls `TaskExecutor.Execute`, so no
   campaign task can see `TurnOutcome`. The chat's own observer and consultation adapters
   (`cmd/nerd/chat/session_boot_helpers.go:37`, `:53`) do the same. Threading
   `observation.Return` through them is the same shape S1 used for the chat.
5. **S4 Open #2 (`context_budget/2` reachable through the planner's `context_` prefix) is still
   open**, and is the only producer of the fact gating `final_injectable` and
   `stage_shard_tool_allowed` — the model deciding its own tool catalogue.
6. **S4 Open #3 (`TestModelAssertablePredicatesDoNotFeedTheVerdict`) is still TO WRITE.** The
   list-based test that landed is the floor; the general one would have found `context_budget`
   and would fail whenever either side moves.
7. **F5's second-order effect is worth a look by whoever owns the turn close.** Because
   `checkHollowSuccess` is skipped when `result.Error != nil`, its deferred
   `cleanupPerTurnCoverageFacts` is skipped too. `assertTurnEvidence` is skipped on that path as
   well so nothing it asserts leaks, but `recordGoFileCreations` populates
   `perTurnCreatedSourceFacts` from inside the tool loop, and those are not retracted on an
   errored turn. Pre-existing, not introduced by this wave.
8. **The registry's own accounting is now stale.** 36 of the 55 test names it cites do not exist
   as functions; most are legitimately marked TO WRITE, but several (C-01, C-04, C-05, C-08) were
   satisfied in substance by tests under different names, so the registry cannot be read as a
   checklist. Reconciling names to landed tests is cheap and would make the registry usable as the
   gate it was written to be.
