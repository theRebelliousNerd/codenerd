# Open Questions — `internal/types`

> Last verified: **2026-08-15**

## Q1 — When to delete `KernelInterface`? — **ANSWERED (in progress)**

**Context:** Narrow bridge exists for autopoiesis and cycle avoidance; full `Kernel` is richer.  
**Decision:** (c) force all callers onto `Kernel`, in three revertible steps documented on
`KernelInterface` in `types.go`.  
**Blocker resolved:** the import-graph proof is trivial — `internal/autopoiesis` already imports
`internal/types`, and `Kernel` adds no new dependency beyond `mangle-go/analysis`, which types already
imports. There was never a cycle to reintroduce.  
**Status:** step 1 done (`KernelFact` is now an alias for `Fact`). Step 2 (autopoiesis switches its
`Orchestrator.kernel` field to `Kernel`) belongs to that package's owner. Step 3 deletes
`KernelInterface`, `Fact.ToFact` and `core.AutopoiesisBridge`. `cmd/nerd/cmd_mcp_select.go`'s
`cliMCPKernel` satisfies `mcp`'s own interface and is a genuine edge adapter that stays.

## Q2 — Should `NewKernelTx` return `error` instead of panicking? — **ANSWERED**

**Decision:** keep the panic; add a non-panicking probe instead. `types.TransactorOf(k)` returns
`(KernelTransactor, bool)` for code holding a Kernel of unknown provenance, and the panic message now
names the concrete type and prints the one-line forwarding fix — because the real-world failure is not
"someone wrote a weird kernel", it is a forwarding adapter that dropped one method.  
`typestest.MockKernel` removes the test-only motivation for an error return.

## Q3 — How strict should name-constant heuristics stay?

**Context:** Slash-count ≤ 2 and extension filters prevent path-as-atom bugs.  
**Risk:** Legitimate hierarchical atoms deeper than two segments would be misclassified.  
**New evidence:** the Decl-conformance sweep (`fact_conventions_guard_test.go`) shows the live problem
is the opposite direction — 11 sites pass a plain string into a slot the corpus declares `/name`, and
shape inference cannot save them because the string is not name-shaped at all. Heuristic strictness is
not the bottleneck; explicit `MangleAtom` at the assert site is.  
**Lean:** leave the heuristic; keep pushing call sites to `MangleAtom` / `MangleString`.

## Q4 — Is `SessionContext` a god-struct problem? — **ANSWERED**

**Decision:** stays flat. Nesting is a rename of ~40 fields across the packages that populate and read
them, for navigability the section banners already provide. Revisit when a section needs its own
behaviour (methods, zero-value semantics, independent serialization). Rationale is on the struct.

## Q5 — Where do shared test mocks live? — **ANSWERED**

**Decision:** `internal/types/typestest`, a sibling package that imports `internal/types` and nothing
else from this repo, so it can never cycle and never ships in a production binary. It holds
`MockKernel` (`Kernel` + `KernelTransactor`) with compile-time assertions for both.
Migrating the ~14 hand-written mocks is per-package work.

## Q6 — Expand `VirtualStore` interface how far? — **ANSWERED**

**Decision:** ≥2 packages outside `core` must need a method before it is added, and it is added in the
shape they need rather than mirrored from `*core.VirtualStore`. Each addition obliges three adapters to
implement it, and an adapter with nothing to return implements a stub — which is how a single-consumer
method becomes three silent nil paths. Policy recorded on the interface.

## Q7 — JSON containers as first-class Mangle types?

**Context:** Maps/slices become JSON strings.  
**New evidence:** the encoding is now pinned by table tests (`container_toatom_test.go`), including key
ordering, nil-vs-empty, and the named error for unencodable contents — so it is a contract that can be
migrated deliberately rather than an accident of `ToAtom`'s default branch.  
**Future:** if Mangle gains native compound types, revisit without breaking stored facts.

## Q8 — Should `Fact.String` and `ToAtom` share one renderer? — **NEW, OPEN**

**Context:** They already disagree. `Fact.String` renders a container with `%v` (`map[a:b]`) while
`ToAtom` JSON-encodes it, and `Fact.String` renders floats with `%f` while the Mangle AST renders
`Float64(2.0)` as `2`, which re-parses as `int64`.  
**Why it matters:** `Fact.String` output is not display-only — `northstar.RenderVisionMangle` writes it
into a `.mg` file the kernel loads at boot. The `%f` verb is load-bearing and now has a test saying so.  
**Options:** (a) make `Fact.String` delegate to `ToAtom(…).String()` and fix the float rendering
upstream; (b) make `Fact.String`'s container branch JSON-encode like `ToAtom`; (c) leave both, keep the
tests.  
**Lean:** (b) first — it is additive and removes the only known lossy divergence — then (a) once the
upstream float rendering is settled.
