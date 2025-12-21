# internal/store - Persistence Layer & Memory Tiers

This package implements codeNERD's multi-tier persistence system using SQLite. It provides four memory shards: Vector (semantic search), Graph (relational links), Cold Storage (facts), and Session (conversation history).

**Related Packages:**
- [internal/core](../core/CLAUDE.md) - Kernel consuming stored facts
- [internal/embedding](../embedding/CLAUDE.md) - Embedding engines for vector ops
- [internal/shards](../shards/CLAUDE.md) - Shards using LearningStore

## Architecture

The store package implements four memory tiers:
- **Shard B (Vector)**: Semantic search with sqlite-vec support
- **Shard C (Graph)**: Knowledge graph edges and relationships
- **Shard D (Cold)**: Persistent facts with access tracking and archival
- **Session**: Conversation turns, activation logs, compressed state

## File Index

| File | Description |
|------|-------------|
| `local.go` | Package marker documenting LocalStore modularization across 10 files. Points to local_core, local_world, local_vector, local_graph, local_cold, local_session, local_verification, local_knowledge, local_prompt, local_review. |
| `local_core.go` | Core LocalStore struct and initialization with SQLite connection pooling. Exports NewLocalStore() creating database schema and managing thread-safe operations. |
| `local_vector.go` | Vector store operations (Shard B) with keyword-based semantic search fallback. Exports VectorEntry, StoreVector(), VectorRecall() for content retrieval by similarity. |
| `local_graph.go` | Knowledge graph operations (Shard C) storing entity relationships. Exports KnowledgeLink, StoreLink(), QueryLinks() for graph traversal by direction. |
| `local_cold.go` | Cold storage and archival tier (Shard D) with access tracking. Exports StoredFact, ArchivedFact, StoreFact(), LoadFacts(), MaintenanceCleanup() for fact lifecycle. |
| `local_session.go` | Session management for conversation turns and activation logs. Exports LogActivation(), GetRecentActivations(), StoreSessionTurn() for session persistence. |
| `local_world.go` | World model cache storing file metadata and AST facts. Exports WorldFileMeta, WorldFactInput, UpsertWorldFile(), ReplaceWorldFactsForFile() for incremental scanning. |
| `local_knowledge.go` | Knowledge atoms for agent knowledge bases used by Type 3 agents. Exports KnowledgeAtom, KnowledgeStore, StoreKnowledgeAtom(), GetKnowledgeAtoms() for domain expertise. |
| `local_prompt.go` | Prompt atoms for JIT Prompt Compiler with contextual selectors. Exports PromptAtom with 11 selector dimensions and StorePromptAtom(), QueryPromptAtoms() for compilation. |
| `local_review.go` | Review findings storage for code review persistence. Exports StoredReviewFinding and StoreReviewFinding() for review result archival. |
| `local_verification.go` | Verification records and reasoning traces for learning. Exports VerificationRecord, StoreVerification() for tracking verification attempts and outcomes. |
| `learning.go` | LearningStore for Autopoiesis shard learnings with per-shard SQLite databases. Exports Learning, LearningStore with Save(), Load(), LoadByPredicate(), DecayConfidence() methods. |
| `vector_store.go` | Extended vector operations with real embedding support via embedding engine. Exports SetEmbeddingEngine(), StoreVectorWithEmbedding(), VectorRecallWithEmbedding() for ANN search. |
| `trace_store.go` | TraceStore for reasoning traces with self-learning capabilities. Exports ReasoningTrace, NewTraceStore(), StoreTrace(), QueryTraces() for LLM interaction persistence. |
| `embedded_store.go` | Read-only access to baked-in intent corpus from go:embed. Exports EmbeddedCorpusStore, SemanticMatch, NewEmbeddedCorpusStore(), SemanticSearch() for intent classification. |
| `learned_store.go` | Writable store for dynamically learned intent patterns. Exports LearnedPattern, LearnedCorpusStore, LearnPattern(), SemanticSearch() for runtime learning. |
| `migrations.go` | Versioned schema migration system for database upgrades. Exports MigrationResult, MigrateKnowledgeDB(), MigrateSchema() handling v1→v4 schema evolution. |
| `init_sqlite.go` | SQLite driver import for mattn/go-sqlite3 registration. Provides side-effect import for database/sql driver availability. |
| `init_vec.go` | sqlite-vec extension registration when built with sqlite_vec tag. Auto-registers vector extension via vec.Auto() for ANN support. |
| `vec_support_enabled.go` | Build constraint setting defaultRequireVec=true when sqlite_vec enabled. Requires vector extension presence at runtime. |
| `vec_support_disabled.go` | Build constraint setting defaultRequireVec=false for default builds. Treats sqlite-vec as optional for graceful degradation. |
| `prompt_reembed.go` | Force re-embedding for prompt_atoms when switching providers. Exports ReembedAllPromptAtomsForce() regenerating all prompt embeddings. |
| `reembed_all.go` | Batch re-embedding across all *.db files in workspace. Exports ReembedResult, ReembedAllDBsForce() for workspace-wide embedding refresh. |
| `trace_store_test.go` | Unit tests for TraceStore persistence and query operations. Tests StoreTrace(), QueryTraces(), and schema creation. |
| `archival_test.go` | Unit tests for cold storage archival and restoration. Tests MaintenanceCleanup(), RestoreArchivedFact(), and access tracking. |

## Key Types

### LocalStore
```go
type LocalStore struct {
    db              *sql.DB
    embeddingEngine embedding.EmbeddingEngine
    vectorExt       bool   // sqlite-vec available
    traceStore      *TraceStore
}

func NewLocalStore(path string) (*LocalStore, error)
func (s *LocalStore) StoreFact(predicate string, args []interface{}, factType string, priority int) error
func (s *LocalStore) StoreVector(content string, metadata map[string]interface{}) error
func (s *LocalStore) VectorRecall(query string, limit int) ([]VectorEntry, error)
```

### LearningStore
```go
type LearningStore struct {
    basePath string
    dbs      map[string]*sql.DB  // One DB per shard type
}

func NewLearningStore(basePath string) (*LearningStore, error)
func (ls *LearningStore) Save(shardType, predicate string, args []any, campaign string) error
func (ls *LearningStore) Load(shardType, predicate string) ([]Learning, error)
```

### PromptAtom
```go
type PromptAtom struct {
    AtomID           string
    Content          string
    Category         string
    OperationalModes []string  // Contextual selectors
    ShardTypes       []string
    IntentVerbs      []string
    Priority         int
    IsMandatory      bool
    Embedding        []byte
}
```

## Storage Tiers

| Tier | Table | Purpose |
|------|-------|---------|
| Vector | `vectors` | Semantic search with embeddings |
| Graph | `knowledge_graph` | Entity relationships |
| Cold | `cold_storage` | Persistent facts with access tracking |
| Archival | `archived_facts` | Old, rarely-accessed facts |
| Session | `session_turns` | Conversation history |
| Activation | `activation_log` | Spreading activation scores |
| Knowledge | `knowledge_atoms` | Agent knowledge bases |
| Prompts | `prompt_atoms` | JIT prompt fragments |
| World | `world_files`, `world_facts` | Filesystem cache |

## Schema Versions

| Version | Description |
|---------|-------------|
| v1 | Basic knowledge_atoms table |
| v2 | Added embedding column |
| v3 | Added vec_index virtual table |
| v4 | Added content_hash for deduplication |

## Dependencies

- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/asg017/sqlite-vec-go-bindings` - Vector extension (optional)
- `internal/embedding` - Embedding engines

## Testing

```bash
go test ./internal/store/...
```
