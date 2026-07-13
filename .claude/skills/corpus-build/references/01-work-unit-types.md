# Work-unit types and agent routing

The judge emits only the work-unit types needed by the selected corpus. Every unit names exact inputs, outputs, dependencies, write paths, acceptance commands, and an owning agent. Codex subagents share one checkout, so isolation comes from non-overlapping write sets and serial integration—not assumed worktrees.

| Type | Purpose | Default owner | Required evidence |
|---:|---|---|---|
| 1 | Interfaces, schemas, and pinned contracts | `corpus-foundation-worker` | compiling type/Decl contract plus consumers |
| 2 | Package-local Go implementation | `corpus-builder` | targeted tests and build |
| 3 | Mangle schema, rule, or permission work | `corpus-builder` with `mangle-programming` | declaration, safety, stratification, policy test |
| 4 | Prompt atom or JIT selection behavior | `corpus-builder` with `prompt-architect` | atom/manifest entry and compiler/selection test |
| 5 | Unit, fuzz, benchmark, or race regression | relevant test-forge skill | failing-before/passing-after or explicit new contract |
| 6 | Cross-component integration | `corpus-integration-worker` | executable interaction test |
| 7 | Runtime registration and contested wiring | `corpus-wiring-worker` | route/registry evidence and reachability test |
| 8 | CLI, MCP, tool, or articulation surface | `corpus-surface-worker` / `corpus-comms-plumber` | user-facing invocation reaches implementation |
| 9 | Architecture reconciliation and governance | `corpus-governance-reconciler` then `corpus-doc-auditor` | status tied to test/commit evidence |

## Unit contract

Each unit is a JSON object with at least:

```json
{
  "id": "WU-007",
  "type": 7,
  "requirement_ids": ["REQ-04"],
  "depends_on": ["WU-002"],
  "owner": "corpus_wiring_worker",
  "read_paths": ["internal/shards/registration.go"],
  "write_paths": [".corpus-build/intents/WU-007.json"],
  "acceptance": ["go test -count=1 ./internal/shards/..."],
  "risk": "shared registration hub"
}
```

No unit may use a directory-only write claim for contested files. If two units need the same file, they emit registration intents and the serial wiring worker owns the final edit.

## Routing rules

- Use `corpus-reader` and `corpus-judge` for analysis only; they do not edit product code.
- Workers do not delegate recursively.
- `corpus-critic`, defense, wiring, and doc auditors are read-only until an owning repair/reconciliation lane is explicitly dispatched.
- Mangle work always loads the Mangle skill and follows declaration, binding, atom, negation, and aggregation guardrails.
- LLM-facing behavior becomes prompt atoms and selection logic; it is never buried in an ad-hoc shard prompt.
- Use the three test-forge pattern skills as references; the owning implementation worker remains accountable for a green unit.
- Token/cost estimates are prohibited. Record only hook-provided usage and measured command duration.
