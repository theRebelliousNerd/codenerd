# Context Mangle Surface (pointer)

> Last verified against codebase: 2026-07-13  
> Package-owned `.mg` files: **none** (crash dump `internal/context/debug_program_ERROR.mg` is not design source).

Mangle surface for context compilation lives in core defaults:

| Path | Role |
|------|------|
| `internal/core/defaults/schemas_context.mg` | Decl: `context_relevant`, `should_include_context`, `context_reachable`, `context_file_priority`, `turn_age_category`, `should_mask_observation`, `should_preserve_reasoning` |
| `internal/core/defaults/policy/context_compilation.mg` | C1 relevance, C4 hop reachability, C3 observation masking rules |

Consumed by `internal/context/compressor.go` (`BuildContext` queries `should_include_context`) and `compressor_metrics.go` (`assertTurnAgeCategories`).

Deep narrative: [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) §§4.7, 5.4, 9 and [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md).
