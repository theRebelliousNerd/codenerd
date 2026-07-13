# retrieval — Dependency Map

> Last verified: **2026-07-13**  
> Evidence: imports in package sources + reverse grep for `codenerd/internal/retrieval`

## 1. Upstream (what retrieval imports)

```
internal/retrieval
  ├─ standard library
  │    context, sync, os, path/filepath, regexp, bytes, sort, ...
  ├─ codenerd/internal/logging   (logging.Context convenience)
  └─ simd/archsimd               (scanner_amd64.go only, build tag)
```

**No** imports of: `core`, `store`, `embedding`, `perception`, `session`, `prompt`, `mangle`.

## 2. Downstream (who imports retrieval)

### Production Go (non-test)

| Importer | Symbols used |
|----------|--------------|
| `cmd/nerd/chat/process_seed.go` | `ExtractKeywords` |
| `cmd/nerd/chat/session_boot.go` | `DefaultSparseRetrieverConfig`, `NewSparseRetriever` |
| `cmd/nerd/chat/session_shared_boot.go` | same |
| `cmd/nerd/chat/model_types.go` | type `*retrieval.SparseRetriever` field |

### Test

| Importer | Notes |
|----------|-------|
| `internal/retrieval/*_test.go` | same package or `retrieval_test` for integration |

### Not importers (despite related schemas)

- `internal/session` — clean loop does not import retrieval  
- `internal/context` — consumes **facts**, not this package  
- `internal/embedding` — peer for future T4  
- `internal/tools` — no search tool wrapper found  

Verify reverse deps:

```powershell
rg "codenerd/internal/retrieval" -g "*.go" --glob "!*_test.go"
```

## 3. Logical (fact) dependencies

```
retrieval extract/search
        │ (caller asserts)
        ▼
  core.Kernel EDB
        │
        ▼
  internal/context (compressor / activation priorities)
        │
        ▼
  prompt JIT / articulation context
```

Schema authority: `internal/core/defaults/schemas_knowledge.mg`.

## 4. Dependency rules for evolution

| Change | Allowed? |
|--------|----------|
| retrieval → logging | yes |
| retrieval → embedding | discouraged inside package; prefer caller injection |
| retrieval → core/kernel | **no** (breaks purity / testability) |
| chat → retrieval | yes (current) |
| context → retrieval types | optional future; today fact-only |

## 5. Optional SIMD edge

Building with `-tags=simd` on amd64 pulls `simd/archsimd`. Default Windows/CI builds use generic scanner — **no** simd module required for normal `go test ./internal/retrieval`.

## 6. Diagram (boot + seed)

```
session_boot / session_shared_boot
        │ NewSparseRetriever(workspace)
        ▼
   Model.Retriever  ──(no method calls observed)──► ∅
        
process_seed (fix|debug|review|security)
        │ ExtractKeywords
        ▼
   kernel.LoadFacts(issue_*)
        ▼
   context activation (later turns)
```
