# store — Current State

> Last verified: **2026-07-13**  
> Source root: `internal/store/`

## Scale

| Kind | Count (approx) |
|------|---------------:|
| Non-test `.go` | ~39 |
| Test `.go` | ~44 |
| `.mg` | 0 |
| Build-tag variants | `cgo`, `sqlite_vec && cgo`, defaults |

## File inventory by subsystem

### Core LocalStore

| File | Role |
|------|------|
| `local.go` | Modular map comment only |
| `local_core.go` | Struct, ctor, schema, stats, vec probe |
| `local_cold.go` | Cold + archival + maintenance |
| `local_graph.go` | Graph CRUD / traverse / hydrate |
| `local_graph_query.go` | `LocalStoreGraphAdapter` |
| `local_vector.go` | Keyword vector path |
| `local_session.go` | Session, activation, compression |
| `local_world.go` | World model cache |
| `local_knowledge.go` | Knowledge atoms + semantic bridge |
| `local_prompt.go` | Prompt atom CRUD |
| `local_review.go` | Review findings |
| `local_verification.go` | Verifications + trace facades |

### Vector / embedding extensions

| File | Role |
|------|------|
| `vector_store.go` | Engine attach, store/batch, semantic recall |
| `vector_store_bruteforce.go` | Brute cosine paths |
| `vector_store_reembed.go` | Re-embed helpers |
| `vector_utils.go` | Encoding helpers |
| `prompt_reembed.go` | Prompt atom re-embed |
| `reembed_all.go` | Multi-DB force re-embed |
| `reflection_worker.go` | Async reflection cycle |
| `reflection_search.go` | Trace/learning recall |
| `reflection_utils.go` | Shared helpers |
| `reflection_reembed.go` | Reflection re-embed |
| `trace_reflection.go` | Trace embedding backlog |
| `learning_reflection.go` | Learning embedding backlog |

### Satellite stores

| File | Role |
|------|------|
| `trace_store.go` | Reasoning traces |
| `learning.go` | Per-shard learnings |
| `learning_candidates.go` | Taxonomy candidates in LocalStore |
| `tool_store.go` | Tool executions |
| `tool_cleanup.go` | Retention / smart cleanup |
| `embedded_store.go` | RO intent corpus |
| `learned_store.go` | RW learned patterns |

### Infrastructure

| File | Role |
|------|------|
| `migrations.go` | Additive + versioned migrations |
| `indexes.go` | Conditional indexes |
| `pragmas.go` | Re-export sqlpragmas |
| `fact_codec.go` | Fact arg encode/decode |
| `init_sqlite.go` | Driver import |
| `init_vec.go` | Register sqlite-vec (cgo) |
| `vec_support_enabled.go` / `vec_support_disabled.go` | `defaultRequireVec` |

### Tests (representative)

| Pattern | Coverage |
|---------|----------|
| `*_test.go` unit | codec, migrations, tools, vectors, reflection, learnings |
| `*_integration_test.go` | cold, graph, session, traces |
| `*_benchmark_test.go` | graph, knowledge, migrations, reembed, vector |
| `vector_e2e_test.go` / `vector_boundary_test.go` | end-to-end / edge vectors |
| `archival_test.go` | archival logic |
| `mocks_test.go` | mock embedding engines |

## Hotspots

1. **`vector_store.go`** — most complex fallback ladder and ANN drift care.
2. **`local_core.go` initialize** — schema surface area; migration order sensitivity.
3. **`reflection_worker.go`** — background concurrency vs single-writer DB.
4. **`migrations.go`** — long-lived DBs; must remain additive-safe.
5. **`system/factory.go` (external)** — construction order of store + embedding + reflection.

## On-disk artifacts (runtime)

| Path | Store |
|------|-------|
| `.nerd/knowledge.db` | `LocalStore` |
| `.nerd/shards/<type>_learnings.db` | `LearningStore` |
| `.nerd/tools.db` | `ToolStore` |
| Temp `codenerd-intent-corpus-*.db` | `EmbeddedCorpusStore` extract |
| Configurable learned DB path | `LearnedCorpusStore` |

## Status summary

Package is **production-complete** for multi-tier local memory. Remaining work is operational polish and consumer wiring clarity — not greenfield implementation.
