# internal/mangle/transpiler - LLM Mangle Sanitizer

This package provides a compiler frontend for LLM-generated Mangle code, fixing common AI hallucinations and syntax errors.

**Related Packages:**
- [internal/mangle](../CLAUDE.md) - Mangle engine consuming sanitized code
- [internal/mangle/feedback](../feedback/CLAUDE.md) - Error classification for repair

## Architecture

The Sanitizer acts as a compiler frontend with multiple passes to fix common LLM errors:
1. **Preprocess**: Fix SQL-style aggregations (`Sum = count(X)` → temp predicate)
2. **Parse**: Convert to AST
3. **Pass 1 - Atom Interning**: `"string"` → `/atom` based on Schema
4. **Pass 2 - Aggregation Repair**: temp_agg → `|> do fn:group_by(...)`
5. **Pass 3 - Safety Injection**: Add variable bindings for negation
6. **Serialize**: Output valid Mangle

## File Index

| File | Description |
|------|-------------|
| `sanitizer.go` | Compiler frontend sanitizing LLM-generated Mangle with multi-pass repair. Exports `Sanitizer`, `NewSanitizer()` with AtomValidator, `Sanitize()` running full pipeline, `SanitizeAtoms()` for atom-only pass, and `preprocessAggregations()` converting SQL-style `VAR = count()` syntax. |
| `sanitizer_test.go` | Unit tests for sanitization passes. Tests atom interning, aggregation repair, safety injection, and SQL-style preprocessing. |

## Key Repairs

### SQL-Style Aggregation
```datalog
# Input (LLM hallucination)
Sum = count(X)

# Output (valid Mangle)
llm_agg("count", Sum, X)  # Later repaired to |> syntax
```

### Atom Interning
```datalog
# Input (string where atom expected)
status("active")

# Output (proper atom)
status(/active)
```

### Safety Injection
```datalog
# Input (unsafe negation)
unsafe(X) :- not safe(X).

# Output (bound variable)
unsafe(X) :- candidate(X), not safe(X).
```

## Dependencies

- `internal/mangle` - AtomValidator for type checking
- `github.com/google/mangle/ast` - AST types
- `github.com/google/mangle/parse` - Mangle parser

## Testing

```bash
go test ./internal/mangle/transpiler/...
```

---

**Remember: Push to GitHub regularly!**
