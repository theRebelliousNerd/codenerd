# 00 — Alignment & Vision Review (`internal/logging`)

> Last verified: **2026-07-13**  
> Scoring: 0–5 where 5 = fully aligned with codeNERD north star and observed code quality bar

## Scoring summary

| Dimension | Score | Notes |
|-----------|------:|-------|
| Inversion of control (LLM creative / logic executive) | **5** | Logging is pure side-channel; never owns `next_action` |
| Constitutional safety surface | **3** | Records `SafetyCheck` audits; does not enforce `permitted` |
| JIT / prompt-atom discipline | **4** | LLM I/O tracer supports JIT debugging without owning prompts |
| Wiring honesty | **4** | High real fan-in; shutdown partially incomplete |
| Production default-deny for diagnostics | **5** | No config / `debug_mode:false` ⇒ no files |
| Deterministic observability for logic systems | **4** | Audit Mangle strings ready; not loaded into kernel |
| Privacy / secret hygiene | **2** | Full prompt dump when `trace_llm_io`; no redaction |
| Test grounding | **5** | ~141 tests; concurrency + fact formatting covered |
| Config coherence with `internal/config` | **3** | Mirror struct; `json_format` vs `Format` drift |
| Scope discipline (not a kitchen-sink) | **5** | No browser/product terms; focused file telemetry |

**Composite:** ~**4.0 / 5** — solid substrate aligned with north star; main risks are privacy of LLM dumps, config field drift, and incomplete unified shutdown.

---

## Dimension evidence

### 1. Inversion of control — 5

**North star:** LLM describes; Mangle decides.

**Evidence:** Package exports only log/audit/trace APIs. No perception, no action routing, no fact assert into the executive kernel. Callers in perception/articulation/shards use it for diagnostics only (`LogLLMRequest` documents callsites like `"perception-transducer"`).

### 2. Constitutional safety — 3

**Evidence for:** `AuditEventType` includes `safety_check` / `safety_block` / `safety_allow`; `AuditLogger.SafetyCheck(action, allowed, reason)` records outcomes for post-hoc review.

**Evidence against:** Logging cannot block an action. Default-deny lives in policy Mangle rules, not here. Score is intentionally mid: the package is a **witness**, not a **gate**.

### 3. JIT / prompt atoms — 4

**Evidence:** `llm_io_logger.go` exists specifically to debug “what the LLM actually sees” for perception/articulation/JIT quality. It does not hardcode shard prompts; it dumps what callers pass.

**Gap:** No link to prompt-atom inventory or automatic callsite registry.

### 4. Wiring honesty — 4

**Evidence:** `logging.Initialize` from `cmd/nerd/main.go`, chat `session_boot.go` / `session_shared_boot.go`; dozens of internal packages import it. Not a dormant library.

**Gap:** `CloseAll` does not close audit/LLM I/O; `ReloadConfig` does not rebind open streams.

### 5. Production silence — 5

**Evidence:** `initializeInternal` returns early without creating `.nerd/logs` when `!DebugMode`. Missing config ⇒ production mode. `Get` returns no-op loggers. Audit and LLM I/O also gate on flags.

### 6. Deterministic / Mangle-queryable observability — 4

**Evidence:** `generateMangleFact` produces stable predicate-shaped strings; JSON audit lines include `mangle` field; `json_format` produces parseable category logs.

**Gap:** No in-repo consumer that asserts these facts into a Mangle program for live queries (offline only).

### 7. Privacy — 2

**Evidence against:** `LogLLMRequest` writes full system + user prompts; history truncated only at 2000 chars; responses untruncated. Intended for debug, dangerous if enabled on multi-tenant or secret-bearing workspaces.

### 8. Tests — 5

**Evidence:** Comprehensive table of unit tests for init, levels, categories, escape, every major mangle branch, concurrency, disabled paths. Benchmark for `escapeString` optimization (comment claims ~180×).

### 9. Config coherence — 3

**Evidence for:** Field names largely match `LoggingConfig` docs; dual helpers `IsCategoryEnabled` exist in both packages with same semantics.

**Evidence against:** `json_format` (bool) vs `format` (string); logging reads **only** `config.json`; YAML-only workspaces may not drive this package as operators expect.

### 10. Scope discipline — 5

**Evidence:** No Vectryx product terms; pure Go stdlib (`log`, `os`, `json`, `sync`). Clear non-goals relative to observability/transparency.

---

## Alignment verdict

Keep logging as a **thin, high-assurance witness**. Improve (in order): secret handling for LLM I/O, config schema single-source-of-truth, unified close, optional offline→kernel audit ingest tool **outside** this package if product needs executive use of logs.
