# mangle/ - Mangle/Datalog Reference Atoms

**109 files** - Comprehensive Mangle language reference for rule synthesis.

## Structure

| Directory | Files | Purpose |
|-----------|-------|---------|
| `syntax/` | Core | Syntax rules and grammar |
| `builtins/` | Core | Built-in predicates and functions |
| `builtins_complete/` | Extended | Complete builtin reference |
| `patterns/` | Core | Common Mangle patterns |
| `antipatterns/` | Core | What NOT to do |
| `errors/` | Core | Error types and fixes |
| `error_fixes/` | Extended | Detailed error resolution |
| `error_messages/` | Extended | Error message catalog |
| `failure_modes/` | Core | AI-specific failure patterns |
| `analysis/` | Advanced | Program analysis techniques |
| `engine/` | Advanced | Mangle engine internals |
| `go_integration/` | Advanced | Go embedding patterns |
| `data_model/` | Core | Type system, structs, maps |
| `codenerd/` | codeNERD | codeNERD-specific schemas |

## Top-Level Files

| File | Purpose |
|------|---------|
| `syntax_core.yaml` | Core syntax reference |
| `declarations_complete.yaml` | Decl syntax |
| `builtins_complete.yaml` | All built-in functions |
| `patterns_comprehensive.yaml` | Pattern encyclopedia |
| `antipatterns_complete.yaml` | Anti-pattern catalog |
| `negation_safety.yaml` | Safe negation patterns |
| `stratification.yaml` | Stratification rules |
| `recursion_mastery.yaml` | Recursive rule patterns |
| `transforms_complete.yaml` | Aggregation transforms |
| `type_system.yaml` | Type system reference |
| `go_integration_complete.yaml` | Go API reference |
| `repair_identity.yaml` | MangleRepairShard identity |

## Usage

Mangle atoms are critical for:
- Legislator rule synthesis
- MangleRepair validation
- Schema-aware code generation

Selected via `languages: ["/mangle"]` or specific shard types.
