# store — Dependency Map

> Last verified: **2026-07-13**  
> Package: `internal/store/`

## Upstream (store imports)

| Package | Why |
|---------|-----|
| `database/sql` + `github.com/mattn/go-sqlite3` | SQLite driver |
| `github.com/asg017/sqlite-vec-go-bindings/cgo` | Optional ANN (`init_vec.go`, cgo) |
| `codenerd/internal/embedding` | Embed engines / task types |
| `codenerd/internal/config` | Reflection config, workspace root for learnings default path |
| `codenerd/internal/logging` | CategoryStore |
| `codenerd/internal/types` | `MangleAtom`, `ShardLearning` |
| `codenerd/internal/sqlpragmas` | Pragma profiles (leaf) |
| `codenerd/internal/core/defaults` | Embedded intent corpus FS |

**Does not import:** `internal/core` (kernel), `internal/session`, `internal/perception` (avoids cycles; adapters live in `system` / local mirrors).

## Downstream (who imports store)

Evidence from repo greps (`codenerd/internal/store`):

| Consumer | Usage sketch |
|----------|--------------|
| `internal/system` | Construct LocalStore, LearningStore; SetEmbeddingEngine; graph adapter; trace adapter |
| `internal/core` | VirtualStore fields `localDB`, `learningStore`; tools helpers |
| `internal/world` | World file/fact persistence for scans |
| `internal/prompt` | Compiler / predicate selector DB access |
| `internal/init` | Shared KB, strategic knowledge/docs, agents registration, validation |
| `internal/perception` | Taxonomy store, semantic classifier |
| `internal/campaign` | Specialist knowledge, intelligence gatherer, document ingestor, decomposer |
| `internal/context` | Compressor session/state |
| `internal/verification` | Verification history |
| `internal/shards/system` | Base shard store access |
| `internal/testing/context_harness` | Real engine harness |
| `cmd/nerd` (indirect via system) | Runtime |
| `cmd/query-kb` | KB query CLI |
| `cmd/tools/prompt_builder`, `predicate_corpus_builder` | Corpus tooling |

## Dependency direction diagram

```
sqlpragmas  embedding  config  logging  types  defaults
     \         |         |        |       |       |
      \        +----+----+--------+-------+-------+
       \            |
        v           v
              internal/store
                    |
    +-------+-------+-------+--------+--------+
    |       |       |       |        |        |
 system   core    world   prompt   init   campaign
    |       |       |       |        |        |
    +-------+-------+-------+--------+--------+
                    |
                 cmd/nerd  (boot)
```

## Cycle-break patterns

| Risk | Mitigation |
|------|------------|
| store ↔ core | VirtualStore holds store pointers; store never imports core |
| store ↔ perception | Local `ReasoningTrace` mirror; `StoreReasoningTrace(any)`; adapter in `system/factory_adapters.go` |
| store ↔ mcp pragmas | mcp imports `sqlpragmas` leaf, not store |

## Sibling packages (not deps, related)

| Package | Relation |
|---------|----------|
| `internal/persist` | Factsnap helpers — adjacent persistence, separate package |
| `internal/sqlpragmas` | Leaf pragma implementation store re-exports |
| `internal/embedding` | Pure engines; store is consumer |
