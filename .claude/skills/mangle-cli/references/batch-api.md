# Mangle CLI Batch API

The batch command enables running multiple queries in a single CLI invocation,
which is more efficient than running separate commands for each query.

## Why Use Batch?

1. **Performance**: File is parsed once and cached for all queries
2. **Atomicity**: All results returned together
3. **Efficiency**: Single process invocation vs. multiple
4. **Agent-friendly**: Designed for programmatic use by coding agents

## Basic Usage

```bash
# From a JSON file
node mangle-lsp/dist/cli.js batch queries.json

# From stdin (not well supported on Windows)
echo '[...]' | node mangle-lsp/dist/cli.js batch -
```

## Query Format

Queries are provided as a JSON array:

```json
[
  {
    "id": 1,
    "type": "hover",
    "file": "src/rules.mg",
    "line": 10,
    "column": 5
  },
  {
    "id": 2,
    "type": "diagnostics",
    "file": "src/rules.mg"
  }
]
```

Or wrapped in an object:

```json
{
  "queries": [
    { "id": 1, "type": "symbols", "file": "src/rules.mg" }
  ]
}
```

## Query Types

### hover

Get hover information at a position.

```json
{
  "id": 1,
  "type": "hover",
  "file": "src/rules.mg",
  "line": 10,
  "column": 5
}
```

**Result**:
```json
{
  "contents": "**parent/2**\n\n*Defined in 3 clause(s)*",
  "range": { ... }
}
```

### definition

Find definition location.

```json
{
  "id": 2,
  "type": "definition",
  "file": "src/rules.mg",
  "line": 15,
  "column": 3
}
```

**Result**:
```json
{
  "locations": [
    { "uri": "src/rules.mg", "range": { ... } }
  ]
}
```

### references

Find all references.

```json
{
  "id": 3,
  "type": "references",
  "file": "src/rules.mg",
  "line": 10,
  "column": 5,
  "includeDeclaration": true
}
```

**Result**:
```json
{
  "locations": [
    { "uri": "src/rules.mg", "range": { ... } },
    { "uri": "src/rules.mg", "range": { ... } }
  ]
}
```

### completion

Get code completions.

```json
{
  "id": 4,
  "type": "completion",
  "file": "src/rules.mg",
  "line": 20,
  "column": 10
}
```

**Result**:
```json
{
  "items": [
    { "label": ":gt", "kind": "Function", ... }
  ]
}
```

### symbols

List all document symbols.

```json
{
  "id": 5,
  "type": "symbols",
  "file": "src/rules.mg"
}
```

**Result**:
```json
{
  "symbols": [
    { "name": "parent/2", "kind": "predicate", "range": { ... } }
  ]
}
```

### diagnostics

Get all diagnostics for a file.

```json
{
  "id": 6,
  "type": "diagnostics",
  "file": "src/rules.mg"
}
```

**Result**:
```json
{
  "parseErrors": [],
  "semanticErrors": [
    { "code": "E001", "message": "...", "range": { ... } }
  ],
  "totalErrors": 2,
  "totalWarnings": 1
}
```

### format

Get formatting edits for a file.

```json
{
  "id": 7,
  "type": "format",
  "file": "src/rules.mg"
}
```

**Result**:
```json
{
  "edits": [
    { "range": { ... }, "newText": "..." }
  ],
  "formatted": "# Full formatted content..."
}
```

### fileInfo

Get complete file analysis (combines multiple queries).

```json
{
  "id": 8,
  "type": "fileInfo",
  "file": "src/rules.mg"
}
```

**Result**:
```json
{
  "path": "src/rules.mg",
  "lineCount": 150,
  "hasSyntaxErrors": false,
  "hasSemanticErrors": true,
  "ast": { "declCount": 5, "clauseCount": 42 },
  "predicates": [
    { "name": "parent", "arity": 2, "definitionCount": 3, "referenceCount": 8 }
  ],
  "symbols": [...],
  "diagnostics": { ... }
}
```

## Output Format

```json
{
  "version": "1.0",
  "results": [
    {
      "id": 1,
      "type": "hover",
      "file": "src/rules.mg",
      "result": { "contents": "..." }
    },
    {
      "id": 2,
      "type": "diagnostics",
      "file": "src/rules.mg",
      "result": { "parseErrors": [], "semanticErrors": [...] }
    }
  ],
  "summary": {
    "total": 2,
    "succeeded": 2,
    "failed": 0
  }
}
```

## Error Handling

If a query fails, the result includes an `error` field:

```json
{
  "id": 3,
  "type": "hover",
  "file": "nonexistent.mg",
  "result": null,
  "error": "File not found: nonexistent.mg"
}
```

## Exit Codes

- `0` - All queries succeeded
- `1` - One or more queries failed

## Common Patterns

### Initial File Exploration

Get everything about a file:

```json
[
  { "id": 1, "type": "fileInfo", "file": "src/rules.mg" }
]
```

### Targeted Inspection

Multiple lookups at different positions:

```json
[
  { "id": 1, "type": "hover", "file": "src/rules.mg", "line": 10, "column": 5 },
  { "id": 2, "type": "hover", "file": "src/rules.mg", "line": 25, "column": 0 },
  { "id": 3, "type": "references", "file": "src/rules.mg", "line": 10, "column": 5 }
]
```

### Multi-File Analysis

Analyze multiple files efficiently:

```json
[
  { "id": 1, "type": "diagnostics", "file": "src/rules.mg" },
  { "id": 2, "type": "diagnostics", "file": "src/helpers.mg" },
  { "id": 3, "type": "diagnostics", "file": "src/queries.mg" }
]
```

### Navigation Workflow

Find definition, then all references:

```json
[
  { "id": 1, "type": "definition", "file": "src/rules.mg", "line": 50, "column": 10 },
  { "id": 2, "type": "references", "file": "src/rules.mg", "line": 50, "column": 10, "includeDeclaration": true }
]
```

## Tips for Agents

1. **Start with fileInfo** - Get comprehensive overview first
2. **Use IDs** - Track which result corresponds to which query
3. **Batch related queries** - Multiple queries on same file = one parse
4. **Check the summary** - Quick way to verify all queries succeeded
5. **Handle errors gracefully** - Check for `error` field in results
