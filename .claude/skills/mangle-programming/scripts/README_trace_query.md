# Mangle Query Tracer

Step-by-step query evaluation debugger for Google Mangle programs. Simulates query evaluation, showing which rules fire and why.

## Purpose

Debug "why doesn't my rule derive anything?" by showing:
- Which rules could potentially match
- Variable bindings attempted
- Which body predicates succeed/fail
- Final results or detailed explanation of failure

## Usage

```bash
# Query a Mangle file
python trace_query.py <mangle_file> --query "predicate(X, Y)"

# Query with seed facts
python trace_query.py policy.mg --query "next_action(X)" \
    --facts "user_intent(/id1, /query, /read, /foo, _). test_state(/failing)."

# Query inline code
python trace_query.py --check-string "<code>" --query "pred(X)"

# Verbose output
python trace_query.py policy.mg --query "next_action(X)" -v
```

## Options

- `--query, -q` - Query to evaluate (e.g., `"next_action(X)"` or `"?next_action(X)"`)
- `--facts, -f` - Seed facts to add (semicolon or period separated)
- `--max-steps` - Maximum evaluation steps (default: 100)
- `--verbose, -v` - Show more detailed trace information
- `--check-string, -s` - Analyze inline Mangle code instead of file

## Exit Codes

- `0` - Query succeeded with results
- `1` - Query failed (no results)
- `2` - Parse error or usage error

## Example Output

```
QUERY: next_action(X)

STEP 1: Trying rule at line 71
  next_action(/read_error_log) :- test_state(/failing), retry_count(N), N < 3.

  Checking: test_state(/failing)
    -> MATCH (fact): test_state(/failing)
  Checking: retry_count(N)
    -> MATCH (fact): retry_count(2)
  Checking: N < 3
    -> TRUE
  Rule SUCCEEDED with 1 solution(s)

STEP 2: Trying rule at line 75
  next_action(/analyze_root_cause) :- test_state(/log_read).

  Checking: test_state(/log_read)
    -> NO MATCH
  Rule FAILED: Could not satisfy all body predicates

======================================================================
RESULTS: 1 result(s) found
  1. next_action(/read_error_log)
```

## Supported Mangle Patterns

- **Variables**: UPPERCASE (X, Y, Var)
- **Atoms**: /lowercase (/active, /user1)
- **Strings**: "quoted"
- **Numbers**: 42, 3.14
- **Negation**: `!pred(X)` or `not pred(X)`
- **Comparisons**: `X < 3`, `X != Y`, `X >= 5`
- **Wildcards**: `_`
- **Lists**: `[X, Y, Z]`

## Limitations

- Maximum recursion depth: 10 levels (configurable via max_steps)
- Equality (`=`) is treated as comparison, not unification
- Does not support all aggregation functions
- Does not support Mangle pipelines (`|>`)
- Simplified evaluation model (not identical to actual Mangle engine)

## Compatible With

Mangle v0.4.0 (November 2024)
