# codeNERD

codeNERD is a high-assurance, logic-first CLI coding agent. The model is the creative center; the Mangle kernel is the executive. Logic determines reality; the model merely describes it.

## North Star

The current generation of coding agents makes a category error: it asks LLMs to do both creativity and executive control. codeNERD separates those roles. The LLM handles problem solving, synthesis, and insight; the deterministic Mangle layer handles planning, memory, orchestration, and safety.

This repo exists to make that split real in production: creative power with deterministic safety, long-horizon context without prompt drift, and parallel specialists whose behavior is grounded by logic rather than luck.

### Inversion of Control

- **LLM as creative center**: problem solving, synthesis, goal shaping, and insight.
- **Logic as executive**: planning, memory, orchestration, safety, and policy.
- **Transduction interface**: natural language and code are converted into formal atoms that the kernel can reason over.

## The Vision (Steve, 2026-09-18) — read before any architectural change

- This is an experimental project and a different paradigm. Mangle — a deductive database
  programming language, not Datalog — plus JIT prompt compilation is meant to be the
  replacement for subagents and skills: the harness decides, not the model's discretion.
- The harness forces. Test coverage is not optional: the agent cannot stop until the tests
  that actually harden the system exist. Domain knowledge is not at the LLM's discretion: it
  is pushed into the context window when the harness decides it is needed — by user request,
  task type, and the history of what has been built. The agent keeps working not until the
  task is done, but until the behavior the north star of that system envisions holds.
- It knows the codebase better than any other coding agent because it never has to grep
  around: the kernel has the math to hand the LLM what it needs — a blend of domain knowledge
  from SQLite, vector search, CodeDOM and history — injected at the right time, for the right
  reason, to the right agent, so the job finishes in as few turns as possible without wasting
  tool calls or thinking tokens double-checking.
- Quality and capability first. Tokens are the constraint, not the goal. Lost-in-the-middle
  is eliminated by context compression, pruning and ordering — where a fact sits in the
  window is a decision.
- Tools exist for exactly three things: condense the search space, reduce the turns to
  complete the task, and offload cognition to deterministic code (a change with a blast
  radius is carried out by the tool, not by the LLM hand-editing). A tool that does none of
  these is cruft.
- "Clean fixpoint," not "clean loop." Go is fine and may be as large as the work needs — it
  is the FFI, the drivers, the tools. The executive decisions — what a turn is, whether it is
  done, what is delegated and with what task, what enters the window, what the verdict is —
  are the fixpoint of the kernel over the facts. The drift to hunt is decisions computed in
  Go instead of derived.
- Complexity is not a defect. This may be a highly complex system; what is scored is whether
  a decision is derived and whether an obligation is forced.
- It hunts its own bugs and intelligently extends its own tooling, in Mangle and in Go.
- The original bet lives in `.codex/skills/codenerd-builder/references/` — an input to be
  judged, not a request to restore. `Docs/architecture/` (July 2026) is orientation, not the
  original, and not authoritative.

## Use codeNERD. Do Not Hand-Write This Repo.

**Default: every change to `internal/`, `cmd/`, `pkg/` goes through codeNERD.**

```powershell
.\nerd.exe fix "<symptom, file:line, root cause, what to change, how to verify>"
```

Then review the diff, build, test, and commit. Your job is to *aim* codeNERD and
*verify* it, not to write the Go yourself. Hand-editing is the failure mode this
repo exists to eliminate: when you edit by hand, the system under test never
runs, no defect in it is discovered, and the session quietly becomes "an
assistant writes Go" — which teaches us nothing about codeNERD.

This is the working agreement, not a mechanical block. The `settings.json`
deny rules and the `block-direct-codebase-edits.py` hook that used to enforce
it were removed by the architect on 2026-09-03 ("that was for your dumber
predecessor"). The rule stands on judgment now: aim codeNERD first, and when
you hand-edit, say so in the commit and say why codeNERD could not do it.

**What a good brief looks like.** Vague briefs fail and burn tokens; precise
ones land. Name the file and line, the exact symptom, the root cause, the change
you want, what must NOT change, and the verification command. Size it to one
turn — multi-file tasks hit the tool-iteration ceiling, so split them.

**When codeNERD genuinely cannot do it**, that is itself the finding. File the
defect, fix the *blocker* so codeNERD can proceed, and say so explicitly. Fixing
the blocker is dogfooding; doing its job for it is not.

**Legitimate exceptions**, which still deserve a sentence of justification:
safety-gate and permission logic (the model should not widen the rule that
constrains it), and a broken build that prevents codeNERD from running at all.

## Repo Contract

- JIT is the standard for all new LLM-facing behavior.
- New prompt behavior becomes prompt atoms first, not ad-hoc shard prompt text.
- Internal prompt atoms live under `internal/prompt/atoms/<category>/`.
- Project-specific or user-agent prompt atoms live under `.nerd/agents/`.
- Always look for wiring gaps before deleting "unused" code. This codebase frequently has partially wired features and dormant integration points.
- Keep root-level agent guidance concise. Put subsystem detail in scoped `agents.md` files or skill references.
- Push to GitHub regularly and use conventional commits.

## Maintenance Rule For This File

- Keep this file focused on repo-wide instructions, not subsystem encyclopedias.
- Preserve the north star, hard requirements, live command snippets, and a current file map.
- Move deep reference material into scoped docs such as `internal/mangle/agents.md`, `internal/prompt/agents.md`, and `internal/core/agents.md`.
- When editing this file, verify that every path and command still works.

## Quick Commands

### Build With sqlite-vec Support (PowerShell)

```powershell
if (Test-Path .\nerd.exe) {
    Remove-Item .\nerd.exe -ErrorAction SilentlyContinue
}
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -tags sqlite_vec -o nerd.exe ./cmd/nerd
```

**`-tags sqlite_vec` is required, not optional.** `internal/store/vec_support_enabled.go`
is behind `//go:build sqlite_vec && cgo`; without the tag the binary takes
`vec_support_disabled.go`, `defaultRequireVec` is false, and every boot logs
`sqlite-vec not available; falling back from ANN to lexical search` four times.
Nothing fails — the JIT selector's vector half just quietly degrades to lexical
matching. Setting `CGO_CFLAGS` alone does not enable it. Measured 2026-08-10:
four warnings per boot without the tag, zero with it.

### Build With sqlite-vec Support (bash)

```bash
rm -f ./nerd.exe
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -tags sqlite_vec -o nerd.exe ./cmd/nerd
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

- Prompt or shard-behavior changes: read `internal/prompt/agents.md` first.
- Mangle logic changes: read `internal/mangle/agents.md` before editing `.mg` files.
- Core runtime or execution changes: read `internal/core/agents.md` and `internal/session/README.md`.
- Integration or "code exists but doesn't run" issues: audit wiring before deleting code or declaring a system unused.
- Specialized references live under `.claude/skills/` and `.codex/skills/`.

## Mangle Guardrails

- All predicates need `Decl` before use.
- Variables are uppercase; atoms use `/lowercase`.
- Negation only works when variables are already bound by positive atoms.
- Aggregation must use `|> do ... let ...` pipeline syntax.
- Do not use Mangle for fuzzy matching or large natural-language pattern banks. Use embeddings or other retrieval first, then assert structured facts into the kernel.
- For deeper syntax, failure modes, and examples, use `internal/mangle/agents.md` and the Mangle skill references instead of expanding this root file.

## Development Guidelines

- Run `go test ./...` before handoff.
- Keep new LLM systems JIT-first.
- Prefer adding prompt atoms and selection logic over hardcoding prose in shards.
- When in doubt, preserve the architectural north star and trim encyclopedic detail.

## Deep References

- `.claude/skills/codenerd-builder/references/`
- `.claude/skills/mangle-programming/references/`
- `.codex/skills/` mirrors these skills for Codex-oriented workflows.
