# Go module directive syntax loses Tier 3 import identity

**Status:** source-confirmed, bounded retrieval correctness gap; proposed repair, not implemented or measured.  
**Reviewed:** 2026-10-30; public main `388205138ba6d47c1cff0c5e97c896dfa642a391`, rechecked at 11:45 UTC.  
**Method:** static source and primary grammar review only. No repository modification, application execution, tests, inference, benchmark or task-outcome claim.

## Finding and boundary

Tier 3's Go import expander compares correctly decoded source import paths with an undecoded, single-token regex capture from the workspace's `go.mod`. For a valid double-quoted module declaration, that capture includes quotes; for a valid parenthesized module declaration, it is `(`. A conventional hosted-module import therefore fails the same-module comparison and is rejected by the external-import branch. The equivalent unquoted declaration resolves correctly.

The lost product is **Tier 3 selected files and their `tiered_context_file` facts**, not Tier 2 `candidate_file` rows. This is neither a global rejection of valid Go modules nor a claim that the agent cannot discover the imported file another way. The default task-brief floor also matters: it is **60**, while import neighbors score **50**, so a direct brief difference requires a supported floor of 50 or lower. At defaults, the demonstrated difference is in the retrieval result/facts, not necessarily the final model brief.

The intended local contract is unusually clear: this expander says it follows packages inside the current module, excluding standard-library and external modules. Syntactically different valid spellings of the same module path should not change that identity. The ordinary unquoted case is already covered by targeted tests. [Expander contract and regex](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/go_imports.go#L18-L34), [existing fixture and assertions](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/wiring_test.go#L21-L82).

## These are valid module spellings, not malformed-input witnesses

Use these three alternatives for the same workspace-root `go.mod`, keeping the rest of the file and workspace unchanged:

```go
module example.com/proj
```

```go
module "example.com/proj"
```

```go
module (
    example.com/proj
)
```

Go's official module grammar allows a module path to be a string and allows the single module path inside a parenthesized block. The full `golang.org/x/mod/modfile` parser also explicitly handles a `module` line block and decodes double-quoted module arguments before assigning the module path. These witnesses deliberately do **not** use raw backticks; the pinned parser's string handler reserves other quote forms. [Official Go module grammar](https://go.dev/ref/mod#go-mod-file-module), [x/mod v0.42.0 block dispatch](https://github.com/golang/mod/blob/v0.42.0/modfile/rule.go#L269-L327), [module argument handling](https://github.com/golang/mod/blob/v0.42.0/modfile/rule.go#L420-L439), [double-quote decoding](https://github.com/golang/mod/blob/v0.42.0/modfile/rule.go#L860-L874).

This is normal current-module resolution, not an attempt to make Tier 3 resolve a dependency, `replace`, a nested module, or a multi-module `go.work`. The fixture root contains the `go.mod` and `internal/beta` directory; its source imports `example.com/proj/internal/beta`.

## Exact producer-to-consumer chain

1. **The real task executor has this retrieval producer.** Factory construction creates the sparse retriever and attaches a `TaskRetriever` to both the session executor and its spawner. The task configuration obtains the user's retrieval threshold parameters. The kernel gates issue retrieval for `/fix`, `/debug`, `/review` and `/security`; this is not a claim about every intent. [Factory](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2694-L2710), [task configuration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_task_retrieval.go#L11-L39), [intent policy](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/retrieval.mg#L11-L23).

2. **Tier 3 is reached from selected source files.** Each seed creates a fresh tiered builder. After explicit-file and keyword tiers, `BuildContext` expands imports of the already selected files; `.go` dispatches to `goImportNeighbors`. Selected import neighbors receive relevance 0.5. [Fresh builder](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/facts.go#L227-L243), [tier order](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/tiered_context.go#L220-L251), [expansion and Go dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/tiered_context.go#L450-L489).

3. **The two identity producers disagree.** `parseGoImports` uses the Go parser and `strconv.Unquote`, producing `example.com/proj/internal/beta`. `goModulePath` instead reads root `go.mod` and stores the literal regex capture: `"example.com/proj"` including quotes, or `(`. Its cache preserves that value within the builder; it does not correct it. [Import decoding](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/go_imports.go#L64-L85), [module extraction/cache](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/go_imports.go#L88-L115).

4. **The intended external-import guard makes the omission deterministic.** Neither malformed identity equals or prefixes the decoded import. The first import-path segment is `example.com`, containing a dot, so the GOPATH-like fallback is unavailable and the resolver returns empty. With `module example.com/proj`, prefix removal instead yields existing directory `internal/beta`. The caller skips empty resolutions. [Call and skip](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/go_imports.go#L38-L60), [directory resolution](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/go_imports.go#L118-L140).

5. **Missing selections cannot become that tier's evidence.** `TieredContextFacts` emits `tiered_context_file` only for selected files. Task retrieval queries the brief policy and renders its paths; the executor carries the brief into task context. This is a path/metadata handoff, not automatic injection of every selected file's source bytes. [Fact construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/facts.go#L387-L431), [task retrieval](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/decisions.go#L326-L381), [brief rendering](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/decisions.go#L234-L257), [executor handoff](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/issue_retrieval.go#L40-L76).

## Minimal source-derived comparison

Reuse the existing Go-import fixture: `internal/alpha/alpha.go` imports `fmt` and `example.com/proj/internal/beta`; `internal/beta/beta.go` contains only its package declaration and `const Name = "beta"`. Keep `beta_test.go` as an exclusion control. Change only the module declaration among the three alternatives above.

Use a retrieval-eligible `/fix` task such as `repair internal/alpha/alpha.go`, a stable included workspace and sufficient pass budget. For an isolated full-pipeline comparison, use no embedding searcher or relevance judge and ensure beta has no independent keyword hit. This lowercase request names alpha but no capitalized symbol for the definition fallback. The keyword extractor removes named paths from prose-domain extraction. [Fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/wiring_test.go#L21-L29), [keyword extraction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/sparse.go#L199-L218), [path/prose separation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/sparse.go#L275-L296).

The static prediction is:

| Observation | Unquoted control | Double-quoted or block |
|---|---|---|
| Tier 1 alpha | Present | Present |
| Tier 3 beta | Present, score 50 | Absent through this edge |
| Beta's Tier 3 fact | Present | Absent |
| Brief at explicit floor 50 | Includes beta | Omits beta absent another route |
| Brief at default floor 60 | Excludes this score-50 entry | Also excludes it |

The floor-50 configuration is supported: configuration validation requires a positive threshold, while defaulting chooses 60. The brief rule compares relevance against that floor. **Do not present the default-60 row as a demonstrated model-visible regression.** [Defaults, validation and parameters](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/retrieval.go#L13-L72), [brief selection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/retrieval.mg#L25-L36).

These are unexecuted comparison cases, not reported test results.

## Counterevidence and limits

- **Other routes can recover beta.** Explicit mention, keyword ranking, embedding similarity or a named-symbol definition search may select it independently. Existing deduplication then correctly avoids adding it again as Tier 3. Tier 4 remains independent of module extraction. [Deduplication](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/tiered_context.go#L450-L475), [semantic/definition fallback](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/tiered_context.go#L578-L642).
- **World/CodeDOM is not this parser.** In particular, the world dependency-link resolver independently strips quotes from the simple no-comment quoted declaration and can retain the corresponding workspace edge. CodeNERD's manifest-pin parser already uses `modfile.ParseLax`. Do not claim global Go parsing or dependency-graph failure. [World module reader](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/world/dependency_links.go#L90-L106), [world package resolution](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/world/dependency_links.go#L223-L242), [manifest pins](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/manifest_pins.go#L80-L90).
- **Defaults deliberately separate evidence from starting points.** The config describes import neighbors as compressor evidence, not default brief entries. Compressor code has a tiered-file consumer, but this report does not establish a particular activation or final answer change. The brief itself tells the model that structural queries can find omissions. [Compressor consumer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor.go#L402-L427), [activation handoff](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor.go#L461-L469), [brief limitation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/decisions.go#L225-L227).
- **Existing bounds remain useful.** Tier 3 stops at its cap or an expired context and limits each imported package to three non-test Go files, ordered by size/path. Defaults allocate ten Tier 3 files. A timeout, an already-selected beta or a full tier can hide the proposed comparison and must be separated from this parsing issue. [Tier bounds](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/tiered_context.go#L91-L151), [package selection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/go_imports.go#L153-L194).
- The common unquoted form works. No prevalence estimate, security bypass, unsafe acceptance, unavoidable extra model turn or measured task-quality loss is established.

## Narrow repair and complete cost

Parse the root module declaration into its semantic path using a grammar-aware parser; preserve the per-builder cache, current-module-only policy, directory existence checks, deduplication and file caps. CodeNERD already depends on `golang.org/x/mod v0.42.0`, so this need not introduce a new dependency. Choose strict versus lax parsing and malformed/missing-file behavior explicitly rather than accidentally changing how partially edited manifests are treated. [Existing dependency](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/go.mod#L29-L35).

**Do not substitute `modfile.ModulePath` and assume both cases are solved.** That helper handles an ordinary quoted line, but its line-oriented implementation returns the remainder of the first `module` line; a block still yields `(`. Use the full parse result or an equivalently grammar-complete implementation. [Lightweight helper](https://github.com/golang/mod/blob/v0.42.0/modfile/read.go#L930-L961).

The repair adds syntax parsing/allocation once per fresh builder instead of a regex capture; both paths already read the manifest. Correctly recovering neighbors may add directory enumeration/stat work, selected-file metadata/facts and, when policy admits them, brief text. Subsequent model reads may add tokens. Conversely, a useful recovered neighbor may avoid later search/read/model turns, but that is conditional and unmeasured. File-count budgets and estimated tokens are not actual provider billing or elapsed-time savings. This is primarily an evidence-correctness repair, not a promised token optimization.

## Fair evaluation plan

1. Extend the existing module-path and end-to-end Tier 3 tests with ordinary, quoted and parenthesized equivalents; verify identical canonical module paths, selected beta path and score. Check both `tiered_context_file` and rendered brief at floors 50 and 60.
2. Retain external-module, standard-library, test-file, self-edge, duplicate, missing-directory, no-manifest and cache controls. Add malformed/duplicate module declarations and document the chosen error policy. Do not silently expand scope to nested modules or external dependencies.
3. Test independent Tier 1/Tier 2/Tier 4 recovery, full-tier and deadline cases. Report omissions attributable to parser identity separately from capacity, coverage and cancellation.
4. Measure manifest parsing CPU/allocation, directory and file operations, selected-path/fact differences, brief tokens and actual provider tokens separately. Include common unquoted workspaces to expose added overhead.
5. If authorized later, pair task-level trials on identical code/corpus/config except declaration spelling. Compare neighbor recall, useful follow-up reads, tool/model turns, correctness, latency and total cost—not merely a smaller prompt. No runtime evaluation was performed for this report.

## Prior-audit and branch differentiation

Reviewed guidance, relevant main retrieval audits and research-branch inventories through `research-main-20261030-1117`, plus the current worktree-lane differences. The selected retrieval module-reader file is unchanged in the reviewed lane comparisons. This is a bounded novelty check, not an assertion that every historical branch/file was exhaustively audited.

The previous [definition-scan amplification audit](https://github.com/theRebelliousNerd/codenerd/blob/c3a596c8e9444a523b3368a25843cb3bad2de112/Docs/audits/retrieval-definition-scan-amplification.md) concerns repeated Tier 4 native searches; this finding concerns Tier 3 module identity. [Go major-version pin continuity](https://github.com/theRebelliousNerd/codenerd/blob/07bdcb50873498d22fce498f599073df00ac5750/Docs/audits/knowledge-go-major-version-pin-continuity.md) concerns dependency-family/version selection; [observed-version receipts](https://github.com/theRebelliousNerd/codenerd/blob/b7fe0e7e57df31ccd3dc5b51a4f292bb197c491c/Docs/audits/knowledge-acquisition-observed-version-receipts.md) concerns knowledge acquisition evidence. Neither is this root-module declaration parser. Existing retrieval freshness, coverage, publication and allocation repairs should remain intact.

**Recommendation:** fix and regression-test the local identity conversion; retain all retrieval budgets and disclosure controls. Treat expected evidence parity as the acceptance condition, and any end-to-end efficiency or answer-quality improvement as a separate measurement.

