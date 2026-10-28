---
doc-class: deep-dive
subsystem: tools-observation-codenerd-navigation
implementation-status: planned
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
feature-compared-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
verification-method: static-source-inspection
supersedes: []
---

# Preserve evidence meaning when workspace search becomes navigation

## Decision

Keep the compact `search_code` projection and immutable `search_expand` replay, but make **occurrence, symbol identity, and source version** separate contracts. A lexical match is useful navigation evidence; it is not automatically a dependency. A shortened display name must not become an ambiguous or different element address. A projection must be derived from the same bytes as the retained match, not a second filesystem state.

Three source-grounded gaps remain in the shared local-search codec:

1. Matching lines inside a body become dependency/reference edges, even when the match is only a comment, string, unrelated identifier, or regex alternative. A declaration-line match need not name the declaration.
2. Method qualifiers are deliberately stripped before the codec keys its symbol map. Distinct same-named methods in one file collapse; a rendered address can be ambiguous or resolve to a same-named top-level function.
3. Both production producers collect hits, then reopen matched files for symbol/edge projection. Mutation between those stages can combine old hit text with a new enclosing symbol. Faithful later replay does not repair an inconsistent initial projection.

These are capability failures before a model follows an address or summarizes a dependency. They do not establish that CodeNERD made an incorrect edit, bypassed an edit guard, or completed an incorrect production task.

**Evidence status:** static source inspection and proposed design only. No implementation, tests, benchmarks, provider inference, CodeNERD execution, or GitHub writes were performed. The examples and acceptance cases below are unexecuted witnesses, not observed incidents.

## 1. Source freeze and non-duplication

At approximately 12:08 UTC, main was [356f65e][main], the only enumerated `feat/*` branch was [feat/conversation-brief at c2d39e5][feature], and the research branch was [5d3cba1][research]. All three heads were re-resolved unchanged at approximately 12:22 UTC. Full recursive trees were available without truncation. The main/feature blob identities match for `internal/tools/core/search.go`, `internal/observation/codesearch.go`, `internal/tools/codedom/elements.go`, `internal/core/virtual_store_file_actions.go`, and the inspected codec tests. This is not an omission inferred from a capped compare response.

The [root guidance][guidance] requires world/CodeDOM-first context, relevant revision-aware detail, recoverable eviction, and tools that condense search space or offload deterministic cognition. The [architecture-document standard][standard] requires evidence, target design, and checkable exits to stay separate.

Existing work is deliberately preserved:

| Existing account | Boundary of this proposal |
|---|---|
| [Structure coverage and refactor preconditions][structure-study] | Structural-index enumeration, generation-bound pages, use-set proof, and refactor preconditions. Current main already has [observation deficits and generation-bound cursors][structure-observation]; those are not newly reported as absent. This note concerns the separate lexical-search projection into navigation. |
| [Request-visible tool contract][catalog-study] | Whether granted tools and schemas actually reach the request. This note assumes the appropriate search/expansion tools were offered and admitted. It does not propose removing structural-first policy or widening tool grants. |
| [Research observation identity and admission][research-study] | Remote acquisition semantics, URL identity, paging, and working-context adoption. This note concerns local source bytes, lexical-to-semantic promotion, and element identity. Reuse the shared source-adoption design rather than designing a competing ledger. |
| [TUI evidence turn ownership][tui-study] | Which turn owns evidence and whether the first articulation receives it. Correct delivery cannot repair a search projection that already changed what the evidence means. |
| [Retrieval freshness and context value][retrieval-study] | Sparse/semantic retrieval cache coverage and invalidation. This note concerns a live tool result, including stable-input failures without any cache. |

The [broker TODO][broker] already records the code-search codec as built, wired through both producers, and expandable without rerunning. Those are real achievements. Its stronger characterization of the output as dependency-bearing edges is exactly the claim that requires qualification here. Historical byte measurements there are not measurements from this review.

## 2. Existing journey and guarantees

A normal workspace investigation can prefer structural tools first and later admit raw search under Mangle policy. The session withholds raw-search definitions only when its structural offer includes `find_symbol`; the current working policy opens search after configured structural trials or misses. This is a real executive regime, not a claim that search is missing. [Offer logic][structural-offer], [policy][search-policy]

For an admitted modular `search_code` call:

1. `parseContentSearch` resolves the workspace-contained scope and regex; `contentSearch.run` collects line hits. `grep` shares that acquisition path. [Acquisition][search-acquisition]
2. `executeSearchCode` converts hits to `observation.Search`, creates a displayed-path-to-real-path map, and invokes the shared codec. [Producer][search-producer]
3. `Project` maps hit line numbers to enclosing elements, constructs symbols and edges, and caps the displayed projection. `Encode` retains the raw search result in the bounded retention store. [Projection][projection], [encoding][encoding]
4. `search_expand` and the retained-handle branch of `recall_context` read retained matches, not current files. [Expansion][expansion], [recall][recall]

The VirtualStore `handleSearchCode` is a second producer. It performs literal substring matching, has a configured per-file size exclusion, and uses the same codec. It also keeps raw `search_result` facts rather than asserting the projected graph into Cartographer-owned predicates. The projection issues therefore affect its output, but this report does **not** claim it injects false `code_calls` facts. [VirtualStore producer][virtual-search]

Preserve these existing protections:

- Containment and membership filtering in modular search; do not substitute free-form shell searches.
- Explicit cap notices, projection omission counts, and expansion's distinction between the end of retained matches and an acquisition cap.
- Immutable retained replay and retention entry/byte/TTL bounds.
- CodeDOM's ambiguity refusal and existing edit revision checks.
- The distinction between a useful positive lexical occurrence and an authoritative absence or semantic reference proof.

## 3. Finding N1: a lexical hit becomes a dependency

`search_code` accepts the same regex pattern as `grep`. Its description promises symbols and dependency edges. In `Project`, an occurrence is associated with an enclosing element by line. Any hit at the element's declaration line sets `Declared = true`; there is no check that the matched text is the declaration's name. Hits at or before `DeclLine` are excluded from reference counts, including doc comments. Remaining hits become `EdgeReferences`. If exactly one symbol was marked declared, **all** such edges target that symbol; otherwise they target the query text. [Tool contract][search-definition], [edge construction][edge-construction]

The rendering calls these references and dependency edges, not “lexical mentions” or “candidate links.” [Rendered claims][rendering]

### Unexecuted witnesses

**Body string/comment.** In a file with a declaration `Encode` and an unrelated function whose body contains the string `"Encode"`, searching `Encode` can mark the former declared and the latter as referencing it. The same applies to a matching body comment. A doc comment before the declaration is excluded by the current implementation and is not this witness.

**Signature match.** A query matching a parameter type, return type, or another part of a declaration line marks the enclosing function as declaring the searched thing. The mere presence of one such line does not identify a unique declaration of the query.

**Regex alternatives and shadowing.** `Encode|Decode`, local shadowing, a different receiver's same-named method, or a string literal may produce useful occurrences without one common resolved target. A single declaration hit in the sampled results is not evidence that every other match binds to it.

This is not a demand to make every lexical search type-check the repository. The cheaper correct default is to say what was established: matching occurrence inside a named element. A semantic edge may be added only when a suitable analysis establishes its kind, target, and domain. Keep candidate and verified relations distinguishable, including in compact text.

Existing tests intentionally pin the sole-declaration target rule and multiple-declaration fallback. They demonstrate the current contract; they do not validate its semantic inference for the counterexamples above. [Current tests][edge-tests]

## 4. Finding N2: shortening a method name destroys its address

`ElementsFromSource` exposes each parsed element's `e.Key`, including its qualification. `Project` strips everything before the final dot for method names, then constructs `file + ":" + name` and uses that value as the symbol-map key. The first occurrence sets that map entry's extent; subsequent same-key occurrences increment its count. [Element extraction][element-extraction], [identity construction][identity-construction]

For two methods such as `A.encode` and `B.encode` in one file, this yields one `file:encode` entry, not two distinct navigable symbols. Its extent belongs to the first inserted element while its hit count can include both.

The downstream resolver deliberately accepts file-scoped keys and bare names. It refuses multiple candidates, and exact element keys win. A bare function name can therefore resolve to a top-level function when the search occurrence was in a method with the same name; absent that exact key, multiple methods can instead yield an ambiguity error. Both are failures of search-to-navigation fidelity, though the ambiguity refusal is a valuable safeguard. [Resolver][resolver], [lookup semantics][lookup]

**Required contract:** display labels may be short; navigation identity may not be shortened. Each printed address must preserve the projected element identity and source revision. Redemption must return that element at the recorded revision when retained evidence supports it, or explicitly report stale/source-changed/current-revision evidence and offer bounded reacquisition. It must not silently substitute another element or current bytes for the historical claim. A non-addressable occurrence needs a targeted source-view action; this is not a promise to retain historical full-file bytes forever.

Use CodeDOM's existing element key/canonical-reference machinery rather than inventing another receiver-normalization convention. File-scoped qualified keys can be sufficient when the interface already supports them; a new global identity service is not necessary.

A current regression explicitly demands `w.py:encode` after stripping `Widget.encode`. It needs refinement, not blind preservation as proof that the result is addressable. A one-method fixture cannot detect collisions. [Qualifier test][qualifier-test]

## 5. Finding N3: retained text and projected structure can describe different bytes

The modular producer completes `search.run()`, then supplies an `os.ReadFile` callback to `Encode`. The VirtualStore producer similarly reads each file for hits, discards that buffer, and later supplies another `os.ReadFile` callback. The `Search`/ `Match` records contain query, displayed path, line, text, and a truncation flag, but no searched-source revision or immutable source buffer. [Modular producer][search-producer], [VirtualStore producer][virtual-search], [search shape][search-shape]

`Project` invokes the callback and associates old match line numbers with elements parsed from the returned bytes. It does not compare a searched-source digest, verify the old line's identity, or reject a changed file. [Projection read][projection-read]

**Unexecuted witness:** gather a hit inside function A; before projection's second read, change the file so function B occupies that line. The codec can name B while retaining A's original matching text. A same-length rewrite can preserve line numbers and file length. No mtime-restoration trick is needed: the producer simply does not carry the searched revision.

The existing expansion tests rewrite/delete source **after the complete search call returns**, then verify retained replay. They correctly establish post-encoding immutability, not coherence between the two reads inside encoding. [Tool-pair test][replay-test], [codec replay tests][codec-replay-tests]

The extraction helper's comment already says it takes text so the projection can describe exactly the bytes observed. The helper honors that contract for its input; the producers supply a later read. Repair the producer/codec seam rather than adding another warning to `search_expand`. [Extraction contract][element-extraction]

## 6. Proposed observation and navigation contract

The following is proposed design notation, not existing API names.

```text
WorkspaceSearchObservation:
  observation_id, owner_scope, workspace_identity
  query_spec: kind (literal|regex), text, path, file_filter, flags
  acquisition: interval, coverage, limits, omissions
  sources[]: file identity, digest/revision, captured representation
  occurrences[]: source_id, line/span, matched text
  navigation[]: occurrence -> exact element key + source revision
  relations[]: relation_kind (occurrence|reference|import)
               evidence_strength (lexical|syntactic_candidate|resolved)
               analyzer + domain + supporting occurrence/source IDs
  projection: version, selected rows, explicit omissions
  expansion: retained observation identity, available|unavailable, continuation
```

### Acquisition and projection

Capture an immutable per-file source representation once and derive line matches and structural attribution from it. Prefer processing one file at a time so the correction does not require keeping every full file resident until the entire scan finishes. Retain only the evidence needed for replay and navigation after projection; immutable parsed records or content-addressed buffers can be shared when their identity is verified.

Reading once makes matching and projection agree on captured bytes; an instantaneous filesystem-revision claim still requires stable-read validation or an immutable source version. A coherent per-file capture is not an atomic whole-workspace snapshot. Cross-file claims need a declared observation interval, generation evidence, or a stronger supported snapshot mechanism. Do not claim filesystem-wide coherence merely because each file was read once.

For files too large or unsupported for structural parsing, keep valid lexical occurrences with explicit “structure unavailable” status. Do not invent enclosing elements or label unsupported parsing as successful semantic analysis. Handle a changing source by retrying under an explicit acquisition budget, returning an unresolved projection, or taking a supported immutable snapshot—not by silently mixing versions.

### Navigation and relations

- Preserve the exact element key independently of its display name.
- Keep relation kind (occurrence, reference, import) separate from evidence strength (lexical, syntactic candidate, resolved). Lexical grouping remains useful even without semantic resolution.
- If a consumer needs callers/importers/use sites, Mangle derives an obligation for the available structural operation with a stated domain. A search-generated edge must not satisfy a stronger obligation merely because it has two endpoints.
- Keep raw-match expansion immutable. Reading the current source is a separate operation with its own revision and outcome.
- Adopt the typed local source descriptor into working context through the shared observation design; do not attach a multi-file search to an incidental focus-file revision or create another independent archive.
- Existing authorization applies at acquisition, consumption, and any expansion route. An observation handle is not a grant of broader workspace access.

### Resource bounds are part of the repair

The modular acquisition builds a candidate-file slice before scanning. `searchFile` appends every scanned line to `lines`; it prunes that slice only when `contextLines > 0`, so the default zero-context search retains all scanned lines of the current file. The walk/scan loops do not accept a context parameter for cancellation. The 1 MiB line-scanner limit is not a total-file memory limit. [Acquisition loops][search-acquisition], [scanner][scanner]

Likewise, the projection's 1 MiB per-file ceiling is checked after its source-reader callback returns bytes. The retention store's 8 MiB default protects retained payloads, not allocations already incurred by reading, searching, projecting, or JSON encoding them. A payload above the retention budget produces no handle; preserve the projection but disclose that expansion is unavailable. [Projection ceiling][projection-read], [retention limits][retention], [oversize refusal][retention-refusal], [encoding][encoding]

The VirtualStore has its own per-file stat-size ceiling, which must be credited; it does not have the modular regex/result-cap semantics. A common observation contract should not accidentally erase these differences. [VirtualStore producer][virtual-search]

Use bounded context rings, byte-aware acquisition/serialization, streaming or bounded snapshots, and cancellation checks inside the owned work. Bounds should yield explicit partial/unavailable states with resumable or targeted next actions. They must not become arbitrary task-ending call limits or silent evidence truncation.

## 7. Full cost model and alternatives

Evaluate **cost per correct, supported navigation/answer**, not bytes in the first tool response.

| Cost/benefit | Required accounting |
|---|---|
| Model/provider | Input, output, cached-input and reasoning usage where reported; retries, expansions, clarification and corrective turns; final answer quality. Do not double-count subcategories of provider totals. |
| Acquisition | Directory entries visited, files opened, bytes read, re-reads avoided, scanner buffers, cancellation latency, IO errors and excluded domains. |
| Projection | Parse calls and bytes, relation-resolution work, digest CPU, temporary peak memory, symbol/edge serialization. |
| Retention | Encoded and retained bytes, copies, TTL/evictions, unavailable expansion handles, archive/ledger duplication. |
| End-to-end | Time to correct source location and supported answer; wrong-address rate, false-dependency rate, abstentions, failed/abandoned tasks and repair effort. |
| Safety/capability | No wider grants, no lost containment/membership checks, no substitution of lexical evidence for semantic or edit preconditions. |

Compare at least:

1. **Current projection:** cheapest code change, but preserves the three identified fidelity gaps.
2. **Truthful lexical projection plus exact keys:** rename relation strength, preserve symbol identity, and share capture bytes. Likely the smallest correct baseline; no mandatory type-check on every search.
3. **On-demand structural enrichment:** add verified edges only when the user's question needs them and the supported analyzer can establish them. Costs extra calls/analysis but avoids universally paying for semantic resolution.
4. **Always-on semantic graph:** potentially stronger navigation but substantially greater parsing/resolution/build-configuration cost and unsupported-language handling. Do not assume this wins.
5. **Raw grep only:** simple, preserves visible lines, but shifts grouping/navigation cognition to the model and can increase repeated reads and context. Retain as a baseline, not the recommended rollback.

Single-capture projection may save the second file read but can increase buffer lifetime or spooling; measure both. Qualified names and relation-kind labels add tokens but may remove ambiguity/recovery turns. Honest partialness may increase abstentions; that is not a regression if it replaces unsupported dependency claims. No numerical savings or performance outcomes are claimed here.

## 8. Evaluation plan — not executed

### Deterministic acceptance

Run through both production producer seams when implementation/evaluation is authorized; separately preserve their regex versus literal query semantics.

| Fixture | Required result |
|---|---|
| Real call plus body string/comment containing the same term | Occurrences remain discoverable; only established bindings receive semantic-reference labels. |
| Doc comment at/before `DeclLine` | Preserve the current exclusion from body reference counts; no broadened regression claim. |
| Query matches a parameter/return type on a function declaration | Do not claim the enclosing function declares the query merely from line coincidence. |
| Regex alternatives, substrings, shadowing, same method name on different receivers | No unsupported single-target dependency inference. |
| Two qualified methods with same bare name in one file | Two distinct navigation identities and correct independent extents/counts. |
| Same-named method and top-level function | Following each search address returns the intended element, not the function selected by bare-key precedence. |
| Mutation between match acquisition and projection | One captured version throughout, explicit conflict, or non-projected lexical evidence; never mixed old text/new symbol. |
| Mutation/delete after encoding | Expansion still returns the original retained observation while available, clearly historical; expiry/unavailability remains explicit. |
| Mutation after projection but before `get_element` redemption | Preserve element identity; return retained historical evidence if supported, or explicit source-changed/current-revision status and bounded reacquisition. Never silently equate current content with the earlier observation. |
| Source/parser unsupported or over structural limit | Lexical hits survive with explicit unavailable structure; no fabricated navigation address. |
| Large no-match file, long line, high match density, many files | Bounded working memory and honest omissions; output count cap is not mistaken for work/byte bound. |
| Cancellation during traversal/read/projection | Owned work settles; canceled/partial state cannot become complete negative evidence. |
| Retention refusal/expiry and partial expansion | No advertised usable handle when storage failed; exact continuation and honest cap end-state. |

The method collision fixture must redeem the actual rendered address through `get_element`; checking a pretty label is insufficient. The mutation fixture must synchronize **between** the two current producer stages; changing source only after the result returns tests the already-working replay guarantee. No fake model should be preloaded with the correct address that the projection failed to provide.

### Normal user journey

Use ordinary chat/executor paths with CodeDOM/world-model context enabled, actual request-visible offers, structural-first policy, and existing authorization. Include:

- “Find where this log/config text is used and explain the code path.”
- “Which methods implement this behavior?” with duplicate receiver-method names.
- “Does this function actually depend on that symbol?” with lexical decoys.
- A non-code/config discovery control, to distinguish codec issues from tool-offer or unsupported-analysis limits.

Capture original request, effective offer, all observed results, source revisions, chosen navigation actions, final consuming request and answer. Score lexical recall, address redemption accuracy, relation precision, version coherence and supported final claims alongside total cost. A compact answer that hides the right file or invents dependency evidence fails even if it is cheaper.

### Checkable exits

- **SEARCH-NAV-01:** no distinct element identities merge because of display-name shortening; returned addresses redeem to the intended element under a matching revision or yield explicit stale/source-changed evidence with bounded reacquisition.
- **SEARCH-NAV-02:** lexical occurrences never masquerade as resolved dependency edges; relation strength survives rendering and consumption.
- **SEARCH-NAV-03:** match text and projected structure derive from the same per-file captured version, in both producers.
- **SEARCH-NAV-04:** existing no-rerun replay, cap disclosure, containment, membership and edit guards remain intact.
- **SEARCH-NAV-05:** full-task correctness/cost and acquisition resource bounds are evaluated, not only codec output bytes.

Spec-first implementation would refine the observation/tools contracts, correct the two producer seams and identity/relation projection, then add the composition witnesses. Current test definitions are evidence of existing coverage, not proof those exits have passed.

## Source references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[research]: https://github.com/theRebelliousNerd/codenerd/commit/5d3cba11bae145302c2e7579047032f47e1a3abf
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#north-star
[standard]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/journeys/09-architecture-doc-standard.md#the-citation-rules-unchanged-from-2026-09-20-because-they-were-measured
[structure-study]: https://github.com/theRebelliousNerd/codenerd/blob/5d3cba11bae145302c2e7579047032f47e1a3abf/Docs/audits/structure-coverage-and-refactor-preconditions.md
[catalog-study]: https://github.com/theRebelliousNerd/codenerd/blob/5d3cba11bae145302c2e7579047032f47e1a3abf/Docs/audits/request-visible-tool-contract-and-investigation-affordances.md
[research-study]: https://github.com/theRebelliousNerd/codenerd/blob/5d3cba11bae145302c2e7579047032f47e1a3abf/Docs/audits/research-observation-identity-and-admission.md
[tui-study]: https://github.com/theRebelliousNerd/codenerd/blob/5d3cba11bae145302c2e7579047032f47e1a3abf/Docs/audits/tui-evidence-turn-ownership.md
[retrieval-study]: https://github.com/theRebelliousNerd/codenerd/blob/5d3cba11bae145302c2e7579047032f47e1a3abf/Docs/audits/retrieval-freshness-coverage-and-context-value.md
[structure-observation]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/observation.go#L17-L194
[broker]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/broker/TODO.md#L115-L145
[structural-offer]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/working_context.go#L101-L136
[search-policy]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/working_set.mg#L103-L123
[search-acquisition]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/core/search.go#L337-L486
[search-definition]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/core/search.go#L591-L619
[search-producer]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/core/search.go#L597-L677
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch.go#L330-L472
[encoding]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch.go#L191-L214
[expansion]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/core/search.go#L686-L760
[recall]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/core/context_recall.go#L14-L55
[virtual-search]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/virtual_store_file_actions.go#L433-L557
[edge-construction]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch.go#L420-L472
[rendering]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch.go#L698-L794
[edge-tests]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch_test.go#L111-L165
[element-extraction]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/elements.go#L85-L106
[identity-construction]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch.go#L397-L435
[resolver]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/elements.go#L637-L739
[lookup]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/world/codemodel/model.go#L417-L459
[qualifier-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch_test.go#L603-L624
[search-shape]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch.go#L15-L36
[projection-read]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch.go#L364-L376
[replay-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/core/search_codec_test.go#L115-L150
[codec-replay-tests]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/codesearch_test.go#L316-L389
[scanner]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/core/search.go#L541-L587
[retention]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retain/retain.go#L35-L63


[retention-refusal]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retain/retain.go#L142-L193
