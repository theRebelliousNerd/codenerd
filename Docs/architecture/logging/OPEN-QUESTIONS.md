# OPEN QUESTIONS — `internal/logging`

> Last verified: **2026-08-15**

## Q1 — Should audit Mangle facts ever enter the live kernel?

**Context:** Facts are offline-friendly strings; loading them would make the executive depend on telemetry.

**Options:**

1. Keep offline-only (current intent)  
2. Explicit opt-in session command that loads a slice of audit facts with fuel limits  
3. Separate “forensic kernel” process  

**RESOLVED (2026-08-15): option 1, offline-only.** `nerd audit facts` /
`ExportAuditFacts` writes a standalone `.mg` (Decls + deduplicated facts) marked
"Do not load into the live kernel". Telemetry that re-entered the executive would
let the record of what happened change what happens next. Building the exporter
also exposed that the fact strings never parsed as Mangle at all — booleans were
emitted as bare `true`, which the language has no literal for — now `/true` /
`/false`, with a parser-backed test.

## Q2 — Single config file truth: JSON only, YAML only, or dual-write?

**Context:** Package reads `config.json`; other loaders use YAML; schema fields drift.

**RESOLVED (2026-08-15):** JSON only, and both spellings accepted. It was always
the same file — `.nerd/config.json`, the one `config.LoadUserConfig` reads — only
the key differed. `format: "json"|"text"` is canonical (it is what
`config.LoggingConfig` serializes); `json_format: true` stays a legacy alias, and
the two are OR'd so neither loader can disable what the other enabled.
`logging.ApplyConfig(logging.Config{...})` accepts an injected struct from boot to
avoid the second parse; wiring `internal/config` to call it is the remaining
step (tracked in TODO).

## Q3 — Is process-global Once acceptable long-term?

**Context:** Tests reassign Once; multi-workspace CLI may mis-bind.

**RESOLVED (2026-08-15): rebind, not a new entry point.** `Initialize` compares
absolute paths; a different workspace closes every sink, rearms the LLM I/O
`sync.Once`, drops the loaded config and re-initializes, logging the move in the
new boot log. Last-writer-wins matches `--workspace` semantics: an override that
arrives after `main()`'s eager `Initialize(cwd)` still has to win, and under the
old bare `Once` it silently lost — the whole run logged into the wrong tree.
Keying loggers by workspace ID was rejected: nothing in this process wants two
live workspaces, and it would double every lookup for a case that does not exist.

## Q4 — How aggressive should LLM I/O redaction be?

**Context:** Aggressive redaction can hide the bugs JIT debugging needs.

**RESOLVED (2026-08-15): exactly that.** Shape-based redaction is on by default
(`redact.go`); `trace_llm_io_raw: true` restores the full dump for one run and the
boot log says which mode is active. The asymmetry decides it: a key written into a
file that later gets pasted into a bug report is unrecoverable, while a masked
value during JIT debugging costs one config flag. Char/token counts are taken
before redaction so context accounting stays truthful.

## Q5 — Should RequestLogger become immutable builder style?

**Context:** `WithField` mutates; concurrent use is unsafe.

**Open:** Copy-on-write fields map vs document single-goroutine ownership.

## Q6 — Performance category vs OpenTelemetry

**Context:** Timers already emit structured duration fields.

**Open:** Bridge to `internal/observability` spans, or keep file-only forever?

## Q7 — Category proliferation

**Context:** 29 categories; new subsystems keep adding.

**Open:** Cap taxonomy; introduce hierarchical categories (`shard.coder`) vs flat list?

## Q8 — Who is responsible for closing audit/LLM I/O in interactive chat?

**RESOLVED (2026-08-15): nobody has to be.** `CloseAll` now closes all four sinks
(categories, problems, audit, LLM I/O), so the obvious shutdown call is the
complete one and no teardown-hook inventory is required. `CloseAudit` and
`CloseLLMIOLogger` remain for callers that want finer control; every closer is
idempotent, and a `Logger` handle captured before shutdown degrades to a no-op.
