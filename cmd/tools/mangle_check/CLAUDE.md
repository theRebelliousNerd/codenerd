# cmd/tools/mangle_check - Mangle AST Inspector

This tool inspects Mangle AST clause field structure for debugging and development.

## Usage

```bash
go run ./cmd/tools/mangle_check
```

## File Index

| File | Description |
|------|-------------|
| `inspect_clause.go` | Simple AST clause field inspector using reflection on `ast.Clause` struct. Displays all field names in the Mangle Clause type for debugging parser integration and understanding AST structure. |

## Output

Lists all fields in `github.com/google/mangle/ast.Clause`:
- Head
- Body
- Premises (if present)
- etc.

## Dependencies

- `github.com/google/mangle/ast` - Mangle AST types

## Building

```bash
go run ./cmd/tools/mangle_check
```

---

**Remember: Push to GitHub regularly!**


> *[Archived & Reviewed by The Librarian on 2026-01-25]*