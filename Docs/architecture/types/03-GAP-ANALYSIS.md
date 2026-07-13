# 03 — Gap Analysis: `internal/types`

> Last verified: **2026-07-13**

## 1. Spec / vision vs reality matrix

| Desired state | Reality | Gap severity | Notes |
|---------------|---------|--------------|-------|
| Single fact type across monorepo | `types.Fact` + aliases (`core.Fact`, `world.Fact`) | **Low** | Aliases are good; watch for private duplicates |
| Fail-loud ToAtom | Implemented + tested | **None** | Historical silent fallback removed |
| Single kernel interface | `Kernel` **and** `KernelInterface` | **Medium** | Dual APIs + `KernelFact` |
| Atomic multi-op asserts | `KernelTx` required | **Low** | Panic if unimplemented (strict) |
| Safe arg extract everywhere | Helpers exist; not universal adoption | **Medium** | Call sites still may use bare asserts |
| Capability-split LLM API | Implemented | **None** | Pattern is healthy |
| SessionContext as blackboard | Implemented; large | **Low** | Growth management needed |
| VirtualStore complete surface | Marker + subset of methods | **Low** (intentional) | Expand only with real consumers |
| Transaction tests in-package | Mostly in `core` | **Low** | Acceptable if documented |
| Typed context keys for model/priority | String constants | **Low–Med** | Session uses typed key; spawn uses strings |
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
| G-01 | Call sites bypassing `ToAtom` conventions | Code review + shared assert helpers in core |
| G-02 | Incomplete mock kernels missing `KernelTransactor` | Document panic; provide test helper mock in `testing` or `types` |

### P1 — API coherence

| ID | Gap | Mitigation |
|----|-----|------------|
| G-10 | Dual `Kernel` / `KernelInterface` | Migration plan: alias, then delete bridge |
| G-11 | `Fact` vs `KernelFact` duplication | Collapse to one type with conversion methods only if needed |
| G-12 | Stringly `CtxKeyPriority` / model keys | Migrate to unexported typed keys like session |

### P2 — adoption & hygiene

| ID | Gap | Mitigation |
|----|-----|------------|
| G-20 | Not all consumers use `Extract*` | Lint or codemod for `Args[i].(string)` patterns |
| G-21 | `SessionContext` field sprawl | Document sections; consider nested sub-structs only if needed |
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

## 5. Recommendation

**Do not expand** `types` with domain DTOs. **Do** schedule a careful `KernelInterface` consolidation behind adapters so autopoiesis and core share one mental model.
