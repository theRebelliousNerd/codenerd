# system — TODO

> Last verified: **2026-07-13**  
> Docs-only backlog derived from code review. No code changes in this corpus pass.

## P0

1. **Guard maintenance lifecycle**  
   - Store `StartMaintenanceSchedule` cancel on Cortex.  
   - Call cancel from `Close`.  
   - Nil-guard `runMaintenance` if LocalDB already closed.

2. **Unit-test GetOrBootCortex**  
   - Cache hit, multi-key, failed boot not cached, Close eviction.

## P1

3. **Unify TUI boot with cache**  
   - Either call GetOrBootCortex from chat, or register BootCortexWithConfig result into the same keyed cache with maintenance policy.

4. **Wire ResetCortexForWorkspace**  
   - From auth/provider/model change commands so next GetOrBoot is fresh.

5. **sessionVirtualStoreAdapter**  
   - Prefer VirtualStore paths for Read/Write when `vs != nil`.

## P2

6. Implement `LocalStoreTraceAdapter.LoadReasoningTrace`.  
7. Close MCP bridges on Cortex.Close.  
8. Log cache hit/miss (key prefix only).  
9. Test SpawnTask system-profile routing via mock ShardManager.  
10. Remove or relocate `debug_program_ERROR.mg` crash artifact from package tree.

## P3

11. Boot stage timing logs for CLI path (parity with TUI).  
12. Document disable-system-shards production policy in user docs (not just arch corpus).  
13. Consider per-key boot lock instead of global write lock (only if multi-workspace concurrent boot becomes hot).

## Done / already true (do not re-open without evidence)

- Keyed Cortex cache (Bug #15 fix)  
- Failed boots not cached  
- BootConfig DI overrides  
- missingLLMClient soft path  
- Hybrid prompt ingest  
- HolographicCodeScope for core/world cycle  
- User agent discovery + agents.json sync  
