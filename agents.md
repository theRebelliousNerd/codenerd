# codeNERD

codeNERD is a high-assurance, logic-first CLI coding agent. The model is the creative center; the Mangle kernel is the executive. Logic determines reality; the model merely describes it.

## North Star

The current generation of coding agents makes a category error: it asks LLMs to do both creativity and executive control. codeNERD separates those roles. The LLM handles problem solving, synthesis, and insight; the deterministic Mangle layer handles planning, memory, orchestration, and safety.

This repo exists to make that split real in production: creative power with deterministic safety, long-horizon context without prompt drift, and parallel specialists whose behavior is grounded by logic rather than luck.

### Inversion of Control

- **LLM as creative center**: problem solving, synthesis, goal shaping, and insight.
- **Logic as executive**: planning, memory, orchestration, safety, and policy.
- **Transduction interface**: natural language and code are converted into formal atoms that the kernel can reason over.

### How This Shapes Runtime Changes

- The world model, durable facts, and CodeDOM are the model's primary codebase context. Use targeted, revision-aware source views when those representations need detail or refresh.
- Mangle manages the active working context throughout execution: relevance, retention, eviction, retrieval, and ordering. A growing tool transcript with occasional summarization does not fulfill this design. Evicted context must remain recoverable; stale evidence must not survive a source change as current truth.
- Models use typed, policy-mediated operations. Do not give codeNERD models free-form CLI/shell access by default or use it to bypass missing tool wiring. Build/test tools must constrain their inputs and expose structured results. Development agents may use their own shell to build and verify codeNERD.
- Bound each model request's context while allowing a task to continue as long as it makes progress within the user's constraints. Arbitrary tool-call counts are not task-completion criteria; detect stalls and repeated failures explicitly.
- Validate this architecture through the normal production entry paths with CodeDOM and world-model context enabled. Restricted-tool benchmarks are supplementary evidence.

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
- The north star is a final state, not a goal to be hit. It describes the behavior the
  finished system exhibits; the harness continuously derives the distance between what the
  evidence shows and that state, and applies that distance as pressure on every agent it
  runs. Nothing achieves the north star; every turn moves toward it, and the pressure never
  lets go.
- The original bet lives in `.codex/skills/codenerd-builder/references/` — an input to be
  judged, not a request to restore. `Docs/architecture/` (July 2026) is orientation, not the
  original, and not authoritative.

## Use codeNERD Where It Can Do the Job (a suggestion, not a mandate — for now)

codeNERD is meant to fix codeNERD, and every change routed through it is a measurement of
how close it is to that. So *prefer* aiming it at work it can plausibly land, and hand-edit
freely where it can't yet — this is a dogfood signal to collect, not a gate to pass.

```powershell
.\nerd.exe fix "<symptom, with the evidence: log lines, file:line, what was observed>"
```

What tends to land: one file, one named symptom, the evidence quoted, no diagnosis handed
over (a brief that carries the answer produces a stenographer). What tends not to: multi-file
causes, anything upstream of the file the symptom names, and tests — as of 2026-09-18 the
coder shard does not write them unless told, and its verdict can contradict its own evidence.

When you try it, record the outcome in the dogfood ledger
(`.claude/skills/codenerd-dogfood/references/component-ledger.md`): brief, minutes, tool
calls, what landed, what it missed. When you hand-edit instead, say so in the commit; if
codeNERD *couldn't* do it, say why — that is the finding, and fixing that blocker is the
highest-value dogfood there is.

Two things stay off-limits for codeNERD regardless: safety-gate and permission logic (the
model should not widen the rule that constrains it), and a broken build that stops it from
running at all.

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
- `agents.md` is a byte-identical clone of this file (this file is gitignored; `agents.md` is the tracked twin). Edit here, copy there, `cmp` them before committing.

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

## Grok Harness

Grok Build project wiring lives under `.grok/` (rules, agents, personas, roles, skills). Domain skills remain under `.agents/skills/`. Orient with `/codenerd-session` or `grok inspect`.

## Deep References

- `.claude/skills/codenerd-builder/references/`
- `.claude/skills/mangle-programming/references/`
- `.codex/skills/` mirrors these skills for Codex-oriented workflows.
