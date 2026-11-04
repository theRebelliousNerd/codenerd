---
doc-class: deep-dive
subsystem: campaign-recurse-gate-measurement
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 8909d7c3b23e42b5b25b5c102129de64f6400d80
verification-method: static-source-and-test-definition-inspection
supersedes: []
---

# Recurse workspace baselines need gate interference admission

## Finding and decision

**Recurse can manufacture a failed workspace baseline by running individually healthy, self-contained project checks concurrently against the same build outputs.** Its workspace measurement launches up to four gates at once without a compatibility or isolation condition. Package-script detection admits both a build script and a test script as workspace gates. A test script may correctly build its own prerequisites through `pretest`, yet the separately launched build can remove or replace those same outputs while the test consumes them. The driver retains that ordinary nonzero exit as a failed gate after all processes finish. [Parallel measurement][baseline] [Script detection][js] [Result classification][run]

The recommended first correction is conservative: **serialize workspace gates whose shared-output compatibility is unknown, while retaining bounded parallelism for checks with an admitted noninterference or isolation basis.** Use the same execution contract for baseline and candidate measurements. Do not weaken the ratchet, call a real nonzero exit a pass, discard an inconvenient gate, or ask the model to explain away a host-created failure.

This is a conditional source-composed correctness and portability finding, not a reproduced race, measured incident rate, demonstrated false-green commit or claim that every parallel gate pair conflicts. Public main resolved at approximately 06:37 UTC on November 4, 2026 to [`8909d7c3b23e42b5b25b5c102129de64f6400d80`][revision] and was rechecked unchanged at approximately 06:47 UTC. All codeNERD links below pin that revision. Guidance, architecture, implementation, the current audit queue, neighboring reports and relevant authored tests were read. No application, compiler, gate, build, test, benchmark, failure injection or model inference was executed. No implementation, repository file, credential or setting was changed. The witness and evaluations below are unrun.

## Why this belongs in the recurse contract

The forever-loop journey makes host-measured gates the basis for finding selection and before/after retention. Root guidance separates creative problem solving from deterministic executive control. A failure introduced by the measurement schedule is therefore upstream of both the repair model and the policy deciding which finding to fix. Better interrogation cannot reconstruct a check's missing stable inputs from a bare failed-process receipt. [Journey][journey] [Root guidance][root]

This is a foreign-repository concern, not a demand that every supported toolchain build like codeNERD. Project checks are allowed to be package scripts or owner-declared commands. Those commands often compile, generate, clean, cache or stage files before checking them. Correct execution in isolation does not imply safe simultaneous execution against one checkout.

The production route is shared: CLI and chat call `RunRecurseCycles`. The first pass discovers gates and measures its workspace baseline before visiting nodes. Mission mode calls the same measurement helper; its optional before/after snapshot capture does not serialize these processes or add an interference disposition to their ordinary verdicts. No mission is required for the primary witness. [CLI][cli] [Chat][chat] [Pass admission][pass] [Measurement and snapshot handling][measure]

## Source trace

### 1. Detection proves availability, not compatibility

`jsGates` independently adds `js:build`, `js:test` and, when present, `js:lint`, each as `ScopeAll`. With no alternative package-manager lockfile, build and test become `npm run build` and `npm run test`. The test script need only exist and not contain npm's placeholder phrase. The detector does not inspect its lifecycle hooks or the build's output domain. [Detected commands][js]

That is appropriate for preserving the project's commands, but it does not establish that they can overlap. npm documents that `npm run <name>` executes its matching pre/main/post lifecycle, and that scripts run from the package root. Thus a `pretest` that invokes the project's build is a supported producer, not a missing dependency invented by this audit. [npm lifecycle and working directory][npm]

Both `Gate` and the explicit `GateSpec` describe invocation, kind, scope and pass interpretation, but neither carries a parallel-safety declaration, output/resource domain or prerequisite relation. `split` partitions executable availability and sorts by ID. Being runnable and sorted is not an admission proof for concurrent execution. [Gate type][gate] [Project declaration][schema] [Availability and ordering][split]

### 2. Workspace baseline launches unrelated gate processes into one mutable root

`measureWorkspace` collects workspace gates, allocates one result slot per gate, creates a semaphore with capacity four, and starts one goroutine per gate. Each admitted goroutine calls `r.measure(ctx, gate, "", nodes)`; the function waits for all of them and then installs every result into `r.state`. No gate-kind barrier or shared-output exclusion appears in this path. The focused-sweep audit exclusion does not protect a build/test pair. [Parallel baseline][baseline]

`measure` calls `gates.Run` with the same `r.root`. `gates.Command` calls `build.WorkspaceCommand`, which sets `cmd.Dir = root`. The process wrapper adds noninteractive input, bounded pipe waiting and process-tree cancellation, not per-checkout filesystem isolation or serialization. The Unix and Windows start helpers each start their own process scope. A process group or Windows job object is not a separate build directory. [Measurement][measure] [Command][run] [Workspace command][command] [Process wrapper][process] [Unix start][unix] [Windows start][windows]

The workspace's recurse owner lock excludes a second recurse loop. It does not distinguish or lock the gates deliberately launched by the one admitted owner. This witness needs no competing user edit, second agent or concurrent recurse process. [Owner admission][owner]

### 3. Completion of all producers does not repair an already sampled failure

A process that starts, exits normally with an unaccepted nonzero exit, and is not canceled becomes `Passed == false` with no `Err`. That is an ordinary failed result, not `Unverified`. `FindingsAll` extracts diagnostics and, when no parser understands the output, retains a fallback finding. After the goroutines join, the baseline stores those results without retrying a check whose inputs another gate changed. [Exit classification][run] [Finding conservation][findings] [State publication][baseline]

The retained `state` is explicitly described as the current tree's last measurements. Located workspace findings flow through `open` into node visits; unlocated failures are reported separately rather than assigned to an arbitrary node. Neither path identifies self-interference. The distinction affects what happens next, but both are incorrect as evidence of a stable project defect in the witness. [State contract][state] [Candidate handoff][open] [Unattributed boundary][unattributed]

The candidate path uses a different schedule. After a changed attempt, it executes own-node checks in a loop, then selected workspace checks in another loop, calling the synchronous `measure` each time. For the detected JS pair, a selected test finding or a workspace-tested improvement can therefore have a concurrent baseline and serialized after-checks. A change in measured outcome may partly reflect the host's changed schedule rather than the candidate's contents. That comparability risk is source-supported; this report does not claim a particular edit would satisfy all ratchet or mission conditions. [After-check schedule][after]

## Conditional witness: two self-contained npm checks

### Preconditions

Use a generic, admitted npm package in an otherwise stable checkout:

- The package has healthy tracked source and tests. All required local tools and dependencies are already available.
- Its `build` script runs a synchronous producer that removes its disposable `dist` directory and then regenerates it from the current source. A clean-before-compile wrapper is sufficient. The generated tree is ignored; no tracked source, policy, test or oracle is changed.
- Its `pretest` script invokes that same build. Its `test` script then loads a generated module from `dist` and checks it. No detached process continues after an invocation returns.
- Both `npm run build` and `npm run test` succeed when run individually or one after the other on this source. The test is self-contained: it does not assume codeNERD ran a separate build first.
- No special script-level lock or isolated output directory prevents simultaneous builds. No extra lint gate is needed.
- Recurse reaches its first workspace measurement with an active context. Every process start succeeds and all output is complete.

The cleaning behavior is a stated property of this fixture, not a claim that `tsc` itself deletes its output directory. TypeScript's documented `outDir` is one ordinary destination for emitted files; the same witness can use a small synchronous custom producer or another build tool. [TypeScript output directory][ts-out]

### Allowed interleaving

Name the independently detected build invocation **A**, and the detected test invocation, including its lifecycle hooks, **B**.

1. Both acquire a baseline semaphore slot. A has started but has not yet performed its clean.
2. B's `pretest` completes its own clean and build. The generated module now exists and is correct.
3. Before B's test loads that module, A performs its clean and removes it.
4. B tries to load the missing module and exits with a normal failing status. Its own prerequisite build had succeeded; another gate invalidated it.
5. A completes regeneration and exits successfully.
6. `measureWorkspace` joins both invocations, stores A's pass and B's failure, and publishes the finding. The final generated output is healthy and a fresh serialized B would pass without any source repair.

No claim is made about how frequently this interleaving occurs. The source supplies the concurrent admission and shared root; the fixture supplies a legal output collision. A controlled future evaluation should use process barriers to select this order rather than hoping sleeps happen to reproduce it.

The smallest established failure is **an ordinary failed baseline retained for a healthy stable source because the harness permitted another measurement to invalidate a self-contained check's inputs**. An unlocated module-load failure can be the fallback finding, so no parser improvement is required to establish this defect.

For a separate candidate-selection check, use a project test reporter that supplies a real source/test location when its generated prerequisite is missing, and verify current attribution. That extension must not be presented as already established by Node's default error format. A location-bearing result can enter repair selection; an unattributed result remains visible but is not visited.

### Persistence is conditional, not necessarily self-healing

On the ordinary `gates.Set` route, node visits remeasure only their own `ScopeNode` gates; the optional error-free refresh measures separate state. This detected JS pair is workspace-scoped. Pass start skips a fresh workspace measurement when `r.state` is nonempty, and pass end repeats it only when the kept count increased. Accordingly, if a pass makes no kept change, the failed workspace observation need not be independently rechecked in the next pass. An ordinary future changed attempt or kept pass can refresh it; this audit does not predict what a model will do. [Visit schedule][visit] [Pass refresh conditions][pass]

This matters even when no erroneous edit is retained. The loop can report an artificial open failure, spend repair effort on it if attributed, or compare it with a differently scheduled candidate result. Runtime prevalence, wasted tokens and retained-change consequences remain unmeasured.

## Prior work and novelty boundary

The current audit inventory and these neighboring reports were checked. The contribution is deliberately narrow:

- **Post-verification effects** concerns effects first produced after the executor's changed-path census and omitted from adoption/compensation. Its final risk section explicitly leaves parallel baseline effects outside its witness. Here no executor has run, no source/configuration/oracle effect exists, and all generated output may remain legitimately disposable. The problem is one check invalidating another check's inputs during measurement. Capturing all final source diffs would not fix it. [Prior effect boundary][prior-effects]
- **Workspace-test witness** concerns whether a fresh workspace test is scheduled for improvement retention. Its current-main addendum describes that correction. This report assumes the test is scheduled and actually runs; the error is unsafe simultaneous execution during the baseline. [Prior test scheduling][prior-test]
- **Gate subsumption** concerns withholding an independently measured finding because another gate has matching kind/language. This witness uses distinct discovered gates and preserves their findings. Suppressing one as a supposed duplicate would lose a real obligation, not establish safe composition. [Prior subsumption][prior-subsumption]
- **Finding census completeness** concerns retaining findings and target-presence information from a gate's output. Complete logs and an exact census do not establish a stable observation if the runner itself changed the input underneath that gate. [Prior census][prior-census]
- **Compensation ownership** concerns unrelated user writes during an attempt. There is only one operation owner here, and the interfering writer is another check it launched. [Prior ownership][prior-ownership]

This is not a proposal to infer all build dependencies from arbitrary shell text or to replace the subsystem source-import DAG. The missing decision is whether two admitted gate processes may share a mutable execution domain at the same time.

## Proposed contract and bounded correction

### Invariants

1. **Noninterference before overlap.** Two gates may overlap only with an admitted basis that their reads/writes and external resources are compatible, or execution isolation that preserves each command's required environment and inputs. Unknown project-script effects do not imply read-only behavior.
2. **Comparable measurements.** Baseline, candidate and pass-end checks use the same scheduling/isolation contract, except where a changed contract is explicitly admitted and the baseline is re-established.
3. **Honest disposition.** A known or unresolved interference invalidates the observation as project-defect evidence. Preserve the actual process exit and output, mark the observation inconclusive/contaminated, and obtain a safe fresh result before using it to select a source repair or claim improvement.
4. **Lifecycle ownership.** Cancellation still joins active checks and releases resources before publishing a baseline. The existing wait-before-publication behavior is useful and must survive any scheduling repair.
5. **No weakened oracle.** Do not mark a failed command passing, omit the test, accept missing output as success, or move the disposable directory into source solely to make its writes visible.

### Smallest useful implementation direction, not implemented here

Begin with serial execution for opaque workspace commands. This directly excludes the self-contained collision witness without inventing dependencies or modifying the project's scripts. Preserve a bounded parallel path only when its compatibility basis is explicit and reviewable. A single serial group for one checkout is a safe initial baseline; more precise output/resource groups can be introduced when measurements justify them.

Because root guidance assigns executive control to Mangle, express admission facts and the chosen scheduling disposition through the existing policy boundary rather than letting an LLM decide which checks “probably” do not conflict. Go should observe the command/resource declarations, enforce the admitted schedule and collect results. No model call is intrinsically necessary.

Do not treat alphabetical ID order or “build before test” as the general solution. The primary witness is fixed by non-overlap in either order because both checks are self-contained. Separate checks that genuinely depend on one another need an explicit project-owned pipeline or dependency relation. Two builds, lint generators or audit commands can also share outputs. Reusing a common gate execution plan across baseline and candidate paths avoids reproducing the same gap in two implementations.

Isolated workspaces are an alternative, not a free optimization: dependencies, generated inputs, absolute paths, tool environments, databases, ports and caches may still be shared. Merely cloning tracked files can omit the very inputs the check requires. Do not silently promote an isolated check's verdict to the live workspace without a defined input/output applicability boundary.

## Unrun evaluation and acceptance

### Mechanical witness first

Use a disposable fixture with two ordinary discovered npm gates and barriers outside the source-under-test domain. Confirm each invocation's healthy serial control before forcing the six-step schedule above through the real `Detect -> measureWorkspace -> gates.Run` path.

Record the process order, clean/publish/read events, actual exit statuses, final artifact contents, stored verdicts and emitted findings. The expected current-source result is conditional, not an executed receipt. Then run the same fixture against a candidate scheduling design with identical commands, source and tool versions.

Required controls:

| Case | Required observation |
|---|---|
| Self-contained build/pretest collision | No concurrent invalidation is admitted, or the affected result is withheld as inconclusive and safely remeasured. |
| Real broken source | A serialized genuine failure remains failed and selectable/reportable; repair must not hide it. |
| Pure independent checks | Authorized parallel execution remains possible when its compatibility basis holds. |
| Two output-producing gates with different kinds/IDs | Classification and string ordering alone do not confer independence. |
| Genuine prerequisite pipeline | Missing dependencies are explicitly admitted or reported; serial unknown order is not claimed to solve them. |
| Canceled queued or running check | Active processes are joined; no partial result vector becomes a successful baseline. |
| Failed prerequisite or artifact publication | The consumer is not represented as having measured a stable successful input generation. |
| No kept changes across passes | A contaminated observation is not perpetually reused as current project truth. |
| Source tree unchanged but disposable output collides | The admission still works; a tracked-source snapshot alone is not the oracle. |
| Legitimate unchanged caches and disjoint output domains | Avoid unnecessary invalidation or blanket write bans. |

Inspect existing tests without overstating their scope. The package-script detection fixture asserts gate IDs, selected package manager and argv. The gate runner fixtures inspect individual commands and process failure/cancellation. The recurse cycle and audit fixtures exercise retention, recovery and workspace-test scheduling. Those read definitions do not establish a shared-output parallel-baseline witness. This is not a claim that no test anywhere covers concurrency. [Detection fixture][detect-test] [Runner fixtures][run-test] [Cycle fixtures][cycle-test] [Audit fixtures][audit-test]

### Production qualification and fair comparison

After the deterministic boundary is established, qualify both normal CLI and chat entry paths on a small non-Go fixture with real local tool processes, then a repository-disjoint sample including self-contained npm scripts, independent Go checks, Rust tooling with its own locks, owner-declared generator/check pairs and mixed-language projects. Preserve world-model context, permissions and campaign acceptance. Do not substitute an injected-executor result for end-to-end qualification.

Compare current concurrency, conservative serialization and admitted isolated/disjoint concurrency using the same repositories, clean/warm artifact states and tool versions. Report process launches, elapsed time, CPU, peak memory, artifact I/O, repeated measurements, contamination detections, genuine failure preservation and false holds. Measure model calls/tokens only in a separately authorized later full-route study. No performance or productivity improvement is claimed here.

### Costs, failure modes and exit criterion

Serialization can raise wall time from overlap toward the sum of gate durations; the existing cap of four is a real latency optimization. Isolation adds disk, copying and dependency setup; precise compatibility declarations add maintenance and can become stale. Retry-based handling adds commands and can repeat side effects. Bound retries by an explicit observation/admission outcome rather than looping indefinitely until a lucky pass appears.

Principal repair failures are assuming build/test independence from their names, treating ignored output as irrelevant, hashing only final source state, declaring every nonzero exit a flaky pass, suppressing duplicate-looking obligations, adding a lock that nested lifecycle scripts deadlock on, losing cancellation joins, and applying safer scheduling only to the initial baseline but not later measurements. A process that deliberately detaches work is outside the synchronous primary witness and needs its own lifecycle treatment.

**Closure:** the self-contained collision can no longer become an ordinary current failed-project baseline solely because another admitted measurement removed its inputs. Independent real failures stay visible; execution costs and compatibility limits are explicit; the same scheduling contract governs the before/after comparison. No broader claim of universal build isolation or complete foreign-repository understanding follows.

## Sources

[revision]: https://github.com/theRebelliousNerd/codenerd/commit/8909d7c3b23e42b5b25b5c102129de64f6400d80
[root]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/agents.md#L1-L30
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/Docs/journeys/10-forever-loop.md#L35-L55
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/cmd/nerd/cmd_campaign_recurse.go#L253-L283
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/cmd/nerd/chat/campaign_recurse.go#L218-L252
[js]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/gates/detect.go#L173-L213
[gate]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/gates/gates.go#L61-L90
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/projectdoc/nerdmd.go#L167-L207
[split]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/gates/gates.go#L170-L196
[baseline]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle.go#L699-L751
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle.go#L754-L839
[run]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/gates/run.go#L15-L99
[command]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/build/workspace_command.go#L31-L62
[process]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/processutil/command.go#L28-L50
[unix]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/processutil/proc_tree_other.go#L10-L32
[windows]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/processutil/proc_tree_windows.go#L14-L81
[owner]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle.go#L384-L390
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/gates/findings.go#L100-L203
[state]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle.go#L269-L287
[open]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle.go#L993-L1031
[unattributed]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle.go#L851-L881
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle.go#L515-L612
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle.go#L1052-L1085
[after]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle.go#L1328-L1349
[prior-effects]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/Docs/audits/recurse-post-verification-effects.md#L185-L191
[prior-test]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/Docs/audits/recurse-workspace-test-witness.md
[prior-subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/Docs/audits/recurse-gate-subsumption.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/Docs/audits/recurse-finding-census-completeness.md
[prior-ownership]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/Docs/audits/recurse-compensation-write-ownership.md
[detect-test]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/gates/detect_test.go#L107-L134
[run-test]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/gates/run_test.go#L25-L86
[cycle-test]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_cycle_test.go#L119-L198
[audit-test]: https://github.com/theRebelliousNerd/codenerd/blob/8909d7c3b23e42b5b25b5c102129de64f6400d80/internal/campaign/recurse_audits_test.go#L362-L386
[npm]: https://docs.npmjs.com/cli/v11/using-npm/scripts/
[ts-out]: https://www.typescriptlang.org/tsconfig/outDir.html

