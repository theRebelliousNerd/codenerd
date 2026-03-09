# codeNERD

codeNERD is a high-assurance, logic-first CLI coding agent. The model is the creative center; the Mangle kernel is the executive. Logic determines reality; the model merely describes it.

## North Star

The current generation of coding agents makes a category error: it asks LLMs to do both creativity and executive control. codeNERD separates those roles. The LLM handles problem solving, synthesis, and insight; the deterministic Mangle layer handles planning, memory, orchestration, and safety.

This repo exists to make that split real in production: creative power with deterministic safety, long-horizon context without prompt drift, and parallel specialists whose behavior is grounded by logic rather than luck.

### Inversion of Control

- **LLM as creative center**: problem solving, synthesis, goal shaping, and insight.
- **Logic as executive**: planning, memory, orchestration, safety, and policy.
- **Transduction interface**: natural language and code are converted into formal atoms that the kernel can reason over.

## Repo Contract

- JIT is the standard for all new LLM-facing behavior.
- New prompt behavior becomes prompt atoms first, not ad-hoc shard prompt text.
- Internal prompt atoms live under `internal/prompt/atoms/<category>/`.
- Project-specific or user-agent prompt atoms live under `.nerd/agents/`.
- Always look for wiring gaps before deleting "unused" code. This codebase frequently has partially wired features and dormant integration points.
- Keep root-level agent guidance concise. Put subsystem detail in READMEs, skill references, or freshly introduced scoped docs if they are deliberately brought back.
- Push to GitHub regularly and use conventional commits.

## Maintenance Rule For This File

- Keep this file focused on repo-wide instructions, not subsystem encyclopedias.
- Preserve the north star, hard requirements, live command snippets, and a current file map.
- Move deep reference material into skills, READMEs, or deliberately reintroduced scoped docs rather than growing this file again.
- When editing this file, verify that every path and command still works.

## Quick Commands

### Build With sqlite-vec Support (PowerShell)

```powershell
if (Test-Path .\nerd.exe) {
    Remove-Item .\nerd.exe -ErrorAction SilentlyContinue
}
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

### Build With sqlite-vec Support (bash)

```bash
rm -f ./nerd.exe
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -o nerd.exe ./cmd/nerd
```

SQLite headers live at `sqlite_headers/sqlite3.h`.

### Test

```bash
go test ./...
```

### Stress / Long-Horizon Validation

- In chat mode, run `/campaign assault ...`.
- Artifacts persist under `.nerd/campaigns/<campaign>/assault/`.

If you see `debug_program_ERROR.mg`, the system crashed and dumped combined `.mg` sources into that file for debugging.

## Quick Reference

- **OODA loop**: Observe -> Orient -> Decide -> Act.
- **Fact flow**: user input -> perception -> `user_intent` -> kernel derives `next_action` -> VirtualStore executes -> articulation responds.
- **Constitutional safety**: every action must derive `permitted(...)`; default deny.

## Working Map

| Area | Live location | Notes |
|------|---------------|-------|
| Kernel | `internal/core/kernel.go` | Package marker; implementation is split across `kernel_*.go`. |
| Policy | `internal/core/defaults/policy/` | Modular Mangle policy corpus. |
| Schemas | `internal/core/defaults/schemas.mg` | Core schema declarations. |
| Prompt compiler | `internal/prompt/compiler.go` | JIT prompt compilation and atom selection. |
| Prompt assembly | `internal/articulation/prompt_assembler.go` | Runtime prompt assembly bridge. |
| Prompt atoms | `internal/prompt/atoms/` | Canonical atom library. |
| Session execution | `internal/session/executor.go` | Clean execution loop. |
| Shard lifecycle | `internal/core/shards/manager.go` | Shard manager and lifecycle plumbing. |
| Shard registration | `internal/shards/registration.go` | Registers domain and system shards. |
| VirtualStore | `internal/core/virtual_store.go` | Action routing and external integration. |
| Research tools | `internal/tools/research/context7.go` | Context7-backed research tooling. |

## When Working In Specific Areas

- Prompt or shard-behavior changes: read `internal/prompt/README.md` and the prompt-architect skill references first.
- Mangle logic changes: use `.claude/skills/mangle-programming/references/` or `.codex/skills/mangle-programming/` before editing `.mg` files.
- Core runtime or execution changes: read `internal/core/README.md`, `internal/session/README.md`, and `internal/shards/README.md`.
- Integration or "code exists but doesn't run" issues: audit wiring before deleting code or declaring a system unused.
- Specialized references live under `.claude/skills/` and `.codex/skills/`.

## Mangle Guardrails

- All predicates need `Decl` before use.
- Variables are uppercase; atoms use `/lowercase`.
- Negation only works when variables are already bound by positive atoms.
- Aggregation must use `|> do ... let ...` pipeline syntax.
- Do not use Mangle for fuzzy matching or large natural-language pattern banks. Use embeddings or other retrieval first, then assert structured facts into the kernel.
- For deeper syntax, failure modes, and examples, use the Mangle skill references instead of expanding this root file.

## Development Guidelines

- Run `go test ./...` before handoff.
- Keep new LLM systems JIT-first.
- Prefer adding prompt atoms and selection logic over hardcoding prose in shards.
- When in doubt, preserve the architectural north star and trim encyclopedic detail.

## Deep References

- `.claude/skills/codenerd-builder/references/`
- `.claude/skills/mangle-programming/references/`
- `.codex/skills/` mirrors these skills for Codex-oriented workflows.
