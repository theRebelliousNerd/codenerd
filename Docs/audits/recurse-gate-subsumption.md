---
doc-class: deep-dive
subsystem: campaign
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-10-30
verified-against: fa6877b932322bb6ca75826289366836d442c051
scope: workspace-gate findings admitted to recurse candidate selection
supersedes: []
---

# Recurse gate deduplication needs evidence of subsumption

## Conclusion and boundary

Recurse can measure and correctly attribute a failed workspace check, yet never offer that finding to its fix-selection policy. Its candidate collector treats any node-scoped gate with the same **kind and language** as evidence that the workspace gate is already measured per node. Those two classification fields do not establish equivalent commands, selected tests, build tags or obligations.

This is a **selected-candidate omission**, not a claim that the failed measurement disappears or becomes a pass. The workspace result remains in `state` and contributes to `openCount`; workspace gates run at the pass boundaries. An improvement attempt or other work might incidentally fix the defect. The narrower defect is that the loop's explicit finding-to-fix path cannot select this independently measured failure while that gate pair remains configured. [Candidate filtering][open] [Pass measurements and counts][pass] [Open-count implementation][count]

All source links below are pinned to public main [`fa6877b932322bb6ca75826289366836d442c051`][baseline], rechecked on 2026-10-30 at 06:43 UTC. This is source inspection and a conditional witness only. No repository program, test, benchmark, model inference or failure injection was run; no production incident rate or productivity improvement was measured.

## Primary tool reference

[Go’s test flag documentation](https://pkg.go.dev/cmd/go#hdr-Testing_flags) describes how command options select tests. A shared language and gate kind therefore cannot, by themselves, establish that two differently filtered commands exercise the same domain. The conditional witness below remains a static CodeNERD analysis, not an executed Go test.

## Existing deduplication is intentional

The optimization is not accidental dead code. `open` documents its intent: avoid attempting a whole-tree check again when a per-node check already measures it, illustrated by `go test ./...` beside `go test {pkg}`. `measuredPerNode` then implements that claim as the existence of a `ScopeNode` gate with equal `Kind` and `Language`. It does not compare command arguments, declarations, selected tests, actual node-run findings or any producer-supplied coverage relation. [Collector and predicate][open]

The repository's own `nerd.md` has paired workspace and per-node Go test/vet commands. That is a genuine motivation for avoiding redundant remediation. `TestRecurseOpen_SkipsWholeTreeRepeatsOfPerNodeGates` explicitly expects the Go workspace test finding to be excluded while another-language test and an audit remain. The inspected fixture supplies classification metadata and findings, not argv or an execution-domain relationship, so it confirms the existing assumption rather than distinguishing genuine repeats from independent checks. This test was read, not run. [Repository declarations][repo-config] [Intentional dedup test][dedup-test]

The recommendation is therefore not to abolish useful deduplication or count every similar diagnostic as separate necessary model work. It is to require an accountable basis for the subsumption decision and preserve independent obligations when that basis is absent.

## Production path

Both the CLI and chat use `RunRecurseCycles`. Each pass detects the workspace gates, measures workspace-scoped gates, visits DAG nodes, and measures workspace gates again at the end. A node visit measures its applicable node gates, gathers `open`, and passes only that slice to `recursePolicy.visit`. [CLI entry][cli] [Chat entry][chat] [Pass driver][pass] [Visit driver][visit]

The detection layer supports distinct declarations:

* `commands.test` becomes a workspace-scoped gate.
* A `gates:` entry can independently declare `kind: test`, a different command and `scope: node`.
* A declared `commands.test` suppresses the detected default test gate, but does not remove explicit `gates:` entries.
* Both commands beginning with `go` receive language `go`; their test selector and build flags do not enter that classification.

The schema permits these declarations and requires a node command to contain `{node}` or `{pkg}`. It does not require a node test gate to be equivalent to `commands.test`. [Detection and classification][detect] [Gate schema and validation][schema]

The practical scope is narrower than every default workspace. The inspected stock Go, Python, JavaScript/TypeScript and Rust detector definitions do not by themselves produce the matching same-kind/language all/node pair needed by this witness. Explicit node declarations create the relevant combination, as the repository's own configuration illustrates. Also, detection partitions unavailable programs out of `Set.Gates`; an unavailable node gate does **not** trigger this filter merely because it was declared. Preserve that existing negative control. [Stock definitions][stock] [Availability partition][availability]

Measurement itself retains useful evidence. A failed `go test -json` event with its package and test name becomes a finding; a package belonging to the module resolves to a relative directory. The recurse measurement attributes that target to a node. The later collector skips the *entire workspace gate* before examining its individual findings whenever `measuredPerNode` returns true. [Finding extraction][findings] [Failed-test event][test-target] [Target resolution][target-resolution] [Attribution][attribution] [Filter][open]

The policy adapter asserts `recurse_finding` facts only for the supplied `open` slice. Mangle ranks test findings and derives candidates from those facts. A finding withheld by this Go filter cannot be recovered by ranking it more highly, changing a stall threshold or improving the model prompt. [Policy projection][projection] [Ranking][policy] [Candidate rules][candidates]

## Conditional witness: integration failure beside passing unit tests

Use a clean, admitted Go workspace with an ordinary source package `store`, no pending recovery state, all required executables available, and these valid declarations:

```yaml
commands:
  test: go test -json -count=1 -tags=integration ./...
gates:
  - id: unit
    kind: test
    run: go test -json -count=1 -run '^TestUnit$' {pkg}
    scope: node
```

Assume the package has a passing ordinary `TestUnit` and an integration-tagged `TestContract` that genuinely fails on the current implementation. The workspace check executes `TestContract`; the node check does not. Hold configuration, source and unrelated prerequisites stable through the observation. No malicious model, failing parser, unavailable gate, source race, forbidden write, prior refusal or stalled finding is needed.

1. Detection keeps both explicit gates. They have distinct IDs and argv, but both are `test`/`go`.
2. The workspace run fails and its JSON test event produces an attributed target such as `store::TestContract`.
3. The node run passes and therefore supplies no failing finding for this obligation.
4. `open` appends the node gate's empty finding list. It then excludes the workspace gate because the node gate shares kind and language.
5. The integration finding is absent from the slice projected as `recurse_finding`, so no fix attempt can be selected **for that finding** on this visit.

The same filtering recurs on later visits while the declarations remain unchanged. Repeating the full workspace measurement does not repair candidate admission: the newly observed failure encounters the same filter again. If other work leaves this failure unchanged, the pass can retain a nonzero overall open count while its node visits report no selectable fix for that obligation. The loop still performs its separate improvement step; the witness does not claim that no executor callback runs at all. [Visit and improvement sequencing][visit]

Build tags are only one way to separate domains. Distinct `-run` selectors, a workspace race-enabled check versus ordinary node tests, or different declared audit commands sharing the empty language classification also require explicit consideration. They are additional proposed fixtures, not executed incidents. Exact argv equality after placeholder expansion would itself be insufficient to prove arbitrary cross-package behavioral equivalence; an admitted producer-specific coverage relation is stronger than a superficial string heuristic.

## Mitigations and what is not established

The workspace failure is not erased from the measurement ledger. `openCount` iterates retained state findings, and the pass-end journal records its count. Unattributed findings have their own explicit reporting route; the witness deliberately uses a correctly attributed package so it is not an attribution problem. [Counts][count] [Pass end][pass] [Attribution][attribution]

Keep/revert safety remains separate. The loop remeasures node gates and the selected workspace ratchet/target gates after a changed attempt. Current policy makes build and lint workspace-wide per-attempt ratchet kinds, while tests and audits are pass-boundary measurements unless targeted. Consequently a workspace **test** failure omitted from selection is not automatically rerun as that attempt's target. This note does not claim a falsely accepted change, nor that every workspace test is rerun after every attempt. [Post-attempt measurement][ratchet] [Ratchet kinds][ratchet-policy]

Independent model activity might repair the same implementation, a user could change declarations, or a different eligible gate could expose the same cause. Such paths limit claims about eventual outcome; they do not demonstrate that the suppressed workspace check is covered by the passing node check.

The root guidance assigns executive selection to Mangle and host code to measurement/adaptation. The narrow integration issue is the host silently deciding that a measured obligation is redundant without providing the policy an identity or coverage witness. No new permission or autonomous authority is required to preserve and classify the observation. [Repository guidance][guidance]

## Bounded correction

Update the campaign and gate specifications before implementation. Distinguish gate classification from gate identity and from a coverage/subsumption claim:

1. Preserve every measured finding with its gate/declaration identity, execution domain, source applicability and disposition.
2. Admit suppression only with an explicit producer-supported relationship showing which node measurements cover which workspace obligation for the current configuration and relevant source scope. Matching `kind` and `language` is not that relationship.
3. Have host adapters report the descriptors and observations; let Mangle derive whether work is independently eligible, covered by another admitted candidate, held, or deferred. Report why a candidate is suppressed and what covers it.
4. Where equivalence is unknown, retain the independent candidate rather than silently treating it as a duplicate. A conservative first correction may cost more remediation work; measure that cost rather than masking it.
5. Preserve existing genuine-repeat optimization through explicit supported families or adequately grounded finding relationships. Do not let a model's assertion that two checks are equivalent waive an owner-declared obligation.

Do not fix this by relabeling the workspace check, deleting its measurement, removing its failure, or weakening independent verification. Avoid an unbounded command-semantics inference engine: a small explicit relationship for known producers, with a conservative fallback for custom gates, is a testable starting point.

## Unrun evaluation and costs

First evaluate deterministically through the real declaration parser, gate detector, finding parser, attribution, visit adapter and Mangle selection. Assert both the retained global measurement and the actual candidate facts. A direct `measuredPerNode` unit test alone cannot establish that valid declarations reach this boundary. Keep production entry adapters, world-model and CodeDOM context in the subsequent full-path evaluation; restricted-tool substitutes are supplementary. Real-model work requires separate authorization and is not needed to prove this field comparison.

| Fixture | Required distinction |
|---|---|
| Workspace integration-tag failure; passing node unit selector | Workspace finding is selectable or explicitly covered by real compatible evidence, never silently suppressed |
| Same kind/language; different `-run`, tags or race setting | Classification similarity cannot stand in for domain coverage |
| Two custom scripts with empty language and the same kind | Unrecognized program language is not evidence of equivalence |
| Known supported workspace/node repeat | Redundant remediation can still be avoided, with the covering identity recorded |
| Node gate unavailable, skipped, unverified or inapplicable | Its mere declaration cannot certify coverage |
| Distinct gate failure plus a genuine duplicate in the same visit | Deduplicate only the covered obligation, not an entire class of gates |
| Config/source changes or resumed loop | Old subsumption evidence cannot silently cover a changed declaration/domain |
| No change, cancellation, refused path or regression | Existing ratchet and authority behavior remains intact |
| Passing workspace and node controls | No fabricated failure or gratuitous fix request |

Compare current filtering, conservative retention, and explicit-policy subsumption on identical workspace/checker/configuration snapshots. Record independent obligations measured, projected, selected and resolved; false suppression; duplicate attempts; unavailable observations; and final task quality. Include failed and unresolved runs, not only successful survivors. Preserve the global open count so a changed display cannot masquerade as improved coverage.

No saving or speedup is measured. The current existence scan can examine O(G squared) gate pairs across one `open` call for G gates, in addition to visiting relevant findings. Explicit descriptor normalization and relation indexing add metadata, versioning and policy-join work; their complexity depends on the chosen representation. Persisted provenance adds retention and migration costs. Most importantly, re-admitting omitted work can increase gate executions, model turns, context tokens and wall time while improving completeness. Measure all of those, plus CPU, memory, IO and human review separately. Do not call a lower token count a win if it came from leaving a required check's failure unaddressed.

The exit is a production-route deterministic test showing that the witness's independently failing workspace obligation reaches selection, alongside preserved genuine-repeat optimization and existing ratchet controls. This is a proposal with source-derived expectations, not a claimed implementation or validated runtime result.

## Duplicate check

The bounded review read root and campaign guidance, the current Astra queue, relevant gate/recurse audits, and current unmerged research branch diffs (`research-main-20261030-0544` and `research-main-20261030-0614`). Targeted searches included `measuredPerNode`, its intentional dedup test, workspace/node equivalence and candidate suppression; the available local audit corpus was also checked for the exact mechanism. No exact existing report was found. This is not a claim to have exhausted every historical discussion.

The current `recurse-gate-contract-continuity` proposal concerns environment and passing-exit semantics lost **after** a finding is selected; this finding is suppressed **before** policy selection. Gate comparison/domain and scoped-overlay audits concern comparing or composing measurements, not admitting independent measured findings to recurse work. Existing compensation/write-ownership, campaign recovery and mission-driven improvement work is not reopened here. [Current audit queue][queue]

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/fa6877b932322bb6ca75826289366836d442c051
[open]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/campaign/recurse_cycle.go#L702-L739
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/campaign/recurse_cycle.go#L441-L499
[count]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/campaign/recurse_cycle.go#L678-L686
[repo-config]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/nerd.md#L5-L40
[dedup-test]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/campaign/recurse_cycle_test.go#L513-L537
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/cmd_campaign_recurse.go#L199-L237
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/campaign_recurse.go#L157-L174
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/campaign/recurse_cycle.go#L760-L813
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/gates/detect.go#L19-L117
[stock]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/gates/detect.go#L123-L213
[availability]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/gates/gates.go#L171-L195
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/projectdoc/nerdmd.go#L518-L548
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/gates/findings.go#L98-L175
[test-target]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/gates/findings.go#L503-L530
[target-resolution]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/gates/findings.go#L636-L667
[attribution]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/campaign/recurse_cycle.go#L524-L630
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/campaign/recurse_policy.go#L64-L89
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/core/defaults/policy/recurse.mg#L129-L159
[candidates]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/core/defaults/policy/recurse.mg#L236-L255
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/campaign/recurse_cycle.go#L955-L1029
[ratchet-policy]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/core/defaults/policy/recurse.mg#L257-L329
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/agents.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/Docs/audits/ASTRA_QUEUE.md
