# Grok harness rules for codeNERD

These rules shape *how* the agent works in this repo. Product north star and build commands live
in root `AGENTS.md` — do not restate them here.

## Delegation defaults

Use subagents aggressively on this large codebase. Prefer parallel `explore` / specialist agents
over sequential monolithic searching.

| Task shape | Prefer |
|------------|--------|
| "Where is X?" / map a package | `explore` (quick → medium → very thorough) |
| Multi-file design before coding | `plan`, then implement |
| `.mg` rules, Decl, safety, stratification | skill `mangle-programming` + agent `mangle-logic-architect` |
| Idiomatic Go, concurrency, errors | skill `go-architect` + agent `go-architect` |
| Kernel / shards / VirtualStore / policy | skill `codenerd-builder` |
| "Code exists but doesn't run" | skill `integration-auditor` + agent `wiring-auditor` |
| Prompt atoms / JIT / Piggyback | skill `prompt-architect` + agent `prompt-jit` |
| Logs / cross-system trace | skill `log-analyzer` |
| Config / boot / engines | skill `codenerd-config-expert` |
| After substantial edits | skill `check-work` (or run build + targeted tests yourself) |
| Design feature before code | skill `arch-propose` **v3** (canonical under `.codex/skills/arch-propose`, synced to `.agents/skills/`) + fleet agents |
| Realize architecture corpus → code | skill `corpus-build` **v3** (canonical under `.codex/skills/corpus-build`, synced to `.agents/skills/`) + packetizer/foundation/wiring lanes + micro-skills |
| Codex fleet registry | `.codex/config.toml` + `.codex/agents/*.toml` + `.codex/hooks/` — see `Docs/architecture/_rebuild/CODEX_PORT_IS_CANONICAL.md` |
| Fill Docs/Spec from existing code | skill `spec-doc-sprint` |

**Design → build pipeline**: `arch-propose` → (human go) → `corpus-build` → optional `spec-doc-sprint` / `nerd-evolve`.

Built-ins always available: `general-purpose`, `explore`, `plan`.

Project agents also present: `mangle-logic-architect`, `nerd-evolve-*`, `arch-propose-*`, `corpus-*`.

## Wiring before deletion

This repo has partial integrations and dormant hooks. Before removing "unused" code:

1. Grep callers, registration sites, Mangle predicates, VirtualStore routes, and shard registration.
2. Prefer agent `wiring-auditor` or skill `integration-auditor` over gut feel.
3. Prefer fixing wiring gaps over deleting half-integrated features.

## Mangle edits

- Read `internal/mangle/agents.md` before non-trivial `.mg` changes.
- Every predicate needs `Decl`. Variables uppercase. Atoms `/lowercase`.
- Negation only after positive binding. Aggregations use `|> do … let …`.
- Prefer skill `mangle-programming` / agent `mangle-logic-architect` for new rules.

## Prompt / LLM-facing edits

- JIT-first: new behavior → prompt atoms under `internal/prompt/atoms/<category>/`.
- Do not grow shard prompts with ad-hoc prose when an atom + selector is possible.
- Read `internal/prompt/agents.md` before changing compiler/assembly paths.

## Verification bar

Before handoff on non-trivial changes:

1. Build with sqlite-vec CGO flags from root `AGENTS.md` when the binary is needed.
2. Run targeted `go test` for packages you touched; prefer `go test ./...` when feasible.
3. If Mangle sources changed, run mangle check tooling when available (`nerd mangle-check` / skill scripts).
4. Do not claim green without running the relevant checks.

## Scope discipline

- Keep root `AGENTS.md` short. Put depth in scoped `agents.md` or skills.
- Do not invent docs the user did not ask for.
- Prefer small, reversible edits; ask before destructive git / shared-remote actions.
