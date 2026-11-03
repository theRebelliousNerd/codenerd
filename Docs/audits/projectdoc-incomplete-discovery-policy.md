---
doc-class: deep-dive
subsystem: projectdoc-system-session-policy
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-02
verified-against: fda0eea82a8aa82c8118c09897c8e924de964e67
verification-method: static-source-inspection
supersedes: []
---

# Project document discovery must distinguish incomplete policy from absence

## Decision and evidence boundary

Make the completeness of project-document discovery part of write admission, not merely an incidental property of a returned document slice. At the inspected revision, `projectdoc.LoadAll` suppresses filesystem enumeration errors and can return a valid but incomplete collection with no error. Shared boot publishes that collection's facts as ordinary project policy. Existing fail-closed guards still reject unavailable kernels and query failures, but cannot distinguish a complete policy with no matching prohibition from a healthy query over rules that discovery never loaded. [Loader][loadall], [publication][publish], [shared query][kernel-match].

The proposed contract has two parts: preserve discovery failures through the API, and prevent unknown policy coverage from becoming permission to mutate. **Returning an error alone does not establish the second part.** The present boot caller warns on a reported loading error and continues; it does not establish a read-only degraded mode. Optional absence must remain supported, but it must mean absence established within the intended discovery scope, not an unexamined subtree.

This report inspects public repository `theRebelliousNerd/codenerd` at **fda0eea82a8aa82c8118c09897c8e924de964e67**. It describes a conditional source-derived witness, not a reproduced incident, measured failure rate, demonstrated exploit, or successful unauthorized write. No repository program, build, test, model inference, benchmark, or fault injection was run. Only this standalone scratch document was written. All repository citations below pin the inspected revision.

## Scope and existing work

Repository guidance makes deterministic executive authority distinct from model creativity and requires tracing production wiring. Session guidance specifically requires recognized writes to retain a target and live kernel authority. That existing division should be preserved: model prose must not decide whether a failed policy census is safe, and a repair must not widen capabilities or manufacture exact permission. [Repository guidance][guidance], [session guidance][session-guidance].

Two related audits bound this finding:

- **Projectdoc logical root identity** concerns a successfully loaded `.nerd/nerd.md` fallback being misclassified downstream. It explicitly preserves the facts and write protection from successfully loaded documents. The present witness uses an ordinary root document and fails earlier, during discovery of a different module document. It neither requires nor repairs fallback identity. [Prior root audit][prior-root].
- **Workspace membership failure and incremental census** concerns a dynamic Git failure becoming a cached excluded answer before an error-aware world scanner can observe it. Here `LoadAll` directly discards a filesystem walk error, even with successful membership and no Git worktree. It is a separate consumer-side completeness failure, not evidence that the world scanner's existing error handling is absent. [Prior membership audit][prior-membership].

This is a bounded review of those related reports and current implementation/test definitions, not an exhaustive claim about every historical branch. Older architecture descriptions of fail-open query gates must not override the current fail-closed code.

## Where observation loses its meaning

### Selected file errors and discovery errors take different paths

`Find` tries root `nerd.md`, then `.nerd/nerd.md`. It selects a candidate only when `os.Stat` succeeds and it is not a directory; all stat failures move to the next candidate. If neither qualifies, `Load` returns absence. Once a path has been selected, however, `Load` returns its read or parse error. Root lookup failure and selected-file read failure are therefore not equivalent. This related root-discovery boundary should be corrected deliberately, without presenting every unreadable root file as silently ignored. [Find and Load][find-load].

`LoadAll` first loads that root, then obtains workspace membership and explicitly refreshes it. Errors from those stages are returned. Its subsequent `filepath.WalkDir` callback nevertheless starts with:

`if werr != nil { return nil }`

For ordinary visits it applies `mem.Admit`, skips nonmembers/directories and duplicate/root entries, and reads module files named `nerd.md`. A discovered module's read or parse error sets `walkErr`, stops traversal, and is returned. Conversely, the outer walk error is explicitly discarded when `walkErr` is nil. The successfully collected modules are sorted and appended to the root collection. [Complete loader control flow][loadall], [strict parsing][strict-parse].

This yields distinct results:

| Observation | Current loader result |
|---|---|
| Successful census with no project documents | Empty successful result |
| Explicitly excluded directory | Normal pre-descent pruning |
| Membership construction or explicit refresh fails | Error |
| Selected root or discovered module cannot be read or parsed | Error |
| Member directory cannot be enumerated | Callback failure suppressed; discovered subset can succeed |
| Root candidate stat fails | Candidate skipped; fallback or absence may result |

Go gives the callback the directory-read error precisely so the caller can decide whether to propagate it. `SkipDir` is normal traversal control; a different returned error stops the walk. `WalkDir` also does not follow directory symlinks. Thus neither intended exclusion nor unsupported symlink traversal should be relabeled as the witness's lost error. [Go callback contract](https://pkg.go.dev/io/fs#WalkDirFunc), [Go filesystem walk](https://pkg.go.dev/path/filepath#WalkDir).

Membership retains useful boundaries. `Admit` passes directory `SkipDir` through, while an excluded file becomes an ordinary nonprocessing result. Reserved `.git` and `.nerd` segments stay excluded from module discovery; the designated fallback is handled by the separate root load. Propagating failures must not become permission to read excluded content. [Admission][member-admit], [reserved directories][member-exclude].

## Minimal cold boot witness

Use a disposable, non-Git workspace with a readable root document and an ordinary member directory such as `module-a`. That directory contains a valid `nerd.md` whose unique rule forbids the match `protected.txt`. Place a writable `protected.txt` elsewhere in the workspace. Keep root and other loaded documents valid and ensure none independently supplies the same denial.

Require these conditions:

1. The effective process can access the workspace, read its root document, and complete membership setup. Non-Git membership admits ordinary nonexcluded paths; refresh resets its caches without itself enumerating the entire tree. [Membership creation][member-create], [refresh][member-refresh], [decisions][member-decide].
2. Enumeration of `module-a` fails. This may be an actual access or I/O failure; no concurrent attacker, Git failure, narrow timing race, or fallback layout is required.
3. The kernel starts without an independent previously asserted copy of that unique rule. This is a cold-policy witness, not a claim that a running kernel retracts an old rule when a directory becomes unreadable.
4. Other boot stages and fact assertion succeed.

The walk suppresses the directory error. Root and other discovered documents can still be returned and published. `project_doc` for the missing module and its frontmatter facts are absent from this publication, while retained root rules remain active.

The rule's reach beyond its source directory is intentional in this witness. `Facts` emits each `project_forbidden_path(Match, Reason)` unchanged: it neither prefixes the module directory nor attaches a module scope. The shared Go gate slash-normalizes and case-folds the target and match, then applies substring matching across the loaded facts. Consequently a module's `protected.txt` match can prohibit a target elsewhere. **Blocking writes only beneath the unreadable directory is not a sufficient repair for the current global rule semantics.** Changing those semantics would be a separate compatibility decision. [Fact projection][facts], [matching][kernel-match].

For a POSIX/Linux permissions fixture, establish the effective identity, ACLs and capabilities and verify that enumeration genuinely fails. Merely applying mode `000` is not portable proof: privileged identities and applicable capabilities may bypass discretionary checks. A target in a separate accessible directory also avoids confusing missing project policy with removal of OS protection. Nothing here makes an OS-denied write succeed. [Linux directory-open failures](https://man7.org/linux/man-pages/man3/opendir.3.html), [Linux permission capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html).

A second, simpler consequence is lost module purpose/requirements. It does not require a write proposal or establish that a model necessarily behaves incorrectly.

## Production publication and guard trace

### Shared boot publishes the returned collection

The TUI calls `BootCortexWithConfig` and uses its kernel, VirtualStore, session executor and Spawner. Default boot runs kernel initialization before execution-layer and final-executor construction. `initKernel` loads available world facts, calls `loadProjectDoc`, and returns nil. [TUI entry][chat-boot], [boot ordering][boot-steps], [kernel initialization][kernel-load].

`loadProjectDoc` reports returned errors to stderr and the boot logger. On nil error it collects `Facts()` from every returned document and calls `kernel.LoadFacts`. A fact-loading failure is also reported. On success it selects the ordinary root for the advisory prompt and logs the number of loaded documents, facts, forbidden paths and commands. These counts describe the returned subset; they are not a completeness certificate, and no suppressed-directory diagnostic accompanies them. Empty results return silently. [Publication and diagnostics][publish].

The execution layer binds the same kernel to VirtualStore and hydrates modular tools. The factory separately forwards the root document to the session executor and Spawner. Losing a module's facts is not equivalent to losing the root prompt pointer, and module-only documents must not be promoted into global prompt instructions. [Execution wiring][execution-wire], [prompt wiring][prompt-wire].

### Current guards are present but do not know census health

For the session path, effective tool capability is checked first. The constitutional gate remains separate, including approval handling where applicable. `executeToolCall` then invokes the consolidated write guards before backend execution. The project guard rejects invalid/missing targets, a nil kernel, query errors, and matching loaded rules. [Capability][session-allow], [permission][session-permission], [dispatch][session-dispatch], [guard set][guard-set], [project guard][session-guard].

VirtualStore independently invokes its project guard before dispatching mutations, including listed campaign writers. Modular hydration binds a write guard to its registry and the process-global registry; registry execution applies its capability envelope and guard before calling the tool. Their project checks also preserve missing-authority and query-error refusal. These shipped repairs must remain. [VirtualStore dispatch][vs-dispatch], [VirtualStore guard][vs-guard], [binding][registry-bind], [registry dispatch][registry-dispatch], [registry guard][registry-guard].

All these project checks ultimately need the loaded forbidden-path facts. A healthy query with no matching fact returns no project prohibition. The separate Mangle edit bar also consumes `project_forbidden_path`; it does not rediscover missing files. Repeating checks over the same incomplete input cannot recover a rule that was never projected. [Shared query][kernel-match], [policy consumer][policy-bar].

The source conclusion is therefore **loss of the omitted document's restriction at these project-policy checks**, conditional on no duplicate rule or other independent authority. It is not that the tool becomes available, exact constitutional permission is manufactured, all safety gates fail, a model requests the write, or the backend necessarily mutates. Existing constitutional denials, other guards, containment, OS permissions and effect-specific checks can still stop it.

## Recovery and failure semantics

A later healthy `LoadAll` refreshes membership and walks again, so its returned set can recover. Recovery of that helper is not automatically recovery of already-published kernel policy. The inspected boot world-refresh and TUI `/scan` route call the world scanner; `/refresh-docs` refreshes orientation and extracts knowledge. None of those inspected entry bodies directly calls `loadProjectDoc` to republish its rules. Do not advise that restoring permissions followed by a successful scan necessarily repairs current project protection. A genuinely fresh healthy boot reruns the loader/publication path. [Boot world refresh][world-boot], [scan][scan], [document refresh][doc-refresh].

Recurse separately calls `LoadAll` while choosing a node's Northstar. An omitted own document can lead to root fallback text or no text; the attempt carries that selection and renders it for an extend angle. This reread is context delivery, not fact repair. Direct root-based gate discovery and critical/document path lists use `Load`, so the module-only witness does not remove a successfully readable root's declarations from those consumers. [Selector][recurse-selector], [attempt][recurse-attempt], [rendering][recurse-text], [direct gates][direct-gates], [direct lists][direct-lists].

Simply replacing swallowed errors with returned errors would enter the factory's existing warning-and-continue branch. It would improve honesty but could leave even the readable subset unpublished. An audit closure must therefore distinguish:

- **Observation correctness:** no successful-complete result after an operational discovery failure.
- **Admission correctness:** an incomplete policy census cannot silently authorize mutation.
- **Recovery correctness:** only a successfully validated publication clears degradation; retries and stale work cannot erase newer state.

Saltzer and Schroeder's fail-safe defaults and complete mediation motivate including initialization and recovery in authority checks. They do not prescribe one CodeNERD implementation or prove an observed incident. The concrete contract here follows from the repository's own machine-enforced project restrictions. [Primary protection principles](https://web.mit.edu/saltzer/www/publications/protection/Basic.html).

## Proposed implementation contract and costs

Give discovery an explicit result that distinguishes complete absence, complete documents and incomplete observation, with bounded path/operation/error diagnostics. Propagate actual walk/read failures; distinguish missing optional root candidates from lookup errors. Continue honoring normal exclusions, root precedence, strict parsing and module ordering.

Keep facts and census health under one publication owner. Stage the candidate before changing active authority, and publish the validated generation coherently with its health. For an incomplete cold load, a defensible default is booting a diagnosable read-only session, or refusing boot with a usable error. A read-only mode preserves inspection without granting the model authority to clear the fault. Any exceptional repair operation must have an explicit host-controlled scope, not a generic bypass.

If retaining last-known-good facts for availability, label them stale and define admission separately. Retention preserves known denials but cannot prove that an unreadable new document introduced no additional global restriction. Concurrent reloads need generation checks and serialized publication so an older scan cannot overwrite newer healthy policy. Ordinary directory traversal is not an atomic filesystem snapshot; failure propagation alone does not establish revision consistency or close every read-to-use race. An atomic kernel update also does not reverse already-started filesystem effects.

Costs are real:

- Traversal, document reads/parsing, membership refresh and sorting remain. Tracking errors need not add a second full walk; richer manifests or revision checks may add I/O, memory and hashing.
- Full generations can require staged and active fact sets simultaneously. Coherent replacement, duplicate/global rules and ownership of facts need explicit design rather than indiscriminate predicate deletion.
- Failing early can reduce wasted work on broken trees but reduces availability. Retrying adds latency and I/O; use bounded backoff and cancellation where supported, not a rapid retry loop or an unjoined worker.
- Publishing health adds kernel/interface, executor, registry, UI and recovery complexity. Rewalking the whole project before every write is an expensive alternative, not a demonstrated improvement.
- More accurate Northstar/instruction delivery may increase context or turns. No token, latency, memory, quality or dollar saving was measured here.

## Fair evaluations that remain unrun

Use deterministic fault seams first and real disposable filesystem integration second. Preserve production capability and permission gates; do not disable them merely to reach a later assertion.

| Case | Required observation |
|---|---|
| Healthy ordinary root and modules | Same documents, global rule semantics, root prompt and policy outcomes |
| Empty healthy workspace | Optional absence remains usable, not falsely degraded |
| Excluded subtree and fallback root | Intended pruning and explicit fallback remain; no unauthorized enumeration |
| Member subtree enumeration failure | Explicit incomplete status naming operation/path; never ordinary successful census |
| Discovered module read or parse failure | Error remains visible; proposed admission behavior matches documented contract |
| Unique module forbid targeting elsewhere | Cold production boot carries degradation to session, VirtualStore and registry admission; no backend mutation |
| Duplicate forbid in a readable document | Known denial continues; avoid mistaking this control for recovery of completeness |
| Kernel unavailable or query fails | Existing fail-closed checks still reject writes; reads remain diagnostic |
| Root lookup error versus genuine absence | Distinct results; unreadable higher-priority candidate does not silently establish fallback equivalence |
| Recovery and competing reloads | Only complete validated publication clears fault; stale completion cannot replace newer policy |
| Recurse own purpose unavailable | Incomplete source status is visible rather than silently certified as a full module view |

For the mutation case, drive normal shared construction and a deterministic typed proposal with a valid capability and exact authorization for a harmless disposable target, subject to the project restriction. Record which guard refused, facts actually published, backend invocation count and file state. A model not proposing a write is not enforcement evidence. Isolated helper tests are useful but not sufficient proof of boot-to-dispatch closure.

For POSIX permission integration, use an identity that genuinely lacks access; report skips where capabilities/ACLs defeat the setup. Separate Git and non-Git fixtures. In Git cases, distinguish explicit membership-refresh failure, intended exclusion and directory enumeration failure; do not assume Git's successful command result is an atomic filesystem census. Include symlink and concurrent deletion cases with explicitly documented scope rather than silently broadening discovery.

Current inspected tests cover root/module ordering, exclusions, malformed modules and empty workspaces; factory tests preserve module facts without module-to-root prompt promotion. Session tests distinguish capabilities/constitution from project denial, enforce facts without the prompt copy, and reject missing/query-failed kernels. Retain them. They do not by themselves establish the conditional discovery-failure-to-production-admission chain described here. This is a statement about inspected definitions, not all possible coverage. [Loader tests][loader-tests], [factory tests][factory-tests], [guard tests][guard-tests].

Compare healthy baseline and proposed implementation with identical trees, document sizes, membership settings, cache state, fault placement and credentials. Measure directory/file operations, boot and reload latency, heap allocations and peak memory separately, publication latency, refused writes and recovery time. Faulted work must not be counted as faster successful completion. No live model call is needed to establish this deterministic boundary; authorized paired model runs would be needed only for later quality or cost claims.

Close the finding only when complete absence and incomplete discovery remain distinguishable through normal boot, all affected admission surfaces and recovery. A new loader error, a warning, a green parsing test, or a retained root prompt is not alone that result.

## Pinned implementation references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/agents.md#L1-L117
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/session/agents.md#L1-L21
[prior-root]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/Docs/audits/projectdoc-logical-root-identity.md#L14-L36
[prior-membership]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/Docs/audits/workspace-membership-failure-and-incremental-census.md#L12-L29
[find-load]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/projectdoc/nerdmd.go#L253-L290
[loadall]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/projectdoc/nerdmd.go#L293-L388
[strict-parse]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/projectdoc/nerdmd.go#L391-L411
[member-create]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/workspace/membership.go#L152-L179
[member-refresh]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/workspace/membership.go#L221-L245
[member-decide]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/workspace/membership.go#L291-L367
[member-admit]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/workspace/membership.go#L454-L493
[member-exclude]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/workspace/membership.go#L606-L612
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/projectdoc/facts.go#L52-L138
[chat-boot]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/cmd/nerd/chat/session_shared_boot.go#L89-L122
[boot-steps]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/system/factory.go#L2891-L2925
[kernel-load]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/system/factory.go#L1439-L1491
[publish]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/system/factory.go#L1531-L1608
[execution-wire]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/system/factory.go#L1654-L1705
[prompt-wire]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/system/factory.go#L2660-L2699
[session-allow]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/session/executor_tools.go#L2503-L2509
[session-permission]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/session/executor_tools.go#L2572-L2594
[session-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/session/executor_tools.go#L2639-L2652
[guard-set]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/session/write_guards.go#L30-L52
[session-guard]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/session/executor_tools.go#L1648-L1699
[kernel-match]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/projectdoc/gate.go#L36-L83
[vs-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/core/virtual_store_routing.go#L392-L407
[vs-guard]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/core/virtual_store_projectdoc.go#L14-L71
[registry-bind]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/core/virtual_store_tools.go#L41-L68
[registry-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/tools/registry.go#L376-L423
[registry-guard]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/core/virtual_store_write_guard.go#L55-L98
[policy-bar]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/core/defaults/policy/coder_edit_bar.mg#L29-L45
[world-boot]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/system/factory.go#L1494-L1528
[scan]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/cmd/nerd/chat/helpers_scan.go#L157-L199
[doc-refresh]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/cmd/nerd/chat/helpers_scan.go#L273-L301
[recurse-selector]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/campaign/recurse_cycle.go#L1321-L1351
[recurse-attempt]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/campaign/recurse_cycle.go#L987-L993
[recurse-text]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/campaign/recurse_attempt.go#L188-L210
[direct-gates]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/gates/detect.go#L15-L43
[direct-lists]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/projectdoc/critical.go#L8-L29
[loader-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/projectdoc/loadall_test.go#L27-L193
[factory-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/system/factory_projectdoc_test.go#L12-L188
[guard-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fda0eea82a8aa82c8118c09897c8e924de964e67/internal/session/executor_projectdoc_test.go#L130-L224

