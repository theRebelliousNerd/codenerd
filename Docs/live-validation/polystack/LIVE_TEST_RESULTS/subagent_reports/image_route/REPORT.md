# Image LLM Routing Audit — Nano Banana 2

**Repo:** `C:\CodeProjects\codeNERD`  
**Date:** 2026-07-13  
**Verdict:** **Image path is isolated after fixes** (was partially wired; spawn path could hit worker Ollama)

---

## Summary

| Check | Status | Location |
|-------|--------|----------|
| `IsImageShardType` | OK | `internal/config/user_config.go:555-566` |
| `GetImageLLMConfig` defaults `gemini-3.1-flash-image` | OK | `internal/config/user_config.go:533-534,568-592` |
| `NewImageClientFromUserConfig` never Ollama | OK | `internal/perception/client_factory.go:427-455` |
| `SetImageLLMClient` / `clientForShardType` | OK (fixed no-worker fallback + spawn lock) | `internal/core/shards/manager.go:165-194` |
| Factory boot wires image client | OK | `internal/system/factory.go:561-569,882-884` |
| Chat `session_boot` wires image client | OK | `cmd/nerd/chat/session_boot.go:374-387` |
| `Cortex.SpawnTask` uses image client | **FIXED** | was TaskExecutor/worker; now ShardManager |
| Campaign spawn paths set image client | **FIXED** | `cmd/nerd/cmd_campaign.go` start + resume |
| Spawn deadlock on image client inject | **FIXED** | nested RLock under write lock |

---

## Verified surfaces (with line refs)

### 1. Config — `IsImageShardType` / `GetImageLLMConfig`

**File:** `internal/config/user_config.go`

- `DefaultImageModel = "gemini-3.1-flash-image"` — **L533-534**
- `IsImageGenerationModel` — flash-image / lite / nano-banana aliases — **L536-553**
- `IsImageShardType` accepts:  
  `image_generator`, `image-generator`, `imagegenerator`, `imagen`, `image`, `nano_banana`, `nanobanana` — **L555-566**
- `GetImageLLMConfig` defaults provider `gemini` + model `DefaultImageModel`; normalizes aliases `nano-banana-2` → API id — **L568-592**
- Worker config explicitly does **not** cover image gen — comment **L511-513**

**Tests (pre-existing):** `internal/config/ollama_worker_config_test.go:35-64`

### 2. `NewImageClientFromUserConfig` — never Ollama worker

**File:** `internal/perception/client_factory.go:427-455`

- Reads `GetImageLLMConfig()` only
- Rejects non-`gemini` providers (`fmt.Errorf("image provider %q not supported...")`)
- Requires `gemini_api_key` / `GEMINI_API_KEY` / `GOOGLE_API_KEY`
- Builds **only** `NewGeminiClientWithConfig` with image model
- No call path to `NewOllamaClient*` / `NewWorkerClientFromUserConfig`

Contrast worker path: `NewWorkerClientFromUserConfig` **L369-425** (ollama case **L381-394**).

**New test:** `TestNewImageClientFromUserConfig_NeverOllama` in `internal/perception/client_ollama_test.go`

### 3. ShardManager — `SetImageLLMClient` / `clientForShardType`

**File:** `internal/core/shards/manager.go`

- Field `imageLLMClient` — **L45-47**
- `SetImageLLMClient` — **L165-173**
- `clientForShardType` — **L175-194**
  - Image types → `imageLLMClient` only (**no** fallback to worker/main) — **fixed this audit**
  - Other types → `llmClient` (worker/main)
- Spawn injects client via `clientForShardTypeLocked` — `manager_spawn.go:305-308`  
  - **Deadlock fix:** spawn holds write lock; must not call `RLock` variant

**New test:** `TestShardManager_clientForShardType_ImageIsolation` in `manager_accessors_test.go`

### 4. Factory + session_boot wiring

**Factory** `internal/system/factory.go`:

- Creates image client in `initPerceptionLayer` — **L561-569**  
  `NewImageClientFromUserConfig` → `NewScheduledLLMCall("image_generator", imgClient)`
- Attaches in `initShardManagement` — **L882-884**  
  `shardManager.SetImageLLMClient(bctx.imageLLMClient)`

**Chat boot** `cmd/nerd/chat/session_boot.go:374-387`:

- Worker → `shardMgr.SetLLMClient(shardLLMClient)` (**L372-373**)
- Image separate → `SetImageLLMClient(imgScheduled)` (**L375-379**)
- Boot message: `Image generator: gemini / %s (Nano Banana 2)` (**L380-385**)

**TaskExecutor path** (`initFinalExecutors` **L1018-1024**) still uses **worker/main only** for JIT — correct for coder/spawn-create; **must not** handle image (see fix below).

### 5. Config default model

- Code default: `gemini-3.1-flash-image` (`DefaultImageModel`)
- Workspace `.nerd/config.json` `image.model` also set to that id (operator config; not part of this audit)

---

## Gaps found and fixed

### GAP-1 — `Cortex.SpawnTask` bypassed image client (critical)

**Before:** Non-system shards (including `image_generator`) went to `TaskExecutor`  
(`factory.go` old L286-294).  
`normalizeTaskIntentVerb` mapped `image_generator` → `/create`  
(`task_executor.go` old L90-92), which runs on **worker Ollama**.

So `nerd spawn image_generator …` could hit Ollama despite ShardManager having Nano Banana 2 attached.

**Fix:**

1. `Cortex.SpawnTask` / `SpawnTaskWithContext` — if `config.IsImageShardType(normalized)`, always route through `ShardManager` (`factory.go` SpawnTask block).
2. `normalizeTaskIntentVerb` for image names **fails closed** (no `/create` map) so accidental TaskExecutor use cannot silent-misroute (`task_executor.go`).

**Test:** `TestCortex_SpawnTask_ImageRoutesToShardManagerNotTaskExecutor`,  
`TestNormalizeTaskIntentVerb_ImageFailsClosed`

### GAP-2 — Image client missing on campaign CLI ShardManagers

**Before:** `cmd/nerd/cmd_campaign.go` called only `SetLLMClient` (start ~L259, resume ~L709).

**Fix:** After `SetLLMClient`, call `NewImageClientFromUserConfig` + `SetImageLLMClient` (start + resume).

### GAP-3 — `clientForShardType` fell back to worker when image client nil

**Before:** `IsImageShardType && imageLLMClient != nil` else worker — silent Ollama for image.

**Fix:** Image types always return `imageLLMClient` (may be nil); never worker.

### GAP-4 — Deadlock: spawn held write lock + `clientForShardType` RLock

Any spawn that reached client selection deadlocked on `sm.mu`.

**Fix:** `clientForShardTypeLocked` for callers already holding the lock; spawn uses it.

---

## Isolation diagram (post-fix)

```
config.image ──► GetImageLLMConfig ──► gemini-3.1-flash-image
                      │
                      ▼
         NewImageClientFromUserConfig ──► GeminiClient only
                      │                    (never Ollama)
        ┌─────────────┼─────────────────┐
        ▼             ▼                 ▼
 session_boot    factory boot      campaign start/resume
        │             │                 │
        └─────────────┴─────────────────┘
                      ▼
           ShardManager.SetImageLLMClient
                      │
   spawn image_* ─────┼──► clientForShardTypeLocked → imageLLMClient
   spawn coder    ────┘──► llmClient (worker/main)

   Cortex.SpawnTask("image_generator") → ShardManager (NOT TaskExecutor)
   TaskExecutor + image_* name         → error (fail closed)
```

---

## Residual notes (not blocking isolation)

1. **No dedicated image_generator factory** — spawn falls back to `BaseShardAgent` (or registered profile). Client injection is correct; agent body is still generic (`BaseShardAgent.Execute` is a stub string unless overridden). Full image-gen UX may need a specialist agent later; **routing** is isolated.
2. **JIT chat loop** for free-form “draw me an image” still uses main/worker LLM unless user/spawn path hits `image_generator`. Perception does not auto-select Nano Banana for natural language image requests.
3. **`taskDelegatorAdapter` / VirtualStore** with intent `image_generator` still hits TaskExecutor and now **errors** (fail closed) rather than Ollama. Prefer Cortex.SpawnTask or ShardManager for image work.
4. **API keys** in local `.nerd/config.json` are operator secrets; not logged in this report.

---

## Tests run

```
go test ./internal/core/shards/ -run "clientForShardType|ModelCapability|ModelName"  # ok
go test ./internal/system/      -run "SpawnTask_Image|SpawnTask_Routing"             # ok
go test ./internal/perception/  -run "NewImageClient"                                # ok
go test ./internal/session/     -run "ImageFailsClosed"                              # ok
go test ./internal/config/                                                          # ok (prior full package)
```

---

## Confirmation

**Image LLM path is isolated:** Gemini Nano Banana 2 (`gemini-3.1-flash-image`) is created only via `NewImageClientFromUserConfig`, attached via `SetImageLLMClient`, selected by `clientForShardType` for image shard types, and `Cortex.SpawnTask` / campaign boots now preserve that isolation. Image work no longer falls through to the Ollama worker path.
