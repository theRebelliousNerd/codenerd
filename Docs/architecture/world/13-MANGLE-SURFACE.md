# world — Mangle Surface

> Last verified: **2026-07-13**  
> Predicates world emits or lists, and where Decl lives.

## Schema home

Primary Decl source: **`internal/core/defaults/schemas_world.mg`**

World package does **not** ship production `.mg` rules. It emits EDB.

## Replace set (`WorldPredicates`)

From `internal/world/world_predicates.go`:

```
file_topology
directory
symbol_graph
dependency_link
code_defines
code_calls
assigns
guards_return
guards_block
guard_dominates
safe_access
uses
call_arg
error_checked_return
error_checked_block
function_scope
symbol_defined
symbol_referenced
code_diagnostic
symbol_completion
```

Used by `ApplyIncrementalResult` when `Full=true` via `kernel.RemoveFactsByPredicateSet`.

## Topology & existence

| Predicate | Arity (conceptual) | Emitter | Decl |
|-----------|--------------------|---------|------|
| `file_topology` | Path, Hash, Lang, Mtime, IsTest | Scanner | schemas_world |
| `directory` | Path, Name | Scanner | schemas_world |
| `file_exists` | Path | **Derived** rule | schemas_world |
| `project_language` | Lang atom | Incremental full | **not in WorldPredicates** |
| `entry_point` | Path | Incremental full heuristics | **not in WorldPredicates** |

## Fast AST

| Predicate | Emitter |
|-----------|---------|
| `symbol_graph` | TreeSitterParser, mangle fastparse |

## Deep / holographic graph

| Predicate | Emitter |
|-----------|---------|
| `code_defines` | Cartographer (Go) |
| `code_calls` | Cartographer (Go) |
| `assigns`, `guards_*`, `uses`, `call_arg`, `safe_access`, `function_scope`, `error_checked_*`, `guard_dominates` | DataFlow* |

## CodeDOM / scope (often session-local)

| Predicate | Emitter |
|-----------|---------|
| `code_element` | CodeElement.ToFacts |
| `element_signature` | CodeElement.ToFacts |
| `element_visibility` | CodeElement.ToFacts |
| `element_parent` | CodeElement.ToFacts |
| `code_interactable` | CodeElement.ToFacts |
| `active_file` | FileScope.emitScopeFacts |
| `file_in_scope` | FileScope.emitScopeFacts |
| Language Stratum-0 (e.g. go_struct, py_decorator) | EmitLanguageFacts |

**Not** in `WorldPredicates` — scope refresh / explicit retract responsibility.

## LSP

| Predicate | Emitter |
|-----------|---------|
| `symbol_defined` | lsp.Manager.projectDefinitions |
| `symbol_referenced` | projectReferences |
| `code_diagnostic` | projectDiagnostics |
| `symbol_completion` | Decl’d; completion projection optional/future |

## Git

| Predicate | Emitter |
|-----------|---------|
| `git_history` (+ churn maps as implemented) | ScanGitHistory |

Outside replace set.

## Policy consumption examples (core corpus)

- `file_exists` from topology
- Impact / context selection using `code_defines` / `code_calls`
- Test-related rules matching topology `IsTestFile`
- Spreading activation sketches using LSP symbol refs (documented in `lsp/README.md`)

Exact rule IDs live in core Mangle modules — world only supplies EDB.

## Decl hygiene checklist for new predicates

1. Add `Decl name(...) bound [...].` in schemas_world (or correct module).  
2. Emit with matching arity and atom slash conventions (`/go`, `/true`).  
3. Decide: global world replace vs scope-ephemeral.  
4. If global: append `WorldPredicates`.  
5. Add unit test asserting fact shape.  
6. Document in this file.

## Known Decl/emitter mismatches to watch

| Issue | Detail |
|-------|--------|
| `dependency_link` | Decl + list, sparse emission |
| `symbol_completion` | Decl + list, limited projection |
| Path absolute/relative | Same predicate, different identity |
| `symbol_graph` type args | May be strings vs `/name` atoms depending on path — verify against bound |

## Apply semantics reminder

```
Full:    Remove(WorldPredicateSet) → Load(NewFacts)
Delta:   RetractExact(RetractFacts) → Retract("directory") → Load(NewFacts)
```

Directory facts always refreshed on delta apply.
