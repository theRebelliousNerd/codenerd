# Open Questions — `internal/types`

> Last verified: **2026-07-13**

## Q1 — When to delete `KernelInterface`?

**Context:** Narrow bridge exists for autopoiesis and cycle avoidance; full `Kernel` is richer.  
**Options:** (a) Keep dual forever; (b) Make `KernelInterface` an alias subset via embedding; (c) Force all callers onto `Kernel`.  
**Blocker:** Need import-graph proof that autopoiesis can depend on the same interface without reintroducing cycles.

## Q2 — Should `NewKernelTx` return `error` instead of panicking?

**Context:** Fail-closed panic is deliberate after removing non-atomic fallback.  
**Tradeoff:** Recoverable errors help tests/mocks; panic catches incomplete production wiring early.  
**Lean:** Keep panic for production honesty; provide a test-only constructor if needed.

## Q3 — How strict should name-constant heuristics stay?

**Context:** Slash-count ≤ 2 and extension filters prevent path-as-atom bugs.  
**Risk:** Legitimate hierarchical atoms deeper than two segments would be misclassified.  
**Options:** Document max hierarchy depth; require `MangleAtom` for anything non-trivial; or consult Mangle `ast.Name` only.

## Q4 — Is `SessionContext` a god-struct problem?

**Context:** Many sections (campaign, git, TDD, tools, constitutional).  
**Options:** Keep flat for simplicity; nest by domain; move some fields to on-demand kernel queries only.  
**Lean:** Flat is OK while prompt assembler expects direct fields; nest only if compilation/navigation pain rises.

## Q5 — Where do shared test mocks live?

**Context:** Many packages reimplement `mockKernel` for `types.Kernel`.  
**Options:** `internal/types` testexport; `internal/testing`; leave duplicated.  
**Constraint:** Mocks must not force `types` to import heavy packages.

## Q6 — Expand `VirtualStore` interface how far?

**Context:** Comment says expand as needed for shards.  
**Risk:** Interface bloat or leak of core-only methods.  
**Process:** Require ≥2 packages needing the method before adding.

## Q7 — JSON containers as first-class Mangle types?

**Context:** Maps/slices become JSON strings.  
**Future:** If Mangle gains native compound types, revisit encoding without breaking stored facts.
