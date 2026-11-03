# Recurse finding-census completeness

## Finding and evidence boundary

At public main `1a179efba7deb5c6149aea1da4970e66f92592b5`, ordinary recurse uses a work-selection list capped at **50 findings per gate invocation** as if it were the complete diagnostic census for two acceptance questions: whether a failing gate gained findings, and whether the selected finding disappeared. The cap preserves useful bounded work, but the returned slice has no completeness or saturation status. The ratchet receives its length and a membership test, not the limits of those observations. [Cap][cap] [Collector][collector] [Consumer][target] [Policy projection][policy-input]

Consequently:

- A comparable failing check with 50 distinct findings before and 51 or more afterward can contribute `fail,50 → fail,50`, failing to veto a real increase.
- A selected finding still present in the raw after-output can be omitted from the retained first 50, causing the outer target predicate to say resolved. Changing output order alone can change which findings are visible at this boundary.

The first is a missing regression veto; the second is an unsupported target-absence conclusion. **Neither converts a failing process into a passing process.** Independent campaign acceptance, other gates, metrics, ownership, permissions and optional error-free checks can prevent adoption. The report distinguishes a production-reachable faulty observation from the additional conditions required for an incorrect keep; it does not claim an observed bad commit or model behavior.

**Status:** static source-derived finding, inspected 2026-11-03. No application, model, gate, test, benchmark or reproduction was executed. Existing tests were read only. All CodeNERD references pin the revision above; only this scratch Markdown document was written.

## Contract at issue

The intended ordinary ratchet permits incremental work on an already failing baseline. Its comments explicitly allow equal-count replacement of findings because repairing one compiler problem can expose another. This report does not call that policy a defect or recommend requiring every existing gate to become green. [Ratchet contract][ratchet]

The necessary narrower contract is:

> A bounded list used to choose repair work must not establish a complete failure count or prove a target absent unless the host has independently established those properties over the required measurement.

A fresh process, unchanged command and matching workspace are necessary but insufficient: the parser can still discard records from the completed result. A gate's failure status is valid evidence; its retained diagnostic prefix is only partial evidence of which failures exist.

## Production trace

| Boundary | Current behavior |
|---|---|
| Normal entry | CLI and chat invoke `RunRecurseCycles`, constructing ordinary attempt campaigns and returning the orchestrator's retained owned writes. [CLI][cli] [Chat][chat] |
| Workspace gate | Project `nerd.md` can declare build/test/lint commands and additional gates. This is not confined to Go or to optional `--gates` measurements. [Declaration][declaration] |
| Host execution | `gates.Run` retains argv, process exit and bounded output; launch/cancellation failures are unverified. [Runner][runner] |
| Finding collection | `Findings` ignores pass/unverified results, visits the output and calls an `add` closure. That closure immediately returns once the output slice reaches 50; otherwise it normalizes identity, deduplicates and appends. No completeness field is returned. [Collector][collector] |
| Measurement storage | `recurseRun.measure` calls `gates.Findings`, attributes retained findings and stores them in `gateRun.findings`. Mission snapshots establish source freshness, not enumeration completeness. [Measurement][measure] |
| Selected attempt | The ordinary finding path copies the reporting gate's command, exit codes, environment and target key into the attempt. [Attempt][attempt] |
| After measurement | Following a changed attempt, applicable node gates and relevant workspace gates run again. Workspace build/lint gates are ordinarily included in the cycle ratchet. [Remeasurement][remeasure] [Policy kinds][ratchet] |
| Projection | `BeforeCount` and `AfterCount` are slice lengths. For the ordinary target, `TargetOK` means the result is not unverified and the selected ID is absent from that slice. [Projection][target] |
| Decision | The host emits `recurse_ratchet_gate` facts without completeness. A failing gate worsens by count only when after exceeds before; a resolved target can satisfy cycle success. [Facts][policy-input] [Rules][ratchet] |
| Adoption | If the independent predicates permit `ratchetKeep`, the host commits the changed paths and updates baseline state. [Commit][keep] |

Generic positioned diagnostic lines participate: `file:line[:column]: message` is recognized through the stream parser and forwarded to the same capped callback. Distinct target paths keep distinct identities even where message normalization collapses numbers. A conditional fixture can therefore use ordinary concise diagnostics with different file paths; it need not rely on malformed output, duplicate findings or megabytes of text. [Diagnostic parser][diagnostic] [Callback][parser-callback] [Identity][identity]

## Two discriminating conditional witnesses

### A. An independently successful repair, with a saturated secondary lint check

Use one workspace and the same declared command, environment and scope on both sides. All output fits below the runner's byte limit.

1. Gate A reports the selected defect. Gate B is a workspace-scoped lint gate with exactly 50 distinct, recognized diagnostics. Both are measured normally.
2. An authorized attempt repairs A, passes its actual campaign acceptance command and leaves valid owned writes. Its change also introduces one additional B diagnostic, without deleting tests or worsening any other independently measured condition.
3. A's after-check passes. B's after-check exits nonzero and emits 51 distinct diagnostics.
4. B's parser returns 50 on both sides. The outer ratchet receives `/fail, /fail, 50, 50`; its count-regression rule does not fire.
5. A resolves the selected target. If no other veto applies, the ordinary keep rule permits adoption despite B's increased diagnostic census.

This is deliberately a **two-gate** witness. It does not require bypassing A's acceptance command, changing B's command to emit fewer lines, modifying the oracle, disabling normal verification, or persuading a model to claim success. The conditional assumption is that the normal executor produces and retains a legitimate A repair with the B regression; no actual model has been run to establish its frequency.

A custom project lint check can measure a constraint that independent compiler/tests do not check. An optional error-free check may catch the same regression, but it is neither universal nor required for the ordinary CLI/chat path. The ordinary mechanism must state its own evidence limit even when another layer sometimes supplies a veto.

### B. A still-open target moves outside the first 50

Let a selected target T be the 50th unique retained diagnostic in the baseline. In the after-output, place 50 other unique diagnostics before the same T, with T unchanged and still present. Both commands fail normally. The returned after slice contains no T.

At the ordinary target projection, `!hasFinding(after, T)` is true and the command is not unverified; therefore `TargetOK` becomes true. Its actual meaning is “not among the retained first 50,” not “absent from the completed diagnostic stream.” [Target absence][target]

This demonstrates the projection defect without claiming the complete campaign necessarily succeeds. The selected gate's acceptance command can still fail and trigger remediation, and failed task attempts are undone. Those are real countercontrols. [Acceptance contract][campaign-witness] [Acceptance construction][acceptance] [Authored controls][acceptance-controls] [Failed-task undo][failed-undo]

The outer driver also records an executor error in the journal detail rather than using that error itself as a ratchet veto. The callback may return retained writes from earlier successful task attempts even when later campaign acceptance is blocked. Whether a particular schedule leaves eligible changes requires joined-path evaluation; cancellation instead follows the explicit owned-undo branch. Do not infer a bypass of all campaign verification from the isolated negative-membership calculation. [Returned writes][owned] [Executor boundary][execerr] [Error detail and commit][keep]

Output ordering is not a semantic assertion about absence. New diagnostics can displace T in a deterministic ordered report. Parallel/interlaced reporting can create additional reorderings; Go's documented JSON test stream explicitly permits events from different tests to interlace and supplies package/test identity for that reason. The witness does not depend on that Go behavior, but an implementation must not assume all supported runners enumerate findings in one stable order. [Go test event format](https://pkg.go.dev/cmd/test2json#hdr-Output_Format)

## The byte budget is a separate incompleteness source

The per-run output buffer keeps the first and last halves of a 1 MiB budget and inserts a dropped-byte notice in between. This is honest presentation-level truncation. However, `Findings` parses that already reduced string, and neither `Result` nor the returned findings carry a typed completeness disposition to the ratchet. A diagnostic omitted from the middle can therefore be absent even below the 50-finding cap. [Result shape][buffer] [Buffer behavior][buffer-impl]

The main witnesses above deliberately stay below 1 MiB to isolate the 50-entry mechanism. Removing or raising the finding cap alone would not solve the byte-loss case. Conversely, retaining all raw output would not solve the existing callback cap.

The system should distinguish process completion, capture completeness, parser coverage, retained work-list size and comparable census. A dropped-byte marker in prose is not the same as a policy fact requiring caution. Parsing an incomplete JSON stream can also lose records even if the remaining text is useful for a human; no claim is made here that every truncated report produces false acceptance.

## Durability and mission consequences

In mission mode, the before and after ordinary checks can have valid source snapshots. If an attempt is kept and the ordinary target appears resolved with those source identities, the mission path can set its witness to `/resolved`. Mission policy then derives `/supported_gate_deficit` from the kept, resolved, changed-source observation. That source binding does not independently test whether the selected diagnostic was beyond the parser cap. [Mission support branch][target] [Mission rule][mission-rule]

For witness A, the selected A deficit really is repaired; the issue is missing evidence that B got worse, not necessarily a false claim about A. For witness B, a supported-deficit receipt would additionally require that the attempt reach keep despite the independent acceptance controls. That stronger result remains a conditional consequence, not an observed incident.

The durable check sidecar retains exit, command, status and the runner's output. With small complete raw output, later inspection can recover the omitted diagnostic and challenge the verdict. With byte truncation, the sidecar retains only the already bounded string. Neither path persists a dedicated ordinary-finding completeness field because none exists in the upstream projection. [Sidecar][mission-record] [Mission settlement][mission-finish]

This matters for future requirement evidence: a receipt should preserve what its observation establishes and what was not enumerated. A durable incorrect absence claim becomes harder to notice once the attempt campaign is released, even when raw evidence remains recoverable. This report does not equate historical gate-deficit support with arbitrary strategic requirement satisfaction.

## Existing safeguards and limits

- **Pass remains pass, fail remains fail.** The parser cap does not alter process status. Pass-to-fail and unverified-after transitions remain independent regression predicates. [Runner][runner] [Rules][ratchet]
- **Small comparable censuses:** when all relevant diagnostics are captured and retained, this particular cap loss does not arise. Exactly 50 real findings can be complete; size 50 alone is not proof of overflow.
- **Duplicates:** the cap is over retained identities, not simply lines. A valid evaluation must use more than 50 distinct IDs and separately test repeated duplicates. [Collector][collector]
- **Multiple paths:** a collapsed multi-directory node can append several capped invocations and exceed 50 total. The cap is per invocation, not a universal maximum for every `gateRun`. [Measurement][measure]
- **Scope:** focused filtering happens after finding collection. No scope-mismatch is needed for these witnesses; use one fixed scope to distinguish census loss from prior domain issues.
- **Independent controls:** forbidden paths, mission scope, pinned oracle inputs, ownership/postimage checks, metric regression and optional error-free comparisons remain active. [Guards][guards] [Metrics][metrics]
- **Campaign acceptance:** a failing selected gate is a genuine blocker/remediation opportunity. The two-gate witness avoids assuming it disappears. Existing test definitions assert that behavior; they were not run. [Authored acceptance controls][acceptance-controls]
- **No malicious actor required:** a large lint backlog, changed diagnostic order or broader regression can produce the issue. No exploit, unauthorized command execution or permission escalation is established.

Severity is a **conditional correctness and assurance risk**: ordinary keep decisions can lack a required regression veto, and target evidence can overstate absence. Likelihood and actual user impact are unmeasured. The issue is not merely that a prompt shows fewer diagnostics; the omitted information influences policy inputs.

## Prior-art and duplication screen

The exact symbol `MaxFindingsPerRun` appeared in the collector and its test definitions, not in the searched documentation. Bounded searches also covered `recurse_ratchet_gate`, findings completeness, saturated/fifty findings, mission storage, the current audit queue and relevant recurse/gate reports. No exact report of this ordinary capped-census→count/absence path was found. This is a bounded screen, not a claim that every historical discussion was inspected.

Important adjacent work already exists:

- **Gate comparison domain admission** identifies incompatible optional error-free measurements and opaque-failure counts, and proposes enumeration-completeness metadata. Credit that principle rather than rediscovering it. The concrete producer here is ordinary `gates.Findings` dropping recognized records at 50, with a separate target-absence consumer. No changed scope or opaque report is necessary. [Prior domain audit][prior-domain]
- **Scoped recurse addendum** discusses optional snapshot restriction/overlay and opaque failure retention. The present witnesses use fresh ordinary measurements of one fixed scope. [Prior scope addendum][prior-scope]
- **Gate subsumption** concerns work suppression between distinct workspace and node checks. The present case retains the gate and runs it again; its diagnostic census is incomplete. [Prior subsumption audit][prior-subsumption]
- **Mission error-free witness receipts** concerns missing focused-group source evidence. This report uses ordinary target keys and can have valid source snapshots; the missing property is enumeration completeness. [Prior mission audit][prior-mission]

The existing `TestFindings_BoundedPerRun` constructs 150 distinct failed tests and expects exactly the cap. It establishes that bounding is intentional. It does not assert a completeness signal or exercise capped results through the ratchet. Preserve bounded selection while repairing the downstream inference contract. [Existing cap test][cap-test]

## Proposed bounded repair contract

This section is a design proposal, not an implementation or evaluated fix.

1. **Separate work selection from evidence.** Keep the bounded actionable list. Produce a distinct host-owned observation containing capture disposition, parser disposition, exact or lower-bound distinct count, retained count, omitted/saturation information and measurement identity.
2. **Answer the target question independently.** While processing the complete stream, track whether the selected stable ID is observed. Absence requires completed relevant enumeration, or a separately admitted targeted witness. A positive observed match remains useful even when the rest is partial.
3. **Preserve count semantics.** An exact count can be compared under the existing equal-count replacement policy. Two lower bounds of 50 do not prove equality or nonincrease. Explicit unknown/incomparable facts should prevent an unqualified keep conclusion when this gate's regression check is required.
4. **Propagate completeness through composition.** Preserve it across per-directory aggregation, scope filtering, baseline state, policy facts and mission sidecars. A source hash is not a substitute. Legacy records without metadata remain readable but cannot be silently upgraded to complete.
5. **Parse before display truncation where practical.** A bounded UI/prompt excerpt can coexist with streaming diagnostic extraction or a complete durable artifact. If raw capture, parsing or persistence hits a limit, report that loss and abstain from unsupported absence.
6. **Retain protections and host authority.** Do not fix the issue by permitting unlimited output, removing verification, changing failed exits to pass, letting the model attest completeness, or hiding the saturated gate from the comparison. Mangle should decide the disposition from trustworthy host measurements.
7. **Make recovery specific.** A remeasurement can narrow to the same selected target only under an explicit target-witness contract; it does not replace the whole gate's nonregression obligation. Repeatedly rerunning a deterministically capped full command does not make missing evidence complete.

A conservative first version can mark uncertain censuses and block only claims depending on them. Detecting actual overflow is better than treating every list of length 50 as incomplete. Exact distinct counts require accounting for deduplication beyond the retained list; marking every post-cap callback as an extra unique finding would overcount duplicates.

Reiter's closed-world work formalizes the distinction between proving a statement and treating absence of a proof as its negation. The engineering inference here is that omission by a known prefix cap is not a valid closure boundary for target absence. This is a defect in supplied observation semantics, not an accusation that Mangle's negation implementation is wrong. [Reiter, On Closed World Data Bases](https://www.cs.ubc.ca/tr/1977/tr-77-16)

Cousot and Cousot's abstract-interpretation framework supplies a useful design analogy: an abstraction should preserve the properties it claims to establish, while allowing explicit imprecision. A capped prefix is an adequate abstraction for “some useful repair candidates,” but not for exact cardinality or arbitrary nonmembership. This proposal does not claim a formal proof for CodeNERD. [Abstract interpretation](https://www.di.ens.fr/~cousot/COUSOTpapers/POPL77.shtml)

## Costs and failure modes

Keeping only K=50 rich finding records bounds selected-work payload, but counting D distinct findings exactly can require O(D) identity storage even with compact hashes; streaming over N bytes remains O(N) parse work. Complete diagnostic artifacts add I/O, quota, retention and recovery requirements. A bounded identity store that fills must expose unknown cardinality rather than wrap, overwrite or pretend its count is final.

Target-presence tracking can be constant additional state for a known target, but its sound absence result still depends on full relevant capture and correct parsing. Extra targeted rechecks cost subprocess startup and tool execution and need identity/freshness validation. Conservative abstention can stall repositories with large existing backlogs; that operational cost should be measured, not concealed by weakening the contract.

Other risks include mistaking duplicate callbacks for overflow, mismatched normalization across counts and IDs, schema migration that invents completeness, malformed/truncated machine records, nondeterministic ordering, falsely claiming that a parser recognizes every possible failure, and losing completeness during collapsed-node aggregation.

No LLM call is required to recognize this boundary or preserve metadata. Whether the correction saves later investigation or model turns is unmeasured. Charge additional parser CPU, identity memory, output storage, kernel facts, remeasurement and owner review against any claimed benefit.

## Fair evaluation plan — unrun

First compare policy inputs from fixed recorded outputs; then use authorized production-path fixtures through normal CLI/chat campaign execution. Do not present an injected `Execute` stub as proof that the ordinary worker passes every inner gate.

Required cases:

| Case | Expected observation/decision |
|---|---|
| 49, 50 and 51 distinct diagnostics | Exact completeness where known; truthful overflow beyond the retained list. |
| 50 before, 51 or 150 after; gate fails both times | No unsupported nonincrease from two retained counts of 50. |
| 60 before, 51 after | Preserve uncertainty unless exact counts are available; do not label every saturated result a regression. |
| Hundreds of duplicate lines, fewer than 50 identities | Preserve deduplication; no invented distinct count. |
| T retained before, T moved beyond cap after | T remains observed/open when the full stream contains it. |
| T genuinely absent after complete enumeration | Legitimate resolution remains possible. |
| Genuine A fix plus saturated B regression | Normal acceptance of A, but outer adoption blocked or unresolved for B. |
| Independent gate catches B | Existing veto preserved; report which layer rejected. |
| Output below/at/above byte budget; diagnostic only in discarded middle | Capture limits explicit; no unsupported absence. |
| Go JSON, text diagnostics, pytest and Rust failures | Parser-specific completeness exercised without assuming one protocol fits all. |
| Collapsed nodes and focused scopes | Completeness composes correctly; repeated identities have declared semantics. |
| Cancellation, launch failure, no parsed failure, successful exit | Existing status handling preserved; partialness is not a fake pass. |
| Mission snapshot/sidecar round trip | Durable disposition and evidence limits survive serialization. |

Use identical source fixtures, command identities, budgets and raw outputs for baseline and proposed variants. A separate full-stream reference census should establish expected IDs/counts; do not use the capped implementation as its own oracle. Include legitimate equal-count replacement controls so the fix does not silently change incremental-repair policy.

Report false absence, missed count regression, false unresolved/false regression, actual adopted outcomes, retained evidence, CPU/wall time, peak memory, artifact bytes and additional commands. Separate collector/policy correctness from model task success. Preserve all failed and abandoned attempts in denominators.

**Completion condition:** the system can still choose a small useful repair list, but neither its ratchet nor its durable mission evidence can mistake an omitted diagnostic for a resolved one or two saturated prefixes for proof that the underlying gate did not worsen.

## Pinned source references

[cap]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/findings.go#L59-L62
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/findings.go#L98-L175
[diagnostic]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/findings.go#L331-L370
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/findings.go#L196-L237
[parser-callback]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/findings.go#L582-L599
[cap-test]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/findings_test.go#L137-L157
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L708-L777
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1057-L1084
[remeasure]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1208-L1232
[guards]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1234-L1280
[target]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1297-L1333
[keep]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1336-L1382
[policy-input]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_policy.go#L199-L259
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/core/defaults/policy/recurse.mg#L258-L329
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/core/defaults/policy/recurse.mg#L356-L398
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/cmd/nerd/cmd_campaign_recurse.go#L225-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/cmd/nerd/chat/campaign_recurse.go#L218-L267
[declaration]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/detect.go#L15-L91
[campaign-witness]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_attempt.go#L34-L54
[acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_attempt.go#L95-L113
[acceptance-controls]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_attempt_acceptance_test.go#L127-L177
[owned]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/orchestrator_attempt_writes.go#L109-L129
[failed-undo]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/orchestrator_attempt_writes.go#L158-L183
[execerr]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1168-L1205
[buffer]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/run.go#L15-L41
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/run.go#L43-L97
[buffer-impl]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/run.go#L99-L131
[mission-rule]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/core/defaults/policy/mission.mg#L24-L31
[mission-record]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_mission_storage.go#L80-L121
[mission-finish]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_mission.go#L319-L367
[prior-domain]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/gate-comparison-domain-admission.md#L47-L88
[prior-scope]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/gate-scoped-recurse-current-main-addendum.md#L1-L38
[prior-subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/recurse-gate-subsumption.md#L1-L40
[prior-mission]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/mission-errorfree-witness-receipts.md#L1-L51

