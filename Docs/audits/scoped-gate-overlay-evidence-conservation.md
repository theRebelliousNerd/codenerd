# Scoped gate overlays must conserve unknown evidence

## Decision and evidence boundary

Keep package-scoped verification, but make an empty measurement an identity operation on the existing evidence. A measurement of no packages must not turn an unavailable whole-module gate into a pass. Likewise, a successful measurement of one package must not certify packages for which the baseline never had usable evidence.

This review is pinned to public main [`99c5526e69391fafbce678b5fa467d589c4ad2a5`](https://github.com/theRebelliousNerd/codenerd/commit/99c5526e69391fafbce678b5fa467d589c4ad2a5), inspected October 29, 2026. It is a source-derived conditional finding. No tests, gate commands, model calls, inference, training or benchmarks were run. No actual incorrectly retained commit is claimed.

The [current-main gate addendum](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/Docs/audits/gate-scoped-recurse-current-main-addendum.md) already identifies findings-only reconstruction as an evidence risk. This document completes that lead with a concrete ordinary-production input sequence, its later policy consequence, and a repair contract. It is not another report of the standalone CLI comparison-domain gap, nor a claim that scoped checks are inherently unsound.

## Implemented path

The initial full measurement can record a selected lint gate as unavailable when its binary is absent. It carries a reason and no findings. The measurement contract distinguishes pass, fail, unavailable and unverified; pass means the gate completed without findings. [Status definitions and measurement](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/gates/errorfree.go#L30-L67).

After a changed recurse attempt, `goPackagesOf` selects directories of changed `.go` files. A change with no `.go` paths deliberately produces a non-nil empty slice. The callback uses this to scope vet, staticcheck and golangci-lint, while other gate kinds retain their own behavior. [Package selection](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/campaign/recurse_error_free.go#L103-L125), [measurer construction](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/gates/errorfree.go#L98-L133).

For such an empty scoped gate, `commandGate.Measure` returns pass with the `NoPackageScope` marker before looking for the executable. Its reason says that nothing was measured. This is a deliberate empty-domain result and is not itself evidence that the missing tool ran. [Early return and executable lookup](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/gates/errorfree.go#L173-L189).

The recurse loop restricts its retained baseline to the after scope, asks the ratchet, and overlays the result only after a keep verdict and a successful commit. `RestrictTo` only filters old pass/fail rows, leaving unavailable/unverified rows as they were. `Overlay`, however, selects its replacement branch by the after measurement's status, not the old status. With a scoped after pass it calls `restrict` on the old measurement, merges findings, derives pass/fail from the merged count, and clears scope. [Restriction and overlay](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/gates/errorfree_scope.go#L40-L110), [production ratchet and overlay](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/campaign/recurse_cycle.go#L978-L1053).

## A concrete conditional sequence

Use a selected optional lint gate such as staticcheck whose executable is absent. Let the workspace's other gates and the actual attempt target remain independently verifiable.

1. The startup full measurement records staticcheck as unavailable, with zero findings and a missing-binary reason.
2. A legitimate attempt changes only non-Go files. This must be an attempt whose own target or improvement obligations can genuinely pass; changing a documentation file is not automatically sufficient for keep.
3. The scoped lint measurement receives the explicit empty package list. It returns pass over `NoPackageScope`, launches no process, and does not even perform the executable lookup.
4. `RestrictTo` leaves the old unavailable status intact. The gate policy does not classify unavailable-to-pass as a regression. It also does not derive improvement for that transition; the comparison is same under the current rules.
5. If the independent ratchet conditions permit keep and commit succeeds, `Overlay` restricts the old empty finding list, appends the new empty list, sets status to pass, and sets scope to nil.

The result is a retained baseline row that represents whole-module pass even though this gate has never successfully run. The old reason may still say the binary is not installed, because the scoped branch starts from the previous measurement rather than constructing a clean observation receipt. The current policy projection does not include that reason, argv or scope, so those contradictory diagnostics do not correct its pass fact. [Gate transition rules](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/core/defaults/policy/gates.mg#L25-L80), [snapshot projection](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/gates/errorfree_snapshot.go#L96-L114).

No tool installation, malformed report, forged JSON or concurrent writer is needed for this conditional sequence. It requires an ordinary kept non-Go attempt before the next full refresh. It does not establish that such an attempt has occurred in a real run.

## Why this matters beyond a misleading status

Before the next full refresh, suppose a later attempt does touch a Go package while staticcheck remains absent. That after measurement reaches the executable lookup and returns unavailable. The retained baseline now says pass. Restriction leaves a pass with no findings; policy therefore sees pass-to-unavailable and derives regression.

That can veto a later otherwise acceptable attempt because a prior empty observation invented a pass, not because the gate became less available. The accurate unavailable-to-unavailable comparison would not derive that regression. This is a conditional false-regression path through the existing consumer, not a claim that all future cycles are blocked. A new full measurement restores the missing-tool status, another veto may independently reject the attempt, and tool installation would change the facts.

The full-refresh default is ten scoped cycles; zero requests only the initial full measurement. Refresh happens between attempts and replaces the retained baseline. Charge that recovery as delayed correction rather than treating it as evidence that intermediate statuses were valid. [Refresh implementation](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/campaign/recurse_error_free.go#L31-L49), [configuration](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/cmd/nerd/cmd_campaign_recurse.go#L79-L89).

The same structural concern exists when an unverified whole-module baseline is followed by a successful nonempty partial measurement: a clean subset does not establish completion outside that subset. A separate opaque-failure continuation also remains possible: a fail with positive in-scope findings compared with a scoped fail having no parsed findings may avoid a regression veto, after which findings-only overlay can lose the opaque failure. Unlike the empty-domain witness, that case requires particular process/report behavior and all other keep conditions. It should remain a distinct evaluation case, not be asserted as a reproduced incident.

## Safeguards and counterexamples

The ratchet still requires a changed tree and successful target or qualifying improvement, rejects forbidden writes, checks workspace/node gate regressions and metric regressions, and can require measured testing for improvements. The additional error-free gate comparison is not the only keep criterion. [Ratchet keep rule and guards](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/core/defaults/policy/recurse.mg#L285-L323), [metric guards](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/core/defaults/policy/recurse.mg#L379-L392).

Overlay is not called for reverted attempts. Full-scope after measurements replace the old measurement. After unavailable/unverified measurements take their explicit replacement branch. An ordinary genuine pass baseline followed by an opaque fail is already a pass-to-fail regression and normally does not reach overlay. Known findings outside a successfully measured subset are deliberately retained. These are real controls to preserve.

The existing scope tests cover retaining outside findings and comparing an intentionally empty domain. They do not supply an unavailable or unverified baseline to the empty-scope overlay path. Tests were read, not run. [Current fixtures](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/gates/errorfree_scope_test.go#L16-L67).

## Proposed evidence contract

Separate observation completion from defect enumeration. A finding list is evidence about observed defects; it is not a complete encoding of whether a tool was available, whether it finished, or which domain it covered.

For each gate, retain a finite collection of coverage segments. A segment records its package domain, observation status, source/configuration applicability, observation identity and findings. The aggregate verdict is derived from those segments and the required domain, not reconstructed solely from the union of finding lists. Preserve failure reasons and provenance with the observation that produced them.

The minimum invariants are:

- Empty-domain overlay preserves the prior whole-domain assurance, including unavailable, unverified and opaque failure. It may add a receipt saying no measurement was requested, but cannot improve or worsen global evidence.
- A successful partial observation replaces evidence only for that scope. Unknown outside evidence remains unknown.
- A full successful measurement may establish full pass. A partial measurement may do so only if the rest of the required domain already has valid, applicable successful evidence.
- Opaque process failure survives restriction/overlay as an unresolved observation, rather than becoming a count of zero defects.
- A composite baseline identifies its component observations; its newest snapshot ID does not imply all evidence was collected at that revision.

Implement the narrow empty-scope identity correction first, then strengthen the general partial-evidence representation. Do not silently certify old snapshots that lack coverage/completion metadata. Keep their historical diagnostics readable and mark unknown assurance honestly.

The host should record faithful measurements and domain transformations. Mangle should derive whether the expected domain is covered, whether required evidence is unresolved, and whether a comparison is admissible. Avoid moving the final executive decision into another ad-hoc Go status heuristic. Suggested relations and algorithms are design proposals, not an implemented schema or drop-in rule file.

An algorithm is: normalize the requested domain; retain unchanged evidence for its complement; replace only the covered segment with the new observation; preserve incomplete observations; derive coverage of the required domain; then derive defect trends only for comparable completed segments. The empty-domain case must be an explicit identity. Atomic publication of the composed evidence prevents consumers from seeing a partly replaced baseline.

This framing follows the useful distinction in [Build Systems à la Carte](https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems-final.pdf) between scheduling work, deciding what can be reused, and correctness relative to task dependencies. Applying that discipline to verification evidence is a proposed engineering use, not a claim that the paper proves this particular overlay correct or that the idea is novel.

## Costs and failure modes

For F retained findings and S coverage segments, a straightforward transformation scans findings and segments, plus domain matching and sorting. Existing code already scans and sorts finding lists. Additional storage is bounded by retained segment identities and their observations; coalesce equivalent adjacent scopes and retire superseded evidence rather than keeping every cycle live forever. Preserve enough provenance for audit without copying full raw process output into every segment.

Measure full process CPU and wall time, I/O, parsing, scope resolution, baseline transformation, kernel assertions/fixpoint work, persistence, retained memory, periodic full refresh and any extra retries. Count model and tool calls spent on a falsely reverted task, not only the cost of the comparison. Conservative unknown states may cause extra full checks; those are real costs, not free correctness improvements.

Risks include scope normalization that merges distinct package patterns, stale source applicability, silently dropping incomplete segments during coalescing, unbounded provenance retention, and confusing a deliberate partial assurance statement with a full guarantee. This proposal does not solve the separate question of whether touched packages form a complete dependency closure. Keep that limitation visible and evaluate indirect changes independently.

## Fair evaluation and acceptance

No evaluation below has been run. Compare current behavior, an empty-scope identity fix alone, and full completion-aware overlay on identical observations. At the policy socket, enumerate old pass/fail/unavailable/unverified/opaque states against full, nonempty partial and empty after scopes. Require identity for empty overlay, preservation of outside uncertainty, correct diagnostics, and no invented count-based pass.

At the complete-system boundary, use the actual recurse measurement callback, ratchet, kept-cycle baseline update and next-cycle comparison. A deterministic fixture should begin with an absent optional lint binary, successfully resolve a non-Go target under the independent guards, then attempt a Go-target change before full refresh. Check the baseline's status and the reason for the next verdict. Include a control where the first attempt is rejected, so no overlay occurs, and a refresh-boundary control where the unavailable state is remeasured.

Add unverified baseline recovery, genuinely clean full baseline, partial pass with unknown outside scope, named findings preserved outside scope, opaque failure, tool installation between cycles, and zero-refresh versus default-refresh schedules. Test the actual empty-scope early return, not only a handcrafted pass row. Mocking process outcomes can isolate contract behavior; separate authorized end-to-end runs should verify real command classification and charge full runtime cost.

Score false full-assurance states, false regression vetoes, unresolved-evidence retention, diagnostic consistency, verified task outcomes, retries, latency distributions and memory. Hold required assurance, source fixtures, tools and model configuration fixed. A smaller number of measured packages is not a performance win if the comparator silently changes the guarantee.

Acceptance is not merely that the first attempted repair can be kept. It is that the baseline remains a truthful input to the next decision: observing nothing must not manufacture certainty.
