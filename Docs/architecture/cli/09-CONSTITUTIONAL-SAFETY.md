# 09 — Constitutional Safety at the CLI Edge

> Last verified: 2026-07-13  
> Safety authority remains the kernel policy corpus; CLI must not bypass it.

## 1. Threat model (CLI-specific)

| Threat | Example | Mitigation |
|--------|---------|------------|
| Unauthorized destructive shell/file ops | Agent issues `rm -rf` via tools | Kernel `permitted` + Dreamer + tool policy |
| Workspace escape | `--workspace` to sensitive paths | OS permissions; still user-responsible; clear errors |
| Infinite agent loops | Stuck multistep | Timeouts (`--timeout`, interactive 30m), cancel signals |
| Panic process death | Bug in processInput | Recover → errorMsg |
| Credential leakage in logs | API keys in verbose traces | Avoid logging secrets; audit debug flags |
| Disabled safety shards | `--disable-system-shard` abuse | Operator-only; document risk |

## 2. CLI controls that support safety

| Control | Location |
|---------|----------|
| Global timeout | `main.go` PersistentFlags |
| Disable system shards | `run` flag + chat.Config |
| Dry-run / trace flags | `cmd_debug.go` |
| Shadow / whatif / dream | `cmd_advanced.go`, chat handlers |
| Approve / reject findings | chat slash commands |
| Signal cancel | `cmd_interactive.go` SIGINT/SIGTERM |

## 3. Non-negotiables

1. Do not add a “yolo” flag that skips `permitted` checks.
2. Do not execute raw user shell without routing through VirtualStore/policy.
3. Do not log full API keys or tokens at Info level.
4. Prefer shadow/dream for speculative exploration before mutation.

## 4. Related packages

- `Docs/architecture/core/` — Dreamer, VirtualStore  
- `Docs/architecture/mangle/` — policy evaluation  
- `Docs/architecture/autopoiesis/` — generated tool safety  
