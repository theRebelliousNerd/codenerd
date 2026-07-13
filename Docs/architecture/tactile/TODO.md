# tactile — TODO (prioritized)

> Last verified: **2026-07-13**  
> Docs-only backlog — items are recommendations, not claims of open PRs.

## P0

1. **Unify boot on audited Composite (or CreateBest)**  
   Chat/session boot should prefer VirtualStore modern executor path so execution facts always reach the kernel.

2. **Fail closed on missing sandbox backend**  
   Composite should not silently run Direct when `SandboxDocker|Namespace|Firejail` requested but unregistered.

3. **Caller audit**  
   Inventory every `NewDirectExecutor` outside VirtualStore; document which are policy-gated.

## P1

4. **Register Linux sandbox executors in Composite when available**  
   Firejail/Namespace auto-register on Linux Composite construction.

5. **Align Windows GetPlatformExecutor with Darwin**  
   Return Composite when Docker available.

6. **Fix RetryExecutor delay**  
   Use `time.NewTimer` / `select` on ctx; remove busy loop.

7. **Decl completeness pass**  
   Diff `AuditEvent.ToFacts` + file facts vs `internal/core/defaults/*.mg`.

## P2

8. **Idle container reaper** for PersistentDocker (config already has IdleTimeout).  
9. **Optional docker stats** → ResourceUsage for DockerExecutor.  
10. **FileOpPatch facts** if patch ops are used.  
11. **Integration test tag** for live Docker (skip if daemon absent).  
12. **CLI surface for swebench Evaluate** (optional; keep package general).

## P3

13. Expand OutputAnalyzer for pytest/cargo/npm patterns (structured only).  
14. Refresh package README (remove legacy `executor.go` mention).  
15. True interleaved Combined capture if required for debugging.  
16. Document Success semantics in public godoc even more loudly for external contributors.

## Done / non-goals

- Implementing constitutional policy inside tactile — **non-goal**.  
- Thin auto-inventory architecture stubs — **replaced by this corpus (2026-07-13)**.
