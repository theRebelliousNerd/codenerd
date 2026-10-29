# tools — Failure Modes

> Last verified: **2026-07-13**

## Catalog of concrete failures

### F1 — Tool not registered

**Symptom:** `ErrToolNotFound` / “not found in any registry”.  
**Causes:** Hydrate skipped; wrong registry (Global empty); typo in LLM tool name.  
**Mitigation:** Boot always HydrateModularTools; log Names() on debug; fuzzy not recommended — fail clear.

### F2 — Required arg missing

**Symptom:** `ErrMissingRequiredArg`.  
**Causes:** Model omitted field; schema mismatch.  
**Mitigation:** Schema Required accurate; retry with tool error content in multi-turn.

### F3 — Arg type mismatch

**Symptom:** `ErrInvalidArgType`.  
**Causes:** Model sent string for integer without coercion path.  
**Mitigation:** Registry validates declared types; tool bodies use coerceInt for known JSON float64 cases (file_ops, shell, lines partially).

### F4 — Workspace path escape

**Symptom:** `ErrPathOutsideWorkspace`.  
**Causes:** `../` or symlink out of tree on guarded tools.  
**Mitigation:** Correct for file_ops. Unguarded tools may **succeed** reading secrets outside root — treat as security failure mode of **missing guard**.

### F5 — File not found / permission

**Symptom:** OS errors wrapped “failed to read/write”.  
**Mitigation:** Return error string to model; do not invent content.

### F6 — edit_file old_text not found

**Symptom:** error “old_text not found in file”.  
**Mitigation:** Model re-reads file; common agent loop.

### F7 — Command timeout

**Symptom:** “command timed out after N seconds”.  
**Mitigation:** Partial stdout may return with error; increase timeout_seconds; avoid long builds without run_build defaults (300s).

### F8 — Command non-zero exit

**Symptom:** “command failed” + output.  
**Mitigation:** Tests/builds intentionally fail this way; model inspects stderr section.

### F9 — Shell injection attempt

**Symptom:** shellquote parse error or safe argv without shell metachar expansion.  
**Note:** `bash` tool intentionally runs scripts; higher risk by design — rely on allowlist + safety gate.

### F10 — Windows bash missing

**Symptom:** falls back to run_command on script string.  
**Risk:** different semantics.  
**Mitigation:** Install Git Bash or use run_command explicitly.

### F11 — Network failures (web/context7)

**Symptom:** request failed, HTTP non-200, parse empty.  
**Mitigation:** Timeouts 30–60s; empty-result friendly messages for context7.

### F12 — Browser start failure

**Symptom:** “failed to start browser”.  
**Causes:** Rod/Chrome missing, sandbox, permissions.  
**Mitigation:** SessionManager errors propagate; research can fall back to web_fetch.

### F13 — Browser session not found

**Symptom:** invalid session_id.  
**Mitigation:** navigate returns new session ID; model must retain ID.

### F14 — Test impact provider nil

**Symptom:** “test impact provider not initialized”.  
**Mitigation:** Ensure boot registers provider; or pass edited_refs after world ready.

### F15 — Dual registry desync

**Symptom:** VS has tool, Global does not (or reverse).  
**Causes:** partial Register without hydrate; test mutated one registry.  
**Mitigation:** Always RegisterAll both; Has() skip on rehydrate.

### F16 — Safety gate deny

**Symptom:** “blocked by safety gate” (session).  
**Not a tools bug** — policy denied.  
**Mitigation:** Inspect permitted rules / pending_action payload match.

### F17 — Executive gate deny / post-validation fail

**Symptom:** blocked by executive gate; post-action validation failed.  
**Mitigation:** Dreamer preflight / validation facts; model retries correctly.

### F18 — Output truncation

**Symptom:** `...[truncated]` in result.  
**Mitigation:** Narrow grep/path; increase max_length only when needed; session 16k feed-back cap separate from shell 50k.

### F19 — Cache miss as error

**Symptom:** research_cache_get returns error on miss.  
**Mitigation:** Callers treat miss as signal to fetch; not silent empty string.

### F20 — Global state pollution in tests

**Symptom:** flaky tests from Global registry / cache / browser.  
**Mitigation:** Prefer NewRegistry in unit tests; isolate browser integration.

## Severity prioritization

| Mode | Sev | Owner package |
|------|-----|---------------|
| F4 unguarded paths | Critical | tools |
| F9 bash allow without policy | High | session+tools |
| F15 desync | High | core boot |
| F14 provider | Medium | system/world |
| F11 network | Medium | ops/env |
| F18 truncation | Low | expected |
