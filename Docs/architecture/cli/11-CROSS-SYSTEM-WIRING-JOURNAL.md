# 11 — Cross-System Wiring Journal (CLI)

> Last verified: 2026-07-13  
> Purpose: prove how CLI features attach to live internal systems (not aspirational wiring).

## 1. Interactive chat boot chain

| Step | Code | Downstream |
|------|------|------------|
| 1. Resolve workspace | `main.go` RunE `Chdir` | All relative `.nerd/` paths |
| 2. Start TUI | `chat.RunInteractiveChat` | Bubble Tea program |
| 3. Boot Cortex | `session_boot.go` / `session_shared_boot.go` | config, logging, store, core, shards, perception, prompt, … |
| 4. Accept input | `process.go` | perception intents, slash router |
| 5. Execute | session/kernel/VirtualStore | tools, file ops, shard tasks |
| 6. Render | `view.go`, glass box | user-visible state |

## 2. Single-shot `nerd run`

| Step | Code | Downstream |
|------|------|------------|
| Parse instruction | `cmd_instruction.go` | — |
| Boot cortex | `internal/system` helpers | kernel + shards |
| OODA loop | instruction runner | perception → kernel → act |
| Print result | stdout | — |

## 3. Campaign wiring

| Surface | Code | Package |
|---------|------|---------|
| Cobra | `cmd_campaign.go` | `internal/campaign` |
| JIT prompts for roles | `campaign_jit_provider.go` | `internal/prompt` via campaign roles |
| Chat | `chat/campaign.go`, `/campaign` | same |

Assault artifacts: `.nerd/campaigns/<id>/assault/` (documented in `cmd/nerd/README.md`).

## 4. Auth wiring

| Provider | Command | Notes |
|----------|---------|-------|
| Claude CLI | `auth claude` | subscription CLI backend |
| Codex CLI | `auth codex` | subscription CLI backend |
| Grok | `auth grok` | API/path specific |
| Status | `auth status` | aggregate readiness |

Implementation: `cmd_auth.go` + perception/config engine clients.

## 5. Mangle developer tools

| Command | Purpose | Package |
|---------|---------|---------|
| `check-mangle` | syntax/semantic validation of `.mg` | mangle tooling |
| `mangle-lsp` | IDE language server | mangle LSP |

These are **dev tools** on the CLI; runtime evaluation still goes through kernel load of policy/schemas.

## 6. Transparency wiring

| Command / UI | Package |
|--------------|---------|
| `glassbox`, `/glassbox` | `internal/transparency` + chat glass_box |
| `transparency`, `/transparency` | same |
| `reflection`, `/reflection` | chat reflection + transparency |

## 7. Intentionally unwired / legacy notes

- Direct imports of coder/tester/reviewer shards in chat boot are commented out; behavior is expected via JIT / intent routing. If a verb silently no-ops, treat as **wiring regression**, not “unused code delete candidate”, until registration paths are audited (`integration-auditor` / `wiring-auditor`).

## 8. Verification checklist for new wires

1. Grep registration / AddCommand / slash case.
2. Boot path imports package.
3. At least one test or manual command path exercises it.
4. Update this journal table.
