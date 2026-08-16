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

**RESOLVED (2026-08-16): keep file-only; `EventSink` is the answer to machine-readable export.** The question as posed had no referent. `internal/observability` contains no OpenTelemetry spans to bridge to — it is `flight_recorder.go` (a Go `runtime/trace.FlightRecorder` singleton with a memory watchdog) and `runtime_metrics.go`. Go execution traces and OTel spans are different artifacts with different consumers, so "Bridge to `internal/observability` spans" named a thing that does not exist, which is why the item never had an actionable shape.

`go.mod` carries `go.opentelemetry.io/otel`, `otel/trace` and `otel/metric` only as `// indirect`, pulled in transitively, and there are zero `go.opentelemetry.io` imports anywhere in the repository. `otel/sdk` — the module that actually implements a `TracerProvider` — is absent entirely. A bridge is therefore not one file but a new direct dependency, an exporter choice, and a provider lifecycle (shutdown and flush) to own.

The thing a bridge would have bought — machine-readable export of the event stream — already ships and is wired. `internal/transparency/event_bus.go:74` calls `attachEnvNDJSONSink(b)` inside `NewGlassBoxEventBus`, so every bus gets the env-configured sink; `NDJSONEventEnvVar` names the file; `internal/transparency/ndjson_sink.go` writes one JSON object per event. It is covered by `TestNewGlassBoxEventBus_WhenNDJSONEnvSet_ShouldAttachSink`, `TestNDJSONSink_ShouldWriteOneJSONObjectPerEvent` and `TestNDJSONSink_WhenTurnFiltered_ShouldExportOnlyThatTurn` in `internal/transparency/observability_test.go`.

The reversal is cheap. `EventSink` (`internal/transparency/event_bus.go:58`) is a one-method interface — `Write(event GlassBoxEvent)` — and a future OTel sink would be about thirty lines registered through `AddSink`, not a refactor. The decision is therefore low-regret in both directions.

The reason not to build it now is already argued in the code at `internal/transparency/event_bus.go:53-57`: nothing in this repository configures an OTel `TracerProvider`, so a bridge "would map categories onto no-op spans and produce exactly the 'looks wired, does nothing' shape this package is trying to remove." Building it would violate the stated purpose of the very package it lives in.

This resolution should be revisited rather than treated as permanent. An OTel bridge becomes worth building the day something in the repository configures a real `TracerProvider` and `otel/sdk` becomes a direct dependency — at which point it is one `EventSink` implementation.

## Q7 — Category proliferation

**PARTIALLY RESOLVED (2026-08-16): the cap half is done; the hierarchy half remains open.** The count in the original question was wrong: there are 30 categories, not 29. The cap half is DONE. `internal/logging/category_inventory_test.go` is the cap.

It declares `categoryInventory`, a map from each Category's string value to the subsystem that owns it, one row per category. Adding a category means adding a row — which is the point: the taxonomy now grows by decision rather than by drift.

`TestCategoryInventory_WhenCategoryAdded_ShouldBeDeclared` discovers the categories from the SOURCE rather than from a hand-maintained duplicate: it parses `internal/logging/logger.go` with `go/parser` and `go/ast` and collects every constant declared with type `Category`. It then asserts in BOTH directions — a new constant with no inventory row fails, and a stale row whose constant was deleted also fails.

The two-direction check is what keeps the guard honest. If AST discovery ever silently returned nothing, every row would read as stale and the test would fail, rather than passing vacuously.

`TestCategoryInventory_ShouldNotExceedTheCap` makes the ceiling numeric: `const categoryCap = 30`, the exact count at the time of writing, so the guard starts satisfied and bites on the 31st. Raising the cap is permitted but must be a deliberate edit with a reason in the commit message; the alternative is folding the new subsystem into an existing category.

This is the inventory-plus-guard idiom the repo already uses for the same purpose in `internal/logging/safety_callsite_audit_test.go`, `internal/build/go_invocation_inventory_test.go` and `internal/sqlpragmas/open_site_audit_test.go`.

**Open:** Whether the taxonomy should become hierarchical (`shard.coder`) instead of a flat list. This is a redesign of the taxonomy rather than a guard on its size, and it was deliberately left untouched. It would have to decide how a hierarchical name maps onto the per-category log file set and onto the `categories` map in the logging config, and whether enabling a parent implies enabling its children. The cap is not a substitute for this answer — it bounds growth, it does not organize it.

## Q8 — Who is responsible for closing audit/LLM I/O in interactive chat?

**RESOLVED (2026-08-15): nobody has to be.** `CloseAll` now closes all four sinks
(categories, problems, audit, LLM I/O), so the obvious shutdown call is the
complete one and no teardown-hook inventory is required. `CloseAudit` and
`CloseLLMIOLogger` remain for callers that want finer control; every closer is
idempotent, and a `Logger` handle captured before shutdown degrades to a no-op.
