# Deep dive: Mangle surface owned by system wiring

System owns no production `.mg` program. Its Mangle responsibility is topology:
which RealKernel shard owns a predicate, when shards evaluate, and which adapters
translate facts at package boundaries.

## Boot-owned predicate topology

| Domain | Representative owned predicates |
|---|---|
| routing | `user_intent`, `next_action`, `routing_result`, `derived_mode` |
| world | `file_topology`, `symbol_graph`, `diagnostic`, `project_profile` |
| tools | `tool_capabilities`, `shard_lifecycle`, `shell_exec_result` |
| policy | complete authorization envelope plus `blocked`, `constitution`, `commit_barrier`, `dangerous_action` |
| campaign | campaign, phase, task, and dependency facts |
| prompts | prompt atom, selection score, and shard prompt base |
| cortex | catch-all shard with no declared ownership list |

`internal/shards/registration.go#DefaultShardPredicateManifests` is the
canonical topology authority;
`internal/system/factory.go#defaultKernelShardConfigs` converts it into live
KernelShardConfig values. Core schemas must declare every predicate before use;
system does not
waive arity, bound-negation, recursion/stratification, atom/string, or
aggregation rules.

## Adapter boundaries

- `KernelAdapter` converts core facts for prompt selection and uses a private
  cloned RealKernel per compile.
- `mcpKernelAdapter` parses string facts, preserves Mangle name constants, and
  retracts exact facts without a doubled terminator.
- `sessionKernelAdapter` forwards the typed kernel surface.
- `HolographicCodeScope` asserts world/code facts without creating a core/world
  import cycle.

## Non-goals

System will not implement fuzzy intent matching in Mangle, author policy rules,
grant itself permission, or allow model-proposed text to bypass parse/analyze/
schema gates. `debug_program_ERROR.mg` is a crash artifact, not package policy.
