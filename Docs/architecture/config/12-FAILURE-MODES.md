# 12 — Failure Modes: config

> Last verified: 2026-07-13  

## 1. Load failures

| Mode | Symptom | Mitigation |
|------|---------|------------|
| Missing `.nerd/config.json` | Empty UserConfig; defaults via Get* | First-run OK; wizard/init seed recommended |
| Malformed JSON | LoadUserConfig error | Fix JSON; do not soft-ignore without log |
| Malformed YAML | Load error | Fix YAML or delete to use defaults |
| Unreadable path (permissions) | Read error | Fix ACLs |
| Wrong workspace root | Config/DB appear “empty” or wrong project | Ensure go.mod at project root; remove stray nested `.nerd` |

## 2. Provider / engine failures

| Mode | Symptom | Mitigation |
|------|---------|------------|
| Explicit provider, missing key | GetActiveProvider returns empty key | Set matching `*_api_key` or env (YAML path) |
| Invalid engine string | SetEngine error | Use api/claude-cli/codex-cli/xai-oauth |
| Invalid YAML provider | Validate fails | Use ValidProviders |
| Env key overwrites YAML provider (OPENAI after ANTHROPIC…) | Surprising provider | Understand applyEnvOverrides order; prefer JSON + explicit provider |
| Expecting env on JSON path | Key empty at runtime | Write key into config.json or add helper |

## 3. Dual-default surprises

| Mode | Symptom | Mitigation |
|------|---------|------------|
| Shard concurrency 4 vs 12 | Too few/many parallel shards depending on path | Always load UserConfig for runtime; unify defaults (gap) |
| Execution timeout 30s vs 10m | Tactile commands timeout or hang | Know which aggregate caller used |
| Context window 128k vs 200k | Compression too early / late | Prefer GetContextWindowConfig on UserConfig |

## 4. Scheduler / timeout failures

| Mode | Symptom | Mitigation |
|------|---------|------------|
| Slot acquire timeout | Calls fail waiting | Raise `slot_acquire_timeout_sec` or max concurrent |
| Context shorter than HTTP | Premature cancel | Align GetLLMTimeouts tiers; avoid ad-hoc 90s contexts |
| Adaptive concurrency stuck low | Throughput drop after 429s | Tune adaptive_recover_after_sec; wait for recovery |
| Subscription spacing | Slower than API mode | Expected for xai-oauth/codex/claude-cli |

## 5. Embedding / Ollama failures

| Mode | Symptom | Mitigation |
|------|---------|------------|
| Bare `embeddinggemma` | Ollama 404 | Helper rewrites to `:300m`; ensure config.json uses tagged model |
| Wrong endpoint | Connection refused | Set embedding.ollama_endpoint / OLLAMA_ENDPOINT (YAML path) |
| Worker model missing | Shard LLM fails | Pull model; set worker.model |

## 6. Features / save failures

| Mode | Symptom | Mitigation |
|------|---------|------------|
| Save Features mid-session | Disk updated but process still old flags | Reload process or re-call LoadUserConfig |
| MkdirAll/write fail | Save error | Disk full / permissions (watch C: free space) |
| Marshal of circular/unexported | N/A for config structs | — |

## 7. Safety-adjacent failures

| Mode | Symptom | Mitigation |
|------|---------|------------|
| Over-broad AllowedBinaries | Tactile can run more tools | Keep list tight; kernel still gates |
| Keys in world-readable config | Credential leak | Restrict filesystem ACLs; prefer env for CI |
| Glass box / TraceLLMIO true | Secrets in prompts may hit disk logs | Disable traces in shared envs |

## 8. Recovery playbook

1. Confirm workspace: `go.mod` present; path from `FindWorkspaceRoot`.  
2. Validate JSON with a formatter.  
3. Check `provider` + matching `*_api_key` or engine block.  
4. Check `engine` for CLI/OAuth modes and credential files.  
5. Check `core_limits` / `api_scheduler` if rate limited.  
6. Run `go test ./internal/config/...` after code changes to helpers.
