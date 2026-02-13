# Mangle Error Message Reference

**Exhaustive catalog of EVERY possible Mangle error with exact messages, causes, and fixes.**

---

## Quick Navigation

| Error Category | File | Use When |
|----------------|------|----------|
| **Parse Errors** | [parse_errors.md](./parse_errors.md) | Syntax violations, token errors, grammar issues |
| **Analysis Errors** | [analysis_errors.md](./analysis_errors.md) | Safety violations, stratification, variable binding |
| **Type Errors** | [type_errors.md](./type_errors.md) | Type mismatches, atom vs string, struct fields |
| **Runtime Errors** | [runtime_errors.md](./runtime_errors.md) | Evaluation failures, fact insertion, queries |
| **Gas Errors** | [gas_errors.md](./gas_errors.md) | Resource limits, timeouts, memory exhaustion |
| **Undefined Errors** | [undefined_errors.md](./undefined_errors.md) | Undeclared predicates, missing functions |

---

## Error Resolution Flowchart

```text
Got an error?
    |
    v
Does it mention "parse" or "syntax"?
    YES -> [parse_errors.md]
    NO  -> Continue
    |
    v
Does it mention "stratification" or "variable not bound"?
    YES -> [analysis_errors.md]
    NO  -> Continue
    |
    v
Does it mention "type mismatch" or "expected <type>"?
    YES -> [type_errors.md]
    NO  -> Continue
    |
    v
Does it mention "limit exceeded" or "timeout"?
    YES -> [gas_errors.md]
    NO  -> Continue
    |
    v
Does it mention "not declared" or "function not found"?
    YES -> [undefined_errors.md]
    NO  -> Continue
    |
    v
Does it occur during evaluation/query?
    YES -> [runtime_errors.md]
    NO  -> Check all categories
```

---

## Error Message Patterns

### Parse Phase (Before Analysis)
```text
<line>:<col> parse error: <details>
token recognition error at: '<token>'
no viable alternative at input '<text>'
mismatched input '<found>' expecting '<expected>'
```
→ See [parse_errors.md](./parse_errors.md)

---

### Analysis Phase (After Parse)
```text
stratification: program cannot be stratified
variable <Var> is not bound in <clause>
variable <Var> in <predicate> will not have a value yet
<atom> does not match arity of <decl>
```
→ See [analysis_errors.md](./analysis_errors.md)

---

### Type System
```text
could not unify <fact> and <decl>: type mismatch
type mismatch <premise> : <error>
:match_field struct type <type> does not have field <field>
expected <type>, got <actual>
```
→ See [type_errors.md](./type_errors.md)

---

### Runtime/Evaluation
```text
could not find declaration for <predicate>
query execution timed out after <duration>
no decl for predicate <predicate>
analysis: <underlying error>
stratification: <underlying error>
```
→ See [runtime_errors.md](./runtime_errors.md)

---

### Resource Limits
```text
fact limit exceeded: <N>
derived facts limit exceeded (inference gas limit)
query execution timed out after <duration>
fact size limit reached evaluating "<rule>"
session validation budget exhausted
```
→ See [gas_errors.md](./gas_errors.md)

---

### Undefined Symbols
```text
predicate <pred> is not declared in schemas
function <func> not found
rule uses undefined predicates: [<list>]
ext callback for a predicate <pred> without decl
```
→ See [undefined_errors.md](./undefined_errors.md)

---

## Error by Source

### Google Mangle Engine Errors

**From `parse/` package:**
- "not a base term"
- "not an atom"
- Parse ANTLR errors (line:col syntax errors)

**From `analysis/` package:**
- Stratification errors
- Safety errors (variable binding)
- Declaration errors
- Type inference errors

**From `engine/` package:**
- Fact store errors
- Evaluation errors
- External predicate errors
- Resource limit errors

---

### codeNERD Wrapper Errors

**From `internal/mangle/engine.go`:**
- "no schemas loaded"
- "predicate X is not declared in schemas"
- "predicate X expects N args, got M"
- "failed to parse schema"
- "failed to analyze schema"

**From `internal/mangle/schema_validator.go`:**
- "rule uses undefined predicates"
- "validation errors:"

**From `internal/mangle/feedback/` package:**
- Pre-validation errors
- Retry budget errors
- Timeout errors

---

## Complete Error Index

### A
- Aggregation syntax errors → [parse_errors.md](./parse_errors.md#7-aggregation-errors)
- Analysis error → [runtime_errors.md](./runtime_errors.md#21-analysis-error)
- Arity mismatch → [analysis_errors.md](./analysis_errors.md#42-arity-mismatch)

### B
- Break error → [runtime_errors.md](./runtime_errors.md#91-break-error-internal)

### C
- Cannot be stratified → [analysis_errors.md](./analysis_errors.md#11-program-cannot-be-stratified)
- Could not find declaration → [runtime_errors.md](./runtime_errors.md#11-could-not-find-declaration)
- Could not unify → [type_errors.md](./type_errors.md#11-cannot-unify-fact-and-declaration)

### D
- Decl requires atom → [type_errors.md](./type_errors.md#61-decl-requires-atom-with-variables)
- Derived facts limit → [gas_errors.md](./gas_errors.md#21-derived-facts-limit-exceeded)
- Do-transforms cannot have variable → [analysis_errors.md](./analysis_errors.md#33-do-transforms-cannot-have-variable)

### E
- Empty rule → [parse_errors.md](./parse_errors.md#7-empty-ruleclause)
- Expected base term → [parse_errors.md](./parse_errors.md#2-not-a-base-term)
- Expected bounds → [type_errors.md](./type_errors.md#62-expected-bounds-vs-got-bounds)
- Ext callback → [runtime_errors.md](./runtime_errors.md#31-ext-callback-for-predicate-without-decl)
- External predicate not registered → [undefined_errors.md](./undefined_errors.md#61-external-predicate-not-registered)

### F
- Fact limit exceeded → [gas_errors.md](./gas_errors.md#11-fact-limit-exceeded)
- Fact size limit → [gas_errors.md](./gas_errors.md#22-fact-size-limit-reached-mangle-engine)
- Failed to analyze schema → [runtime_errors.md](./runtime_errors.md#74-failed-to-analyze-schema)
- Failed to parse schema → [runtime_errors.md](./runtime_errors.md#73-failed-to-parse-schema)
- Function not found → [undefined_errors.md](./undefined_errors.md#22-function-not-found-runtime)

### G
- Gas limit exceeded → [gas_errors.md](./gas_errors.md#21-derived-facts-limit-exceeded)
- Group by distinct variables → [analysis_errors.md](./analysis_errors.md#35-group-by-arguments-must-be-distinct-variables)

### H
- Head variable neither grouped nor aggregated → [analysis_errors.md](./analysis_errors.md#36-head-variable-neither-grouped-nor-aggregated)

### I
- Inclusion constraint → [runtime_errors.md](./runtime_errors.md#51-unexpected-inclusion-constraint)
- Invalid atom → [parse_errors.md](./parse_errors.md#9-invalid-atomconstant-name)
- Invalid variable → [parse_errors.md](./parse_errors.md#8-invalid-variable-name)

### L
- List type mismatch → [type_errors.md](./type_errors.md#51-list-element-type-mismatch)

### M
- match_field errors → [type_errors.md](./type_errors.md#2-field-access-type-errors)
- Max retries exceeded → [gas_errors.md](./gas_errors.md#42-max-retries-exceeded-for-rule)
- Mismatched input → [parse_errors.md](./parse_errors.md#6-mismatched-input)

### N
- No decl for predicate → [runtime_errors.md](./runtime_errors.md#24-no-decl-for-predicate)
- No schemas loaded → [runtime_errors.md](./runtime_errors.md#71-no-schemas-loaded)
- No viable alternative → [parse_errors.md](./parse_errors.md#5-no-viable-alternative)
- Not a base term → [parse_errors.md](./parse_errors.md#1-not-a-base-term)
- Not an atom → [parse_errors.md](./parse_errors.md#2-not-an-atom)

### O
- Out of memory → [gas_errors.md](./gas_errors.md#51-out-of-memory-oom)

### P
- Parse error → [parse_errors.md](./parse_errors.md#3-parse-error-generic)
- Predicate declared more than once → [analysis_errors.md](./analysis_errors.md#41-predicate-declared-more-than-once)
- Predicate expects N args → [runtime_errors.md](./runtime_errors.md#82-predicate-arity-mismatch-codenerd)
- Predicate has no modes → [runtime_errors.md](./runtime_errors.md#63-predicate-has-no-modes)
- Predicate not declared → [undefined_errors.md](./undefined_errors.md#11-predicate-not-declared-in-schemas)

### Q
- Query timeout → [gas_errors.md](./gas_errors.md#31-query-execution-timeout)

### R
- Rule uses undefined predicates → [undefined_errors.md](undefined_errors.md#31-rule-uses-undefined-predicates)

### S
- Session budget exhausted → [gas_errors.md](./gas_errors.md#41-session-validation-budget-exhausted)
- Stratification → [analysis_errors.md](./analysis_errors.md#1-stratification-violations)
- Struct field access → [type_errors.md](./type_errors.md#2-field-access-type-errors)

### T
- Token recognition error → [parse_errors.md](./parse_errors.md#4-token-recognition-error)
- Transform redefines variable → [analysis_errors.md](./analysis_errors.md#31-transform-redefines-variable)
- Type has empty type → [analysis_errors.md](./analysis_errors.md#61-variable-has-empty-type)
- Type mismatch → [type_errors.md](./type_errors.md#1-type-mismatch-errors)

### U
- Unbalanced parentheses → [parse_errors.md](./parse_errors.md#32-unbalanced-parentheses)
- Undeclared predicate → [undefined_errors.md](./undefined_errors.md#1-undeclared-predicate-errors)
- Undefined function → [undefined_errors.md](./undefined_errors.md#2-undefined-function-errors)
- Unsafe variable → [analysis_errors.md](./analysis_errors.md#2-unsafe-variable-errors)
- Unsupported fact argument type → [runtime_errors.md](./runtime_errors.md#83-unsupported-fact-argument-type-codenerd)

### V
- Validation errors → [undefined_errors.md](./undefined_errors.md#32-validation-errors-list)
- Variable cannot have both types → [type_errors.md](./type_errors.md#81-variable-cannot-have-both-types)
- Variable in negation → [analysis_errors.md](./analysis_errors.md#24-variable-in-negation-not-bound)
- Variable not bound → [analysis_errors.md](./analysis_errors.md#21-variable-not-bound-in-rule)
- Variable will not have value yet → [analysis_errors.md](./analysis_errors.md#22-variable-will-not-have-value-yet)

### W
- Wrong number of arguments → [analysis_errors.md](./analysis_errors.md#42-arity-mismatch)

---

## Top 30 Common Errors (AI Agent Edition)

Errors sorted by frequency in LLM-generated Mangle code:

1. **Atom vs String Confusion** → [type_errors.md](./type_errors.md#31-atom-where-string-expected)
2. **Prolog Negation `\+`** → [parse_errors.md](./parse_errors.md#34-prolog-negation-syntax)
3. **Missing Period** → [parse_errors.md](./parse_errors.md#31-missing-period)
4. **Undeclared Predicate** → [undefined_errors.md](./undefined_errors.md#11-predicate-not-declared-in-schemas)
5. **Unsafe Variable (Negation)** → [analysis_errors.md](./analysis_errors.md#24-variable-in-negation-not-bound)
6. **Wrong Aggregation Syntax (SQL-style)** → [parse_errors.md](./parse_errors.md#72-sql-style-aggregation)
7. **Lowercase Aggregation Functions** → [undefined_errors.md](./undefined_errors.md#215-aggregation-casing-errors)
8. **Stratification Violation** → [analysis_errors.md](./analysis_errors.md#11-program-cannot-be-stratified)
9. **Variable Not Bound** → [analysis_errors.md](./analysis_errors.md#21-variable-not-bound-in-rule)
10. **Soufflé `.decl` Syntax** → [parse_errors.md](./parse_errors.md#51-wrong-declaration-syntax)
11. **Hallucinated Functions (fn:split, fn:substring)** → [undefined_errors.md](./undefined_errors.md#211-string-functions)
12. **Type Mismatch (int vs float)** → [type_errors.md](./type_errors.md#41-integer-vs-float-mismatch)
13. **Struct Dot Notation (Map.key)** → [parse_errors.md](./parse_errors.md#212-direct-struct-field-access)
14. **Missing `do` in Pipeline** → [parse_errors.md](./parse_errors.md#183-missing-do-keyword)
15. **Variable in Head Not in Body** → [analysis_errors.md](./analysis_errors.md#21-variable-not-bound-in-rule)
16. **Arity Mismatch** → [analysis_errors.md](./analysis_errors.md#42-arity-mismatch)
17. **NULL/UNKNOWN Keywords** → [parse_errors.md](./parse_errors.md#262-nullunknownundefined-keywords)
18. **Case/When Statements** → [parse_errors.md](./parse_errors.md#272-casewhenelse-statements)
19. **Variable Order (Will Not Have Value)** → [analysis_errors.md](./analysis_errors.md#22-variable-will-not-have-value-yet)
20. **Transform Redefines Variable** → [analysis_errors.md](./analysis_errors.md#31-transform-redefines-variable)
21. **Comment Syntax (`//` instead of `#`)** → [parse_errors.md](./parse_errors.md#10-comment-syntax-errors)
22. **Colon-Prefixed Atoms (`:active`)** → [parse_errors.md](./parse_errors.md#232-colon-prefixed-atoms)
23. **Head Variable Not Grouped/Aggregated** → [analysis_errors.md](./analysis_errors.md#36-head-variable-neither-grouped-nor-aggregated)
24. **Invalid Characters (`&&`, `||`)** → [parse_errors.md](./parse_errors.md#33-invalid-characters)
25. **Confidence Score Type (0-100 vs 0.0-1.0)** → [type_errors.md](./type_errors.md#71-confidence-scores-float-vs-int)
26. **Unbalanced Parentheses** → [parse_errors.md](./parse_errors.md#32-unbalanced-parentheses)
27. **Findall/Bagof/Setof (Prolog)** → [parse_errors.md](./parse_errors.md#282-findallbagofsetof)
28. **Gas Limit Exceeded** → [gas_errors.md](./gas_errors.md#21-derived-facts-limit-exceeded)
29. **File Path (String vs Atom)** → [type_errors.md](./type_errors.md#72-file-paths-string-vs-atom)
30. **Group By Duplicate Variables** → [analysis_errors.md](./analysis_errors.md#35-group-by-arguments-must-be-distinct-variables)

---

## How to Use This Reference

### For Developers
1. **Got an error?** Copy the error message
2. **Match pattern** Use error pattern regex in each file
3. **Read "What Causes It"** Understand the root cause
4. **Apply "How to Fix"** Follow the solution
5. **Check "Related Errors"** Catch cascade issues

### For LLM Prompt Engineering
Include relevant error reference sections in prompts:
```text
When writing Mangle rules:
1. All predicates MUST be declared (see undefined_errors.md)
2. Use /atom for enums, "string" for text (see type_errors.md)
3. Variables UPPERCASE, atoms /lowercase (see parse_errors.md)
4. Negation uses !, not \+ (see parse_errors.md#34)
5. Aggregation: |> do fn:Count(), not count() (see undefined_errors.md#215)
```

### For Error Recovery Loops
```go
// Classify error, inject appropriate reference
if strings.Contains(err, "not declared") {
    // Inject undefined_errors.md section
} else if strings.Contains(err, "stratification") {
    // Inject analysis_errors.md#1
} else if strings.Contains(err, "type mismatch") {
    // Inject type_errors.md#1
}
```

---

## Maintenance

**When adding new errors:**
1. Add to appropriate category file
2. Update this README index
3. Add to flowchart if major category
4. Update "Top 30" if frequent in AI generation

**When updating:**
- Keep exact error messages from Mangle source
- Include minimal reproducing examples
- Show both wrong and correct code
- Link related errors

---

## Regex Search Patterns

To find errors in logs:

```python
# Parse errors
r'(\d+):(\d+)\s+(parse error|syntax error|token recognition)'

# Analysis errors
r'(stratification|variable.*not.*bound|unsafe)'

# Type errors
r'(type mismatch|could not unify|expected.*got)'

# Undefined errors
r'(not declared|not found|undefined)'

# Gas errors
r'(limit exceeded|timeout|budget exhausted)'
```

---

## Contributing

Found an error not documented here?
1. Add to appropriate file with:
   - Error pattern (regex if variable)
   - Exact message example
   - Reproducing code
   - Fix
   - Related errors
2. Update this README index
3. Test fix works

---

## Related Documentation

- [syntax_core.md](../syntax_core.md) - Complete syntax reference
- [patterns_comprehensive.md](../patterns_comprehensive.md) - Best practices
- [antipatterns/](../antipatterns/) - What NOT to do
- [codenerd/schema_validator.md](../codenerd/schema_validator.md) - Pre-validation system


> *[Archived & Reviewed by The Librarian on 2026-01-25]*