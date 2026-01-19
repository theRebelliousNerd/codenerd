# Mangle Module Analyzer - Quick Start

## TL;DR

```bash
# Basic analysis
python analyze_module.py internal/core/defaults/*.mg

# With virtual predicates
python analyze_module.py *.mg --virtual "$(cat .mangle-virtual-predicates | grep -v '^#' | tr -d '\n' | tr -d ' ')"

# Generate graph
python analyze_module.py *.mg --graph | dot -Tpng > deps.png
```

## Common Use Cases

### 1. Check Before Commit
```bash
# Verify no missing predicates
python analyze_module.py internal/core/defaults/*.mg --check-completeness \
  --virtual "$(cat .mangle-virtual-predicates | grep -v '^#' | tr -d '\n' | tr -d ' ')"
echo $?  # 0 = pass, 1 = fail
```

### 2. Find Cross-File Issues
```bash
# Show detailed analysis with dependency info
python analyze_module.py schemas.mg policy.mg coder.mg -v
```

### 3. Visualize Architecture
```bash
# Create module dependency graph
python analyze_module.py internal/core/defaults/*.mg --graph | \
  dot -Tpng -Gdpi=150 > module_deps.png
```

### 4. CI/CD Integration
```bash
# Strict mode (warnings = errors)
python analyze_module.py *.mg --strict --check-completeness \
  --virtual "file_content,symbol_at,tool_instance"
```

### 5. JSON for Tooling
```bash
# Machine-readable output
python analyze_module.py *.mg --json > analysis.json
jq '.conflicts[] | select(.severity == "error")' analysis.json
```

## What It Detects

| Issue | Severity | Description |
|-------|----------|-------------|
| Arity mismatch | ERROR | Same predicate with different argument counts |
| Declaration conflict | ERROR | Same predicate declared differently |
| Missing definition | ERROR | Predicate used but never defined |
| Duplicate definition | WARNING | Predicate defined in multiple files |
| Duplicate declaration | WARNING | Predicate declared in multiple files |

## Reading the Output

### Module Dependencies
```
coder.mg
  ├── imports from: policy.mg (3 predicates)
  └── imports from: schemas.mg (19 predicates)
```
**Means:** `coder.mg` uses predicates defined in `policy.mg` and `schemas.mg`

### Conflicts
```
--- Conflict #1 (ERROR) ---
Type: arity_mismatch
Predicate: learning_signal
Message: Predicate 'learning_signal' defined with different arities: [1, 2]
```
**Means:** `learning_signal` is sometimes called with 1 arg, sometimes 2. This is a bug.

### Missing Definitions
```
'tdd_state' - used but never defined
  Used in: coder.mg:357
  (May be a Go virtual predicate - add to --virtual list if intentional)
```
**Means:** `tdd_state` is referenced but no rule defines it. Either:
1. It's a typo
2. It's missing a definition
3. It's a Go FFI predicate (add to --virtual list)

## Options Reference

```
--check-completeness   Exit with error if any predicates undefined
--graph               Output DOT format graph
--json                Output JSON (machine-readable)
--virtual LIST        Comma-separated Go FFI predicates to ignore
--strict              Treat warnings as errors
-v, --verbose         Show detailed analysis
```

## Exit Codes

- **0** = All checks passed
- **1** = Issues found (conflicts or missing definitions)
- **2** = Parse error

## Tips

1. **Maintain `.mangle-virtual-predicates`** - Keep it updated with Go FFI predicates
2. **Run before commits** - Catch issues early
3. **Use --verbose** - Find unused code
4. **Generate graphs regularly** - Visualize architecture changes
5. **Automate in CI** - Enforce module coherence

## Examples

### Example 1: First-time analysis
```bash
python analyze_module.py internal/core/defaults/*.mg
# Review output, note missing predicates
# Add Go FFI predicates to .mangle-virtual-predicates
# Run again to verify
```

### Example 2: After refactoring
```bash
# Before refactor: save baseline
python analyze_module.py *.mg --json > before.json

# After refactor: compare
python analyze_module.py *.mg --json > after.json
diff before.json after.json
```

### Example 3: Find dependencies
```bash
# Which files does policy.mg depend on?
python analyze_module.py *.mg -v | grep -A 10 "policy.mg"
```

## Troubleshooting

**Q: Too many "missing definitions" warnings**
A: Add Go FFI predicates to `.mangle-virtual-predicates`

**Q: "Arity mismatch" error**
A: This is a real bug. Fix the predicate to use consistent argument counts.

**Q: "Duplicate definition" warning**
A: This is OK if you have multiple rules for the same predicate. Only a warning.

**Q: Graph is too cluttered**
A: Analyze fewer files at once, or filter the DOT output.

## See Also

- [Full Documentation](./README_ANALYZER.md)
- [diagnose_stratification.py](./diagnose_stratification.py) - For negation cycle detection
