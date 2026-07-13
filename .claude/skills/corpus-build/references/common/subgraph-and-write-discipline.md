# Package-scope Isolation + Read-Before-Write Discipline

Distilled from the two MANDATORY sections in the repo-root `CLAUDE.md`. These
are not style preferences — violating either corrupts a live, shared,
multi-tenant database that other NeuroLog client apps depend on.

## MANDATORY — Package/fact-space isolation

A single running codeNERD instance serves every NeuroLog client app
(proof-forge, neuroCode, demos, experiments, external consumers) out of one
shared graph. Every writing caller MUST scope its nodes/edges/certificates
under a named package-scope (e.g. `proof-forge`, `neuroCode`,
`corpus-build-telemetry`).

- An unscoped write silently corrupts sibling apps' state: a node written
  without a package-scope tag gets joined into every other app's traversal at
  query time. There is no error, no warning — just wrong answers downstream,
  possibly discovered much later by a different team.
- Enforce package-scope filtering in **every** query path, every certificate
  attach, every publication bundle, every cross-domain edge you add or
  touch.
- When reviewing or adding API surface: if an endpoint accepts writes
  without a package-scope parameter, that is a bug — surface it in the
  completion report, do not silently work around it.
- `domain.Edge` historically has NO package-scope field in some code paths
  (known gap, e.g. causal projection) — check for this specific hole before
  assuming isolation is enforced just because the node side looks scoped.

## MANDATORY — Read before write (upsert, don't insert)

codeNERD is durable/persistent. Before creating any persistent record (node, edge,
certificate, blob, publication):

1. **Query the package-scope first** to check for an existing equivalent.
   Duplicate valid-time insertions fragment provenance and break Allen's
   Interval Algebra invariants — this is not a performance nit, it is a
   correctness invariant.
2. **Compute the delta**, apply only the delta. Never blindly overwrite —
   blind overwrites destroy cross-app causality chains that other
   subsystems (consolidation cycle, codeNERDRAG/RAP, attention-routing) depend on to reason
   about "what changed and why."
3. This applies identically to code reviews, migration scripts, seed
   commands, and any agent operating on the live database — not just
   application-layer writers.

## corpus-build-specific application

- `corpus-critic` gates every builder work unit specifically on these two
  rules (package-scope-isolation and read-before-write (persistent store) conformance) — a work
  unit that adds a write path without a package-scope parameter or without a
  preceding read/existence-check fails review regardless of test coverage.
- If a work unit's storage/telemetry writes are intentionally global (e.g.
  a genuinely cross-cutting ledger), say so explicitly and justify it in
  the completion report rather than leaving it ambiguous.
