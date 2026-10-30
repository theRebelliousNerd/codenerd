# Literal target names can become workspace-wide delegated review scope

## Finding and decision

At main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), the shared task formatter treats any target containing the case-insensitive substring `all` as a broad request. A literal filename such as `callback.go` therefore selects discovery across the workspace rather than just that file. For `/security`, `/analyze`, and the single-reviewer `/review` path, a nonempty discovered list replaces the literal target in the delegated task.

The clearest source-derived witness is `/security callback.go` in a workspace containing admitted `callback.go` and `unrelated.go`. The handler announces analysis of `callback.go`, while its execution request names both discovered files. This mismatch does not require a mistaken perception result, an ambiguous sentence, a multi-step request, or a model decision. It occurs in deterministic task construction. Actual additional reads, latency, token cost, findings, or task failure remain conditional on downstream execution and have not been measured.

**Recommended change:** preserve literal target identity independently of scope intent. Resolve an explicit file as that file; enumerate the workspace only when a separate, deliberately recognized broad-scope request authorizes that interpretation. Retain workspace membership, genuine whole-codebase coverage, original request text, structured targets, normal policy, and existing execution controls.

This is a static continuation of the repository's target-fidelity work, not a new claim that task projection or string-based routing is generally problematic. Main was rechecked on October 30, 2026 and remained at the pinned revision. Guidance, current audit inventories, and research additions through `research-main-20261030-1147` were reviewed. No application, tests, model calls, or benchmarks were run; no repository files were changed.

## Existing work this finding preserves

The [C-06/S2 contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/journeys/04-contract-registry.md#L156-L187) already distinguishes a formatter's location argument from the person's words. The [slash-command specialist-target audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/slash-command-specialist-target-continuity.md) traced a different residual: a parsed path was replaced by unsplit prose at specialist resolution. Current `/fix` and `/refactor` handlers now carry a `slashTarget` containing separate `Target`, `Constraint`, and `Request` fields. That is a real repair, not an absent feature. [Current handlers](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_evolution.go#L11-L62).

This report concerns the next, narrower question: does a valid literal target remain the same scope when formatted? Here the target is already just `callback.go`; no requirement text needs splitting. Correct parsing alone cannot prevent the formatter from interpreting letters inside that path as a whole-workspace instruction. Nor is this the recent multi-step literal-case issue: the witness uses one command and one literal target, without multi-step extraction.

The current system also retains original requests and structured targets in perceived-intent delegation, admits discovered files through workspace membership, runs slash tasks inside a foreground operation, and has a scope-aware specialist-review resolver. These controls constrain the finding and provide useful repair components.

## Production path and exact transformation

### 1. Normal slash dispatch supplies an exact target

The command dispatcher has direct `/security` and `/analyze` handlers. `handleCmdSecurity` assigns `parts[1]` to `target`, invokes `formatShardTask("/security", target, "", m.workspace)`, displays `Running security analysis on: <target>`, and calls `spawnShard("reviewer", task)`. `/analyze` has the same shape with a complexity-analysis task. There is no specialist-registry prerequisite for these two handlers. [Dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands.go#L184-L190), [handlers](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_analysis.go#L593-L628).

### 2. A substring changes the scope category

After normalizing an empty target and workspace-relative paths, `formatShardTask` decides to discover files when the target is `codebase`, contains lowercased `all`, or contains `*`. Thus `callback.go`, `install.go`, and a path below an `install/` directory satisfy the same broad-discovery condition as an intentionally broad target. This is substring matching, not a whole-word instruction parser or a check that the path is a directory. [Formatter condition](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L501-L521).

For `/review`, `/security`, and `/analyze`, a nonempty list wins over the original target. The respective tasks become `review files:...`, `security_scan files:...`, and `complexity files:...`. Only when the list is empty does execution fall through to a single-target representation, apart from the explicit `codebase` case. `/fix` does **not** consume this list: although the shared preamble can perform discovery, that verb's task remains `fix file:<target>`. The demonstrated scope broadening is not a claim about every formatter arm. [Verb arms](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L523-L552).

### 3. Discovery is workspace-scoped, not target-scoped

`discoverFiles(workspace, constraint)` receives no target. It walks the workspace and selects admitted files with allowed code extensions, optionally narrowed by a language-like constraint. With the empty constraint used by `/security`, those extensions are `.go`, `.py`, `.js`, `.jsx`, `.ts`, `.tsx`, `.rs`, `.java`, `.c`, `.cpp`, and `.h`. This is **not literally every workspace file**. Membership failure returns no list; individual paths must pass `mem.Admit`. [Discovery implementation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_multistep.go#L257-L309).

The list deliberately has no arbitrary file-count cap. Its comment explains the earlier defect: a capped list falsely represented a prefix of a codebase as the whole target. Keep that correction. The present issue is deciding to enumerate the workspace for a narrow target, not that genuine broad requests receive complete discovery. [Coverage rationale](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_multistep.go#L311-L314).

### 4. The task is handed to the real executor

`spawnShard` wraps work in `foregroundOperation` and invokes `spawnTask`; the ordinary non-system reviewer path builds a `TaskRequest` and calls the task executor. `shardTypeToTaskRequest` supplies `IntentVerb` and `Task`, not separate `Target` or `Request` fields. Consequently, this slash path does not independently carry `callback.go` in those fields to counter the expanded task. This does not establish that the original slash input is absent from all history or context. [Foreground spawn](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L792-L802), [ordinary dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L35-L79), [request builder](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_routing.go#L361-L379).

The precise witness concerns the formatter output and `TaskRequest.Task`, **not the complete final model input**. The executor normalizes the reviewer persona and can prepend its intent word to the inline task. It also has isolated execution paths and normal prompt construction. None of those facts turns the formatter's file set back into the original single-file set at this boundary. [Executor normalization and inline construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L358-L433).

## Conditional witness and important controls

Assume a normally initialized, idle TUI with a functioning reviewer task executor and a workspace where `callback.go` and `unrelated.go` are both admitted, readable, supported source files. Submit `/security callback.go` through the ordinary command entry. With those two files as the discovery set:

| Boundary | Source-derived value |
|---|---|
| Handler target | `callback.go` |
| Initial status | `Running security analysis on: callback.go` |
| Formatter / `TaskRequest.Task` | `security_scan files:callback.go,unrelated.go` |
| Separate slash request target | Not set by `shardTypeToTaskRequest` |

The relevant oracle is the file **set**, not reliance on listing order. The original file can still be included, so this need not omit the requested work; it introduces unrelated source scope. Possible effects are unnecessary reads, diluted analysis, unrelated findings, and higher cost, but the model might focus on the correct file anyway. Normal tool, workspace, authorization, and work-mode checks remain in force. This is neither a demonstrated write nor an out-of-workspace access or privilege-escalation claim.

Controls that must remain explicit:

- **Empty discovery:** membership initialization failure or no matching files leaves `fileList` empty, so this particular non-`codebase` target uses the exact-target fallback.
- **Unaffected names:** `handler.go`, absent `all` or `*`, does not activate this condition. A case change does not protect a filename containing `all`, because the check lowercases the target.
- **Specialist `/review`:** with a nonempty registered-agent set, `handleReviewCommand` takes `spawnMultiShardReview`, not this fallback formatter. Its `resolveReviewTarget` recognizes broad scope only for exact `.`/`codebase`/empty targets; otherwise it stats the specified path, confines directory traversal to that directory, and admits files through membership. This is a substantive counterexample, not a claim that every `/review` invocation expands. [Review branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L1001-L1070), [target resolver](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L948-L1000).
- **Perceived-intent delegation:** a routed background review can use the same formatter, but the job preserves the original `Target`, `Constraint`, and raw `Request`, and copies them into `TaskRequest`. `TaskText` presents the person's request alongside the synthesized task. That is conflicting narrow and broad information, **not complete loss of the original request**; downstream model recovery is possible. Foreground perceived-intent requests likewise have a field-preserving builder. [Background construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L559-L594), [background projection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/background.go#L156-L200), [request text](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L28-L65).
- **Later disclosure:** successful slash completion includes the task in its displayed response. The expanded scope is not necessarily permanently hidden, even though initial progress names the narrower target. Disclosure after work is different from preserving scope before dispatch. [Completion surface](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L835-L851).

## Repair contract

Prefer a shared target record over another prose heuristic:

1. Preserve `Request`, literal target spelling, constraints, and an explicit target kind: file, directory, supported pattern, intentionally whole workspace, or unresolved. A path's internal letters are never evidence of broad intent.
2. For an explicit file, resolve only that file through normal workspace admission. For a directory, root traversal at that directory. Unknown or invalid targets remain unresolved; they must not silently become the workspace.
3. Broad discovery requires an explicit broad-scope classification or documented reserved command syntax. Define how a real file named like a reserved word is escaped or disambiguated. Keep Mangle's authority over policy; a scope record is evidence and execution context, not a new permission grant.
4. Treat supported globs as patterns whose matches are confined to their declared root, not as an instruction to discard the pattern and enumerate all source. The current `*` branch shares that risk, but the primary witness here needs no glob. The existing specialist resolver uses literal `os.Stat` and is not a drop-in complete glob parser.
5. Construct status, specialist selection, task text, and structured execution fields from the same resolved scope. Keep original wording separately; do not remove it merely because a target is typed.
6. Record resolution outcome, target kind, requested scope, admitted count, and any deliberate broadening. Keep path-bearing diagnostics local or appropriately scoped. If expansion is necessary beyond the user's request, expose that decision rather than silently claiming single-file work.

A small compatibility repair can first remove substring-based broad detection for literal paths and carry the existing request fields through the security/analyze slash adapters. That reduces this witness without requiring a new model call. The fuller resolver should unify behavior across affected formatter callers and specialist review, while retaining each path's legitimate options and restrictions. Simply asking the worker to ignore irrelevant files leaves the pre-dispatch contract wrong; reintroducing a first-N cap hides rather than resolves it.

## Costs and fair evaluation

Scope resolution adds implementation and compatibility work: a target grammar, filesystem errors, nonexistent files, directory and pattern handling, shared representation, and diagnostics. Exact-file resolution should avoid a needless whole-workspace walk; correctly scoped tasks can also shorten task strings and reduce irrelevant context. Those are plausible savings, not measured results. Genuine whole-codebase requests still require adequate coverage and may remain expensive. Never score a capped or incomplete broad review as an efficiency improvement.

The existing formatter integration test checks only that four tasks are nonempty in a temporary workspace; it does not use a populated workspace or a filename containing `all`. The fix/refactor regression tests correctly assert filename and brief preservation and should remain. Their success would not establish this separate review-scope invariant. [Formatter test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/integration_test.go#L661-L683), [brief regressions](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_fix_brief_test.go#L8-L69).

For an authorized future evaluation:

- Enter `/security callback.go` and `/analyze callback.go` through normal TUI command handling in a real initialized fixture, then capture the execution request. First prove the structural mismatch without requiring model compliance.
- Compare equivalent explicit-file requests for `handler.go`, `callback.go`, mixed-case variants, and files below `install/`. Renaming a single target must not change the scope category. Include an unrelated admitted source file so broadening is observable.
- Exercise single-reviewer fallback and populated specialist `/review` separately; preserve the latter's exact-target behavior. Check `/fix` as a negative control rather than expecting its task to broaden.
- Include empty discovery, excluded paths, membership errors, unsupported extensions, directories, nonexistent paths, supported patterns, and intentional broad requests. Verify both containment and completeness under the documented grammar.
- For natural-language requests, inspect the actual perceived target, retained original words, task text, and structured fields. Do not force a classifier result and present that as a normal-entry quality measurement.
- Only then measure actual file reads, irrelevant-file fraction, findings attributable to the requested target, target coverage, elapsed time, tokens, tool calls, and recovery behavior under the same model/configuration. A worker's successful correction is evidence of mitigation, not proof the request was faithfully constructed.
- Keep real tools, membership, normal policy, foreground ownership, and usual model access enabled. Do not obtain attractive scope or cost numbers by disabling discovery, replacing execution with a canned result, or narrowing tool visibility beyond ordinary production controls.

The acceptance criterion is modest and concrete: an explicit admitted file remains that file at the delegation boundary regardless of whether its spelling happens to contain `all`, while a genuinely broad request continues to receive the intended complete, admitted scope.
