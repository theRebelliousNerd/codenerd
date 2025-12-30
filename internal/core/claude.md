# internal/core - System Kernel & Shard Management

This package implements the core runtime components of codeNERD: the Mangle Kernel and ShardManager.

## Architecture

The core package provides:
1. **RealKernel** - The Mangle/Datalog inference engine
2. **ShardManager** - Lifecycle management for ShardAgents
3. **ShadowMode** - Simulation before execution
4. **TDD Loop** - Test-driven development automation

## File Index

| File | Description |
|------|-------------|
| `api_scheduler.go` | Cooperative API slot scheduler allowing many shards but limiting concurrent LLM calls. Shards yield slots between API calls for fair scheduling. |
| `dream_learning.go` | Extracts learnable insights from Dream State multi-agent consultations. Categorizes learnings as procedural, tool_need, risk_pattern, or preference. |
| `dream_router.go` | Routes confirmed learnings to appropriate storage tiers (LearningStore, Ouroboros, ColdStorage). Implements `DreamRouter` with configurable backends. |
| `dreamer.go` | Simulates action impact before execution for pre-flight safety. Exports `Dreamer` and `DreamCache` for speculative evaluation. |
| `hybrid_loader.go` | Parses hybrid .mg files with DATA directives (TAXONOMY, INTENT, PROMPT). Extracts EDB facts and prompt atoms while returning Mangle logic. |
| `intent_inference.go` | Infers `StructuredIntent` from free-form task strings. Maps keywords to verbs/categories for JIT selector compatibility. |
| `kernel.go` | Package marker documenting kernel modularization across kernel_*.go files. Points to kernel_types, kernel_init, kernel_facts, kernel_query, kernel_eval, kernel_validation. |
| `kernel_eval.go` | Policy evaluation and Mangle rule execution. Implements `Evaluate()` to run the inference engine. |
| `kernel_facts.go` | Fact management including `LoadFacts`, `Assert`, `Retract`. Handles deduplication and JIT fact tracking. |
| `kernel_init.go` | Constructor `NewRealKernel()` that boots the Mangle engine. Loads mangle files, predicate corpus, and EDB from hybrid files. |
| `kernel_policy.go` | Policy and schema loading from .mg files. Implements `SetPolicy()`, `SetSchemas()`, `LoadPolicyFile()`. |
| `kernel_query.go` | Query execution via `Query(predicate)` and pattern matching. Returns facts matching the requested predicate. |
| `kernel_types.go` | Core type definitions including `RealKernel` struct, `Fact`/`Kernel` aliases. Breaks import cycles with types package. |
| `kernel_utils.go` | Kernel utility functions for fact manipulation. Provides helpers for canonical form and indexing. |
| `kernel_validation.go` | Schema validation and safety checks using `SchemaValidator`. Validates facts against declared predicates. |
| `kernel_virtual.go` | Virtual predicate handling to delegate queries to external systems. Bridges kernel queries to VirtualStore. |
| `learning.go` | `LearningStore` interface for persisting shard learnings. Methods: Save, Load, LoadByPredicate, DecayConfidence. |
| `limits.go` | `LimitsEnforcer` for hard resource limits (RAM, shards, session time, facts). Triggers callbacks on violations. |
| `llm_client.go` | `LLMClient` type alias and extensions (`SchemaCapableLLMClient`, `TracingClient`). Defines JSON Schema validation capability. |
| `mangle_watcher.go` | Watches .nerd/mangle/*.mg for changes using fsnotify. Triggers validation/repair with debouncing. |
| `predicate_corpus.go` | Baked-in predicate corpus from defaults package. Validates predicate usage at runtime. |
| `rule_court.go` | Validates proposed policy rules against constitutional safety. Uses sandbox kernel to test deadlock/liveness. |
| `shadow_mode.go` | Counterfactual simulation for "what if?" scenarios. Exports `ShadowMode` with separate shadow kernel. |
| `shard_base.go` | `BaseShardAgent` providing common shard functionality. Implements GetID, GetState, SetState, Stop. |
| `shard_config.go` | Shard configuration helpers and profile management. Provides config validation and defaults. |
| `shard_manager.go` | Package marker documenting ShardManager modularization. Points to shard_manager_core, spawn, tools, facts, feedback. |
| `shard_manager_core.go` | `ShardManager` struct and core operations (factories, profiles, disabled set). Manages shard lifecycle. |
| `shard_manager_facts.go` | Fact conversion utilities for shard results. Converts shard outputs to Mangle facts. |
| `shard_manager_feedback.go` | Reviewer feedback interface for cross-shard communication. Routes reviewer findings to shards. |
| `shard_manager_spawn.go` | Shard spawning and execution logic. Implements `Spawn()` with context and session injection. |
| `shard_manager_tools.go` | Intelligent tool routing (§40) for shard-aware tool dispatch. Routes tools based on shard affinity. |
| `spawn_queue.go` | Backpressure-aware prioritized shard spawning. Queues requests when limits reached for graceful degradation. |
| `system_shard.go` | Type 1 permanent shard for system homeostasis. Runs continuously monitoring filesystem and .nerd/ integrity. |
| `tdd_loop.go` | TDD automation state machine (Red→Green→Refactor). Exports `TDDState`, `TDDAction`, `Diagnostic` types. |
| `tool_registry.go` | Bridges generated tools (Ouroboros) with kernel and shards. Exports `Tool` and `ToolRegistry` for registration. |
| `trace.go` | Tracing types for observability. Supports distributed tracing across shard execution. |
| `virtual_fact_store.go` | Virtual fact store backing the kernel. Implements predicate dispatch to handlers. |
| `virtual_store.go` | VirtualStore FFI router for external system integration. Routes `next_action` atoms to drivers (Bash, MCP, File IO). |
| `virtual_store_actions.go` | Action execution methods for file writes, shell commands. Implements constitutional checks. |
| `virtual_store_codedom.go` | Code DOM predicates for semantic code editing. Handles AST queries and mutations. |
| `virtual_store_predicates.go` | Predicate handlers for various domains. Maps predicate names to handler functions. |
| `virtual_store_python.go` | Python execution handlers for Python-specific operations. Integrates Python subprocess calls. |
| `virtual_store_types.go` | VirtualStore types including `ConstitutionalRule`, `IntegrationClient`. Defines action interfaces. |
| `virtual_store_workflows.go` | Workflow predicate handlers for multi-step operations. Implements compound action sequences. |

## Key Types

### RealKernel
```go
type RealKernel struct {
    engine *mangle.Engine
    store  *VirtualStore
}

func (k *RealKernel) LoadFacts(facts []Fact) error
func (k *RealKernel) Query(predicate string) ([]Fact, error)
```

### ShardManager
```go
type ShardManager struct {
    profiles map[string]ShardConfig
    active   map[string]ShardInterface
}

func (sm *ShardManager) Spawn(ctx context.Context, shardType, task string) (string, error)
func (sm *ShardManager) DefineProfile(name string, config ShardConfig) error
```

### ShardConfig
```go
type ShardConfig struct {
    Name          string
    KnowledgePath string
    SystemPrompt  string
    Tools         []string
}
```

### Fact
```go
type Fact struct {
    Predicate string
    Args      []interface{}
}
```

## Shard Types

| Constant | Value | Description |
|----------|-------|-------------|
| ShardTypeSystem | 1 | Always-on core shards |
| ShardTypeEphemeral | 2 | Spawn-execute-die |
| ShardTypePersistent | 3 | LLM-created, survives sessions |
| ShardTypeUser | 4 | User-defined specialists |

## Shadow Mode

Simulates actions before execution:
```go
shadowMode.Simulate(action, target) → SimulationResult
```

Returns predicted:
- Files affected
- Risk level
- Side effects
- Rollback capability

## TDD Loop

Automated test-driven development:
1. **Red**: Generate failing test
2. **Green**: Implement to pass
3. **Refactor**: Clean up implementation
4. Loop until all tests pass

## Dependencies

- `internal/mangle` - Datalog engine
- `internal/shards` - Shard implementations
- `internal/store` - Persistence

## Testing

```bash
go test ./internal/core/...
```

---

**Remember: Push to GitHub regularly!**
