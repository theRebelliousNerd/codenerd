---
doc-class: deep-dive
subsystem: verification
implementation-status: planned
last-verified: 2026-10-27
verified-against: e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a
supersedes: []
---

# Verification: evidence-grounded delegation judging without redundant repair

**Status:** Source-inspected improvement proposal, 2026-10-27. No implementation, tests, inference, training, or benchmark runs were performed. Source baseline: `e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a`. This document does not claim measured accuracy gains, savings, or superiority.

## Decision

Preserve CodeNERD's existing kernel-first verification and final-edit evidence closure. Improve the **handoff between observed execution, the advisory judge, and retry policy**:

1. Reject semantically invalid judge replies as unavailable verification, rather than interpreting missing fields as a judgment.
2. Feed a bounded, host-generated evidence packet to the judge instead of task/result text alone (which may include host-appended evidence summaries).
3. Distinguish an invalid judge response, insufficient evidence, a grounded defect, and a policy-accepted low-confidence objection. Do not automatically rerun the coding worker to repair the judge.
4. Evaluate whether the judge actually adds defect detection beyond the existing deterministic and acceptance gates, including its false-rejection and repair costs.

This is an incremental wiring refinement, not a new claim that the harness lacks build checks, kernel completion policy, or acceptance contracts. It also does not make the judge an executive.

## 1. Source and guidance read

The root [North Star / Repo Contract (`agents.md:5–110`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/agents.md#L5-L110) requires logic-derived executive decisions, JIT-first prompt behavior, revision-aware evidence, and source inspection rather than trusting old generated specifications. The [Core Runtime Guidance (`internal/core/agents.md:1–33`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/core/agents.md#L1-L33) reserves completion for host-issued current acceptance evidence. The [Runtime boundary (`internal/session/README.md:5–17`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/README.md#L5-L17) describes the typed, permission-gated execution boundary.

The existing [verification — Gap Analysis (`Docs/architecture/verification/03-GAP-ANALYSIS.md:1–5`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/Docs/architecture/verification/03-GAP-ANALYSIS.md#L1-L5) is explicitly July-era material. Its [Wave 2 reconciliation (`Docs/architecture/verification/TODO.md:38–66`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/Docs/architecture/verification/TODO.md#L38-L66) already records the fail-closed, JIT, observed-executor, context-propagation, and kernel-policy fixes. This proposal continues that reconciliation; it does not reintroduce its closed backlog.

| Capability | Classification at the pinned source | Evidence |
|---|---|---|
| Distinguish passed, failed, skipped, indeterminate, canceled checks | Implemented | [`VerifyOutcome` (`internal/session/verify_outcome.go:20–40`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/verify_outcome.go#L20-L40) |
| Re-measure final-edit evidence when a workspace changes after gates | Implemented | [`Executor.closeChangeEvidence` / `Executor.remeasureGates` (`internal/session/change_evidence.go:70–160`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/change_evidence.go#L70-L160) |
| Carry typed observed build, tests, change stage, acceptance and findings across the execution boundary | Implemented | [`observedReturn` (`internal/session/observed_return.go:47–139`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/observed_return.go#L47-L139), [`observation.Return` (`internal/observation/subagent.go:97–155`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/observation/subagent.go#L97-L155) |
| Judge only an attempt whose kernel outcome is /done | Implemented | [`TaskVerifier.VerifyWithRetry` (`internal/verification/verifier.go:235–310`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier.go#L235-L310), [`delegation_judge_due` / `judge_rejects` (`internal/core/defaults/policy/delegation.mg:354–398`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/core/defaults/policy/delegation.mg#L354-L398) |
| JIT-compiled judge prompt, no judge tools | Implemented | [`eval/delegation_judge/implementation` and `/review` (`internal/prompt/atoms/eval/delegation_judge.yaml:13–103`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/prompt/atoms/eval/delegation_judge.yaml#L13-L103), [`TaskVerifier.judgeSystemPrompt` (`internal/verification/verifier.go:140–160`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier.go#L140-L160) |
| Missing client, failed call, syntactically malformed JSON fail closed | Implemented | [`TaskVerifier.verifyTask` (`internal/verification/verifier.go:387–420`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier.go#L387-L420), [`TestVerifyWithRetry_WhenVerificationErrors_ShouldFailClosed` and nil-client companion (`internal/verification/verifier_failclosed_test.go:185–227`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier_failclosed_test.go#L185-L227) |
| Semantically validate judge payload completeness and consistency | Missing in inspected parser | [`parseVerificationResponse` (`internal/verification/verifier.go:512–526`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier.go#L512-L526) only strips fences and unmarshals |
| Supply structured execution observations to judge | Partially wired | observations reach verifier, but [`TaskVerifier.VerifyWithRetry` call to `verifyTask` (`internal/verification/verifier.go:261`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier.go#L261) passes only task, `ret.Output`, rubric |
| Snapshot identity across the observed judge handoff | Not represented in inspected Return shape | [`Executor.closeChangeEvidence` (`internal/session/change_evidence.go:95–98`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/change_evidence.go#L95-L98) sets ChecksSnapshot; [`observation.Return` (`internal/observation/subagent.go:97–155`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/observation/subagent.go#L97-L155) carries no corresponding field |
| Runtime defect frequency or savings attributable to these gaps | Unverified | static inspection only |

“Implemented” means an inspected source path exists, not a claim that this pass executed it successfully.

## 2. Gap A: JSON syntax is not a verification contract

[`VerificationResult` (`internal/verification/verifier.go:73–81`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier.go#L73-L81) uses non-pointer `bool`, `float64`, and `string` fields. The parser performs no required-field, range, enum, evidence, or consistency checks after decoding. [`percent` (`internal/verification/verifier.go:343–345`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier.go#L343-L345) subsequently clamps confidence into 0–100; clamping is not validation.

The standard [Go encoding/json contract](https://pkg.go.dev/encoding/json#Unmarshal) ignores unmatched object fields by default and allows JSON null without changing many destination values. Consequently, source analysis predicts:

| Reply | Parser result | Default rejection threshold 0 | Configured threshold greater than 0 |
|---|---|---|---|
| `{}` or `null` | zero-valued non-nil result, no decode error | fail at confidence 0 can veto; worker may retry until cap | confidence 0 does not veto an already-/done attempt |
| `{"success":true}` | success true, omitted confidence/reason become zero/empty | no violations, so judge pass | same |
| explicit fail with omitted confidence | indistinguishable from genuine confidence 0 | veto | potentially accepted as low-confidence objection |
| confidence outside [0,1] | ordinary finite values decode, later clamped | policy sees a manufactured boundary value | same issue |
| success true with a nonempty violations list | parse succeeds; judgeAtom correctly emits fail | veto; returned verdict may still carry Success=true, while persisted disposition is false on retry/escalation | may become accepted under confidence policy |

The default threshold is **zero**, not a positive guessed value: [`DefaultDelegationConfig` (`internal/config/delegation.go:24–27`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/config/delegation.go#L24-L27). Thus the first example is not an unconditional default fail-open bug. The precise problem is **semantic-invalidity conflation**: invalid data can become either a wasteful worker retry or a non-veto, depending on configuration. None of these cases bypasses the requirement that the underlying turn already ended /done.

### Proposed ingress contract

Before any `judge_verdict` fact:

- Require a single non-null object of a versioned schema.
- Track field presence independently of zero values; require status, confidence where applicable, and a nonblank explanation.
- Require finite confidence in [0,1]; reject rather than clamp out-of-range values.
- Define status explicitly as `pass`, `defect`, or `insufficient_evidence`. Invalid/unavailable is a host transport/validation state, not a model-voted status.
- Validate known violation values and corrective-action shapes. An optional extension map can preserve deliberate forward compatibility; unknown decision-bearing keys must not silently supply a different interpretation.
- Reject duplicate decision keys and contradictory status/violation combinations. JSON decoding alone is not enough to detect all duplicates.
- Require a defect to identify at least one evidence ID or clearly labeled unsupported suspicion. Unsupported suspicion cannot be laundered into a host fact.
- Bound reply bytes, items, and cited spans. Preserve the raw reply in a restricted diagnostic artifact, not endlessly in the model context.

A genuine low-confidence defect remains eligible for the existing policy threshold. A missing confidence field must not be treated as that defect.

**Recovery:** malformed/invalid judging returns a typed unavailable outcome without rerunning the worker. A separately budgeted judge-format repair, if adopted, must be derived by policy and operate over the same immutable evidence; its cost counts. Exhaustion stays unavailable, never success.

## 3. Gap B: typed evidence is not passed to the judge

The execution boundary already provides `Changed`, `Build`, `Tests`, `Stage`, `Acceptance`, `Findings`, `Untested`, and `Notes`. The verifier uses `Outcome` and `Missing` to respect the kernel, but the judge prompt receives only `d.Task` and `ret.Output`. The judge's implementation rubric asks it to identify hallucinated APIs, empty tests and missing implementation. The output is not necessarily pure worker prose: [`Executor.appendEvidenceSummary` (`internal/session/change_evidence.go:324–351`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/change_evidence.go#L324-L351) appends host acceptance or stage/verdict summaries to `result.Response`, which `observedReturn` assigns to `out.Output`. The narrow gap is loss of typed provenance and richer artifact evidence, not absence of all host evidence. Judging the rubric's claims may require actual artifact spans beyond those summaries.

This is not proof that every text-only judgment is wrong. It is a narrower evidence limitation: the current judge invocation cannot independently resolve a claim absent from its task/result text.

### Proposed evidence packet

Construct an immutable host packet, with explicit provenance for every field:

- delegation/attempt ID, task contract ID, rubric version and source revision;
- kernel outcome and missing obligations;
- host-observed change stage and acceptance report;
- changed paths and before/after content identities, not unrestricted file bodies;
- each check's scope, command/config identity where available, explicit outcome, source=observed, and an artifact handle for details;
- deterministic selected source/diff spans relevant to named claims;
- worker narrative in a separate source=reported field;
- omitted-evidence counts and retrieval availability.

Do not invent identities from prose. Where the existing return does not carry a needed identity, extend its host-produced shape or mark identity unavailable. The session's current `ChecksSnapshot` is an existing starting point, not evidence that this field is already wired across the boundary.

The default bounded packet should avoid full logs and whole-repository dumps. A judge with no tools can request evidence IDs in a structured response. Policy decides whether a bounded second read is warranted; the host validates paths, permissions and revision, then supplies the requested slice to another judge call. The judge itself gains no shell, write, or completion authority. All new prompt behavior belongs in JIT atoms.

### Freshness and concurrency

For evidence item e, define an applicability predicate:

`usable(e, a) = hostIssued(e) ∧ e.attempt = a ∧ e.snapshot = a.snapshot ∧ scopeCompatible(e, a)`.

A matching source snapshot alone is insufficient if toolchain, configuration, command or relevant environment changed. Include those identities before reusing check receipts. An identity that cannot be established yields unknown applicability.

The existing final-edit closure should remain authoritative for its current duties. Proposed handoff freshness checks prevent a judge citation from describing a superseded artifact; they do not replace that closure or license the judge to cancel a concurrent writer's work. On a changed workspace, invalidate the affected claim and request fresh evidence through the normal policy path. Do not silently apply an old objection to a new revision.

## 4. Decision and retry algorithm

Conceptual sequence, not implementation:

1. Obtain the observed worker result.
2. Assert the real kernel outcome for the attempt. If not /done, use the existing kernel move; do not ask the judge to turn it into success.
3. Freeze the evidence packet with its manifest and rubric identity.
4. Compile the judge's JIT prompt and ask it about residual semantic obligations, not to reinterpret a completed compiler command.
5. Validate the response against the ingress contract.
6. Resolve each citation against the frozen packet. Unsupported or stale citations become insufficient evidence, not a confirmed defect.
7. Assert the host-validated advisory judgment and let Mangle derive accept, retry, evidence acquisition, or escalation.
8. When retrying a worker, carry the precise grounded defect and relevant evidence IDs. When only the judge failed, do not pay for a worker rerun.
9. Persist both raw judge judgment and the separate kernel disposition. Keep a raw objection separately available even when policy accepts the attempt.

The last distinction matters today: [`TaskVerifier.VerifyWithRetry` (`internal/verification/verifier.go:285–295`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier.go#L285-L295) can rewrite the returned judgment's reason and set `verdict.Success = true` over a below-threshold objection. Persistence is more precise than a simple serialization of that field: [`TaskVerifier.storeVerification` (`internal/verification/verifier.go:459–505`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier.go#L459-L505) receives a separate `success bool` disposition, passes that to the store, and separately stores confidence, reason, violations and evidence. The finding is therefore not that storage mistakes `VerificationResult.Success` for the disposition. The narrower proposed refinement is to retain the original judge success/reason alongside the already-separate executive disposition, so a future calibration reader need not reconstruct raw judgment from an edited reason and remaining fields. The existing reconciliation notes that history selection readers are absent; this proposal does not restore an LLM-selected retry shard.

## 5. Accuracy and efficiency objective

The objective is not fewer judge tokens in isolation. Minimize expected complete-system cost subject to an agreed defect-escape ceiling and acceptable legitimate-task completion:

`C_total = C_worker + C_build/test + C_acceptance + C_packet + C_judge + C_evidence_reads + C_repair + C_persistence`.

For every model call count uncached input, cached reads/writes, output, reported reasoning tokens where exposed, retry and canceled-call charges, and missing-meter uncertainty. For tools count subprocess CPU/GPU time as applicable, wall latency, bytes read, storage and network use. Include repeated verification after edits and all attempts that fail. Do not divide only successful-attempt tokens by successes.

A supplemental judge is cost-justified only if its marginal avoided loss exceeds added work:

`P(correct additional detection) × L_escape > C_judge + C_packet + C_reads + P(false veto) × C_unnecessary_repair`.

This is a decision model, not a measured inequality. Loss estimates, thresholds, and coverage strata must be explicit. Quality is the constraint; no token optimization may downgrade unknown into passed.

Risks include larger packets increasing tokens, selective evidence omitting the decisive defect, biased judges confidently rejecting legitimate mocks, source-view exposure of secrets, additional reads becoming expensive, stale snapshots under concurrent writes, and schema strictness breaking older providers. Stage changes behind versioned contracts and preserve diagnostic visibility.

## 6. Evaluation plan, not executed results

### Socket-level tests

Compare the same frozen attempts with: current task/result-text judge (including any appended host evidence summaries); semantic validation only; host evidence packet; packet plus policy-mediated evidence requests. Hold model, sampling parameters, rubric, task and visible budget fixed. Include:

- empty/null/missing-field/duplicate-key and out-of-range replies;
- genuine low-confidence objection versus omitted confidence;
- prose claiming tests pass while observed check is skipped;
- correctly completed code with legitimate test mocks;
- stale citation after a concurrent edit;
- review tasks describing broken code correctly;
- unseen API hallucinations, vacuous tests, and behavior mismatches that compile;
- prompt-like instructions embedded in worker output or source comments;
- truncated or unavailable evidence and provider errors.

Static fixture inspection is useful here; execution remains future authorized work. Existing [`TestParseVerificationResponse_WhenValidJSON_ShouldParse` and companion cases (`internal/verification/verifier_gaps_test.go:15–112`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier_gaps_test.go#L15-L112) cover malformed syntax and some types, and [`TestVerifyWithRetry_WhenVerificationErrors_ShouldFailClosed` and nil-client companion (`internal/verification/verifier_failclosed_test.go:185–227`)](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/verification/verifier_failclosed_test.go#L185-L227) cover outages. Extend rather than discard them.

Measure invalid-reply classification, grounded-defect precision/recall, false vetoes, unsupported-citation rate, calibration by confidence bin, and worker reruns caused solely by judge failure. Separate unavailability from incorrect verdicts. An unresolved task is not scored as successfully verified.

### Complete-system comparison

Use ordinary production entry paths with normal CodeDOM/world context and permissions. Pair task repositories and initial revisions across current CodeNERD and proposed variants; also include a same-model harness baseline with comparable tools, verification access, retries and total budget. Compare solved-and-independently-verified tasks, regressions, unresolved tasks, end-to-end token/cost distributions and latency—not narrative quality alone.

Use hidden acceptance checks and expert review independent of the generating/judging model. Split by repository/task family so calibration examples do not leak into held-out evaluation. Include multi-file tasks, baseline-broken repositories, importer failures, language/toolchain variation, and concurrency cases. Report paired confidence intervals, all failure categories, and cost-quality Pareto curves. A no-judge ablation measures the judge's incremental benefit; it is an experiment, not a recommendation to remove gates.

Scale by changed dependency surface and evidence volume as well as task count. Record when packet limits cause abstention. Wins on one language, benchmark or model are not universal harness superiority.

## 7. Primary literature and applicability limits

- Zheng et al., [Judging LLM-as-a-Judge with MT-Bench and Chatbot Arena](https://arxiv.org/abs/2306.05685), studies position, verbosity and self-enhancement biases. It supports testing judge sensitivity, not assuming its conversational preference agreement proves code correctness. This proposal applies that caution to text-only verification.
- Jimenez et al., [SWE-bench](https://arxiv.org/abs/2310.06770), provides repository-level issue-resolution tasks that require coordinated changes. It motivates evaluating whole systems on actual code outcomes; its original Python scope is not evidence of performance across all CodeNERD languages.
- The [Go JSON API](https://pkg.go.dev/encoding/json#Unmarshal) supplies the decoding semantics underlying the ingress finding. The particular policy consequence above is an inference from the pinned CodeNERD source, not an externally reported exploit or an executed reproduction.

## 8. Checkable handoff

Before implementation, reconcile these findings into the verification/observation/session architecture corpora rather than silently overwriting old specs. Proposed completion criteria:

- Invalid semantic payloads never become an advisory pass or a coding-worker retry.
- Judge receives labeled host evidence and does not fabricate provenance for omitted fields.
- Raw judgment, schema status, evidence applicability and kernel disposition remain separately inspectable.
- Existing non-/done, acceptance, permission and final-edit closure invariants remain intact.
- A grounded retry names an actual applicable evidence item.
- Production-path held-out evaluation reports complete costs and quality trade-offs; no efficiency claim precedes measurement.

No code or tests were changed by this document.

## As built (2026-10-27)

**Implemented:**

- Reply contract `judge/v2`: `parseJudgment`, `rejectDuplicateKeys`, `Judgment`, `JudgeStatus` (`internal/verification/judge_contract.go`). The reply must be a single non-null object with no unknown or duplicate keys and no trailing value. `contract`, `status`, `confidence` and `explanation` are required, and presence is tracked apart from zero. Confidence must be in [0,1] and is refused rather than clamped. Violation and corrective types must be known. Contradictory combinations are refused, and so is a reply over `delegation.judge_reply_bytes`. A broken reply fails the delegation closed and the worker is never rerun.
- Evidence packet: `buildEvidencePacket`, `EvidencePacket`, `EvidenceItem` (`internal/verification/evidence_packet.go`). The packet holds the kernel verdict and missing obligations, stage, acceptance, each check with its own source, changed files as bounded diffs with before/after sha256 identities, findings, untested paths, notes, and the worker narrative labelled `reported`. It is bounded by `delegation.judge_evidence_bytes`. `observation.Return.Snapshot` now carries the closing evidence snapshot (`observedReturn`).
- Grounding: `Judgment.ground` resolves citations against the frozen packet and checks file freshness. A defect with no usable citation, or one that only labels a suspicion, becomes `insufficient_evidence`; `Claimed` and `Downgraded` keep what the judge said.
- Decision/retry algorithm in policy (`delegation.mg`): `judge_verdict` is /pass, /defect or /insufficient_evidence. New moves are `/acquire_evidence` (`judge_may_acquire`, bounded by `delegation.judge_evidence_rounds`; the host expands the requested elided items over the same packet) and `/unverifiable` (fail closed, no worker rerun). `delegation_attempt_failed` keeps judge limits out of retry/escalate. `TaskVerifier.judge` runs the rounds. A retry carries the grounded defect and the cited evidence IDs (`retryTask`, `VerificationResult.CitedEvidence`).
- Raw judgment vs disposition: `VerificationResult{Accepted, Reason, Judgment, Evidence}`, so the judgment is never edited. `task_verifications` gains `judge_contract`, `judge_claimed`, `judge_status` and `judge_explanation` (migration plus schema), and `StoreVerification` takes a `VerificationRecord`.
- JIT atoms `eval/delegation_judge/*` teach the packet and contract. The shipped prompt-corpus seed was refreshed for both atoms; their embeddings are null until re-embedded.

**Ruled differently:**

- The judge's evidence requests are limited to the packet's elided items (full post-change content of a changed file, re-checked fresh at expansion). No arbitrary path reads: anything else would need a new permission path.
- `CorrectiveAction.ShardHint` was removed, so a reply naming a shard is refused: who retries belongs to the persona, not the judge (F5).
- A defect citing only the worker's narrative is grounded. It is packet evidence, labelled reported, and for the review rubric it is the very thing being judged.

**Deferred:** toolchain/config/environment identities per check receipt (the observed `Verification` carries none, and none are invented), and the complete-system and calibration evaluation in section 6, which needs live models and held-out tasks. The cheap socket-level cases (empty/null/missing-field/duplicate/out-of-range replies, omitted vs genuine low confidence, stale citation after a concurrent edit, evidence acquisition) are implemented as tests in `internal/verification/*_test.go`.

## Follow-up 2026-10-28 Reply member identity

**Decision:** preserve the implemented `judge/v2` contract and kernel-owned disposition. Make the ingress validator and typed decoder agree on what constitutes one decision-bearing member. Today the duplicate-name walk compares exact decoded JSON strings, but the subsequent `encoding/json` v1 struct decoder also accepts case-insensitive names. Two different names can therefore write the same field, changing a judgment according to their order instead of rejecting the reply as contract-invalid.

This is a narrow residual contract defect in the as-built implementation above, not a reason to reopen the completed judge redesign. Exact schema names and parser-consistent duplicate handling are established techniques; the finding is their specific mismatch on this production boundary, not a claim of a new parsing technique. It is distinct from stale applicability, prefix-only evidence expansion and request-budget accounting in [verification applicability and progressive evidence](https://github.com/theRebelliousNerd/codenerd/blob/c4539f041ddee0ce15cbda02771af7422cdf053e/Docs/audits/verification-applicability-and-progressive-evidence.md).

**Source freeze:** rechecked at approximately 10:45 UTC, main was `356f65e0e6bb639adaa90ab36212daa714114871`, `feat/conversation-brief` was `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`, and `codenerd-research-docs` was `c4539f041ddee0ce15cbda02771af7422cdf053e`. Main and the feature branch both carry `internal/verification/judge_contract.go` blob `c309bb1176cf7247e0b1af288fbdf40ff3cd315f`. Their path histories identify the same latest change, [judge/v2 implementation commit e4e4d4a](https://github.com/theRebelliousNerd/codenerd/commit/e4e4d4ab554c661ffc0bccf4c74ef6a4121b17d3). No open pull request was returned by the repository's open-PR search. The research branch does not carry the new source file; it must not be used as evidence that main lacks the implementation.

**Verification limit:** static source and official standard-library documentation only. No codeNERD runtime, test, benchmark, live-model inference, failure injection, code change or GitHub write was performed. The payloads below are source-derived witnesses, not executed reproductions or incidents. Frequency in natural model output is unknown.

### Preserve the completed work

Main's [as-built reconciliation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/verification-evidence-and-judge-contracts.md#L192-L209) records required fields, valid statuses and confidence, bounded replies, exact duplicate-key checks, contradictory-combination checks, host evidence packets and policy-derived acquisition. Those paths exist. In particular, `parseJudgment` rejects missing/null required fields and invalid combinations, while `judge` returns unavailable when parsing fails; this report does not classify those protections as absent. [Parser](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/judge_contract.go#L105-L196), [judge call](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L328-L359).

The governing requirement is already in the JIT contract: use the named keys once. The proposal is to enforce that existing requirement at ingress, not to teach the model a new protocol. [Contract atom](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms/eval/delegation_judge.yaml#L98-L113). This serves the root North Star's separation between model creativity and deterministic executive facts; it leaves the executive decision in Mangle. [Root guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#L5-L27).

### Two stages use different member identities

The `judgeWire` shape defines lower-case JSON tags including `status`, `confidence` and `corrective_action`. `parseJudgment` first invokes `rejectDuplicateKeys`, then creates an `encoding/json` decoder, calls `DisallowUnknownFields`, and decodes into that shape. [Wire shape and decode order](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/judge_contract.go#L90-L124).

The duplicate walk keeps a fresh `map[string]bool` per object. It obtains each decoded member name with `Token`, tests `seen[key]`, and records that exact string. `status` and `Status` are therefore distinct to the walk. JSON escape spellings of the *same decoded string* are already handled by token decoding; they are not the gap. [Exact-key walk](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/judge_contract.go#L238-L280).

The repository declares Go 1.27.3. Its source imports the v1 `encoding/json` package, whose official documentation preserves case-insensitive matching of incoming object names to struct fields. Consequently `Status` is not an unknown field even with `DisallowUnknownFields`: it names the same destination as `status`. The v1 compatibility contract also permits duplicate object names and preserves legacy merging behavior. [Module toolchain](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/go.mod#L1-L3), [Go 1.27.3 JSON contract](https://pkg.go.dev/encoding/json@go1.27.3#Unmarshal).

The precise defect is a **semantic field collision**, not a claim that JSON itself makes those two strings the same member name. A wire object can satisfy exact-name uniqueness while assigning one contract field twice.

### Source derived acceptance witness

Consider this unexecuted reply:

```json
{
  "contract": "judge/v2",
  "status": "insufficient_evidence",
  "Status": "pass",
  "confidence": 0.9,
  "explanation": "Assessment of the supplied packet"
}
```

The exact-key walk finds no repeated string. Both status names map to `judgeWire.Status`; the later value is the pass presented to subsequent validation. Required fields exist, confidence is in range, and the decoded pass carries none of the forbidden auxiliary fields. `Judgment.Claimed` and `Status` are initialized from that decoded status, so they do not retain both assignments. [Construction and pass validation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/judge_contract.go#L162-L196).

On the normal delegated-mutation path, the TUI calls `VerifyWithRetry`. The worker must still return `/done`; the judge cannot manufacture the worker's mechanical completion. `judge` then parses, grounds and asserts the resulting pass. The delegation policy accepts a done attempt when neither judge rejection nor unresolved evidence derives. The verifier sets `Accepted=true`, and the TUI displays Accepted. [Production caller](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L455-L528), [judge publication](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L337-L385), [acceptance rules](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/delegation.mg#L378-L423), [display](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers.go#L329-L373).

Swapping the two status assignments instead leaves insufficient evidence and follows the unresolved path. The contract-invalid ambiguity is thus decision-bearing. Neither reading is the intended outcome: the existing invalid-reply policy should apply before any judgment fact is published.

Other field identities share the same seam. For example, `confidence` and `Confidence` can supply different values to one confidence field; a grounded defect could move across a nonzero rejection threshold. That threshold effect is conditional on configuration, not an assertion about the default threshold. Repeating a name variant inside `corrective_action` can change the advice carried to a worker, but it does not directly run a tool: `retryTask` renders corrective action as advice. [Confidence policy](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/delegation.mg#L380-L383), [retry advice](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L482-L522).

### Proposed contract

Prefer exact decoded member names for this versioned protocol. At each schema object, validate names against that object's canonical allowed set before projecting into a Go struct. Keep exact duplicate detection after JSON string unescaping. A root-only check is insufficient because `corrective_action` has its own members. Reject aliases rather than silently lower-casing and choosing one value.

An alternative is explicitly supported aliases with a unique destination-field identity: map each admitted alias to its one canonical field and reject a second assignment even if its spelling differs. That is more compatibility surface and needs a versioned reason; the current atom already asks for exact keys. A decoder migration may supply useful strictness, but changing JSON packages alone is not an acceptance criterion. Unknown-name, duplicate-name, required-field, null, trailing-value and nested-object semantics must still be shown to match the intended contract.

Any refusal follows today's unavailable-verification route and never triggers a worker rerun just to repair a malformed judge reply. Do not recover by selecting the first value, selecting the last, voting between statuses, or scanning the explanation for pass/fail words. The Go boundary establishes one valid judgment; Mangle retains acceptance, retry and escalation authority.

### Checkable exit and cost

Add one scoped contract gap to the verification specification, preserving existing closed gaps and citing the current parser seam. Its exit is:

1. Canonical valid pass, defect and insufficient-evidence replies retain their present behavior.
2. A mixed-case alias by itself is refused under an exact-name contract; aliases combined with a canonical name are refused in both orders.
3. Exercise every decision-bearing member, optional lists that can be erased or replaced, and nested corrective-action fields. A single escaped spelling that decodes to a canonical name remains allowed; canonical and escaped spellings together that decode to the same name are refused as a duplicate.
4. A refused reply publishes no new `judge_verdict`, cannot reach Accepted, and does not spawn a second worker for a judge-format limitation. Cover both an invalid first reply and an invalid reply after a valid insufficient-evidence acquisition round: an earlier round's fact may remain until delegation cleanup, but the refused reply must not replace it or turn it into acceptance.
5. Drive the normal TUI delegation path with world-model and CodeDOM context enabled, and check both internal disposition and user-visible output. Parser cases alone do not establish production wiring.
6. Preserve existing missing-field, duplicate-key, stale-citation, insufficient-evidence and non-done-worker behavior.

The inspected rejection table currently covers identical `status` keys and identical nested `type` keys, not case variants. This is a statement about that inspected table, not a claim to have executed the test suite. [Existing contract cases](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/judge_contract_test.go#L15-L66).

No extra model call or wider evidence permission is required by this correction. The validator already walks bounded replies; the proposed work adds schema-aware name validation. Measure parser overhead and natural-output refusal rates rather than claiming an unmeasured efficiency gain. Count an invalid reply as unavailable, not a failed worker or a verified completion. A rise in correct refusals can initially reduce reported acceptance while improving the truthfulness of that metric.

The research stopping point is this source-grounded contract refinement. This appendix qualifies the as-built no-duplicate-keys claim without reopening the completed judge redesign or creating a second verification program.
