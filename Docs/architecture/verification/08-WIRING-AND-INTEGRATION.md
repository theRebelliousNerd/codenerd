# verification — Wiring and Integration

> Last verified: **2026-07-13**

## Boot wiring

### Primary interactive boot (`session_boot.go`)

Sequence (relevant fragment):

1. LLM client, localDB, shardMgr, taskExecutor already exist  
2. Autopoiesis orchestrator constructed and kernel-bridged  
3. **`verification.NewTaskVerifier(llmClient, localDB, shardMgr, autopoiesisOrch)`**  
4. **`taskVerifier.SetTaskExecutor(taskExecutor)`**  
5. `ChatConfig{ Verifier: taskVerifier, ... }` handed to model  

### Shared boot (`session_shared_boot.go`)

Same construction pattern; also attaches Verifier on config (~line 371).

### Model field

`cmd/nerd/chat/model_types.go`:

- `model.verifier *verification.TaskVerifier`  
- `ChatConfig.Verifier *verification.TaskVerifier`  

`model_update.go` assigns `m.verifier = c.Verifier` on config apply.

## Runtime integration — process OODA Act phase

File: `cmd/nerd/chat/process.go`

Preconditions for verification path:

1. Route is delegate (`RouteDelegate` or legacy shouldDelegate)  
2. `m.verifier != nil`  
3. `shouldVerifyDelegation(intent)` → **`intent.Category == "/mutation"`**  

Then:

```
SetSessionContext(sessionID, turnCount)
VerifyWithRetry(ctx, task, shardType, 3)
ResultToFacts → kernel.LoadFacts
if error is max-retries message → formatVerificationEscalation
else if error → errorMsg
else formatVerifiedResponse
```

If gate fails (non-mutation or nil verifier), process falls through to **`spawnTaskWithContext`** without the quality loop.

## Gating policy (caller-owned)

File: `cmd/nerd/chat/delegation_routing.go`

```go
func shouldVerifyDelegation(intent perception.Intent) bool {
    return intent.Category == "/mutation"
}
```

Tests in `routing_arbitration_roundtrip_test.go` pin:

- `/mutation` + `/fix` → verify  
- `/query` + `/review` → do not verify  
- `/instruction` → do not verify  

## Presentation layer

| Helper | When |
|--------|------|
| `formatVerifiedResponse` | Success path; shows confidence; strips piggyback envelopes via articulation processor when needed |
| `formatVerificationEscalation` | Max retries; synthesizes reason from violations/evidence if Reason empty (Gemini empty-reason quirk) |

## Store integration

Writer only:

```
TaskVerifier.storeVerification
  → LocalStore.StoreVerification(...)
  → INSERT task_verifications
```

DDL indexes: session_id, success, shard_type (`local_core.go`).

No CLI command in this package to dump history; consumers would use store APIs.

## Autopoiesis integration

Only when corrective type is `tool` and orchestrator non-nil:

```go
toolNeed := &autopoiesis.ToolNeed{Name: action.Query, Purpose: action.Reason, Confidence: 0.8}
tool, err := v.autopoiesis.GenerateTool(ctx, toolNeed)
```

Success injects tool name/description into retry context — does not automatically register tools into VirtualStore (that is autopoiesis/boot’s job if generation has side effects).

## TaskExecutor integration

Preferred path:

```go
req := session.TaskRequest{IntentVerb: intent, Task: task}
return v.taskExecutor.Execute(ctx, req)
```

Does **not** use `ExecuteWithContext` / session context / priority — simpler sync execute. Parallel chat path `spawnTaskWithContext` *does* inject session context; verification path currently does not pass `SessionContext` through. That is a real integration asymmetry (gap/open question).

## Kernel / facts

Package does not call kernel. Chat after verify:

```go
facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, verifyErr)
_ = m.kernel.LoadFacts(facts)
```

Even on verification failure/escalation, facts may still load (depends on ResultToFacts with verifyErr).

## Glass box / transparency

Direct `VerifyWithRetry` path does **not** emit the same “Spawning:” glass-box events as the non-verified spawn branch (those events sit on the alternate path). Integration gap for observability.

## Campaign / system factory

`SetTaskExecutor` appears on VirtualStore and campaign orchestrator for *delegation*, but **no** `NewTaskVerifier` in `cmd_campaign.go` or `internal/system/factory.go` was found. Campaign work does not currently share this quality loop unless it reuses chat process.

## Wiring checklist (for auditors)

- [x] Constructed at chat boot  
- [x] TaskExecutor set after construct  
- [x] Model field populated  
- [x] Mutation-only process gate  
- [x] Store table exists  
- [ ] SessionContext passed into executor on verify spawns  
- [ ] Glass-box parity with non-verify path  
- [ ] History read-back  
- [ ] Prompt atoms  
