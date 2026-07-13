# tools — Vision

> Last verified: **2026-07-13**  
> Target state for `internal/tools` (product/architecture intent)

## Mission

Provide a **complete, safe, schema-described toolbox** that any JIT-selected agent can use to perceive and change the world — without the toolbox becoming an unsupervised executive.

## Target properties

1. **Every effect is a named tool** with stable Name matching ConfigAtoms / Mangle atoms (`/read_file` ↔ `read_file`).  
2. **One registration story** — subpackages export `RegisterAll`; boot hydrates once into the authoritative registry used by session.  
3. **Policy outside tools** — tools assume they were already allowed; they still enforce **local physical invariants** (workspace root, timeouts, output caps).  
4. **Intent filters are closed by default** for destructive categories; research tools gated by research intents.  
5. **Containment is universal** for any path-touching or process-spawning tool.  
6. **LLM-facing descriptions** stay short; deep guidance lives in prompt atoms (`available_tools`, tool_nudge), not giant Description strings.  
7. **Helpers vs tools** stay separate: GroundingHelper/ThinkingHelper remain libraries for systems that hold an LLM client; they are not fake registry entries.  
8. **Test impact and world graph** remain injectable (no codedom → world import cycle).  
9. **Ouroboros tools** remain a second execution backend for generated binaries, not a second definition language for builtins.  
10. **Observability** supports post-hoc “what tool ran, how long, success?” for learning (`tool_execution` facts optional).

## Target control flow

```
user_intent
  → Mangle modular_tool_allowed / relevant_tool
  → ConfigFactory materializes AllowedTools[]
  → JIT prompt exposes only those tools
  → LLM requests tool_call
  → session: allowlist ∩ permitted ∩ executive gate
  → Registry.Execute
  → result truncated → multi-turn tool results
  → optional tool_execution fact for learning
```

## Non-goals

- Becoming a general RPA platform  
- Full language servers inside codedom (world/AST owns depth)  
- Network policy engine (beyond basic timeouts/size caps)  
- Replacing constitutional policy with Go if/else trees  

## Success criteria

| Criterion | Signal |
|-----------|--------|
| Agent can code safely in workspace | write/edit cannot escape root |
| Agent can research | context7/web/browser work when intent is research |
| Default deny holds | unlisted tool never executes under safety gate on |
| Tests stay green | `go test ./internal/tools/...` |
| Catalog coherence | every registered Name has Mangle rule or explicit intentional omit |
