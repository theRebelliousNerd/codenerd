---
doc-class: deep-dive
subsystem: campaign-recurse-diagnostic-identity
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-04
verified-against: 0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0
supersedes: []
---

# Preserve diagnostic subjects before the recurse finding census

## Decision and evidence boundary

The recurse finding collector uses a lossy failure signature as the identity of a file diagnostic. The signature replaces every decimal digit sequence with `N`, including digits inside program identifiers. Consequently, ordinary same-file diagnostics such as `undefined: Schema1` and `undefined: Schema2` receive the same ID. The second finding is discarded before the uncapped census, candidate selection, target-presence test or per-finding invocation record can see it. This is deterministic input normalization, not a speculative hash collision. [Collector][collector] [Normalizer][normalizer]

The narrow recommended correction is to separate **diagnostic subject identity** from **presentation/failure-shape normalization**. Preserve semantically meaningful identifier and diagnostic-code components in the identity used for cardinality and target resolution. Keep line-movement stability, duplicate suppression, bounded prompt selection and the existing policy-owned ratchet.

This report is a source-derived correctness finding, not an observed production incident. Public main resolved to **0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0**, committed November 4, 2026 at 04:22:37 UTC. All codeNERD links below use that pin. Inspection covered root and public-repository guidance, campaign guidance, campaign/mission architecture, implementation, authored tests, the current audit inventory and neighboring reports. No repository program, compiler, build, test, gate, model inference, benchmark or failure injection was executed. No implementation, repository file, credential or setting changed. The observations below are conditional on the stated complete output and admitted invocation; proposed checks are entirely unrun.

## 1. Contract and production relevance

The root contract gives the model creative work and the kernel executive control. Recurse's architecture carries a selected finding, its evidence and its actual check into a campaign. The normal CLI and chat both construct an attempt campaign and execute the same `RunRecurseCycles` driver. The issue is therefore upstream of model reasoning and common to both doors. [Root guidance][root] [CLI][cli] [Chat][chat] [Attempt campaign][attempt]

A file diagnostic's documented ID is meant to stay stable as source lines move. The authored identity fixture also explicitly distinguishes two different diagnostics in the same file. Its positive controls use `unreachable code` versus `self-assignment of y`; they do not test two identifier-bearing messages that become equal only after digit normalization. Test-name IDs have a separate intentional contract: the same named failing test retains its ID when its assertion message changes. Those two identity kinds must not be accidentally conflated in a repair. [Identity contract][identity-contract] [Authored controls][identity-test]

The intended executive decisions require more than a display-friendly grouping:

- `FindingsAll` is described as every distinct finding, without the work-selection cap.
- `gateRun.totalFindings` supplies the census used by the ratchet.
- `hasFinding` supplies target persistence, so removing a selected finding can count as resolution.
- A failing gate becomes worse when its reported finding count increases.
- The kept-attempt path commits only after the kernel verdict, but that verdict cannot recover subjects discarded before its inputs are asserted. [Uncapped collector][collector] [Census/presence][census] [Policy][policy] [Host-to-policy projection][projection]

This does not establish that every accepted improvement is wrong, or that every diagnostic has a unique real-world root cause. It establishes that two explicit different source subjects are forcibly treated as one without a tool-specific equivalence rule.

## 2. Exact source mechanism

1. The generic positioned-diagnostic parser captures file, line, column and message. Its Go stream appends a rich `GoDiagnostic`, then passes only file and message to the finding callback with `keyByMessage=true`. Line/column movement is already excluded from that key. [Positioned parser][parser] [Callback][callback]
2. The callback computes `sig := signature(root, msg)`, begins its key with gate ID and normalized target, and appends `sig` for a diagnostic. It hashes that key and returns immediately when that ID is already in `seen`. Only the first record and message survive. [Collector][collector]
3. `signature` replaces workspace spelling and hexadecimal-address-shaped strings, then performs unrestricted `numbers.ReplaceAllString(s, "N")`. The regular expression is `\d+`; it has no identifier, quoted-string, diagnostic-code or numeric-literal boundary. [Digit expression][digit-regex] [Normalizer][normalizer]
4. Recurse `measure` receives only the surviving findings. It adds their IDs to `allFindingIDs`, and records the first invocation in `byFinding`. The later selection cap cannot recover a record that never left `FindingsAll`. [Measurement][measure]
5. The after-check projects `totalFindings` and target membership. For a completed, untruncated, non-empty-test check, an absent selected ID sets `TargetOK`. Those observations are asserted into Mangle without a separate semantic-identity confidence or collision field. [After-check][after] [Projection][projection]

The same callback is used for TypeScript and Rust diagnostics that are keyed by message. This report's minimal witness uses only ordinary Go positioned diagnostics; it does not claim exhaustive parser conformance or identical frequency across languages.

## 3. Minimal conditional witness

Take one completed failed gate result with no truncation, no cancellation, a stable gate ID, and these two small positioned diagnostic lines:

```text
pkg/model.go:3:9: undefined: Schema1
pkg/model.go:4:9: undefined: Schema2
```

Assume both names are genuinely unresolved in the same admitted source file. No path with whitespace, alias, graph transition, second invocation, output cap or malformed compiler record is needed.

Go identifiers may contain digits after their initial letter; `Schema1` and `Schema2` are different identifier spellings. The compiler's identifier checker reports an undeclared name using that identifier's value. These primary sources establish the legitimacy of the example's subject distinction and error shape; they do not represent a compiler run performed for this report. [Go identifiers](https://go.dev/ref/spec#Identifiers) [Go identifier checker](https://github.com/golang/go/blob/master/src/cmd/compile/internal/types2/typexpr.go)

The source trace is:

| Field | First diagnostic | Second diagnostic |
|---|---|---|
| Gate ID | Same gate | Same gate |
| Target | `pkg/model.go` | `pkg/model.go` |
| Message | `undefined: Schema1` | `undefined: Schema2` |
| Signature | `undefined: SchemaN` | `undefined: SchemaN` |
| Finding key and ID | K | K |
| Collector disposition | Retained | Discarded as duplicate |

Thus the uncapped API returns one finding, not two distinct subjects. The display's first message can still say `Schema1`, while the ID silently represents the whole normalized family. Even a literal identifier `SchemaN` shares this normalized message. Computing a hash value is unnecessary to establish equality: the key bytes are already identical.

### 3.1 A valid targeted repair can remain “open”

Start with both diagnostics and select the retained `Schema1` record. If a candidate resolves `Schema1` but `Schema2` remains, the after-check returns the same ID K with the latter message. `hasFinding(K)` is true, so the selected target cannot be marked resolved from this observation.

That is a conditional target-resolution barrier, not a claim that an ordinary campaign must return after a partial fix. The campaign acceptance command can continue trying to make the entire reporting check pass, or the executor may undo its work. The independent whole-command partial-repair barrier is already documented in `recurse-mission-acceptance-scope.md`; correcting diagnostic identity does not repair that objective-and-scope mismatch. The raw gate output also still contains both initial lines, so a capable model may repair both at once. Neither fact restores distinct target identity or the census. [Acceptance contract][attempt] [Acceptance-scope audit][prior-acceptance] [After-check][after]

### 3.2 A genuine increase can be invisible to the ratchet

For one independent failing gate B, a before-output containing only `Schema1` produces count 1. An after-output containing both `Schema1` and `Schema2` still produces count 1. With complete output, this supplies `fail,1 → fail,1`, which does not derive the policy's count-growth veto.

A compositional keep witness requires additional explicit premises:

- Another selected finding A is actually resolved by the candidate, or an admitted improvement independently satisfies its required metric/test conditions.
- Gate B is included in the before/after comparison and is failing both times.
- The candidate introduces the additional unresolved subject; it is not merely discovering a previously hidden compiler suffix.
- The executor returns an observable changed candidate for judgment.
- No other gate, metric, ownership, forbidden-path, mission or optional error-free comparison veto applies.

Under those premises, B's increase cannot supply the veto it would have supplied with distinct subject counts. This is a conditional missing defense, **not a reproduced false commit**. The existing policy deliberately allows equal-count failure substitution because fixing a compiler error can expose another; this report does not ask to replace that policy. Its witness is true growth hidden as equality before the policy can apply its own rule. [Count-growth policy][policy] [After-check][after] [Commit boundary][commit]

A controlled future fixture can keep A and B independent using an owner-declared check. An end-to-end study must additionally demonstrate the ordinary campaign's candidate-return behavior rather than treating a stub executor as proof of a production incident.

## 4. Nonduplication and existing protections

The scoped audit screen included the current `Docs/audits` inventory and targeted repository searches for signature/numbers, normalization/diagnostic and the concrete identifier family. It found no exact report of digit-bearing diagnostic subjects collapsing before the census. This is a bounded novelty assessment, not an assertion about all branches or private discussion.

The adjacent reports remain valid and separate:

- **Finding census completeness** covered the 50-finding selection cap and capture completeness. It explicitly used different target paths so message normalization did not collapse its witness. The current `FindingsAll` and `allFindingIDs` repair preserve more IDs, but cannot recover distinctions lost while generating each ID. [Census audit][prior-census]
- **Collapsed finding invocation identity** covered selecting the actual producer within a multi-path node. Here one file and one invocation suffice; repairing `byFinding` does not create the discarded second subject. [Invocation audit][prior-invocation]
- **Mission acceptance scope** covered requiring the whole reporting command to pass even when the admitted target is only one finding. That independent barrier can prevent a partial repair from reaching the outer ratchet. This report's census collision occurs during a fresh measurement even before any campaign starts, and its conditional ratchet witness explicitly requires a candidate to reach judgment. [Acceptance-scope audit][prior-acceptance]
- **Stall evidence applicability** covered reusing historical failure decisions after current conditions changed. Here the two distinct current diagnostics collide in the same fresh measurement, before any historical stall facts exist. [Stall audit][prior-stall]
- **TypeScript spaced diagnostic paths** covered rejecting an otherwise useful located line during parsing. These short Go paths are recognized correctly; the loss occurs after successful extraction. [Path audit][prior-path]
- **Gate comparison-domain admission** covered whether snapshots measure comparable domains. The witness keeps the same gate, source file, invocation and complete output domain. Correct comparison-domain identity does not make a lossy subject key injective. [Comparison audit][prior-comparison]

Retain the improvements already present: bounded candidate lists, uncapped ID census, capture-truncation refusal, exact per-finding invocation lookup, unknown/unverified distinction, empty-test handling, scope checks and optional error-free gates. Do not manufacture a green verdict, broaden the write scope or ask deliberation to waive a check.

## 5. Proposed contract, not implemented

### 5.1 Separate three identities

Define and document, before implementation:

1. **Diagnostic identity:** gate/invocation domain, normalized source target, diagnostic kind/code where available, and semantically meaningful subject/message components. Its normalization must not erase digits inside an identifier merely because digits can also appear in transient output.
2. **Occurrence information:** source spans and repeated emission locations. These can move while the diagnostic subject remains the same. Repeated identical diagnostics from one tool should not automatically inflate the count.
3. **Failure-shape signature:** optional normalized prose for stall grouping or presentation. It may intentionally merge changing values, but it must not silently stand in for the identity or cardinality contract.

The minimal correction need not introduce a new service or model call. A conservative initial adapter can retain the diagnostic message's semantic text while continuing to exclude separately parsed line/column fields. Tool-specific stable codes/subjects can improve that later. Do not merely change the replacement token from `N` or enlarge the hash: neither addresses pre-hash equivalence.

### 5.2 Preserve the consumers and generation boundary

Feed the corrected identity consistently to `seen`, `allFindingIDs`, target presence, invocation attribution, candidate facts and durable attempt references. Continue using Mangle for selection and keep/revert. The Go side supplies observation identity, not a new autonomous verdict.

Changing finding IDs is a compatibility change. Persisted prior attempts and stalls should remain historical rather than be silently rebound to a new identity. Specify an identity-version boundary and remeasurement/reconciliation behavior; old records must not provide evidence that a newly distinguished subject is resolved or stalled. Conversely, resetting all history without an explicit boundary could repeatedly spend work on known failures. The migration is part of acceptance, not an optional documentation footnote.

For missions, the host must still bind the selected subject to its source snapshot, scope and actual check before design deliberation. The proposer and interrogator can reason about the problem, but neither should infer missing cardinality from a normalized first message or gain authority to enlarge the scope. [Mission preparation][mission] [Mission write guard][scope]

### 5.3 Keep uncertainty honest

A universal text parser cannot promise complete semantic identity for every arbitrary owner command. Establish the supported diagnostic families, retain exact raw evidence for review within existing limits, and expose an explicit unsupported/ambiguous disposition where a stronger claim is unavailable. “All recognized distinct subjects” is a narrower and testable contract than “all errors in every program.”

Do not count every printed line as a separate defect, treat whitespace-only changes as new work, or include line numbers in the stable ID merely to make the witness separate.

## 6. Costs and failure modes

Let B be captured bytes and D the number of semantically distinct recognized diagnostics. The current path already scans B and keeps a set of IDs. Preserving subjects keeps the same broad O(B) parse and O(D) identity-storage shape, but D may grow relative to today's overcollapsed count. That increase is legitimate information, with real memory and prompt-selection consequences.

Keep rich candidate selection bounded separately from the evidence census. A future resource bound that prevents exact distinct counting must carry an unknown/lower-bound disposition rather than pretend its truncated cardinality is complete. Tool-specific parsing adds maintenance and version coverage costs. Persisted identity-version transitions add storage, replay and operational explanation costs. No time, token, memory or success-rate improvement is claimed measured.

Repair risks include:

- Over-splitting repeated emissions or line moves, inflating counts and retries.
- Continuing to normalize identifier digits in one consumer while another preserves them.
- Preserving identifiers but collapsing semantically distinct diagnostic codes or literal values without a justified equivalence.
- Applying diagnostic-message semantics to stable named tests, accidentally making every assertion-value change a new test.
- Rebinding historical stalled/refused records to the wrong new subject.
- Losing exact command, output completeness or source-snapshot qualification while changing identity.
- Fixing only Go while claiming universal JS/Python/Rust coverage.
- Replacing a kernel-derived decision with an LLM assertion that two failures “look the same.”

## 7. Fair evaluation plan, entirely unrun

### Stage A: deterministic identity and census checks

Use the production collector and measurement projection with controlled complete outputs, not a separately reimplemented normalizer.

| Case | Required observation |
|---|---|
| Same file, `Schema1` and `Schema2` | Two subject identities |
| Literal `SchemaN` alongside `Schema1` | Distinct identities |
| Same diagnostic with moved line/column | Stable subject identity |
| Repeated identical line/message | No invented extra subject |
| Same message in two files | Preserve target separation |
| One→two numeric-suffix subjects | Count increases |
| Two→one after selected subject repair | Removed ID absent; surviving ID present |
| Named test with changed numeric assertion message | Preserve the test identity contract |
| Go text/JSON, supported TS and Rust diagnostics | Family-specific identity agreement |
| Truncated, unverified or unsupported output | Existing failure/qualification boundaries remain |
| More than 50 genuine subjects | Bounded selection plus truthful census |
| Historical identity version on resume | No silent cross-generation target/stall binding |

Inspect, but do not treat as passing, the existing identity, bounded-finding and line-movement fixtures. Add regressions through both `FindingsAll` and the normal recurse measurement-to-ratchet handoff. A helper-only identity test is insufficient. [Authored controls][identity-test] [Measurement][measure]

### Stage B: compositional ratchet checks

Compare the current collector, a minimal semantic-preserving variant and any richer typed adapter against the same captured output corpus and same Mangle policy. Predeclare true duplicate equivalences and distinct subject labels. Keep command domains, source fixtures, acceptance rules and capture limits equal.

The controlled two-gate witness must show that B's true growth produces the existing `recurse_gate_worse` veto while A's successful resolution remains recognized. Also require a negative control where one old diagnostic is replaced by one new diagnostic without growth: preserve the existing policy decision rather than improving the score by silently tightening policy.

Measure undercounted subjects, overcounted duplicates, target-presence errors, deterministic ID stability, parse cost and peak memory. Include unsupported and ambiguous results in the denominator. Do not report only successful fixtures.

### Stage C: optional production-path study

Only after structural acceptance, exercise ordinary CLI and chat campaigns with controlled toolchains and permitted executors. Include campaign acceptance remediation, an executor returning an error, candidate undo, ordinary owned-write adoption and optional error-free gates. Distinguish “collector defect demonstrated,” “ratchet input corrected,” and “real campaign changed its final outcome.”

If model-assisted work is later authorized, hold provider/model, prompts, source snapshots, scopes, budgets and acceptance conditions constant. Report retained correctness, useful partial progress, extra attempts, false stops, cost and regressions. More finding IDs or fewer retries alone is not success.

**Exit condition:** the concrete identifier-family witness preserves distinct current subjects and target removal through the real measurement/policy boundary, ordinary duplicate and line-move controls remain stable, and old durable identities cannot impersonate new evidence. Until then, record this as a source-supported identity/census gap, not an implemented or executed fix.

## Pinned source index

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0
[root]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/agents.md#L1-L88
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/cmd/nerd/cmd_campaign_recurse.go#L220-L273
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/cmd/nerd/chat/campaign_recurse.go#L218-L266
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/campaign/recurse_attempt.go#L34-L112
[identity-contract]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/gates/findings.go#L16-L38
[identity-test]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/gates/findings_test.go#L94-L116
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/gates/findings.go#L112-L146
[normalizer]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/gates/findings.go#L223-L232
[digit-regex]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/gates/findings.go#L93-L95
[parser]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/gates/findings.go#L340-L373
[callback]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/gates/findings.go#L591-L607
[census]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/campaign/recurse_cycle.go#L207-L230
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/campaign/recurse_cycle.go#L759-L838
[after]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/campaign/recurse_cycle.go#L1420-L1450
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/campaign/recurse_policy.go#L205-L279
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/core/defaults/policy/recurse.mg#L260-L329
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/campaign/recurse_cycle.go#L1469-L1509
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/campaign/recurse_mission.go#L174-L241
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/internal/campaign/recurse_mission_scope.go#L13-L74
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/Docs/audits/recurse-finding-census-completeness.md
[prior-invocation]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/Docs/audits/recurse-collapsed-finding-invocation-identity.md
[prior-stall]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/Docs/audits/recurse-stall-evidence-applicability.md
[prior-path]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/Docs/audits/recurse-typescript-spaced-diagnostic-paths.md
[prior-comparison]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/Docs/audits/gate-comparison-domain-admission.md
[prior-acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/0c4e1db437b212ba2c9db7a1954b9c21b5a5fcb0/Docs/audits/recurse-mission-acceptance-scope.md

