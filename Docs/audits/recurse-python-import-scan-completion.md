---
title: Recurse Python scan-completion admission
date: 2026-11-03
status: proposed
source_commit: 68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b
verification: static-source-inspection-only
supersedes: []
---

# Do not admit a partial Python import scan as a complete recurse graph

## Finding and decision

**A readable, admitted Python file can lose every import after a long physical line without making recurse graph derivation fail.** `pythonImportTargets` uses a line scanner with an explicit 1 MiB token limit. When scanning stops, it returns the imports accumulated so far without checking `Scanner.Err()`. The caller has no error or coverage result to propagate. Both the importer and its prerequisite can remain valid graph nodes while their relationship disappears. A focused sweep consequently omits the prerequisite, and the same incomplete graph passes ordinary structural soundness checks. [Parser][parser] [Producer][producer] [Derivation][derive] [Focus][focus]

The narrow finding is **scan-completion admission**, not unsupported Python syntax or a missing module-resolution rule. The witness below uses plain absolute imports of regular packages that the current resolver can resolve. It needs no filesystem error, race, malformed Python, paid inference, failed gate, or adversarial model response. One ordinary comment line larger than the scanner's stated bound is sufficient. The fixture and all proposed evaluations are unrun.

Preserve resource limits, but do not turn hitting one into evidence of no further dependencies. Carry a completed/incomplete scan result through the existing graph-acquisition path. Admit dependency-complete planning only after every required source scan completes within its declared capability domain. A bounded, visible refusal is preferable to quietly certifying the partial graph. Supporting larger inputs can be a later adapter improvement; simply increasing the constant does not repair the missing completion contract.

This contributes one falsifiable acquisition boundary to the existing portable recurse/deliberation proposal. It does not propose another agent framework. The proposer and requirements interrogator receive an already selected recurse attempt; another reasoning round cannot retroactively restore a missing host-admitted prerequisite. Independent role reads or broad gates can still discover the file, but those are different observations from the scheduler's dependency closure. [Portable proposal][portable] [Mission preparation][mission] [Mission contract][mission-spec]

## Evidence boundary

Public main resolved to **68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b**, committed November 3, 2026 at 19:47:21 UTC, and remained unchanged when rechecked at approximately 20:16 UTC. This review inspected public guidance, the requirements-interrogator skill, campaign/session architecture, graph acquisition and consumers, mission handoff, current audit inventory and neighboring reports, and authored tests. Repository links below are immutable. Relevant cached source copies were verified against the pinned Git tree's blob identities; the newest module-file report and other changed documents were fetched at the pin. [Pin][pin] [Guidance][guidance] [Interrogator][interrogator]

No repository program, build, test, interpreter, model inference, gate, benchmark, fault injection, or fixture execution ran. No repository source, credential, configuration, permission or setting was changed. Only scratch research sources and this report were written.

The review establishes a conditional source trace, not the prevalence of long source lines, an observed production failure, an unsafe write, an accepted defective change, or a measured quality/cost benefit. It also does not establish that all alternate inspection paths miss the prerequisite.

## 1. The intended contract and present wiring

Root guidance assigns creativity to the model and executive decisions to Mangle, and requires bounded, recoverable context rather than silent loss. The campaign specification keeps dependency order in recurse policy and limits deliberation to design within an admitted work item. The portability proposal already calls for explicit adapter capabilities and unknown states. Those are compatible with refusing an incomplete import observation. [Guidance][guidance] [Mission contract][mission-spec] [Portable proposal][portable]

The operative chain is:

1. `DeriveWorkspaceDAG` canonicalizes the root, refreshes shared membership and collects admitted sources. It invokes the Python producer and eventually returns collapsed nodes plus the cross-cutting close with a nil error. The Python producer currently cannot return an error. [Derivation][derive] [Collection][collection]
2. `scanPythonImports` first creates a node for every directory containing an admitted Python file. It then calls `pythonImportTargets` for each source and resolves the returned names. Discovery of a file and completion of its import scan are not separate outputs. [Producer][producer]
3. `pythonImportTargets` reads the whole file, constructs `bufio.Scanner`, and sets `Buffer(make([]byte, 0, 64*1024), 1024*1024)`. The initial buffer is 64 KiB; the configured maximum is **1,048,576 bytes**. The scan loop's exit is followed immediately by `return out`; there is no `sc.Err()` check. [Parser][parser]
4. The source resolver handles an ordinary package name by checking the root-relative directory and then the `src` domain. For the witness's two package names, successful complete scanning would create both expected edges. No Python file-owner or package-ancestor extension is needed. [Resolver][resolver]
5. Graph edge admission, SCC collapse and structural validation operate on the supplied nodes and edges. They do not carry a file-scan completion fact, so a partial but well-formed acyclic graph is not rejected for that reason. [Graph][graph] [Validation][validation]
6. Both `RecurseSweepOrder` and the live pass driver call the shared derivation and dependency closure. The pass driver correctly propagates an error from derivation if one exists; this source condition supplies none. [Plan][plan] [Pass admission][pass]
7. The mission's design thread receives the selected attempt; its receipt copies the node's allowed paths. Scope, source freshness, ordinary permissions, gates and the ratchet remain independent controls. The missing dependency is upstream of that design input, not authority a role should invent. [Mission input][mission] [Scope guard][scope]

Go's documented scanner contract distinguishes a normal end from an error: `Scan` returns false in both cases, and `Err` supplies the non-EOF error. Its default splitter is line-oriented; a token too large for the configured buffer stops the scan. The official implementation assigns `ErrTooLong` when it cannot grow within the bound. These references support the control-flow premise, not an executed reproduction. [Scanner API][scanner-api] [Scanner implementation][scanner-source]

## 2. Minimal conditional static witness

Consider these four admitted, readable, immutable files under a canonical workspace root:

| Path | Relevant contents |
|---|---|
| `aa_app/__init__.py` | Empty |
| `aa_app/main.py` | The exact three-line construction below |
| `mm_seen/__init__.py` | Empty |
| `zz_support/__init__.py` | Empty |

The initializers already create their directory nodes; no separate implementation file is necessary for the witness.

The three physical lines of `aa_app/main.py` are defined precisely, without expanding a megabyte into this report:

1. The ASCII bytes `import mm_seen\n`.
2. ASCII `# `, followed by **1,048,576 ASCII `x` bytes**, followed by LF.
3. The ASCII bytes `import zz_support\n`.

This is a fixture definition, not an executed generator or a literal multiplication expression in the source. Line 2 is a comment whose content is 1,048,578 bytes before LF, unambiguously larger than the configured scanner bound. Python's lexical reference specifies that a comment ends at its physical line and is ignored by syntax. The witness uses LF, ASCII and ordinary package imports to exclude encoding, continuation and import-grammar disputes. It does not claim that arbitrary host resource limits cannot reject a large file. [Python lines and comments][python-lexical]

Assume:

- Membership refresh, walking, project restrictions, canonicalization and all file reads succeed.
- No `go.mod`, other language graph, symlink alias, hidden directory or excluded/generated treatment affects this fixture.
- These names are unambiguous local regular packages in the intended root import domain, with no custom import hook or preloaded shadow.
- There are no other imports or paths supplying `aa_app -> zz_support`.
- Focus is exactly the existing node `aa_app`.
- The ordinary kernel and graph planning infrastructure are available.

### Exact source trace

| Boundary | Source-derived result |
|---|---|
| Collection | All four paths enter the admitted Python inventory. |
| Node acquisition | `aa_app`, `mm_seen` and `zz_support` all exist. |
| Line 1 | The regex recognizes the complete name `mm_seen`; it enters the output list. |
| Line 2 | The line token exceeds 1 MiB; the scanner stops before returning that token. |
| Line 3 | Never reaches either import regex. |
| Parser return | Returns the prefix containing only `mm_seen`, without reporting incomplete scanning. |
| Resolution | Adds `aa_app -> mm_seen`; receives no `zz_support` target to resolve. |
| Derivation | Returns the ordinary graph and nil error. |
| Focus closure | Retains `aa_app` and `mm_seen`, but not `zz_support`. |

The returned prefix is credible: one real import was acquired and its edge is correct. The defect is not detectable by checking that the import list is nonempty or that every retained edge has a valid endpoint. [Parser][parser] [Producer][producer] [Graph][graph] [Focus][focus]

A semantically irrelevant change to comment layout distinguishes the outcomes. Replace line 2 with enough short comment lines to carry the same repeated `x` content, keeping each line comfortably under the bound. The current scanner can then reach line 3 and supply the second edge. Alternatively move `import zz_support` before the large comment: its edge survives, although the scan is still incomplete. These are proposed controls, not observations from a run.

### Full-sweep consequence, bounded separately

With only the shown edges, the full recorded graph initially permits `mm_seen` and `zz_support`. The lexical tie-break chooses `mm_seen`; `aa_app` then becomes ready and sorts before `zz_support`. Consequently the policy's graph-derived order can place the importer before its missing prerequisite. This is not Mangle violating an asserted edge. It correctly orders the incomplete observations it receives. [Ordinal projection][ordinals] [Ordering rules][ordering]

The focused omission is the primary witness. A full sweep still contains and can later visit `zz_support`; the report does not claim the node vanishes from every run.

## 3. What the witness excludes and what still works

- **Recognized syntax:** both imports are complete, single-line absolute imports. From-import member parsing is not involved.
- **Ordinary package identity:** `zz_support/__init__.py` supplies the directory the current resolver expects. There is no top-level module-file ownership gap.
- **No implicit ancestor omission:** the imported package is top-level, and its one initializer shares its node. Missing parent-package initializers are not the cause.
- **Healthy acquisition permissions:** the file is already admitted and its entire contents were successfully read. The scan boundary is not a denial that should be bypassed.
- **Finite scanner memory remains useful:** the issue is the absent completion result, not the existence of a resource limit.
- **Known focus is validated:** the focus node exists; unknown-node rejection remains effective.
- **Structural graph safety remains useful:** SCC handling, endpoint checks and policy ordering do not need to become permissive.
- **Independent verification can still help:** a workspace gate, source reader or role may inspect the dependency; this does not repair the separate focused-scheduling observation.
- **Mutation remains governed:** a missed prerequisite does not establish a permission escape, wrong retained improvement, or failed test accepted as passing. Keep all mission write and ratchet boundaries.
- **Rederivation is not recovery by itself:** a subsequent pass on identical bytes encounters the same source scan limit. Recovery requires a changed acquisition method, an explicit incomplete disposition, or changed source, not merely more debate.

The same function also turns a failed `os.ReadFile` into an empty list, and other language adapters have their own I/O behavior. That is visible adjacent evidence for reviewing a common completion/error result. It is **not needed for this witness**, and this report does not claim to have established every language's operational-failure semantics. [Parser][parser]

## 4. Nonduplication

The current public audit inventory and queue were screened, together with the portable proposal, the three Python prerequisite reports, JS workspace acquisition, finding-census completeness, membership failure and nearby deliberation reports. Public code searches for `pythonImportTargets`, `Scanner.Err` in audits and `token too long` in documentation were also inspected. The Python-symbol search returned the three existing prerequisite reports; no dedicated scan-completion predecessor was identified in that bounded screen. Search indexing and this review are not proof of novelty across private work, all branches or all history. [Inventory][inventory] [Queue][queue]

| Existing work | Boundary owned there | Distinction here |
|---|---|---|
| Python from-import prerequisites | Names after `from ... import` are dropped by parsing. | The missed statement is a plain `import zz_support` that is never reached. |
| Python package-initializer prerequisites | A resolved leaf does not retain implicit regular-package ancestors. | This witness is a top-level package and has no separate ancestor node. |
| Python module-file prerequisites, added at 68e9fbbe | A flat source module does not map to its directory owner. | The target is already a package directory that current resolution handles. |
| Portable recurse deliberation crucible | Honest capability limits and a common design/evidence protocol. | This is a concrete missing completion signal below that proposal. |
| Recurse finding-census completeness | The original bounded gate-diagnostic finding concerned records needed by the ratchet; its current-main addendum credits the capped-list repair. | No gate output or ratchet decision is involved; source import scanning loses an edge before planning. |
| Membership failure and incremental census | A dynamic Git-membership failure becomes normal exclusion before the world scanner observes it. | Membership succeeds here; a local line scanner stops on an already read source file. |

[From-import][prior-from] [Initializer][prior-init] [Module file][prior-file] [Portable proposal][portable] [Finding census][prior-census] [Membership failure][prior-membership]

The existing authored Python graph test checks short absolute and relative import examples and package order. It is a valuable regression control. The inspected case does not place a supported import beyond the scanner's token bound or assert a scan-completion error. Tests were read, not run; no claim is made that all repository tests were exhaustively inspected. [Existing test][test]

## 5. Proposed contract, not implemented

### 5.1 Separate syntax support, acquisition completion and graph soundness

These are three different questions:

1. Does this adapter claim to interpret this source form and import domain?
2. Did it finish observing all required admitted source input under its declared limits?
3. Is the resulting supplied graph structurally well formed and policy-orderable?

A yes to the third cannot replace a yes to the second. A capability manifest saying “Python” is not an acquisition receipt. Preserve the existing narrow grammar honestly without granting completeness to an aborted scan.

The smallest initial correction can use ordinary Go error returns: `pythonImportTargets` returns targets plus a completion error; its producer propagates that error; derivation returns it through the already error-aware plan/pass callers. This is a proposed shape, not a committed API. If a richer partial-result interface is desired, require explicit coverage and do not let partial edges silently enter the current complete-planning contract.

### 5.2 Name the supported outcome

For each required file, record enough bounded host evidence to distinguish:

- Completed scan, including a legitimately empty import set.
- Input/resource limit reached.
- Read failure or cancellation.
- Unsupported source interpretation, if that is actually detected.

Bind the result to path, source identity, adapter/version and relevant limits. A byte count alone does not show that the last physical line was consumed. Avoid persisting source payloads or a megabyte-long diagnostic merely to report the problem.

Aggregate completeness before publishing a current graph. The safe initial behavior is a clear derivation error and no new attempt from that failed acquisition. Partial design-only analysis could be offered separately with its limitations, but it must not call a focused closure dependency-complete or broaden writes to compensate.

### 5.3 Keep limits and failure honest

Raising the maximum may improve coverage for some files, but must not be the only repair. Retaining a limit and returning “scan incomplete at this file” meets the immediate truthfulness obligation. Supporting arbitrary long lines requires an explicit processing and memory contract.

Do not blindly skip an oversized physical line and resume regex matching. A general Python source can carry lexical state across physical lines; treating later text as independent imports without preserving that state can create false edges. A chunked line reader, tokenizer or existing semantic index must establish its own supported-domain and completeness contract. It must not execute repository code to discover imports.

A whole-file read already allocates the input before the scanner's 1 MiB bound applies. Therefore that bound does not bound this function's total input allocation. A streaming redesign may reduce peak memory, but that is a separately measurable benefit, not a result demonstrated here. [Parser][parser]

### 5.4 Preserve policy and ownership

Go measures acquisition success and supplies typed observations; Mangle continues to own ordering, stop conditions and scope admission. A role can propose a narrower read or a better adapter, but cannot declare that an incomplete source scan was complete.

Do not substitute a broad root node or add every directory as a dependency after a scanner error without a separately specified conservative-planning mode. Node paths are mutation scopes as well as visit identities. A fallback that increases the permitted scope can change the task rather than repair the evidence.

If an earlier successful graph is retained for diagnosis, mark it historical unless current source and acquisition identities justify reuse. Do not use a previous graph as current simply because a fresh acquisition failed. Avoid automatic repeated retries of an unchanged deterministic limit failure.

### 5.5 Integration and acceptance

Update the campaign/portability specification to identify the completion owner and caller behavior before implementation. The specification should state whether resource-limited graph acquisition blocks execution or permits a clearly limited alternative mode, and how that mode preserves the user's scope.

Immediate acceptance can be conservative: every required scan either completes and contributes its supported observations, or returns a visible incomplete result that prevents dependency-complete planning from being claimed. Improving Python semantic coverage is related work, not a prerequisite for making this failure honest.

## 6. Costs and failure modes

Let B be the bytes read from admitted Python sources, F the file count, V the directory-node count and E the acquired edge count.

- **Current acquisition cost:** whole-file materialization reads B bytes across files; peak input memory depends on the largest simultaneously retained file, not only the scanner limit. Scanner buffering adds up to its configured ceiling. Token extraction and regex matching process the prefix reached before the failure. These are source-level cost components, not a benchmark.
- **Minimum completion check:** checking and transporting an error adds little work compared with the existing read/scan, but changing signatures and keeping callers honest has engineering and compatibility cost.
- **Richer receipts:** file identities, limits and error summaries add O(F) metadata if retained per file. Avoid retaining all raw source content in receipts or feeding it into deliberation.
- **Streaming or semantic acquisition:** implementation complexity, encoding support, parser versioning and cancellation handling can outweigh the tiny completion-check change. Compare separately.
- **Graph work:** completed acquisition can add real edges and additional focused prerequisite visits. More work can be the correct outcome. Lower tokens or fewer attempts caused by omitting a dependency are not efficiency gains.
- **Failure handling:** explicit incomplete results may increase visible refusals for repositories previously given a partial plan. Report that trade-off honestly; do not hide the increase by counting refusals as successfully completed jobs.
- **Downstream deliberation:** the admission-only repair need not add a model call. If later recovery asks for user input or alternative adapters, include those calls, waiting time and storage costs in evaluation.

Principal repair risks are reporting an error but still publishing the partial graph; clearing historical state prematurely; mislabeling a limit as source corruption; leaking source contents in diagnostics; unbounded retry; indiscriminately broadening scope; and fixing the megabyte fixture by moving the silent boundary to a larger number.

## 7. Fair evaluation plan, entirely unrun

### Stage A: deterministic production-path checks

Use scripted files and the real acquisition, graph, focus and planning boundaries. Do not test only an extracted regex or a hand-built DAG.

| Case | Required observation |
|---|---|
| The exact witness above | Current-source prediction: partial import list and omitted focused prerequisite. Proposed correction: complete correct graph or explicit incomplete admission failure, never ordinary success with the partial closure. |
| Same comments distributed over short lines | Both supported imports are acquired when scanning completes. |
| Import before versus after the large line | Distinguish prefix retention from suffix loss; completion state must not depend on whether the lost suffix happens to contain an import. |
| Long line at the beginning, middle and end | A limit is reported even if the retained prefix already contains useful edges. |
| Lines comfortably below and above the limit | Preserve ordinary inputs and report exceeded inputs. Inspect exact boundary cases separately rather than assuming the newline consumes no buffer. |
| LF, CRLF and a final line without newline | Establish the declared supported line-ending behavior; do not infer universal Python newline support from one scanner case. |
| File over 1 MiB composed of many short lines | Distinguish token length from total file size; the current constant is not a whole-file-size limit. |
| Empty file and no-import file | Healthy empty observations remain successful, distinct from failure. |
| Read error and cancellation | Their causes remain visible and cannot become empty dependencies; keep authority closed. |
| Existing short Python, JS/TS, Rust and Go controls | No regression in other admitted graph paths. |
| Other file supplies the omitted dependency | Preserve the edge while still reporting the incomplete required scan; an alternate edge is not a completion receipt. |
| Focused planning and live pass admission | Both consume the same completeness rule; no node attempt is admitted from the failed graph. |

No test fixture should require a live model, dependency installation, execution of the imported packages or a provider credential. A later live-pass fixture must use controlled executors and gates and assert that they are not entered after graph-acquisition rejection. The existing mission scope and permission checks remain mandatory controls.

### Stage B: adapter evaluation before model claims

Compare the current implementation, minimal error propagation and any separately proposed complete long-line implementation on the same source snapshots. Include ordinary repositories, repositories with generated/static data, and deliberate boundary fixtures. Stratify results instead of presenting a pathological fixture as prevalence evidence.

Measure:

- Required-file completion classification and diagnostic precision.
- Expected-edge recall and spurious edges under a predeclared supported import domain.
- Focused-closure membership and dependency order.
- Incorrect dependency-complete success rate.
- Files/bytes read, CPU, wall time, allocations and peak memory.
- Whether historical graphs are reused honestly after a failure.
- Added legitimate prerequisite work and recovery effort.

A complete reference graph must be produced independently of the implementation under test, using known fixture semantics or a declared compatible resolver. Counting agreement with the same incomplete parser is not an oracle.

### Stage C: optional end-to-end study

Only after structural acceptance, compare recurse missions with identical model/provider, prompts, limits, scopes, source snapshots and acceptance gates. Include all outcomes: retained correct changes, no change, explicit incomplete acquisition, blocked permissions and recovery.

Report total model input/output tokens, tools, wall time, user decisions, graph-acquisition cost and externally assessed task correctness. Control for the extra prerequisite visits the corrected graph legitimately adds. Do not claim a better requirements interrogator merely because the host stopped hiding an acquisition failure.

**Exit criterion:** the exact suffix-import witness and supported ordinary controls are handled truthfully through both planning and pass admission; incomplete scans are never indistinguishable from completed empty/prefix scans; existing membership, scope, permission and ratchet boundaries remain intact. Larger-input throughput and model-quality improvements require their own evidence.

## Source links

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/agents.md#L1-L79
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/.agents/skills/requirements-interrogator/SKILL.md
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_workspace.go#L59-L101
[collection]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_workspace.go#L264-L317
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_workspace.go#L455-L481
[parser]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_workspace.go#L490-L511
[resolver]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_workspace.go#L513-L544
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_workspace.go#L131-L236
[focus]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_dag.go#L15-L77
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_plan.go#L85-L113
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_plan.go#L11-L71
[ordinals]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_plan.go#L115-L139
[ordering]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/core/defaults/policy/recurse.mg#L415-L479
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_cycle.go#L515-L612
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_mission.go#L175-L317
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_mission_scope.go#L12-L83
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L110
[test]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/internal/campaign/recurse_workspace_test.go#L112-L133
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/Docs/audits/portable-recurse-deliberation-crucible.md
[prior-from]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/Docs/audits/recurse-python-from-import-prerequisites.md
[prior-init]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/Docs/audits/recurse-python-package-initializer-prerequisites.md
[prior-file]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/Docs/audits/recurse-python-module-file-prerequisites.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/Docs/audits/recurse-finding-census-completeness.md
[prior-membership]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/Docs/audits/workspace-membership-failure-and-incremental-census.md
[inventory]: https://github.com/theRebelliousNerd/codenerd/tree/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/Docs/audits
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/68e9fbbe9bcb4c093a302fa6de76f99e4d7e380b/Docs/audits/ASTRA_QUEUE.md
[scanner-api]: https://pkg.go.dev/bufio#Scanner.Scan
[scanner-source]: https://go.dev/src/bufio/scan.go
[python-lexical]: https://docs.python.org/3/reference/lexical_analysis.html#comments
