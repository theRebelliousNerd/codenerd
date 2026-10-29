# Effective build tags in lint receipts

## Decision and evidence boundary

Report the build tags that the selected tool actually applies, while retaining the configured environment and argv as separate inputs. The new `run_lint` receipt currently combines tag names from `GOFLAGS` and argv. For `go vet`, those inputs have precedence rather than union semantics. A supported workspace configuration can consequently produce a receipt naming a tag that the actual invocation overrides.

This is a source-derived conditional finding at CodeNERD main [`99c5526e69391fafbce678b5fa467d589c4ad2a5`](https://github.com/theRebelliousNerd/codenerd/commit/99c5526e69391fafbce678b5fa467d589c4ad2a5), inspected October 29, 2026. No tests, application execution, model calls, inference, training or benchmarks were performed. No observed missed defect, accepted bad patch or token saving is claimed.

The typed lint tool, build configuration plumbing and model-facing receipt are implemented. Effective-configuration resolution and stronger inclusion evidence are proposals. Keep the new receipt: it addresses a real need to explain which build was checked. Correct its semantics rather than removing the information or requiring the model to reconstruct flag precedence from raw strings.

## The complete production path

`run_lint` supports vet, staticcheck and golangci-lint. Its vet argv begins with `go vet -json` and the selected packages. It obtains an environment through the shared build package; for the `go` binary specifically, it invokes `build.GoInvocation`, which also injects configured `build.go_flags`. After the command returns, it stores the actual argv and `GOFLAGS`, then computes `BuildTags` with `buildTags(goflags, argv)`. [Tool invocation and receipt](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/tools/shell/run_lint.go#L208-L233).

`GoInvocation` loads the workspace's configuration and independently prepares environment and argv. `GetBuildEnv` copies configured environment values into the process environment; `AppendGoFlags` inserts configured flags after the subcommand, before package patterns. The latter avoids injecting a flag already present in the original argv, but that conflict check does not remove or combine a corresponding value from `GOFLAGS`. The ordinary vet template does not already contain `-tags`. [Invocation](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/build/invocation.go#L31-L40), [environment assembly](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/build/env.go#L131-L176), [argv injection](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/build/env.go#L307-L345), [explicit configuration loading](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/build/env.go#L494-L506).

The receipt helper scans `strings.Fields(goflags)` and the argv, accepts both equals and separate-argument forms, splits tag lists on commas or spaces, and deduplicates names into one list. It never clears an earlier tag set when a later occurrence replaces it. Thus its output is a set of names mentioned in inputs, not necessarily the effective custom-tag set. [Helper](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/tools/shell/run_lint.go#L281-L312).

The distinction reaches a consumer. The tool description says `build_tags` names the tags the gate ran with. The corresponding prompt atom tells the model that a tagged file is verified when its tag appears in that list. The atom is optional and requires `run_lint`, so it is not present in every prompt; the tool's JSON receipt still exposes the field. [Tool contract](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/tools/shell/run_lint.go#L113-L135), [prompt guidance](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/prompt/atoms/capability/run_lint.yaml#L1-L25).

## Go applies replacement and precedence

CodeNERD declares Go 1.27.3. The inspected Go source at immutable revision `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` shows the relevant semantics directly: vet applies `SetFromGOFLAGS`, then parses explicit flags; the `tagsFlag.Set` implementation resets its slice before adding the current comma-separated value. The later explicit `-tags` value therefore replaces earlier custom tags. This is not an inference from shell syntax or a claim that Go concatenates all tag inputs. [Declared version](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/go.mod#L1-L3), [vet ordering](https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/vet/vetflag.go#L138-L150), [tag replacement](https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/work/build.go#L379-L392).

Go also parses `GOFLAGS` through its quoted-word parser rather than plain `strings.Fields`. A robust receipt implementation should follow the relevant tool's actual accepted grammar and report unsupported or invalid input honestly. The simple conflicting-value witness below does not depend on quoting edge cases. [GOFLAGS parsing](https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/base/goflags.go#L27-L80).

## A supported configuration is enough

Consider this illustrative workspace configuration, not a file written or run by this audit:

```json
{
  "build": {
    "env_vars": {"GOFLAGS": "-tags=feature_a"},
    "go_flags": ["-tags=feature_b"]
  }
}
```

The model selects `run_lint` with gate `vet` and a permitted package. The process receives the configured environment and an argv equivalent to `go vet -tags=feature_b -json ./pkg`. Under the inspected Go semantics, the effective custom tag set is `feature_b`. The helper reports both `feature_a` and `feature_b`.

For a concrete file-selection consequence, let `pkg` contain ordinary healthy Go source and an additional file gated solely by `//go:build feature_a`. Assume the platform, filenames, toolchain and all other constraints are compatible; neither tag is supplied by another source; and the command completes normally. The additional file is excluded by the actual invocation, yet `feature_a` appears in the receipt. A defect present only in that file need not be examined by this vet run. The run can legitimately be clean for the selected configuration while its receipt overstates which custom tags were active.

This is a configuration-supported route, not arbitrary argv injection through the model's package argument. Package validation and the tool's closed gate vocabulary remain intact. An explicit environment value also avoids reliance on the repository's automatic `sqlite_vec` default. The existing automatic-tag test is a useful positive control, but it checks one environment-derived tag without a conflicting argv tag. [Current receipt test](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/internal/tools/shell/run_lint_test.go).

## Limits and relation to existing audits

This finding is about the receipt's effective-configuration claim. It does not prove that the agent reports final completion, that independent verification accepts the patch, or that a particular deployed configuration uses conflicting values. Raw argv and `GOFLAGS` remain in the result, so the mismatch is diagnosable. A reader applying Go precedence can recover the correct interpretation.

With only one effective tag occurrence, or with identical environment and argv values, the demonstrated union defect does not change the set. With neither source supplying tags, an empty custom-tag list does not mean Go has no platform, compiler, release or other implicit build constraints. A listed custom tag alone also cannot establish that a file was in the selected package scope or satisfy an arbitrary Boolean build expression. These are reasons to narrow the wording, not evidence that every current receipt is wrong.

The proof here is for `go vet`. Staticcheck and golangci-lint may apply their own configuration files, options and package-loading behavior. Their effective tag contracts require independent inspection; do not apply Go argv precedence mechanically to another binary or claim this review established their exact effective configurations.

The earlier [tagged-test constraint audit](https://github.com/theRebelliousNerd/codenerd/blob/99c5526e69391fafbce678b5fa467d589c4ad2a5/Docs/audits/tagged-test-constraint-witness.md) concerns choosing a Boolean-unsatisfying tag assignment and accepting a process that did not execute the intended named test. This refinement concerns a new lint receipt that misstates the tags the tool actually selected, even with a simple single-tag file constraint. The broad lesson about evidence identity is shared; the producer, immediate consumer and repair seam are different. Targeted source and documentation searches found no earlier exact lint precedence witness, which is a bounded review rather than a novelty claim.

## Proposed receipt contract

Separate four questions:

1. What inputs were configured? Preserve the final process argv and relevant environment representation already returned, subject to the existing disclosure policy.
2. What configuration did this tool apply? Resolve effective custom tags with the tool's grammar and precedence, and record the resolver/tool identity and whether the result is known.
3. Which files or packages did that configuration include? When an obligation concerns a particular file, require inclusion evidence for that file under the selected package and build context.
4. What did the successful gate establish? A completed lint run establishes its gate's result over its actual selected domain, not universal correctness or named-test execution.

A minimal repair can correct vet's precedence calculation and change prompt wording to say that tags describe the invocation rather than prove a file was verified. An explicit `unknown` or unsupported-resolution state is safer than an invented union for another tool. Do not change the actual invocation to make it agree with the mistaken receipt: the configured override may be deliberate. Do not silently enable both mutually exclusive feature sets.

For a more complete design, the build package can expose one immutable invocation record containing executable, working directory, finalized argv, filtered environment identity and tool-specific effective configuration. Both execution and rendering consume that record. Distinguish the record's configuration calculation from observed file inclusion; neither a host-generated identifier nor a digest proves that a file participated.

The resolver should process applicable environment flags in order, then explicit argv under the selected tool's rules. It must represent explicit empty overrides, distinguish absent from empty, stop at the relevant flag boundary, and preserve parse failures. Tokenization must match the supported grammar. Canonicalization may sort a final set for comparison, but it must not erase the precedence history before resolving it. Keep raw inputs so unexpected tool behavior remains explainable.

If stronger file-level verification is required, derive inclusion through the same authorized toolchain and build context, or consume trustworthy file-selection evidence already produced by that invocation. Evaluating one `//go:build` expression alone does not account for filename suffixes, package selection and every tool-specific inclusion rule. Additional discovery commands are a proposed evaluation/implementation cost, not actions authorized or performed by this document.

## Failure modes and full costs

The immediate precedence calculation is linear in the total flag-input length and needs no model call. Keeping an invocation record adds small serialization and allocation costs. Richer file-selection evidence can require package discovery, cache access, filesystem I/O or an additional process. Any cache must bind to toolchain, workspace/source applicability and effective configuration; otherwise a correct tag list can still accompany stale inclusion evidence.

Count total costs across configuration loading, parsing, discovery, lint execution, output parsing, receipt storage/rendering, JIT/tool-result tokens, model reasoning, subsequent tool calls and repair retries. Correcting a misleading receipt may cause an additional needed lint run. That is increased assurance with its cost, not automatically a regression or a saving. Conversely, an accurate receipt may prevent redundant reruns, but that benefit remains unmeasured.

Important failure cases include treating a tool configuration file as irrelevant, confusing a default with an explicit empty override, accepting malformed quoting as valid tags, parsing arguments after the package boundary as flags, and reporting custom tags as the complete build context. Do not discard raw failed-command diagnostics just because effective configuration could be resolved. A command that never ran must remain unavailable or failed rather than gaining verification credit from a planned tag set.

## Fair evaluation plan

No evaluations below have been run. At the receipt socket, compare the existing union helper, a vet-specific precedence resolver, and a richer invocation/inclusion contract on the same finalized inputs. The oracle is the selected tool's effective configuration and file selection, not the helper's own output.

Cover environment-only, argv-only and equal-source controls; conflicting sources; several applicable `-tags` occurrences; explicit empty values; supported equals and separate-argument forms; quoted legacy syntax; invalid inputs; and package/flag boundaries. Separate expected behavior for each supported binary instead of pooling all lint tools under one parser. Preserve raw argv and environment fields in every variant.

At the complete-system level, use the actual workspace config loader through `run_lint` to real vet in a separately authorized evaluation environment. A small package can contain ordinary source plus files selected by mutually exclusive custom tags, with a unique diagnostic in only one variant. Compare the receipt against the actual selected files and findings. Include successful, nonzero opaque failure, canceled and unavailable invocations. Keep the existing automatic-tag fixture and package-scope validation as regression controls.

The model-facing evaluation should hold task, model/version, tool budget and code revision constant. Compare the current prompt wording with the corrected wording and receipt. Score false claims about checked files, correct unresolved reporting, verified task outcomes, unnecessary reruns and total task tokens/compute. A lower token count obtained by accepting an excluded file as verified is not an efficiency win. A new parser's microbenchmark cannot establish improved end-to-end coding accuracy.

Acceptance should require that the conflicting configuration reports only the effective custom tag set, or explicitly marks it unresolved, while retaining both raw inputs. File-specific assurance must remain limited to actual inclusion and the gate's scope. The purpose is an accurate evidence contract that lets the agent reason cheaply without asking it to reverse-engineer the invocation each time.
