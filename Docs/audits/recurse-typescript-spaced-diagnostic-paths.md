---
doc-class: deep-dive
subsystem: campaign-recurse
implementation-status: proposed-refinement
last-verified: 2026-11-04
verified-against: 705bb782e828cf77b18a76e1b20df5bca39ca802
verification-method: static-source-inspection
---

# Recurse loses located diagnostics for admitted paths containing spaces

## Finding and boundary

Recurse can admit a TypeScript source file into a subsystem, complete its configured compiler process, retain the compiler's failing result, and nevertheless exclude the diagnostic from that subsystem's repair candidates solely because the filename contains an ordinary space. The TypeScript diagnostic expression requires a whitespace-free pathname. A failed workspace gate whose only diagnostic names `src/widget demo.ts` therefore becomes an unattributed finding, even though the compiler supplied a location and the existing `src` node owns it. Unattributed findings are deliberately reported but never visited. [Parser][parser] [Attribution][attribution] [Selection][selection]

This is a targeted-repair availability and portability defect, not a false-green claim. Keeping unknown failures out of arbitrary nodes is a useful safeguard. The problem is losing a known location before that safeguard receives it. No model behavior, successful repair, incorrect adoption, exploit or incidence rate is established here.

The review pins public CodeNERD main at `705bb782e828cf77b18a76e1b20df5bca39ca802`, rechecked unchanged at approximately 04:20 UTC on 2026-11-04. Root and campaign guidance, architecture and mission documents, relevant source, existing tests, the audit queue and adjacent audits were read. No application, test, compiler, gate, model inference or behavioral reproduction was executed. Only research copies and this scratch document were written outside the repository. All evaluation below is proposed and unrun.

## Why this is a supported producer/consumer mismatch

The workspace collector admits membership-approved files by source suffix; it does not exclude interior spaces in a filename or directory. `isJSSource` recognizes `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.cjs`, `.mts` and `.cts`, while excluding `.d.ts`. `scanJSImports` creates a node for every directory containing those files before resolving imports. An admitted `src/widget demo.ts` therefore contributes to the `src` JS/TS node without needing an import edge. [Collector][collector] [JS graph][js-graph]

Detected package build, test and lint scripts are workspace-scoped. A project with `package.json` and `tsconfig.json` but no build script also receives a workspace-scoped `npx tsc --noEmit` gate. An explicit `nerd.md` build command is likewise `ScopeAll`; its first program may be `tsc`, which is recognized as JS/TS. Thus a normal supported entry supplies `Result.Node == ""` and depends on the emitted diagnostic pathname for work ownership. [Declared gates][declared] [Detected JS gates][detected] [Runner][runner]

TypeScript's own non-pretty reporter calls `formatDiagnostic`. In the pinned TypeScript 5.9.3 implementation, that formatter converts the source filename to a relative filename and directly concatenates it with `(line,column): ` and the diagnostic category/code/message. It does not tokenize or quote interior spaces. This supplies an upstream production basis for the witness, rather than inventing an unsupported log syntax. TypeScript's `files` configuration supports an explicit source-file allowlist; `pretty` controls the decorated presentation. [TypeScript reporter][ts-reporter] [TypeScript formatter][ts-formatter] [Files option][ts-files] [Pretty option][ts-pretty]

The CodeNERD parser accepts a narrower pathname language:

- `tscError` starts with `^(\S+?\.[A-Za-z]+)`, so any interior space prevents a match.
- The generic positioned-diagnostic parser also excludes whitespace, and in any case expects colon-delimited positions rather than TypeScript's parentheses.
- The Rust arrow parser and pytest summary parser also use whitespace-free captures. Their broader tool-specific consequences are not established by this TypeScript witness and should be qualified separately rather than assumed identical. [Parser expressions][parser]

The boundary is not a shell quoting failure. The compiler obtains the filename through its project configuration; no pathname containing spaces needs to be split into command arguments in this witness.

## Conditional production witness

Use an ordinary repository whose source membership admits `src/widget demo.ts`, with no forbidden-path rule covering it. Pin a locally available compatible TypeScript toolchain, configure the project to include that file, and declare the build command `tsc --noEmit --pretty false`. Installation, network access or credential changes are not part of the witness. Assume the source contains one genuine type error and the completed compiler result is nonzero, untruncated and contains this formatter-compatible diagnostic:

```text
src/widget demo.ts(1,7): error TS2322: Type 'string' is not assignable to type 'number'.
```

This line is a stipulated conditional input consistent with the inspected formatter, not captured output from a run performed for this report. The exact error wording is immaterial provided the normal `error TS<number>: message` suffix remains.

1. The normal graph construction includes the JS/TS `src` node. The workspace build result has no node-specific fallback owner. [Collector][collector] [JS graph][js-graph] [Runner][runner]
2. `FindingsAll` visits the line. The Go/generic stream consumer does not recognize it, and `tscError` rejects the space in `widget demo.ts`. No other listed parser recognizes this line. [Stream consumer][stream] [Extraction][extraction]
3. With no parsed findings, extraction calls `add(nodeOr(r.Node), lastLine(...), false)`. The empty workspace node becomes `<unattributed>`. The error message can remain visible, but the structured target is no longer the file. [Fallback][fallback]
4. `measure` passes that target to `attributeNodeInRoot`, which correctly returns the special unattributed node. That special ID is not a DAG node. [Measurement][measurement] [Attribution][attribution]
5. The `src` visit receives only workspace findings whose `Node` equals its own ID. This failure is absent from `open(src)`, so it is not submitted to the policy as a repair candidate for `src`. `reportUnattributed` surfaces it separately. [Selection][selection] [Reporting][reporting]

The discriminating control changes only the source filename and matching configuration to `src/widget_demo.ts`, leaving the error, gate, scope and output size otherwise unchanged. Its formatter-compatible line matches the TypeScript expression, and its target reaches the existing `src` prefix ownership rule. This contrast isolates diagnostic pathname decoding, not graph discovery, invocation identity, byte truncation or the 50-finding work limit.

For a minimal build-only project with no applicable test gate, the independent improvement path does not provide a compensating attempt: it skips improvement when no test gate can verify the node. In a larger project, independently selected improvement or another gate can still cause work on the same source. Consequently the general claim is loss of this diagnostic's intended repair route, not that every possible route to editing the file is impossible. [Improvement countercontrol][improvement]

## Mixed output is a separate observable symptom

If the same completed workspace build contains both a recognized diagnostic for `src/plain.ts` and the spaced-path diagnostic, `FindingsAll` retains the first and does not add an unattributed fallback, because fallback is conditional on the entire extracted slice being empty. The second diagnostic is absent from both the repair list and the uncapped identity set supplied by this invocation. Its bytes remain in `Result.Output`. [Extraction][extraction] [Measurement][measurement]

This is not the old 50-entry truncation defect: two short diagnostic lines suffice. It also does not by itself prove an incorrect keep. The process still fails, the selected attempt has an independent acceptance command, and other gates and ratchet conditions remain active. The existing census audit already identifies parser completeness as a remaining general qualification. This report contributes a concrete, supported-output producer and a narrower work-selection failure, rather than claiming that qualification is newly discovered. [Prior census audit][prior-census]

## Contract and bounded refinement

The useful contract is: **for a supported diagnostic format, a complete located diagnostic naming an admitted source path must preserve that path through decoding into ownership and work selection, including ordinary interior spaces.** If decoding is unsupported or ambiguous, preserve a truthful unresolved disposition and the raw evidence; do not guess that the failure belongs to the root or cross-cutting node.

A bounded correction can remain in the host decoder. Recognize TypeScript's location-and-code suffix as structure and preserve the preceding pathname, instead of treating a pathname as a whitespace-delimited token. Normalize the captured path once through the existing relative-path logic, then retain the existing node ownership and scope controls. This does not require asking the model to interpret logs, adding a new planning subsystem, changing failed exits to passes, broadening edit permission or running every workspace command per node.

Do not apply an indiscriminate greedy-expression change across every tool. Test logs, stack frames, Windows drive colons, filenames containing parentheses, embedded delimiter-like text, ANSI decoration and compiler versions need format-specific treatment. `FindingsAll` invokes the Go/generic stream consumer first, so precedence must remain explicit. Truly location-free errors must remain unowned. Full handling of arbitrary newline-containing filenames or every tool's escaping convention is outside this witness and must not be claimed from an interior-space fix. [Stream consumer][stream] [Normalization][normalization]

Parser-completeness metadata and durable uncertainty are valuable broader work already proposed by the census audit. They should be coordinated with this correction, not repackaged as a new architecture. A narrow decoder correction can restore this known location even before that broader contract is implemented. Conversely, marking a result incomplete without preserving a plainly recoverable location would leave the demonstrated repair gap in place.

## Costs and failure modes

- Parsing still reads the captured output. A bounded delimiter-aware decoder can be linear in input size; neither a speedup nor a particular implementation complexity has been measured.
- Correct extraction creates finding records that the current decoder omits. This can increase real repair work, retained identities and model turns. That is recovered work, not a free performance improvement; measure the added CPU, memory and downstream attempts.
- Broader matches can turn stack frames or explanatory lines into false work. Preserve the existing distinction between diagnostics and indented logs, and require a complete supported suffix rather than merely seeing `.ts` in a line.
- An unattributed fallback ID and a newly recognized file-specific ID are different. Any persisted or cross-pass history must not interpret a parser-version change as proof the old failure was repaired. Establish fresh comparable measurements or explicit migration semantics.
- A fallback for every unmatched line would manufacture findings from ordinary build chatter. The mixed-output symptom needs honest coverage accounting, not indiscriminate line-to-finding conversion.
- Outside-workspace targets, forbidden paths, unrelated nodes and uncertain owner matches must retain their existing conservative behavior. Recovering text is not authorization to edit that location.

## Nonduplication screen

The current audit queue and available recurse/gate reports were screened for diagnostic filename spaces, whitespace-restricted paths, `tscError` and `fileLineMsg`. This is a bounded search, not an assertion that all historical conversation was examined.

- The root-crate diagnostic-ownership audit concerns a successfully parsed Rust location that could not map to its root crate. Current source includes the root Rust ownership refinement. Here the location is lost **before** ownership receives it, and a normal non-root JS/TS directory already supplies the correct owner. Changing root ownership cannot repair this witness. [Prior root-crate audit][prior-root]
- The finding-census audit's original cap witness has been addressed; its addendum explicitly leaves parser completeness qualified. This report does not claim a novel completeness principle. It establishes the current admitted-path mismatch and resulting repair omission with one TypeScript diagnostic. [Prior census audit][prior-census]
- The legacy-build diagnostic-evidence audit already recommends an explicit pathname grammar, whitespace handling and spaced-path fixtures. Credit that guidance: preserving whitespace is not a new parser design principle. Its concrete producer is `parseBuildDiagnostics` in the legacy VirtualStore `build_project` route, emitting `diagnostic/5` facts from Go-style colon-delimited output; its witnesses concern drive-colon loss and spurious timestamp records. This report instead establishes the distinct `tscError` → `FindingsAll` → workspace ownership → recurse candidate route, using TypeScript's parenthesized location format. Repairing the legacy helper alone does not change this decoder or restore the missing recurse candidate. Shared lexical principles and fixtures should be coordinated where formats agree, without pretending those routes or fact contracts are interchangeable. [Prior legacy-build audit][prior-legacy]
- The mission acceptance-scope audit concerns a selected target whose copied workspace acceptance command includes unrelated baseline failures. Here no target attempt is selected for the affected file at all. The witness does not need mission mode, sibling failures or an acceptance workaround. [Prior mission-scope audit][prior-mission]

## Fair evaluation plan: unrun

First compare recorded, version-pinned compiler outputs through the existing extraction, attribution and work-selection boundaries. Then, if separately authorized, exercise a normal recurse run with a locally available compiler and inspect the selected attempt. A constructed parser result alone is not evidence that full campaign repair succeeds.

| Case | Required observation |
|---|---|
| Plain filename versus one interior space, identical error | Both retain their real source target and enter the same owning subsystem's candidate set. |
| Space in a parent directory instead of the basename | Ownership matches the actual admitted directory, not a truncated word. |
| One spaced-path diagnostic alone | No invented unattributed owner when the supported location is recoverable. |
| Spaced and plain diagnostics together, both orders | Both locations survive; neither ordering nor the presence of a recognized sibling silently removes the other. |
| Windows separators/drive and native POSIX paths | Path normalization preserves identity on the relevant platform. Cross-platform textual fixtures are not substitutes for native-path behavior. |
| Parentheses or delimiter-like text in a filename | Decoder uses the actual location suffix or explicitly declines ambiguity; no incorrect target. |
| Indented test logs, stack frames and ordinary chatter | No new false diagnostics from broad matching. |
| Genuine location-free failure or unsupported pretty format | Failure remains visible and honestly unresolved; no arbitrary assignment to `src`, root or wiring. |
| Failed, passed, unverified and truncated result controls | Existing exit/status and truncation behavior remains intact. |
| Forbidden/outside-workspace source and unknown owner | Recovery of a textual path does not expand scope or bypass admission. |
| Decoder change across persisted/cross-pass history | No false resolution or regression label caused solely by changing diagnostic identity. |

Keep source, tool version, command, scope, output budget and error content equal between baseline and proposed variants. Use independently recorded compiler output as the format reference; do not use the new parser to decide its own expected answer. Report target-preservation rate, false/unattributed assignments, actual policy candidate selection, extra attempts, CPU/wall time, memory and stored bytes. Separate successful location recovery from actual repair success and retain failed cases in the denominator.

Completion means supported located failures for ordinary spaced paths reach the same constrained repair route as their no-space controls, while unknown locations remain conservative. It does not mean every diagnostic format is now complete or every type error can be repaired.

## Pinned sources

[parser]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/gates/findings.go#L64-L91
[extraction]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/gates/findings.go#L112-L184
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/gates/findings.go#L179-L202
[normalization]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/gates/findings.go#L205-L237
[stream]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/gates/findings.go#L340-L382
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/campaign/recurse_workspace.go#L269-L342
[js-graph]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/campaign/recurse_workspace.go#L550-L573
[declared]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/gates/detect.go#L51-L118
[detected]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/gates/detect.go#L175-L213
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/gates/run.go#L15-L81
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/campaign/recurse_cycle.go#L700-L817
[attribution]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/campaign/recurse_cycle.go#L851-L915
[reporting]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/campaign/recurse_cycle.go#L938-L981
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/campaign/recurse_cycle.go#L993-L1106
[improvement]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/internal/campaign/recurse_cycle.go#L1184-L1223
[ts-formatter]: https://github.com/microsoft/TypeScript/blob/c63de15a992d37f0d6cec03ac7631872838602cb/src/compiler/program.ts#L666-L677
[ts-reporter]: https://github.com/microsoft/TypeScript/blob/c63de15a992d37f0d6cec03ac7631872838602cb/src/compiler/watch.ts#L124-L139
[ts-files]: https://www.typescriptlang.org/tsconfig/files.html
[ts-pretty]: https://www.typescriptlang.org/tsconfig/pretty.html
[prior-root]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/Docs/audits/recurse-root-crate-diagnostic-ownership.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/Docs/audits/recurse-finding-census-completeness.md
[prior-legacy]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/Docs/audits/legacy-build-project-diagnostic-evidence.md
[prior-mission]: https://github.com/theRebelliousNerd/codenerd/blob/705bb782e828cf77b18a76e1b20df5bca39ca802/Docs/audits/recurse-mission-acceptance-scope.md
