## 2024-05-13 - Siege Journal Init
**Learning:** Understanding the codeNERD architecture is crucial before finding vulnerabilities. The system uses a JIT Clean Loop architecture where shards are replaced by JIT-driven SubAgents.
**Action:** Proceed with identifying a pipeline or boundary test surface.
## 2024-05-13 - Session Executor Clean Loop
**Learning:** Found cascading failure path where JIT compilation fallback uses a hardcoded prompt ("You are an AI assistant helping with software development.") that does not inform the LLM about available tools, likely causing tool schema violations or tool hallucinations in subsequent steps. Also, tool context timeouts might leak goroutines if tools do not respect `ctx.Done()`.
**Action:** Create an integration test crossing Session, JIT Compiler (mocked to fail/hang), ConfigFactory, and LLM to assert fallback behavior and tool hallucination blocking.
