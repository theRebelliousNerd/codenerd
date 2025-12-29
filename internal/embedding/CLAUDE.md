# internal/embedding - Vector Embedding Generation

This package provides vector embedding generation for semantic search, supporting multiple backends: Ollama (local) and Google GenAI (cloud).

**Related Packages:**
- [internal/prompt](../prompt/CLAUDE.md) - JIT compiler using embeddings for atom selection
- [internal/store](../store/CLAUDE.md) - Vector store consuming embeddings
- [internal/shards/researcher](../shards/researcher/CLAUDE.md) - Research using embeddings for similarity

## Architecture

The embedding package provides:
- **EmbeddingEngine interface**: Unified API across providers
- **Ollama backend**: Local embedding via HTTP API (embeddinggemma)
- **GenAI backend**: Google's Gemini API with 100-batch limit
- **TaskType selection**: Intelligent task type based on content

## File Index

| File | Description |
|------|-------------|
| `engine.go` | Core `EmbeddingEngine` interface and factory with provider selection. Exports `EmbeddingEngine` (Embed, EmbedBatch, Dimensions, Name), `Config`, `DefaultConfig()`, and `NewEngine()` factory routing to Ollama or GenAI. |
| `genai.go` | Google GenAI embedding engine using Gemini API with 100-batch limit. Exports `GenAIEngine`, `NewGenAIEngine()` with API key, model, and task type configuration, plus retry logic for transient failures. |
| `ollama.go` | Ollama local embedding engine via HTTP API at localhost:11434. Exports `OllamaEngine`, `NewOllamaEngine()` with endpoint and model configuration, and 3-retry backoff for runner failures. |
| `task_selector.go` | Intelligent TaskType selection based on content type for optimal GenAI embeddings. Exports `ContentType` enum (11 types), `SelectTaskType()` returning task types like RETRIEVAL_QUERY, CODE_RETRIEVAL_QUERY, SEMANTIC_SIMILARITY. |
| `task_selector_test.go` | Unit tests for content type detection and task type selection. Tests SelectTaskType() with various content configurations. |

## Key Types

### EmbeddingEngine
```go
type EmbeddingEngine interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
    Name() string
}
```

### Config
```go
type Config struct {
    Provider       string // "ollama" or "genai"
    OllamaEndpoint string // Default: "http://localhost:11434"
    OllamaModel    string // Default: "embeddinggemma"
    GenAIAPIKey    string
    GenAIModel     string // Default: "gemini-embedding-001"
    TaskType       string // SEMANTIC_SIMILARITY, RETRIEVAL_QUERY, etc.
}
```

### HealthChecker
```go
// Optional interface for pre-flight availability checks
type HealthChecker interface {
    HealthCheck(ctx context.Context) error
}
```

## Health Check Pattern (BUG-001 Fix)

The `HealthChecker` interface enables fast-fail when embedding services are unavailable.
Without this, batch embedding operations on 779+ atoms with 3 retries each can block
boot for 35+ minutes.

**Usage in factory.go:**
```go
if checker, ok := engine.(embedding.HealthChecker); ok {
    if err := checker.HealthCheck(ctx); err != nil {
        // Graceful degradation - continue without embeddings
        embeddingEngine = nil
    }
}
```

**WSL2 Note:** Ollama running on Windows requires `OLLAMA_HOST=0.0.0.0` to be
reachable from WSL2, since localhost in WSL2 refers to the Linux VM.

## Content Types

| Type | Task Type |
|------|-----------|
| code | CODE_RETRIEVAL_QUERY / RETRIEVAL_DOCUMENT |
| query | RETRIEVAL_QUERY |
| question | QUESTION_ANSWERING |
| documentation | RETRIEVAL_DOCUMENT |
| fact | FACT_VERIFICATION |
| classification | CLASSIFICATION |
| clustering | CLUSTERING |

## Dependencies

- `google.golang.org/genai` - Google GenAI SDK
- `internal/logging` - Structured logging

## Testing

```bash
go test ./internal/embedding/...
```

---

**Remember: Push to GitHub regularly!**
