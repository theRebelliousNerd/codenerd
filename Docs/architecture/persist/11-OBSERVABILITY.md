# 11 — Observability: persist / factsnap

> Last verified against codebase: **2026-07-13**

## 1. Current state

| Signal | Present? |
|--------|----------|
| `internal/logging` categories | **No** |
| Metrics / counters | **No** |
| OpenTelemetry / tracing | **No** |
| Structured audit events | **No** |
| Debug dump hooks | **No** (package *is* a dump format) |
| Verbose test logs | **Yes** — `TestSizeComparison` logs byte sizes |

Production code surfaces problems only via returned `error` values with `factsnap:` prefixes.

## 2. Implications

Operators cannot see:

- How long a 100k-fact export took  
- Snapshot byte size in production  
- Codec chosen when Auto is used  
- Failures except where callers log the error

Because **no production caller exists**, the lack of observability has not yet hurt ops — but the first integration should log at the **caller** or add thin logging inside factsnap.

## 3. Recommended logging (future, not implemented)

If adding observability, prefer existing categories rather than inventing a parallel system:

| Event | Suggested category | Fields |
|-------|--------------------|--------|
| Write start/end | `CategoryStore` or new `CategoryPersist` | path, codec, fact_count, bytes, duration_ms |
| Read start/end | same | path, codec, fact_count, duration_ms |
| ToAtom failure | same | index, predicate |
| Rename failure | same | tmp, path |

Avoid logging full fact contents (may contain secrets / source).

## 4. Debug workflow today

```powershell
go test ./internal/persist/factsnap/ -v -run TestSizeComparison
```

Inspect temporary files during development by calling `Write`/`Read` from a scratch main or test.

For format debugging, decode with the same mangle-go SimpleColumn loaders the package uses.

## 5. Glass box / transparency

No integration with `internal/transparency` or glass-box UI. Snapshots are opaque binary+compression files; operators inspect via `Read` + pretty-print facts, not via TUI panes.

## 6. Observability gap score

**1 / 5** — acceptable for an unwired library; insufficient once wired into long-horizon campaigns.
