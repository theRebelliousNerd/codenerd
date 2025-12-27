# Implementation Plan: JIT-Driven Universal Agent Refactor

**Status: ✅ COMPLETE (Dec 27, 2024)**

> This plan was the original detailed roadmap. We achieved the same goals with a simpler "Clean Loop" approach. See `ARCHITECTURE_CHANGES.md` for the actual implementation.

---

## Summary

| Metric | Value |
|--------|-------|
| Code Removed | ~35,000 lines |
| Code Added | ~1,600 lines |
| Net Reduction | 95% |
| Tests Passing | 49/49 |

---

## Phase 1: JIT Configuration Engine ✅

- [x] Task: Define Configuration Schema & Types
    - [x] `internal/jit/config/types.go` - AgentConfig, validation
- [x] Task: ConfigFactory Implementation
    - [x] `internal/prompt/config_factory.go` - Intent → tools/policies mapping
    - [x] ConfigAtom registry for mapping intents
    - [x] Priority-based atom merging

**Approach Change:** Instead of refactoring the existing Compiler, we created a standalone ConfigFactory.

## Phase 2: Mangle-World Bridge ✅

- [x] Task: GraphQuery Interface
    - [x] `internal/world/graph_query.go` - Consistent world model access
- [x] Task: Intent Routing Rules
    - [x] `internal/mangle/intent_routing.mg` - Declarative persona/action routing

**Approach Change:** Instead of virtual predicates, we used direct Mangle rules for persona selection.

## Phase 3: The Universal Executor ✅

- [x] Task: Session Executor
    - [x] `internal/session/executor.go` (391 lines) - Universal execution loop
    - [x] `internal/session/spawner.go` (385 lines) - JIT-driven subagent creation
    - [x] `internal/session/subagent.go` (339 lines) - Lifecycle management
- [x] Task: Constitutional Safety
    - [x] Integrated safety gates into executor
- [x] Task: Telemetry
    - [x] Structured logging for execution loop

**Approach Change:** Created `internal/session/` package instead of `internal/executor/universal.go`.

## Phase 4: Autopoiesis & Learning Integration 🔄

- [x] Task: Ouroboros preserved (unmodified)
- [x] Task: Thunderdome preserved (unmodified)
- [ ] Task: Mangle-driven learning triggers (future enhancement)

**Status:** Autopoiesis works via existing VirtualStore integration. Mangle-driven learning is a future enhancement.

## Phase 5: Initialization & Persistent Shard Handling ✅

- [x] Task: Init Refactor
    - [x] `internal/init/agents.go` - Stubbed research/tool generation
    - [x] JIT handles research via prompt atoms now
- [x] Task: Persistent Agent Support
    - [x] SubAgent lifecycle types (Ephemeral, Persistent, System)

## Phase 6: The Purge ✅

- [x] Task: Deleted Shards
    - [x] `internal/shards/coder/` - DELETED
    - [x] `internal/shards/tester/` - DELETED
    - [x] `internal/shards/reviewer/` - DELETED
    - [x] `internal/shards/researcher/` - DELETED
    - [x] `internal/shards/nemesis/` - DELETED
    - [x] `internal/shards/tool_generator/` - DELETED
- [x] Task: Tests Passing
    - [x] All 49 tests pass

---

## Future Enhancements (Not Blocking)

- [ ] Migrate `.nerd/shards/` → `.nerd/agents/`
- [ ] Hot-reload for .mg files and prompt atoms
- [ ] Mangle-driven learning triggers (`trigger_learning` predicate)
- [ ] Refactor campaign orchestrator to use Spawner
- [ ] Remove legacy `/spawn coder` commands

---

## Key Files Created

| File | Lines | Purpose |
|------|-------|---------|
| `internal/session/executor.go` | 391 | Universal execution loop |
| `internal/session/spawner.go` | 385 | JIT-driven SubAgent creation |
| `internal/session/subagent.go` | 339 | Context-isolated execution |
| `internal/prompt/config_factory.go` | 205 | Intent → tools/policies mapping |
| `internal/mangle/intent_routing.mg` | 228 | Declarative routing rules |
| `internal/jit/config/types.go` | 46 | AgentConfig types |

---

## Conclusion

The JIT-Driven refactor is complete. The original 6-phase plan was simplified into a "Clean Loop" architecture that achieved:

1. **95% code reduction** in agent layer
2. **Declarative persona selection** via Mangle
3. **Zero Go code** for new personas (just .mg rules + prompt atoms)
4. **All tests passing**

See `ARCHITECTURE_CHANGES.md` for the complete technical documentation.
