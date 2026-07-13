# Secondary Reliability Fixes — Live Matrix Bugs

**Date:** 2026-07-13  
**Repo:** `C:\CodeProjects\codeNERD`  
**Scope:** Four secondary reliability bugs from live CLI matrix / wave2_gap probes  
**Constraint:** Small focused fixes + tests; no rewrites

---

## Summary

| # | Bug | Status | Reason |
|---|-----|--------|--------|
| 1 | Campaign pause OK but resume finds none / status stuck validating | **FIXED** | Pause was a stub that only printed a message |
| 2 | `nerd why` fails with "no modes declared" | **FIXED** | Hollow tracer required `descr[mode(...)]`; production Decls lack modes |
| 3 | `whatif` hangs after first kernel line | **FIXED** | Post-kernel path spawned researcher via SpawnTask and could stall indefinitely |
| 4 | `image_generator` spawn hang / missing from spawn help | **FIXED** | Undocumented type, BaseShardAgent no-op path, no fail-fast without Gemini client |

---

## 1. Campaign pause / resume / status stuck validating — FIXED

### Evidence
- Wave2: `07a_campaign_pause.txt` exit 0 → “Campaign paused…”
- `07b_campaign_resume.txt` → “No paused campaigns found.”
- `07c_campaign_status.txt` → Status still `/validating`

### Root cause
`runCampaignPause` was a **message-only stub**:

```go
// before
fmt.Println("Campaign paused...")
return nil  // never wrote StatusPaused to disk
```

Resume scanned campaign JSON for `StatusPaused` (or Active). Campaign stayed `/validating`, so resume found nothing.

### Fix
**File:** `cmd/nerd/cmd_campaign.go`

- `runCampaignPause` now loads latest non-terminal campaign, sets `StatusPaused`, updates `UpdatedAt`, atomically writes JSON.
- `runCampaignResume` prefers `StatusPaused`, then Active; flips paused → active on disk before re-running.
- Helpers: `findLatestPausableCampaign`, `findCampaignByStatuses`, `writeCampaignJSON`.

### Tests
`cmd/nerd/campaign_pause_test.go` — pause write + find paused after validating → paused.

---

## 2. `nerd why` "no modes declared" — FIXED

### Evidence
```
Tracing query: next_action(Var0)
Error: trace failed: predicate next_action has no modes declared
```
Also `why blocked` → parse error (`blocked` without arity → invalid atom).

### Root cause
1. CLI used hollow `mangle.Engine` + `ProofTreeTracer`, which hard-failed when `Decl` had no `descr[mode(...)]`.
2. Production schemas use `Decl next_action(ActionType) bound [/name].` (no modes).
3. Bare `blocked` is not a declared predicate (closest: `action_blocked`); arity fallback only covered a few names.

### Fix
**Files:**
- `internal/mangle/engine.go` — synthesize all-output modes when none declared (same as differential.Query).
- `cmd/nerd/cmd_query.go` — prefer `RealKernel.TraceQuery` (store scan, no modes); aliases `blocked` → `action_blocked`; always build valid query arity; hollow tracer as fallback.

### Tests
- `internal/mangle/engine_mode_synth_test.go`
- `cmd/nerd/campaign_pause_test.go` (`TestBuildWhyQuery`)

---

## 3. `whatif` hangs after first kernel line — FIXED

### Evidence
Live CLI: prints kernel implications, then TIMEOUT ~180s; no further output.

### Root cause
After printing `derives_from_hypothetical(...)`, `runWhatIf` called:

```go
cortex.SpawnTask(ctx, "researcher", prompt)
```

That path goes through JIT/spawn queue and could stall far beyond useful CLI latency, leaving the user with a hang after the first useful line.

### Fix
**File:** `cmd/nerd/cmd_advanced.go`

- Kernel implications remain the deterministic, always-complete result.
- Optional elaboration uses **bounded** `cortex.LLMClient.Complete` (45s), not `SpawnTask(researcher)`.
- Fail soft on timeout/error; command always returns after kernel section.

---

## 4. `image_generator` spawn hang / spawn help — FIXED

### Evidence
- Spawn help listed only generalist/specialist/coder/researcher/reviewer/tester.
- `nerd spawn image_generator …` hung ~600s, 0 stdout after spawn log line.

### Root cause
- Type not documented in CLI help.
- No registered factory → fell through to `BaseShardAgent` (no-op) or queue wait paths without a dedicated image agent.
- Missing Gemini image client did not fail closed early.
- Long default CLI timeout (25m) amplified hangs.

### Fix
| Area | Change |
|------|--------|
| `cmd/nerd/cmd_spawn.go` | Document `image_generator` in help; 3m outer budget for image spawns; nil-safe fact recording |
| `internal/core/shards/image_generator.go` | Real agent calling image LLM with timeout |
| `internal/core/shards/config.go` | `DefaultImageGeneratorConfig` (2m timeout) + description |
| `internal/core/shards/manager_spawn.go` | Fail closed if image type and `imageLLMClient == nil` |
| `internal/shards/registration.go` | Register factory + profiles for image aliases |

### Tests
`internal/core/shards/image_generator_test.go` — missing client, successful spawn with stub LLM.

---

## Verification

```
go test ./internal/mangle/ -run "TestEngineQuery_SynthesizesModesWhenMissing|TestProofTreeTracer_NoModesDeclaredRegression"  OK
go test ./internal/core/shards/ -run "TestImageGenerator|TestSpawnAsync_Image|TestSpawn_Image|TestCoreShardDescriptions"  OK
go test ./cmd/nerd/ -run "TestFindLatestPausable|TestBuildWhyQuery"  OK
go test ./internal/core/shards/ -count=1  OK
go test ./internal/mangle/ -count=1  OK
go build ./cmd/nerd/  OK
```

---

## Files touched

- `cmd/nerd/cmd_campaign.go`
- `cmd/nerd/cmd_query.go`
- `cmd/nerd/cmd_advanced.go`
- `cmd/nerd/cmd_spawn.go`
- `cmd/nerd/campaign_pause_test.go` (new)
- `internal/mangle/engine.go`
- `internal/mangle/engine_mode_synth_test.go` (new)
- `internal/core/shards/manager_spawn.go`
- `internal/core/shards/config.go`
- `internal/core/shards/image_generator.go` (new)
- `internal/core/shards/image_generator_test.go` (new)
- `internal/core/shards/shards_coverage_test.go`
- `internal/shards/registration.go`

---

## Not fixed (out of scope)

- Campaign remaining stuck in `/validating` **during** start (plan validation) is a lifecycle issue separate from pause/resume persistence.
- Live Gemini image quality / binary PNG write path (agent returns model text; file artifact pipeline not added).
- Full end-to-end live re-matrix against polystack APP (unit tests + build only in this pass).
