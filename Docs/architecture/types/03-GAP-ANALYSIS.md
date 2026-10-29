# 03 — Gap Analysis: `internal/types`

> Last verified: **2026-08-15**

## 1. Spec / vision vs reality matrix

| Desired state | Reality | Gap severity | Notes |
|---------------|---------|--------------|-------|
| Single fact type across monorepo | `types.Fact` + aliases (`core.Fact`, `world.Fact`) | **Low** | Aliases are good; watch for private duplicates |
| Fail-loud ToAtom | Implemented + tested | **None** | Historical silent fallback removed |
| Single kernel interface | `Kernel` **and** `KernelInterface` | **Low** (was Medium) | `KernelFact` collapsed to `= Fact`; deprecation path written on `KernelInterface`, step 1/3 done |
| Atomic multi-op asserts | `KernelTx` required | **Low** | Panic if unimplemented (strict) |
| Safe arg extract everywhere | Helpers exist; adoption now ratcheted | **Low** (was Medium) | `fact_conventions_guard_test.go` fails on new Decl-contradicting args, `%v` fact args, and `MangleAtom` asserts on query results |
| Capability-split LLM API | Implemented | **None** | Pattern is healthy |
| SessionContext as blackboard | Implemented; large | **Low** | Growth management needed |
| VirtualStore complete surface | Marker + subset of methods | **Low** (intentional) | Expand only with real consumers |
| Transaction tests in-package | `typestest` + examples cover buffer/commit/rollback | **None** | Plus a repo-wide `KernelTransactor` conformance ratchet |
| Typed context keys for model/priority | Typed keys in `ctxkeys.go`, dual-writing the legacy strings | **None** | Legacy constants deprecated; readers migrate independently |
| Observability hooks | Minimal | **None** | Appropriate for types package |

## 2. Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|---------------|
| No `.mg` files | Types package must not own policy |
| No LLM HTTP code | Correct separation |
| VirtualStore missing many FS methods | Comment says expand as needed; cycles |
| Interfaces without implementations in-package | By design |
| Low observability score | Contracts package |

## 3. Prioritized gaps

### P0 — correctness / safety

| ID | Gap | Mitigation |
|----|-----|------------|
| G-01 | ~~Call sites bypassing `ToAtom` conventions~~ | **Closed as ratchet.** `fact_conventions_guard_test.go` enforces it; 17 existing findings baselined with reasons and tabled in `TODO.md` for their owning packages |
| G-02 | ~~Incomplete mock kernels missing `KernelTransactor`~~ | **Closed as ratchet.** `kernel_transactor_guard_test.go` + `typestest.MockKernel`; 3 production adapters and 14 test doubles baselined |

### P1 — API coherence

| ID | Gap | Mitigation |
|----|-----|------------|
| G-10 | Dual `Kernel` / `KernelInterface` | **In progress.** 3-step plan on `KernelInterface`; step 2 (autopoiesis → `Kernel`) belongs to that package |
| G-11 | ~~`Fact` vs `KernelFact` duplication~~ | **Closed.** `type KernelFact = Fact`; `Fact.ToFact` kept as a deprecated identity shim |
| G-12 | ~~Stringly `CtxKeyPriority` / model keys~~ | **Closed.** `ctxkeys.go` typed keys with dual-write migration |

### P2 — adoption & hygiene

| ID | Gap | Mitigation |
|----|-----|------------|
| G-20 | Not all consumers use `Extract*` | Partly closed: the dangerous subset (`MangleAtom` asserts on query results) is ratcheted. A bare `Args[i].(string)` on a `/string` slot is correct and is deliberately NOT flagged |
| G-21 | ~~`SessionContext` field sprawl~~ | **Closed as a decision.** Stays flat; rationale on the struct (Q4) |
| G-22 | No in-package compile-time iface compliance tests for optional LLM interfaces | Optional: mock stubs with `var _` |

### P3 — documentation / DX

| ID | Gap | Mitigation |
|----|-----|------------|
| G-30 | Package-level `agents.md` missing | Optional; this corpus covers it |
| G-31 | Heuristic name-constant rules not documented outside code | This corpus § ToAtom |

## 4. Decision log (implicit in code)

| Decision | Evidence |
|----------|----------|
| Remove silent ToAtom fallback | Comments + renamed tests in `types_comprehensive_test.go` |
| Remove non-atomic KernelTx fallback | `transaction.go` panic message |
| Move GraphQuery into types | Comment in `interfaces.go` |
| JSON-encode containers for assert | `ToAtom` switch for maps/slices |
| `Fact.String` renders containers as quoted JSON too | Its output is loaded back as Mangle (`northstar.RenderVisionMangle`); a bare `map[a:b]` is a parse error |
| `Fact.String` keeps `%f` for floats | mangle-go renders `Float64(2.0)` as `2`, which re-parses as `int64` |

## 5. Recommendation

**Do not expand** `types` with domain DTOs. **Do** schedule a careful `KernelInterface` consolidation behind adapters so autopoiesis and core share one mental model.
