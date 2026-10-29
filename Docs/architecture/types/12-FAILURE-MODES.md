# 12 — Failure Modes: `internal/types`

> Last verified: **2026-07-13**

## FM-01 — EDB poison via bad Fact args

| | |
|--|--|
| **Symptom** | Mangle eval errors: “value 0x… is not a number”; inconsistent rule results |
| **Cause** | Historical silent `%v` coercion of pointers/interfaces; or callers bypassing ToAtom |
| **Mitigation** | Current `ToAtom` errors; use typed args; never pass raw structs with unexported fields |
| **Detection** | Assert errors; comprehensive tests; debug_program dump |

## FM-02 — Path treated as name constant (or reverse)

| | |
|--|--|
| **Symptom** | Wrong constant type in rules; string compares fail; hierarchical atom surprises |
| **Cause** | Heuristic false positive/negative in `isValidMangleNameConstant` |
| **Mitigation** | Use `MangleAtom` for intentional atoms; quote paths as plain strings without relying on heuristic |
| **Detection** | Unit tests; inspect `Fact.String()` |

## FM-03 — NewKernelTx panic

| | |
|--|--|
| **Symptom** | Process crash: `Kernel requires KernelTransactor for atomic transactions` |
| **Cause** | Mock or alternate kernel without `Transaction()` |
| **Mitigation** | Implement `KernelTransactor` on all production/test kernels used with multi-op paths |
| **Detection** | Stack trace; warn log on CategoryKernel just before panic |

## FM-04 — Dual API mismatch (Kernel vs KernelInterface)

| | |
|--|--|
| **Symptom** | Adapter bugs; facts asserted on one bridge not visible on another; incomplete retract semantics |
| **Cause** | Two parallel APIs with different method names and `KernelFact` conversion |
| **Mitigation** | Prefer one path per subsystem; consolidate long-term |
| **Detection** | Integration tests; e2e kernel/session tests |

## FM-05 — SessionContext staleness / races

| | |
|--|--|
| **Symptom** | Shard sees old files/intent; dream flag wrong; tool list outdated |
| **Cause** | Shared mutable `*SessionContext` without snapshotting |
| **Mitigation** | Treat as immutable snapshot per turn; rebuild at session pipeline boundaries |
| **Detection** | `session_context_isolation` e2e; chat process tests |

## FM-06 — Extract type mismatch silent zeros

| | |
|--|--|
| **Symptom** | Logic treats missing numeric arg as `0`; bool false when atom unexpected |
| **Cause** | `ExtractInt64` returns `(0, false)` — callers ignore `ok` |
| **Mitigation** | Always check `ok`; prefer `Arg*` with bounds checks |
| **Detection** | Code review; unit tests on call sites |

## FM-07 — JSON container lossy encode

| | |
|--|--|
| **Symptom** | Nested structure becomes string; downstream expects typed args |
| **Cause** | Intentional JSON encode of maps/slices for opaque payloads |
| **Mitigation** | Document consumers must `json.Unmarshal` when reading; or flatten into multiple facts |
| **Detection** | Contract tests between producer and consumer |

## FM-08 — Optional LLM interface missing

| | |
|--|--|
| **Symptom** | Features silently disabled (no grounding sources, no multi-turn tool results, no piggyback) |
| **Cause** | Type assertion fails; code takes fallback path |
| **Mitigation** | Explicit checks + logging at consumer; feature flags |
| **Detection** | Provider integration tests (`var _ types.ToolResultsProvider`) |

## FM-09 — Thought signature dropped

| | |
|--|--|
| **Symptom** | Gemini multi-turn tool loops lose reasoning continuity |
| **Cause** | Not reading `ThoughtSignatureProvider` / response field across turns |
| **Mitigation** | Follow interface docs; wire through tool loop |
| **Detection** | Multi-turn FC integration tests |

## FM-10 — VirtualStore interface incomplete for new need

| | |
|--|--|
| **Symptom** | Compile errors when adding methods; or unsafe type assert to concrete `*core.VirtualStore` |
| **Cause** | Marker interface expanded only as needed |
| **Mitigation** | Add method to `types.VirtualStore` carefully; avoid concrete leaks |
| **Detection** | Build all consumers |

## Summary table

| ID | Severity | Package-local fix? |
|----|----------|--------------------|
| FM-01 | Critical | Yes (already) + call-site hygiene |
| FM-02 | Medium | Heuristic / MangleAtom discipline |
| FM-03 | High (crash) | Implementer completeness |
| FM-04 | Medium | API consolidation |
| FM-05 | Medium | Consumers |
| FM-06 | Medium | Consumers |
| FM-07 | Low–Med | Contract docs |
| FM-08 | Medium | Consumers |
| FM-09 | Medium | Consumers |
| FM-10 | Low | Careful interface growth |
