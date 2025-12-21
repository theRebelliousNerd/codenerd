# cmd/query-kb - Knowledge Base Query Tool

This tool queries and inspects shard knowledge databases stored in `.nerd/shards/*.db`.

## Usage

```bash
# List all knowledge DBs and sample from each
go run ./cmd/query-kb

# Query specific database
go run ./cmd/query-kb .nerd/shards/bubbleteaexpert_knowledge.db

# Deep query with vectors/atoms display
go run deep_query.go <database.db> [--vectors|--atoms]
```

## File Index

| File | Description |
|------|-------------|
| `main.go` | Knowledge database browser listing tables and sampling rows from `.nerd/shards/*.db`. Exports no API; standalone CLI tool that scans for shard knowledge DBs and displays knowledge_atoms table contents. |
| `deep_query.go` | Deep query tool displaying full atom content and vector embeddings with optional filtering. Exports no API; build-tagged tool showing knowledge_atoms (ID/concept/content/confidence) and vector embeddings. |

## Database Schema

Queries the standard shard knowledge database schema:
- `knowledge_atoms` - ID, concept, content, confidence
- Vector tables for semantic search

## Dependencies

- `modernc.org/sqlite` - Pure Go SQLite driver

## Building

```bash
go run ./cmd/query-kb
go run deep_query.go .nerd/shards/researcher_knowledge.db --atoms
```
