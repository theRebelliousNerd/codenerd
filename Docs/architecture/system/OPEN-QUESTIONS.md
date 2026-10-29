# Open questions: system

> Last verified: 2026-07-13. Verified defects are not disguised as questions;
> they live in [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md).

## Q1 — Should chat share the GetOrBoot cache?

Chat currently uses `BootCortexWithConfig` so it can inject in-memory config and
then add TUI-only transparency, retrieval, preferences, observers, and tools.
Should it register that Cortex under the same canonical identity, or should chat
remain an explicitly separate owner with direct-boot lifecycle semantics?

The decision affects cache sharing, maintenance, duplicate system shards, and
config changes. It does not block fixing the current incomplete cache identity.

## Q2 — What is the public reset contract?

Current Reset functions evict without Close. Should the API expose distinct
`Evict*` and `CloseAndReset*` operations, or make Reset always close? Callers need
an explicit answer for long-lived processes and tests.

## Q3 — Which optional components become required by product mode?

Embeddings and MCP currently degrade with warnings. A retrieval-required or
tool-required mode may need a typed boot capability contract that converts a
selected dependency from soft to hard failure.

## Q4 — Who owns direct-Boot maintenance?

Only cached GetOrBoot instances start maintenance. If chat remains direct boot,
should it start its own schedule, explicitly disable maintenance, or delegate
retention to another owner? Double-ticker prevention is part of the decision.

## Q5 — What is the supported Close concurrency contract?

Normal sequential Close is bounded and idempotent for tested resources. Is
concurrent Close, StartMaintenanceSchedule during Close, or Close during a
system-shard callback supported? The answer determines whether lifecycle fields
need a mutex/state machine.

## Q6 — How should crash dumps be retained?

The package tree contains `debug_program_ERROR.mg`. Future dumps should likely
live under a bounded `.nerd/debug/` location with redaction and retention, but
the operator-access and support contract is not pinned.

## Q7 — Is `LocalStoreTraceAdapter.LoadReasoningTrace` part of the contract?

The method returns nil, nil. Either implement typed trace reconstruction with
tests or narrow the interface/adapter so callers cannot interpret an absent
trace as a successful empty load.
