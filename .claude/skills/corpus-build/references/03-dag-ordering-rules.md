# DAG Ordering Rules

Build plans use a 5-level dependency DAG. Work units within a level run in parallel (up to 3 concurrent worktrees). Levels execute sequentially.

## Level Definitions

### Level 0: Core Types and Interfaces (no dependencies)

Build foundation types, interfaces, and constants. These have no dependencies on other work units.

**Work unit types**: Type 1 (new package/file) for interface definitions and data types
**Example**: CausalEdge type, CausalGraph interface, TemporalDecay scoring types
**Constraint**: Level 0 units MUST NOT import from other Level 0 units in the same build

### Level 1: Implementations (depends on Level 0)

Build functions implementing Level 0 interfaces. May import types defined in Level 0.

**Work unit types**: Type 1 (new file), Type 2 (complete partial)
**Example**: BTC-Alpha operator implementation, temporal decay scoring, causal chain validator
**Constraint**: Level 1 units within the same level should not have cross-dependencies. If unit A needs types from unit B, and both are Level 1, the corpus-judge must move one to Level 0.

### Level 2: Tests (depends on Level 0 + Level 1)

Create unit and integration tests for code built in Levels 0-1.

**Work unit types**: Type 3 (unit tests), Type 4 (integration tests), Type 5 (cross-system tests)
**Example**: Tests for CausalEdge, BTC-Alpha operators, causal+graph integration
**Constraint**: Level 2 fix agents get Level 1 output files as context (failure cascades may originate in Level 1)

### Level 3: Wiring (depends on Level 0 + Level 1, parallel with Level 2)

Build API endpoints, protocol handlers, Mangle rules, page agent updates. These connect the subsystem to the rest of codeNERD.

**Work unit types**: Type 6 (REST API), Type 7 (page agents), Type 8 (Mangle), Type 9 (corpus docs), Type 11 (protocol handlers)
**Constraint**: Level 3 units write intent files for reserved files (see below), not the files directly.

### Level 4: Verification (depends on all prior levels)

Run wiring audit across all integration surfaces. This is a single sequential unit.

**Work unit types**: Type 10 (wiring verification)
**Constraint**: Always sequential (1 unit). Incorporates all intent files from Level 3.

## Reserved-File Pattern

### Problem

Multiple work units in the same DAG level may need to modify the same file (e.g., cmd/nerd/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main) to register routes, .nerd/config.json to add config sections). Parallel worktree modifications to the same file cause merge conflicts.

### Solution: Registration Intent Files

Builders in Levels 0-3 do NOT modify reserved files directly. Instead, they write intent files:

```
.corpus-build/intents/<WU-NNN>_intents.json
{
  "work_unit": "WU-003",
  "intents": [
    {
      "target_file": "cmd/nerd/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main)",
      "action": "register_route",
      "content": "router.GET(\"/api/v1/causal/chains\", h.ListCausalChains)"
    },
    {
      "target_file": ".nerd/config.json",
      "action": "add_config_section",
      "content": "causal:\n  enabled: true\n  max_chain_depth: 10"
    }
  ]
}
```

The Level 4 wiring worker (corpus-wiring-auditor) reads ALL intent files and incorporates them into the target files sequentially, avoiding conflicts.

### Reserved Files

These files are common write targets that use the intent pattern:

| File | Reason |
|------|--------|
| cmd/nerd/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main) | Route registration |
| internal/mcp/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main) | MCP tool registration |
| internal/mcp/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main) | A2A capability registration |
| internal/app/server/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main) | Lifecycle registration |
| .nerd/config.json | Config section addition |
| configs/development.yaml | Dev config overrides |
| configs/testing.yaml | Test config values |
| configs/production.yaml | Prod config values |
| MCP/tool schemas*/*.proto | Protobuf definitions |

## Cycle Detection

If the corpus-judge produces a plan with circular dependencies (A depends on B, B depends on A), the DAG is invalid. Resolution: extract shared interface types into a Level 0 "interface-only" work unit that both A and B can depend on. The build_dag.py script (when implemented) detects cycles using Python stdlib graphlib.TopologicalSorter.

## Example: Causal Subsystem

```
Level 0 (parallel):
  WU-01: corpus-builder -> CausalEdge type + CausalGraph interface
  WU-02: corpus-builder -> TemporalDecay scoring types

Level 1 (parallel, after L0):
  WU-03: corpus-builder -> BTC-Alpha operator implementation
  WU-04: corpus-builder -> Complete temporal decay scoring
  WU-05: corpus-builder -> Causal chain validator

Level 2 (parallel, after L1):
  WU-06: go-architect / test-forge unit -> Tests for WU-01,02,03,04,05
  WU-07: test-forge integration -> Integration: causal + graph traversal

Level 3 (parallel, after L1):
  WU-08: corpus-comms-plumber -> /api/v1/causal/* endpoints (writes intents)
  WU-09: corpus-builder -> Causal Mangle predicates (writes intents)
  WU-10: corpus-builder -> MCP tool handlers (writes intents)

Level 4 (sequential, after L2+L3):
  WU-11: corpus-wiring-auditor -> Incorporate intents, verify surfaces
```
