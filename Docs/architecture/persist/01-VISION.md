# 01 — Vision: persist / factsnap

> Last verified against codebase: **2026-07-13**  
> Mode: target architecture (grounded in existing API)

## 1. Product role

**Vision:** codeNERD can freeze and restore *slices of logical reality* — selected EDB facts — as small, portable files under `.nerd/`, without round-tripping through lossy JSON dumps or full sqlite copies.

Typical intended products of that capability:

| Scenario | Snapshot content | Consumer |
|----------|------------------|----------|
| World-model checkpoint | `code_*`, `file_*` index facts | Resume scan without full reparse |
| Campaign phase freeze | `campaign_*`, task status facts | Assault / multi-day goals |
| Dream / what-if branch export | Projected / hypothetical facts | Shadow evaluation |
| Debug dump | Predicate slice from kernel query | Support / `debug_program`-adjacent tooling |
| Cross-machine handoff | Fact bag without full workspace DB | Offline review |

## 2. Architectural placement

```
┌─────────────────────────────────────────────────────────┐
│  Executive plane                                         │
│  core.Kernel / mangle.Engine / store.LocalStore          │
│         │ export selected facts                          │
│         ▼                                                │
│  []types.Fact  ──►  factsnap.WriteCodec  ──►  *.sc.zst   │
│         ▲                        │                       │
│         └──── factsnap.Read ◄────┘                       │
│                (then Assert under policy)                │
└─────────────────────────────────────────────────────────┘

LLM / prompt / JIT  ── never call factsnap ──
```

Principles of the vision:

1. **Facts, not transcripts.** Snapshots are atoms, not chat logs.  
2. **Codec choice is operational**, not semantic (gzip vs zstd does not change meaning — proven by `TestCodecParity`).  
3. **Policy on rehydrate, not on bytes.** Files on disk are not trusted; asserting into the kernel still requires `permitted` / validation paths.  
4. **Complement, don’t replace, `internal/store`.** sqlite remains the interactive cold store; factsnap is **portable slice export**.

## 3. Non-goals

- Becoming a general object store or blob FS.  
- Versioned multi-writer concurrent databases.  
- Replacing campaign JSONL result streams (those are *event* logs, not EDB slices).  
- Embedding-model vector dumps (different shape; see `internal/embedding`).  
- Any Vectryx-product-specific storage story.

## 4. Success criteria (future)

| Criterion | Signal |
|-----------|--------|
| At least one production caller | Import from campaign, world, core dump, or CLI |
| Round-trip in integration test | Export kernel query → write → read → re-assert → query equalish |
| Operator discoverability | Documented path under `.nerd/` + optional CLI verb |
| No silent format drift | Keep Deterministic SimpleColumn + parity tests green |

## 5. Relationship to current code

The vision is **already half-built**: write/read/codec/legacy/tests match the desired library. Vision work remaining is **wiring and productization**, not inventing a new format.
