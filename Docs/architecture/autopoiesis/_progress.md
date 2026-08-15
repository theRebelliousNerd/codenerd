# Progress — Autopoiesis architecture corpus

## 2026-07-13 — Full rebuild (subagent contract)

Rebuilt `Docs/architecture/autopoiesis/` to the **cli/** quality bar per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`.

### Research performed

- Listed `internal/autopoiesis/` and `prompt_evolution/`  
- Read package README, modular split note, types, orchestrator, Ouroboros, checker, delegation, analysis, tools, feedback, kernel, runtime registry, thunderdome/panic, quality/patterns, compiler, Yaegi, profiles/traces, complexity/persistence  
- Grepped exports, reverse imports, factory/chat/process wiring  
- Skimmed prior thin corpus (replaced)

### Documents written (required set)

- README.md  
- IMPLEMENTED_SPEC.md (flagship)  
- 00-ALIGNMENT-VISION-REVIEW.md  
- 01-VISION.md  
- 02-CURRENT-STATE.md  
- 03-GAP-ANALYSIS.md  
- 04-ARCHITECTURAL-PRINCIPLES.md  
- 05-INTERNAL-ARCHITECTURE.md  
- 06-PUBLIC-API-AND-TYPES.md  
- 07-DEPENDENCY-MAP.md  
- 08-WIRING-AND-INTEGRATION.md  
- 09-SAFETY-AND-INVARIANTS.md  
- 10-TESTING-ALIGNMENT.md  
- 11-OBSERVABILITY.md  
- 12-FAILURE-MODES.md  
- TODO.md  
- OPEN-QUESTIONS.md  
- _progress.md (this file)

### Constraints honored

- Docs only under `Docs/architecture/autopoiesis/`  
- No Go/Mangle/test edits  
- No pre-impl “zero code” claims  
- Real paths and honest dual-path / partial wires  

### Note on legacy filenames

Older corpus used names like `01-DOMAIN-MODEL.md`, `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-TESTING-STRATEGY.md`, `08-FAILURE-MODES.md`, `09-CONSTITUTIONAL-SAFETY.md`, `09-MANGLE-SURFACE.md`. The **canonical map is README.md**; prefer the numbered set above if both exist.

## 2026-08-15 — Backlog implementation pass

Implemented the TODO backlog (see TODO.md for per-item detail and the enforcing test for each).

### Decisions made and recorded at the point of decision

| Question | Decision | Where the rationale lives |
|----------|----------|---------------------------|
| Light generation paths (Q1) | Deleted as production routes | `autopoiesis_tools.go` `GenerateTool` |
| Sandbox (Q2) | Compiled binary is the product default; Yaegi is a configured alternate | `ouroboros.go` `OuroborosConfig.ExecutionMode` |
| `AllowExec` default (Q6) | Deny; opt in per workspace via `Config.AllowToolExec` | `ouroboros.go` `DefaultOuroborosConfig`, `autopoiesis_types.go` `Config` |
| Agent scheduling owner (Q4) | Shards. Autopoiesis authors and emits `prompts.yaml` | `autopoiesis_agents.go` `writeAgentSpec` |
| SPL promotion authority (Q5) | Human-in-the-loop by default | `prompt_evolution/evolver.go` `AutoPromote` |

### Audits encoded as tests, not prose

- `tool_creation_routing_test.go` — AST inventory of every `ToolGenerator` call and every Ouroboros
  consumer, with exemption lists that go stale loudly.
- `kernel_listener_wiring_test.go` — chat files that wire an Orchestrator must start the delegation
  listener, at the documented cadence.
- `kernel_parity_test.go` — registry vs `tool_registered` parity, run automatically after boot sync.
- `checker_failclosed_test.go` — golden sample per `ViolationType`; empty/failed policy denies.
- `build_env_threading_test.go` — the operator's build environment reaches both compile sites.

### Defects found while implementing

Three production defects surfaced and were fixed; they are listed at the bottom of TODO.md (tool output
JSON round-trip, hardcoded windows/amd64 compile target, nil `*config.UserConfig` at both compile sites).
The second and third were only visible because the new multi-stage e2e test actually compiles and executes
a generated tool on the host.

### Not done

- P3 "Reduce dual templates vs JIT prompt residual prose" — still open, needs the JIT corpus to cover every
  Ouroboros stage before the legacy prompt strings can go.
