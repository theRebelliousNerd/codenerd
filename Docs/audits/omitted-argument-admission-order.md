---
doc-class: deep-dive
subsystem: tools
implementation-status: proposed
last-verified: 2026-10-29
verified-against: 9dcb01b9736bf4763aa3ac391d29f133d73c6611
verification-method: static-source-inspection
supersedes: []
---

# Omitted argument admission order

## Conclusion

CodeNERD's new registry compatibility rule removes recognized omission sentinels from optional typed arguments before schema validation. The guarded session path can reject those same raw arguments before they reach the registry. A call accepted by the new direct-registry contract can therefore remain unavailable through normal session dispatch.

The concrete witness is an otherwise valid `delete_element` request with optional array argument `paths: "null"`, no replacement, and complete evidence that the selected element has no remaining uses. The registry would omit the sentinel; an earlier session target parser instead rejects its string type.

This is a scoped compatibility and availability gap, not an authorization bypass or an observed mutation failure. The new normalization arrived in `7ae95a092613b2c63ed69669fe2f51c0e8fbf8d5` and remains in inspected main `9dcb01b9736bf4763aa3ac391d29f133d73c6611`. The proposed correction is one schema-aware argument view shared by admission, execution, and accounting. [Introducing change][introduction] [Registry normalization][registry]

## The compatibility contract

`withoutOmittedOptionals` drops optional nil values. For optional fields declared with non-string types, it also drops recognized string sentinels. The sentinel vocabulary includes empty text, `null`, `none`, `nil`, `<nil>`, `undefined`, and `n/a`, after trimming and case folding. Required arguments and declared string values are preserved. The helper copies the map only when it drops a value. [Normalization rules][normalization]

This is explicit application compatibility policy, not a requirement of JSON Schema. The official reference distinguishes null from absence: an optional property may be omitted, but a present value must satisfy its declared type. A string sentinel is not natively an array. [Null semantics](https://json-schema.org/understanding-json-schema/reference/null) [Optional properties](https://json-schema.org/understanding-json-schema/reference/object#required)

The registry now applies that compatibility step before validation, followed by the allowlist, write guard, and handler. Its lower write guard consequently sees normalized arguments. The mismatch arises because earlier serving-layer guards still inspect the original map. [Registry ordering][registry] [Registry write protection][lower-guard]

## Conditional witness

Use a schematic request:

```json
{
  "path": "example.go",
  "ref": "<valid reference to an unused unexported function>",
  "paths": "null"
}
```

The reference placeholder denotes a real, valid reference in the hypothetical fixture, not a literal input asserted to work. Assume:

- The target is inside the permitted workspace, with effective tool authorization, constitutional safety checks, write protection, contextual permissions, and Dreamer restrictions all satisfied.
- The target is not a protected header. Its revision is current; any supplied revision matches.
- Complete uses evidence is available and empty. `replace_with` is omitted.
- No separate preflight, semantic, or environmental condition prevents the operation.

`delete_element` requires `path` and `ref`; `paths` is an optional array used with replacement. The implementation validates the target and revision, rejects headers, requires complete uses evidence, and distinguishes deletion with no uses from replacement across real use paths. It rechecks uses at commit. Omitting `paths` in the empty-uses case is therefore legitimate; the witness does not evade a replacement or uses requirement. [Tool schema][delete-schema] [Semantic safeguards][delete-handler]

With direct registry admission, `"null"` in this optional non-string field is removed before validation. That only establishes compatibility at this boundary, not guaranteed deletion: every subsequent guard and semantic condition still applies.

Through the session path, the original string reaches target extraction first. That parser accepts arrays, not strings, and returns `paths must be an array of strings`. The call is rejected before registry normalization can interpret the value as omitted. [Target extraction][target-parser]

## Serving trace

`executeAndRecordToolCall` calls `guardWrite` using the raw input before `executeToolCall`. A contextual write guard, when installed, derives its paths through `TargetPaths`; without that guard, this particular early check returns without parsing. It is not present in every turn. [Initial guard ordering][initial-order] [Conditional contextual guard][context-guard]

Conditioned on earlier allowlist and constitutional admission, the ordinary serving path independently runs `runWriteGuards`. For this write tool, `projectForbidsWrite` calls `TargetPaths`, and a parsing error becomes a denial. `TargetPaths` first collects the singular path and then parses non-nil `paths`; it does not reach a successful target decision for the sentinel string. The resulting message can describe invalid write targets under `nerd.md`, but this is not evidence that the real `example.go` target was protected by project instructions. [Write classification][write-classification] [Ordinary guards][ordinary-guards] [Project target check][project-check] [Parser contract][target-parser]

The build-exclusion project check explicitly exempts `delete_element`, so it is not the explanation for this witness. The later modular dispatch reaches the registry only after the earlier write guards. Normalization there cannot repair an already rejected call. [Build-exclusion behavior][build-exclusion] [Late registry dispatch][late-dispatch]

The original call input is also used in surrounding execution accounting. Merely adding a late handler workaround would leave multiple interpretations of one call. The session guidance instead calls for an exact payload and a coherent execution contract, while tool guidance preserves guards, effects, and containment. [Session contract][session-guidance] [Tool contract][tool-guidance]

## Counterevidence and limits

Omitted `paths` already works subject to the remaining checks. Native JSON null is also ignored by `TargetPaths` because that optional value is nil. The distinctive case is a recognized **string** sentinel in an optional declared array, not every missing value.

The new registry tests show the intended boundary behavior with a synthetic read-only typed probe: optional nil and sentinel values are normalized, declared string values remain strings, and required typed nil is rejected before guards or execution. Those tests were inspected, not run. They establish neither serving-path admission nor authorization for a mutation. [Optional boundary tests][optional-tests] [Required boundary tests][required-tests]

Unsupported non-sentinel strings should still fail. Required arrays must not disappear, declared string `"None"` must retain its meaning, and a real array containing `"null"` must not be mistaken for an omitted field. Array members may identify real paths. The proposed compatibility fix must preserve these distinctions.

A model could retry with `paths` omitted. That limits the availability impact but can consume retries and does not make the two admission paths equivalent. No actual retry cost, failed task, or successful deletion was measured.

## Relationship to earlier findings

The numeric argument-coercion audit concerns decoded numeric types, not recognized omission sentinels arriving after an earlier guard. The edit-observation precondition audit concerns continuity across batch edits. The package-selector audit concerns preserving meaningful dot selectors rather than deciding when an explicitly recognized sentinel becomes absent. These are neighboring boundary problems, not evidence that this exact serving-order issue is already repaired. [Numeric coercion scope][numeric-predecessor] [Edit precondition scope][edit-predecessor] [Package selector scope][selector-predecessor]

## Design options

The preferred option is one schema-aware normalized, frozen argument view before any argument-sensitive target guard, preimage capture, authorization preflight, timeout decision, handler, postvalidation, or accounting. Preserve the original payload separately, with a digest or equivalent provenance and an explicit normalization event. Bind the view to the tool schema identity used to interpret it. Validate the effective typed arguments before deriving targets. Tool-name checks that do not inspect argument meaning can remain earlier.

For the witness, normalization removes only the optional `paths` sentinel; `example.go` remains the actual target and must pass every applicable path guard. The proposal changes when the adopted compatibility interpretation is established, not which real targets are authorized. A raw-payload record should explain the transformation without allowing the discarded sentinel to replace or hide the effective target set.

Every stage should consume the same effective arguments, not normalize independently against mutable schemas or maps. Required fields, declared strings, and actual array members remain unchanged. Apply raw input size and resource limits before normalization so compatibility cannot conceal an oversized payload.

This does not mean weakening `TargetPaths`. Teaching a generic target parser to ignore every malformed string could hide required multi-file targets. Nor should any guard be removed merely because a lower registry guard exists. Preserve the order and effect of all authorization checks over the shared view.

An alternative is uniform strict rejection: remove the new sentinel compatibility promise and reject the same malformed typed value at every entry point. That is a compatibility rollback, not equivalent implementation of the current rule. Output guidance asking the model to omit fields is useful but leaves correctness dependent on retries.

No option should replay a successful mutation merely to obtain cleaner evidence or accounting. Establish the argument contract before dispatch, then record the one operation that actually occurred.

## Cost and future evaluation

The current helper scans fields and checks required-field membership, giving work on the order of fields times required fields, with a map copy only on change. A shared view adds schema binding, provenance, and bounded storage; freezing nested values may add copying work. Measure these costs rather than assuming they are negligible. No additional model call is inherently required.

Future evaluation should compare direct registry admission, the current guarded session path, shared early normalization, and uniform strict rejection. Use the real `executeAndRecordToolCall` route and registry with an authorized empty-uses fixture, covering native and Piggyback dispatch. Deterministic requests or a deterministic model double can isolate transport behavior; all execution remains future work.

Test omitted fields, native null, each recognized sentinel, casing and whitespace, unsupported strings, required arrays, declared string `"None"`, and arrays containing `"null"`. Compare the normalized value and target set seen at every stage.

Negative controls must preserve denials for protected and outside-workspace paths, contextual guard rejection, stale revisions, incomplete uses evidence, and nonempty uses without a valid replacement. Replacement cases must still require the complete real path set. Check schema changes, raw size limits, cancellation, and postvalidation without allowing mutation before all admission conditions pass.

Record admission decisions and reasons, raw and effective argument digests, schema identity, target sets, handler entry, mutation count, semantic outcomes, and accounting. Measure normalization time and memory, end-to-end latency, retry rounds, and model tokens. Success means consistent compatibility **and** unchanged safety decisions, not merely fewer rejected calls. No tests, application-model inference, training, scanner, or implementation were run during this source audit.

[introduction]: https://github.com/theRebelliousNerd/codenerd/commit/7ae95a092613b2c63ed69669fe2f51c0e8fbf8d5
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/registry.go#L345-L423
[normalization]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/omitted_args.go#L5-L58
[lower-guard]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/virtual_store_write_guard.go#L28-L98
[delete-schema]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/codedom/element_edit.go#L129-L147
[delete-handler]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/codedom/element_edit.go#L556-L594
[target-parser]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/projectdoc/tool_gate.go#L37-L108
[initial-order]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor_tools.go#L1017-L1049
[context-guard]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/write_guard.go#L211-L241
[write-classification]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/projectdoc/tool_gate.go#L190-L202
[ordinary-guards]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor_tools.go#L2580-L2593
[project-check]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor_tools.go#L1621-L1637
[build-exclusion]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion_project.go#L38-L41
[late-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor_tools.go#L2698-L2712
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/agents.md#L3-L26
[tool-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/agents.md#L3-L14
[optional-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/registry_boundary_test.go#L153-L195
[required-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/registry_boundary_test.go#L198-L243
[numeric-predecessor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/tool-arg-coercion.md#L1-L15
[edit-predecessor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/edit-observation-precondition-continuity.md
[selector-predecessor]: https://github.com/theRebelliousNerd/codenerd/blob/83aa2027ee122c01f532bb0b0e33106d391af185/Docs/audits/explicit-package-selector-fidelity.md

