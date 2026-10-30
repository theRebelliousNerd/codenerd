# Legacy build project diagnostic evidence parsing

## Scope and conclusion

Read-only source review of CodeNERD main at **`8d373fb6ca05ef03fec59b923c5f1925066d623a`**, rechecked on 2026-10-30. This document proposes a bounded implementation and evaluation pass. No repository code, tests, builds, model calls, or benchmarks were run during this review.

**The legacy `build_project` action can lose valid Windows-path diagnostics and promote unrelated colon-delimited output into `diagnostic/5` facts.** Its parser splits at the first three colons and validates only the second field as an integer. These are policy inputs, not merely display annotations. The recent arity/type repair makes their representation valid but does not establish that their contents describe a compiler diagnostic.

This is a structured-evidence fidelity defect, **not a demonstrated successful-build misreport, commit bypass, or unsafe code adoption**. The handler retains the failed execution status and full output. Actual downstream work depends on the active intent, other facts, and the consumer; the examples below are source-derived, not reproduced incidents.

## What the existing repair accomplished

Commit [`032df1420a07930894ec224d4e915283bd36f9ba`](https://github.com/theRebelliousNerd/codenerd/commit/032df1420a07930894ec224d4e915283bd36f9ba) correctly repaired this producer to match the declared `diagnostic(Severity, FilePath, Line, ErrorCode, Message)` shape: five arguments, severity as a name, line as a number, and file/code/message as strings. It also removed a keyword filter that missed ordinary Go messages such as `undefined: symbol` when they did not contain the literal word `error`.

The new [contract test](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/mangle_review_contracts_test.go#L131-L150) supplies a relative-file error, a warning, and a malformed line-number field; it asserts the resulting facts into a real kernel. That is useful coverage and should remain. It does not cover drive-letter paths, irrelevant timestamped output, malformed column fields, or nonpositive locations. The residual here is lexical admission, not a request to restore the old keyword filter or four-field representation.

This is separate from [compile diagnostic witness admission](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/compile-diagnostic-witness-admission.md), which concerns turning a recurse finding category into a stock-vet analyzer flag. The present producer is the legacy VirtualStore build action; it does not construct recurse witnesses or change gate receipts.

## The implemented producer and consumer path

1. **Dispatch and execution.** `executeAction` sends `ActionBuildProject` to `handleBuildProject`; `ActionRunBuild` instead takes the modular-tool path. The legacy handler defaults to `go build ./...`, while an explicit command payload can select a wrapper or other build command. It creates a host-shell command, uses the workspace directory/environment and a timeout, and executes it through the injected executor. See [routing](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/virtual_store_routing.go#L442-L450), [command selection](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/virtual_store_actions.go#L299-L338), and [handler](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/virtual_store_actions.go#L490-L540).

2. **Outcome and extraction remain separate.** Success requires a nonnil result, no executor error, `result.Success`, exit code zero, and no result error string. The handler emits `build_result(success, output)` and parses diagnostics only when that success check fails. It returns `ActionResult.Success`, the full output, and the collected facts. A parsing miss therefore does not turn this result into success. [Handler outcome](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/virtual_store_actions.go#L512-L540).

3. **Facts reach the kernel.** For a normally returned failed `ActionResult`, `RouteActionResult` still records completion. `recordActionCompletion` appends the handler's `FactsToAdd` and calls `injectFacts`; it separately records the execution outcome. The early route-error branch is not needed for these examples because the handler represents the unsuccessful build in its result and returns a nil handler error. [Completion path](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/virtual_store_routing.go#L132-L210).

4. **Policy consumes the extracted content.** An error diagnostic derives a diagnostic-based commit blocker. A diagnostic whose file matches the current coder target or an impacted file derives repair priority; those priorities contribute context relevance, and an error at the target contributes coder guidance. With a `/fix` intent, any error diagnostic supports the TDD repair strategy. See [build policy](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/coder_build.mg), [coder guidance](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/coder_guidance.mg), and [strategy selection](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/strategy.mg#L5-L11). The JIT compiler actually queries and renders matching `injectable_context` rows; this is not solely an unused schema. [Prompt consumer](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/prompt/compiler_kernel.go#L207-L259).

These consumers establish why evidence fidelity matters, but not that every parsing error changes a final decision. Other diagnostics or build/test facts can already establish the same condition; an absolute path may not match a relative coder target; other safeguards remain. In particular, [the commit barrier](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/commit_gate.mg#L5-L28) also requires positive build/test evidence. Missing a diagnostic-based blocker is not by itself permission to commit.

## Two source derived witnesses

The relevant code is [parseBuildDiagnostics](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/virtual_store.go#L1136-L1162). It takes each line containing a colon, calls `SplitN(line, ":", 4)`, parses only `parts[1]`, and emits file `parts[0]` and message `parts[3]`. The column field is neither parsed nor checked. The examples are proposed fixture inputs; their outcomes follow from those operations and have not been executed here.

### A valid location disappears

```text
C:\CodeProjects\codeNERD\main.go:12:7: undefined: thing
```

The four fields become `C`, `\CodeProjects\codeNERD\main.go`, `12`, and `7: undefined: thing`. Converting the second field to an integer fails, so the parser emits no diagnostic. This is the same Windows diagnostic shape already covered by the repository's [tactile regression test](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/tactile/audit_test.go#L1233-L1243). The legacy handler also explicitly supports the native Windows shell; Windows is not an excluded platform. [Shell selection](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/virtual_store_actions.go#L1220-L1228).

The conditional reachability claim is narrow: if an unsuccessful legacy build returns this supported diagnostic format, structured extraction loses it. This does not claim that every ordinary `go build ./...` invocation emits absolute paths, or that no other producer can supply the same diagnostic. Relative filenames without embedded colons continue to parse.

### A log line becomes a source error

```text
2026/10/30 07:38:15: retrying dependency fetch
```

The second field is `38`, so the parser emits an error diagnostic with file `2026/10/30 07`, line `38`, empty code, and message `retrying dependency fetch`. Nothing establishes that the first field is a source path or that the third field is a valid source column. A failed explicit build wrapper can legitimately mix operational logs and compiler output; the command payload supports that entry path. This example is not a claim about default Go compiler log formatting.

Such a fact can add a misleading diagnostic-based blocker or support a repair strategy, but it does not make a successful invocation fail: this parser runs only after the handler already judged the invocation unsuccessful. Nor does the bogus timestamp path automatically receive target-specific context priority, which requires a matching target or impacted-file fact. The immediate proven defect is false structured attribution.

Malformed location strings such as `a.go:12:not-a-column: note` are a related negative fixture. Accepting them because the line field is numeric demonstrates the same admission gap; it is not a separate audit finding.

## The tactile parser is a useful comparison but not a rescue path

Tactile's [Go diagnostic pattern](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/tactile/audit.go#L29) anchors a `.go` filename followed by numeric line and column fields. Its greedy file group preserves the drive colon, and [AnalyzeBuildOutput](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/tactile/audit.go#L1092-L1139) checks numeric conversion and a positive line. The timestamp example does not satisfy that grammar. Its [implemented specification](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/architecture/tactile/IMPLEMENTED_SPEC.md#L424-L436) explicitly scopes this to Go, including Windows drive paths, rather than a universal compiler parser.

Two boundaries prevent assuming this already repairs the legacy route:

- Tactile's automatic analysis recognizes an actual `go` or `go.exe` binary and its subcommand. The legacy build action supplies `bash` or `cmd.exe`, so the shown automatic classifier does not select its Go-build analysis. [Classifier and adapter](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/tactile/audit.go#L983-L1039).
- Tactile emits request-scoped `execution_diagnostic/6`, while the legacy consumer described above reads `diagnostic/5`. These are different contracts; interchangeable-looking messages do not create an automatic bridge. [Tactile fact conversion](https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/tactile/audit.go#L1159-L1195).

Do not blindly replace the legacy helper with all of `AnalyzeBuildOutput`. Its severity classification uses a different rule, and its standalone analysis success is based on parsed error count. The audit adapter overwrites that success using the execution result. A repair must preserve the legacy handler's exit-based failure authority and its declared fact shape; parser agreement alone cannot establish command success.

## A bounded repair direction

1. **Specify the accepted location grammar.** Preserve colon-bearing paths, validate both coordinates under an explicit supported convention, and keep message colons intact. Reject unrelated operational text as a source diagnostic without dropping it from the raw build result. Define handling of whitespace, CRLF, coordinate overflow, zero/missing columns, and compiler-originated adjusted filenames rather than adding ad hoc exceptions after each failure.
2. **Share lexical parsing where contracts agree.** Prefer one deterministic location parser with separate adapters for `diagnostic/5` and `execution_diagnostic/6`, or demonstrate equivalent behavior with shared fixtures. Preserve severity names, numeric line types, the empty code field when no code exists, and message/string identity. Do not convert diagnostic content into commands or allow the model to invent locations.
3. **Keep tool scope honest.** The legacy action accepts custom commands, but this helper claims Go-style extraction. Inventory supported non-Go outputs before enforcing a `.go` restriction everywhere. If another compiler is intended, select a typed parser explicitly or retain the output as unclassified failure evidence; permissive colon splitting is not a multi-language contract.
4. **Do not broaden the claim into lifecycle repair.** Diagnostic retirement, provenance/attempt ownership, path normalization against workspace identity, and legacy-versus-modular routing deserve their own contracts. This pass should not claim to solve those merely by parsing a drive path correctly. Unknown or truncated output must not become a clean build.

This repair needs no additional model invocation and no repeated build to reinterpret already captured output. Whether a shared parser belongs in tactile or a smaller dependency-neutral package is an implementation choice to evaluate, not a reason to duplicate a third parser.

## Costs and fair evaluation

The expected benefit is more accurate structured evidence, not a measured speedup. Parsing itself is small compared with a build, but incorrect diagnostics may waste later inspection, prompt context, model turns, retries, or human debugging. Count those costs only when an end-to-end fixture actually triggers them. A fact difference alone does not prove a saved model call.

A repair also has costs: parser migration and compatibility review, stricter-format false negatives, cross-platform fixtures, extra allocations or regex work, and potentially more correctly retained facts. Retaining all diagnostic details can increase kernel indexing/fixpoint and prompt costs. If a cap is added, keep explicit completeness information; silently dropping errors to improve speed would weaken the evidence contract. Tactile's 100-detail-fact limit is useful context, not permission to copy that policy into a different consumer without evaluation.

For an authorized implementation pass, compare the existing main and candidate against the same captured output corpus, command result, policy facts, tool versions, and cache conditions. Start with pure parser and kernel assertions, then use a fake executor through the real routed build action to establish delivery. Keep constitution/permission setup explicit; bypassing those in a handler-only test does not prove route coverage. No paid-model evaluation is needed to prove the lexical defect.

| Case | Required observation |
|---|---|
| Existing relative-file error without the word error | Preserve exact file, line, message, severity and five-field type contract. |
| Existing warning and malformed line-number case | Preserve the intentional distinction and current rejection. |
| Windows drive path with backslashes and with forward slashes | Keep the entire path and correct coordinates; do not split the drive prefix. |
| Unix absolute/relative paths, spaces, and colons in the message | Preserve supported identities without greedy capture into the message. |
| Timestamped wrapper log mixed with real compiler diagnostics | Keep log text in raw output; emit only supported source diagnostics. |
| Missing, negative, zero, nonnumeric or overflowing coordinates | Follow the documented grammar, with no invented location or panic. |
| Failed command with no recognized diagnostics | Remain unsuccessful; zero extracted diagnostics is not proof of a clean build. |
| Executor error, timeout, nil result or output truncation | Preserve failure/unknown execution semantics independently of parsing. |
| Allowed routed legacy build versus direct tactile Go build | Verify each fact family and intended consumer, without assuming a bridge. |
| Coder target and matching valid error | Demonstrate expected repair priority/guidance; separately test a nonmatching path. |
| Unrelated log and `/fix` intent | Do not add diagnostic-driven policy conclusions solely because of the log. |
| Existing independent blocker or other diagnostic | Preserve safety behavior; report when the parser fix changes evidence but not the decision. |

Report precision and recall against labeled diagnostic records, exact location/message fidelity, emitted fact count, unchanged execution status, and relevant policy-query results. Also report parsing CPU/allocation cost, kernel assertion/query cost, retained output/fact bytes, and any measured end-to-end work difference. A fair success criterion is **better attribution without weaker failure semantics or loss of previously supported formats**, not simply fewer facts or fewer blockers.

## Recommended disposition

Treat this as a small follow-on to the arity/type repair: challenge the source-derived fixtures, settle the supported grammar, and test the complete routed fact-consumer path before implementation is accepted. Keep the finding scoped to legacy build diagnostic extraction. The evidence supports a concrete fidelity defect; it does not establish production frequency, autonomous repair failure, or a bypass of CodeNERD's independent acceptance safeguards.
