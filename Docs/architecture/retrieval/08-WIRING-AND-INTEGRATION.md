# retrieval — Wiring and Integration

> Last verified: **2026-08-15**  
> Focus: how (and how not) retrieval joins the live agent loop

## 1. Boot wiring

### Legacy path — `cmd/nerd/chat/session_boot.go`

```
logStep("Initializing sparse retriever...")
retrieverCfg := retrieval.DefaultSparseRetrieverConfig(workspace)
retriever := retrieval.NewSparseRetriever(retrieverCfg)
...
// SystemComponents assembly:
Retriever: retriever,
```

### Shared path — `cmd/nerd/chat/session_shared_boot.go`

Same construction, same `SystemComponents.Retriever` assignment.

**Historical finding (fixed 2026-08-15):** the corpus recorded this as
"`Model.Retriever` is retained idle". It was worse than that — `Model` had no
retriever field at all, and `bootCompleteMsg` never copied
`SystemComponents.Retriever` anywhere. The instance was unreachable, not merely
unused. `cmd/nerd/chat/model_types.go` now declares `retriever
*retrieval.SparseRetriever` and `model_update.go` assigns `m.retriever =
c.Retriever` alongside the other components.

## 2. Issue seed wiring (live)

`cmd/nerd/chat/process_seed.go` → `(*Model).seedIssueFacts(ctx, intent, rawInput)`,
called from `process.go` with the turn context.

**Gate:** kernel non-nil; intent verb ∈ `{/fix, /debug, /review, /security}`;
non-empty raw input.

**Steps:** the method now delegates the whole pass to
`retrieval.SeedIssueFacts(ctx, m.kernel, SeedRequest{...})`, passing the session
retriever (warm keyword cache), the workspace root, the glass-box bus and a
`DefaultSeedTimeout` (5s) budget that is deliberately independent of
`config.GetLLMTimeouts()`.

Inside `SeedIssueFacts`:

1. Truncate issue text to 4000 chars (caller-side).
2. `ExtractKeywords` → `issue_text/2`, `issue_keyword/3`, `keyword_weight/2`.
3. `InvalidateFromKernel` — drop cached hits for files written since the last pass.
4. `TieredContextBuilder.BuildContext` under the budget (T1 mentions → T2 keyword
   ranking → T3 import neighbors → T4 semantic/definition).
5. Resolve and relativize paths → `file_mentioned/2`, `context_tier/2`,
   `tiered_context_file/5`, `candidate_file/2`, `keyword_hit/3`,
   `issue_context/3`.
6. `sink.LoadFacts(facts)`.
7. Log the `SeedReport` summary and emit a `CategoryKernel` glass-box event.

**Degradation contract:** if the budget expires or the build errors, the
keyword-extraction facts are still asserted. Losing the whole seed to a slow
filesystem is worse than losing its disk-ranked half. `SeedReport.TimedOut`
records which happened.

## 3. Schema wiring (kernel)

`internal/core/defaults/schemas_knowledge.mg` §52 declares the EDB surface. The
retrieval package does not load Mangle; it asserts against those Decls and
`TestSeedFacts_ShouldMatchSchemaDeclArity` fails the build if the two drift.

Bound conformance is not optional and not visible at runtime — a fact whose Go
value contradicts its Decl is dropped by `RealKernel.addFactIfNewLocked` with a
log line and no error. Two such bugs were live in this path before the wire:
`issue_keyword`'s weight and `tiered_context_file`'s relevance were asserted as
0..1 float64 into `/number` slots, so every fractional one was rejected and only
the weight-1.0 mentions survived.

## 4. Context compressor wiring (consumer)

`internal/context/compressor.go` reads `issue_keyword`, `issue_text`,
`file_mentioned`, `tiered_context_file` into `IssueActivationContext`. It now
divides integer keyword weights by 100 to undo the percent scaling the Decl
forces; without that, `computeIssueScore`'s clamp to 1.0 would flatten every
keyword to the maximum boost.

Priorities also appear in `internal/context/types.go` and
`activation_scoring.go` for those predicates.

## 5. Cache invalidation wiring

Driven from Mangle rather than from the write path: `InvalidateFromKernel` reads
`file_written/4` and `file_modified_externally/1` out of the EDB and drops the
affected keyword-cache entries, advancing a timestamp cursor guarded by
`SparseRetriever.mu`. No writer has to know the retriever exists, so a new
write path cannot forget to invalidate.

## 6. CLI surface

`cmd/nerd/cmd_retrieve.go` (`retrieveCmd`, `nerd retrieve`): runs one pass, loads
the facts into a real kernel, and reads them back for display. Flags: `--facts`,
`--stats`, `--ripgrep`, `--workspace`, `--timeout`, `--max-files`.

## 7. What is still **not** wired

| Integration | Status |
|-------------|--------|
| VirtualStore action `search_code` / similar | absent |
| Session clean-loop executor hooks | absent |
| Prompt atoms calling retrieval | absent (facts only) |
| Campaign assault automatic sparse pass | not via this package import |
| Embedding engine into T4 | injection point exists (`SemanticSearcher`); nothing constructs one |
| Mangle rules over `candidate_file` / `keyword_hit` | Decls exist, no rules yet |

## 8. Fact-flow placement

```
user input
  → perception Intent (verb)
  → seedIssueFacts  [RETRIEVAL TOUCHPOINT — live]
  → kernel EDB
  → (orient) context activation / JIT
  → decide next_action
  → VirtualStore act
  → articulation
```

Retrieval sits on the **Observe/Orient edge**, not Act, and remains read-only:
file *edits* still go through the constitutional tool path.
