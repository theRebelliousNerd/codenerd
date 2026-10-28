---
doc-class: deep-dive
subsystem: verification
implementation-status: partial
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
