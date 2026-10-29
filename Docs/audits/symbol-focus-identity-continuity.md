---
doc-class: deep-dive
subsystem: context
implementation-status: proposed
last-verified: 2026-10-29
verified-against: 9dcb01b9736bf4763aa3ac391d29f133d73c6611
verification-method: static-source-inspection
supersedes: []
---

# Symbol focus identity continuity

## Conclusion

The Go scanner produces namespaced symbol identifiers, but the perception firewall's bare-target lookup compares human text directly with those identifiers. A unique, current function named `ComputeTotal` can therefore remain unresolved when the correctly extracted target is `ComputeTotal`, even though the indexed row is present as `func:ComputeTotal`.

The immediate source-derived result is a low-confidence focus fact with no resolved file or symbol. That loses the canonical file seed used by focus-driven context rules. Under additional fact-visibility and budget conditions, the compressed context can retain the raw target's intent facts while omitting the function's symbol facts from this selection path. Current shard ownership means a resolver-only fix is not yet proven to change actual world-row rendering.

This is a bounded identity-continuity finding, not a measured incident, established regression, authorization bypass, or prediction of a wrong edit. The proposal preserves the distinction between human names, symbol IDs, and paths, then resolves them explicitly before compiling focus facts.

## Producer and consumer contracts

The production Go scanner uses TreeSitter with canonical fact paths. Its function producer constructs an ID as `func:` plus the function name. The resulting `symbol_graph` row carries that ID, kind, visibility, path, and signature. The schema calls its first argument `SymbolID`, not a human display name. Boot refresh and TUI scans feed the world model through the scanner and apply path. [Go scanner entry][go-scan] [Function facts][function-facts] [World schema][world-schema] [Boot application][boot-scan] [TUI scanning][tui-scan]

The perception resolver initializes an unresolved result and treats path-like targets separately. For a bare target, it queries `symbol_graph`, examines at most 2,000 rows, and compares argument zero with the target using case-insensitive equality. On a match it reads the canonical path from argument three and returns confidence 85. It does not translate a human function name into the scanner's namespaced ID. [Resolver][resolver]

If symbol lookup misses, a bounded filename search checks whether a path contains the target text. If that also misses, the resolver returns confidence 30 with empty resolved-path and symbol fields. The firewall asserts the resulting focus fact. Its serialized fields are raw reference, resolved path, symbol name, and integer confidence. [Focus assertion][focus-assertion] [Focus fact representation][focus-fact]

Thus the issue is not absence of an indexed symbol or failed natural-language extraction in the witness. The producer and consumer use different identity forms at an otherwise connected boundary.

## Conditional witness

Use a normal, non-test Go file `charge.go` containing a single uniquely named `ComputeTotal` function. Require a fresh classification through a running perception firewall, rather than a reused intent or the direct-transducer fallback, and:

- A successful current scan, an included workspace file, and source below the scanner's AST-size limit.
- A correctly recognized target exactly equal to `ComputeTotal`, with no explicit `Scope.File`, punctuation, or vague-target classification.
- The relevant symbol row within the resolver's first 2,000 rows.
- No filename containing `computetotal`, so filename fallback cannot accidentally resolve the request.
- No duplicate function or method name requiring disambiguation.

The real producer supplies `func:ComputeTotal`, not a hand-written bare-ID fixture. The equality check does not match `ComputeTotal`; the filename search does not match `charge.go`. The expected focus result is the original raw reference, empty resolved file and symbol, and confidence 30. No execution was performed to observe that result. [Producer identity][function-facts] [Lookup and fallback][resolver]

Two useful positive controls isolate the boundary. An exact target `func:ComputeTotal` can match the indexed row. An explicit target `charge.go`, or an empty target with `Scope.File` set to `charge.go`, can identify the file without bare-name resolution. Supplying that file alongside a nonempty `ComputeTotal` target does not itself replace the target. These controls do not establish that all qualified names or file-like strings resolve correctly. [Scope adaptation][scope-adaptation]

## What the context rules lose

The normal TUI processing path invokes perception, whose focus-resolution step asserts the fact consumed by policy. Focus-derived context relevance uses the resolved file at priority 95; direct dependencies receive priority 70, and bounded graph traversal begins with that file as a hop-zero seed. [TUI perception entry][tui-perception] [Firewall perception][firewall-entry] [Focus relevance][focus-rule] [Dependency relevance][dependency-rule] [Traversal seed][hop-rule]

For the witness, the focus rule names an empty file key instead of `charge.go`. The dependency join cannot use the real file's links from that empty key. This does not remove every dependency fact or every possible seed: it removes the canonical file and its dependency expansion **from this focus path**.

Separately, the intent rule assigns priority 100 to the raw target. Correctly preserving the user's text is useful, but `ComputeTotal` is not the canonical file key or the indexed `func:ComputeTotal` symbol ID. A higher priority cannot repair an identity mismatch. [Intent priority][intent-rule]

The firewall increments an internal clarification counter for low-confidence resolution. That is not evidence that the serving interaction automatically asks a clarifying question. Likewise, this finding does not rely on a `FocusResolutions` field being rendered directly to the model.

## Serving consumer and materialization limit

The compressor actually queries `should_include_context`. Its entity index contains nonempty string arguments from facts it holds, and resolution uses exact fact strings or exact argument values. A signature containing `ComputeTotal` is not an exact bare-name argument match, nor are `func:ComputeTotal` and `charge.go`. [Selection consumer][compressor] [Entity index and lookup][entity-lookup]

In a fixture where the compressor holds the relevant world facts **and** the current raw-target intent or focus fact, the priority-100 bare key can resolve to those latter facts. With enough budget, that creates a nonempty selected set. Go-side activation fallback runs when the kernel-derived selected set is empty, not whenever the chosen facts fail to include the intended symbol. A nonempty raw-target selection can therefore prevent that fallback from compensating for the missing file seed. This is a conditional selection consequence, not a demonstrated current-TUI token delta. [Fallback condition][compressor]

Current boot comments identify the compressor's source as the catch-all primary kernel, while shard registration assigns `symbol_graph`, `file_topology`, and `dependency_link` to the world shard. Cortex-backed decisions and primary-kernel fact storage therefore must not be treated as one fact universe. The independently supported finding is the missing canonical focus and dependency seeds. Fixing the resolver alone is not proven to make world rows render; projection and materialization parity must also be established. [Compressor storage boundary][storage-boundary] [World ownership][world-ownership]

When compression is active, the TUI obtains compressed context and passes it into articulation, whose helper renders the compressed block. This supplies an actual serving consumer rather than an inference from an unused data field. It still does not prove that every model request lacks the symbol: other prompt sections and retrieval paths remain possible. [Compressed context route][compression-route] [Articulation rendering][rendering]

## Counterevidence and bounded impact

Other rules can rescue the same material. Modified-file facts, failing-test seeds, `context_atom`, and independent world facts can select the file or symbol. When fallback does run, Go activation uses substring-based target relevance, including a target-match boost. Tools, CodeDOM observations, and JIT context can also obtain the code later. [Other context seeds][other-seeds] [Go activation alternative][activation]

The finding is therefore not total context starvation. Core retained facts, conversation history, intent text, and other selected context can remain. A separate workspace-evidence route can use `Intent.ScopeFile` directly even when focus lookup misses; delegation still formats the task from the original `Intent.Target`, rather than substituting the focus result. Nor is a focus result an authority grant: path permissions, tool guards, and mutation preconditions remain independently necessary. [Evidence target selection][evidence-targets] [Delegation target][delegation-target]

The current resolver's scan bound is intentional. A row outside the first 2,000 is a different bounded-coverage case; the witness places the real row inside that range. Duplicate names and methods also need separate treatment: replacing equality with an unqualified first-match suffix search could make resolution less safe, not more accurate.

A source-only screen of more than 90 current audit documents found no exact finding at this live perception boundary. The world-holographic-continuation audit is a genuine predecessor: it includes a bare-name versus receiver-qualified caller-ID repair in a body helper. The distinct consumer here is `PerceptionFirewall.resolveTarget` reading fast-scanner `SymbolID` facts, before edit or impact ranking. Context-ledger work also discusses whole-kernel versus whole-fallback selection; this proposal does not reopen that mechanism. It addresses the identity of a focus entity supplied to it, not a repository-wide absence of identity repairs. [World predecessor][world-predecessor] [Selection predecessor][selection-predecessor]

## Proposed typed resolution

Keep three concepts separate: canonical symbol identity, human display or qualified name, and canonical workspace path. The scanner's namespaced ID should not silently double as all three.

Provide a bounded, generation-aware alias index from human names to current symbol candidates. A unique exact alias can resolve to its canonical symbol and path; ambiguous or unavailable cases should retain explicit uncertainty and candidate identities rather than choose the first row. The existing `func:Name` string is not globally unique across files or packages. Candidate identity must include canonical file, language, symbol kind, receiver where applicable, and source generation. Use language-aware case rules; case folding must not silently merge distinct Go identifiers.

Reuse current canonical paths and bind results to the world-model generation. Renames, deletions, and rescans must invalidate stale aliases. A failed, canceled, or partial index refresh must not publish a new complete generation; query errors and incomplete coverage must remain distinguishable from a complete search with no match. Unknown facts should not become invented paths, and low-confidence resolution must not create permission or mutate the user's requested target.

Emit the canonical focus file and symbol separately from the raw reference. Keep context inclusion policy responsible for priorities and traversal, while host code supplies measured identities and provenance. Do not compensate by indiscriminately raising priority, broadening every query, or asking a model to confer authoritative identity.

## Cost and future evaluation

An alias index costs time and memory proportional to indexed symbols on construction or refresh. Incremental maintenance may reduce repeated work, but adds invalidation complexity. The current lookup is bounded yet still queries and scans candidates; compare actual query and scan costs rather than assuming an index is free. No additional model call is inherent in deterministic identity translation.

Future evaluation should run the complete producer-to-firewall-to-kernel-to-compressor-to-request chain. Generate facts through the real Go scanner, not synthetic `symbol_graph` rows with bare IDs. Establish real shard ownership and projection first; if world facts are absent from the compressor's universe, report that materialization limit separately rather than assuming a rendered-symbol improvement. Hold world generation, fact visibility, budget, and alternate relevance seeds constant.

For the same bare-target input, compare the current bounded lookup, a typed generation-aware alias resolver, and an explicit unresolved/candidate-disclosure fallback. Exact namespaced IDs and explicit file scopes are positive controls, not fair substitutes for the difficult input. Include duplicate functions across files or packages, distinct method receivers, names differing only by case, ambiguous qualified names, punctuation, vague targets, missing symbols, stale generations, renamed or deleted files, scan caps, query failures, cancellation, and insufficient budgets. Verify that ambiguity yields candidates or uncertainty, not an arbitrary file.

Capture producer rows, extracted target, focus fact, derived priorities, entity matches, selection reason, selected facts, and the actual serialized request. In the minimal fixture, confirm whether canonical file and symbol facts differ between treatments when those facts are held and budget permits. Then enable modified, failure, and activation seeds to measure real rescue behavior.

Report resolution precision and ambiguity handling, missing or extra context facts, cold and warm query/index time, refresh work, memory, serialized token count, tool follow-ups, and downstream task correctness separately. Count complete model input/output tokens, retries, source reads, wall latency, and human repair, not only the first context block. An alias index may save repeated lookup work yet spend more memory or introduce stale-identity mistakes; broader context can increase tokens or distract from the task. Keep model, permissions, source snapshot, and total work budgets fixed in any later end-to-end comparison. Do not use wrong-edit anecdotes or final completion alone to establish this boundary. All evaluation is proposed; no tests, application-model inference, training, scanner execution, or implementation were run during this audit.

[go-scan]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/world/incremental_scan.go#L487-L500
[function-facts]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/world/ast_treesitter.go#L135-L158
[world-schema]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/schemas_world.mg#L60-L65
[boot-scan]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L1470-L1491
[tui-scan]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/helpers_scan.go#L159-L176
[resolver]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/perception.go#L746-L814
[focus-assertion]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/perception.go#L681-L691
[focus-fact]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/transducer.go#L344-L364
[tui-perception]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L190-L207
[firewall-entry]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/perception.go#L483-L513
[focus-rule]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/context_compilation.mg#L25-L26
[dependency-rule]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/context_compilation.mg#L62-L63
[hop-rule]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/context_compilation.mg#L107-L109
[intent-rule]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/context_compilation.mg#L21-L22
[compressor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/context/compressor.go#L724-L767
[entity-lookup]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/context/compressor_metrics.go#L556-L654
[compression-route]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L927-L945
[rendering]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/helpers_articulation.go#L237-L241
[other-seeds]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/context_compilation.mg#L28-L68
[activation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/context/activation_scoring.go#L102-L221
[scope-adaptation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/understanding_adapter.go#L505-L510
[storage-boundary]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session_shared_boot.go#L142-L145
[world-ownership]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/registration.go#L55-L64
[evidence-targets]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/turn_evidence.go#L111-L129
[delegation-target]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L528-L535
[world-predecessor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/world-holographic-continuation.md
[selection-predecessor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/context-ledger-prefix-costs.md#L23

