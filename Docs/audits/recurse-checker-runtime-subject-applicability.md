---
doc-class: deep-dive
subsystem: campaign-recurse-gate-applicability
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a
verification-method: static-source-inspection
supersedes: []
---

# Recurse gate applicability must not confuse a checker's runtime with its subject

## Finding and decision

**A project-declared check implemented in Python can be excluded from the Go code it actually checks.** `languageOf` labels every `python` or `python3` invocation as Python, even when it runs an arbitrary project script. `nodeGates` uses that label to suppress the check on a Go-only node. For a canonical `commands.test` wrapper, detection also removes the native Go test gate, while ordinary improvement admission rejects the surviving workspace Test because its inferred language is Python. The test command can have just run successfully over the Go package during workspace baseline measurement, yet the visit reports that its configured test scope cannot certify an improvement. [Declaration and override][declarations] [Classifier][classifier] [Node selection][node-selection] [Improvement admission][improvement]

This is an **invocation-runtime/checked-subject conflation**, not a missing executable, absent test suite, unsupported Go graph, or generic claim that matching language proves test coverage. The code already recognizes the distinction for `go run`: an arbitrary Go program is classified as generic because what it checks is its own business. The corresponding distinction is missing for interpreter-launched project scripts. [Classifier][classifier]

Specify the checked domain separately from the launcher/runtime identity. Preserve known native toolchain adapters, owner-declared checks, exact invocation receipts, Mangle scheduling, permissions and the ratchet. Do not solve the false exclusion by claiming that every interpreter script verifies every node. Unknown subject coverage, an explicitly generic project check, and a supported toolchain adapter need distinct meanings.

**Evidence boundary:** conditional source composition at public main `6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a`, not a runtime reproduction. No repository program, test, build, gate, interpreter fixture, model inference, benchmark or failure injection was executed. No repository source, credential, configuration, permission or setting was changed. Public source copies and this standalone report were the scratch work products. All proposed evaluations below are unrun.

## 1. Architectural contract and reviewed scope

Root guidance assigns creative problem solving to the model and planning, orchestration and safety to the deterministic executive. The campaign mission specification puts deliberation inside already selected recurse work. The portable-recurse proposal expressly preserves `nerd.md` commands as a project-specific escape hatch rather than requiring every project to resemble a built-in toolchain. A host-side applicability classification that excludes the project's actual checker is upstream of the proposer or requirements interrogator. More debate does not repair that input. [Root guidance][root] [Mission specification][mission-spec] [Portable proposal][portable] [Requirements interrogation][interrogator]

The review read root and campaign guidance, architecture documentation guidance, campaign internals and mission design, current gate acquisition and execution, recurse selection and improvement consumers, relevant authored tests, the public audit inventory and queue, and neighboring reports. Implementation claims below are grounded in code rather than historical architecture prose. The seven-file source slice covering declaration/schema, classification, command execution, recurse consumers and angle policy was checked against its fetched Git blob identities. Main was rechecked unchanged at 06:16 UTC on November 4, 2026. [Campaign guidance][campaign-guidance] [Architecture guidance][architecture-guidance] [Campaign internals][campaign-internals] [Inventory][inventory] [Queue][queue]

The finding has two connected manifestations:

1. **Declared node check omission:** the host removes an otherwise available node-scoped checker before invoking it for a foreign-language node. This can affect a lint, test, build or audit entry.
2. **Canonical test liveness loss:** a declared workspace test replaces detected native tests, but its runtime-derived label prevents that workspace test from admitting an ordinary improvement on the actual subject node.

Both arise from the same producer field and applicability join. No second scheduling mechanism or model behavior is needed. The primary witness deliberately chooses one ordinary Go package and one ordinary Python script in another directory, with no mixed-language SCC, package-name collision, stale observation, custom build tags or concurrency.

## 2. Source trace

### 2.1 Project authority is acquired, then narrowed by the launcher

`fromNerdMD` constructs `ScopeAll` gates from `commands.build/test/lint`, and constructs either workspace or node gates from explicit `gates` entries. Both construction paths assign `Language: languageOf(argv)`. `Detect` suppresses every detected gate whose kind appears in canonical `commands`, while keeping explicitly declared extra gates. Thus `commands.test` is a real override, not an optional description beside the built-in test. [Declaration and override][declarations]

The available `GateSpec` fields are ID, kind, command, scope and allowed exits. `Commands` supplies command strings and environment. There is no per-gate checked-language or generic-subject override in those types or this producer. A separate project-wide primary-language tag does not enter either `languageOf` call. Node validation requires `{node}` or `{pkg}`, but does not infer what the script checks. [Schema][schema] [Validation][validation] [Producer][declarations]

`languageOf` uses the basename of `argv[0]`, lowercased with `.exe` removed. It returns Python for `python` and `python3`, without distinguishing `-m pytest` from `tools/check_contracts.py`. Likewise, it returns JS/TS for `node`. It has a specific exception for `go run ...`, returning the generic empty language, and treats unknown programs such as `make` or a direct script path as generic. [Classifier][classifier]

The field comment saying an owner-declared gate has an empty language is not a reliable description of current construction: recognized launcher names produce a nonempty language. The behavior of the assignments and consumers, rather than that comment, establishes the finding. [Gate type][gate-type] [Producer][declarations]

### 2.2 The inferred label becomes an execution filter

`Gate.AppliesTo` accepts an empty gate language for every node, otherwise requiring equality with one of `node.Languages`. `nodeGates` requires both `ScopeNode` and applicability. A normal visit measures only those selected own-node gates before forwarding findings to policy. A filtered node gate therefore supplies neither an invocation nor its findings for that node. The gate is not moved to the unavailable list for this exclusion. [Applicability][applicability] [Availability][availability] [Node selection][node-selection] [Visit][visit]

Workspace baseline measurement is different: it selects `ScopeAll` gates without filtering by node language, apart from the separately documented focused-audit omission. Consequently a workspace Test wrapper can execute and pass while still failing later improvement applicability. This distinction prevents the overstated claim that interpreter-launched checks never run. [Workspace measurement][workspace-measurement]

### 2.3 Improvement admission composes with the canonical override

After policy derives a measurable angle, `improve` requires either an own-node Test or `hasApplicableWorkspaceTest`. That helper checks workspace scope, Test kind and the same `AppliesTo(node.Languages)` relation. If neither route exists, the driver writes `improve_skipped` and returns before `r.run`. [Improvement admission][improvement]

In an otherwise normal Go project with `commands.test: python3 tools/run_go_tests.py`:

- the declared Test is runnable but labeled Python;
- native `go:test` is removed by the canonical Test override;
- `go:vet` can remain, but it is not a Test;
- the Go-only package node fails the declared Test's language join;
- the ordinary improvement is skipped even if the wrapper just ran that package's tests successfully at baseline.

The native Go gate definitions confirm the distinction between the remaining vet and suppressed Test. The subsequent after-check fallback also uses `g.AppliesTo(node.Languages)` when choosing workspace tests for a changed improvement without an own Test. A repair that changes only initial admission would leave this later mismatch. [Native Go gates][native-go] [After-check selection][after-checks]

Both CLI and chat call `RunRecurseCycles`. The pass driver acquires the graph, detects gates, measures the workspace, visits nodes and delegates improvement to this path. Mission preparation is entered only after `r.run` begins. Therefore the skipped improvement has no design attempt for a proposer or challenger to rescue. Other eligible repair attempts may still occur. [CLI][cli] [Chat][chat] [Pass driver][pass] [Visit][visit] [Run-to-mission boundary][run]

## 3. Conditional witness A: a working canonical test blocks its own subject

Use a fresh, clean, admitted temporary Go repository with:

```text
go.mod
nerd.md
lib/lib.go
lib/lib_test.go
tools/run_go_tests.py
```

`lib` contains an ordinary Go package and at least one genuinely collected, passing test. It contains no Python source. The script directory is separate, with no import edge collapsing it with `lib`. Both `go` and `python3` are available through the environment actually used by the host. Membership, command acquisition, parsing, snapshots and ordinary kernel initialization succeed. There are no other declared Test gates, outstanding findings or unresolved recovery records. The optional error-free `MeasureGates` path is not needed.

Relevant `nerd.md` declaration:

```yaml
---
schema: nerd/v1
commands:
  test: python3 tools/run_go_tests.py
---
```

Define the script's intended behavior as the following ordinary wrapper, **not executed or installed in this review**:

```python
import subprocess
import sys

sys.exit(subprocess.run(["go", "test", "-count=1", "./lib"]).returncode)
```

The future fixture must independently qualify that this script, in the command's actual working directory and environment, runs the `lib` tests and faithfully returns their status. A filename or human claim is not that qualification. No special Go flags, environment divergence, shell, tool download or interpreter import hook is required. Python's documented script invocation runs the supplied program, and its `subprocess.run` interface can launch the named command and expose its return code. These are primary language-library contracts supporting the fixture; they are not runtime evidence for CodeNERD. [Python invocation](https://docs.python.org/3/using/cmdline.html#interface-options) [Python subprocess](https://docs.python.org/3/library/subprocess.html#subprocess.run)

Start an ordinary focused recurse pass for `lib` with a measurable improvement angle and no prior visit carried into it. A fresh pass zero supplies the stabilize angle when the tests metric is available. `metrics` publishes the count after a successful source census; the angle rule does not require that the count increase before proposing the attempt. [Metric producer][metrics] [Angle rules][angles]

Static trace under those premises:

| Boundary | Expected source-derived result |
|---|---|
| Graph | `lib` is discovered as a Go node; the script's own Python directory does not change `lib.Languages`. |
| Detection | Runnable `nerd.md:test` has scope all and language Python; detected `go:test` is suppressed. |
| Baseline | The workspace test runs from the root and, by the independently qualified premise, executes `lib`'s real tests successfully. |
| Own-node selection | `lib` can retain native vet, but has no own-node Test. |
| Angle | Policy selects the measurable ordinary improvement angle. |
| Workspace-test applicability | Python does not match the Go-only node. |
| Admission | `improve_skipped` is written; the detail says the configured test scope cannot currently certify the improvement. |
| Execution | No executor callback or mission design request occurs for this skipped improvement. |

Go package discovery and the separate Python source-directory scan supply the fixture's node-language premise. The gate's host working directory is the workspace root, and the declared argv/env travel through `build.WorkspaceCommand`; the source does not force the Python script to test Python files. [Go nodes][go-nodes] [Python nodes][python-nodes] [Gate runner][runner] [Command builder][command-builder]

**Metamorphic control:** replace only the canonical command with `go test -count=1 ./lib`. Qualify that it executes the same Go tests under the same effective environment. The current producer labels it Go, so the workspace-test applicability check admits the improvement. This does not guarantee a kept change: execution, independent evidence, metrics, permissions and ratchet guards still decide that. The contrast isolates the wrapper's implementation runtime from the actual tested subject.

A second control removes the command override and restores detected `go:test`. It checks declaration precedence and the default route, but is less tightly matched because its `-cover` arguments differ. Keep those controls separate.

## 4. Conditional witness B: an explicit node checker is never invoked on its intended node

Use the same ordinary Go node, but replace the canonical Test override with an extra declaration:

```yaml
---
schema: nerd/v1
gates:
  - id: package-contract
    kind: audit
    run: python3 tools/check_contract.py {node}
    scope: node
---
```

The script is an owner-defined checker of a contract in the supplied package directory, not a Python syntax test. Qualify it independently with a controlled contract violation in `lib` and a distinguishing nonzero result when invoked with `lib`. The primary observable is whether that exact invocation happens, not whether a model subsequently understands or fixes its diagnostic.

The declaration satisfies the shown schema shape, its program is available, and `{node}` is present. `languageOf` nevertheless labels it Python. `nodeGates` excludes it for `lib`, so the ordinary visit never runs `python3 tools/check_contract.py lib`. Because it is node-scoped, workspace baseline measurement does not run it as a substitute. Native Go checks can all pass while this independent owner-declared check is absent from the visit. [Schema and validation][schema] [Validation][validation] [Classifier][classifier] [Node selection][node-selection] [Workspace measurement][workspace-measurement]

This is a stronger check-omission consequence than the safe refusal in witness A. It does not establish a falsely accepted commit: there need not be any mutation, and other checks may independently expose the same condition. It establishes that the configured check is not executed for its intended Go node. Its gate identity remains in the set rather than being deleted from every plan.

In an unfocused sweep, the separate `tools` Python node may itself receive this gate with `tools` as the argument. That is not fulfillment of the requested `lib` check. A focused `lib` witness removes that distracting invocation without relying on a missing prerequisite edge or malformed graph.

A portable future control can use the same checker behind a properly qualified language-neutral launcher, or an equivalent `go run` checker. Current classification admits the generic form. Do not recommend adding a shell wrapper merely to disguise the interpreter as a production fix; it adds a launch layer and leaves the missing subject contract unresolved.

## 5. Existing protections and limits

- **A scope filter is useful.** Native `go vet` should not run on an unrelated Python-only directory, and native `python3 -m pytest` should not be treated as a universal test oracle. Preserve supported adapters rather than blanking every language label.
- **The graph can be correct.** These witnesses require no lost import, empty fallback node, SCC language union or cross-cutting ID collision. `lib` is an ordinary, correctly discovered Go package.
- **Availability remains real.** The interpreter is installed. PATH disagreement, missing virtual environment and failed command startup are independent negative controls, not necessary causes.
- **Workspace measurements remain visible.** Witness A specifically credits a successful baseline invocation; there is no claim that its output vanishes or becomes a fabricated pass.
- **Canonical override is intentional.** Automatically restoring the native test beside the owner's command may violate project command authority and still omit setup/custom assertions. Correct the applicability contract rather than defeating precedence.
- **An ordinary skip is not an unsafe execution.** Witness A prevents work. Witness B omits a declared checker. Neither proves an accepted defective change or a permission bypass.
- **Coverage is separate.** Correctly admitting a wrapper does not prove it selected every changed file, collected tests, or ran a sufficient behavioral oracle. Existing no-tests, source identity, test witness and mission evidence obligations remain.
- **Model roles remain bounded.** A proposer cannot broaden scope or classify an arbitrary executable as safe merely by explaining it. Host-supported domain evidence or an admitted owner declaration is needed.

## 6. Nonduplication

The public inventory and queue were screened, and neighboring reports were read. Code searches for `languageOf`, interpreter/gate and wrapper/language combinations in audit documents did not identify a dedicated report of this producer-to-consumer mechanism. That is a bounded current-public-main screen, not a claim about private work, all history or unindexed branches. [Inventory][inventory] [Queue][queue]

| Existing report | Owned boundary | Difference here |
|---|---|---|
| [Portable recurse/deliberation crucible][portable] | Capability declarations, portability limits and behavioral evidence. | This supplies a concrete failing applicability join within its owner-declared-command escape hatch. |
| [Recurse gate language domain][prior-path-language] | Correct constituent path languages are erased when SCCs collapse; Go workspace scoping selects foreign paths. | One correctly typed Go path is enough here. The error is the checker label before any SCC or path expansion. |
| [Recurse workspace test witness][prior-workspace-test] | Earlier omission of workspace Test remeasurement, with a current repair admitting applicable workspace tests. | The repaired route exists, but an arbitrary interpreter wrapper is falsely deemed inapplicable. |
| [Recurse Cargo default-member evidence][prior-cargo] | Matching Rust labels can admit a command that did not select the changed crate. | This is false exclusion despite independently qualified subject coverage, not false inclusion from too-coarse matching. Both constrain a sound domain design. |
| [Recurse Go workspace acquisition][prior-go-workspace] | `go.work` root fails to acquire Go package nodes/language. | Here `go.mod`, real Go nodes and their language are present and correct. |
| [Gate declared-PATH executable selection][prior-path] | Availability and launch select the wrong executable search domain. | The intended interpreter is available and selected. No lookup ambiguity is required. |
| [Recurse gate subsumption][prior-subsumption] | Same-kind/language classification wrongly eliminated an independent workspace finding. | This filter acts before a node invocation, or before improvement dispatch, not in the repaired finding collector. |

The reports already establish that classification is not coverage. The novel concrete contribution is **arbitrary script launcher classification → foreign-node gate exclusion**, plus its canonical-test-override consequence. Do not relabel the broad domain principle itself as new.

## 7. Proposed pinned contracts and bounded implementation direction

Update campaign/gate capability and current-state documentation before implementation. These are proposed contracts, not existing fields or completed repairs.

| Boundary | Required contract | Acceptance condition |
|---|---|---|
| Invocation identity | Preserve the exact authorized program, arguments, working directory, environment identity and expected exits independently of subject metadata. | Adding subject metadata does not change which executable or arguments run. |
| Runtime identity | Describe the launcher/toolchain used to execute the checker. | `python3 script.py` may say Python runtime without asserting a Python-only subject. |
| Checked subject | Carry supported adapter evidence, an explicit admitted owner declaration, or unknown; permit multi-language and generic domains without pretending they are exact coverage. | Qualified Go wrappers are not excluded merely for using Python. |
| Node selection | Evaluate the declared/adapter subject against the admitted node/path domain. | Witness B invokes the checker for `lib`; unrelated restricted native checks remain excluded. |
| Improvement admission and after-check | Use the same subject contract at both consumers. | Witness A can enter the normal attempt and rerun its actual test afterward; no admission-only patch. |
| Evidence | A runnable applicable command is still not proof of execution, selected tests or freshness. | Missing/failed/cancelled/no-tests and wrong-subject evidence cannot silently certify completion. |
| Unknown or incompatible declaration | Surface an explicit unresolved/unsupported applicability state with the gate and node identities. | A well-formed but unqualified script is not silently omitted or promoted to universal coverage. |
| Change and replay | Bind subject interpretation to the command/configuration generation; reconsider after declaration or script/profile changes. | Old receipts do not acquire new coverage merely because a domain field changed. |

A reasonable first implementation extends the current `Gate`/project-declaration path, not the scheduler architecture. Retain narrow recognized command adapters such as Go test/vet and Python `-m pytest`; distinguish those from arbitrary script execution. An owner can express that a checker applies to Go, several named domains or a generic project contract. Decide explicitly how older declarations migrate, and expose unknowns rather than silently interpreting all legacy interpreter commands as universal.

Do not statically inspect arbitrary script bodies to claim complete subjects: dynamic process selection, configuration, plugins and conditionals make that a separate program-analysis problem. Do not run scripts merely to discover what they might check. A fixture qualification or actual authorized check can supply evidence later, but domain metadata must not grant new execution permissions.

Preserve the current common execution builder and known gate identities, allowed exits, no-tests meaning, environment handling, invocation-bound receipts and failure classification. Expose a bounded applicability explanation for plan/status so a runnable-but-excluded checker can be distinguished from unavailable tools and actual failed checks. Let host code acquire the facts and existing executive policy derive decisions; do not ask the model to infer away a safety guard.

## 8. Costs and failure modes

No latency, token, quality or cost improvement has been measured.

- **Host cost:** additional declaration parsing, validation, adapter matching and applicability projection. An explicit metadata design should not need an extra interpreter launch or model call for classification.
- **Real work increases:** recovering omitted checks and improvements adds subprocesses, possible repair attempts and model calls. A cheaper run that skipped required work is not the fair efficiency baseline.
- **Configuration cost:** owners need a small clear way to state subjects and resolve unknowns. A global primary-language tag is insufficient for polyglot or cross-language checkers.
- **Compatibility:** changing generic semantics can broaden execution on existing repositories. Preserve authority and make behavior changes explicit; do not silently run every formerly filtered script everywhere.
- **Underclaim and overclaim:** an incorrectly narrow declaration repeats this omission; an overly broad one can test the wrong component and accidentally satisfy a later witness. Neither launcher identity nor owner metadata alone proves executed coverage.
- **Invalidation:** command strings, script/configuration inputs and adapter revisions can change subject interpretation. Reuse must not inherit stale applicability without generation checks.
- **Cross-platform behavior:** interpreter aliases, `.exe`, direct scripts, paths with spaces and virtual environments need tests, but none justifies assuming that runtime language equals subject language.
- **Bounded failure:** unsupported launch forms, unavailable tools, denied execution, contradictory domain declarations and stale receipts must remain observable non-success states. Do not retry by weakening permissions or bypassing project commands.

The full cost ledger for a later comparison should include gate CPU/wall time and output bytes, acquisition work, artifact/storage overhead, every failed/retried model call, input/output/cache token categories, known and unknown spend, user interventions, false exclusions, unnecessary inclusions, completed qualified checks and independently supported final outcomes.

## 9. Evaluation plan, entirely unrun

### A. Deterministic semantic contract

Use disposable public/synthetic workspaces and pinned Go/Python toolchains. Independently qualify each checker's actual subject and return behavior before using it as an oracle. A failed fixture precondition is not a CodeNERD counterexample.

| Case | Required result |
|---|---|
| Witness A Python-wrapped Go test | Baseline, ordinary improvement admission and after-test selection agree about the declared Go subject. |
| Equivalent direct Go test | Same subject applicability and invocation outcome; retain distinct exact invocation identities. |
| Witness B Python node checker | Exact `lib` invocation occurs; the controlled violation is not omitted solely because `lib` contains Go. |
| Arbitrary Node-launched checker of Go/Rust/filesystem contracts | Runtime and checked domain remain distinct. |
| Native `python3 -m pytest` on a Go-only node with no declared cross-language role | No accidental universal applicability. |
| Native Go test/vet on Python-only node | Existing exclusion remains. |
| Generic `make`, direct script, and existing `go run` checker | Preserve intentionally generic behavior while avoiding unsupported exact-coverage claims. |
| Multiple supported subject languages | Include intended subjects, exclude unrelated ones and preserve per-path SCC concerns separately. |
| Unknown script subject | Visible unresolved disposition, not silent omission or universal proof. |
| Missing interpreter or PATH/env mismatch | Existing unavailable/startup-error behavior; no success substitution. |
| Command/profile/script changes between baseline and after-check | Refresh applicability/evidence; no stale domain or receipt reuse. |
| Test returns no tests, fails, is cancelled or runs an unrelated package | No claim of a sufficient test witness merely because the domain matches. |
| Owner command override | Do not silently resurrect native defaults to mask the missing domain. |

Existing tests were read, not run. `TestLanguageOf` covers native Python pytest, generic Go run, make and direct scripts; it does not include an arbitrary Python script checking Go. `TestRecurseOpen_RetainsIndependentWorkspaceGates` supplies already-typed gates and guards the repaired collector, not command-domain acquisition. Neither establishes this joined behavior. [Classifier test][classifier-test] [Independent-finding test][collector-test]

### B. Exercise the normal production paths

After an authorized implementation exists, exercise the actual plan and CLI/chat pass routes with normal world-model/CodeDOM infrastructure enabled, using deterministic executor doubles first. Capture detected gate descriptors, applicability dispositions, exact command invocations, graph node languages, `improve_skipped` rows and executor-call counts. A unit test that manually constructs a corrected `Gate.Language` does not test the failing producer.

For witness A, first establish an unchanged passing baseline and the current skipped-improvement result, then require the candidate implementation to enter the bounded attempt and execute the qualified after-test. For witness B, inject a harmless controlled contract violation and observe the exact checker invocation and finding path. Do not conflate successful dispatch with successful repair.

Only after those host contracts hold should a separately authorized live comparison involve model proposals. Keep repository snapshots, task wording, provider/model/effort, JIT corpus, permissions, budgets and checker semantics fixed. Compare current inference, explicit subject metadata and any adapter-only alternative over both affected fixtures and conventional projects. Include held-out wrappers, negative controls and unavailable cases. Report quality at equal total cost and cost at equal independently supported outcomes. Count extra valid checks as required work, not a regression hidden by call-count optimization.

### C. Stopping and acceptance

The narrow finding is closed when the same qualified checker is not excluded from its actual subject merely because it uses another runtime, both admission and subsequent measurement use one coherent domain contract, unsupported subject knowledge is visible, and native toolchain checks are not broadened indiscriminately. This does not close general test-selection fidelity, behavioral sufficiency, SCC path typing, or all portable-recurse capability gaps.

## Source references

[root]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/agents.md
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/architecture/AGENTS.md
[campaign-internals]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/architecture/campaign/INTERNALS.md
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/architecture/campaign/MISSION-IMPROVEMENT.md
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/.agents/skills/requirements-interrogator/SKILL.md
[inventory]: https://github.com/theRebelliousNerd/codenerd/tree/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/audits
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/audits/ASTRA_QUEUE.md
[declarations]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/gates/detect.go#L15-L90
[classifier]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/gates/detect.go#L93-L118
[gate-type]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/gates/gates.go#L60-L87
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/projectdoc/nerdmd.go#L167-L209
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/projectdoc/nerdmd.go#L518-L547
[applicability]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/gates/gates.go#L115-L127
[availability]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/gates/gates.go#L154-L196
[node-selection]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_cycle.go#L993-L1007
[workspace-measurement]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_cycle.go#L700-L751
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_cycle.go#L1052-L1105
[improvement]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_cycle.go#L1181-L1225
[after-checks]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_cycle.go#L1324-L1349
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_cycle.go#L1575-L1586
[angles]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/core/defaults/policy/recurse.mg#L331-L370
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_cycle.go#L515-L612
[run]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_cycle.go#L1228-L1300
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/gates/run.go#L45-L109
[command-builder]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/build/workspace_command.go#L31-L64
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/cmd/nerd/cmd_campaign_recurse.go#L253-L290
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/cmd/nerd/chat/campaign_recurse.go#L218-L260
[classifier-test]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/gates/detect_test.go#L320-L336
[collector-test]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_cycle_test.go#L513-L538
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/audits/portable-recurse-deliberation-crucible.md
[prior-path-language]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/audits/recurse-gate-language-domain.md
[prior-workspace-test]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/audits/recurse-workspace-test-witness.md
[prior-cargo]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/audits/recurse-cargo-default-member-evidence.md
[prior-go-workspace]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/audits/recurse-go-workspace-acquisition.md
[prior-path]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/audits/gate-declared-path-executable-selection.md
[prior-subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/Docs/audits/recurse-gate-subsumption.md
[native-go]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/gates/detect.go#L123-L132
[go-nodes]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_workspace.go#L392-L449
[python-nodes]: https://github.com/theRebelliousNerd/codenerd/blob/6d427d6c7eb8903071ed8ce3e9fcd3b80b5a590a/internal/campaign/recurse_workspace.go#L461-L481
