# verification — Safety and Invariants

> Last verified: **2026-07-13**

## Scope of “safety” here

This package enforces **quality / non-fraudulent completion**, not OS-level or constitutional permission. Still, it is a safety-adjacent control: it reduces the chance that the agent **presents incomplete or fake work as done**.

## Invariants

### I1 — Loop bound

`VerifyWithRetry` runs at most `maxRetries` attempts (default 3). It cannot infinite-loop on quality failure.

### I2 — Escalation on exhaustion

If all attempts fail quality (or soft conditions that still leave Success false / violations nonempty), the error is `ErrMaxRetriesExceeded`. Last result is still returned for human inspection.

### I3 — Spawn failures are not quality retries

If `spawnTask` errors, the function returns immediately. It does not burn retries on infrastructure failure or reinterpret spawn error as a quality violation.

### I4 — Success conjunction

Accepted completion requires:

```
verification.Success == true AND len(verification.QualityViolations) == 0
```

Either side alone is insufficient.

### I5 — Intent verbs must be slash-prefixed before TaskExecutor

`normalizeIntentVerb` guarantees empty → `/general`, personas mapped, else `/consult/<name>`. Protects against hard validation failures mid-retry.

### I6 — Review mode must not punish reported defects

When `isReviewTask` is true, the system prompt instructs the judge that reporting incomplete code is success. (Caller may still skip verification for pure queries.)

### I7 — Nil LocalStore is safe

`storeVerification` returns immediately if `localDB == nil`. No panic.

### I8 — Mutex for session/executor fields

`SetTaskExecutor` and `SetSessionContext` take `mu`. `storeVerification` reads session under `RLock`.

### I9 — No package-level mutable global verifier

State is instance-bound on `*TaskVerifier`.

## Non-invariants (explicit)

| Claim | Reality |
|-------|---------|
| “No mock code ever ships” | False if LLM judge is nil/down (fail-open) |
| “Corrective research always hits the web” | False — specialist-only today |
| “result_hash is content of result” | False — SHA-256 of **task** |
| “JSON schema enforced strictly” | Unmarshal is best-effort; wrong types may zero fields |
| “Thread-safe concurrent VerifyWithRetry” | Not guaranteed for shared instance parallel calls |

## Fail-open policy (quality vs availability)

| Condition | Behavior | Safety implication |
|-----------|----------|--------------------|
| `client == nil` | Success=true, conf 0.5 | Offline may accept bad work |
| LLM Complete error | Success=true, conf 0.3, reason skipped | Same |
| JSON parse error | `basicQualityCheck` | Catches some placeholders; not all violations |
| Autopoiesis tool fail | Empty corrective context | Retry weaker, not false success |
| Specialist spawn fail | Fall through other corrective types | Degraded |

**Recommendation:** document product modes:

- **Strict:** verify errors → fail (or force basic check + fail if incomplete)  
- **Lenient (current):** prefer forward progress  

## Concurrency

- Single-call loop is sequential.  
- `ctx` cancellation should abort LLM and executor if they honor context (depends on implementations).  
- Do not share one `TaskVerifier` across parallel mutations without additional locking review (unexported fields mutated in loop without holding `mu`).

## Constitutional / Mangle

No Decl predicates. No `permitted` checks inside verification. Kernel policy remains authoritative for action legality at VirtualStore/execution boundaries.

## Secret / prompt injection notes

- Task and result text are fed to the judge LLM. Malicious task content could try to jailbreak the judge (“ignore quality rules”). Mitigations are prompt-hardening (not currently specialized).  
- Truncation limits prompt size (DoS-ish token blowups).  

## Testing as safety net

Invariants partially enforced by tests:

- max retries error constant text  
- normalize table  
- basic quality multi-violation  
- nil client soft success  
- no executor hard error  

Not covered well: concurrent use, live LLM jailbreak resistance, integration with real TaskExecutor multi-retry.
