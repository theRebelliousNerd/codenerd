# verification — TODO

> Last verified: **2026-07-13**  
> Docs-only backlog — **not** an implementation commitment in this corpus pass.

## P0

1. **Fail-open policy decision** — document product default; optionally add strict mode when judge LLM errors.  
2. **Chat escalation check** — use `errors.Is(err, verification.ErrMaxRetriesExceeded)` in `process.go` instead of string equality.  

## P1

3. **Multi-attempt integration tests** with fake `TaskExecutor` + fake `LLMClient`.  
4. **Close the learning loop** — feed `GetQualityViolationStats` / recent history into `heuristicShardSelection` or selection prompts.  
5. **Research/docs corrective honesty** — either wire JIT research path or stop advertising Context7/web in comments/types.  
6. **`ExecuteWithContext` on verify spawns** — parity with non-verify delegation session context.  

## P2

7. **Extract judge + selector prompts** to `internal/prompt/atoms/` (JIT-first repo contract).  
8. **Glass-box events** for attempt N, violations, shard switch, corrective type.  
9. **Expand or document** `basicQualityCheck` coverage vs full `QualityViolation` set.  
10. **Fix hash naming** — store task hash and result hash distinctly.  

## P3

11. Config-driven `maxRetries` and optional confidence floor.  
12. Shared persona→intent package used by chat + verification (DRY).  
13. Optional CLI/`nerd` subcommand to dump session verification history.  
14. Prompt-injection hardening for judge system prompts.  

## Explicitly not TODO

- Porting verification into Mangle as primary engine  
- Verifying every query/review by default  
- Merging campaign assault scoring into this package  

## Wave 2 reconciliation (2026-09-25, verified against the code)

Most of this list predates the 2026-09-23 rework (sweep finding F5): the
kernel's verdict and `delegation_move` (`policy/delegation.mg`) now decide a
delegation, and the judge can only withhold. Each item re-checked:

| # | Item | Classification | Evidence |
|---|---|---|---|
| 1 | Fail-open decision | already done | `verifyTask` returns `ErrVerificationUnavailable` for a missing client, a failed call or a malformed judgment, and `VerifyWithRetry` fails the delegation closed; `verifier_failclosed_test.go`. |
| 2 | `errors.Is` for escalation in chat | already done | `cmd/nerd/chat/process.go`: `errors.Is(verifyErr, verification.ErrMaxRetriesExceeded)`. |
| 3 | Multi-attempt tests with fakes | already done | `verifier_delegation_test.go` (`TestVerifyWithRetry_*`: retry carries why, judge withholds at the configured confidence, rubric follows the persona, malformed judgment fails closed). |
| 4 | Feed history into shard selection | declined | `heuristicShardSelection` no longer exists; no LLM or Go heuristic picks the retry shard. A history input to `delegation_move` would be new kernel policy, a maintainer decision. `result_hash`/`GetQualityViolationStats` still have no production reader. |
| 5 | Research/docs corrective honesty | already done | `CorrectiveType` is advice: "They reach the retry as words; nothing here runs them" (`verifier.go`, `retryTask`). |
| 6 | Session context on verify spawns | built | commit 7aac9a3: attempts ran through `ExecuteObserved` with no session context; `Delegation.SessionContext`/`Priority` now reach every attempt via `ExecuteObservedWithContext`, and the chat call site passes the `sessionCtx` and `PriorityHigh` its unverified path uses. `TestVerifyWithRetry_EveryAttemptRunsWithTheDelegationsSessionContext`. Still open, in lane B's call site: verified attempts do not get `withShardModelContext`, so a persona's configured provider is not applied to them (the judge shares the ctx, so this needs a per-attempt hook, not a ctx change at the call site). |
| 7 | Judge + selector prompts as atoms | already done | `judgeSystemPrompt` compiles `internal/prompt/atoms/eval/delegation_judge.yaml`; the selector is gone. |
| 8 | Glass-box events per attempt | open | The verifier has no event bus; the chat owns the Glass Box (`cmd/nerd/chat`, lane B). Attempts are persisted (`StoreVerification`) and logged by the executor. |
| 9 | `basicQualityCheck` coverage | already done (deleted) | A malformed judgment fails closed instead of falling back to a keyword scan. |
| 10 | Task vs result hash naming | declined | `result_hash` is written and never read by production code; renaming a write-only column is churn. Revisit with a reader (the store is lane B's). |
| 11 | Configurable retries and confidence floor | already done | `Delegation.MaxAttempts` from `shard_profiles.<persona>.max_retries`; `delegation_judge_reject_confidence` is a required config param (`policy/delegation.mg`). |
| 12 | Shared persona -> intent mapping | already done | The executor's `intentFor` (kernel `persona_verb`) is the one mapping; chat's `personaToIntent` was removed (`delegation_routing.go`, finding F6). |
| 13 | CLI dump of verification history | out of lane | `cmd/nerd` (lane B). |
| 14 | Judge prompt-injection hardening | mitigated structurally | An injected "pass" cannot accept a delegation: the judge is asked only about a turn the kernel ended `/done`, and can only withhold. Data fencing in the judge atom would still be an improvement. |
