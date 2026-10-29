---
doc-class: deep-dive
subsystem: tools
implementation-status: proposed
last-verified: 2026-10-29
verified-against: ca94da614edff9157acf31bb54f65e4f9695ae92
verification-method: static-source-inspection
supersedes: []
---

# Explicit package selector fidelity

## Conclusion

The verification backend can erase a valid, explicitly supplied Go package selector before deciding what to build or test. The optional-path compatibility change introduced in `400c2f32958dcf978d478f3b00ad640500fb9008` applies directory-omission semantics to package-array entries. Those semantics treat `.` and `./` as omitted, although both select the package in the working directory. The behavior remains in inspected main revision `ca94da614edff9157acf31bb54f65e4f9695ae92`. [Introduction][introduction] [Current package normalization][normalization] [Directory omission rules][omission]

The consequence depends on context: an explicit root-package request can become a no-op, a written-package-only run, or a recursive run. A mixed selector list can also silently lose the root package. This is a source-derived conditional finding, not an executed incident, guaranteed false completion, permission bypass, or fabricated test result.

The proposed correction separates directory-default normalization from package-selector normalization, preserving meaningful explicit selectors while retaining the intended omitted-directory compatibility behavior.

## The meaning lost at normalization

For both `[]string` and `[]any` package arguments, `executeTypedVerification` now calls `optionalPackages`. That helper discards every entry for which `tools.IsOmittedOptionalPath` returns true. The shared predicate includes empty strings, `.`, `./`, and case-insensitive null-like spellings. [Array handling][normalization] [Package filtering][package-filter] [Shared predicate][omission]

That rule is reasonable for a directory field whose omitted value means the current workspace, but not interchangeable with package selection. Go's command documentation states that a package path beginning with a `.` path element denotes the package in the corresponding directory. This official source is an API reference, not proof of the installed toolchain version. [Go package-path semantics][go-reference]

The backend's own validation explicitly permits `.` and relative `./` selectors while rejecting escaping paths. Thus the selector is meaningful and admitted by the intended contract; it disappears before that validation can preserve its meaning. In the parent revision `b2be2799914ec942c90e08a7b487adc405c6428c`, package lists were copied without this filtering. [Selector validation][selector-validation] [Parent behavior][parent-normalization]

## Conditional outcomes

Assume an admitted verification-tool call with a string-array package argument, a recognized Go module at the workspace root, successful directory resolution, and otherwise unchanged options. Rows concern `run_tests` unless `run_build` is specified. These are expected decisions from source, not observed executions.

| Explicit selection and context | Selection or action after filtering | Difference from request |
| --- | --- | --- |
| `["."]`; session test-scope callback exists but returns no packages | `ran: false`; no process starts | Explicit root check is skipped |
| `["."]`; callback returns only existing `./internal/alpha` | Written-package test run for `./internal/alpha` | Root package is not requested |
| `["."]`; no test-scope callback, or tool is `run_build` | Default `./...` | Recursive scope replaces root-only scope |
| `[".", "./internal/alpha"]` | Only `./internal/alpha` remains | Root selector is removed without fallback |
| `["./..."]` | Unchanged | No selector loss |

An empty normalized package list defaults to `./...`, except that eligible test calls substitute written packages or return the no-work result. A nonempty surviving list does not take that fallback. If the written scope itself contains `.`, the resulting selection may coincide with the request incidentally. Recursive `./...` also includes the root package; therefore this is not universally a missed-root-check defect. It can instead add unrelated work and failures. [Default and scoped selection][selection] [Scope eligibility][scope]

The same issue can affect `./`. A nested working-directory example needs an important qualification: framework detection recognizes Go through its markers, including `go.mod`, rather than treating every ordinary subdirectory as an independently detected Go project. A recognized nested module below the workspace root is a valid broadening case because written-scope substitution requires the canonical workspace root; an arbitrary subdirectory without a recognized marker is not the witness. [Framework detection][framework] [Scope eligibility][scope]

## Production path and retained safeguards

`run_build` and `run_tests` declare package arrays and invoke `executeTypedVerification`. The registry validates arguments and applies the allowlist and installed guard before execution. A package array containing `.` is a legitimate admitted shape. In the session dispatch path, `WithTestScope` supplies a callback based on the turn's written paths before `executeToolCall`; modular dispatch carries that context into `Registry.Execute`. Thus the no-work and written-package branches are reachable in the normal session path, not merely helper-only hypotheticals. [Tool definitions][definitions] [Registry dispatch][registry] [Session scope installation][session-scope] [Modular dispatch][modular-dispatch]

This does not establish universal native-JSON-null support. Registry validation rejects null for declared array and string fields, and rejects an invalid working-directory type before the helper runs. Direct helper tests accepting nil do not establish admission through the normal registry. A scalar `"."` package argument is likewise not the production witness: the schema declares an array. [Argument validation][argument-validation]

The backend remains truthful about the result it actually produces. `nothingWrittenToTest` reports `ran: false`, a directory, and a reason, without an exit code. It returns no tool error, but does not claim a successful test process. When execution occurs, the backend records the actual argv and exit status and reports the actual directory, scope, and output. Build output likewise reports its actual command. [No-work result][no-work] [Execution and reporting][execution] [Build reporting][build-report]

Receipts retain an actual-run contract and reject negative exit codes. Session counters and sequencing advance on receipts, not merely on a no-run response. Workspace test-plan matching checks exact argv and ordering after the last write. These protections can prevent a mismatched run from satisfying a separately declared obligation. Other gates or acceptance checks may rerun the needed verification, and a model may notice the returned scope and correct its request. None restores the original argument before this backend decision, but each limits stronger completion claims. [Receipt contract][receipts] [Receipt consumption][receipt-consumption] [Obligation matching][obligation-matching]

## Counterevidence and audit scope

Existing omission tests deliberately classify `.` and `./` as omitted directories; that behavior need not change. A build fixture supplies `["."]` but checks artifact side effects rather than selector fidelity. Null-compatibility cases exercise aliases, not preservation of explicit dot selectors. These tests were read, not run, and helper coverage is not evidence of normal registry admission. [Directory tests][omission-tests] [Build fixture][build-tests] [Compatibility tests][compatibility-tests]

The audit screened 92 main audit documents. Upstream test-request delegation, test-evidence receipts and obligation matching, and numeric argument coercion are adjacent but distinct: this finding concerns valid explicit selection erased inside the verification backend after handoff. The research lane at `9d884332bbb3fc6d9676fc1acc07a26bc7f6d21a` adds memo identity, test-removal subject identity, and inclusion projection, not this selector issue.

Selected active lanes were checked before the main merge, not exhaustively audited. The current main versions of the verification and workspace-guard files retain the same relevant blobs as the introducing change. This proposal does not supersede those neighboring audits or justify reverting the entire compatibility fix.

## Proposed correction

Use separate normalization contracts for optional directories and package selectors. Preserve valid explicit `.`, `./`, `./...`, and mixed lists. Represent provided versus unset selection before applying defaults so an explicit selector cannot become an omission accidentally.

Keep current written-scope defaults for truly omitted selection. If backward compatibility requires filtering empty or null-like string sentinels, specify that package-specific set explicitly rather than importing directory semantics. Accept genuine nil only where the public API actually admits it; unsupported types should remain rejected, not silently converted into defaults. Preserve the intended omitted or null-like working-directory recovery.

The Go transport layer should preserve selector meaning and provenance; Mangle policy should continue to own applicable scope defaults and authorization. This needs neither a new tool nor model-generated authority, and should not force full-suite fallback or widen filesystem permissions. Any future sentinel normalization should occur consistently before policy consumes the value.

## Cost and future evaluation

Separate predicates and a provided/unset distinction can operate in time proportional to total argument bytes. The main work is integration and regression coverage, not repository-wide analysis. Preventing accidental recursive runs may save process time, CPU, and output tokens, but no saving has been measured. Preserving an explicit root request may correctly run checks that were previously skipped. No additional model call is inherent in the correction.

Future evaluation should compare the parent revision, the introducing/current behavior, and the proposed correction using identical project scope, toolchain, options, and timeouts. Exercise normal registry and session paths with deterministic tool requests or a deterministic model double, covering native and Piggyback consumers. Treat `[]any` as the ordinary decoded-array case and `[]string` as an additional direct-adapter case.

Capture original arguments, normalized selection, resolved directory, actual argv, `ran`, scope reason, receipts, and final gate outcome separately. Cover missing and empty packages; compatibility sentinels alone and mixed with dot; `.`, `./`, `./...`; mixed root and subpackage selection; root written and unwritten; no Go writes; and a recognized nested module.

A failing root suite with a passing subpackage must not disappear behind an explicit root request. Conversely, an unrelated failing subpackage must not be charged to a root-only request. Keep legitimate omitted-default behavior identical. Check invalid types and traversal rejection before process creation, configured Go flags, and timeout or cancellation without evidence inflation.

Report exact selection fidelity, requested packages omitted, extra packages executed, process count, CPU and wall time, p50/p95 latency, retry tokens, permission preservation, and receipt honesty. No tests, application-model inference, training, scanner, or implementation were run during this audit.

[normalization]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification.go#L87-L111
[package-filter]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification.go#L339-L348
[omission]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/workspace_guard.go#L277-L295
[introduction]: https://github.com/theRebelliousNerd/codenerd/commit/400c2f32958dcf978d478f3b00ad640500fb9008
[parent-normalization]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/tools/shell/verification.go#L87-L111
[go-reference]: https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/cmd/go/alldocs.go#L3095-L3111
[selector-validation]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification.go#L121-L135
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification.go#L112-L119
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification.go#L272-L290
[session-scope]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/session/executor_tools.go#L1037-L1048
[no-work]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification.go#L306-L314
[execution]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification.go#L198-L248
[build-report]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification.go#L179-L196
[receipts]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/test_runs.go#L9-L54
[receipt-consumption]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/session/executor_tools.go#L1055-L1064
[build-tests]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification_test.go#L122-L166
[compatibility-tests]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/verification_test.go#L225-L312
[framework]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/framework.go#L64-L93
[definitions]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/shell/execute.go#L507-L583
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/registry.go#L342-L415
[argument-validation]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/registry.go#L458-L535
[obligation-matching]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/session/workspace_test_plan.go#L159-L170
[omission-tests]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/tools/omitted_optional_path_test.go#L10-L16
[modular-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/ca94da614edff9157acf31bb54f65e4f9695ae92/internal/session/executor_tools.go#L2694-L2718

