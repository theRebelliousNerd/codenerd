# verification — Open Questions

> Last verified: **2026-07-13**

## Product / policy

1. **Should fail-open remain the default** when the judge LLM is unavailable, or should production boots fail-closed (escalate / basic-check hard fail)?  
2. **Is `/mutation` the right sole gate forever?** Some high-stakes `/query` tools (e.g. security audit generators that emit code) might need verification.  
3. **Should escalation auto-open a multistep / plan path** instead of only messaging the user?  

## Architecture

4. **Who owns persona→intent mapping** long-term — verification, chat, session, or a neutral `internal/types`/`internal/routing` helper?  
5. **Should `TaskVerifier` become an interface** for easier chat tests and alternate implementations (e.g. no-op, strict static)?  
6. **Should verification assert Mangle facts** like `task_verification(session, attempt, success, …)` for kernel-visible learning?  
7. **Is write-only SQLite enough**, or should verification history enter Vectryx/long-term memory when paired deployments exist? (Stay general — no product-specific schema invention here.)  

## Correctives

8. **What is the intended research path post-“researcher removed”?** Specialist only, TaskExecutor `/research`, or Context7 tool?  
9. **Should `CorrectiveDecompose` invoke multistep decomposer** instead of a markdown hint?  
10. **Does `GenerateTool` during a retry create tools visible to the same retry’s spawn**, or only later turns?  

## Judge quality

11. **How should confidence interact with Success?** Today confidence is display/persist only.  
12. **Should head+tail truncation replace head-only** for large results?  
13. **Can `isReviewTask` be replaced by intent.Verb/Category** from perception for fewer false classifications?  

## Integration

14. **Should campaign/JIT assault use TaskVerifier?** Currently no construction found outside chat.  
15. **Verify path vs spawn path SessionContext parity** — intentional simplification or bug?  
16. **Glass-box silence during retries** — acceptable or UX defect?  

## Testing

17. **What is the minimum fake-LLM contract** the package should ship for downstream tests?  
