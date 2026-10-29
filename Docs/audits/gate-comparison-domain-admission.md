# Gate comparison domain admission

## Decision and evidence boundary

Before deriving that no gate got worse, CodeNERD should establish that both measurements answer the same question. The current error-free gate comparison can report a keepable pair after a gate disappears from the second snapshot, or after the test scope changes under the same gate name. A separate evidence-completeness problem can call an opaque command failure an improvement because it has fewer parsed findings.

These are source-derived conditional scenarios, not executed reproductions. This review is pinned to main [`8decae5763cb88bbd842795ca81d3e2559ec2b20`](https://github.com/theRebelliousNerd/codenerd/commit/8decae5763cb88bbd842795ca81d3e2559ec2b20), inspected on October 29, 2026. No implementation, tests, benchmarks, model calls, inference or training accompanied it. The recommendation is to add a comparison admission contract around the existing policy, preserving its intended ability to work on a baseline that already has findings.

The new `nerd gates` command and optional recurse measurement are implemented. Domain admission and explicit finding-enumeration completeness are proposals. None of the examples establishes that an actual user lost work, that an autonomous attempt was incorrectly committed, or that a measured speedup is available.

## What the production path implements

`MeasureErrorFree` runs selected measurers sequentially. The table includes build, vet, tests, staticcheck, golangci-lint, govulncheck and gofmt; the CLI adds policy and prompt-atom corpus measurers. `Only` selects names or labels, while `TestPackages` changes the packages supplied to the test gate. Each measurement retains its status, count, findings, argv, exit code, output, reason and duration. The snapshot also retains root, time and starting HEAD. [Measurement contract and selection](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree.go#L40-L138), [CLI options](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/cmd/nerd/cmd_gates.go#L34-L109).

The Mangle projection is narrower. `Snapshot.Facts` emits snapshot existence, each gate's status and count, and deduplicated finding keys. It does not project the root, invocation, selected package scope, tool identity or enumeration completeness. `Compare` asserts the pair and reads `cycle_keepable`, gate changes and new/gone findings. Its result map and its final requirement that a `gate_change` exist iterate only the gates in the after snapshot. [Projection and comparison](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree_snapshot.go#L96-L218).

The policy joins before and after by gate name. It treats pass-to-fail, measured-to-unverified/unavailable, and a growing failing count as regression. A decreasing failing count is improvement. Its comments explicitly permit equal-count finding changes because a repair can reword or expose another finding. This report does not mistake that deliberate policy for a bug. The issue is whether the counts are measurements of a comparable domain at all. [Gate policy](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/core/defaults/policy/gates.mg#L25-L80).

`nerd gates diff` loads named snapshots, or the most recent two saved snapshots when no paths are provided. It prints `keepable: no gate got worse` and succeeds when `Comparison.Keepable` is true. Loading validates JSON and a nonempty snapshot ID, but not a measurement domain. [CLI consumer](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/cmd/nerd/cmd_gates.go#L63-L154), [loading](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree_snapshot.go#L70-L84).

## G1 An omitted gate leaves the judgment domain

Consider two ordinary measurements of the same workspace:

1. The first selects `vet,test` and records both.
2. The second selects `vet`, with vet unchanged.
3. The user invokes `nerd gates diff` over those snapshots.

The test gate has no after row. The inner join in `gate_compared` therefore produces no comparison for it. There is no rule that turns a gate present only before into a missing-measurement obligation. `cycle_keepable` requires the pair and the absence of any derived regression. `Compare` checks only after-side gates, so it does not reject the omitted test. The displayed statement can consequently cover more than the actual comparison established. [Join and keepable rule](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/core/defaults/policy/gates.mg#L29-L32), [negative conclusion](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/core/defaults/policy/gates.mg#L75-L80), [after-side iteration](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree_snapshot.go#L174-L214).

No regression in the omitted tests is required to demonstrate this reporting gap. Their current state is unknown to the pair. If an omitted test did regress, this comparison would not observe it. Conversely, including an unavailable or unverified after row for a previously measured gate is already handled as worse; dropping the row is the distinct case.

A boundary variant is an unknown `--only` value. Selection retains matching measurers without checking that each requested name matched. An unmatched request can produce an empty snapshot with a valid generated ID. Comparing a populated before snapshot to that empty after snapshot makes the join and the Go validation loop empty, yet the negative keepable rule can still succeed. This is an option-validation and domain-admission issue, not a claim that an empty selection runs hidden gates. [Selector and measurement loop](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree.go#L104-L138).

The asymmetry matters: an after-side gate with no matching before measurement can cause `Compare` to return its missing-`gate_change` error. The demonstrated omission schedule is the opposite direction. Do not describe all mismatched gate sets as silently accepted.

## G2 The same gate name can describe a narrower experiment

Let the first G3 measurement run `go test -count=1 -json ./...`. Assume it reports a failure in one package. Let a later G3 measurement select only a different passing package through `--tests`. Both rows are named `test`. The current policy sees fail-to-pass and derives better, although the failed package was not rechecked.

The source explicitly supports these test package options, stores the resulting argv, and then omits argv from policy projection. This is not speculation about Go's test selection: the Go command documentation states that package-list mode compiles and tests the packages listed on the command line. [CodeNERD invocation table](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree.go#L82-L103), [argv recording](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree.go#L213-L234), [Go test contract](https://pkg.go.dev/cmd/go#hdr-Test_packages).

This can happen without any source edit. The narrower test result remains valid evidence for its own package scope. It does not establish improvement over the whole-module result. The same admission boundary should handle explicitly named snapshots from different roots, which the loader and `Compare` do not reject based on workspace identity. Different source revisions, however, are expected in a before/after comparison and must not be prohibited merely because their content hashes differ.

This is separate from the earlier [test-gate memoization identity audit](https://github.com/theRebelliousNerd/codenerd/blob/research-main-20261029-0850/Docs/audits/test-gate-memoization-identity.md). That audit concerns reuse of a turn-local test result under an incomplete cache key. Here both processes may actually run; the error is comparing different measurement scopes as one trend. Main and the eight available `research-main-20261029-*` documentation inventories were screened for gate-comparison overlap.

## G3 An opaque failure can look numerically better

`commandGate.Measure` distinguishes unavailable programs, interrupted or unreadable reports, named findings, and a nonzero exit with no named finding. The last case is stored as `MeasureFail`, with the failure reason and raw output retained. `MeasureErrorFree` sets `Count` to the length of parsed findings. A valid nonzero exit with no parsed findings therefore has count zero. [Command result handling](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree.go#L165-L210), [count assignment](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree.go#L120-L138).

For a same-gate pair with a prior `fail, N` where N is positive and a later opaque `fail, 0`, Mangle derives improvement through its decreasing-count rule. The reader `readCompile`, for example, returns any positioned compile findings and the remaining text without requiring at least one finding. The conditional witness is a command that terminates nonzero with an output form not enumerated as a finding, not a claim that every nonzero command does so. [Compile reader](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/gates/errorfree.go#L236-L241), [improvement rule](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/core/defaults/policy/gates.mg#L50-L54).

The implementation does not change this gate to pass and does not discard its error. The narrower defect is that raw failure evidence is not represented in the ordinal comparison contract. The current projection makes zero enumerated defects indistinguishable from complete enumeration of zero defects inside the fail-count ordering. Adding one synthetic finding alone would not solve the general case: N old findings could still decrease to one opaque failure.

## What this does and does not imply for recurse

The CLI `recurseGateMeasure` captures one fixed option set in the returned closure. Thus the ordinary `nerd campaign recurse --gates` path does not switch `--only` or `--tests` between measurements as G1 and G2 describe. Its independent workspace and node gates, target-resolution requirement, changed-tree requirement, forbidden-path check and metric guards remain active. An error-free comparison is an additional veto, not the only criterion for keeping a cycle. [Fixed options](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/cmd/nerd/cmd_campaign_recurse.go#L305-L322), [ratchet rules](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/core/defaults/policy/recurse.mg#L118-L194).

Recurse projects snapshot facts directly through `recursePolicy.ratchet`; it does not invoke the public `gates.Compare` helper. A validation fix confined to that helper would therefore not repair the shared policy's evidence contract. An opaque failure in an added gate can fail to provide a regression veto, but a claim that a cycle is kept also requires showing that all independent checks allow it. No such executed end-to-end outcome is claimed here. [Direct projection](https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/campaign/recurse_policy.go#L225-L294).

## Proposed comparison contract

Introduce a host-produced measurement manifest and make policy admission depend on it. Go should faithfully measure identity and completeness; Mangle should decide whether that evidence permits a comparison, requires a narrower statement, or leaves a required obligation open.

The manifest should distinguish:

- Logical workspace identity and source revision or dirty-tree identity for each observation. Before and after revisions may differ by design.
- Requested gate set, resolved gate set, and explicit unknown names. Empty requested scope is valid only when the caller explicitly chooses a non-judgmental empty report.
- Per-gate scope: package selectors, relevant build tags and configuration, measurer and parser version, and toolchain identity where available.
- Enumeration status: complete, partial, opaque failure, unavailable or interrupted. Preserve process exit and raw diagnostics separately from the count.
- Collection completion and persistence schema version. A partially assembled set must not look like a complete set with fewer rows.

Avoid raw argv equality as the entire implementation. The build gate adds a temporary output directory, so byte-identical argv would incorrectly reject otherwise comparable runs. Normalize only semantically irrelevant fields with a versioned rule; do not silently discard package selection, tags or environment values that change the experiment. Do not persist secrets while recording environment identity.

For a first conservative version, a full comparison requires equivalent workspace and measurement specifications and the same resolved gate set. A caller requesting an explicit intersection comparison can obtain it, but the result must say which gates were excluded and must not be exported as full-cycle keepability. A changed test selection requires a new baseline or an explicitly limited comparison, not a fabricated pass for omitted packages.

## Proposed algorithm and Mangle responsibilities

1. Resolve requested gate names before execution. Reject unknown names and record the exact selected set. Parse aliases deterministically and deduplicate them.
2. Produce immutable observations keyed by snapshot and gate. Reject duplicate gate identities and inconsistent status/count values on load. Count parsed findings deterministically, while retaining whether enumeration is complete.
3. Construct a comparison request with an explicit expected domain, scope mode and manifest compatibility result. Publish its inputs atomically and seal collection before judgment.
4. Derive missing-before, missing-after, incompatible-scope and incomplete-evidence obligations by joining the expected domain against observations. Negation is meaningful only relative to the positively established expected domain and sealed collection.
5. Derive per-gate trends only for admitted comparisons. An opaque fail is still failure, but not a trustworthy defect-count decrease. Preserve the existing deliberate equal-count replacement policy after admission rather than changing it accidentally.
6. Derive full keepability only when every required gate is covered and comparable, no required evidence is unresolved, and no admitted gate regressed. An explicitly scoped report uses a different conclusion and wording.
7. Feed the same admission facts into CLI diff and recurse. Preserve unknown or incomparable outcomes as visible reasons; never convert them to an empty regression list that implies permission to keep.
8. Retire transient pair facts after consumers finish. Persist compact manifests and verdict provenance alongside snapshots, rather than retaining every historical comparison in the live kernel.

These are proposed responsibilities, not drop-in `.mg` syntax or an implemented schema. They use existing relational joins and negation with an explicit finite domain; no new Mangle engine feature is required. Counts or aggregates can summarize outstanding obligations for display, but the acceptance condition should not rely on an empty aggregate emitting a zero row. Host identity must not be self-asserted by a model control packet.

The larger optimization opportunity is to make partial verification honest enough to compose. A collection of package-scoped measurements could eventually cover a declared full test domain without rerunning unaffected packages, provided coverage, source applicability, invocation identity and dependency validity are established. That is a separate capability, not something implied by matching gate labels today. The build-system correctness framework in Mokhov, Mitchell and Peyton Jones provides useful grounding: correctness is defined relative to tasks, inputs and dependencies, rather than merely matching a stored result label. Applying that idea to gate evidence is the proposal here, not a claimed theorem or novel invention. [Build Systems à la Carte, section 3.6](https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems-final.pdf).

## Costs and failure modes

For G gates and F findings, manifest validation and set comparison can be linear in G plus serialized manifest size with hashed sets, or O(G log G) with sorted deterministic sets. Existing finding projection and diff work still costs O(F) storage and processing before engine effects. A domain manifest does not require rerunning a compiler merely to notice mismatched options. Conversely, repairing missing evidence may require expensive new measurements; those costs belong to the proposal.

Measure complete cost as gate process CPU and wall time, dependency discovery, I/O and hashing, parse work, manifest construction, kernel assertion and fixpoint work, persistence, report rendering, plus any additional retry or repair execution. Include failed, canceled, opaque and rejected comparisons. If a future agent consumes summaries, count prompt tokens, inference and repeated investigation caused by misleading or overly conservative summaries as well. Do not credit fewer tests as efficiency unless the same required assurance was established.

Important risks are overstrict compatibility that forces avoidable rebaselining, normalization that erases meaningful flags, stale manifests, schema migration that invents missing identity, nondeterministic tool output, and complete-looking reports from incomplete readers. A workspace path is not a universal project identity. A starting HEAD is not a dirty-tree snapshot. Requiring identical source revisions would defeat before/after analysis. Unknown older snapshots should remain readable as historical reports, with comparison limitations stated rather than retroactively supplied with guessed metadata.

Separating an opaque failure from count improvement does not establish its cause. It could reflect infrastructure, configuration, an unsupported report or a code defect. Preserve that uncertainty. For autonomous keep decisions, unresolved required evidence should block that assurance claim, while the user may still review and retain the underlying work through an explicitly authorized separate process.

## Fair evaluation plan

No experiments in this section have been run. Compare current behavior, gate-set validation alone, scope identity alone, enumeration completeness alone, and the combined contract on identical inputs. Add an explicitly labeled intersection-report variant, not an intersection result disguised as full assurance.

Deterministic fixtures should include:

- Matching full and matching intentionally partial domains, unchanged controls and genuine improvements/regressions.
- Before-only and after-only gates, empty sets, unknown aliases, duplicate names and missing observation rows.
- Whole-module versus one-package tests, reordered equivalent selectors, changed tags, and changed tool versions.
- Different roots, moved-but-explicitly-mapped workspaces, different source revisions and dirty trees.
- Named failure to opaque nonzero failure; opaque failure to named failure; malformed reports; interruption; unavailable tools; and complete zero findings with successful exit.
- Same-count finding replacement, preserving the current intended policy.
- Temporary build-output paths that differ while semantic invocation remains equivalent.
- Legacy snapshots without manifests, incomplete collection and persistence/load round trips.
- Recurse using its normal fixed options and independent gates, including a case where those gates correctly veto a problem that an added measurement cannot characterize.

At the policy socket, feed the same recorded observations to each variant and require correct domain, trend and unresolved-evidence labels. At the complete-system level, use the real `nerd gates measure` to persisted snapshots to `gates diff` route, then the optional recurse route in an authorized evaluation environment. Score false full-assurance claims, false improvements, missing-domain detection, false incompatibility, retained diagnostics and end-to-end disposition. Keep a trusted oracle describing which packages and gates were actually covered; a favorable exit code is not its own oracle.

Report latency and CPU distributions, fact counts, peak memory, snapshot size, remeasurement cost and actual verified task outcomes. Hold source fixtures, measurement commands, tools and assurance requirements fixed between variants. For an incremental or composed-measurement extension, compare against a fresh full measurement of the same required domain and charge manifest/dependency maintenance. No reliability, token or compute win should be claimed before that comparison.

## Checkable handoff

The first implementation milestone should make the three conditional scenarios unrepresentable as an unqualified improvement or full keepable result. It must also retain legitimate partial reports, changing source revisions, same-count finding replacement, raw failure evidence and independent recurse guards. Update the gates and recurse architecture contracts together, because they share policy but not the same Go comparison entry point.

Completion requires a visible comparison domain, a reason for every excluded or unresolved gate, and one shared admission contract consumed by both CLI and recurse. A count decrease is useful evidence only after the system can say what was counted and whether it finished counting.
