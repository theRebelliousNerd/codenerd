# usage — Open Questions

> Last verified: **2026-07-13**

## Q1 — Single owner: Cortex or chat?

Chat constructs its own tracker in `session.go` while Cortex also owns one from `factory.go`. In interactive mode, which should be canonical?

**Options:**

1. Chat always uses `cortex.UsageTracker` when Cortex is booted.  
2. Chat-only tracker when Cortex is not fully booted; promote later.  
3. Keep dual writers with merge-on-load (complex).

**Impact:** Dual Save races and double Load caches.

## Q2 — Should incomplete metering be a hard quality gate?

If only ZAI tracks, is a green CI enough, or should a campaign/assault assert non-zero usage for LLM phases?

**Trade-off:** Strict gates break offline/mock tests; loose gates leave operator blind.

## Q3 — Events: ring buffer or drop the type?

`UsageEvent` suggests an audit log. Keeping unused types invites deletion mistakes. Prefer:

- Bounded ring (e.g. last N=1000 events) for debugging, or  
- Remove from public docs as “reserved” and leave struct for forward-compat JSON.

## Q4 — Cost accuracy standard

Estimated USD from static tables will be wrong for promo pricing and cached tokens. Is `cost_est_usd` “good enough for ranking models” or must it stay empty until provider billing APIs exist?

## Q5 — Typed vs string context keys for attribution

String keys are easy for shards to set but collide with any other `"session_id"` context value. Should attribution move to a private struct key in `usage` with setters only via `WithShardContext`?

**Breaking:** any external setter of string keys would stop working (only `WithShardContext` and Track readers today).

## Q6 — Budget enforcement layer

If product wants “stop when $X”, does that become:

1. Soft UI warning only, or  
2. Kernel facts + `permitted` deny for further LLM actions?

North star prefers (2) only if executive control is required — and that logic would **not** live inside `internal/usage`.

## Q7 — Embedding and tool_gen operations

Types comment `tool_gen`; tests use `embedding`. Which call sites should own those operation tags (embedding engine vs perception vs tools)?

## Q8 — Multi-process safety

Do we care about two `nerd` processes in one workspace (e.g. CLI + chat)? If yes, need file locking or SQLite. If no, document “single writer” as invariant.

## Resolved / closed for this corpus

| Topic | Resolution |
|-------|------------|
| Is usage pre-implementation? | **No** — live package with tests and wiring |
| Does usage need Mangle Decl? | **No** for current scope |
| Nil tracker panics? | **No** — designed ambient |
