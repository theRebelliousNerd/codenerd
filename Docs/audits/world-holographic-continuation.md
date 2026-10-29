---
doc-class: deep-dive
subsystem: world-holographic
implementation-status: partial
last-verified: 2026-10-28
verified-against: 8cd6e1f8804709996b46e43641a3ffe464d2088a
verification-method: static-source-inspection
supersedes: []
---

# Holographic context: preserve live capability, complete the identity bridge

## Scope and verdict

Source-only review of CodeNERD main at `8cd6e1f8804709996b46e43641a3ffe464d2088a`; GitHub comparison against main was identical at 2026-10-28 04:07 UTC. No tests, tools, inference, or application execution were performed. The reported dead-code audit is a lead, not independent runtime evidence.

**The two flagged functions are not evidence that CodeNERD has lost function-body access.** `HolographicProvider.fetchFunctionBody` and `extractLineRange` implement a legacy eager source-body projection. Current holographic context deliberately ranks and describes relevant code without pasting every caller body into the prompt; existing CodeDOM tools retrieve the selected source. The useful next innovation is stronger, revision-aware identity across that boundary—not restoring indiscriminate body injection.

## What the source actually wires

| Stage | Observed implementation | What this establishes |
|---|---|---|
| Change evidence | CodeDOM mutation symbol extraction emits `modified_function` / `modified_interface` facts. | The impact rules have an implemented producer; old documents saying the input was never asserted are stale. |
| Impact ranking | Mangle derives bounded caller impact and `context_priority_file` from modified symbols and call/definition facts. | Impact is based on the graph represented in the kernel, not a proof of complete semantic impact. |
| Context enrichment | `getContextInternal` invokes `applyImpactPriorities`. | Ranked callers enter normal holographic context; the special legacy build helper is not necessary for this. |
| Session integration | Factory installs the provider in the executor and session spawner. `withFileContext` invokes budgeted rendering inside an active working loop, otherwise ordinary rendering. | This is a production context path, including an explicit policy/budget interface. |
| Model-facing selection | The priority section renders caller name, file basename, priority and depth, with an explicit omitted count. | Selection information is present. It is not the full canonical source address. |
| Actual source | The production tool catalog registers CodeDOM; `get_element` resolves refs, rejects ambiguity, reads current source and renders source identity and revision. | Selected source retrieval exists independently of the dead body helpers. |

Primary source: [mutation symbols](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/core/codedom_modified_symbols.go), [impact rules](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/core/defaults/policy/impact.mg), [normal context enrichment](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/world/holographic.go#L835-L890), [factory installation](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/system/factory.go#L2444-L2605), [session rendering](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/executor_tools.go#L1333-L1368), [canonical production catalog](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/tools/catalog/catalog.go#L32-L55), [element retrieval](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/tools/codedom/elements.go#L594-L850).

The mutation evidence above is not inferred from the helper's existence alone: both [the tool mutation fact path](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/core/virtual_store_tool_facts.go) and [the CodeDOM edit path](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/core/virtual_store_codedom.go) call `modifiedSymbolFacts`. Likewise, [CodeDOM's actual registrar](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/tools/codedom/register.go#L7-L20) includes `GetElementTool`, alongside structural query tools.

## What the flagged helpers were supposed to do

`fetchFunctionBody` delegates to `callerSpan`, then extracts the entire selected line interval. `extractLineRange` clamps bounds, but no longer imposes the old 50-line ceiling. The file explicitly explains why bodies are not pasted into the holographic prompt: historical prefix truncation concealed incomplete source, while explicit element/file reads can obtain the selected function. `ResolvePrioritizedCallers` resolves line spans without filling every body. `FormatWithPriorities` directs the reader to `get_element`/`read_file` instead.

The older span resolver is not an ideal universal identity resolver. Its Go AST selection compares the declaration's bare name, without receiver identity. Its non-Go fallback is regex/brace based rather than a language-complete parser. These are reasons **not** to reconnect that helper as the authoritative source hydrator. They do not prove the live CodeDOM resolver has the same defects.

See [explicit design rationale and impact API](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/world/holographic_impact.go#L20-L145), [body/span resolver](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/world/holographic_impact.go#L385-L550), and [whole-range projection and output](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/world/holographic_impact.go#L682-L785).

**Disposition:** classify both as retained legacy/test-facing projection helpers, with a live alternative for the intended capability. Do not call them missing production wiring. Deletion is a separate maintenance decision requiring the project's test/API policy; this research does not authorize deletion.

## The genuine continuation: identity-preserving lazy hydration

The current priority prompt writes `c.Name` and `filepath.Base(c.File)`. Two packages can contain the same basename and similarly named methods. That display is useful for human orientation but insufficient as a unique source address. Current CodeDOM resolves ambiguity explicitly; this finding is an information-loss opportunity, not an observed wrong-file incident.

Proposed continuation:

1. Convert each impact candidate into a typed inspection handle containing workspace-relative full path, canonical element ref when resolvable, source revision, graph generation, depth and the change/edge evidence that produced its rank.
2. Keep unresolved, stale and unsupported candidates explicit. Never fabricate a ref from a display name or treat missing graph coverage as no impact.
3. Let policy choose inspection obligations against the current task and available context budget. The host supplies measured candidate sizes and lookup outcomes; Mangle supplies selection policy. Do not encode a second hidden selection heuristic in the body helper.
4. Satisfy selected obligations through the existing `get_element`/structural-read path. Preserve its ambiguity failures, current-source loading, secret-path protections and revision output. Large elements can expose named parts with an explicit full-source option; a part must not be represented as the whole element.
5. Revalidate or invalidate an obligation if the file revision or structural generation changes before its evidence is consumed. A citation from the old revision remains historical evidence, not current completion evidence.

Minimal contract (proposal, not an implemented schema):

```text
ImpactCandidate = (candidate_id, relative_path, canonical_ref_or_unresolved,
                   source_revision, graph_generation, depth, evidence_ids)
InspectionDecision = (candidate_id, task_id, required_view, reason, policy_revision)
InspectionEvidence = (candidate_id, actual_revision, returned_view, completeness,
                      access_result, source_citation)
```

Selection must not mark an obligation satisfied merely because a request was attempted. Retrieval errors and ambiguity remain obligations or explicit blockers. Canonical refs and revisions should be preserved as data; display labels are not identifiers. Existing parser coverage includes Go, Mangle, Python, TypeScript/TSX and JavaScript; unsupported language cases must remain explicit, rather than falling back to a claimed exact semantic result. See [prompt display](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/world/holographic.go#L530-L562) and [parser dispatch](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/world/codemodel/model.go#L289-L330).

This continuation is distinct from the existing [structural coverage/refactor-preconditions audit](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/Docs/audits/structure-coverage-and-refactor-preconditions.md): that work concerns the index's observation domain, completeness and safe edit preconditions. This proposal concerns retaining identity and revision evidence between a ranked impact candidate and its selected source view. It should consume existing coverage/generation evidence rather than invent a second index or repeat historical gaps that have since been addressed.

## Costs and fair evaluation

Do not claim fewer tokens simply because bodies disappear from the initial prompt. Total cost includes graph/index maintenance, ranking queries, source reads/parsing, handle metadata, tool request/response tokens, extra model turns, stale-evidence retries, and the downstream debugging caused by insufficient context. Lazy hydration can avoid irrelevant body reads; it can also increase latency when many callers are needed.

Compare three complete systems under the same model, task evidence, permissions and total budgets: current ranked holographic context plus ordinary tools; eager complete caller bodies; and proposed canonical-handle policy-selected hydration. Also compare just the context-selection socket while holding the rest of the harness fixed. Record accepted task correctness, source-address precision, unsupported/stale disclosures, unique revisions read, bytes parsed, model input/output tokens including retries, wall latency and human repair—not only first-prompt size.

Required fixtures: equal basenames in different directories; same method name on different receivers; edits between ranking and read; renamed/deleted elements; a caller larger than the default part threshold; incomplete graph coverage; unsupported languages; revoked file access; duplicate impact paths; cancellation during hydration; and changes whose relevant callers lie beyond the configured impact depth. The desired outcome is correct evidence selection with honest limits, not automatic saturation of the prompt.

## What should not be re-opened as a missing feature

Current source already attaches impact priorities on the normal context path, supplies session/spawner integration, exposes revision-bearing element reads, and has budgeted holographic rendering. Historical documents can motivate the design, but their old “unwired” descriptions must not be reused as current findings. This review establishes source wiring only; runtime effectiveness and the proposed identity bridge remain unmeasured.

## As built (2026-10-28)

- **Identity in the prompt.** The impact-prioritized caller section (`HolographicProvider.promptSection`, `internal/world/holographic.go`) renders each caller as `name` — `workspace/relative/path:start-end` (`HolographicProvider.workspaceRel`), not the file basename. A header line gives the read-back form once: `get_element path=<path> ref=<name>`. Putting it on every line instead nearly doubled each line's bytes and halved how many callers a budget admits. Test: `TestWorkspaceRel_SameBasenameStaysDistinct`.
- **Legacy helpers deleted.** `HolographicProvider.fetchFunctionBody` and `extractLineRange` had no production caller; `get_element` is the live hydration path. The size and path-containment guards they shared live in `callerSpan`, and their tests now exercise them through `locateCallerLines`.
- **Deferred:** typed `ImpactCandidate` handles carrying graph generation and evidence IDs, and policy-selected inspection obligations. They need the structure-coverage generation (`Docs/audits/structure-coverage-and-refactor-preconditions.md`, built) to be joined into impact facts, and a paired task study to show that the selection beats the current ranking.

## Status note (2026-10-28)

Fixed: the production prompt path never attached spans (the file-reading resolver is only reached from the unused `BuildWithImpactPriorities`, and it matches bare names against receiver-qualified ids), so callers were addressed by path alone. `applyImpactPriorities` now takes each caller's span from the `code_defines` rows the impact rules join, with no file read, and a (file, symbol) with several spans prints a count instead of a guess (`holographic_impact.go attachDeclaredSpans`).
Ruled out: restoring body injection; the legacy resolver is left as test-facing (it cannot match qualified ids and has no production caller).
Left: typed `ImpactCandidate` handles with source revision and graph generation, and policy-selected inspection obligations; they need a consumer and the paired study.
