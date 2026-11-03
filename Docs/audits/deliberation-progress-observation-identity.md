---
doc-class: deep-dive
subsystem: deliberation-progress
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: 033df25a56e6064c0be7da7a9f413842d7354f50
supersedes: []
---

# Deliberation progress must distinguish newly inspected scopes

## Finding and evidence boundary

The deliberation conductor can classify successful investigation of a new workspace scope as no progress when its returned text equals an earlier result. `runProbes` executes the full challenge-supplied call, but its result record contains only the stable item key, tool name, output and error. `progressFingerprint` hashes those records and critical-item identities, without the probe's arguments. The full call is still retained in the challenge, but the stopping projection does not read it. [Probe execution][probes] [Fingerprint][fingerprint] [Record types][types]

For example, successful searches for the same absent symbol in three different existing directories return the same built-in grep text. If the same unresolved requirement keeps its stable item key, all three completed rounds have the same progress fingerprint. Under the defaults, the conductor increments the unchanged count to two and stops before round four. That fourth round would have been within the configured four-round allowance, and is the first opportunity for the roles to assess the third probe's result. [Negative grep][grep] [Counter update][counter] [Stop admission][stop] [Defaults][defaults]

The confirmed static finding is a **projection collision between different executed investigations**, with a conditional early-stop consequence. It is not an observed production failure, proof that every changed argument is useful progress, or a claim that the system must continue indefinitely. No incorrect downstream implementation, permission bypass, failed gate accepted as passing, or actual financial loss was demonstrated.

This report is pinned to public main [033df25a56e6064c0be7da7a9f413842d7354f50][pin], resolved November 3, 2026. Before freezing, main advanced to `978e61b0c2285cd40eca052e72afac87b1069a47`; the [complete one-commit comparison](https://github.com/theRebelliousNerd/codenerd/compare/033df25a56e6064c0be7da7a9f413842d7354f50...978e61b0c2285cd40eca052e72afac87b1069a47) changes only one unrelated audit-queue disposition, leaving the cited implementation unchanged. Public guidance, architecture, implementation, existing audits and authored tests were inspected. No repository program, build, test, model inference, gate, benchmark or failure injection was run. No implementation is included. The witnesses and evaluations below are unexecuted source-derived proposals.

## 1. Why this matters to Socratic requirements work

The repository's north star makes the model the creative center and Mangle the executive. It requires bounded context, recoverable evidence and explicit stall detection while work makes progress. The proposer is specifically told to keep candidate IDs and open-item keys stable across rewordings and to refine using measured probes. The challenger identifies data blockers and supplies exact read-only calls rather than questioning the user for missing facts. [Root guidance][guidance] [Proposer contract][proposer] [Challenger contract][challenger]

A requirements question can remain the same while its evidence coverage expands. “Have all three affected modules been checked for callers?” is one unresolved obligation. Negative results for module A, module B and module C contribute different facts even when their text is identical. Renaming the obligation after each search is neither necessary nor desirable. A progress detector must not require such renaming to recognize a materially different completed read.

The current design already has valuable safeguards:

- The first round is divergent; later rounds use the previous challenge and probes.
- Each proposed candidate receives exactly one validated assessment.
- Critical data items require a probe; critical design and values items are separate.
- Host execution goes through the guarded read registry.
- Mangle owns empirical selection, readiness, stop thresholds and values disposition.
- A stopped design is not labeled ready or executed as a completed implementation.
- The entire challenge, including the proposed call arguments, survives in the saved round.

Preserve all of these. The correction belongs at the measurement-to-policy boundary, not in a second agent framework, broader execution permissions or an arbitrary increase to limits. [Collector][collector] [Validation][validation] [Read adapter][read] [Policy][policy] [Persistence][persistence]

The session specification accurately describes its present fingerprint as stable critical-item identities plus observed outputs and errors. It also says a brief rewording alone must not reset the counter. This audit proposes refining that measurement contract to distinguish meaningful new evidence coverage from both repeated reads and prose churn. It does not claim the implementation contradicts the literal list of fields in that paragraph. [Session contract][spec]

## 2. Production route and exact loss

### 2.1 The conductor path is wired

The chat model builds a `deliberation.Service`, marshals frames for the registered roles and assigns `Probe: reads.Execute`. `/deliberate` and `/innovate` invoke that service with a design-only thread. A stopped outcome is rendered through `Outcome.Note`, which returns the thread's status rather than a completed design. [Chat construction][chat] [Design command][command] [Status rendering][note]

The recurse-mission constructor likewise assigns `Probe: read.Execute` to the same service. Mission preparation invokes `m.Deliberator.Run`, stores the thread identity/path, and maps outcomes other than proceed or ask-once to a stopped disposition. Thus the fingerprint is relevant to both ideation and the design stage of recurse. Mission write admission, source/vision checks, verification and promotion remain separate controls. This report does not assert that the early stop can defeat any of them. [Mission construction][mission-constructor] [Mission handoff][mission]

### 2.2 The exact invocation exists and is executed

`Item.Probe` is a complete `types.ToolCall`. A challenge includes that object in its candidate-bound item. Validation requires a non-nil probe for each critical data item, and the service selects an empirical candidate under policy. [Item shape][types] [Validation][validation] [Empirical rule][policy]

For the selected candidate, `runProbes` iterates critical data items, checks the call budget and invokes:

```go
result, e := s.Probe(ctx, *item.Probe)
p := ProbeResult{Key: item.Key, Tool: item.Probe.Name, Output: result}
```

Errors are recorded separately. The result is appended to the same round and saved. The full call's input is used by `ReadTools.Execute`, which passes `call.Name` and `call.Input` to the registry. The problem is not execution against the wrong input; it is the later comparison's omission of that input. [Probe execution][probes] [Read adapter][read]

### 2.3 The stop measurement ignores the saved invocation

`progressFingerprint` adds:

1. For every critical item: candidate ID, item key and kind.
2. For every round probe: the JSON encoding of `ProbeResult`.

It sorts these strings and hashes them. It does not include the item's `Probe`, selected candidate's source scope, candidate evidence, or another host-bound invocation descriptor. `ProbeResult` has no input or subject field. [Fingerprint][fingerprint] [Result shape][types]

The derived `deliberation_probe` receipt is also hashed from that same narrow result. Its fact includes a round number, but `progressFingerprint` does not consume that fact. Adding an always-changing round number to the progress hash would be the wrong repair: every repeated round would then appear new. [Receipt publication][probes] [Fingerprint][fingerprint]

This is a deterministic loss before SHA-256, not a conjecture about cryptographic hash collisions. The different executions are deliberately encoded as identical strings before hashing.

### 2.4 The omitted distinction controls whether the next round exists

After an empirical round, the service executes its probes, computes the fingerprint, increments `Thread.Unchanged` when it matches the previous fingerprint, and saves. At the beginning of the next iteration it asserts `deliberation_tick`, queries `deliberation_stop`, and returns a stopped outcome before calling `collectRound` if a stop is derived. [Counter update][counter] [Stop admission][stop]

The default settings are four rounds and two consecutive equal-fingerprint transitions. Policy derives no-progress when the unchanged count reaches its limit while the next round is still at or below the round cap. Consequently the false equivalence can remove a still-budgeted round; it is not merely an inaccurate diagnostic after the ordinary cap is exhausted. [Defaults][defaults] [Stop policy][stop-policy]

### 2.5 Retained evidence is real counterevidence, but does not repair this decision

The prior full round is supplied in `Frame.Previous`. Its challenge can identify the path associated with the previous conductor probe. The saved thread contains all rounds, not only the fingerprint. Accordingly, this report does **not** claim that conductor probe arguments are universally erased or unavailable to a model. [Previous-round replay][collector] [Persistence][persistence]

The new issue is narrower: this information is not consulted by the host's progress measurement, and the next role call can be stopped before it is allowed to interpret a newly obtained result. Improving only the model-visible read observations does not necessarily fix that executive decision.

## 3. Small source-derived witness, unrun

### Preconditions

Use one ordinary design-only thread, a stable workspace, successful storage and proof recording, short payloads, the existing read permissions, and the default four-round/two-repeat configuration. No consultation, truncation, concurrency, source change, malformed JSON or human question is needed.

The user's requirements identify exactly three small, readable and admitted module directories, `alpha`, `beta` and `gamma`. The design requires checking whether the exact deprecated symbol `LegacyHook` remains in each. In this hypothetical fixture it is absent from all three, and there are no ignored, secret, unreadable or failed-search files in the declared search domains.

Script the role results to isolate the host boundary rather than relying on a model to produce a particular exploration order. Each of rounds 1–3 contains two materially different, valid candidates:

- `c1`: preserve the API while changing the implementation, conditional on finishing the specified caller inspection. Its one critical item has stable key `caller-coverage`, kind `data`, and a concrete grep probe.
- `c2`: an alternative design with the same stable unresolved critical design item in each round. It is not ready or empirical.

The recommendation remains `c1`. Its success flag stays false while the data obligation is open. This is admissible for empirical selection: that rule requires critical data blockers and absence of non-data blockers, not a ready-success assertion. Every candidate has a brief, success test and exactly one assessment. Keep all non-probe parts of the fingerprint constant. [Validation][validation] [Selection rules][policy]

### Three real subjects, one projected result

Use the same pattern and normal successful grep parameters, changing only the requested existing directory:

| Completed round | Executed input | Relevant new fact | Result text |
|---|---|---|---|
| 1 | `grep`, pattern `LegacyHook`, path `alpha` | The admitted alpha domain was searched without a match. | `No matches found for pattern: LegacyHook` |
| 2 | `grep`, pattern `LegacyHook`, path `beta` | The admitted beta domain was searched without a match. | Same text |
| 3 | `grep`, pattern `LegacyHook`, path `gamma` | The admitted gamma domain was searched without a match. | Same text |

The built-in successful zero-match branch returns only the pattern, not the search path. The missing-root branch does include a path, but is explicitly excluded by this fixture. Positive results can carry file paths, but are not required for a useful investigation. [Grep source][grep]

Each resulting `ProbeResult` is therefore:

```json
{"key":"caller-coverage","tool":"grep","output":"No matches found for pattern: LegacyHook"}
```

The actual input remains different in `Challenge.Assessments[*].Items[*].Probe`, and a candidate's evidence may record the cumulative inspected domains. Neither affects the fingerprint. Keep stable IDs and keys exactly as the proposer contract requests. [Fingerprint][fingerprint] [Stable-key instruction][proposer]

### Counter and stopping trace

Starting from a new thread with an empty prior fingerprint:

| Point | Fingerprint relationship | Unchanged count | Consequence |
|---|---|---:|---|
| After round 1 and alpha probe | First fingerprint | 0 | Round 2 may start. |
| After round 2 and beta probe | Equal to round 1 | 1 | Round 3 may start. |
| After round 3 and gamma probe | Equal to round 2 | 2 | Three different domains have been investigated. |
| Before round 4 | Count reaches no-progress limit; 4 is within round cap | 2 | Policy stops before proposer/challenger invocation. |

Round 3's challenger requests the gamma probe before that host probe executes. The earliest normal round in which the roles can assess gamma's new result is round 4. A scripted fourth response could discharge `caller-coverage` and produce a cited ready design; the inspected service does not call it in this schedule. The report does not assert that a real model would necessarily reach that result or that symbol absence alone proves every aspect of the design correct. [Ordering][collector] [Probe/counter order][counter] [Pre-round stop][stop]

A control repeats the alpha search in all three rounds, under an unchanged workspace and query. That schedule genuinely repeats the same admitted observation. It produces the same fingerprints and stop trace. The current measurement cannot discriminate it from the expanding-coverage schedule.

### Limits and alternative successful paths

- A model can batch all three probes in one challenge, use a wider admitted search or obtain the necessary evidence in its own read loop. Those are valid paths that can avoid this witness. This report does not claim every task needs three sequential rounds.
- A changed output string can change the existing fingerprint even when the input is omitted. The confirmed collision is the equal-output case.
- Resolving a critical item changes the fingerprint's critical-item set. The issue occurs while the same overarching requirement legitimately remains unresolved until its final probe is assessed.
- Different keys or extra incidental observations can change the fingerprint. Requiring key churn or irrelevant output changes would be an unreliable workaround.
- The last probe and original intent remain persisted. The failure is a conditional early stop, not unrecoverable data deletion.
- A resource limit may correctly stop useful work. The defect is calling distinct completed investigation identical for the no-progress measurement, not the existence of a separate hard resource limit.

## 4. Nonduplication and portability scope

The pinned audit inventory, current queue and relevant deliberation/recurse reports were inspected. Targeted repository searches included `progressFingerprint`, `deliberation_no_progress` and deliberation/progress/probe identity. Search results were indexed at an earlier main revision, so implementation claims use the explicitly fetched pin rather than search freshness. No exact predecessor for this control-path witness was identified in that bounded screen; no exhaustive historical novelty is claimed. [Audit queue][queue]

- **Deliberation read invocation identity** concerns arguments omitted from the next model request inside `Agent`. That report explicitly distinguishes conductor probes because their full calls survive in the challenge. The present witness uses those conductor probes and does not require any model-visible ambiguity. The consumer losing identity is `progressFingerprint`, which can prevent the next model request entirely. Repairs may share a receipt type, but they are independently testable boundaries. [Earlier read report][read-audit]
- **Portable recurse deliberation and crucible** proposes stronger objection/evidence progress tracking generally. This report supplies a concrete implemented collision and default stopping trace, not another complete portable framework. [Portable proposal][portable]
- **Terminal-budget reports** concern fitting closing notes so a role can answer. This witness uses small successful responses and ample byte capacity. [Budget report][budget-audit]
- **Values admission, freshness and continuation ownership** concern a human-answer boundary. There is no human pause or answer in this witness. [Option report][option-audit] [Freshness report][freshness-audit] [Ownership report][owner-audit]
- **Selected review handoff** concerns important findings omitted after a ready decision. Here no ready decision is reached and the critical requirement remains visible. [Review handoff][review-audit]

The mechanism is language-neutral. Identical successful negative searches can occur in Go, Python, JavaScript/TypeScript, Rust or mixed repositories. That supports testing those environments; it does not establish universal portability, parser coverage, build-tool availability or acceptance-oracle completeness. The same general distinction also applies to domain-specific paged reads, but no such additional tool path is claimed verified here.

## 5. Proposed contract, not implemented

Update the session/deliberation specification first. Preserve bounded work and stable obligation identity, while separating three different measurements:

1. **Obligation change:** a candidate-bound critical requirement is introduced, refined, discharged or reopened with the relevant supporting evidence.
2. **Evidence acquisition:** a successfully completed, relevant read adds a previously unobserved scope or an admitted new source state.
3. **Presentation change:** wording, ordering, opaque call IDs, timestamps or repeated model advice changes without adding either of the above.

Neither a new descriptor nor a longer explanation proves that an obligation is resolved. The host records mechanical facts; Mangle derives whether those facts support continuation under the existing resource envelope. A model may propose relevance or resolution, but must not be able to reset the budget merely by inventing a novelty label.

### 5.1 Bind completed observations to their real invocation

For each executed conductor probe, preserve a typed association among:

- thread, candidate and stable item identity;
- tool name and effective non-secret subject/arguments necessary to interpret the result;
- completion/error state and any scope/completeness limitation;
- output identity and, when available, a host-supported source version or receipt.

The current saved challenge already holds the requested call. A minimal correction can join an executed result back to that exact selected item rather than duplicating all arguments into the model context. An execution-time descriptor is stronger when a tool normalizes or changes the effective query. Do not infer effective values that the tool does not report.

Include only probes that actually ran. Hashing every requested call would manufacture progress when the call budget prevented execution. Associate the receipt with the selected candidate, not any other candidate with a same-named item. Preserve consultation and direct probe origins separately if they share a carrier.

### 5.2 Define useful novelty without rewarding churn

Use a versioned, typed encoding for semantic identity. Do not make round number, provider call ID, request-map key order or a new timestamp sufficient to count a repeated read as new. Normalize only according to the tool's actual semantics and workspace policy. Distinct case-sensitive paths must not be merged by a blanket lowercase transform; aliases and platform-specific separators need a declared policy.

A newly observed exact scope can justify a bounded continuation opportunity without proving full coverage. Define whether overlapping reads add material coverage, how failed reads are treated, and how source changes invalidate earlier observations. Repeating a failed read can be allowed by a separate retry policy, but a fluctuating error string alone must not establish useful progress.

A simple invocation-aware fingerprint is an incremental option, not a complete solution. Comparing only the immediately preceding round can count alternating A/B reads as novel indefinitely until the hard cap. If the intended contract is task-level acquisition novelty, retain a bounded seen-evidence ledger or equivalent derived state across the thread. Its scope and retention are a design decision to make explicitly.

### 5.3 Keep source freshness, authority and completion separate

A descriptor proves which read was requested or executed, not that its result is current, exhaustive or correct. A digest proves identity under its chosen representation, not truth. Preserve missing/partial/failed evidence as such. Do not treat a successful negative search over a filtered domain as proof of absence outside that domain.

No change here authorizes mutation, broadens the read catalog, weakens mission verification or converts “investigation advanced” into “requirement satisfied.” Continue to stop truthfully at applicable round, tool, context, cost, time, cancellation and permission boundaries. A genuine values preference remains distinct from an empirical blocker.

### 5.4 Keep the correction in the existing architecture

Go can construct exact receipts and compute bounded comparison material. Mangle should retain the executive continuation/stop judgment. Add or refine JIT guidance only where the participant contract changes; do not introduce ad-hoc prompts or ask the user to answer repository facts that read tools can determine.

For compatibility, version the progress representation. A saved old fingerprint cannot be interpreted as if it were produced by the new descriptor. Either reconstruct from sufficiently complete retained executed records, with explicit limits, or mark comparison state unknown and follow a bounded migration rule. Never reset total budgets silently. No new storage subsystem or model call is intrinsically required for the minimal host-side association.

## 6. Failure modes to prevent

| Proposed repair risk | Required safeguard |
|---|---|
| Every fresh call ID counts as progress | Exclude transport-only IDs from semantic novelty. |
| New key spelling defeats stalls | Keep stable requirement identity; distinguish renaming from resolution. |
| Requested but skipped probes count | Use completed execution receipts, not proposal presence. |
| Different paths alias accidentally | Use tool/workspace-aware identity and test case-sensitive and case-insensitive hosts. |
| Argument order changes identity | Canonicalize typed mappings without flattening meaningful values. |
| Changed arguments are irrelevant busywork | Bind admissible acquisition to the unresolved requirement and retain global bounds. |
| Alternating the same two reads resets forever | Define thread-level seen evidence; preserve round/time/cost limits. |
| Error text jitter or consultation paraphrase resets progress | Separate result identity, semantic observation and useful advancement. |
| Full argument echo leaks secrets or bloats context | Use bounded safe descriptors or guarded references; make redaction limitations explicit. |
| Source edits reuse an old negative result | Keep source-validity admission separate from invocation identity. |
| Extra metadata breaks terminal-context capacity | Prefer host-only joins when adequate; account for any model-visible bytes and preserve budget tests. |
| The new metric creates false readiness | Continue to require the independent ready assessment and normal execution/verification admission. |

These are proposed design obligations, not claims that all are currently observable defects in this subsystem.

## 7. Evaluation plan, all unrun

### A. Deterministic projection and stopping

Use scripted role outputs with the real production read adapter and kernel. Inspect outgoing frames and saved records, not merely a hand-built hash helper.

1. Construct the three-directory negative-search witness. Confirm the exact scopes really ran, then establish that a proposed identity-aware policy allows round four within the original limits.
2. In round four, supply the scripted assessment that discharges only the now-supported item. Require correct design readiness, with no implementation or test-pass claim.
3. Repeat the same scope three times. Preserve the expected no-progress stop.
4. Keep inputs identical but reorder argument-map keys, vary call IDs or reword briefs. These must not create acquisition novelty.
5. Change only a meaningful scope parameter. Keep output identical. Distinguish the observation.
6. Test same item key on different candidates and a skipped probe after the call cap. Credit only the actual chosen execution.
7. Test A/B/A repetition, overlapping scopes, paginated reads, refused reads, tool errors, truncated outputs and source changes. State expected novelty/coverage semantics before execution.
8. Exercise state reload with old and new progress representations. Preserve original request, owner and total budget.
9. Confirm round-cap, tool-cap, context-cap, cancellation, read-only restrictions, values rules and mission admission still hold.
10. Exercise both `/deliberate` and the recurse-mission constructor through normal boot wiring, rather than using only a standalone mock conductor.

The authored unit fixture asserts that brief/detail rewording leaves the fingerprint unchanged and adding a result changes it. It does not compare equal-result probes with different arguments. The chat fixtures separately cover a successful empirical probe followed by readiness and configured stall/round stops. Those are useful seams to extend, not executed evidence for the new case. [Fingerprint fixture][unit-tests] [Empirical and stop fixtures][chat-tests] [Boot policy fixture][boot-test]

Do not “fix” this by merely loosening the no-progress threshold in the fixture. The required distinction must be present in the production measurement, with the same configured bounds.

### B. Behavioral usefulness and fair costs

After deterministic correctness is established and execution is separately authorized, compare:

- **A: current measurement**, based on critical identities and result text;
- **B: invocation-aware measurement**, using bounded completed-read descriptors;
- **C, optional: bounded scope-coverage tracking**, which recognizes new coverage and rejects already-seen/irrelevant churn.

Use a frozen repository-disjoint set of requirements and ideation tasks, including staged negative searches, inherently repeated reads, overlapping domains, changing sources and tasks that should stop unresolved. Include Go, Python, JS/TS, Rust and mixed workspaces, plus unavailable/unsupported tools. Hold model/provider, JIT corpus, permissions, original request, candidate limits and total resource envelope fixed where feasible.

Predeclare what constitutes useful new evidence and independent acceptance of the resulting design. Blind reviewers to variant when possible. Report:

- false stall rate on adjudicated useful new evidence;
- false continuation rate on repeated or irrelevant evidence;
- proportion reaching an independently accepted design;
- requirement omissions, unsupported resolution claims and unresolved/stopped fraction;
- latency and full cost to the same acceptance endpoint;
- sensitivity to configuration, repository size, host path semantics and provider variability.

Include failed, cancelled, refused and budget-exhausted cases in the denominator. Cluster uncertainty by repository/task family rather than treating repeated seeds as independent repositories. Report equal-budget quality and full-cost-to-equal-quality separately. A reduced false-stall rate bought with excessive wandering, cost or false readiness is not an unqualified improvement.

### C. Full cost model and tradeoffs

No speedup, savings or reliability percentage is established here.

- **Current host work:** the fingerprint serializes probe records, constructs critical-item strings, sorts the list and hashes all resulting bytes. Its cost depends on item count, probe count and output length. Sorting involves string comparisons; it is not constant cost per record.
- **Minimal correction:** joining a completed probe to its candidate/item and encoding relevant arguments adds lookup, validation, canonicalization and hashing work proportional to the identity data inspected. A direct typed association avoids ambiguous lookups. Candidate-count bounds do not independently bound argument or item bytes.
- **Context:** the conductor already retains the full call in the previous challenge. A host-only identity correction can reuse that data without necessarily adding model-visible bytes. New visible descriptors, a coverage ledger or recovery notes do consume context and can close evidence earlier.
- **Persistence:** the current service writes full thread snapshots several times, including after probes and progress updates. Additional durable descriptors increase serialization, disk bytes and sync/rename work. Rewriting growing snapshots can amplify cost over many rounds; the bounded defaults limit but do not eliminate it. [Save implementation][persistence] [Save sites][probes] [Counter save][counter]
- **Freshness:** checking source versions may add file metadata reads, content hashing or tool-specific validation. Full-workspace hashing is not free and should not be imposed per probe without measuring it. Unknown source identity must remain explicit.
- **Provider work:** avoiding a false stop can require an additional proposer/challenger round and its reads. That extra work is the intended purchased opportunity, not a token saving by definition. Conversely, better repeated-read detection can save calls, but this has not been measured.
- **Recovery and user cost:** include restarts, manual rephrasing, repeated analysis, abandoned tasks and user interventions. Omitting them can make premature stopping look artificially efficient.
- **Operational complexity:** versioned identity, redaction, seen-state retirement and cross-platform normalization add migration and maintenance obligations. Prefer the smallest mechanism that meets the agreed contract.

Measure total model input/output tokens, visible cache charges, provider calls, recorded/estimated spend and their uncertainty, tool calls, read bytes, host CPU/allocations, serialized and persisted bytes, latency, user questions and final acceptance. Include JIT/consultation and downstream execution/verification when evaluating complete mission outcomes. Do not report only the hash's runtime or only the final successful round.

## 8. Primary literature and limits

These sources motivate distinctions and experiments, not claims that their published results transfer to codeNERD:

- W3C's **PROV-DM** distinguishes entities, activities and the relationships by which results are produced. The relevant inference is that an observation's output is not the same thing as the activity and scoped input that produced it. No RDF implementation or general provenance database is required by this audit. [PROV-DM](https://www.w3.org/TR/prov-dm/)
- Mokhov, Mitchell and Peyton Jones, **Build Systems à la Carte** (2018), separate scheduling from deciding whether a task needs rebuilding and discuss dependency-aware traces. The analogous design choice here is separating “what read happened?” from “should another reasoning round be admitted?” Matching identity does not prove an LLM judgment correct. [Primary paper](https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems-final.pdf)
- Shinn et al., **Reflexion** (2023), study agents retaining feedback across trials. That supports testing whether correct feedback continuity improves subsequent decisions, while their benchmark results do not establish the correct stopping rule, model choice or return on cost for this repository. [Primary paper](https://arxiv.org/abs/2303.11366)
- Zhou et al., **Language Agent Tree Search** (2024 version), combine search, reasoning and environmental feedback. The pertinent lesson is to evaluate the value and cost of additional grounded exploration. This audit does not propose importing Monte Carlo tree search or assume more exploration is always better. [Primary paper](https://arxiv.org/abs/2310.04406)

## 9. Checkable exit

A conductor must be able to distinguish a repeated observation from a successfully executed, relevant new scope with identical result text, using host-supported identity rather than changed prose or call IDs. Its progress policy must preserve no-progress stopping for repetition and all independent resource/permission/acceptance boundaries.

Establish that distinction through the three-scope witness and its repeated-scope control on the normal chat and mission design paths. Record both the static contract and appropriately scoped executed evidence before calling the gap closed. A useful round being admitted is not proof that the task is complete; it is the opportunity for the existing proposer/challenger mechanism to judge the new evidence.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/033df25a56e6064c0be7da7a9f413842d7354f50
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/agents.md#L3-L23
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/prompt/atoms/system/deliberation_proposer.yaml#L13-L29
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/prompt/atoms/system/requirements_interrogator.yaml#L13-L27
[types]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/types.go#L17-L68
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/types.go#L144-L182
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/round.go#L10-L72
[read]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/read_tools.go#L57-L81
[probes]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/outcome.go#L138-L181
[fingerprint]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/outcome.go#L118-L135
[counter]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/service.go#L139-L165
[stop]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/service.go#L100-L120
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/core/defaults/policy/deliberation.mg#L14-L58
[stop-policy]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/core/defaults/policy/deliberation.mg#L38-L40
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/config/routing.go#L18-L32
[grep]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/tools/core/search.go#L551-L593
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/persistence.go#L25-L74
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L873
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/cmd/nerd/chat/deliberation.go#L18-L50
[command]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/cmd/nerd/chat/deliberation.go#L96-L119
[note]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/outcome.go#L89-L115
[mission-constructor]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/cmd/nerd/cmd_campaign_mission.go#L40-L68
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/campaign/recurse_mission.go#L212-L292
[unit-tests]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/deliberation/deliberation_test.go#L86-L97
[chat-tests]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/cmd/nerd/chat/deliberation_test.go#L230-L268
[boot-test]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/internal/system/deliberation_boot_test.go#L13-L49
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/Docs/audits/ASTRA_QUEUE.md
[read-audit]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/Docs/audits/deliberation-read-invocation-identity.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/Docs/audits/portable-recurse-deliberation-crucible.md
[budget-audit]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/Docs/audits/deliberation-terminal-budget.md
[option-audit]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/Docs/audits/deliberation-values-option-admission.md
[freshness-audit]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/Docs/audits/deliberation-values-evidence-freshness.md
[owner-audit]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/Docs/audits/recurse-values-continuation-ownership.md
[review-audit]: https://github.com/theRebelliousNerd/codenerd/blob/033df25a56e6064c0be7da7a9f413842d7354f50/Docs/audits/deliberation-selected-review-handoff.md

