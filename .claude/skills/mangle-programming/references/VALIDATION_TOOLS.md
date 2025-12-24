# Mangle Validation Tools

Comprehensive validation and analysis tools for Mangle programs and Go integration code.

## validate_mangle.py - Mangle Syntax Validator

Comprehensive validation for Mangle source files:

```bash
# Basic validation
python3 scripts/validate_mangle.py program.mg

# Strict mode (checks undeclared predicates)
python3 scripts/validate_mangle.py program.mg --strict

# Validate inline code
python3 scripts/validate_mangle.py --check-string "parent(/a, /b)."

# Verbose output (shows stratification info)
python3 scripts/validate_mangle.py program.mg --verbose
```

**Checks performed:**

- Syntax validation (periods, balanced brackets, arrow operators)
- Declaration syntax (Decl with .Type<> and modes)
- Aggregation pipelines (|> do fn: let)
- Safety constraints (head variables bound in body)
- Negation safety (variables bound before negation)
- Built-in function validation (fn:Count, fn:Sum, etc.)
- Basic stratification analysis

## diagnose_stratification.py - Stratification Diagnostic Tool

**Deep analysis for stratification issues** - the #1 cause of "unsafe" or "cannot compute fixpoint" errors:

```bash
# Analyze a file for stratification issues
python3 scripts/diagnose_stratification.py program.mg

# Detailed dependency analysis
python3 scripts/diagnose_stratification.py program.mg --verbose

# Output dependency graph in DOT format (for Graphviz)
python3 scripts/diagnose_stratification.py program.mg --graph > deps.dot

# JSON output for tooling integration
python3 scripts/diagnose_stratification.py program.mg --json

# Check inline code
python3 scripts/diagnose_stratification.py --check-string "bad(X) :- !bad(X)."
```

**What it detects:**

| Pattern | Detection | Fix Suggestion |
|---------|-----------|----------------|
| Direct self-negation | `bad(X) :- !bad(X)` | Use base case or split predicates |
| Mutual recursion | `winning -> losing -> !winning` | Add terminal conditions |
| Game theory cycles | Classic minimax patterns | Provides stratified rewrite |
| Complex negative cycles | Multi-predicate cycles | Suggests helper predicates |

**Output includes:**

- Stratum assignment for all predicates
- Cycle path for violations
- Line numbers where issues occur
- Targeted fix suggestions based on violation type
- DOT graph visualization for dependency analysis

**Exit codes:**

- `0` - Program is stratified (safe)
- `1` - Stratification violations found
- `2` - Parse/fatal error

## dead_code.py - Dead Code Detection Tool

**Finds rules that can never fire and unreachable/unused code**:

```bash
# Check policy and schema files
python3 scripts/dead_code.py policy.mg schemas.mg

# Full report (default)
python3 scripts/dead_code.py policy.mg schemas.mg --report

# JSON output for tooling
python3 scripts/dead_code.py *.mg --json > dead_code.json

# Only show unused predicates
python3 scripts/dead_code.py policy.mg --unused-only

# Only show undefined predicates
python3 scripts/dead_code.py policy.mg --undefined-only

# Ignore virtual/builtin predicates
python3 scripts/dead_code.py policy.mg --ignore query_learned --ignore recall_similar

# Verbose output with suggestions
python3 scripts/dead_code.py policy.mg --verbose
```

**What it detects:**

| Issue Type | Description | Severity |
|------------|-------------|----------|
| Unreachable rules | Rules with undefined body predicates | Error |
| Unused predicates | Predicates defined but never referenced | Warning |
| Undefined predicates | Predicates used but never defined | Error |
| Shadowed rules | Rules that may be unreachable due to earlier rules | Info |

**Exit codes:**

- `0` - No dead code detected
- `1` - Dead code found
- `2` - Parse/fatal error

## trace_query.py - Query Evaluation Tracer

**Step-by-step query debugging** - answers "why doesn't my rule derive anything?":

```bash
# Basic query tracing
python3 scripts/trace_query.py policy.mg --query "next_action(X)"

# Query with seed facts
python3 scripts/trace_query.py policy.mg --query "next_action(X)" \
    --facts "user_intent(/id1, /query, /read, /foo, _). test_state(/failing)."

# Verbose tracing
python3 scripts/trace_query.py policy.mg --query "block_commit(X)" -v

# Inline code
python3 scripts/trace_query.py --check-string "a(X) :- b(X). b(1)." --query "a(X)"
```

**What it shows:**

| Output | Description |
|--------|-------------|
| Rules attempted | Which rules could potentially match |
| Variable bindings | How variables bind during evaluation |
| Body predicate status | Which body predicates succeed/fail |
| Final results | Derived facts or failure explanation |

**Exit codes:**

- `0` - Query succeeded with results
- `1` - Query failed (no results)
- `2` - Parse/usage error

## explain_derivation.py - Predicate Lineage/Provenance

**Proof tree visualization** - answers "how would X be derived?" and "why is X true?":

```bash
# Explain a fully ground fact
python3 scripts/explain_derivation.py policy.mg schemas.mg \
    --explain "delegate_task(/coder, \"fix bug\", /pending)"

# Explain with variables (all possible derivations)
python3 scripts/explain_derivation.py policy.mg --explain "next_action(X)"

# Show all derivation paths
python3 scripts/explain_derivation.py policy.mg \
    --explain "next_action(/run_tests)" --all-paths

# Export to JSON
python3 scripts/explain_derivation.py policy.mg \
    --explain "permitted(/fs_read)" --json
```

**Output includes:**

- Complete proof tree from goal to base facts
- Which rules contribute to the derivation
- Variable bindings at each step
- All alternative derivation paths (with `--all-paths`)

## analyze_module.py - Cross-File Module Analyzer

**Multi-file coherence analysis** - detects cross-file issues in large Mangle codebases:

```bash
# Analyze all core policy files
python3 scripts/analyze_module.py internal/core/defaults/*.mg

# Check completeness (fail on undefined predicates)
python3 scripts/analyze_module.py *.mg --check-completeness

# With known virtual predicates (implemented in Go)
python3 scripts/analyze_module.py *.mg --check-completeness \
    --virtual "file_content,symbol_at,recall_similar"

# Generate dependency graph
python3 scripts/analyze_module.py *.mg --graph | dot -Tpng > deps.png

# JSON output for tooling
python3 scripts/analyze_module.py *.mg --json
```

**What it detects:**

| Issue Type | Description | Severity |
|------------|-------------|----------|
| Missing definitions | Predicate used but never defined | Error |
| Duplicate definitions | Same predicate defined in multiple files | Warning |
| Arity mismatches | Same predicate with different arities | Error |
| Unused exports | Defined but never referenced elsewhere | Info |
| Circular dependencies | File A depends on B depends on A | Warning |

## generate_stubs.py - Go Virtual Predicate Stub Generator

**Generates idiomatic Go stubs** for Mangle virtual predicates:

```bash
# Generate stubs for all predicates
python3 scripts/generate_stubs.py internal/core/defaults/schemas.mg \
    --output stubs.go

# List all predicates with arities
python3 scripts/generate_stubs.py schemas.mg --list

# Generate only virtual predicates
python3 scripts/generate_stubs.py schemas.mg --virtual-only \
    --output virtual_preds.go

# Generate specific predicates
python3 scripts/generate_stubs.py schemas.mg \
    --predicates "user_intent,recall_similar" --output user_preds.go
```

**Type mappings:**

| Mangle Type | Go Type |
|-------------|---------|
| `Type<n>`, `Type<name>` | `engine.Atom` |
| `Type<string>` | `engine.String` |
| `Type<int>` | `engine.Int64` |
| `Type<float>` | `engine.Float64` |
| `Type<[T]>` | `engine.List` |
| `Type<{/k: v}>` | `engine.Map` |
| `Type<Any>` | `engine.Value` |

## profile_rules.py - Performance Analyzer

**Static analysis for Cartesian explosion risks** and expensive rule patterns:

```bash
# Basic analysis
python3 scripts/profile_rules.py policy.mg

# CI/CD check (fail on high-risk rules)
python3 scripts/profile_rules.py policy.mg --warn-expensive

# Only show medium+ severity
python3 scripts/profile_rules.py policy.mg --threshold medium

# Show rewrite suggestions
python3 scripts/profile_rules.py policy.mg --suggest-rewrites

# With predicate size estimates
echo '{"big_table": 10000, "filter": 100}' > sizes.json
python3 scripts/profile_rules.py policy.mg --estimate-sizes sizes.json
```

**What it detects:**

| Pattern | Risk | Description |
|---------|------|-------------|
| Cartesian products | High | Multiple large predicates before filter |
| Unbounded joins | Medium | Predicates without shared variables |
| Nested aggregation | Medium | Complex `\|>` pipelines |
| Large recursive rules | Medium | Recursive predicates with wide bodies |
| Non-indexed lookups | Low | Missing selectivity optimizations |

**Example Cartesian explosion:**

```mangle
# BAD: 10K × 10K = 100M combinations before filter!
result(X, Y) :- big_table(X), big_table(Y), filter(X, Y).

# GOOD: Only 100 filter matches × 2 lookups = 200 operations
result(X, Y) :- filter(X, Y), big_table(X), big_table(Y).
```

## validate_go_mangle.py - Go Integration Validator

Validates Go code that uses the Mangle library:

```bash
# Validate single file
python3 scripts/validate_go_mangle.py internal/mangle/engine.go

# Validate directory
python3 scripts/validate_go_mangle.py internal/

# Validate entire codebase
python3 scripts/validate_go_mangle.py --codebase /path/to/project
```

**Checks performed:**

- Correct github.com/google/mangle/* imports
- Proper AST type handling (Constant.Type checks)
- Engine API usage patterns (EvalProgram, QueryContext)
- Error handling for parse/analysis operations
- Fact/Atom construction correctness
- codeNERD-specific patterns (VirtualStore, ToAtom)

## generate_template.py - Template Generator

Generate boilerplate Mangle files:

```bash
python3 scripts/generate_template.py schema > schemas.gl
python3 scripts/generate_template.py policy > policy.gl
```
