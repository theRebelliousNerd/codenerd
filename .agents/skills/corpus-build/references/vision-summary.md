# codeNERD Vision Summary (corpus-build inject)

Regenerate if root AGENTS.md drifts. Ported ecosystem mutation date in skill metadata.

## North Star

codeNERD is a high-assurance, logic-first CLI coding agent.
- **LLM = creative center** (problem solving, synthesis, insight)
- **Mangle kernel = executive** (planning, memory, orchestration, safety, policy)
- **Transduction**: NL/code ↔ formal atoms the kernel can reason over

## Runtime spine

```
user input → perception → user_intent → kernel derives next_action
  → VirtualStore executes → articulation responds
```

OODA: Observe → Orient → Decide → Act.
Constitutional safety: every action must derive `permitted(...)`; default deny.
JIT is the standard for new LLM-facing behavior (prompt atoms under `internal/prompt/atoms/`).

## Key live locations

| Area | Location |
|------|----------|
| Kernel | `internal/core/` |
| Policy | `internal/core/defaults/policy/` |
| Schemas | `internal/core/defaults/schemas.mg` |
| Mangle engine | `internal/mangle/` |
| Perception | `internal/perception/` |
| Articulation | `internal/articulation/` |
| Prompt JIT | `internal/prompt/` |
| Session | `internal/session/` |
| Shards | `internal/shards/` |
| Campaign | `internal/campaign/` |
| Store | `internal/store/` |
| Tools / MCP | `internal/tools/`, `internal/mcp/` |
| CLI | `cmd/nerd/` |
| Architecture corpora | `Docs/architecture/` |

## Build / test

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
go test ./...
```

## What corpus-build must preserve

Strengthen the creative/executive split. Mangle for deduction/policy; Go for effects;
prompt atoms for LLM text; VirtualStore for external effects; audit wiring before deletes.
