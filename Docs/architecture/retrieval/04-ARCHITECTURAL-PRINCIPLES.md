# retrieval — Architectural Principles

> Last verified: **2026-07-13**  
> Binding for changes under `internal/retrieval/` and its chat/session wires.

## P1 — Retrieval proposes; logic disposes

The package returns data structures and may help callers assert facts. It must **not** decide `next_action`, open write tools, or bypass `permitted(...)`. Kernel remains executive.

## P2 — Cheap before expensive

Order of cost: mentioned paths → keyword sparse scan → import expand → embeddings. Never invert this funnel without an explicit budget flag.

## P3 — Pure package boundary

`internal/retrieval` depends only on stdlib (+ optional SIMD) and `internal/logging`. No imports of kernel, store, perception, or embedding. Integration owns wiring.

## P4 — Structured facts over free text

When connecting to the agent, prefer Mangle predicates (`issue_keyword`, `candidate_file`, …) over stuffing ranked lists into opaque prompt strings.

## P5 — Bounded work

Every search path honors:

- context cancellation
- per-keyword timeout
- result limits / tier budgets
- cache size + TTL

Unbounded walks are defects.

## P6 — Word-boundary honesty

Lexical hits must respect identifier boundaries (`isWordBoundary`). Substring matches inside longer identifiers are noise, not features.

## P7 — Path normalization

All cross-OS comparisons normalize separators (`\` → `/`). Windows drive-letter pitfalls in colon-parsed formats must be documented when that parser is used.

## P8 — Cache safety

Cache entries clone slices on Get/Set. Callers must never observe shared mutable hit arrays.

## P9 — Build-tag parity for scanners

`ScanBuffer` behavior (offsets of non-overlapping matches for the generic path; documented differences if any) must remain test-covered so SIMD builds do not silently change ranking.

## P10 — Wire before delete

Dormant constructors and schema Decl pairs are **integration debt**, not free delete candidates. Audit chat, session, context, and schemas first.

## P11 — Language expanders are pluggable in spirit

T3/T4 language-specific logic should stay isolated functions (today: Python extractors). Prefer adding `extractImportsGo` style helpers over growing one mega-regex.

## P12 — Comment truthfulness

Public comments that claim “ripgrep” or “vector similarity” must match the real backend. Drift is a documentation bug class.

## Anti-principles

- Do not put large natural-language pattern banks in Mangle; keep fuzzy work in retrieval/embeddings.
- Do not load entire files into LLM context without `LoadContent` budgets / higher-level context compression.
- Do not add client-app-specific heuristics into this general package.
