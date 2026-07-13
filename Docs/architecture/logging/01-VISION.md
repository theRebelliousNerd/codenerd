# 01 — Vision (`internal/logging`)

> Last verified: **2026-07-13**  
> Status: Target architecture (product intent for this package)

## Purpose

Give every codeNERD subsystem a **cheap, categorized, workspace-local evidence channel** so humans (and offline tools) can reconstruct:

- What boot did  
- How perception/articulation framed LLM work  
- Which actions VirtualStore routed  
- How long operations took  
- Whether constitutional checks allowed or blocked  

…without the log path ever becoming part of the executive decision loop.

## Target properties

### 1. Default silent production

Production installs write **zero** diagnostic files unless an operator enables `debug_mode`. Silence is a feature: disk, privacy, and performance.

### 2. Category isolation

Operators enable only the systems under investigation (`kernel`, `jit`, `campaign`, …) so log volume stays human-scale.

### 3. Three streams, one switch

| Stream | Audience |
|--------|----------|
| Category text/JSON files | Engineers grepping subsystem behavior |
| Audit JSON + Mangle facts | Offline logic analysis, campaign postmortems |
| LLM I/O full dump | Prompt/JIT quality debugging (explicit opt-in) |

### 4. Correlation without distributed tracing vendor

`RequestLogger` request IDs, audit `SessionID` / `ShardID` / `RequestID` fields — enough to stitch a single OODA turn across files **without** OpenTelemetry mandate.

### 5. Performance honesty

Timers surface slow paths via thresholds and sampling so debug mode does not flood disks with every 1ms call.

### 6. Witness of safety, not enforcement

Audit records safety allow/block. The vision **rejects** moving `permitted(...)` evaluation into the logger.

### 7. Config single source of truth (target)

Operators should edit **one** logging schema that both `internal/config` and this package honor (including format mode and category map), whether loaded from JSON or YAML.

### 8. Safe LLM tracing (target)

When `trace_llm_io` is on, dumps should still support redaction hooks (API keys, bearer tokens, optional path denylist) so “debug quality” does not equal “secret exfiltration to disk.”

### 9. Clean process lifecycle (target)

Initialize once per workspace; close **all** streams on shutdown; optional rebind for tests without process restart.

## Non-goals (stable)

- Replacing TUI glass-box / transparency UX  
- Becoming metrics TSDB  
- Shipping logs to remote SIEM as a built-in  
- Asserting log lines into the live kernel by default (would couple executive state to disk I/O)  

## Success metrics

| Metric | Target |
|--------|--------|
| Production disk writes with default config | 0 under `.nerd/logs` |
| Time to enable focused debug | edit one JSON field + restart command |
| Reconstruct shard failure from audit alone | possible for instrumented paths |
| Prompt debug with `trace_llm_io` | full package visible for one session |
| Package import graph | no reverse edge into core/mangle from logging |
