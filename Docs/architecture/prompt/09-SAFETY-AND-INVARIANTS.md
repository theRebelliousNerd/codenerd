# prompt — Safety and Invariants

> Last verified: **2026-07-13**

## Constitutional role

This package does **not** evaluate `permitted(...)`. It:

1. Injects **safety atoms** (skeleton / high priority).  
2. Attaches **policy file lists** on `EffectiveAgentRuntimeConfig`.  
3. Sets `RequirePolicyEnforcement: true` in generated configs.  
4. Restricts **AllowedTools** to ConfigAtom lists.

Default deny remains a **kernel / VirtualStore** property. Prompts are advisory+contextual; tools without permission still fail closed downstream.

## Invariants

### I1 — Skeleton categories require kernel

If `AtomSelector` has no kernel, `loadSkeletonAtoms` returns CRITICAL error → `Compile` fails. Production must always wire `WithKernel`.

### I2 — Safety content in skeleton budget

`CategorySafety` and `CategoryIdentity` default to `PriorityMandatory` in budget manager. They still cannot exceed absolute total budget (oversize atom skipped + warn).

### I3 — production compilation facts are scope-owned

`VERIFIED CURRENT` — `Compile` acquires a `KernelCompilationScope`, and production
`KernelAdapter.NewCompilationScope` supplies a cloned `RealKernel`. All context,
candidate, dependency, conflict, mandatory, and vector facts live in that clone;
deferred `Close` discards them on success, error, cancellation, and panic.
`KernelRetracter` remains a compatibility cleanup interface, not proof of safe
overlapping compiles.

### I4 — Context validation

`TokenBudget > 0`, `ReservedTokens >= 0`, `ReservedTokens < TokenBudget`. Invalid contexts never reach selection.

### I5 — Structured-output isolation

For `legislator` and `mangle_repair` shard types, piggyback and reasoning-trace protocol atoms are filtered before selection (`output_mode.go`) so structured emitters are not forced into envelope protocols.

### I6 — Mandatory budget absolute cap

Mandatory atoms larger than total budget are **rejected**, not force-included. Cumulative mandatory overflow also skips later mandatories.

### I7 — Fact schema order

`PromptAtom.ToFact` must keep Priority at index 2 and TokenCount at index 3 per `schemas_prompts.mg` (documented fix for prior swap bug).

### I8 — Versioned cache identity is field-complete

`VERIFIED CURRENT` — concurrent identical `compilation-context-v2` hashes share
one compilation. Every prompt-affecting context field is encoded; set-like slices
are sorted/deduplicated without mutation, and the retry/tool surface changes the
key. `PARTIAL` — cache hits return the same result pointer, so immutability remains
a caller convention rather than a type-enforced ownership rule.

### I9 — Input caps

Budget Fit truncates pathological atom lists (`maxAtomsInput`); DB load limits 10k atoms; DetectCycles aborts deep stacks.

### I10 — Config merge safety

`Generate` fails if no ConfigAtom is found (except consult→general fallback).
`VERIFIED CURRENT` — `internal/session/executor_tools.go#Executor.isToolAllowed`
fails closed for nil or empty configs and accepts only exact listed tool names.
ConfigAtom capability lists are still not sufficient authorization: the
downstream constitutional gate must also derive `permitted/3`.

## Concurrency safety

- Compiler DB maps use RWMutex; collection snapshots pointers under RLock then releases before slow IO.  
- Cache locked separately from DB.  
- Budget/Assembler own mutexes for config mutation during Fit/Assemble.  
- WaitGroup tracks in-flight Compile for orderly shutdown patterns.

## Mangle safety coupling

Negation-safe Mangle rules for selection live outside this package but **bind** it:

- Atoms blocked by context must not appear in `selected_result`.  
- Exclusive groups / conflicts handled in Mangle (Go resolver does not fully re-check conflicts).

## Anti-patterns (forbidden)

| Anti-pattern | Why |
|--------------|-----|
| Hardcoding system prompts in shards | Bypasses atoms + selection |
| Selecting safety via vector only | Breaks P2 |
| Supplying a production kernel adapter without a private compilation scope | Cross-turn and concurrent selection contamination |
| Allowing all tools when ConfigAtom missing | Silent privilege expansion |
| Swapping prompt_atom arg order | Silent wrong priority |

## Logging of safety-relevant events

- Mandatory atom skip (oversize/saturated) → `CategoryContext` Warn  
- Skeleton CRITICAL → error return  
- Flesh degrade → Warn continue  
- Config generate fail → JIT Warn (compile may still return prompt without config)
