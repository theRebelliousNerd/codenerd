# Mangle CLI Output Schemas

JSON schemas for all CLI command outputs.

## Common Types

### Position

```json
{
  "line": 10,    // 1-indexed
  "column": 5    // 0-indexed
}
```

### Range

```json
{
  "start": { "line": 10, "column": 0 },
  "end": { "line": 10, "column": 15 }
}
```

### DiagnosticSeverity

One of: `"error"`, `"warning"`, `"info"`

---

## check Command Output

### CheckResult

```json
{
  "version": "1.0",
  "files": [FileDiagnostics],
  "summary": {
    "totalFiles": 1,
    "totalErrors": 5,
    "totalWarnings": 2,
    "totalInfo": 0
  }
}
```

### FileDiagnostics

```json
{
  "path": "src/rules.mg",
  "diagnostics": [CLIDiagnostic]
}
```

### CLIDiagnostic

```json
{
  "severity": "error",
  "code": "E001",
  "source": "mangle-semantic",
  "message": "Variable 'X' in fact head must be ground",
  "range": {
    "start": { "line": 10, "column": 0 },
    "end": { "line": 10, "column": 15 }
  },
  "context": "bad_fact(X)."  // optional: source line
}
```

**Source values**:
- `mangle-cli` - CLI/IO errors
- `mangle-lexer` - Lexer errors
- `mangle-parse` - Parser errors
- `mangle-semantic` - Semantic errors
- `mangle-stratification` - Stratification errors

---

## symbols Command Output

### SymbolsResult

```json
{
  "path": "src/rules.mg",
  "symbols": [CLISymbol]
}
```

### CLISymbol

```json
{
  "name": "parent/2",
  "kind": "predicate",
  "range": {
    "start": { "line": 5, "column": 0 },
    "end": { "line": 5, "column": 25 }
  },
  "selectionRange": { ... },
  "children": [CLISymbol]  // optional
}
```

**Kind values**: `"predicate"`, `"declaration"`, `"clause"`

---

## hover Command Output

### HoverResult

```json
{
  "contents": "**parent/2**\n\n*Defined in 3 clause(s)*\n*Referenced 5 time(s)*",
  "range": {
    "start": { "line": 10, "column": 0 },
    "end": { "line": 10, "column": 6 }
  }
}
```

Returns `null` if no hover information available.

---

## definition Command Output

### DefinitionResult

```json
{
  "locations": [
    {
      "uri": "src/rules.mg",
      "range": {
        "start": { "line": 5, "column": 0 },
        "end": { "line": 5, "column": 25 }
      }
    }
  ]
}
```

---

## references Command Output

### ReferencesResult

```json
{
  "locations": [
    {
      "uri": "src/rules.mg",
      "range": { ... }
    },
    {
      "uri": "src/rules.mg",
      "range": { ... }
    }
  ]
}
```

---

## completion Command Output

### CompletionResult

```json
{
  "items": [
    {
      "label": ":gt",
      "kind": "Function",
      "detail": "Greater than comparison",
      "documentation": "Succeeds if first arg > second arg",
      "insertText": ":gt($1, $2)"
    }
  ]
}
```

---

## format Command Output

### FormatResult (array)

```json
[
  {
    "path": "src/rules.mg",
    "formatted": true,
    "diff": "--- a/src/rules.mg\n+++ b/src/rules.mg\n..."
  }
]
```

**Fields**:
- `path` - File path
- `formatted` - `true` if file is already formatted (or was formatted)
- `diff` - Unified diff (only with `--diff` flag)
- `error` - Error message if formatting failed

---

## batch Command Output

### BatchOutput

```json
{
  "version": "1.0",
  "results": [BatchResult],
  "summary": {
    "total": 4,
    "succeeded": 3,
    "failed": 1
  }
}
```

### BatchResult

```json
{
  "id": 1,
  "type": "hover",
  "file": "src/rules.mg",
  "result": { ... },
  "error": "optional error message"
}
```

---

## file-info Command Output

### FileInfo

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
  "predicates": [
    {
      "name": "parent",
      "arity": 2,
      "isExternal": false,
      "isPrivate": false,
      "definitionCount": 3,
      "referenceCount": 8
    }
  ],
  "symbols": [CLISymbol],
  "diagnostics": {
    "parseErrors": [],
    "semanticErrors": [CLIDiagnostic],
    "totalErrors": 2,
    "totalWarnings": 1
  }
}
```

---

## SARIF Output (check --format sarif)

Follows SARIF 2.1.0 specification for GitHub Actions integration.

```json
{
  "$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
  "version": "2.1.0",
  "runs": [{
    "tool": {
      "driver": {
        "name": "mangle-cli",
        "version": "1.0.0",
        "informationUri": "https://github.com/theRebelliousNerd/MangleLSP",
        "rules": [
          {
            "id": "E001",
            "shortDescription": { "text": "Variables in facts must be ground" },
            "defaultConfiguration": { "level": "error" }
          }
        ]
      }
    },
    "results": [
      {
        "ruleId": "E001",
        "level": "error",
        "message": { "text": "Variable 'X' in fact head must be ground" },
        "locations": [{
          "physicalLocation": {
            "artifactLocation": { "uri": "src/rules.mg" },
            "region": {
              "startLine": 10,
              "startColumn": 1,
              "endLine": 10,
              "endColumn": 16
            }
          }
        }]
      }
    ]
  }]
}
```
