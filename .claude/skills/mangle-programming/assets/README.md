# Mangle Programming Skill Assets

Templates and worked programs. Every `.mg` file here loads on the pinned engine
(codeberg.org/TauCeti/mangle-go v0.5.1-0.20260413190942-4dcaa582c6d3) and was evaluated with
`nerd check-mangle --standalone --eval` on 2026-09-19. Check a copy the same way after you edit it.

For codeNERD's own declarations, read the live files in `internal/core/defaults/schemas*.mg`
and `internal/core/defaults/policy/`; this directory does not keep a copy of them (a copy drifts,
and a second `Decl` of a predicate the kernel already declares is a hard load error).

## Templates

### starter-schema.mg + starter-policy.mg

A pair that loads as one unit: the schema declares every predicate once (EDB and derived), the
policy holds the rules.

```bash
cat starter-schema.mg starter-policy.mg > program.mg
nerd check-mangle --standalone --eval reachable,sink_node,leaf_node program.mg
```

The policy covers transitive closure, list-building paths (`fn:list:cons`), set difference by
projection (`has_out_edge(N) :- edge(N, _).` then `!has_out_edge(N)`, never `!edge(N, _)`),
aggregation, classification, map and list access, integer-timestamp validity, cycle detection
and an average (`fn:avg` yields a float, so its column is `/float64`).

## Examples

### examples/vulnerability-scanner.mg

Dependency tracking (transitive), CVE propagation, patched-version exclusion, vulnerability paths,
counts, and secure projects by projection.

### examples/access-control.mg

RBAC with role inheritance, explicit denials, owner override, an `action/1` domain so "any action"
rules have their head variables bound, and unprotected resources by projection.

### examples/aggregation-patterns.mg

Count, sum, min and max, multi-variable grouping, averages, filtering before aggregation (a
comparison in the body), aggregation over joins, nested aggregation and existence checks.

## Go integration

### go-integration/

`main.go` embeds the engine directly: parse, analyse, evaluate, add a fact, re-evaluate, query.
It was built and run against the pinned version inside the codeNERD module (2026-09-19). Standalone:

```bash
cd go-integration
go mod tidy
go run main.go
```

codeNERD itself wraps the engine in `internal/mangle` (`Engine.LoadSchemaString`, `Evaluate`,
`GetFacts`, `AtomStrings`); read that package before embedding the engine again.
