# Reviewer-to-fixer tasks must keep each finding's file owner

## Finding and decision

**A broad follow-up fix can hand the coder one selected file followed by findings from several files, with the individual filenames removed.** The ordinary review parser has already recovered the correct file and line for each finding. The next task formatter first renders all findings against the broad target `codebase`, then asks Mangle to select one file, and finally prefixes the unchanged mixed-file list with `fix file:<selected file>`.

The deterministic defect is the resulting task projection: a finding originally about B is rendered as an unqualified line number and message under a task naming A. It is not a finding that every model edits A incorrectly, that B's evidence disappears everywhere, or that permissions or verification are bypassed. Original request text, the structured broad target, retained review evidence, code inspection, verification, and delivery checking can all limit the downstream consequence.

**Decision:** preserve the association between each finding and its source file when selecting a preferred execution target. If policy intentionally dispatches one file at a time, partition the findings after selection and retain the remaining obligations explicitly. If it dispatches broad work with one preferred starting file, label that distinction and carry fully qualified citations. Do not repair a contradictory task by silently dropping the other requested files.

Source pin: [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), confirmed as main on October 30, 2026. This is static source inspection, not an executed reproduction. Guidance, active branches, current audits and research additions through [`research-main-20261030-1417`](https://github.com/theRebelliousNerd/codenerd/commit/6d97defb8901c0320d7561d4f15d4c13ee799bc9) were reviewed. No application execution, tests, model inference, benchmarks, or repository writes were performed.

## Existing repairs and neighboring work

The ordinary `extractFindings` path already repairs a consequential producer/consumer mismatch: it extracts `file`, `line`, `message`, and normalized severity, while preserving the raw finding. It accepts the reviewer's CRITICAL/HIGH/MEDIUM/LOW vocabulary and legacy aliases. Its tests explicitly check locations, severity, optional lines, and exclusion of another file when formatting an explicit target. This proposal depends on that repair working; it must not replace it with another prose parser. [Parser and rationale][parser], [current tests][findings-tests].

The file-selection rule is also deliberate and repaired. Mangle ranks files by citation count, then worst severity, then earliest finding index, rather than Go map iteration order. The host checks that exactly one file was derived and keeps the generic target if it cannot obtain that result. This report does not propose moving selection back into Go, changing the ranking, or treating selection as authority to write. [Policy][selection-policy], [host adapter][selection-adapter], [selection tests][selection-tests].

This is a continuation of existing fidelity work at a different boundary:

- [The multi-shard location audit][prior-location] concerns a separate parser that leaves typed coordinates empty before deduplication. Here the ordinary parser supplies correct coordinates, all findings survive, and the later task renderer removes the file component. No aggregation or deduplication is required.
- [The target-substring audit][prior-substring] concerns a literal path becoming workspace-wide discovery. Here `codebase` is genuinely broad; discovery or substring interpretation is not the cause.
- [C-06/S2][contract] and the [slash specialist-target audit][prior-slash] distinguish original wording, location, and constraints. Current natural-language delegation carries those fields separately. The residual is not loss of the original request or a slash parser defect.
- [The delegated-verdict audit][prior-verdict] concerns reconstruction of a prior result's success status. The present witness uses a successfully completed review and concerns the ownership of its individual citations.

The existing `priorShardContext` codec demonstrates a useful alternative representation: it constructs typed findings with file, line, severity and message and retains full output behind an expansion handle. But the review-to-`/fix` arm uses `formatFindingsForTask`, not that codec. A correct helper on another branch does not repair this one. [Codec and its callers][codec], [codec tests][codec-tests].

## Reachable production path

### 1. An ordinary review can establish the correctly located prior result

A delegated query can run as a background reviewer. The background runner retains its observed result, runs the existing completion/verification loop, and, on successful same-session completion, calls `storeShardResult`. With no configured follow-up, it displays the report and returns to the conversation. This is a normal producer for a prior reviewer result, without the multi-shard parser implicated by earlier audits. [Background routing][background-rule], [execution and retention][background-run], [completion][background-finish].

`storeShardResult` saves the full output and runs `extractFindings`, then installs the result as `lastShardResult` and in the recent shard history. The witness requires no intervening shard to replace it. [Storage][storage].

### 2. The broad fix is formatted before the selected file is known

For a later `/fix` intent, `formatShardTaskWithContext` takes the reviewer branch and performs these operations in order:

1. `formatFindingsForTask(priorResult.Findings, target)` builds the finding string.
2. If that string is nonempty, the caller resolves `actualTarget` when the original target is generic.
3. It returns `fix file:<actualTarget> findings:[<already built string>]`, preserving the current constraint.

For `target == "codebase"`, the renderer's file filter is disabled. It emits each finding as `severity@L<line>:<message>`, or `severity:<message>` without a line. It never renders that finding's file. Resolving A afterward neither rebuilds the list for A nor restores B's identity. [Formatting order][formatter], [finding projection][projection].

Use the exact broad sentinel `codebase` for the main witness. An empty target has the same relevant bypass, but `none` is **not** interchangeable here: despite being listed by the later target-resolution branch, it is not exempted by the earlier finding filter. This audit does not generalize the demonstrated mixed-file result to every generic spelling.

### 3. The task reaches the real mutation executor

Normal `processInput` constructs this contextual task before mutation verification. It refuses a mutation lacking the derived verification obligation, and refuses a required verification route without a verifier. When admitted, it calls `VerifyWithRetry` with the synthesized task, the original `intent.Target`, current constraint, original input, and session context. The broad structured target is not rewritten to the selected path merely because the task prose names that path. [Production formatting and guards][chat-delegation].

The verifier forwards these fields into `TaskRequest` for each attempt. `TaskText` retains the person's own words and appends the synthesized task; ordinary inline execution then supplies that combined text to `ProcessWithIntent` under its preset intent. Thus the problematic string is real worker input, not just a log label or unused legacy renderer. It is also **not the entire model input**: the original request, constraints and other context can counteract it. [Verifier handoff][verifier], [task text contract][task-text], [inline execution][executor].

## Source-derived witness

Use an initialized same-session TUI, ordinary kernel/world/CodeDOM wiring, a completed single-reviewer result, and an admitted foreground mutation. Choose Yolo for the simplest schedule, or explicitly approve the proposed work; the finding does not bypass work-mode admission. Assume successful perception yields `/fix`, category `/mutation`, target `codebase`, and normal delegation with verification. This classification is a condition, not a promise that every model will choose it.

Let the reviewer return three valid finding lines:

```text
- [HIGH] pkg/cache.go:12: Shared state is mutated without a lock.
- [MEDIUM] pkg/cache.go:24: A failed refresh discards the prior value.
- [HIGH] pkg/queue.go:90: A canceled caller remains in the wait list.
```

The user then asks, “Fix the findings across the codebase.” Keep the findings as the current prior reviewer result. Their messages deliberately do not repeat the filenames, so the file field is the source of the association.

The source predicts:

1. All three findings are parsed with their proper file and line.
2. The `codebase` renderer retains all three but strips their file fields.
3. Mangle selects `pkg/cache.go`: two citations beat one under the existing count-first rule.
4. With no additional constraint, the synthesized task is:

```text
fix file:pkg/cache.go findings:[high@L12:Shared state is mutated without a lock.; medium@L24:A failed refresh discards the prior value.; high@L90:A canceled caller remains in the wait list.]
```

`pkg/queue.go` is absent from that string. Its line 90 and diagnostic now sit under the sole named file, `pkg/cache.go`. The stored finding still points to `pkg/queue.go`; the corruption is in this projection, not the original object.

A useful equivalence check is to replace only the third finding's file with `pkg/worker.go`, keeping it a separate one-citation file. The selected file and synthesized task remain identical even though the third repair's location changed. This is equivalence of **the task projection**, not proof that the complete prompt or retained session state is identical.

The repository already has an even closer fixture: `reviewerOutput` contains two findings in `internal/auth/session.go` and two in `internal/auth/login.go`. `TestFormatShardTaskNamesTheDerivedFile` uses that fixture with `codebase` and confirms the selected `session.go` prefix, but does not inspect whether login's finding text remains under it. The separate finding-format test filters with explicit `session.go` and correctly excludes login's text. These are valuable tests whose composition is missing, not absent tests or executed failures. [Fixture and explicit-target control][findings-tests], [broad-target test][selection-tests].

## Consequences, protections and limits

The immediate, unconditional result under the stated input is an under-specified and misleading finding-to-file association in the synthesized task. Possible later costs are inspecting an unrelated line in A, searching again for the missing location, repairing only A while neglecting B, or making an unnecessary change. None was measured or reproduced, and none is required to establish the projection mismatch.

Several safeguards remain material:

- The original request says the work is broad, and `TaskRequest.Target` remains `codebase`. `TaskText` preserves that wording. A model can recognize the mismatch and investigate rather than follow the narrow prose literally.
- The original review output, structured findings and UI history remain retained. `buildSessionContext` can collect raw ordinary findings into `RecentFindings`, subject to its existing last-twenty bound. This is an alternate carrier, not a guarantee every JIT prompt renders it. No claim of global evidence erasure is made. [Storage][storage], [session-context projection][session-context].
- Explicit-target formatting can correctly exclude unrelated files. The documented single-file positive case must stay intact; this audit does not rely on a malformed path or a suffix collision.
- The worker has ordinary tools and world/CodeDOM context to inspect actual source. Target text is not permission, and restriction/path/effect checks still apply. Mutation verification, kernel verdicts, evidence-grounded judging and retries can catch or repair bad work. [Mutation guard and request][chat-delegation], [verification loop][verifier].
- After an accepted mutation, the enabled delivery checker can compare the original request to reports and propose remaining work. It requires the configured kernel/Jev/instruction-coverage path and is not guaranteed to detect a lost file association. Its presence limits a claim of inevitable abandonment, not the demonstrated malformed handoff. [Delivery handoff][delivery-call], [delivery enablement][delivery-gate].

The narrow scope is ordinary prior-review-to-fix synthesis. It does not claim that every `/fix` slash handler takes this contextual route, that one file's findings are always falsely assigned to another, or that a reviewer finding is itself established truth.

## Proposed contract

Keep three separate concepts: the user's requested scope, the policy-selected next execution scope, and each finding's source location. A selected path must not implicitly become the owner of every unqualified diagnostic that follows it.

At the boundary, retain typed finding identity/provenance, file, optional line, severity, message, and source revision when available. Preserve unresolved locations honestly; do not assign them the selected target by default. Render file-qualified citations from that record rather than reparsing already-rendered prose.

Two compatible designs need an explicit policy choice:

1. **Broad task with a preferred start:** retain the broad target and all requested findings with their file paths; identify A as the preferred first file, not the exclusive location of all diagnostics.
2. **One-file task:** obtain A first, then project only A's findings into that task. Record B's findings as pending, explicitly covered by another task, deferred for a stated reason, or declined. Preserve the parent's broad obligation until those dispositions are settled. Merely moving filtering after selection would otherwise turn misattribution into silent scope loss.

Reuse the existing typed observation projection where its recall and admission contract fits. A bare expansion handle is insufficient if the receiving route cannot redeem it. Do not reintroduce an LLM classification pass to reconstruct filenames the host already possesses. Preserve current constraints and original words in either design, keep Mangle responsible for execution selection, and do not conflate a correctly transported finding with a verified defect.

## Costs and fair evaluation

Adding filenames increases prompt bytes roughly with the total citation-path length. Grouping findings by file can reduce repetition but requires stable grouping and unambiguous line ownership. Typed records and remainder accounting add storage, identity/lifecycle plumbing, and regression coverage. A single-file plan may add task transitions or judge calls; broad dispatch may increase context and distraction. Existing handle-based projection can bound requests but adds recall latency and demands lifetime guarantees. No net token, latency, or accuracy benefit is claimed without measuring the whole task.

Evaluation is proposed, not run:

1. Enter a real background review through the TUI and return controlled, valid ordinary reviewer output. Let normal completion parse/store it. Then submit the broad fix through normal perception/routing/work-mode admission. Capture the admitted intent, policy-selected file, synthesized task, `TaskRequest` fields, final worker request and available alternative context. A direct helper call alone is insufficient.
2. Assert all three findings retain their two-file ownership relationships at the worker, or that selected and deferred obligations explicitly partition the complete original set. Verify that no diagnostic from B is rendered as an unqualified citation owned by A.
3. Preserve controls: single-file explicit target, one finding, repeated findings in one file, count/severity/index ties, missing kernel/derivation, missing line, unresolved location, empty findings, multiple review generations, session changes, constraints, and normal verification refusal. Distinguish `codebase`, empty and `none` rather than assuming their current behavior is identical.
4. Include the existing two-file fixture and the equivalence pair changing only B's filename. Add a case with more than twenty findings so a separately bounded alternate context does not accidentally mask a broken task contract.
5. In separately authorized end-to-end runs, keep provider/model, workspace revision, tools, permissions, world/CodeDOM context and verification obligations fixed. Compare current formatting, fully qualified broad projection, and policy-scoped partitioning. Measure verified repairs per original finding, missed and unnecessary edits, wrong-file investigations, rereads, model/embedding/judge requests, tokens, context bytes, latency, human correction and total cost per verified successful task. Report model recovery as recovery, not proof the original projection was correct.

**Acceptance:** every finding handed to a worker remains associated with its actual source location, and narrowing execution to one file never silently reassigns or discards another file's requested work.

## Branch boundary

The formatter/projection section was identical on the observed three lane heads (`a53cfc30`, `80af1863`, `aa7988de`) and two QA heads (`51cfd426`, `767a3291`). This is a scoped comparison, not a claim of whole-branch equivalence. The proposal extends existing task- and finding-fidelity repairs; it does not claim a new general identity principle.

[parser]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L42-L108
[findings-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/findings_test.go
[selection-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/delegation.mg#L620-L696
[selection-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L281-L345
[selection-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_decision_test.go#L61-L144
[prior-location]: https://github.com/theRebelliousNerd/codenerd/blob/58c84997ba39254e22c590f6cb3bab43a58509c1/Docs/audits/review-finding-location-deduplication.md
[prior-substring]: https://github.com/theRebelliousNerd/codenerd/blob/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits/delegation-target-substring-scope.md
[contract]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/journeys/04-contract-registry.md#L156-L187
[prior-slash]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/slash-command-specialist-target-continuity.md
[prior-verdict]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/delegated-verdict-handoff.md
[codec]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L124-L235
[codec-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_codec_test.go
[background-rule]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/routing_arbitration.mg#L311-L317
[background-run]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/background.go#L182-L242
[background-finish]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/background.go#L251-L277
[storage]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L264-L297
[formatter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L112-L140
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L238-L259
[chat-delegation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L590-L633
[verifier]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/verification/verifier.go#L253-L339
[task-text]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L28-L65
[executor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L375-L449
[session-context]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_session_context.go#L90-L114
[delivery-call]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L672-L687
[delivery-gate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_delivery.go#L42-L69
