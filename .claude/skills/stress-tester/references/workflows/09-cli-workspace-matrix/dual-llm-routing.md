# Workflow: Dual-LLM Routing (main / worker / image)

## What It Stresses

- **Main** LLM (TUI agent / high-priority client): SuperGrok OAuth (`engine=xai-oauth`) or API key (`engine=api` + `xai_api_key`)
- **Optional worker** LLM: `config.worker` (typically Ollama `gemma4:12b`) for shards / spawn / create / classification — cheap local testing
- **Image** LLM (Nano Banana 2): `config.image` → Gemini **`gemini-3.1-flash-image`** — **must never** use Ollama worker
- Boot messages and shard manager injection paths that split these three clients

## Why This Exists (2026-07 live matrix)

Routing contract in `internal/config/user_config.go` and `cmd/nerd/chat/session_boot.go`:

| Role | Config key | Typical provider | Used for |
|------|------------|------------------|----------|
| Main | `engine` / `provider` / `model` | xAI Grok (oauth or api) | Interactive agent, high-priority |
| Worker | `worker` (optional) | Ollama | spawn, create, classification when set |
| Image | `image` | Gemini only | `image_generator` shard / Nano Banana 2 |

**Invariant:** Image generation is excluded from worker routing. `IsImageShardType` / `SetImageLLMClient` keep Gemini image models off Ollama.

**Auth note:** SuperGrok OAuth may return `invalid_grant` if refresh revoked → `nerd auth grok` or switch to `engine=api` + key for matrix runs.

## Severity Levels

| Level | Action |
|-------|--------|
| **Conservative** | Inspect config + boot messages; no image call if no Gemini key |
| **Aggressive** | Main Grok + worker Ollama: one create (worker path) + prove main still configured |
| **Chaos** | Misconfig image.provider=ollama (if accepted) / wrong model; assert fail-closed or re-route to Gemini defaults |
| **Hybrid** | create (worker) + spawn image_generator (Gemini) in same workspace serial |

## Conservative Procedure (PowerShell)

```powershell
$APP = Join-Path $env:TEMP "nerd-dual-llm-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $APP | Out-Null

# Ensure workspace config (copy from monorepo or write minimal). Example dual-LLM block:
# {
#   "engine": "api",
#   "provider": "xai",
#   "model": "grok-4",
#   "xai_api_key": "<key>",
#   "worker": { "provider": "ollama", "model": "gemma4:12b" },
#   "image":  { "provider": "gemini", "model": "gemini-3.1-flash-image" }
# }

$configPath = Join-Path $APP ".nerd\config.json"
# ... write or copy config ...

# 1) Boot smoke — capture routing messages (chat) or status/logs (CLI)
nerd status -w $APP 2>&1 | Tee-Object -Variable statusOut
# Worker may be offline: Ollama unreachable is a soft fail for embeddings/worker init

# 2) Prove image model identity from code/config defaults
# API id must be gemini-3.1-flash-image (Nano Banana 2), not an Ollama tag
$cfg = Get-Content $configPath -Raw | ConvertFrom-Json
if ($cfg.image) {
  $imgModel = $cfg.image.model
  if ($imgModel -and $imgModel -notmatch "flash-image|nano-banana") {
    throw "FAIL: image.model looks non-Gemini: $imgModel"
  }
  if ($cfg.image.provider -and $cfg.image.provider -ne "gemini") {
    throw "FAIL: image.provider must be gemini, got $($cfg.image.provider)"
  }
}

# 3) Optional: worker present implies shards can use ollama — main must remain non-ollama for dual setup
if ($cfg.worker -and $cfg.worker.provider -eq "ollama") {
  if ($cfg.engine -match "ollama" -or $cfg.provider -eq "ollama") {
    Write-Warning "Main is also Ollama — dual split not exercised (acceptable for pure-local)"
  }
}
```

### Aggressive — exercise worker vs image (when keys available)

```powershell
# Worker path: create should succeed via Ollama if worker up; else fall back / error clearly
Get-Process nerd -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
nerd create "Write routing-proof.txt with worker-or-main-ok" -w $APP --timeout 15m
if ($LASTEXITCODE -ne 0) { throw "create failed exit $LASTEXITCODE" }

# Image path: must not log Ollama as image provider
# Requires GEMINI_API_KEY / configured gemini credentials
nerd spawn image_generator "Generate a 1x1 solid blue PNG description only if image gen unavailable; otherwise minimal image" -w $APP --timeout 15m
# Inspect boot/session logs for:
#   "Image generator: gemini / gemini-3.1-flash-image (Nano Banana 2)"
# MUST NOT: image client initialized with provider ollama

$logs = Join-Path $APP ".nerd\logs"
if (Test-Path $logs) {
  $bad = Select-String -Path (Join-Path $logs "*") -Pattern "image.*ollama|Image generator: ollama" -ErrorAction SilentlyContinue
  if ($bad) { throw "FAIL: image path touched Ollama:`n$bad" }
}
```

## Pass Criteria

- [ ] Config documents three lanes: main, optional worker, image
- [ ] Image model defaults / config stay on Gemini Nano Banana 2 family (`gemini-3.1-flash-image` or lite)
- [ ] `image_generator` never initialized with Ollama worker client
- [ ] When `worker` unset, shards share main client (documented fallback)
- [ ] When worker Ollama is down, boot logs soft-fail for worker without crashing main
- [ ] OAuth `invalid_grant` handled by re-auth or API-key engine switch (env, not product panic)
- [ ] No panic / `debug_program_ERROR.mg`

## Log / Message Patterns

| Pattern | Expected |
|---------|----------|
| `Worker LLM for shards: ollama (model: …)` | Worker active |
| `Worker LLM init failed … (shards share main client)` | Soft fallback |
| `Image generator: gemini / gemini-3.1-flash-image (Nano Banana 2)` | Correct image path |
| `Image LLM (Nano Banana 2) unavailable` | Missing Gemini key — not an Ollama fallback |

## Related Surfaces

- `internal/config/user_config.go` — `WorkerLLMConfig`, `ImageLLMConfig`, `IsImageShardType`
- `cmd/nerd/chat/session_boot.go` — main / worker / image client wiring
- `perception.NewWorkerClientFromUserConfig` / `NewImageClientFromUserConfig`
- Panic catalog auth notes; [full-cli-surface.md](full-cli-surface.md) image section
