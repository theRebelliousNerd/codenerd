# codeNERD Vision Summary (corpus-build inject)

Regenerate if root `AGENTS.md` / north-star material drifts. Last ported: 2026-07-13.

## North Star

codeNERD is a high-assurance, logic-first CLI coding agent. The model is the creative
center; the Mangle kernel is the executive. Logic determines reality; the model describes it.

### Inversion of Control

- **LLM as creative center**: problem solving, synthesis, goal shaping, insight
- **Logic as executive**: planning, memory, orchestration, safety, policy
- **Transduction**: NL and code → formal atoms the kernel can reason over

## Runtime Spine

```
user input → perception → user_intent → kernel derives next_action
  → VirtualStore executes → articulation responds
```

OODA: Observe → Orient → Decide → Act.

Constitutional safety: every action must derive `permitted(...)`; default deny.

## Repo Contracts

- JIT is standard for new LLM-facing behavior
- Prompt atoms under `internal/prompt/atoms/<category>/` (or `.nerd/agents/`)
- Audit wiring before deleting "unused" code
- Conventional commits; push regularly when authorized

## Key Live Locations

| Area | Location |
|------|----------|
| Kernel | internal/core/ |
| Policy | internal/core/defaults/policy/ |
| Schemas | internal/core/defaults/schemas.mg |
| Prompt compiler | internal/prompt/compiler.go |
| Prompt assembly | internal/articulation/prompt_assembler.go |
| Session | internal/session/executor.go |
| Shards | internal/core/shards/, internal/shards/ |
| VirtualStore | internal/core/virtual_store.go |

## Mangle Guardrails

- `Decl` before use
- Variables Uppercase; atoms `/lowercase`
- Negation only after positive binding
- Aggregation `|> do ... let ...`
- No fuzzy NL matching in Mangle — embeddings first, then structured facts

## Build / Test

PowerShell (sqlite-vec):

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
go test ./...
```

## What corpus-build Must Preserve

When realizing a feature, implementations must strengthen — never weaken — the
creative/executive split. Prefer Mangle for deduction and policy; Go for effectful
orchestration; prompt atoms for LLM-facing text; VirtualStore for external effects.
