# Mangle CLI Command Reference

Complete documentation for all `mangle-cli` commands.

## Global Options

| Option | Short | Description |
|--------|-------|-------------|
| `--format` | `-f` | Output format: `json`, `text`, `sarif` |
| `--quiet` | `-q` | Suppress non-essential output |
| `--help` | `-h` | Show help |
| `--version` | `-v` | Show version |

---

## check

Run diagnostics on Mangle files (parse, semantic, stratification).

### Usage

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js check [options] <files...>
```

### Options

| Option | Values | Default | Description |
|--------|--------|---------|-------------|
| `--format` | `json`, `text`, `sarif` | `json` | Output format |
| `--severity` | `error`, `warning`, `info` | `info` | Minimum severity to report |
| `--fail-on` | `error`, `warning`, `never` | `error` | When to exit non-zero |

### Examples

```bash
# Check single file
node .claude/skills/mangle-programming/scripts/mangle-cli.js check src/rules.mg

# Check multiple files
node .claude/skills/mangle-programming/scripts/mangle-cli.js check src/*.mg

# Human-readable output
node .claude/skills/mangle-programming/scripts/mangle-cli.js check --format text src/rules.mg

# Only show errors
node .claude/skills/mangle-programming/scripts/mangle-cli.js check --severity error src/rules.mg

# CI mode: fail on warnings
node .claude/skills/mangle-programming/scripts/mangle-cli.js check --fail-on warning src/rules.mg

# Generate SARIF for GitHub
node .claude/skills/mangle-programming/scripts/mangle-cli.js check --format sarif src/*.mg > results.sarif
```

### JSON Output Structure

```json
{
  "version": "1.0",
  "files": [{
    "path": "src/rules.mg",
    "diagnostics": [{
      "severity": "error",
      "code": "E001",
      "source": "mangle-semantic",
      "message": "Variable 'X' in fact head must be ground",
      "range": {
        "start": { "line": 10, "column": 0 },
        "end": { "line": 10, "column": 15 }
      },
      "context": "bad_fact(X)."
    }]
  }],
  "summary": {
    "totalFiles": 1,
    "totalErrors": 1,
    "totalWarnings": 0,
    "totalInfo": 0
  }
}
```

---

## symbols

List all document symbols (predicates and their clauses).

### Usage

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js symbols [options] <file>
```

### Examples

```bash
# JSON output
node .claude/skills/mangle-programming/scripts/mangle-cli.js symbols src/rules.mg

# Human-readable
node .claude/skills/mangle-programming/scripts/mangle-cli.js symbols --format text src/rules.mg
```

### JSON Output Structure

```json
{
  "path": "src/rules.mg",
  "symbols": [{
    "name": "parent/2",
    "kind": "predicate",
    "range": {
      "start": { "line": 5, "column": 0 },
      "end": { "line": 5, "column": 25 }
    },
    "children": [{
      "name": "parent [1]",
      "kind": "clause",
      "range": { ... }
    }]
  }]
}
```

---

## hover

Get hover information at a specific position.

### Usage

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js hover <file> --line <n> --column <n>
```

### Options

| Option | Required | Description |
|--------|----------|-------------|
| `--line` | Yes | Line number (1-indexed) |
| `--column` | Yes | Column number (0-indexed) |

### Examples

```bash
# Get hover info at line 10, column 5
node .claude/skills/mangle-programming/scripts/mangle-cli.js hover src/rules.mg --line 10 --column 5
```

### JSON Output Structure

```json
{
  "contents": "**parent/2**\n\n*Defined in 3 clause(s)*\n*Referenced 5 time(s)*",
  "range": {
    "start": { "line": 10, "column": 0 },
    "end": { "line": 10, "column": 6 }
  }
}
```

---

## definition

Find the definition location of a symbol.

### Usage

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js definition <file> --line <n> --column <n>
```

### Examples

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js definition src/rules.mg --line 15 --column 10
```

### JSON Output Structure

```json
{
  "locations": [{
    "uri": "src/rules.mg",
    "range": {
      "start": { "line": 5, "column": 0 },
      "end": { "line": 5, "column": 25 }
    }
  }]
}
```

---

## references

Find all references to a symbol.

### Usage

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js references <file> --line <n> --column <n> [--include-declaration]
```

### Options

| Option | Description |
|--------|-------------|
| `--include-declaration` | Include the declaration in results |

### Examples

```bash
# Find all references
node .claude/skills/mangle-programming/scripts/mangle-cli.js references src/rules.mg --line 10 --column 5

# Include declaration
node .claude/skills/mangle-programming/scripts/mangle-cli.js references src/rules.mg --line 10 --column 5 --include-declaration
```

### JSON Output Structure

```json
{
  "locations": [
    { "uri": "src/rules.mg", "range": { ... } },
    { "uri": "src/rules.mg", "range": { ... } }
  ]
}
```

---

## completion

Get code completions at a position.

### Usage

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js completion <file> --line <n> --column <n>
```

### JSON Output Structure

```json
{
  "items": [{
    "label": ":gt",
    "kind": "Function",
    "detail": "Greater than comparison",
    "documentation": "Succeeds if first arg > second arg",
    "insertText": ":gt($1, $2)"
  }]
}
```

---

## format

Format Mangle source files.

### Usage

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js format [options] <files...>
```

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--write` | `-w` | Write formatted output back to files |
| `--check` | | Check if files are formatted (exit 1 if not) |
| `--diff` | | Show diff of formatting changes |

### Examples

```bash
# Check if formatted
node .claude/skills/mangle-programming/scripts/mangle-cli.js format --check src/*.mg

# Format in place
node .claude/skills/mangle-programming/scripts/mangle-cli.js format --write src/*.mg

# Show what would change
node .claude/skills/mangle-programming/scripts/mangle-cli.js format --diff src/rules.mg
```

### JSON Output Structure

```json
[{
  "path": "src/rules.mg",
  "formatted": true,
  "diff": "--- a/src/rules.mg\n+++ b/src/rules.mg\n..."
}]
```

---

## batch

Run multiple queries in a single call (efficient for multiple operations).

### Usage

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js batch <queries.json>
node .claude/skills/mangle-programming/scripts/mangle-cli.js batch -  # Read from stdin
```

### Query Format

```json
[
  { "id": 1, "type": "hover", "file": "src/rules.mg", "line": 10, "column": 5 },
  { "id": 2, "type": "diagnostics", "file": "src/rules.mg" },
  { "id": 3, "type": "symbols", "file": "src/rules.mg" },
  { "id": 4, "type": "fileInfo", "file": "src/rules.mg" }
]
```

### Query Types

| Type | Required Fields | Optional Fields |
|------|-----------------|-----------------|
| `hover` | `file`, `line`, `column` | |
| `definition` | `file`, `line`, `column` | |
| `references` | `file`, `line`, `column` | `includeDeclaration` |
| `completion` | `file`, `line`, `column` | |
| `symbols` | `file` | |
| `diagnostics` | `file` | |
| `format` | `file` | |
| `fileInfo` | `file` | |

### JSON Output Structure

```json
{
  "version": "1.0",
  "results": [{
    "id": 1,
    "type": "hover",
    "file": "src/rules.mg",
    "result": { ... }
  }],
  "summary": {
    "total": 4,
    "succeeded": 4,
    "failed": 0
  }
}
```

---

## file-info

Get complete analysis of a file in one call.

### Usage

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js file-info <file>
```

### JSON Output Structure

```json
{
  "path": "src/rules.mg",
  "lineCount": 150,
  "hasSyntaxErrors": false,
  "hasSemanticErrors": true,
  "ast": {
    "declCount": 5,
    "clauseCount": 42,
    "packageDecl": null
  },
  "predicates": [{
    "name": "parent",
    "arity": 2,
    "isExternal": false,
    "isPrivate": false,
    "definitionCount": 3,
    "referenceCount": 8
  }],
  "symbols": [...],
  "diagnostics": {
    "parseErrors": [],
    "semanticErrors": [...],
    "totalErrors": 2,
    "totalWarnings": 1
  }
}
```
