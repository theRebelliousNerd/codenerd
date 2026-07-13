# 01 — Vision: `internal/diff`

> Last verified against codebase: 2026-07-13  
> Status: Target product/architecture vision grounded in living code

## 1. Purpose

Provide a **single, trustworthy, high-assurance text-diff service** for codeNERD so that:

- The interactive agent can show proposed file mutations as reviewable hunks.  
- Word-level highlighting can refine remove/add pairs.  
- Binary and pathological inputs cannot crash or hang the approval UX.  
- Callers never reimplement Myers / LCS ad hoc.

## 2. Product outcomes

| Outcome | Vision | Reality today |
|---------|--------|---------------|
| One structured model | `FileDiff` is the lingua franca for in-process diffs | **Met** |
| Safe by default | Binary + timeout + context clamp | **Mostly met** |
| Cheap repeated work | Content-keyed cache | **Met** with unbounded growth risk |
| UI-agnostic core | No Bubble Tea / Lipgloss imports | **Met** |
| Reviewable apply pipeline | Diff → human/policy → write | Consumer-owned; package stays pure |
| Multi-file batches | Optional `[]FileDiff` helpers | **Not in package** (UI owns mutation lists) |
| Git interoperability | Parse/emit standard patches | **Out of scope** unless a future consumer needs it |

## 3. Architectural vision

```
┌─────────────────────────────────────────────────────────┐
│  internal/diff (library kernel of text comparison)      │
│  ┌──────────┐  ┌────────────┐  ┌─────────────────────┐  │
│  │  Engine  │──│ sergi dmp  │──│ FileDiff/Hunk/Line  │  │
│  └────┬─────┘  └────────────┘  └─────────────────────┘  │
│       │ cache (bounded in vision; unbounded today)      │
└───────┼─────────────────────────────────────────────────┘
        │
        ├── cmd/nerd/ui DiffApprovalView
        ├── (future) CLI non-interactive patch preview
        └── (future) campaign/review artifacts as structured JSON
```

### Design constraints (vision)

1. **No I/O** — callers supply strings; package never reads disk.  
2. **No policy** — permission remains in Mangle / VirtualStore.  
3. **Stable types** — avoid thrashing `FileDiff` field names; UI aliases depend on them.  
4. **Bounded cost** — timeout, context clamp, binary gate stay mandatory.  
5. **Optional isolation** — prefer `NewEngine()` over `DefaultEngine` when cache isolation matters.

## 4. Non-goals

- Replacing `git diff` for VCS workflows.  
- Semantic / AST-aware diffs (tree-sitter, etc.).  
- LLM-authored natural-language “summaries of changes” (articulation’s job).  
- sibling-platform product concepts (foreign-product-surface, foreign-agent-kit, etc.).  
- Embedding fuzzy matching (use retrieval packages first; then structured facts).

## 5. Success metrics (engineering)

| Metric | Target |
|--------|--------|
| Test package green | `go test ./internal/diff/...` always |
| Race clean on concurrent compute | `go test -race` |
| Binary never produces hunk spam | `IsBinary` + empty hunks |
| Cache does not OOM long sessions | Bounded or TTL cache (gap today) |
| UI can render without panics | Contract with `diffview.go` |

## 6. Evolution path

1. **Harden cache** — deep clone on hit; size cap / LRU; optional content verification.  
2. **API polish** — optional `ComputeDiffOptions{ContextLines, DisableCache}`.  
3. **Decouple word-level** — return internal spans instead of exporting sergi `Diff` types to UI.  
4. **Observability** — optional debug counters (hits, binary skips, timeouts).  
5. **Only if needed** — unified-diff serialize for logs/artifacts (keep out of hot path until demanded).
