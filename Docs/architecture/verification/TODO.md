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
