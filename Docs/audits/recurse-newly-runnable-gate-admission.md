---
doc-class: deep-dive
subsystem: campaign-recurse-gate-admission
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: f52db4a9224cd9cc8b97619eefb1c5f3169fb329
verification-method: static-source-inspection
supersedes: []
---

# Newly runnable recurse gates need first-observation admission

## Finding and decision

**Recurse can rediscover that a formerly unavailable workspace check is now runnable, remove its unavailable warning, and finish the next pass without ever running it.** The pass redetects the gate set, but decides whether to obtain a workspace baseline from whether the *entire* result map is empty. A result from an unrelated check makes that map nonempty. The newly runnable check has no result of its own; candidate collection reads its missing map entry as an empty finding list. With no kept attempt, the closing measurement is also skipped. [Pass admission][pass] [Availability reporting][availability] [Candidate lookup][open] [Pass close][close]

The primary conditional witness keeps the source, dependency graph, project document, gate IDs and command arguments unchanged. Only the availability of an already declared checker changes between passes. It needs no model mistake, edit, commit, bad graph attribution, collision, corrupted journal or reused passing result. The result is **an omitted measurement and repair opportunity**, not proof that a failing process was labeled passing or that a defective patch was accepted.

**Recommended bounded correction:** reconcile the current required workspace checks with their own observations before visits consume the measurement state. A check with no observation must be measured or explicitly remain unmeasured; being runnable is not evidence that it ran. Preserve ordinary compatible reuse, intentional focus exclusions, finite budgets, host execution, and Mangle's ownership of repair selection and completion.

**Evidence limit:** this is a source-composed conditional finding, not an executed incident. No test, build, repository program, gate command, model inference, benchmark or fault injection was run. No implementation, repository file, credential, permission or setting was changed. Public-source copies and this standalone report are the research artifacts. Every fixture, evaluation and performance outcome below is proposed and unrun.

## 1. Source boundary and intended behavior

The inspected public `theRebelliousNerd/codenerd` main is [`f52db4a9224cd9cc8b97619eefb1c5f3169fb329`][pin], resolved on November 4, 2026 and rechecked unchanged at 03:16 UTC. The complete recursive tree, root and campaign guidance, architecture guidance, campaign internals, mission specification, forever-loop journey, requirements-interrogator guidance, current audit inventory and relevant earlier reports were inspected before selecting this boundary. Ninety-six retained source files were checked against the pinned tree's Git blob identities. All repository references below are immutable. [Root][root] [Campaign guidance][campaign-guidance] [Architecture guidance][architecture-guidance] [Campaign internals][internals] [Mission contract][mission-spec] [Journey][journey] [Interrogator][interrogator]

The intended system is portable, continuously improving work against the workspace's own checks. `gates` explicitly separates runnable checks from unavailable ones and says unavailable checks must be reported rather than mistaken for passes. The journey describes missing-tool coverage as unverified. Those contracts require two different observations: whether a command can be launched, and what happened when that check actually ran. [Gate contract][gate-contract] [Journey gate contract][journey-gates]

Root guidance keeps models responsible for creativity and the deterministic host/kernel responsible for execution reality. Mission mode already connects a proposer and requirements interrogator to each selected work item. That connection does not substitute for the missing observation: a check that has never run contributes no finding for selection, so its underlying defect need not reach either role. Another reasoning round cannot retroactively make that command execute. A model might notice the problem independently, but that is not conservation of the declared measurement obligation. [Root][root] [Mission selection contract][mission-spec] [Mission preparation][mission-prepare]

## 2. Production trace

### 2.1 Both user entry paths share the same lifecycle

CLI and chat call `RunRecurseCycles` with the requested pass count and subsystem focus. The optional error-free gate callback is separate; ordinary workspace gates do not require it. The CLI's `--gates` option enables that additional subsystem, while chat's shown configuration does not supply `MeasureGates`. [CLI][cli] [CLI flags][cli-flags] [Chat][chat]

The shared driver creates an initially empty `state` map, acquires the workspace lock, settles recovery and prepares Git before entering passes. The witness assumes those ordinary admissions succeed and that there is no unfinished recovered pass. It is a live transition within one run. [Driver initialization][init]

### 2.2 Gate detection correctly notices newly available tools

Every pass calls `gates.Detect`. Detection reloads `nerd.md`, composes declared and detected checks, and calls `split`. A declared gate becomes a `Gate` with its stable ID, kind, argv, scope, language, source, environment and accepted exits. `split` calls `lookPath`, normally `exec.LookPath`, for a bare executable name. Failed lookup places the check in `Unavailable`; successful lookup places it in `Gates`. [Detection][detect] [Declaration projection][declared] [Availability partition][split]

Go documents `LookPath` as searching for an executable in the current path under the host's conventions. A binary becoming available in an already configured absolute PATH directory is therefore a valid transition without changing the running process's PATH string. The witness does not depend on workspace-relative command lookup or a project-specific PATH override. [Go executable lookup][lookpath]

`noteUnavailable` clears `RecurseCycleResult.UnavailableGates` and rebuilds it from this pass's unavailable list. A newly runnable check consequently loses its old warning at this point, before any measurement-coverage check occurs. That is correct as a report of *availability*, but incomplete if it is the only retained indication that the check has no evidence. [Availability reporting][availability]

### 2.3 Any cached result suppresses the whole initial measurement

Immediately after detection, `pass` tests `len(r.state) == 0`. When true, it runs `measureWorkspace`; otherwise it calls only `seedSeenFromState`, which copies existing finding IDs into the seen set. It does not compare the new gate set with cache keys or install an unmeasured state for absent checks. [Pass admission][pass] [Seen-state reuse][seen]

`measureWorkspace` selects the current workspace-scoped checks, subject to the explicit focused-audit exclusion, runs them with bounded concurrency, and stores each result by `gateKey(g.ID, "")`. Even a successful result with no findings creates an entry. Thus one unrelated passing workspace check is sufficient to suppress first measurement of a different gate on every later pass. The optimization asks whether *anything* has been measured, not whether *this gate* has. [Workspace measurement][measure-workspace]

### 2.4 Neither visits nor empty candidate lists repair the omission

`visit` freshly runs only node-scoped gates. For workspace gates, `open` iterates the newly detected set and reads `r.state[gateKey(g.ID, "")].findings`. Go's map-index semantics return the value type's zero value when the key is absent. Here that means an empty finding slice; no explicit missing-observation branch is taken. [Visit entry][visit] [Candidate lookup][open] [Go map semantics][map-index]

The policy adapter emits `recurse_finding` facts only for supplied findings. `recurse_candidate` requires such a fact. The scheduler is therefore not ignoring a measured defect: the host never measured the check, and supplies no candidate from it. [Policy projection][projection] [Candidate rule][candidate]

The separate `refreshGates` function does not establish otherwise. It returns immediately when `MeasureGates` is nil and, when configured, owns error-free snapshots and `efOpen`, not this missing ordinary workspace result. Its name is not evidence that it refreshes `r.state`. [Separate error-free refresh][error-free]

### 2.5 Closing measurement depends on a keep

The pass captures the kept count, then calls `measureWorkspace` at close only if that count increased. If there were no attempts or none was kept, no closing refresh occurs. Pass completion and the bounded-run stop still follow the kernel's node-visit and finished-pass counts. Under the witness below, the run can use its second pass without executing the newly runnable check. [Pass close][close] [Pass completion and stop][completion]

## 3. Conditional witness

Use a fresh whole-workspace run with two passes. No focused subsystem is selected, so the intentional exclusion of workspace audit gates from a focused sweep is not involved.

Illustrative committed workspace:

```text
nerd.md
.gitignore             # excludes .nerd/
svc/__init__.py
svc/problem.py         # contains the property audit G would report
```

The Python files are discoverable without a Python project marker. Graph acquisition adds nodes for admitted Python source directories independently of gate detection's marker rules. Deliberately omit `pyproject.toml`, `pytest.ini`, `setup.cfg`, `setup.py`, `tox.ini`, `go.mod`, `package.json` and `Cargo.toml` so no unrelated detected test gate changes the witness. This is a supported declared-check workspace, not a failed scanner. [Graph acquisition][graph] [Python graph nodes][python-nodes] [Detected gate markers][detected]

The unchanged `nerd.md` declares two workspace checks with unique IDs:

```yaml
---
schema: nerd/v1
gates:
  - id: baseline
    kind: build
    run: baseline-check
    scope: all
  - id: contract
    kind: audit
    run: contract-check
    scope: all
---
```

These are illustrative native checker names, not executables run or installed for this audit. In a future fixture, the first checker exits zero without changing the workspace. The second, when available and invoked, exits nonzero and prints a small ordinary diagnostic such as `svc/problem.py:1: contract violation`. The current generic file/line parser recognizes that form; attribution maps its target to `svc`. No specialized language parser, truncated output, or location-free finding is needed. [Gate schema][gate-schema] [Finding parser][finding-parser] [Attribution][attribution]

Explicit preconditions:

1. The Git checkout is clean, source membership and graph discovery succeed, and the ordinary kernel, journal and locking operations succeed.
2. `baseline-check` is available throughout. `contract-check` is not resolvable during the first `Detect`, but becomes executable in an already configured absolute PATH directory before the second `Detect`. The process environment, gate declarations and workspace source remain unchanged. There is no competing executable with that name.
3. No test gate is declared or detected. No optional error-free measurement callback is configured. The baseline check produces no finding. The unavailable audit cannot yet produce one.
4. No external source writer, hidden successful measurement, resumed visit, policy override, canceled context or storage failure intervenes.

The transition follows directly from the inspected control flow:

| Boundary | Runnable set | Cached result for G | Consequence |
|---|---|---|---|
| First detection | B only; G unavailable | Absent | G has an unavailable warning |
| First workspace measurement | B runs and passes | Absent | `r.state` becomes nonempty from B |
| First visits and close | No findings and no test gate | Absent | No fix; improvement skips; no keep, so no closing measurement |
| Availability changes outside the workspace | G can now launch | Absent | Source, document and graph remain unchanged |
| Second detection | B and G | Absent | G's unavailable warning is removed |
| Second admission | B's cached entry exists | Absent | Nonempty-map branch skips workspace measurement |
| Second visits | No node check supplies G | Absent | Missing map key yields no findings and hence no repair candidate |
| Second close and stop | No attempts were kept | Absent | No closing measurement; requested pass count can complete |

The improvement guard is important rather than incidental. For each physical node, `improve` returns without executing when no runnable node test or applicable workspace test can certify the improvement. Cross-cutting nodes return immediately. Therefore the main witness does not need a scripted model to choose no change: it can reach zero executor calls through normal host guards. [Improvement guard][improve]

**Established result:** after the availability transition, G can be present in the current runnable set, absent from unavailable warnings, absent from measured state, and absent from candidate facts for the entire requested pass. Its genuine failure remains unobserved. `Summary` has no separate unmeasured-gate field; its `Unverified` count is an attempt outcome count, not a coverage census. Do not reinterpret a zero count as an explicit assertion that G passed. [Result and summary][summary]

### Bounds and counterevidence

- If G is already runnable at startup, the empty initial map causes a proper baseline run. This is the essential positive control.
- If no other result populated `r.state`, a later pass can take the empty-map branch and discover G. The unrelated B entry is a necessary precondition here.
- A node-scoped G is measured at its visit and is not the primary affected route.
- A later kept attempt causes a closing refresh with that pass's detected set. That can restore coverage, but it occurs after the visits and is absent in this witness. A new invocation also starts with an empty map.
- Per-attempt build/lint or selected-target checks can measure some missing workspace checks in other runs. An audit with no prior result is not a selected target, and the stock ratchet kinds are build and lint. In any event, this witness reaches no attempt. [After-check selection][after] [Ratchet kinds][ratchet-kinds]
- A passing G still demonstrates missing coverage, but the failing-G case shows the repair consequence. Nothing here proves false adoption, escaped permissions, data loss, a live-model failure rate, or universal failure after every tool installation.

## 4. Distinctness from existing work

The novelty claim is bounded to the inspected public tree, audit inventory, queue and relevant bodies, not every historical branch or private discussion. [Queue][queue]

- **Finding-owner graph transition** identifies cached diagnostics whose old component owner no longer exists. This witness keeps the graph identical and has no G diagnostic to reproject. Even perfect reattribution of every retained finding leaves the missing observation absent. That report's reuse boundary is relevant prior work; the distinct contribution here is admission of a newly runnable producer. [Graph-owner audit][prior-owner]
- **Resumed-visit applicability** concerns reuse of completed visits after restart. This is a fresh invocation's live transition, with ordinary visits in both passes and no inherited completion. [Resume audit][prior-resume]
- **Gate contract continuity** preserves environment and passing-exit semantics through a measured attempt's campaign. Here no G attempt exists. The declared environment and exit set are unchanged; repairing their downstream handoff cannot create an initial observation. [Contract audit][prior-contract]
- **Gate subsumption** ensures independent *measured* workspace findings reach policy. The current `open` includes such findings correctly. It does not synthesize a measurement for an absent key. [Subsumption audit][prior-subsumption]
- **Composed gate identity collision** diagnoses distinct declarations sharing a key. B and G have distinct IDs and therefore distinct workspace-result keys here. Both have workspace scope and the same declaration-source label; no key collision or overwrite is needed. [Identity audit][prior-identity]
- **Gate comparison domain admission** and **scoped overlay evidence conservation** govern error-free before/after judgments. The primary witness disables that optional path and never reaches a ratchet pair or overlay. Their principle that unknown is not a pass remains applicable, but the faulty consumer is different. [Comparison audit][prior-comparison] [Overlay audit][prior-overlay]
- **Executable selection under declared PATH** concerns using a different lookup environment from execution. Here ambient PATH is unchanged and correct; the detector successfully resolves the intended tool after it appears. The missing step is running that already admitted check. [Executable audit][prior-path]

The broad portable recurse/deliberation proposal already calls for capability-aware acquisition and honest evidence. This report supplies a concrete lifecycle requirement to that design, not a new agent framework or a claim to have invented evidence manifests. [Portable proposal][portable]

## 5. Required contract and bounded design

### Per-check coverage before consumption

Before a pass treats its workspace measurement domain as ready for candidate selection, each applicable workspace check in the admitted set must have one explicit disposition:

1. An observation demonstrably reusable for the current declared check and source applicability domain.
2. A fresh completed observation from the existing runner, including failure, no-tests, truncation and execution-error distinctions.
3. An explicit unmeasured, unavailable, interrupted or incompatible state with the required policy/reporting consequence.
4. An intentional out-of-scope disposition under the existing focused-sweep contract.

An entry for B cannot satisfy G. Successful executable lookup must change G from unavailable to runnable-but-unmeasured until there is evidence of execution; it must not erase the uncertainty represented by the earlier missing-tool warning. A permitted missing-tool state can remain visible without forcing automatic installation or broadening any command permission.

### Smallest useful implementation direction

Update the campaign/gate specifications first. At the existing pass boundary, reconcile the freshly detected set with retained observation keys. Measure newly runnable required workspace checks with no compatible observation before visits consume them, or preserve an explicit deferred/blocked coverage state. This can reuse `measure`, `measureWorkspace`'s execution discipline, existing parsers and policy projection rather than creating another runner.

For the narrow witness, an explicit per-key presence check is enough to expose the hole. Do not call that complete freshness handling: a same-ID check may later change command, scope, environment, accepted exits, parser interpretation or toolchain. Define a descriptor identity and compatibility policy if the repair retains results across those changes. Preserve historical observations rather than relabeling them with today's contract. Secret environment values must not be copied into public logs, model prose or unrestricted facts; an ordinary public digest of low-entropy secrets is not a safe substitute.

Separate discovery, availability, required coverage and observed outcome in the state supplied to Mangle. Go performs enumeration and execution and supplies observations; Mangle continues to decide candidate eligibility, admission, completion and stopping. If acquiring required evidence exceeds the owner's remaining duration or is interrupted, expose the incomplete coverage and let the existing authority decide the disposition. Do not silently spend an extra pass or extend a mission budget.

A coherent publication boundary matters. Do not publish a partially reconciled set as complete if execution or fact publication fails. Preserve successful independent observations, identify outstanding checks, and ensure a later pass can retry missing work without replaying uncertain source effects. Gate execution can itself have effects; the existing post-verification-effects audit remains an independent prerequisite for treating arbitrary checks as harmless. This correction does not grant gates new write authority. [Post-verification effects][prior-effects]

Do not fix this by clearing the entire map blindly, treating runnable checks as passing, keeping a stale “not on PATH” reason after resolution, inventing findings from unavailable tools, running every audit inside an intentionally focused sweep, installing tools automatically, or prompting the model to remember which checks were skipped. Those responses either discard valid reuse, misstate evidence or change scope.

## 6. Costs and failure modes

Let G be the admitted gate count, D the total bytes in the descriptors being compared, U the required checks without compatible observations, and E the retained result/output bytes.

- **Host bookkeeping:** checking membership is expected O(G); canonical descriptor comparison/fingerprinting is O(D), including deterministic ordering of environment and exit-set members where appropriate. A replacement manifest needs O(G + D) metadata. This does not require a source scan merely to identify a never-measured key.
- **Execution cost:** the primary correction runs the U checks the current code omits. Added CPU, I/O, tool startup and wall time are their real costs, not a regression to hide. With the current four-worker baseline limit, serial total duration and bounded-parallel critical-path duration should both be recorded. Heavy audits can dominate a pass.
- **Retention:** new observations add their bounded output and findings to E. `gates.Run` currently caps retained output at 1 MiB per result and marks dropped bytes. Extra history or descriptor generations need a separate retention policy; retaining every obsolete gate result indefinitely would defeat the long-horizon memory goal. [Runner output bound][runner]
- **Freshness and concurrency:** resolution can succeed and execution still fail if the executable disappears or becomes unusable. Keep the runner's actual error, not the successful lookup, as outcome evidence. Concurrent source/declaration changes require incompatibility handling; the primary witness does not exercise those races.
- **Model cost:** no extra model call, embedding request or deliberation round is needed to establish or repair the missing-observation boundary itself. Discovering a real finding can legitimately add proposer/interrogator work, execution attempts, verification and tokens. Compare complete requested coverage, not only lower model-call counts obtained by omitting work.
- **Adoption and recovery:** the no-attempt witness has no commit or rollback cost. A general repair can reach attempts earlier, so ordinary write guards, compensation, exact check contracts and durable receipts must remain intact. New state must not grant completion to a historical visit or import a foreign receipt.
- **Operational friction:** a previously quiet run may now visibly block on a slow or failing newly available audit. That is useful evidence, but it needs clear reporting and cancellation. Repeatedly retrying a persistently failing launch without a state change can waste work; a measured failed launch and a missing observation need distinct retry semantics.

A conservative alternative reruns all applicable workspace checks every pass. It is simpler to reason about but adds work for G rather than U checks and can repeatedly trigger expensive or stateful commands. Compare it fairly with selective reconciliation. A more elaborate descriptor/source cache may save executions but has implementation, invalidation and storage costs. No latency, token, quality or savings number has been measured for any option.

## 7. Existing tests and proposed evaluation

Read, not run:

- `TestDetect_MissingToolchainIsUnavailableNotDropped` verifies missing-tool partition and reasons at one detection point. It does not compose later availability with a nonempty recurse cache. [Detection test][detect-test]
- `TestRecurseSelection_IndependentWorkspaceCheckReachesPolicy` explicitly calls `measureWorkspace` before checking candidate projection. It proves useful behavior after measurement, not admission of a missing result on a later pass. [Selection test][selection-test]
- `TestRecurseCycles_ANonGoWorkspaceUsesItsDeclaredGates` exercises a declared check in a Python-source fixture. It supports the normal non-Go route but uses a stable available gate. [Non-Go test][non-go-test]
- The contract-continuity tests cover check configuration handoffs and reject changed restored declarations. They should remain controls; they do not execute the missing first observation described here. [Contract tests][contract-tests]

These are scoped observations about inspected test bodies, not proof that no other test exists.

### Deterministic baseline and repair comparison

Use disposable repositories, native fixture checkers and the shipped kernel through `RunRecurseCycles`. Keep model behavior disabled or replace only the executor with a recorder that stops at the first offered work item. For the primary current-code schedule, assert that it is never called. Place invocation logs outside the source tree so logging cannot create changed files or trigger the closing keep condition.

Use an explicit barrier between pass boundaries to make the already-declared checker available. Do not use sleep-based timing, mutate the process-global PATH concurrently, or conflate this fixture with tool installation permissions. A later validating session can prepare the private executable fixture before the run and expose it at the barrier. Keep the same fixture and transition schedule for the pinned baseline and candidate repair.

Record: each detected set and unavailable list, each effective gate descriptor, actual invocation starts and finishes, result keys, source and graph identity, candidate facts, executor calls, visit completion, pass counts, final summary and total wall/CPU time. The primary success criterion is not a fixed gate-call count: **G must acquire an observation or an explicit incomplete disposition before the second pass consumes coverage; a genuine attributed G failure must reach normal selection when otherwise eligible.** A repaired run may correctly stop with G still failing.

| Control | Required interpretation |
|---|---|
| G missing first, runnable second; B already cached | Primary coverage hole is repaired without relabeling lookup as execution |
| G present from startup | Existing baseline and candidate behavior remains correct |
| G always unavailable | Warning/unknown coverage remains explicit; no fabricated result or automatic installation |
| No unrelated cached result | Existing empty-state baseline path remains valid |
| G node-scoped | Ordinary visit measurement continues to work |
| G runnable but launch fails | Actual runner error remains unverified; availability is not success |
| G completes successfully | Observed pass replaces unmeasured state, with no synthetic finding |
| G fails with a small attributed diagnostic | Finding enters existing policy, subject to unchanged refusal/stall/permission rules |
| Whole-workspace versus focused sweep | Existing focus exclusion is preserved and labeled rather than accidentally broadened |
| Cancellation or budget expires during reconciliation | No false complete coverage and no unrequested budget extension |
| Later keep or fresh invocation | Existing recovery routes remain correct without being required for the primary repair |
| Same ID with changed argv/environment/scope/exit rule | Declared compatibility policy is enforced; a missing-key fix is not advertised as complete freshness |
| Optional error-free gates enabled | Independent result stores and verdicts remain separate and consistent |

Then exercise CLI and chat adapters with the same source and transition, confirming the user-visible result exposes any remaining measurement gap. No provider call is required to prove this deterministic boundary. Any later live-model evaluation requires separate authorization and should compare matched tasks, budgets, full coverage, real findings, retained changes, all gate costs and all model usage. A lower attempt count caused by missing G is not an efficiency win.

## 8. Exit criterion

The bounded change is complete when a newly runnable declared workspace gate cannot become neither unavailable nor measured simply because another gate has a cached result. Its actual observation or explicit incomplete state must reach the current pass before absence of findings is consumed as absence of work, without weakening focus, permissions, budgets, evidence semantics or Mangle ownership.

The demonstrated issue is small enough for a host-adapter correction and broad enough to matter to any long-running workspace whose tools or declared checks become available over time. Better Socratic design depends on receiving the real outstanding obligations first.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/f52db4a9224cd9cc8b97619eefb1c5f3169fb329
[root]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/agents.md#L3-L36
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/agents.md
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/architecture/AGENTS.md
[internals]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/architecture/campaign/INTERNALS.md#L152-L178
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L128
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/journeys/10-forever-loop.md
[journey-gates]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/journeys/10-forever-loop.md#L109-L130
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/.agents/skills/requirements-interrogator/SKILL.md
[gate-contract]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/gates/gates.go#L1-L20
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_mission.go#L173-L256
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/cmd/nerd/cmd_campaign_recurse.go#L248-L272
[cli-flags]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/cmd/nerd/cmd_campaign_recurse.go#L81-L92
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/cmd/nerd/chat/campaign_recurse.go#L218-L228
[init]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L356-L453
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L515-L571
[close]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L601-L612
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/gates/detect.go#L15-L45
[declared]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/gates/detect.go#L52-L92
[split]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/gates/gates.go#L155-L196
[availability]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L692-L697
[seen]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L615-L626
[measure-workspace]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L699-L751
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L1052-L1065
[open]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L1007-L1032
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_policy.go#L64-L100
[candidate]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/core/defaults/policy/recurse.mg#L236-L255
[error-free]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_error_free.go#L29-L72
[completion]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/core/defaults/policy/recurse.mg#L472-L485
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_workspace.go#L62-L103
[python-nodes]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_workspace.go#L461-L483
[detected]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/gates/detect.go#L129-L182
[gate-schema]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/projectdoc/nerdmd.go#L167-L179
[finding-parser]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/gates/findings.go#L116-L186
[attribution]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L788-L799
[improve]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L1181-L1224
[summary]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L144-L205
[after]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle.go#L1324-L1350
[ratchet-kinds]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/core/defaults/policy/recurse.mg#L261-L264
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/gates/run.go#L15-L81
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/ASTRA_QUEUE.md
[prior-owner]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/recurse-finding-owner-graph-transition.md
[prior-resume]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/recurse-resumed-visit-applicability.md
[prior-contract]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/recurse-gate-contract-continuity.md
[prior-subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/recurse-gate-subsumption.md
[prior-identity]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/recurse-composed-gate-identity-collision.md
[prior-comparison]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/gate-comparison-domain-admission.md
[prior-overlay]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/scoped-gate-overlay-evidence-conservation.md
[prior-path]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/gate-declared-path-executable-selection.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/portable-recurse-deliberation-crucible.md
[prior-effects]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/Docs/audits/recurse-post-verification-effects.md
[detect-test]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/gates/detect_test.go#L156-L178
[selection-test]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_gate_subsumption_test.go#L11-L86
[non-go-test]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_cycle_test.go#L354-L388
[contract-tests]: https://github.com/theRebelliousNerd/codenerd/blob/f52db4a9224cd9cc8b97619eefb1c5f3169fb329/internal/campaign/recurse_contract_continuity_test.go#L160-L227
[lookpath]: https://pkg.go.dev/os/exec#LookPath
[map-index]: https://go.dev/ref/spec#Index_expressions
