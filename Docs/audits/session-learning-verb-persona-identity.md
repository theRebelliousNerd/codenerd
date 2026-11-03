---
doc-class: deep-dive
subsystem: session-system-autopoiesis-prompt
implementation-status: proposed
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
scope: finished session turn identity through failure grouping and evolved role selection
---

# Preserve intent and serving persona separately in session prompt learning

## Decision

Carry the turn's actual serving persona alongside its intent verb into prompt-evolution evidence. Preserve the intentional verb-level failure grouping, but do not reuse that grouping label as a prompt's role selector.

The normal session executor compiles a `/fix` turn as `/coder`. Its finished-turn adapter then writes `/fix` into `ExecutionRecord.ShardType`. Failure grouping preserves this value, and atom generation stamps it into `ShardTypes`. A candidate successfully generated from those failures therefore targets role `fix`, while the same task's next ordinary compilation is role `coder`. The evolved-atom reader correctly rejects that mismatch even when the candidate has an approved exact-version trial or is promoted.

This is an identity handoff defect, not missing boot wiring or a reason to weaken role gating. It can purchase and retire failure evidence without producing a candidate eligible for the originating persona. The same field mismatch also prevents those groups from refining seeded `/coder` strategies, independently of the previously reported global-limit-before-scope lookup defect.

**Evidence boundary:** public main was re-resolved at approximately 08:43 UTC on 2026-10-30 to [`388205138ba6d47c1cff0c5e97c896dfa642a391`][main]. This is static source inspection with independent source challenge, not a reproduced production incident. No repository program, test, model inference or benchmark was run. No implementation was changed. The fixture and evaluation below are proposed.

## Preserve the deliberate distinction between verbs and personas

`TurnRecord.IntentVerb` is explicitly documented as the canonical action verb and a learner's grouping key. Grouping `/fix` failures separately from `/refactor` failures is therefore intentional evidence granularity, not itself a defect. Both verbs may be carried out by the coder persona. The prompt contract separately represents `IntentVerbs` and `ShardTypes`; one describes what is being done, the other who is doing it. [Turn record contract][turn-contract], [prompt dimensions][dimensions]

The repair should not simply replace every stored verb with its current persona and merge all coder work into one group. Nor should it translate `fix` into `coder` inside the generic selector matcher, which would make unrelated fields silently interchangeable. Preserve both identities and specify where each is used.

The existing source-role pin is a useful safeguard: generated guidance must not spread to other roles merely because the generating model requests that. The defect is that the adapter supplies the verb as if it were the observed role. Keep the pin strict and repair its producer. [Pin contract][pin]

## Production trace

### The session prompt has the correct persona

The stock taxonomy maps `/fix` to `/coder`. `buildCompilationContextWith` keeps `intent.Verb` in `cc.IntentVerb` and separately resolves the persona into `cc.ShardType`, with the bare agent name in `cc.ShardID`. Thus the ordinary fix turn can select coder-specific instructions and strategies without pretending the persona is named `fix`. User-defined agent handling separately supplies that agent's role when no built-in mapping exists. [Taxonomy][taxonomy], [compilation context][compile-role]

### The finished-turn carrier loses that distinction

`persistTurn` emits `TurnRecord` with `IntentVerb`, task, outcome, included atom IDs and serving provider/model, but no separate serving-role field. The shared factory installs `turnEvolutionRecorder` on the session executor. Its `executionRecordFor` adapter trims `rec.IntentVerb`, names the local variable `shardType`, and writes it into both the execution record and any prefilled PASS verdict. For a fix turn, that value is `/fix`, not `/coder`. [Finished-turn producer][producer], [shared wiring][wiring], [adapter][adapter]

The ordinary admission controls remain: the executor checks the persona's learning policy; the factory recorder drops unverified/read-only outcomes; verified work receives a host-derived PASS without another judge call; hollow/failed work reaches the judge for an explanation. The proposed witness uses admitted failed turns, not an unverified question or a model's self-reported success. [Recording policy][record-policy], [outcome translation][adapter]

### Grouping and generation treat the verb as the role

`FeedbackCollector` stores the supplied shard field and `GroupKeyFor` copies it unchanged. The selected failure group retains `/fix`. After the serving-pin, cohort, screen and claim gates, `generateForGroup` forwards `group.key.ShardType` to `GenerateFromFailures`. The generator also writes that value into its generated-record metadata. [Persistence][record-store], [group identity][group], [generation handoff][generate], [candidate metadata][candidate]

`requirePin` checks that the role is nonempty; it does not recover the role used by the session. `/fix` satisfies that presence check. `applyPin` then replaces the generated atom's role selectors with the supplied source value. Normalization strips a leading slash, so the result is `ShardTypes: ["fix"]`. The model cannot repair the mistake by suggesting `coder`: role stamping deliberately overrides it. [Presence and role stamping][pin], [conversion][convert], [normalization][normalize]

The successful publication path completes the generation claim, marks the selected task/verdict evidence consumed and persists the candidate. Its wrapper retains both the atom and the same mistaken `ShardType` metadata. This can be normal successful bookkeeping; no database corruption or failed write is needed. [Publication][publication], [wrapper][wrapper]

### Approval does not repair a role mismatch

The evolved-atom manager validates and loads wrapper atoms, then calls `MatchesContext` **before** considering promoted state or an approved trial version. The role selector uses literal normalized comparison. `fix` does not match `/coder`, even if `cc.IntentVerb` is `/fix` and provider/model are identical. Ordinary trial approval does not change selectors. [Wrapper loading][load], [serving admission][serve], [role match][match], [literal comparison][literal], [trial approval][trial]

The structural `PromptAtom.Validate` checks required identity/content/category and self-relations, not a verb-to-persona remapping. Consequently, a nonempty otherwise-valid generated atom can survive loading with the wrong role and then be correctly omitted by context matching. This is not a claim that the loader must admit every generated response. [Validation][validate]

## One conditional full-path fixture

Use a normal shared boot with learning enabled for the coder persona and a valid, stable serving provider/model. Run three distinct `/fix` session turns that finish `/failed` or `/hollow` and yield eligible, versioned agent-failure verdicts in one problem group. Hold unrelated failure groups out of the fixture to isolate identity from scheduling. The default minimum is three; an authorized implementation fixture should pin its configuration explicitly rather than rely on defaults drifting. [Defaults][defaults]

Then arrange an explicitly invoked or enabled evolution cycle with sufficient judge, screen, generation and atom allowances. The shared-cause screen permits the cohort, the generation claim succeeds, the generator returns one otherwise-valid candidate, admission keeps it, and persistence succeeds. These are explicit reachability conditions, not predictions of real model answers.

The expected source-derived sequence is:

1. The original compilation is `/fix` intent, `/coder` persona.
2. All three records and their failure group carry `/fix` in the field subsequently used as source role.
3. Generation stores a candidate with `ShardTypes: ["fix"]` and the expected provider/model pin; its selected evidence is consumed on successful publication.
4. Reload the candidate through the normal evolved manager and approve its exact version for a trial, preserving expiry and exposure allowance. Compile another `/fix` task on the same provider/model.
5. That compile again has role `/coder`. The candidate is excluded by role match before trial-state admission or downstream fitting can make it available.

No adverse concurrency, exhausted budget, missing model pin, source-ID collision, trial quota race, disabled learning or stale compiler cache is necessary. Promotion is not required; an approved trial is enough to expose the mismatch. A promoted copy would meet the same context check, so promotion alone is not a repair either.

The proven consequence is missed candidate eligibility for the originating persona after successful learning work. It is not proof that the candidate would improve a task, that every candidate is omitted, or that all self-learning fails. Actual provider spend and prevalence are unmeasured. When the same evidence is already consumed, simply waiting for another cycle does not deliberately regenerate a corrected candidate from it under the unchanged generator contract.

## Related paths that already work

### Persona learning is a different working producer

`recordShardLearnings` resolves `personaForVerb`, checks that persona's learning policy, and saves typed lessons to that persona's learning store. The session's terminal path invokes it separately from `recordTurn`. This supplies a real comparison: a failed fix turn can still teach the coder's persona store even while its prompt-evolution record mislabels the role. Do not report the coder as incapable of learning or remove this path as a duplicate. [Persona resolution and storage][persona-learning], [terminal learning calls][terminal-learning]

### The TUI also has a separate recording adapter

`recordShardExecution` receives an explicit `shardType`, obtains a prompt manifest/atom list, and submits its own execution record with strategy attribution disabled. It is not the shared session adapter's `rec.IntentVerb` assignment. Whether a particular TUI call supplies bare or slash-prefixed role text, its attribution and grouping need their own contract; this report does not claim every TUI record is wrong in the same way or that this additional record repairs a headless session's wrong-role candidate. [TUI adapter][tui]

Avoid combining the two producers into a synthetic fixture that quietly credits the session with a correctly named TUI record. Test each actual entry path and its producer identity. Any duplicate-record or last-compiler-result ownership concern is outside this finding.

### Strategy serving works but the wrong group cannot refine its rows

The strategy provider selects by the compilation's problem type and shard. The defaults include `/coder` strategies; finished session turns can report their `strategy/` atom IDs for outcome attribution. None of that changes the failure group's role. At the refinement handoff, `updateStrategies` rejects a `/coder` strategy for a `/fix` group even if it appears in the returned candidate window. [Strategy provider][strategy-provider], [seed roles][strategy-seeds], [outcome attribution][strategy-outcomes], [refinement match][strategy-match]

The [scope-before-limit audit][strategy-audit] owns a different omission: unrelated rows exhausting a global candidate limit before the current-group filter. Correcting that SQL scope will not make `/fix` equal `/coder`; correcting this producer identity will not remove the global-window defect. Keep the repairs and their regression fixtures distinct.

## Relationship to existing audits and tests

The [prompt-evolution lifecycle audit][lifecycle] owns quarantine, trial/version lineage, claims, consumed evidence, recording and exposure attribution. The [screening-cohort audit][screening] owns which admitted failure records buy generation. The [JIT strategy-context audit][context] owns cancellation, metering and correlation crossing the classifier seam. The [trial-exposure continuation][trial-audit] owns admission of overlapping exposures. Their mechanisms should be preserved; this report concerns the meaning of the source-role field before those mechanisms consume it.

Current recorder tests check verified/failed/unverified translation, unique task IDs, provider/model provenance, nonempty grouping for an unnamed verb, opt-in automatic evolution and boot wiring. The pinning tests supply `/coder` evidence directly and check that role stamping yields `coder`. Those are useful controls, but they do not connect an actual `/fix` session compilation through the recorder to generated selectors and back to a normal coder compilation. [Recorder tests][recorder-tests], [pinning tests][pin-tests]

A bounded review of the current audit inventory and active research documents found no report owning this exact verb-as-role producer boundary. The relevant assignment and role stamping remain on all five other inspected lane/QA tips. This is not an exhaustive statement about every historical QA note and is not attributed to the latest screening repair.

## Proposed contract and repair direction

Update the session, system, prompt and prompt-evolution specifications with an explicit invariant:

> A learning observation preserves the canonical action verb and the serving persona as distinct identities. Grouping may retain the action distinction. Generated role selectors name only the persona supported by the source observation, and a normal recompilation of that same admitted persona is not excluded solely by substituting its action verb for its role.

Capture the effective role used for compilation/execution at the session boundary, then carry it through `TurnRecord`, persistence, grouping, cohort identity, candidate provenance and role stamping. Prefer the actual immutable serving context over a fresh taxonomy lookup when an asynchronous recorder processes the record: mappings and custom-agent registrations can change after execution. This is identity transport, not a new executive routing decision in Go.

Preserve the verb in a separately named field and make the grouping policy explicit. A group may need problem type, intent verb, persona and configured provider/model scope. Do not silently merge fix and refactor evidence just because both ran as coder. If cross-verb generalization is desired, it needs a declared evidence/policy rule rather than reuse of a misleading field. Conversely, applying an intent selector in addition to the role selector is a separate scoping decision; the identity correction must not silently introduce or remove that policy.

Missing or ambiguous role identity should be observable and should not authorize a broadly applicable atom. Keep the raw observation for diagnosis and any other valid learning use. Do not replace an unknown role with an empty selector, guess from free-form task text, or buy an LLM classification merely to reconstruct a role already known at execution time.

Retain recording policy, queue bounds, cancellation, context/usage ownership, cohort screening, claim fencing, global call and atom caps, candidate quarantine, exact-version trial approval, role/provider/model matching, normal prompt fitting and execution verification. A migration or identity correction is not permission to enable automatic evolution or promotion.

## Rollout and historical data

This is not only a new-field change. Existing records, candidate wrappers and ledger keys already encode the conflation.

1. **Version the observation schema.** Identify the producer and identity contract of new records. Historical `shard_type` may mean a session verb or a TUI/legacy role. Do not run a blanket slash-strip or verb-to-persona rewrite over every producer's records. Preserve original values and the evidence supporting any derived correction.
2. **Backfill only justified mappings.** A retained execution/compilation receipt may establish the actual role. A present-day taxonomy entry alone does not prove which custom or dynamically mapped persona served an old turn. Keep unresolved records explicitly unknown and out of role-pinned generation until a documented resolution exists.
3. **Preserve event idempotency.** The collector hashes execution identity including shard type. Changing an old record in place can alter payload identity without changing its task ID. Define a migration or versioned derived identity rather than making a replay look like a conflicting event or silently inventing a new execution. [Payload identity][payload]
4. **Keep consumption and reanalysis explicit.** Consumption is keyed by task ID and verdict version, while generation claims also encode group and generator identity. Correcting a group field does not justify deleting all consumed evidence or replaying every paid generation. Decide whether an affected cohort merits one bounded, authorized reanalysis, record its reason/version, and retain the prior spend and output. [Claim identity][claim-identity], [consumption][consumption]
5. **Correct candidate metadata and selectors together.** The current stored-pin migration re-applies selectors from wrapper `ShardType`; changing only the atom selector can therefore be undone on a later boot. A corrected candidate needs coherent provenance, a new semantic version/digest and an explicit lifecycle disposition. Retain the original for audit or rollback. [Stored-pin enforcement][pin-migration], [wrapper][wrapper]
6. **Do not turn old approval into new reach.** Candidate hashes include shard selectors and trial envelopes name an exact hash. A role correction changes where guidance can be served. Revoke or replace affected trials explicitly; do not transfer their approval or success counts automatically. A previously promoted but mis-scoped candidate must not become newly active in coder prompts merely because a maintenance migration rewrites its selector. Require a documented review/retrial policy for that change. [Candidate hash][hash], [trial][trial]
7. **Define restart and mixed-version behavior.** Make backfill repeatable, bounded and recoverable. Preserve untouched records and files on error. Ensure an older binary cannot reinterpret new fields or re-stamp corrected candidates under the obsolete contract. Advance the appropriate source generations/reload state so warm compilation caches do not conceal corrected identities.

Migration must report counts of proven corrections, unresolved roles, affected pending/promoted candidates, revoked trials, deferred reanalyses and errors. Those are host-side observations; they are not evidence of improved task quality.

## Costs and fair evaluation

No additional model request is intrinsically needed to preserve a role already present in the session. The direct change adds a small identity field and validation/serialization work. The broader cost can come from schema/index migration, historical receipt inspection, candidate re-versioning, cache invalidation, review and any explicitly authorized regeneration or retrial. More correctly eligible atoms also compete for prompt budget and may increase downstream tokens or displace other useful context. Report those costs rather than assuming the repair saves tokens.

Start with a deterministic, scripted full-path fixture. Use the real production recorder, collector, grouping, generator conversion/pinning, candidate persistence/reload, trial policy and compilation context. The scripted model supplies fixed eligible verdicts and a fixed valid candidate; it must not preconstruct an already-correct role selector and bypass the failing adapter. Compare against the pinned baseline with identical evidence, model identity, configuration, budgets and generated content.

| Case | Required observation |
|---|---|
| `/fix` session to generated candidate to approved trial | Retain intent `/fix` and persona `/coder`; the candidate passes the corresponding role check after the repair. |
| `/fix` and `/refactor` on coder | Retain distinct verb grouping unless a separately declared policy authorizes aggregation. |
| Tester and reviewer verbs | Correct persona identity without spreading guidance to coder or unrelated roles. |
| Custom agent and mapping change before asynchronous persistence | Use the observed serving role, not a later guessed mapping. |
| Missing role or unknown legacy producer | Observable unresolved state; no empty/broad role selector. |
| No trial, wrong model, wrong role, expired or exhausted trial | Preserve existing rejection; fixing the identity must not bypass lifecycle admission. |
| Disabled learning, unverified turn, disabled automatic evolution | Preserve existing no-record/no-automatic-purchase behavior. |
| Failed screen, failed generation, rejected candidate, failed publication | Preserve existing evidence, spend and error contracts. |
| Same task replay and migrated historical record | No duplicate event, silent identity overwrite or unearned strategy credit. |
| Already consumed cohort | No accidental unlimited regeneration caused by a new group spelling. |
| Pending and promoted wrong-role candidates across reopen | Coherent metadata/selectors; no resurrected obsolete pin or silent activation under old approval. |
| Direct TUI record and persona-learning store | Preserve their separate producer semantics and valid existing lessons. |
| Seeded coder strategy and fix failure group | After identity repair, role equality is possible; independently preserve the scope-before-limit regression and global refinement budget. |

Observe identity at every boundary, not merely a nonempty row or successful generation count. For final serving, verify the candidate is eligible and, in a controlled budget fixture, actually included in the outgoing prompt. Passing role matching alone does not guarantee inclusion: quarantine, tools, dependencies, conflicts, relevance and fitting still apply. Do not force inclusion in ordinary production just to satisfy the test.

Measure grouped evidence identity, consumed/reanalyzed records, generated and discarded candidates, exact versions, trial facts, final prompt inclusion, database/file operations, migration time and retained bytes, cache churn, host allocations, model calls/tokens where reported, unknown usage, latency and human review. Keep unsuccessful and deferred work in the accounting.

Only after the deterministic contract holds should an authorized matched-task study assess usefulness. Hold task families, models, tools, verification and total resource budgets constant; compare candidate exposure and independently verified task outcomes with uncertainty. Better role fidelity is a correctness result. Better coding quality or lower total cost remains an empirical question.

## Acceptance boundary

The repair is complete when a normal session can retain its action verb for learning granularity, name its actual serving persona for role pins, and carry a legitimately admitted candidate back through that persona's ordinary compilation without weakening existing gates. Historical uncertainty and changed approvals must remain explicit. A populated learning database or a successful atom-generation counter alone does not establish that round trip.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[turn-contract]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_learning.go#L12-L60
[dimensions]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms.go#L186-L190
[taxonomy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/perception/taxonomy.go#L648-L653
[compile-role]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L1647-L1774
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L2472-L2488
[wiring]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L164-L179
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L275-L354
[record-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_learning.go#L141-L178
[record-store]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/feedback_collector.go#L379-L392
[group]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/feedback_collector.go#L532-L594
[generate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L175-L247
[candidate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/atom_generator.go#L191-L218
[pin]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/pin_policy.go#L34-L63
[convert]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/atom_generator.go#L232-L265
[normalize]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms.go#L324-L374
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L273-L319
[wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L26-L78
[load]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/evolved_atoms.go#L199-L252
[serve]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/evolved_atoms.go#L278-L305
[match]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms.go#L476-L482
[literal]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms.go#L597-L615
[validate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms.go#L713-L742
[trial]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L295-L331
[persona-learning]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_shard_learning.go#L61-L102
[terminal-learning]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L2450-L2495
[tui]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L659-L717
[strategy-provider]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_atoms.go#L59-L94
[strategy-outcomes]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_atoms.go#L153-L179
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver.go#L88-L102
[strategy-seeds]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_store.go#L478-L520
[strategy-match]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L373-L403
[strategy-audit]: https://github.com/theRebelliousNerd/codenerd/blob/77069960b69aba14a67dbd682d6f65435e8ada50/Docs/audits/strategy-refinement-scope-before-limit.md
[lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md
[screening]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-screening-cohort.md
[context]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/jit-strategy-context-ownership.md
[trial-audit]: https://github.com/theRebelliousNerd/codenerd/blob/77069960b69aba14a67dbd682d6f65435e8ada50/Docs/audits/prompt-trial-exposure-admission.md
[recorder-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning_test.go
[pin-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/pinning_test.go#L128-L144
[payload]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/feedback_collector.go#L283-L315
[claim-identity]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L201-L215
[consumption]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L380-L409
[pin-migration]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/pin_migration.go#L33-L74
[hash]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/evolved_atoms.go#L43-L111
