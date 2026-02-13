# cmd/tools/prompt_builder - Prompt Corpus DB Builder

This tool builds the baked-in prompt corpus database for JIT prompt compilation by parsing YAML atom definitions and generating embeddings for semantic search.

## Usage

```bash
# Build with sqlite-vec support
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go run -tags=sqlite_vec ./cmd/tools/prompt_builder
```

## File Index

| File | Description |
|------|-------------|
| `main.go` | Prompt corpus builder parsing YAML atom definitions from internal/prompt/atoms/ with embedding generation. Outputs prompt_corpus.db with AtomDefinition records (ID/Category/Priority/Selectors) and 3072-dimensional embeddings for semantic JIT selection. |
| `sqlite_vec.go` | SQLite-vec support for vector storage in the prompt corpus database. Handles binary vector encoding and vec0 virtual table creation with build tag gating. |

## Output

Creates prompt corpus database with:
- Atom metadata (ID, category, priority, mandatory flag)
- 11-dimension contextual selectors
- Polymorphic content variants (standard, concise, min)
- 3072-dimensional embeddings for semantic search
- Content hashes for cache invalidation

## Atom Definition Schema

```yaml
id: "identity/coder"
category: "identity"
priority: 100
is_mandatory: true
operational_modes: ["/active"]
shard_types: ["/coder"]
content: |
  You are a code-generating agent...
```

## Flags

| Flag | Description |
|------|-------------|
| `-input` | Input directory (default: internal/prompt/atoms) |
| `-output` | Output database path |
| `-skip-embeddings` | Skip embedding generation |

## Dependencies

- `internal/embedding` - Embedding engine
- `github.com/mattn/go-sqlite3` - SQLite with CGO
- `gopkg.in/yaml.v3` - YAML parsing

## Building

Requires CGO_CFLAGS for sqlite-vec headers.

---

**Remember: Push to GitHub regularly!**


> *[Archived & Reviewed by The Librarian on 2026-01-25]*