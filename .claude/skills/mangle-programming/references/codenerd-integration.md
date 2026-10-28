# Mangle inside codeNERD

## Authority split

Go measures, asserts and executes, and Mangle judges. Don't reimplement a
policy branch in Go because a rule is inconvenient, and don't make an LLM
the authority for something the facts can decide.

Go keeps:
- measurement of the filesystem, processes and network;
- parsing outside formats into ground facts;
- the normalization needed to state a fact truthfully;
- executing what Mangle permitted;
- rendering and logging.

Mangle keeps:
- permission and safety;
- routing and shard or tool selection;
- freshness and lifecycle;
- ranking, thresholds, budgets and conflict resolution;
- graph closure;
- what counts as complete or blocked.

Config values enter as `config_param` facts. A Go constant for a policy
threshold is a bug.

## Program layers

The action kernel assembles schemas, then policy, then learned rules on
every `rebuildProgram`. That order is part of the trust model, and a parse
or analysis error anywhere is a boot failure everywhere.

| Layer | Where |
| --- | --- |
| Decls and shared vocabulary | `internal/core/defaults/schemas*.mg`. One Decl per predicate, in one file. |
| Constitutional and policy derivations | `internal/core/defaults/policy/*.mg` and the topic files beside them |
| Orientation (separate engine) | `internal/orient/*.mg` |
| Package-local programs | search `//go:embed` and program builders |
| Go-backed predicates | `internal/core/external_predicates.go` (VirtualStore) |
| Generated rules | `internal/mangle/feedback`, gated by `kernel_rule_bounds.go` |

An external callback without a matching external Decl isn't wired. An
external Decl without a registered callback is an incomplete program.

## Adding one judgment end to end

1. Search the Decls and rules for an existing predicate with the same
   meaning.
2. Run the experiment in a lab (`lab-protocol.md`). Decide which unique
   feature carries it.
3. Add or extend the Decl in the owning schema file.
4. State the Go measurements as ground facts with exact name/string and
   int/float shapes.
5. Add the smallest rule set that derives the judgment.
6. Query it from the production consumer, and delete the Go branch it
   replaces.
7. Add a table test through `NewRealKernel` (or the owning engine) with a
   positive case, a near miss, and the negation or stratification case.
8. Run the Decl budget tests. Undeclared asserts are budgeted defects.
9. If prompt atoms teach this predicate, update the atom source and its
   mirrors in the same change.

## Fact admission traps

- Wrong arity means a different predicate. `validateAgainstDeclLocked`
  warns instead of rejecting, because legacy producers exist. New work
  must not add to that count.
- `sanitizeFactForNumericPredicates` doesn't excuse loose typing. Build
  the declared representation at the producer.
- Retraction changes negation and aggregate results. The kernel rebuilds
  from a fresh EDB for full evaluation. Don't reintroduce a path that
  reuses derived facts.
- Cone evaluation must stay identical to full evaluation. A rule shape
  that defeats cone closure needs a differential test.
- Decl bounds aren't checked on the hand-written corpus
  (`types-as-contracts.md`). A green kernel boot says nothing about types.

## Proofs and Glass Box

`kernel_explanation.go` serves "why did the kernel decide this?" from
proofs. Prefer proof data to a rationale an LLM reconstructs. Post-hoc
proofs cover rules, recursion, equality and negation. `let`/`do` evidence
needs recorder mode. Don't put a recorder on a hot path without measuring
(`proofs-as-data.md`).

## Verification commands

```bash
CGO_CFLAGS="-I$PWD/sqlite_headers" go test -tags sqlite_vec -count=1 ./internal/mangle/...
CGO_CFLAGS="-I$PWD/sqlite_headers" go test -tags sqlite_vec -count=1 ./internal/core/
CGO_CFLAGS="-I$PWD/sqlite_headers" go build ./... && go build -o nerd ./cmd/nerd
./nerd check-mangle internal/core/defaults/policy/<file>.mg
```

Narrow `-run` is for iteration. Before calling it done, run the whole
owning package and at least one real-kernel boot test. Rebuild the binary
after any `.mg` change: schemas and policy are embedded with `go:embed`.
