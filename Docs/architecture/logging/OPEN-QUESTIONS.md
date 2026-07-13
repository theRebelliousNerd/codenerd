# OPEN QUESTIONS — `internal/logging`

> Last verified: **2026-07-13**

## Q1 — Should audit Mangle facts ever enter the live kernel?

**Context:** Facts are offline-friendly strings; loading them would make the executive depend on telemetry.

**Options:**

1. Keep offline-only (current intent)  
2. Explicit opt-in session command that loads a slice of audit facts with fuel limits  
3. Separate “forensic kernel” process  

**Recommendation:** remain offline-only unless product needs cross-turn logic queries of telemetry.

## Q2 — Single config file truth: JSON only, YAML only, or dual-write?

**Context:** Package reads `config.json`; other loaders use YAML; schema fields drift.

**Open:** Who owns migration? Should `logging.Initialize` accept an injected config struct from boot to avoid dual parse?

## Q3 — Is process-global Once acceptable long-term?

**Context:** Tests reassign Once; multi-workspace CLI may mis-bind.

**Open:** Export `InitializeForWorkspace` that allows rebind if workspace changes, or key loggers by workspace ID?

## Q4 — How aggressive should LLM I/O redaction be?

**Context:** Aggressive redaction can hide the bugs JIT debugging needs.

**Open:** Default redact secrets only; optional `trace_llm_io_raw: true` for full dump?

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

**Context:** Long TUI sessions; PersistentPostRun may not run the same path.

**Open:** Chat teardown hook inventory needed (wiring audit in CLI corpus).
