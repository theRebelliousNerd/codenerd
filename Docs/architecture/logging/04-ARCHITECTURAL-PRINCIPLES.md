# 04 — Architectural Principles (`internal/logging`)

> Last verified: **2026-07-13**  
> These principles are **binding** for changes to `internal/logging/`.

## P1 — Side-channel only

Logging **never** decides actions. No `next_action`, no VirtualStore routes, no `permitted` evaluation. If a feature needs executive force, it belongs in Mangle policy or kernel code.

## P2 — Production silence by default

If `debug_mode` is false or config is missing, **no** log directory creation and **no** audit/LLM files. Prefer silent no-ops over stderr spam (except config/open failures).

## P3 — Fail open for the product, fail soft for telemetry

Logging failures must not crash the agent. Pattern already used: open errors → stderr warn + no-op logger; audit disabled → return.

## P4 — Category isolation

New subsystems get a **typed `Category` constant** (and ideally convenience wrappers) rather than stuffing messages into `boot` or free-form files. Prefer existing categories over inventing parallel log paths.

## P5 — Avoid circular imports via mirror config

This package **must not** import `internal/config` / `internal/core`. Config is mirrored locally. If schema must be shared, introduce a tiny neutral types package — do not pull the world into logging.

## P6 — Structured when it matters

Prefer `StructuredLog` / audit events for machine analysis; free-text `Info` is fine for human grepping. When `json_format` is on, do not invent a third serialization.

## P7 — Mangle facts are strings, not authority

`generateMangleFact` is for offline queryability. Do not silently assert into the kernel from hot log paths. If product needs live facts, use an explicit loader with fuel budgets.

## P8 — Opt-in for high-sensitivity dumps

Full LLM I/O is gated by `trace_llm_io`. Never enable it as a side effect of `debug_mode` alone. Treat dumps as secret-bearing.

## P9 — Cheap when disabled, bounded when enabled

Disabled path: nil logger pointer checks only. Enabled path: sampling for non-slow performance; avoid logging full blobs on every API tick unless LLM I/O is explicitly on.

## P10 — Concurrency-safe shared writers

All global maps and files go through existing mutexes. Do not add lock-free shared mutable state without tests under `-race`.

## P11 — Idempotent process init

`Initialize` remains Once-guarded for production. Tests may reset unexported state; production code must not re-enter init for a second workspace without a deliberate redesign.

## P12 — Convenience must stay thin

`logger_convenience.go` is only `Get(cat).Level(...)`. Do not embed business logic, formatting policies, or filtering beyond what `Logger` already does.
