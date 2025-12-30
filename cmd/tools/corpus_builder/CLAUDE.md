# cmd/tools/corpus_builder - Intent Classification DB Builder

This tool builds the baked-in vector database for intent classification by parsing `.mg` files and generating embeddings for semantic search.

## Usage

```bash
# Build with sqlite-vec support
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go run -tags=sqlite_vec ./cmd/tools/corpus_builder
```

## File Index

| File | Description |
|------|-------------|
| `main.go` | Corpus builder parsing all .mg files in internal/core/defaults/ and extracting DATA facts for embedding generation. Outputs `intent_corpus.db` with CorpusEntry records (Predicate/TextContent/Verb/Target/Category) and 3072-dimensional embeddings. |
| `sqlite_vec.go` | SQLite-vec support for vector storage in the corpus database. Handles binary vector encoding and vec0 virtual table creation with build tag gating. |

## Output

Creates `internal/core/defaults/intent_corpus.db` with:
- Intent facts extracted from DATA directives
- 3072-dimensional embeddings (Gemini embedding-001)
- Verb/Target/Category classification metadata

## Pipeline

1. Load API key (GEMINI_API_KEY or .nerd/config.json)
2. Create embedding engine (3072 dimensions)
3. Parse .mg files, extract DATA facts
4. Generate embeddings in batches (32 entries)
5. Store in SQLite with vec0 virtual table

## Dependencies

- `internal/core` - .mg file parsing
- `internal/embedding` - Embedding engine
- `github.com/mattn/go-sqlite3` - SQLite with CGO

## Building

Requires CGO_CFLAGS for sqlite-vec headers.

---

**Remember: Push to GitHub regularly!**
