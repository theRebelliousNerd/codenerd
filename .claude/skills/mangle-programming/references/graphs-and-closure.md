# Graphs and closure: where recursion earns its keep

Recursion over a graph is where Mangle is strongest. The Mangle-specific
twist is what you can carry along the path (lists, structs, costs, time
intervals) and how you can prune it (lattices) while it's still being
computed.

## Patterns

- **Permission inheritance (Zanzibar-style)**: `relation(Object, Rel,
  Subject)`, group membership closed recursively, objects inheriting from
  parents, and roles implying roles (owner ⊇ editor ⊇ viewer). codeNERD's
  `permitted(...)` is a policy closure, and this is the scalable shape for
  it: shard ⊂ campaign ⊂ session scopes, and tools inheriting from tool
  classes.
- **Connected components with leader election**: link nodes symmetrically,
  close, and name each component by its smallest member. The `fn:min`
  aggregate needs a closed closure first. A `min` merge lattice
  (`lattices-and-merge.md`) labels while closing, which is
  label-propagation without iteration code. Measure both.
- **Weighted shortest paths**: the Vectryx skill calls this a Mangle
  limit, and for plain Datalog it is: summing costs around cycles never
  stops. A `fundep` plus `merge` keeping the cheaper cost per (source,
  target) changes that. The engine's `shortest_path.mg` does it for hop
  count. Do it for real costs, with a cycle in the graph, and confirm it
  terminates and matches Dijkstra in Go.
- **Path witnesses**: carry `[X, Y, ...]` as a list so the answer includes
  the route. Cut cycles with a "not already on the path" check built on
  `fn:list:contains` (probe how to negate it; `:filter` wants a `/true`
  result) or with a lattice, so cycles don't keep producing new lists.
- **Cycle detection**: `reach(X, X)` over dependency, wait-for or import
  edges. Report the cycle with a witness path.
- **Temporal reachability**: edges valid over intervals. A path exists only
  where its edges' intervals overlap (`examples/temporal_graph_intervals.mg`).
  Example: which shard could have influenced this file during this
  campaign phase.
- **Same-generation and set relations**: `examples/reversesamegen.mg` and
  `ancestor.mg` are the classics. They're useful for "siblings in the task
  tree" questions.

## Cost control

- Close only in the direction the question needs. A symmetric closure
  over a dense graph is quadratic however elegant the rule.
- Restrict both ends: `reach(S, T)` for the 3 starts you care about, not
  all pairs. Seed recursion with a selective base case.
- Path lists in a recursive head grow the universe of values. A lattice,
  a depth bound carried as a number, or a cycle check must cut them off.
  Remember that termination isn't guaranteed by analysis
  (`verified-traps.md`).
- Cone evaluation in the kernel limits re-evaluation to predicates that
  depend on what changed. Recursion that fans out across unrelated
  predicates defeats it.

## Experiments to push

1. Weighted shortest path with a merge lattice on a cyclic graph:
   termination, correctness against Go Dijkstra, and time at 100k edges.
2. Model codeNERD's permission scopes as inheritance and diff the result
   against today's `permitted` facts over a real session EDB.
3. Leader election with a `min` lattice versus closure plus `fn:min`, at
   100k nodes.
