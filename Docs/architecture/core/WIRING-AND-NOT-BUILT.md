# core: wiring and what is NOT built

Verified 2026-09-21 against commit `3463477` (`main`). Read from
`internal/core/kernel_facts.go`, `kernel_eval.go`, `kernel_provenance.go`,
`kernel_validation.go`, `kernel_sysfacts.go`, `virtual_store.go`,
`virtual_store_routing.go`, and the boot-guard audit in
`Docs/journeys/01-docs-audit.md:114,121`.

## Wired and reachable

- Action routing evaluates the boot guard first (`virtual_store_routing.go:41-43`),
  then Dreamer simulation, constitution, allowlists, and validators. Default is
  deny: nothing routes until policy derives `permitted`.
- The guard is a switch, not a derivation: `DisableBootGuard` /
  `IsBootGuardActive` (`virtual_store.go:395-408`). The chat host releases it
  after startup at `cmd/nerd/chat/process.go:151-155`. While it is engaged,
  every route is denied regardless of policy.
- There are two guards, not one: the `VirtualStore` field above and a sibling
  `ExecutivePolicyShard` guard. The docs audit
  (`Docs/journeys/01-docs-audit.md:114,121`) records that the second guard was
  missed, stayed engaged, and cost a 13-minute debug; both must release before
  anything routes.
- The fact path is one chain: `Assert` validates (`ValidateFact`,
  `kernel_facts.go:186-278`), canonicalizes (`canonFact`,
  `kernel_facts.go:155-167`), and dedups (`addFactIfNewLockedErr`,
  `kernel_facts.go:442-486`); `rebuildProgram` (`kernel_eval.go:61-156`) then
  `evaluate` (`kernel_eval.go:171-306`) derive consequences; `Query`
  (`kernel_query.go:24-142`) reads them back.
- Git state reaches policy through `UpdateSystemFacts` /
  `parseGitStatus` (`kernel_sysfacts.go:24-107`, `kernel_sysfacts.go:193-223`).

## Exists but nothing in production calls (or no caller traced in this pass)

- `Explain` (`kernel_provenance.go:72-112`): reachable only after
  `EnableProvenance` (`kernel_provenance.go:29-36`); no production caller
  traced in this pass enables it, so derivation trees are available but
  unpopulated in normal runs.
- `Clear` / `Reset` / `Clone` (`kernel_eval.go:392-493`): kernel lifecycle
  helpers with no production caller traced in this pass; they serve tests and
  support, not the routing path.
- `GetStartupValidationResult` (`kernel_validation.go:730-740`): the startup
  verdict accessor exists, but no consumer of it was traced in this pass.
- `healLearnedRules` (`kernel_validation.go:557-688`) runs inside load-time
  validation; it is not a runtime repair loop and nothing invokes it outside
  that flow.

## Assumed by the design, not done by the code

- Numbers are int64-only by convention plus scrubbing
  (`sanitizeFactForNumericPredicates`, `kernel_facts.go:1161-1186`): one
  float64 fact reaching the pinned fork aborts the whole kernel fixpoint, so
  every producer upstream of `Assert` must hold the invariant — the store
  scrubs what it recognizes, it does not coerce.
- `rebuildProgram` trusts concatenation order (schemas, then policy, then
  learned); conflicting `Decl permitted(` lines across those layers are a
  boot-time authoring error surfaced via the debug dump, not resolved by
  precedence logic in the code.
- The host is trusted to release both boot guards. A host that never calls
  `DisableBootGuard` gets a fully denied router with no error pointing at the
  guard — silence, not a diagnostic.

## Deliberately not built

- No metrics exporter among the kernel symbols: counters exist where the
  subsystems keep them (routing, sharding, scheduling), but nothing in the
  `kernel_*.go` surface exports them to an external telemetry system.
- No runtime rule repair: `healLearnedRules` runs once at load
  (`kernel_validation.go:557-688`); a rule that degrades mid-run is not
  re-validated or healed.
- No guard diagnostics: neither guard reports *which* guard denied a route;
  the deny path does not distinguish "boot guard engaged" from "policy said
  no".
