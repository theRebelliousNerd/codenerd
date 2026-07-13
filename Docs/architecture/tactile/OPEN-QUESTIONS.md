# tactile — Open Questions

> Last verified: **2026-07-13**

1. **Should chat boot always force `initModernExecutor` / Composite**, or keep Direct for lower latency/complexity on trusted local workspaces?

2. **Fail closed vs soft fallback** for sandbox modes: product preference when Docker is requested but missing — hard error to user, or automatic direct with warning fact?

3. **Which execution facts are load-bearing for policy today** vs pure telemetry? Need a Decl+rule coverage matrix for `execution_*` predicates.

4. **Is PersistentDocker idle reaping required** before broader SWE-bench / python productization, or is explicit Teardown enough?

5. **Should FileEditor enforce workspace root jail** (path stays under WorkingDir), or remain caller-trusting forever?

6. **RetryExecutor role:** keep for flaky infra only, or expand to selected exit codes (dangerous for tests)? Current `shouldRetry` is very conservative.

7. **Windows Job Objects + nested jobs:** how often do real tools (VS, browsers) break assignment, and should LimitedExecutor be default on Windows?

8. **python.Environment vs campaign assault:** will long-horizon campaigns standardize on persistent containers, or stay host-Direct?

9. **tactile_router shard:** what is the long-term ownership boundary between shard routing and this package’s Executor — rename/docs only, or shared contracts?

10. **OutputAnalyzer home:** stay in tactile (motor-adjacent structured parse) or move next to verification/testing packages?

11. **MaxOutputBytes default 10MB:** adequate for `go test -json` / large builds, or should campaign/testing raise defaults globally?

12. **Are there remaining reverse deps** (tools, shards, JIT) that construct shell outside tactile that should be migrated for motor unification?
