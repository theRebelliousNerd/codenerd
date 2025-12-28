# Mangle Predicate Lineage/Provenance Tool

`explain_derivation.py` - Explains how facts are derived in Mangle/Datalog programs by building proof trees.

## Purpose

Answers questions like:
- "How would predicate X be derived?"
- "Why is fact X true?"
- "What base facts are needed for X to hold?"
- "What are all the ways X could be proven?"

Critical for understanding complex policy derivations and debugging logic programs.

## Quick Start

```bash
# Basic explanation
python explain_derivation.py policy.mg schemas.mg --explain "next_action(/run_tests)"

# Show all derivation paths
python explain_derivation.py policy.mg --explain "delegate_task(/coder, Task, /pending)" --all-paths

# Inline code testing
python explain_derivation.py --check-string "foo(X) :- bar(X). Decl bar(X)." --explain "foo(X)"

# JSON output for tooling
python explain_derivation.py policy.mg --explain "permitted(/fs_read)" --json
```

## Usage

```
python explain_derivation.py <mangle_file> [<mangle_file>...] --explain PREDICATE [OPTIONS]
python explain_derivation.py --check-string CODE --explain PREDICATE [OPTIONS]
```

### Arguments

- `<mangle_file>` - One or more .mg files to analyze (loads all)
- `--explain PREDICATE` - **Required**. The predicate to explain (see format below)
- `--check-string CODE` - Analyze inline Mangle code instead of files

### Options

- `--depth N` - Maximum recursion depth (default: 10)
- `--all-paths` - Show all derivation paths, not just first
- `--json` - Output results as JSON
- `--verbose` - Show detailed analysis

## Predicate Format

### Fully Ground (Concrete)
```
delegate_task(/coder, "fix bug", /pending)
user_intent("id1", /mutation, /fix, "app.go", /none)
next_action(/run_tests)
```

### Partially Ground (With Variables)
```
delegate_task(/coder, Task, /pending)     # Variable: Task
next_action(X)                            # Variable: X
user_intent(_, Category, /fix, _, _)      # Mix of vars and constants
```

Variables are:
- Uppercase identifiers: `X`, `Y`, `Task`, `Score`
- Underscore wildcard: `_`

## Output Format

### Text Output (Default)

```
EXPLAINING: next_action(/run_tests)

PROOF TREE:
----------------------------------------------------------------------

=== PATH 1 ===
next_action(/run_tests)
+-- RULE (line 81):
|   next_action(/run_tests) :- test_state(/patch_applied).
|   NEEDS:
    test_state(/patch_applied)
    +-- EDB FACT (must exist in fact store)


=== PATH 2 ===
next_action(/run_tests)
+-- RULE (line 84):
|   next_action(/run_tests) :- test_state(/unknown), user_intent(_, _, /test, _, _).
|   NEEDS:
    test_state(/unknown)
    +-- EDB FACT (must exist in fact store)

    user_intent(_, _, /test, _, _)
    +-- EDB FACT (must exist in fact store)


======================================================================
REQUIRED EDB FACTS:
----------------------------------------------------------------------

test_state:
  - test_state(/patch_applied)
  - test_state(/unknown)

user_intent:
  - user_intent(_, _, /test, _, _)

======================================================================
STATUS: Derivable if required EDB facts exist in fact store
======================================================================
```

### JSON Output

```json
{
  "derivable": true,
  "paths": [
    {
      "predicate": "next_action(/run_tests)",
      "type": "idb",
      "rule_line": 81,
      "rule_text": "next_action(/run_tests) :- test_state(/patch_applied).",
      "bindings": {},
      "children": [
        {
          "predicate": "test_state(/patch_applied)",
          "type": "edb",
          "rule_line": null,
          "rule_text": null,
          "bindings": {},
          "children": []
        }
      ]
    }
  ],
  "required_edb": [
    "test_state(/patch_applied)"
  ],
  "num_paths": 1
}
```

## Fact Types

The tool distinguishes between:

- **EDB** (Extensional) - Base facts that must exist in the fact store
- **IDB** (Intensional) - Derived facts computed via rules
- **VIRTUAL** - Predicates computed by FFI/VirtualStore (Go runtime)
- **BUILTIN** - Built-in predicates (list:member, time_diff, etc.)

## Examples

### Example 1: Understanding Delegation

```bash
python explain_derivation.py policy.mg schemas.mg \
    --explain "delegate_task(/coder, \"implement feature\", /pending)"
```

Shows which `user_intent` patterns would trigger coder delegation.

### Example 2: Debugging TDD Loop

```bash
python explain_derivation.py policy.mg schemas.mg \
    --explain "next_action(/read_error_log)" --all-paths
```

Shows all the conditions that lead to reading error logs (test failures, retry counts, etc.).

### Example 3: Exploring Activation

```bash
python explain_derivation.py policy.mg schemas.mg \
    --explain "context_atom(Fact)" --depth 5
```

Traces the spreading activation mechanism to understand what facts enter the LLM context.

### Example 4: Testing Recursive Rules

```bash
python explain_derivation.py --check-string "
Decl parent(X, Y).
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).
" --explain "ancestor(A, B)" --all-paths
```

Shows both the base case (direct parent) and recursive case (transitive closure).

## Exit Codes

- `0` - Predicate is derivable
- `1` - Predicate is not derivable (missing facts or no rules)
- `2` - Parse error or file not found

## Implementation Notes

### Parsing
- Handles multi-line rules
- Handles multiple statements per line
- Respects string boundaries and parentheses
- Removes comments before parsing

### Unification
- Variables unify with constants
- Variables unify with other variables
- Constants must match exactly
- Bindings propagate through rule bodies

### Cycle Detection
- Tracks visited predicates to avoid infinite loops
- Respects `--depth` limit for recursion
- Shows partial derivations even for cyclic rules

### Limitations
- Does not execute aggregations (fn:count, etc.) - treats them as opaque
- Does not evaluate arithmetic or comparisons (>, <, ==)
- Does not handle stratification constraints (see `diagnose_stratification.py`)
- Assumes all variables are safe (properly bound)

## Integration with Other Tools

### With Stratification Checker

```bash
# First check if program is stratified
python diagnose_stratification.py policy.mg

# Then explain specific derivations
python explain_derivation.py policy.mg --explain "winning(X)"
```

### With codeNERD Kernel

Use JSON output to integrate with automation:

```python
import subprocess
import json

result = subprocess.run([
    'python', 'explain_derivation.py',
    'policy.mg', '--explain', 'next_action(X)', '--json'
], capture_output=True, text=True)

data = json.loads(result.stdout)
if data['derivable']:
    print(f"Found {data['num_paths']} derivation paths")
    for fact in data['required_edb']:
        print(f"Needs: {fact}")
```

## Troubleshooting

### "No derivation paths found"

- Check that predicate name is spelled correctly
- Verify the predicate has rules (not just `Decl`)
- Try with `--all-paths` to see if partial derivations exist
- Check that required EDB predicates are declared

### "Could not parse target predicate"

- Ensure predicate uses correct syntax: `name(arg1, arg2)`
- Variables must be uppercase or `_`
- Constants should be `/atom`, `"string"`, or number
- Check for balanced parentheses

### Unicode errors (Windows)

The tool uses ASCII box-drawing characters (`+--`, `|`) compatible with all terminals.

## See Also

- `diagnose_stratification.py` - Check for stratification violations
- [Mangle Language Reference](.claude/skills/mangle-programming/references/) - Full language docs
- [codeNERD Architecture](.claude/skills/codenerd-builder/references/) - System design

## Version

v1.0 - Compatible with Mangle v0.4.0 (November 2024)
