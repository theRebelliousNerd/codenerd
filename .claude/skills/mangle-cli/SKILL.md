---
name: mangle-cli
description: >
  This skill should be used when the user asks to "check mangle files",
  "lint .mg files", "get diagnostics for mangle", "find errors in mangle code",
  "analyze mangle syntax", "format mangle files", "get hover info", "find definition",
  "find references", or when programmatically working with Mangle language files (.mg).
  Provides comprehensive CLI access for parsing, semantic analysis, stratification
  checking, code navigation, batch queries, and CI/CD integration via SARIF output.
---

# Mangle CLI Tool

The `mangle-cli` provides machine-readable access to the Mangle language server's
analysis capabilities. Use this skill to check, navigate, and analyze Mangle files.

## CLI Location

The CLI is bundled with this skill at `scripts/mangle-cli.js`. To run:

```bash
node <skill-path>/scripts/mangle-cli.js <command> [options] <files...>
```

Where `<skill-path>` is the path to this skill directory (e.g., `.claude/skills/mangle-cli`).

**Example**:

```bash
node .claude/skills/mangle-cli/scripts/mangle-cli.js check src/rules.mg
```

The CLI is fully standalone - no external dependencies required.

## Commands Quick Reference

| Command | Purpose | Key Options |
|---------|---------|-------------|
| `check` | Run all diagnostics | `--format`, `--severity`, `--fail-on` |
| `symbols` | List predicates/clauses | `--format` |
| `hover` | Info at position | `--line`, `--column` |
| `definition` | Find where defined | `--line`, `--column` |
| `references` | Find all uses | `--line`, `--column`, `--include-declaration` |
| `completion` | Get completions | `--line`, `--column` |
| `format` | Format files | `--write`, `--check`, `--diff` |
| `batch` | Multiple queries | JSON file or stdin |
| `file-info` | Complete analysis | (none) |

## Output Formats

Use `--format` / `-f`:
- `json` (default) - Structured JSON for programmatic parsing
- `text` - Human-readable output
- `sarif` - SARIF 2.1.0 for GitHub Actions integration (check only)

## Position Conventions

- **Lines**: 1-indexed (first line is 1)
- **Columns**: 0-indexed (first character is 0)

## Common Workflows

### Check Files for Errors

```bash
# JSON output (default)
node mangle-lsp/dist/cli.js check file.mg

# Human-readable
node mangle-lsp/dist/cli.js check --format text file.mg

# Only errors (no warnings)
node mangle-lsp/dist/cli.js check --severity error file.mg

# For CI: exit 1 on warnings too
node mangle-lsp/dist/cli.js check --fail-on warning file.mg
```

### Get Complete File Analysis

```bash
node mangle-lsp/dist/cli.js file-info file.mg
```

Returns predicates, symbols, diagnostics, AST structure, line count.

### Code Navigation

```bash
# Hover info
node mangle-lsp/dist/cli.js hover file.mg --line 10 --column 5

# Go to definition
node mangle-lsp/dist/cli.js definition file.mg --line 10 --column 5

# Find all references
node mangle-lsp/dist/cli.js references file.mg --line 10 --column 5
```

### Format Files

```bash
# Check if formatted (exit 1 if not)
node mangle-lsp/dist/cli.js format --check file.mg

# Format in place
node mangle-lsp/dist/cli.js format --write file.mg

# Show diff
node mangle-lsp/dist/cli.js format --diff file.mg
```

### Batch Queries (Efficient)

For multiple queries on the same file, use batch to avoid repeated parsing:

```bash
# From file
echo '[
  {"id": 1, "type": "diagnostics", "file": "file.mg"},
  {"id": 2, "type": "symbols", "file": "file.mg"},
  {"id": 3, "type": "hover", "file": "file.mg", "line": 10, "column": 5}
]' > queries.json
node mangle-lsp/dist/cli.js batch queries.json
```

**Query types**: `hover`, `definition`, `references`, `completion`, `symbols`, `diagnostics`, `format`, `fileInfo`

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (no errors, or `--fail-on never`) |
| 1 | Errors found (or warnings if `--fail-on warning`) |
| 2 | Invalid arguments or CLI error |

## CI/CD Integration

Generate SARIF for GitHub code scanning:

```bash
node mangle-lsp/dist/cli.js check --format sarif src/*.mg > results.sarif
```

## Reference Documentation

- `references/commands.md` - Detailed command documentation
- `references/error-codes.md` - All diagnostic codes and meanings
- `references/output-schemas.md` - JSON output schemas
- `references/batch-api.md` - Batch query format and examples

## Best Practices

1. **Use batch queries** for multiple operations on the same file
2. **Use file-info** for initial exploration (returns everything)
3. **Use --format text** for debugging, JSON for automation
4. **Check exit codes** in scripts for proper error handling
5. **Use SARIF** for CI/CD integration with GitHub Actions
