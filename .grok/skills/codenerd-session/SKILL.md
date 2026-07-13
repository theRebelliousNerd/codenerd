---
name: codenerd-session
description: >
  Bootstrap / orient a Grok session in the codeNERD repo. Use at the start of a new task
  stream, when the user asks "set up", "orient", "/codenerd-session", or when you need a
  quick map of skills, agents, and verify commands for this codebase.
user-invocable: true
---

# codeNERD session orientation

## 1. Product frame (do not re-derive)

- **North star**: LLM is creative center; Mangle kernel is executive.
- **Fact flow**: user input → perception → `user_intent` → `next_action` → VirtualStore → articulation.
- **Safety**: actions need `permitted(...)`; default deny.
- Full product rules: root `AGENTS.md`.

## 2. Load the right skill (read SKILL.md, do not invent)

| Domain | Skill |
|--------|-------|
| Kernel, shards, policy, VirtualStore | `codenerd-builder` |
| Mangle syntax / safety | `mangle-programming` |
| Go quality | `go-architect` |
| Prompt atoms / JIT | `prompt-architect` |
| Wiring gaps | `integration-auditor` |
| Logs | `log-analyzer` |
| Config / engines | `codenerd-config-expert` |
| Orchestration / campaigns | `nerd-orchestrator` |
| Post-change verify | `check-work` |
| Pre-implementation architecture corpus | `arch-propose` |
| Spec/corpus → implementation fleet | `corpus-build` |
| Fill Docs/Spec templates from code | `spec-doc-sprint` |

Skills live under `.agents/skills/` (and some under `.claude/skills/`). Invoke via slash (`/mangle-programming`) or by reading the skill file when the task matches.

**Pipeline**: `/arch-propose` designs → human decides → `/corpus-build` implements → `/spec-doc-sprint` documents live code.

## 3. Subagents to spawn

| Agent | When |
|-------|------|
| `explore` | Locate files / APIs fast |
| `plan` | Design before multi-file edits |
| `mangle-logic-architect` | Non-trivial `.mg` design/debug |
| `go-architect` | Non-trivial Go implement/review |
| `wiring-auditor` | "Exists but doesn't run" |
| `prompt-jit` | Atom / compiler / Piggyback work |
| `nerd-evolve-*` | Only for evolution-loop work |
| `arch-propose-*` | Scout / synthesize / audit pre-impl corpora |
| `corpus-reader` / `judge` / `builder` / … | Spec-to-code fleet |

## 4. Live commands

PowerShell build (sqlite-vec):

```powershell
if (Test-Path .\nerd.exe) { Remove-Item .\nerd.exe -ErrorAction SilentlyContinue }
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

Tests:

```powershell
go test ./...
# or targeted:
go test ./internal/core/... ./internal/prompt/...
```

Inspect harness:

```powershell
grok inspect
```

## 5. First moves on a new task

1. Restate the goal in one sentence.
2. Identify domain → load 1–2 skills max.
3. Spawn `explore` if the surface is unclear.
4. Prefer small diffs; audit wiring before deletes.
5. Verify with the narrowest meaningful tests.

## 6. Out of scope for this skill

Do not rewrite AGENTS.md or invent parallel architecture docs. Point to existing skills and agents instead.
