# 09 — Safety and Invariants: features

> Last verified against codebase: **2026-07-13**

## 1. Constitutional safety relationship

| Concern | Owner | Features role |
|---------|-------|---------------|
| `permitted(...)` | Mangle policy / core | **None** — flags do not grant permissions |
| Default deny | Policy corpus | Unaffected |
| Dreamer / dry-run | Core | Unaffected |
| Enabling DiffEval | Features + kernel_eval | Changes **how** facts are evaluated, not whether actions are permitted |
| Enabling system shards | Features + session_boot | Starts background agents that still go through policy |

**Invariant S1:** Never use a feature flag as a security authorization bit.

## 2. Concurrency invariants

| ID | Invariant | Mechanism |
|----|-----------|-----------|
| C1 | Concurrent Is* reads are race-free | `atomic.Pointer` load |
| C2 | SetActive cannot leave half-written struct | Store whole pointer to copy |
| C3 | Callers cannot mutate live config via original pointer after SetActive | Struct copy in SetActive |
| C4 | Callers must not mutate `Active()` result | Documented contract (not enforced by type system) |

## 3. Configuration invariants

| ID | Invariant | Mechanism |
|----|-----------|-----------|
| K1 | Absent JSON key ≠ false | `*bool` fields |
| K2 | Invalid env does not flip flag | resolveBool fall-through |
| K3 | Missing config file does not wipe prior active | LoadUserConfig early return |
| K4 | Numeric invalid env does not set 0 via parse success | parse rejects ≤0 and non-digits |
| K5 | PerShardFacts remains off in FullyEnabled seed | Explicit `false` in factory |

## 4. Layering invariants

| ID | Invariant |
|----|-----------|
| L1 | No `codenerd/internal/*` imports in package features |
| L2 | No logging import; Boot log is caller responsibility |
| L3 | External tests may import config; production package may not |

## 5. Operational safety

| Risk | Mitigation |
|------|------------|
| DiffEval incomplete edge cases | Default OFF in compile defaults; ops can force `CODENERD_DIFF_EVAL=0` |
| PerShardFacts without coordinator maturity | FullyEnabled keeps false; env opt-in for experiments |
| Flight recorder memory | Bounded 64 MiB / 30 s window at start site (main), not unbounded |
| Provenance memory growth | Default OFF until explicitly requested |
| Disabling all system shards | Master switch; intentional for light sessions / tests |

## 6. What is *not* invariant

- Active config is **not** frozen after boot; SetActive can be called again.  
- Env can change mid-process (tests do this); production rarely does.  
- Summary string format is convenience, not a stable machine API unless versioned later.

## 7. Mangle Decl surface

**None.** No local `.mg` files. No feature-related Decl required in this package.

## 8. Threat model (narrow)

| Threat | Realistic? | Notes |
|--------|------------|-------|
| Local env injection flips eval path | Yes, by design for CI | Operators control env |
| Malicious config.json enables experimental paths | Yes | Workspace trust model — same as other config |
| TOCTOU on Active pointer | Low | Atomic load; snapshot not deep-cloned nested (no nested pointers except *bool) |
| *bool aliasing across fields | Low | SetActive copies top-level struct; pointer fields still share pointees if caller reuses bool vars across fields carefully — typical stack locals are fine |
