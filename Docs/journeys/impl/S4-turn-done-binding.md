# S4 — `turn_done` is reachable from evidence, and it is binding

Seam S4 of the codeNERD unblocking program. Serves `cap_forced_completion` /
`req_obligation_fixpoint`. Closes the first question the forcing-and-completion study
(`Docs/journeys/02-forcing-and-completion.md`, rows 17, 33, 34, 69, 72) put to the architect:
**on an ordinary turn, nothing can ever be done.**

## Status

- **last updated**: 2026-09-18
- **done**:
  - worktree fast-forwarded from `0b5c69d0` to `dogfood/c2-closure` @ `9443b8a0` (S1 `f617ceba`,
    S14 `e60ee53a`, contract registry `9443b8a0` present)
  - trace: the acceptance-only path confirmed, and a second hole found that the brief did not
    name (`ModelObservationPolicy` allow-lists `test_state` by name)
  - policy: `turn_verified` / `turn_unverified` / `turn_missing_evidence` derived from the
    mechanical gates; `turn_done :- turn_executed, turn_verified`
  - binding: one kernel read (`consumeTurnDoneSignal`) feeds `captureTurnOutcome`;
    `resolveTurnOutcome` and `TurnRecord.Verified()` consume it instead of re-deriving
  - the evidence sentence is a function of the derived state
  - the model can no longer assert `build_state` / `test_state` / `turn_verified`
  - fail-before evidence captured for every new test; pass-after green
  - `go build ./...`, `go vet`, targeted runs, full `go test ./...`
- **open**: seven items below; one of them (the prompt-corpus re-embed) should be done before
  the next live session

## Trace

### What `turn_done` needed before this seam

`internal/core/defaults/policy/coder_safety.mg:114-115`:

```
turn_executed(Verb) :- turn_evidence(Verb, _, _, _, _, _), !has_hollow_success(), !build_state(/failing).
turn_done(Verb)     :- turn_executed(Verb), turn_acceptance(Verb, _, _).
```

`turn_acceptance` is asserted in exactly one place — `assertTurnEvidence`
(`internal/session/executor.go:2333`) — and only when `result.Acceptance.Status == "verified"`.
`result.Acceptance` is non-nil only when `result.acceptanceTransaction` is, which comes from
`evidence.ContractFromContext` ← `evidence.WithContract`, whose single production call site is
`cmd/nerd/cmd_direct_actions.go:313`, behind the `--acceptance` flag, registered on `nerd fix`
alone (`cmd/nerd/main.go:200`).

So: **every chat turn, every campaign task, every observer run, and every `nerd fix` without
`--acceptance` could not reach `turn_done` at all.** `turn_executed` was as far as the corpus
could go, and `resolveTurnOutcome` recorded `/unverified` in `turn_cost` forever. The
completion signal the whole forcing story rests on was unreachable on the ordinary path.

### What consumed the unreachable signal

- `consumeTurnDoneSignal` (`executor.go:2606-2615`) — queried `turn_done`, counted the rows, and
  wrote two `Debug` lines. It changed nothing. The "single completion signal" had no consumer.
- `captureTurnOutcome` (`executor_memory.go:128-146`) — queried `turn_done`, but **only inside
  `if result.Acceptance != nil && result.Acceptance.Status == "verified"`**. A Go guard in front
  of the kernel read, re-stating the acceptance conjunct the rule already had. Without a
  contract the query was never issued.
- `resolveTurnOutcome` (`executor_memory.go:149-171`) — the same acceptance-gated re-derivation
  a second time, as a fallback.
- `TurnRecord.Verified()` (`executor_learning.go:60-74`) — `Outcome == /done`, so the prompt
  learner could never see a verified turn and trained on nothing.
- `appendEvidenceReport` (`change_evidence.go:109`) — printed the constant sentence
  `"Requested behavior remains unverified (no acceptance contract)."` on every write turn,
  regardless of what the gates had just measured. It was right by accident: nothing *was*
  verified. It would have stayed on the page after this seam made it false.

### The second hole — the model could have manufactured its own verification

The coordinator flagged the planner's Piggyback prefix allowlist
(`internal/shards/system/planner.go:1080-1090`), which contains `"build_"` and `"context_"`. That
is real: with S1's `recordBuildState` giving `build_state/1` a live producer and this seam making
it decide the verdict, `build_state(/passing).` in a `mangle_updates` entry would be a model
writing its own evidence.

Checking it turned up a wider one. The hard block in `predicateAllowed`
(`internal/core/mangle_updates.go:145-149`) refuses `turn_acceptance`, `turn_evidence`,
`turn_executed`, `turn_done`, `turn_cost` — but `ModelObservationPolicy()`
(`internal/core/mangle_updates.go:37`), the allowlist shared by **the session executor
(`executor.go:1697`) and the chat turn (`cmd/nerd/chat/process.go:1035`)**, lists

```go
"test_state":        {},
```

by name, and the mandatory prompt atom `protocol/piggyback/mangle_updates`
(`internal/prompt/atoms/protocol/piggyback.yaml:97`) **taught the model to write it**:

```
test_state(/passing).    # /passing | /failing | /unknown
```

So the hole was not a prefix an attacker had to find — it was in the protocol the model is
handed on every single turn, marked mandatory. Before S1 it was inert (nothing read `test_state`
as turn evidence). After S1 and S4 it is the difference between a verified turn and an
unverified one. Three surfaces, one fix: the hard block, the allowlist, and the atom.

## Design

### A. Verification is derived, and it is separate from execution

Three predicates, not one, because the questions are different:

| predicate | question |
|---|---|
| `turn_executed(Verb)` | did the turn really act, and is nothing hollow or red? |
| `turn_verified(Verb)` | is the mechanical evidence this intent *requires* affirmative? |
| `turn_done(Verb)` | both |

`turn_verified` deliberately does **not** carry `turn_executed` in its body. Execution and
verification are independent questions about the same turn; conjoining them is `turn_done`'s
job, and the brief states that rule exactly:

```
turn_done(Verb) :- turn_executed(Verb), turn_verified(Verb).
```

Three arms verify:

```
turn_verified(Verb) :- turn_evidence(Verb, _, _, _, _, _), !turn_wrote(Verb).
turn_verified(Verb) :- turn_evidence(Verb, _, _, _, _, _), turn_wrote(Verb), build_state(/passing), test_state(/passing).
turn_verified(Verb) :- turn_evidence(Verb, _, _, _, _, _), has_turn_acceptance(Verb).
```

- **No write → what `turn_executed` already requires.** A `/explain` turn changed nothing, so
  there is no workspace claim to verify. This is what `executor_tools.go:1948-1952` already
  wanted: read-only turns were recorded `/unverified` forever and "the denominator could not
  count read-only work as verified at all".
- **Wrote → both gates green.** Not "build green, and tests if they were owed" — the corpus
  cannot tell a markdown write from a Go write. `turn_created_source/1` covers files *created*
  this turn; nothing in the fact space records a *modified* Go file, and `modified/1` is
  model-asserted, which is a claim and not evidence. Rather than invent a distinction the facts
  cannot support, a turn that ran a write-mutation tool owes both gates. That is conservative in
  the direction `coder_safety.mg` already argues for ("cautious rather than falsely permissive,
  and cautious is the correct direction to be wrong in"), and the acceptance arm remains
  available for anything else.
- **Acceptance stays sufficient.** Unchanged and untouched by the other two.

`turn_wrote` has two arms so dream mode cannot slip past:

```
turn_wrote(Verb) :- has_turn_write(Verb).
turn_wrote(Verb) :- turn_evidence(Verb, _, _, _, _, _), write_oriented_intent(Verb).
```

The hollow rules are all guarded by `DreamMode = /false`, so a dream-mode `/create` that wrote
nothing derives no `hollow_success` and reaches `turn_executed`. With only the first arm it
would then be a "non-write turn" and verify for free. The second arm holds any write-oriented
verb to the gates whether or not a write landed.

### B. The corpus names what is missing, not Go

```
turn_unverified(Verb)          :- turn_executed(Verb), !turn_verified(Verb).
turn_missing_evidence(Verb, /build_not_green) :- turn_unverified(Verb), !build_state(/passing).
turn_missing_evidence(Verb, /tests_not_green) :- turn_unverified(Verb), !test_state(/passing).
turn_build_failed(Verb)        :- turn_evidence(Verb, _, _, _, _, _), build_state(/failing).
```

Go maps each atom to a sentence; it does not decide which atoms hold. `turn_unverified` never
derives for a no-write turn, so no spurious reason is ever produced.

`has_turn_acceptance(Verb) :- turn_acceptance(Verb, _, _).` exists because of the repo's
documented engine trap: **a negated multi-argument literal with wildcards does not exclude on
this fork.** `!turn_acceptance(Verb, _, _)` would have silently failed to exclude anything. The
projection to a single-argument predicate is the same shape `has_hollow_success()` already uses
ten lines above.

**Stratification** (checked by loading the full shipped corpus through `core.NewRealKernel()`):
`turn_evidence`/`build_state`/`test_state` are EDB → `has_turn_write`, `hollow_success` →
`has_hollow_success`, `turn_wrote`, `has_turn_acceptance` → `turn_executed`, `turn_verified` →
`turn_unverified` → `turn_missing_evidence`, `turn_done`. Acyclic; every negation crosses
strictly downward.

### C. One read, and it binds

`consumeTurnDoneSignal` stops being a logger and becomes **the** kernel read: it returns a
`turnVerdict{Done, Unverified, BuildFailed, Missing}` and `captureTurnOutcome` switches on it.
The acceptance guard in front of the query is deleted — that guard was the reason the read never
happened.

```
hollowErr != nil          -> /hollow
result.Error != nil       -> /failed
verdict.BuildFailed       -> /failed        (derived, not a Go build check)
verdict.Done              -> /done
otherwise                 -> /unverified
```

`/hollow` survives as its own atom rather than collapsing into `/failed`: it is load-bearing
downstream (`TurnRecord.Failed()` counts it as a failure, and the chat's `injectShardResultFacts`
routes `/hollow` back to the same shard as `/incomplete`, which `/failed` does not do).

`resolveTurnOutcome`'s kernel re-query is **deleted**, not kept as a fallback. It ran after the
per-turn facts were retracted, so it was asking a kernel that had already forgotten the turn; and
a second derivation path is exactly the "two truths coexist" the no-shims rule forbids. It now
returns `result.TurnOutcome` and, only when the turn never reached the verdict (an error thrown
before `checkHollowSuccess`), classifies that error.

### D. The evidence sentence is a function of the derived state

`appendEvidenceReport` did two unrelated things in one pass: it *verified and persisted*
acceptance (which must happen before the hollow gate, because it can set `result.Error`), and it
*wrote the closing sentence* (which must happen after the verdict, or it has nothing to report).
That is why it printed a constant. It is split, and both callers repointed in the same change:

- `closeAcceptanceEvidence` — verify, persist, set `result.Error`. Runs where the old function ran.
- `appendEvidenceSummary` — the sentence. Runs after `checkHollowSuccess`, so `result.TurnOutcome`
  and `result.MissingEvidence` are set.

The sentence now has three forms instead of one constant:

| state | sentence |
|---|---|
| acceptance report present | `Report.Summary()` — status, snapshot and every obligation it ran |
| `/done` without a contract | `Verified by evidence: the build and the tests were both measured green after the final edit.` |
| `/unverified` | `Unverified: <the build was not verified green / the tests were not verified green>.` |
| `/hollow`, `/failed` | the corresponding sentence |

`verdictSentence` deliberately has **no** acceptance branch. A turn with a report returns
before reaching it, so such a branch would be unreachable — and an unreachable second spelling
of "verified by contract" is exactly how two answers to one question drift apart. (One was
written, found unreachable on review, and deleted.)

### E. `nerd fix` reads the same verdict

`Cortex.SpawnTaskObservedWithTarget` returns `observation.Return` (the carrier S1 already built),
so the direct-action path sees the *same* `result.TurnOutcome` the chat, the campaign and the
learner see, rather than re-reading prose. `nerd fix` prints the outcome and exits non-zero on a
`/failed` outcome — `/unverified` is not an error, which is the whole point of having a third
state.

### F. The model cannot assert evidence about the workspace

Three edits, one change:

1. `predicateAllowed`'s hard block gains `build_state`, `test_state`, `turn_verified`,
   `turn_unverified`, `turn_missing_evidence`, `turn_wrote`, `turn_build_failed`,
   `has_turn_acceptance`. The block sits *before* the policy allowlist, so no caller policy —
   including the planner's `build_` prefix — can delegate that authority.
2. `ModelObservationPolicy()` loses its `"test_state"` entry. Leaving a dead entry behind a hard
   block would be two truths coexisting.
3. `protocol/piggyback/mangle_updates` loses the `test_state(/passing).` line it taught on every
   turn. The atom's own comment says "Add a predicate here and there together" — so they go
   together.

## Changes

Three commits on `worktree-agent-a7ff0d3dc5dfdc0b9`, each built standalone (see below):

| commit | subject |
|---|---|
| `fc2db654` | `feat(policy): a turn is verified by the gates the host ran, not only by a contract` |
| `c15b76d4` | `fix(core): the model cannot report the state of the build or the test suite` |
| `37e8b0bd` | `feat(session): the turn's outcome is the kernel's verdict, read once and binding` |

| file | change |
|---|---|
| `internal/core/defaults/policy/coder_safety.mg` | `turn_verified`, `turn_unverified`, `turn_wrote`, `turn_build_failed`, `turn_missing_evidence`, `has_turn_acceptance` + Decls; `turn_done :- turn_executed, turn_verified`; the 112-113 comment now says why the gates ARE the host verifying |
| `internal/core/mangle_updates.go` | the hard block gains the whole verdict block plus `build_state`/`test_state`; `ModelObservationPolicy` loses its `test_state` entry |
| `internal/prompt/atoms/protocol/piggyback.yaml` | `test_state(/passing).` deleted from the mandatory protocol; a paragraph says the runtime records those itself |
| `internal/session/executor.go` | `consumeTurnDoneSignal` returns `turnVerdict` (the one kernel read); `missingEvidenceSentence`/`missingEvidenceClause`; `ExecutionResult.MissingEvidence`; the close sequence calls `closeAcceptanceEvidence` then `appendEvidenceSummary` |
| `internal/session/executor_memory.go` | `captureTurnOutcome` consumes the verdict, acceptance guard deleted; `resolveTurnOutcome`'s re-derivation deleted |
| `internal/session/executor_tools.go` | the late Debug-only `consumeTurnDoneSignal(verb)` call removed — the read moved into `captureTurnOutcome`, which runs on every path |
| `internal/session/change_evidence.go` | `appendEvidenceReport` split into `closeAcceptanceEvidence` + `appendEvidenceSummary`; `verdictSentence` |
| `internal/system/factory.go` | `Cortex.SpawnTaskObservedWithTarget` |
| `cmd/nerd/cmd_direct_actions.go` | the direct verbs use the observed spawn, print the outcome, and exit non-zero on `/failed` |

No shim: `appendEvidenceReport` is gone, not forwarded, and its two callers (production
and `change_evidence_test.go`) were repointed in the same change. The re-derivation in
`resolveTurnOutcome` is deleted rather than kept as a fallback.

## Tests

New: `internal/session/turn_done_binding_test.go` (7 tests / 10 cases),
`internal/core/turn_evidence_authority_test.go` (4 tests),
`internal/core/turn_verification_corpus_test.go` (the corpus load + derive gate).
Modified to the new contract: `internal/session/executor_memory_test.go`
(`TestExecutorTurnCost_ReadOnlyTurnIsUnverifiedNotFailed` →
`...ReadOnlyTurnIsDoneNotFailed`), `internal/session/change_evidence_test.go` (repointed to
the split functions).

Every test drives a real `core.NewRealKernel()` with the shipped corpus, through the
production entry points `assertTurnEvidence` / `captureTurnOutcome` / `FilterMangleUpdates`.
Nothing is stubbed.

**Method (no stash — the stack is shared with the main checkout and other worktrees).**
Implementation committed to a temporary WIP commit (`f9478d60`), then
`git checkout dogfood/c2-closure -- coder_safety.mg mangle_updates.go executor_memory.go`,
run, `git checkout HEAD -- <paths>`, `git reset --soft HEAD~1`. `verdictSentence` was
hand-reverted to its pre-change constant in place, keeping the signature, so the failure is
behavioural rather than a compile error.

### Fail-before (pre-change policy + pre-change binding restored)

```
--- FAIL: TestTurnDone_DerivesFromEvidenceWithoutAcceptance
    green build + green tests must derive turn_verified, got 0 facts
--- FAIL: TestTurnDone_ReadOnlyTurnIsDoneWithoutGates
    a read-only turn that ran must derive turn_done, got 0 facts
--- FAIL: TestTurnDone_BuildFailingExcludes
    a red build must derive turn_build_failed, got 0 facts
--- FAIL: TestTurnOutcome_UnverifiedNamesMissingEvidence
    expected exactly one turn_unverified, got 0 facts
--- FAIL: TestTurnOutcome_UnverifiedNamesBothGatesWhenNeitherRan
    both gates must be named missing, got []
--- FAIL: TestLearningTrainsOnlyOnDerivedDone/greenGatesTrainAsAWin
    resolveTurnOutcome = "/unverified", want "/done" after cleanup
--- FAIL: TestLearningTrainsOnlyOnDerivedDone/redBuildIsNotAWin
    resolveTurnOutcome = "/unverified", want "/failed" after cleanup

--- FAIL: TestModelCannotAssertTurnDone
    a model must not be able to assert host witnesses; accepted 10:
      [turn_verified(/create). turn_unverified(/create). turn_wrote(/create).
       turn_build_failed(/create). turn_missing_evidence(/create, /tests_not_green).
       has_turn_acceptance(/create). turn_created_source("pkg/foo.go").
       build_state(/passing). build_state(/failing). test_state(/passing).]
--- FAIL: TestPlannerPrefixPolicyCannotReachBuildState
    the build_ prefix must not reach build_state; blocked=[]
--- FAIL: TestModelObservationPolicyNamesNoHostWitness
    ModelObservationPolicy names host witness "test_state"; it is hard-blocked, so the entry is dead
--- FAIL: TestCorpus_TurnVerificationRulesLoadAndDerive
    turn_wrote = 0, want 1 for a /create that wrote
```

Two of these deserve reading twice.

**`TestModelCannotAssertTurnDone` accepted ten of fifteen host witnesses**, including
`build_state(/passing).` and `test_state(/passing).`, under the most permissive policy there
is. `turn_created_source` was on that list too and had never been blocked — it is the fact
the new-source test obligation joins on, so a model could have written itself out of owing a
test. That one was not in the brief; it turned up because the test enumerated the block
rather than spot-checking it.

**`TestCorpus_TurnVerificationRulesLoadAndDerive` failed at the derivation, not at the
declaration check** — and that is how the declaration check got fixed. The first version
looped over `k.Query(pred)` and required no error. Against a corpus declaring none of these
predicates, **every one of those queries came back clean**: an undeclared predicate returns an
empty result set on this engine, not an error. A test asserting "the query did not fail"
would have proved nothing and passed forever. It now calls `validatePredicateDeclaration`
with the expected arity. This is the Decl-contract trap from `internal/mangle/agents.md`
appearing inside the test written to guard against it.

Then, with the new policy in place and only `verdictSentence` reverted, isolating the
sentence:

```
--- FAIL: TestTurnOutcome_UnverifiedNamesMissingEvidence
    the evidence sentence must state the verdict, got
    "did the work\n\nWrote 1 file(s): pkg/foo.go\nEvidence: artifact_changed.
     Requested behavior remains unverified (no acceptance contract)."
```

`TestTurnDone_AcceptancePathStillSufficient` **passed before and after**, by design: it is the
regression guard proving the contract path was not disturbed.

### Pass-after

```
ok  codenerd/internal/session   (all 10 new cases + the 4 pre-existing turn_done tests)
ok  codenerd/internal/core      (4 authority tests + the corpus gate)
```

## Full test run

```
go build ./...                                                    clean
go vet ./internal/session/ ./internal/core/ ./internal/system/ \
       ./cmd/nerd/ ./internal/prompt/ ./cmd/nerd/chat/            clean

go test ./internal/session/                    ok   151.4s
go test ./internal/prompt/                     ok    14.4s
go test ./internal/core/                       ok   231.2s
go test ./internal/core/defaults               ok     2.1s
go test ./internal/core/defaults/policy        ok     1.3s
go test ./internal/core/shards                 ok     5.8s
go test ./cmd/nerd/chat/                       ok    32.7s
go test ./cmd/nerd/                            ok    16.1s
go test ./internal/system/                     ok    63.5s
go test ./internal/shards/ ./internal/shards/system/   ok
```

Each commit was extracted with `git archive` into the scratchpad and built in isolation
(`go build ./...` + `go vet`) rather than assumed to build from the working tree:
`fc2db654` clean, `c15b76d4` clean.

First full `go test ./...`: **87 packages ok, one failure** — `internal/autopoiesis`
`TestOuroborosLoop_HotReload_LockedBinary`, *"timed out waiting for hotreload test
execution"*, then a Windows `Access is denied` unlinking the `.exe` it had just built. Checked
rather than assumed: the package is **untouched** by this seam
(`git diff HEAD -- internal/autopoiesis` is empty) and **unchanged on this branch**
(`git log fc2db654~1..dogfood/c2-closure -- internal/autopoiesis` is empty), and the test
passes in isolation in 0.87s against a 5.54s timeout under full-suite load. It is the same
subprocess-build flake family the brief names for this package.

Second full `go test ./...`: **87 packages ok, 0 failing.** The flake did not recur.

`go test ./internal/session/` re-run green after the final cleanup (the unreachable
acceptance branch in `verdictSentence`).

## Open

1. **The prompt corpus is stale with respect to the edited atom.** Deleting the
   `test_state(/passing).` line from `protocol/piggyback/mangle_updates` changes an atom whose
   embedded copy lives in `internal/core/defaults/prompt_corpus.db` and in the `.nerd` store.
   `TestPromptCorpusAvailableMatchesDisk` only checks the file **exists**, so nothing goes red,
   and until a re-embed the model may still be served the old text — which teaches a predicate
   the filter now hard-blocks. The fix is an operational one (`nerd` re-embed), which this
   agent is not permitted to run. **This is the one item that should be done before the next
   live session**, because the failure mode is the model dutifully emitting a fact that is
   silently dropped.
2. **`context_budget/2` is the same hole, one line away, and deliberately left.** C-27 names
   two reachable instances of the prefix hazard; this seam closed `build_state` and left
   `context_budget`, which matches the planner's `context_` prefix and is the **only** producer
   of the fact gating `final_injectable` and `stage_shard_tool_allowed` — i.e. the model
   deciding its own tool catalogue. It belongs to C-11's seam, not this one, and blocking it
   blind could starve a legitimate producer I have not traced. `TestModelCannotAssertTurnDone`
   enumerates the block as a list, so adding it is one line plus one list entry.
3. **C-27's own invariant test is still TO WRITE.** What landed here is the specific closure
   (a named list, asserted against the filter). The registry asks for the general one —
   `TestModelAssertablePredicatesDoNotFeedTheVerdict`: expand every shipped policy's prefixes
   against the live Decl set and assert none of them appears in a rule body reachable from
   `turn_done` / `turn_executed` / `hollow_success` / `final_injectable` /
   `stage_shard_tool_allowed`. That test would have found `build_state`, `test_state`,
   `turn_created_source` **and** `context_budget` by itself, and would fail whenever either
   side moves. The list-based test is the floor, not the ceiling.
4. **Campaign tasks still read prose, not the verdict.** `Orchestrator.spawnTask` returns a
   `string`, so `executeShardSpawnTask` and its siblings
   (`internal/campaign/orchestrator_task_handlers.go`) cannot see `TurnOutcome`. Requirement 3
   is met for chat (via `observation.Return`, S1), for the session executor and planned steps
   (they run through `Executor.Process`, which is where the single read lives), and now for
   the direct CLI verbs; the campaign is the remaining surface, and widening it means
   threading `observation.Return` through `spawnTask` the way S1 threaded it through the chat.
5. **`cmd/nerd/cmd_interactive.go` has its own spawn loop** and still exits on the error alone.
   It shares `checkDirectSpawnResult` with the one-shot path but not the observed spawn. Small,
   and it did not block this seam.
6. **A markdown-only write turn is `/unverified`, by construction.** `turn_wrote` cannot tell a
   `.md` write from a `.go` one because the fact space has no "modified source this turn"
   predicate — only `turn_created_source` for files *created*. The conservative reading was
   chosen deliberately (see Design A). If that proves noisy in practice, the honest fix is a
   producer for the missing fact, not a looser rule.
7. **`consumeHollowSuccessVerdict` still carries the imperative twin** of the hollow rules
   (verification row 76). Nothing in it became dead here — this seam changed what happens
   *after* the hollow verdict, not the hollow verdict itself — so nothing was deleted from it.
   It remains a standing item for whoever owns that row.
