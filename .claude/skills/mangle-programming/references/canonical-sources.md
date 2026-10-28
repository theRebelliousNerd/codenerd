# Canonical sources and where to dig

## Authority order

1. The TauCeti repository, https://codeberg.org/TauCeti/mangle-go. It is
   canon for this project; don't call it "the fork". Google's original
   Mangle explains the lineage only.
2. The exact module in `go.mod`: `codeberg.org/TauCeti/mangle-go v0.7.0`.
3. Its source and tests. When prose and code disagree, the code wins.
4. Its docs, `docs/` and `readthedocs/`. These have bugs; see
   `verified-traps.md`.
5. A lab run (`lab-protocol.md`). That's the final word for this repo.
6. codeNERD's own Decls, rules and tests, for project conventions.

```bash
MODDIR="$(go env GOMODCACHE)/codeberg.org/!tau!ceti/mangle-go@v0.7.0"
ls "$MODDIR/docs" "$MODDIR/readthedocs" "$MODDIR/examples"
sed -n '1,200p' "$MODDIR/CHANGELOG.md"
```

## Which upstream file answers what

| Question | File |
| --- | --- |
| Values, names, function constants | `docs/spec_datamodel.md`, `docs/structured_data.md` |
| Decl, bounds, descriptors (modes, fundep, merge) | `docs/spec_decls.md`, `readthedocs/declarations.md` |
| Type expressions, tagged unions | `readthedocs/typeexpressions.md`, `constructedtypes.md` |
| Transforms and reducers | `readthedocs/aggregation.md` (buggy example), `examples/map_aggregation.mg` |
| Built-ins | `docs/spec_builtin_operations.md`, `readthedocs/basictypes.md`, the `symbols/` and `builtin/` source |
| Time | `readthedocs/temporal.md`, `examples/temporal_*.mg`, `factstore/temporal.go` |
| Proofs | `readthedocs/provenance.md`, `provenance/` |
| Lattices | `examples/shortest_path.mg`, `engine/seminaivebottomup.go` (`mergeDelta`) |
| Go-backed predicates | `engine/seminaivebottomup.go` (`ExternalPredicateCallback`), `engine/premise.go` |
| Evaluation options | `engine/seminaivebottomup.go` (`With*`) |
| Bounds checking | `analysis/validation.go` (`AnalyzeAndCheckBounds`), `analysis/boundscheck.go` |
| Program analysis examples | `examples/dataflow_*.mg`, `examples/flow_checking.mg` |
| Interactive probing | `docs/using_the_interpreter.md` (`mg`, `--load`, `--exec`) |

## Mines (ideas, not authority)

- `/home/rebelliousnerd/codeProjects/Vectryx/.claude/skills/mangle-programming/`
  is an earlier, larger skill for the Vectryx project. Worth mining:
  - `references/950-ADVANCED_ARCHITECTURE.md`: permission inheritance,
    fraud-ring clustering, the neuro-symbolic mediator, an AST linter,
    taint, deadlock detection and a policy diff.
  - `references/455-EXTERNAL_PREDICATES.md` and
    `456-PREDICATE_IMPLEMENTATIONS.md`: callback templates.
  - `references/460-CONTEXT_INJECTION.md`: relevance scoring for prompt
    context.
  - `references/050-ARCHITECTURAL_PHILOSOPHY.md`: the cost of joins.
  - The `references/journal/` engine findings.
  - Python tools in `scripts/` (stratification diagnosis, derivation
    explainer, dead code).

  It targets an older engine and contains code that doesn't analyze on
  v0.7 (a float compared with `>`, a rule that negates its own head).
  Mine the ideas and run every snippet through a lab before trusting it.
- The engine's own `examples/` directory is the cheapest source of working
  shapes for the unique features.

## codeNERD sources of truth

- Schemas: `internal/core/defaults/schemas*.mg`. Policy:
  `internal/core/defaults/policy/*.mg`. Orientation: `internal/orient/*.mg`.
- Kernel adapter: `internal/core/kernel_eval.go`, `kernel_eval_cone.go`,
  `kernel_facts.go`, `kernel_rule_bounds.go`, `kernel_explanation.go`,
  `external_predicates.go`.
- General engine adapter: `internal/mangle/engine.go`.

A rule is wired only when its program loads, its inputs are asserted
with the declared shapes, its output is queried, and a test observes the
production boundary.
