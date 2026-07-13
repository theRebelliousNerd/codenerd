# retrieval — Wiring and Integration

> Last verified: **2026-07-13**  
> Focus: how (and how not) retrieval joins the live agent loop

## 1. Boot wiring

### Legacy path — `cmd/nerd/chat/session_boot.go`

```
logStep("Initializing sparse retriever...")
retrieverCfg := retrieval.DefaultSparseRetrieverConfig(workspace)
retriever := retrieval.NewSparseRetriever(retrieverCfg)
...
// later in bootComplete / Model assembly:
Retriever: retriever,
```

### Shared path — `cmd/nerd/chat/session_shared_boot.go`

Same construction before/alongside `nerdsystem.BootCortexWithConfig`, assigned into model:

```
Retriever: retriever,
```

Field declaration: `cmd/nerd/chat/model_types.go` → `Retriever *retrieval.SparseRetriever`.

**Critical finding:** grep for `.Retriever` / `m.Retriever` shows **no post-construction method calls**. The instance is retained idle for the session lifetime.

## 2. Issue seed wiring (live)

`cmd/nerd/chat/process_seed.go` → `(*Model).seedIssueFacts`:

**Gate:** kernel non-nil; intent verb ∈ `{/fix, /debug, /review, /security}`; non-empty raw input.

**Steps:**

1. Truncate issue text to 4000 chars  
2. `keywords := retrieval.ExtractKeywords(issueText)`  
3. Build `[]core.Fact`:
   - `issue_text(IssueID, Text)`
   - `issue_keyword(IssueID, Keyword, Weight)` for each weight entry  
   - `file_mentioned(File, IssueID)` for mentioned files  
   - `tiered_context_file(IssueID, File, /tier1, relevance, 0)` for mentioned files (position-decay relevance, floor 0.5)  
4. `m.kernel.LoadFacts(facts)`

**Not done here:** `SearchKeywords`, `FindRelevantFiles`, `BuildContext`, T2–T4 files, `keyword_hit`, `candidate_file`, `issue_context`.

## 3. Schema wiring (kernel)

`internal/core/defaults/schemas_knowledge.mg` Declares the EDB surface under section 52 (keyword extraction / file candidates / tiered context). Retrieval package does not load Mangle; chat/core do.

## 4. Context compressor wiring (consumer)

`internal/context/compressor.go` reads:

- `issue_keyword`, `issue_text`, `file_mentioned`, `tiered_context_file`

to populate issue activation. Without sparse search facts, activation only sees extract-time mentions and keywords (not disk-ranked files).

Priorities also appear in `internal/context/types.go` for those predicates.

## 5. What is **not** wired

| Integration | Status |
|-------------|--------|
| VirtualStore action `search_code` / similar | absent |
| Session clean-loop executor hooks | absent |
| Prompt atoms calling retrieval | absent (facts only) |
| Campaign assault automatic sparse pass | not via this package import |
| Embedding engine into T4 | absent |
| CLI cobra subcommand for sparse search | absent |

## 6. Recommended wiring target (design, not implemented)

```
seedIssueFacts / session observe phase
  keywords = ExtractKeywords(text)
  if m.Retriever != nil:
    ctx, cancel = WithTimeout(parent, searchTimeout)
    tc, err = NewTieredContextBuilder(...).BuildContext(ctx, text)
    // or FindRelevantFiles
  assert full fact set
  LoadFacts
```

Keep **permitted** actions separate: retrieval remains read-only; file *edits* still go through constitutional tool path.

## 7. Registration checklist (for implementers)

- [ ] Use existing `Model.Retriever` or inject into session executor  
- [ ] Bound timeout separately from LLM timeout  
- [ ] Assert only schema-Decl predicates  
- [ ] Deduplicate paths before EDB load  
- [ ] Log tier counts (already available from builder)  
- [ ] Update this corpus when wire lands  

## 8. Fact-flow placement

```
user input
  → perception Intent (verb)
  → seedIssueFacts  [RETRIEVAL TOUCHPOINT]
  → kernel EDB
  → (orient) context activation / JIT
  → decide next_action
  → VirtualStore act
  → articulation
```

Retrieval sits on the **Observe/Orient edge**, not Act.
