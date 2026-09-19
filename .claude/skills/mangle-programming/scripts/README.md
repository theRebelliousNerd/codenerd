# Mangle tools

The verdict on any Mangle program comes from the pinned engine, through the `nerd` binary:

```bash
# Check: parse, declarations, arity, variable safety, stratification (the kernel's own analysis).
# codeNERD's shared schemas (internal/core/defaults/schemas*.mg) load first, so a policy file is
# checked in the context the kernel loads it in.
nerd check-mangle internal/core/defaults/policy/*.mg

# Check a program on its own (examples, probes): no schema preload.
nerd check-mangle --standalone probe.mg

# Run it and print what the named predicates hold afterwards, in the engine's own spelling
# (a name /a and a string "/a" stay distinct). This is how a rule's behaviour is found out.
nerd check-mangle --standalone --eval sibling,has_stop probe.mg
```

`--eval` is the tracer: when a rule derives too much or nothing, evaluate each predicate in its
chain and find the first one whose facts are wrong. The engine's silent behaviours (a wildcard
negation deleted from its clause, a name constant absorbing the clause's period, a float in a
comparison) only show up this way; see `references/020-ENGINE_TRUTHS_v0.5.1.md`.

## Advisory helpers

Two Python scripts remain. They parse Mangle with their own grammar, so they explain and
suggest; the engine decides. Both were checked against it on 2026-09-19.

### `diagnose_stratification.py`

Names the cycle when a program cannot be stratified. The engine only says "program cannot be
stratified"; this says which predicates form the negative cycle.

```bash
python diagnose_stratification.py policy.mg
python diagnose_stratification.py policy.mg --verbose --graph > deps.dot
```

### `profile_rules.py`

Flags rules whose join order risks a large intermediate result. On the pinned engine body order is
execution order (a nested loop as written), so a selective, bound atom belongs first. It reads
estimated predicate sizes from `sizes.json`; `examples/performance_antipatterns.mg` shows what it
flags.

```bash
python profile_rules.py policy.mg --warn-expensive
python profile_rules.py policy.mg --estimate-sizes sizes.json
```

## Removed 2026-09-19

Each of these disagreed with the pinned engine on a basic case, so an agent that trusted it wrote
wrong Mangle: `mangle-cli.js` (reported an error on `Decl p(A, B) bound [/name, /name].`, the
form every codeNERD policy file uses), `validate_mangle.py` (VALID for a file with a duplicate
`Decl`), `trace_query.py` and `explain_derivation.py` (no result, and a negation read as a
positive premise, for a rule the engine evaluates), `analyze_module.py` and `dead_code.py`
(FAILED and "unused" for predicates other files consume), `validate_go_mangle.py` (missing
imports that are present; advice that reintroduces the name/string confusion),
`generate_stubs.py` (imports `github.com/google/mangle`) and `generate_template.py` (three of
four templates fail analysis).
