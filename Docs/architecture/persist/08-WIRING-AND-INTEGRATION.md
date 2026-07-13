# 08 — Wiring and Integration: persist

> Last verified against codebase: **2026-07-13**  
> Status: **library complete, production wiring absent**

## 1. Registration hooks

| Hook type | Present? |
|-----------|----------|
| `init()` registration | No |
| Shard registration | No |
| VirtualStore route | No |
| CLI `AddCommand` | No |
| Config key | No |
| Boot step in `system` / `session_boot` | No |
| Mangle Decl / policy | No |

factsnap is a **import-and-call** library with no self-registration.

## 2. Current call graph

```
go test ./internal/persist/...
        │
        └── factsnap tests ──► Write / Read / LegacyJSON / CanonicalPath
```

No path from `cmd/nerd` main, chat boot, or kernel cycle reaches factsnap.

## 3. Fact-flow position (intended)

```
user_intent → kernel → next_action → VirtualStore → articulation
                   │
                   │  optional side channel (checkpoint / export)
                   ▼
            select []types.Fact
                   │
                   ▼
            factsnap.WriteCodec  →  .nerd/.../*.sc.gz|zst
```

Import path (restore):

```
operator / tool  →  factsnap.Read  →  []types.Fact
                         │
                         ▼
              validate + permitted-gated Assert
                         │
                         ▼
                    kernel EDB
```

## 4. Candidate wiring sites (research notes only)

These are **locations that already persist JSON-like artifacts** and could reasonably call factsnap. They are **not** currently integrated.

| Site | Why candidate | Current format |
|------|---------------|----------------|
| `internal/campaign` assault artifacts | Large structured results under `.nerd/campaigns/...` | JSON / JSONL |
| `internal/world` scan / code element facts | `ToFacts()` already produces fact slices | In-memory / store |
| Kernel debug dumps | Support for large EDB slices | ad-hoc / `debug_program_ERROR.mg` style dumps elsewhere |
| CLI export verb | Operator discoverability | none |

**Do not invent wiring in docs as if present.** When a real importer lands, update this section with the exact file and function.

## 5. Wiring anti-patterns

| Anti-pattern | Why |
|--------------|-----|
| Delete package for “unused” | Tests + design intent exist; violates wiring-before-deletion |
| Import core from factsnap | Cycle risk |
| Assert snapshot without policy | Bypasses constitutional safety |
| Write without canonical extension | Breaks `Read` auto-detect |
| Use factsnap for non-fact blobs | Wrong abstraction |

## 6. How to wire safely (guidance)

1. Convert domain objects → `[]types.Fact` (many already have `ToFacts()`).  
2. Choose codec (gzip default; zstd for large dumps).  
3. Write under workspace `.nerd/` with explicit name.  
4. On load: `Read` → optional validate → assert only through existing kernel/policy APIs.  
5. Add an **integration test** at the caller package, not only factsnap unit tests.

## 7. Related “persist” words that are **not** this package

Grep hits for `persist` / `snapshot` in browser, campaign, autopoiesis refer to **other** persistence mechanisms. Do not conflate:

- `SessionManager.persistSessions` (browser metadata JSON)  
- Campaign assault JSONL results  
- Autopoiesis “persistent agent” (logical agent lifecycle)

Only `internal/persist/factsnap` is this corpus’s subject.
