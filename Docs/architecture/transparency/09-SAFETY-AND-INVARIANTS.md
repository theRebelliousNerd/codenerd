# transparency — Safety and Invariants

> Last verified: 2026-07-13

## 1. Relationship to constitutional safety

codeNERD’s executive rule: actions must derive `permitted(...)`; **default deny**.

Transparency’s job:

| Must | Must not |
|------|----------|
| Explain blocks | Override denies |
| Suggest `/shadow`, `/why`, remediation | Assert `permitted` |
| Classify safety-looking errors | Hide tool outcomes that failed for safety |
| Stay off the decide path | Soften policy for “better UX” |

Remediation text that mentions “override” is **operator guidance**, not an API that mutates the kernel.

## 2. Package invariants

### I1 — Disabled buses are pure no-ops

`GlassBoxEventBus.Emit` / `EmitImmediate` return immediately if `!enabled`. Producers may call freely.

### I2 — Tool bus is never “debug-gated” inside the type

`ToolEventBus.Emit` has no enabled flag. Gating tools would violate product honesty.

### I3 — Emit does not block producers

Full subscriber channels → drop. No unbounded wait.

### I4 — Manager feature methods respect master + flags

`StartShard` / `UpdateShardPhase` / `EndShard` require `IsEnabled() && config.ShardPhases`.  
`ReportSafetyViolation` requires `IsEnabled() && config.SafetyExplanations`.

### I5 — Histories are bounded

- Phase history max 100  
- Violations max 50  
- Consumers must bound their own rings (chat: 500 Glass Box events)

### I6 — ClassifiedError preserves unwrap chain

`Unwrap()` returns original error for `errors.Is` / `errors.As`.

### I7 — Explainer is pure formatting

No kernel evaluate, no assert, no file I/O. Nil/empty trace → safe strings.

### I8 — Dependency direction

No import of chat/UI; events remain plain data.

### I9 — Sequence monotonicity

Bus sequence uses `atomic.Uint64` Add; IDs increase for assigned events.

### I10 — Close semantics

`GlassBoxEventBus.Close` disables, closes subscriber channels, nils slice. After Close, further use is undefined for subscribers; producers still early-return if disabled.

## 3. Concurrency model

| Structure | Sync |
|-----------|------|
| TransparencyManager.enabled/config | `sync.RWMutex` |
| ShardObserver maps/history | `sync.RWMutex` |
| GlassBoxEventBus subscribers/categories/verbose | `sync.RWMutex` |
| GlassBox buffer/timer | `bufferMu` |
| enabled + sequence | `atomic` |
| SafetyReporter | **no mutex** on history slice |

### Concurrency gap (documented)

`SafetyReporter` methods mutate `violations` without locking. Safe if single-threaded chat use; **racy** if multiple goroutines call `ReportViolation` concurrently. Treat as invariant-to-fix or “single owner” convention.

`ToolEventBus` channel ops are concurrency-safe by Go channel rules; drop on full is racy only in the sense of which event is lost, not memory corruption.

## 4. Mangle surface

**None in-package.** No `Decl`, no rules. Explainer **reads** mangle derivation types only.

Negation/stratification issues do not apply here.

## 5. Secret handling

`SafetyReporter` treats paths containing credential-like tokens as `ViolationSecretExposure` and advises against VCS commit. Transparency itself may still put **target paths** into violation records and formatted strings—callers should avoid stuffing secret **values** into action/target/rule strings.

ToolEvent results are truncated but not redacted. Upstream should scrub secrets before Emit when possible.

## 6. Failure containment

| Failure | Containment |
|---------|-------------|
| Panic in PhaseObserver | Would propagate from notifyObservers (callers should keep observers safe) |
| Closed channel receive in chat | listen cmd returns nil |
| Malformed trace | Explainer returns “No derivation…” strings |
| Nil Manager methods | Callers nil-check or lazy-init (`/transparency`) |

## 7. Test gates for safety-relevant behavior

```powershell
go test ./internal/transparency/ -run "Safety|ClassifyError|FormatError|TransparencyManager_Report"
go test -race ./internal/transparency/  # catch bus/manager races; SafetyReporter may still be racy under stress
```

## 8. Invariant checklist for PR review

- [ ] No new asserts into kernel from transparency  
- [ ] Emit paths non-blocking  
- [ ] Tool events not gated by Glass Box off  
- [ ] New history buffers have a max  
- [ ] Status flags match real effects  
- [ ] Concurrent access documented or mutexed  
- [ ] Secrets not embedded in event Details by new producers  
