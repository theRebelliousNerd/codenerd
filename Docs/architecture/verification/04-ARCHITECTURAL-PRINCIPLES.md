# verification — Architectural Principles

> Last verified: **2026-07-13**  
> Binding for work *in or on* `internal/verification/` and its chat call site.

## P1 — Quality executive, creative judge

The **loop structure** (retries, success criteria, escalation) is deterministic Go. The LLM may judge quality and suggest shards/correctives; it must not own the control graph.

## P2 — Orthogonal to constitutional safety

Verification answers “was the work real?”  
Mangle/`permitted` answers “is the action allowed?”  
Never conflate the two gates. Do not put quality heuristics into policy.mg as a substitute for this package.

## P3 — Mutations pay; queries do not (by default)

Callers should scope expensive verify+retry to high-stakes categories (today: `intent.Category == "/mutation"`). Expanding verification to all routes requires an explicit product decision.

## P4 — Prefer TaskExecutor; normalize intents

All spawns go through `spawnTask` → `normalizeIntentVerb`. Never pass bare LLM shard names into `TaskExecutor` without coercion. Keep persona maps in sync with chat delegation (or extract a shared package).

## P5 — Structured results over prose

Success is `VerificationResult` fields, not free-form “looks good.” Parsing may fall back to heuristics, but the loop still consumes structure.

## P6 — Enrich rather than blind retry

On failure, subsequent attempts must include **why it failed** and **what to fix** (`enrichTaskWithContext`). A pure re-run without context is a last resort (heuristic same-shard path).

## P7 — Escalate, do not invent success after exhaustion

After max retries, return `ErrMaxRetriesExceeded` with the last result/verification for human UX. Do not flip Success=true solely because retries ran out.

## P8 — Fail modes must be explicit

Fail-open (nil client / LLM transport error) is a deliberate availability choice. Any change must update docs and ideally be configurable. Silent fail-open in “strict” product mode is a principle violation.

## P9 — Specialists before external research

Corrective path prefers local specialist shards before expensive external calls. When re-adding web/docs tools, keep that priority.

## P10 — Persist for learning; eventually close the loop

Every attempt should be storable with session/turn. New features that *consume* history belong in this package or a clearly owned neighbor — do not leave eternal write-only archaeology.

## P11 — No import cycles with chat

Persona mapping and formatting live split (normalize here; formatVerified* in chat). Do not import `cmd/nerd/chat` from this package.

## P12 — Keep the package single-purpose

Do not absorb test runners, browsers, or campaign scoring. Grow by better verification/correctives, not by becoming a second orchestration kernel.
