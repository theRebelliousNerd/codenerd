# Anti-Hallucination Gate

Architecture docs and even `IMPLEMENTED_SPEC.md` can drift from code reality
(see `docs/architecture/CLAUDE.md`'s decode matrix: "Yes in code / No in
IMPLEMENTED_SPEC" is a real, common state, not an edge case). A spec-driven
fleet that trusts prose over the compiler produces code that references
types, methods, predicates, and endpoints that do not exist. This gate is
mandatory for every corpus-build fleet agent before it writes a line of Go,
TypeScript, or Mangle that names a symbol it did not itself just verify.

## The rule

Before using ANY symbol name pulled from a spec doc, a prior agent's report,
or your own memory of a similar subsystem — grep-verify it against the
actual source first:

1. **Symbols** (types, funcs, methods, constants): `Grep` the exact
   identifier in the target package before calling or extending it. A spec
   that says "call `Graph.TraverseScoped`" is a hypothesis, not a fact,
   until the grep confirms the signature.
2. **Predicates** (Mangle `.mg` rules): confirm the predicate name and arity
   exist in the loaded ruleset — never author a call to a predicate you
   have not located in `internal/deductive/` or the subsystem's `.mg`
   files.
3. **Endpoints / routes**: confirm via the live route snapshot or
   `internal/api/rest/routes_*.go`, not via an aspirational
   `NN-ENGINE-INTEGRATION-SURFACE.md` doc — those describe target state,
   not shipped surface (see `docs/architecture/CLAUDE.md` two-layer model).
4. **Peer output**: another agent's completion report is not ground truth.
   If a prior work unit claims "added `Foo.Bar()`", grep for `Foo.Bar` in
   the actual diff/file before building on it.

## UNVERIFIED flagging protocol

When time or scope does not allow full verification of a referenced
symbol/behavior:

- Mark the exact line with `// UNVERIFIED: <what wasn't confirmed and why>`
  in the code, and surface it explicitly in the completion report's
  "interface assumptions" section (see `reporting-format.md`).
- Never silently ship an unverified assumption as if it were confirmed —
  that is the exact failure mode this gate exists to prevent.
- A build/test gate passing does NOT retroactively verify an assumption
  that was never grep-checked (a mock or stub can compile and pass while
  hiding a wrong assumption underneath it).

## Named-algorithm caveat

Code or docs claiming a known algorithm/pattern by name ("uses A* search",
"Struc2Vec embedding") may implement something materially different. Read
the actual computation before repeating the claim in your own output or
attribution comment.
