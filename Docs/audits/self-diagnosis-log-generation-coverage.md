---
doc-class: deep-dive
subsystem: self-diagnosis-log-evidence
implementation-status: proposed
last-verified: 2026-11-03
verified-against: df135126863e39301d42df083b5541f9a27b8493
verification-method: static-source-inspection
---

# Self diagnosis needs log generation and coverage identity

## Finding and decision

At **`df135126863e39301d42df083b5541f9a27b8493`**, the problems-log writer rotates by renaming the active file and creating a new file at the same pathname. The live self-debug watcher identifies its position by pathname and byte offset, while explicit self-diagnosis reads only the active base filename. These readers do not follow the retained segment family. A successfully retained error can consequently disappear from automatic diagnosis coverage at a rotation boundary. If the replacement has at least the old cursor's size, the live delta reader additionally skips its prefix; equal size can suppress the kernel refresh altogether. [Writer][rotation] [Watcher][collect] [Projection cache][projection] [Diagnosis selection][latest]

The required repair is a **shared generation-aware evidence-read contract**, not disabling rotation or putting complete logs into prompts. A cursor must identify the segment containing its offset, and a report must distinguish all retained evidence in its declared scope from a partial observation. This complements the repository's existing requirement that retained context remain recoverable and stale evidence not masquerade as current. [Root contract][guidance]

**Status.** Source-supported, not reproduced. This review read public source, documentation and test definitions. No application, server, model, test, build, benchmark or failure-injection program was run. Only this standalone Markdown artifact was authored. No incidence, latency, token saving or user harm was measured.

## Actual production chain

### Successful logging creates multiple physical segments

Ordinary enabled category `Warn` and `Error` calls write their category sink and call `mirrorToProblems`. The mirror lazily opens `<run>_problems.log` through `openRotatingFile`, keeps that base pathname in `problemsPath`, and writes timestamped level/category records. `ProblemsSnapshot` returns that pathname plus process counters; it does not return a segment generation or inventory. This report concerns records actually mirrored successfully, not disabled categories or other logging APIs that never enter the mirror. [Producer][producer] [Mirror and snapshot][mirror]

The rotating writer serializes its own writes. Before a nonempty segment would exceed its size threshold, or after its configured age threshold, it closes the old handle, renames the file, prunes old archives and recreates the base file. For a problems log the archive is named `<run>_problems.<UTC timestamp>.log`, not `<run>_problems.log`. The base pathname remains unchanged. Writes are not split by rotation. The witness assumes successful rename, reopen and write; it needs no writer race, crash or external log rotator. [Rotation transaction][rotation] [Archive naming and retention][names]

Default size rotation is **32 MiB**, retaining **three archives**. Zero size/retention settings select those defaults; age rotation is off unless positive. The normal application's `LoggingConfig.ToLoggingConfig` does **not** map the size, age or retained-file fields, so a tiny configured threshold or minute-based rotation must not be advertised as the ordinary chat witness. Default size rotation is sufficient. Package-level tests can use smaller seams, but that would be a test technique, not proof of production configuration. [Defaults][defaults] [Policy][limits] [Config mapping][config]

### Live self-debug reads on chat turns

The session creates a watcher. `/debug-self` can prime it at the current end, intentionally excluding pre-activation history from the new-problem digest; debug-self implies logging but does not change work mode. At turn start, `pendingSelfProblemsForTurn` collects from the snapshot path and calls `syncSelfProblemFacts`. Later articulation receives the rendered pending digest. This is a turn-driven reader, not a continuously running file tailer. [Session ownership][session] [Activation][toggle] [Prime state][watcher] [Turn entry][turn] [Collection and projection][sync] [Articulation][articulation]

The reader opens the path afresh each time, then:

- Resets to zero if the **pathname** changed.
- Resets to zero if the new file's size is **smaller** than the prior offset.
- Returns without reading if size equals offset.
- Otherwise reads starting at the old offset, up to 512 KiB, and advances through complete lines, with the existing capped-chunk exception.

No remembered file identity or generation distinguishes an append from rename-and-recreate. Its mutex protects watcher state; the writer's separate mutex does not make the two operations one generation-aware transaction. [Collection implementation][collect]

The kernel projection is separate from the delta. `project()` skips reading when its cached pathname and offset equal the current watcher values. Otherwise it calls `problemlog.ReadFile` on the **current base path**, and the caller replaces the problem facts wholesale. This is important mitigation, but it is not a read of archived segments. [Projection][projection] [Publication][sync] [Fact replacement][facts]

### Explicit diagnosis also selects only the base file

`latestProblemsLog` first prefers the current logger's path when under the workspace. Otherwise it searches `*_problems.log` and selects a newest candidate. The timestamp inserted before `.log` means retained archives do not match that glob. `selfDiagnosisJob` reads that one path, replaces facts, asks for hot problems and constructs its investigation task. A report with no hot problems returns a reason instead of a job. [Selection][latest] [Task construction][diagnosis]

This route is real: routing reads the kernel's `self_diagnosis_lane`, and `processInput` uses it to call `selfDiagnosisJob` before returning a background-start message. The architecture separates investigation from its later fix continuation. Better evidence acquisition must not turn observation into automatic mutation or bypass the existing work admission. [Route extraction][route] [Route consumer][routeUse] [Chain specification][chainSpec]

## Static witnesses and their limits

### A retained unread tail falls out of both readers

Let P be the base problems pathname. The watcher has consumed through offset C in generation G0. After that collection, successfully logged records include a new error E in G0's remaining tail. A subsequent write triggers normal size rotation. G0 is retained under an archive name, and the incoming record is written to the new generation G1 at P.

At the next turn:

1. The snapshot still names P.
2. Collection opens G1. It never opens G0's retained archive, so E is not collected as a new problem.
3. A refreshed kernel report reads G1 only. If E had already been represented by an earlier whole-file projection, replacing the facts can also remove that representation.
4. Explicit diagnosis independently selects P and reads G1 only. It does not rescue E.

The strongest case puts E after the last projection as well as after the last collection. E then never enters either automatic evidence view, despite remaining in a readable retained archive. No equality of file sizes is necessary.

For a concrete bounded failure, G0 contains E while G1 contains only one warning whose signature has not reached the default three-occurrence hot threshold. Explicit diagnosis can report that its selected path holds no hot problems and start no investigation, although E remains in the same run's archive. The response names the selected path; this is not a fabricated claim that the code explicitly certifies the entire run error-free. The defect is the undeclared coverage reduction behind a run-oriented diagnosis route. [Hot rules][policy] [Default warning threshold][hotDefault] [Diagnosis disposition][diagnosis]

This can occur near the end of a long-lived 32 MiB segment with only a few additional records between turns. It does not require emitting 32 MiB between every pair of turns. A backlog from the per-turn drain cap is another way an unread old tail can exist. Actual frequency is unknown.

### Recreated size is not a generation identifier

Assume the pending batch was consumed, and the last successful projection is cached as `(P,C)`. After rotation, let N be the new base file's size at collection:

| Replacement state | Delta reader | Kernel projection |
|---|---|---|
| N < C | Resets to zero and reads the new generation, within its cap. | Usually refreshes from the whole new base when the resulting offset differs. Old archives remain excluded. |
| N = C | Returns as though nothing changed; no new records are collected. | Same pathname and offset satisfy the cache test, so no refresh occurs. Old projected facts remain; new base records are absent. |
| N > C | Starts at C, omitting the new generation's bytes before C. | If complete-line consumption advances the offset, the whole-base reread can recover the skipped new prefix into facts. |
| New pathname | Resets to zero. | Path change invalidates the cache. This control does not handle the writer's same-name replacement. |

For the equal-size case, enough writes can occur before the next turn that a recreated file reaches exactly the saved byte count, with different valid records of equal aggregate encoded length. Default rotation can produce that state; no application-level invariant prevents it. Equality is a narrow but deterministic cache-collision witness, not a claim that every rotation stalls refresh. An unchanged offset can also result when the new suffix has no complete line available; the complete-line behavior should remain separate from replacement detection.

For N > C, the omission in the **delta digest** must not be inflated into universal absence from the model's world. The independent full-base projection may make those records queryable and injectable. It still does not reconstitute the archived tail. If C lands in a replacement record, the parser ignores leading material before the next recognized header, which can omit that record too. [Parser framing][scan] [Collection][collect] [Projection key][projection]

## Coverage metadata currently describes different losses

`ReadFile` opens one file, parses and groups all its records, then keeps at most 64 distinct problems by urgency. `OmittedProblems` and `OmittedEntries` account for groups discarded **after that read**. They cannot count records in segments that were never enumerated. `self_problem_omitted` reflects this ranking cut, not rotation coverage. Likewise, pending `Dropped` counts parsed records beyond its 500-record cap, and `MoreInFile` describes the current byte-drain cap. None identifies a skipped generation. [Read and ranking][read] [Report fields][report] [Fact metadata][facts] [Pending fields][batch]

The warning surface can name cumulative process warning/error totals beside P. Those counters do not reset merely because the sink rotated, whereas P now names only one segment. Counters can therefore reveal a mismatch but cannot recover which signature or sample is missing. They are not a reader coverage manifest, and the present consumers do not use them as one. [Snapshot counters][mirror] [User-facing totals][summary]

The existing path-plus-byte-range reference is also unstable under replacement. That concern is **already anticipated** by the mission-improvement proposal, which requires log artifact identity and says rotated ranges must not silently become different observations. This report adds the exact collection/projection/diagnosis failure mechanism and should extend that requirement, not claim to originate it. [Existing provenance requirement][priorMission] [Current reference rendering][render]

## Boundaries and controls to preserve

- **Rotation is useful retention, not a bug by itself.** Once an archive is deliberately pruned or fresh-run cleanup removes ordinary logs, complete historical reconstruction may be impossible. Report that loss honestly; do not promise unlimited retention. The witness keeps its relevant archive present and readable. [Retention][names] [Existing cleanup contract][cleanup]
- **No rotation means no replacement witness.** Normal append, new-path detection, smaller-file reset, partial-line waiting and signature deduplication are useful controls. Keep pre-activation history out of the new-event digest even if the historical report includes it.
- **Existing pending samples may survive.** A batch already holds parsed raw problems in memory. This audit does not claim rotation erases every delivered observation, previous conversation message, category mirror or external archive.
- **Manual retrieval remains possible.** A person or permitted file tool can inspect the actual archived file. The failure is in these automatic selectors and freshness checks, not disappearance from all storage.
- **Projection recovery is conditional and real.** A larger replacement whose cursor advances can recover its prefix into current-base facts; an explicit diagnosis also performs a fresh base read irrespective of the watcher's cache. Neither enumerates the old segments.
- **Failure handling is a separate axis.** Missing/unreadable paths, rename failure, reopen failure and retention pruning need their own outcomes. The ordered witness assumes successful filesystem operations and no concurrent read/rename race.
- **No safety bypass follows.** Logs are evidence, potentially containing secrets and untrusted text. Improved retrieval must preserve workspace containment, typed permitted reads, redaction and the distinction between diagnosis and authorized repair. Scoped logging guidance also requires preserving sink ownership and avoiding error floods. [Logging guidance][loggingGuidance]

## Prior work and inspected coverage

The novelty claim is narrow and bounded by the inspected corpus.

- **Mission-driven improvement** already identifies unstable rotated-range provenance and proposes symptom-resolution receipts. It does not trace the base-only enumeration or path/offset projection cache examined here. Reuse its artifact-identity requirement. [Prior proposal][priorMission]
- **JSONL independent-writer rotation** concerns different appenders retaining stale open descriptors, including a fresh write to an unlinked generation. It explicitly excludes general logging. This witness needs one serialized writer, a successful retained archive, and a reader that reopens the base but does not identify its generation. [Distinct audit scope][priorJSONL]
- **Background outcome and continuation ownership** studies what happens after a diagnosis worker returns. Here acquisition can omit a problem before a worker is started. Its recognition of the existing grounded diagnosis chain remains valid. [Adjacent lifecycle audit][priorBackground]
- **Logging architecture and backlog** document rename-based size rotation as implemented. That writer capability is not an end-to-end reader coverage guarantee. [Rotation architecture][loggingSpec] [Completed writer item][loggingTodo]
- **Browser declarative QA** already asks that pruned evidence be reported unavailable, not mistaken for an empty result. That general principle applies, but its browser evidence and item-cap tests are not this problems-log integration. [QA precedent][qa]

The watcher test labelled truncation/rotation overwrites the same path with a **smaller** file. Kernel tests cover growing history and quiet unchanged turns; writer tests cover archive creation and retention; diagnosis tests use a single base file. These are useful positive controls. The inspected cases do not compose the real writer's rename/recreate with equal-or-larger replacement, retained old-tail recovery, and diagnosis coverage. They were read, not executed. [Watcher test][watcherTest] [Kernel test][kernelTest] [Writer test][rotationTest] [Diagnosis test][diagnosisTest]

Targeted searches included latestProblemsLog, projectedOffset, self_problem with rotation, debug-self with rotation, problems with rotated, and relevant QA/audit/journey terms, followed by pinned reads. No exact prior report of this combined reader boundary was located. This is not a guarantee about all historical discussions.

## Proposed contract and repair shape

**Each report identifies its run, included segment generations and read boundaries. Each delta cursor advances within one identified generation. Rotation cannot silently convert unseen retained records into an empty delta or unchanged projection. Unavailable generations are explicit gaps, not evidence of absence.**

1. **Choose coverage explicitly.** A sensible diagnosis default is the retained problems-segment family for one selected run, in order, plus a bounded snapshot of its active segment. If product policy intentionally chooses only the current segment, label that scope and expose earlier retained evidence rather than call it complete run coverage. Do not mix runs by globbing all problems files together.
2. **Give the writer and readers a shared segment vocabulary.** Reuse the logging package's archive naming and lifecycle instead of duplicating divergent globs in chat. Distinguish run identity, immutable segment generation, live alias and end watermark. A robust writer-issued generation is preferable to assuming pathname, size or timestamp alone is unique.
3. **Make collection generation-aware.** Compare the actual opened file against the cursor's generation. On replacement, finish unread retained segments in order and begin the new one at zero, subject to existing bounded work. If retention overtook the cursor, report an explicit gap and its known extent.
4. **Invalidate projection by evidence version.** Cache the generation set and captured read boundaries, not only the delta offset. Refresh the kernel from one coherent declared scope. Merge counts across that scope before applying the 64-problem ranking cut; distinguish ranking omissions from acquisition gaps and retention retirement.
5. **Preserve source references.** Bind pending ranges to their segment identities, potentially multiple segments per batch. Reading a former live alias must validate the artifact or report unavailable. This implements the existing provenance proposal rather than inventing a separate receipt system.
6. **Define the read/rotate race.** Merely adding `os.SameFile` around two pathname reads leaves a check-to-open window. Inspect the opened handle, capture a bounded end, and use a stable snapshot/inventory protocol. Avoid holding the writer's mutex while parsing many megabytes or while running a model. Portable behavior and Windows sharing require explicit verification.
7. **Keep executive decisions in policy.** Publish evidence coverage and freshness for Mangle to reason about. A missing log interval is neither “no errors” nor permission to replay a tool, restart the process or fix source automatically. Update the relevant architecture contracts first.

## Costs and fair evaluation

The current delta reader bounds each collection to 512 KiB, but a changed projection reparses the entire base file. `ReadFile` holds a map of **all** distinct groups before sorting and selecting 64, so that final fact cap is not a cap on transient aggregation memory. Reading every retained segment afresh each turn would improve coverage but multiply I/O and CPU. With T refreshes and B retained bytes, a straightforward full replay costs approximately O(TB), plus grouping and sorting; incremental per-generation aggregation trades memory and invalidation complexity for less repeated work. [Collection][collect] [Report implementation][read]

Three archives plus a live segment are nominally around four threshold-sized files, not a strict 128 MiB bound: a single incoming record can exceed the threshold and is written whole. Do not treat a 64-problem output or 600-character rendering target as proof that reading or retaining the underlying evidence is cheap. [Writer threshold][rotation] [Digest][render]

Measure local bytes read, parsed records, distinct groups, allocation peaks, reader latency, writer blocking, retained disk bytes, duplicate/missing evidence, and model tokens separately. A correct repair may increase diagnosis work because it now sees real failures. Compare cost per correctly evidenced diagnosis under identical retention and prompt budgets, not just fewer reported problems.

### Unrun evaluation matrix

| Case | Required oracle |
|---|---|
| Normal append without rotation | Same deltas, deduplication and facts; no redundant read on genuinely unchanged evidence. |
| New path and smaller same-path replacement | Existing controls preserved; generation metadata updated. |
| Same pathname replaced at equal, smaller and larger sizes | No skipped prefix; projection refreshes on generation change even when byte offset is unchanged. |
| Unread error in retained old tail, new base below warning threshold | Error appears in diagnosis coverage or an explicit declared exclusion; never silently treated as absent. |
| Two or more rotations between turns | Ordered retained-segment continuation; no duplicates and honest gaps if pruning wins. |
| Warning signature split across segments | Counts and hot classification match the declared aggregate scope. |
| More than 512 KiB unread and more than 500 pending records | Work remains bounded; backlog and intentional cuts remain distinguishable from missing generations. |
| More than 64 distinct problems | Ranking omissions reported after complete scoped acquisition; no false claim that ranking metadata describes unread files. |
| Rotation with an unconsumed batch and later range retrieval | Samples remain valid; references identify original segment(s), never replacement bytes. |
| Rename/reopen/read failure, prune, restart and missing archive | Explicit incomplete/unavailable outcomes without log-feedback loops or unsafe recovery. |
| Production integration | Actual enabled logging writer, turn collection, kernel hot rules and explicit diagnosis route; default 32 MiB behavior checked separately from small package seams. |
| Platform and cost comparison | Supported local filesystems and Windows sharing; equal workload before/after, full I/O/memory/latency/token accounting. |

A useful exit criterion is not “rotation created an archive” or “the new file was reread.” It is that every record within the declared retained coverage is represented exactly once in the appropriate aggregation, or is accounted for by an explicit bound/loss state, and that same-size replacement cannot leave a supposedly current projection unchanged.

## Primary references

- [GNU Coreutils tail documentation][gnu-tail] distinguishes following a pathname from following an open descriptor and separately detects truncation. It is a practical precedent that rotation and size decrease are different events, not a requirement to run tail or adopt its complete implementation.
- [Go os.SameFile documentation][go-samefile] defines file-identity comparison and notes platform-dependent semantics. It is a possible mechanism, not proof that one identity check supplies stable multi-segment coverage.
- [Linux rename documentation][linux-rename] explains that renaming changes names without retargeting existing descriptors. This supports the distinction between a live pathname and a particular retained file; the audit's ordered witness does not depend on holding a reader descriptor across rotation.

These references support a narrow engineering inference: an offset is meaningful only within an identified artifact, and completeness is meaningful only relative to a declared set of artifacts.

[rotation]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/logging/rotate.go#L76-L145
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_debug.go#L281-L355
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_debug.go#L393-L419
[latest]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_diagnosis.go#L31-L51
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/agents.md#L17-L24
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/logging/logger.go#L655-L681
[mirror]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/logging/logger.go#L683-L753
[names]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/logging/rotate.go#L163-L208
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/logging/rotate.go#L29-L40
[limits]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/logging/rotate.go#L211-L240
[config]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/config/logging.go#L87-L113
[session]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/session.go#L194-L228
[toggle]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_debug.go#L543-L587
[watcher]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_debug.go#L208-L261
[turn]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/process.go#L148-L155
[sync]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_debug.go#L438-L503
[articulation]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/process.go#L993-L1004
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/problemlog/facts.go#L27-L76
[diagnosis]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_diagnosis.go#L53-L100
[route]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/delegation_routing.go#L292-L303
[routeUse]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/process.go#L570-L588
[chainSpec]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/Docs/architecture/conversation-first-agent.md#L101-L114
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/core/defaults/policy/self_problems.mg#L1-L46
[scan]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/problemlog/problemlog.go#L39-L88
[read]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/problemlog/report.go#L63-L126
[report]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/problemlog/report.go#L10-L60
[batch]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_debug.go#L80-L97
[summary]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/system_warnings.go#L33-L40
[priorMission]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/Docs/innovations/mission-driven-forever-improvement.md#L139-L147
[render]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_debug.go#L121-L186
[cleanup]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/Docs/architecture/logging/WIRING-AND-NOT-BUILT.md#L85-L93
[loggingGuidance]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/logging/agents.md#L1-L7
[priorJSONL]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/Docs/audits/jsonl-independent-writer-rotation.md#L12-L33
[priorBackground]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/Docs/audits/background-outcome-and-continuation-ownership.md#L19-L36
[loggingSpec]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/Docs/architecture/logging/INTERNALS.md#L79-L91
[loggingTodo]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/Docs/architecture/logging/TODO.md#L34-L42
[qa]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/.quality_assurance/qa_audit_browser_declarative.md#L324-L337
[watcherTest]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_debug_test.go#L77-L99
[kernelTest]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/self_debug_kernel_test.go#L48-L129
[rotationTest]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/logging/rotate_test.go#L11-L61
[diagnosisTest]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/cmd/nerd/chat/background_test.go#L89-L121
[gnu-tail]: https://www.gnu.org/s/coreutils/manual/html_node/tail-invocation.html
[go-samefile]: https://pkg.go.dev/os#SameFile
[linux-rename]: https://man7.org/linux/man-pages/man2/rename.2.html
[hotDefault]: https://github.com/theRebelliousNerd/codenerd/blob/df135126863e39301d42df083b5541f9a27b8493/internal/config/logging.go#L73-L85

