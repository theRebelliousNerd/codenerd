# cmd/tools/predicate_corpus_builder - Predicate Corpus DB Builder

This tool builds the baked-in predicate corpus database containing all Mangle predicate signatures, types, categories, and usage examples for schema validation and JIT predicate selection.

## Usage

```bash
go run ./cmd/tools/predicate_corpus_builder
```

## File Index

| File | Description |
|------|-------------|
| `main.go` | Predicate corpus builder extracting all Mangle predicates from schemas.mg and policy/*.mg with signatures, types, categories, and domains. Outputs `predicate_corpus.db` with PredicateEntry records (Name/Arity/Type/Category/SafetyLevel/Domain/ArgumentDefs) and error patterns. |

## Output

Creates `internal/core/defaults/predicate_corpus.db` with:
- Predicate definitions (EDB and IDB)
- Argument specifications with types
- Safety levels (safe, requires_binding, negation_sensitive)
- Domain classification for JIT selection
- Error patterns for repair guidance
- Example usage patterns (correct and anti-patterns)

## Sources

| Source | Content |
|--------|---------|
| `internal/core/defaults/schemas.mg` | EDB predicate declarations |
| `internal/core/defaults/policy/*.mg` | IDB rule definitions |
| `.claude/skills/mangle-programming/references/` | Error patterns and examples |

## Key Types

```go
type PredicateEntry struct {
    Name               string
    Arity              int
    Type               string // "EDB" or "IDB"
    Category           string // core, shard, campaign, safety
    SafetyLevel        string
    Domain             string
    ArgumentDefs       []ArgumentDef
    ActivationPriority int
}
```

## Dependencies

- `github.com/mattn/go-sqlite3` - SQLite (no CGO flags needed)

## Building

```bash
go run ./cmd/tools/predicate_corpus_builder
```

---

**Remember: Push to GitHub regularly!**
