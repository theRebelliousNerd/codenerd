# 12 — Failure Modes: MCP

> Last verified against codebase: 2026-07-13

## FM1 — No integrations configured

**Symptom:** No MCP clients on VirtualStore; no bridge.  
**Cause:** `ToMCPServerConfigs` empty (all disabled or missing).  
**Mitigation:** Expected; agent runs without MCP. Enable servers in config.

## FM2 — Bridge init fails (store open)

**Symptom:** Warn `Failed to init MCP bridge`; no adapters.  
**Cause:** DB path permission, SQLite open failure.  
**Mitigation:** Ensure workspace writable; check `.nerd/` disk space.

## FM3 — Connect failure (server down)

**Symptom:** Status error; auto-connect Warn; CallTool “not connected”.  
**Cause:** Bad URL, refused connection, timeout, SSE endpoint never arrives.  
**Mitigation:** Fix base_url/endpoint; increase timeout; check protocol.

## FM4 — Unsupported / empty protocol

**Symptom:** Connect error `unsupported protocol` / `protocol cannot be empty`.  
**Cause:** Config typo.  
**Mitigation:** Use `http` | `stdio` | `sse`.

## FM5 — Discover lists zero tools

**Symptom:** Connected but empty catalog.  
**Cause:** Server has no tools; list parse failure; capability tools=false.  
**Mitigation:** Inspect server; ListTools errors appear as discover failures.

## FM6 — LLM analysis fails

**Symptom:** Warns; tools still saved with heuristic metadata.  
**Cause:** LLM down, parse failure, cancelled context.  
**Mitigation:** Heuristic path continues; later re-analyze if cache cleared.

## FM7 — Embedding / vec unavailable

**Symptom:** Debug “vector search disabled” or brute force; weaker selection.  
**Cause:** Nil embedder, sqlite-vec missing, dim mismatch.  
**Mitigation:** Fallback cosine; affinity-only scores still work.

## FM8 — Mangle selection always empty

**Symptom:** Debug “Mangle selection failed, using fallback”; Go hybrid used.  
**Cause:** Kernel nil; query error; **policy not loaded**; **no EDB facts**.  
**Mitigation:** Documented gap — load policy + assert facts; fallback remains usable.

## FM9 — Call with bad tool ID / traversal name

**Symptom:** Error invalid tool ID / directory traversal.  
**Cause:** Malformed `server/tool` or hostile name.  
**Mitigation:** Correct routing; reject is intentional.

## FM10 — Unserializable args

**Symptom:** `invalid arguments: cannot serialize to JSON`.  
**Cause:** Non-JSON types in map (channels, funcs, raw AST).  
**Mitigation:** Proxy also rejects non-primitives before manager.

## FM11 — Output too large

**Symptom:** Truncated output with marker.  
**Cause:** Tool returned >500KiB.  
**Mitigation:** Cap prevents memory blowups; caller may need paginated tools.

## FM12 — Race: call before connect completes

**Symptom:** Intermittent not-connected results after boot.  
**Cause:** ConnectAll in goroutine; adapters registered early.  
**Mitigation:** Retry; readiness gate (future); wait for status callback.

## FM13 — Stdio process crash

**Symptom:** Disconnect/errors on subsequent calls; stderr logs.  
**Cause:** Child process exit.  
**Mitigation:** Restart via Disconnect+Connect; fix command.

## FM14 — Usage stats drop

**Symptom:** Warn failed to record usage.  
**Cause:** Store closed or DB locked.  
**Mitigation:** Non-fatal; selection quality may drift.

## FM15 — Policy/schema path drift in tooling

**Symptom:** mangle-check looks for missing `internal/mcp/schemas_mcp.mg`.  
**Cause:** File lives under defaults.  
**Mitigation:** Point tooling at `internal/core/defaults/schemas_mcp.mg`.

## FM16 — Import cycle if boundaries broken

**Symptom:** Compile failure involving config/mcp/store.  
**Cause:** Reintroducing heavy imports across cycle edge.  
**Mitigation:** Keep IntegrationClient/KernelInterface local; conversion in config.

## Recovery summary

| Severity | Mode | Default behavior |
|----------|------|------------------|
| Boot soft-fail | FM2–FM3 | Agent continues without MCP |
| Selection soft-fail | FM7–FM8 | Fallback select |
| Call hard-fail | FM9–FM10 | Error returned |
| Degrade | FM6, FM11, FM14 | Partial success |
| Ops | FM15–FM16 | Fix paths / deps |
