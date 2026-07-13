# system — Open Questions

> Last verified: **2026-07-13**

## Q1 — Should TUI use GetOrBootCortex?

**Context:** Chat calls `BootCortexWithConfig` with `UserConfigOverride` and unpacks fields. CLI uses cached GetOrBootCortex.

**Tradeoffs:**

- **Unify:** one identity model, one maintenance schedule, simpler mental model.  
- **Keep separate:** TUI can inject in-memory config without writing config.json; avoids sharing with short CLI invocations in same process (rare).

**Status:** Open. Prefer unify with optional override registration API.

## Q2 — Who owns Cortex.Close in long-running CLI?

**Context:** Many Cobra handlers call GetOrBootCortex and may not Close, relying on process exit. Cache intentionally retains instances.

**Question:** Is process-lifetime ownership the product rule? If yes, document; if no, define command-scoped vs process-scoped Cortex.

## Q3 — Should Reset close instances?

**Context:** `ResetGlobalCortex` drops pointers without Close.

**Question:** Safer API: `ResetAndClose*` that joins Close errors? Or force callers to Close first?

## Q4 — Maintenance on direct Boot*?

**Context:** Only GetOrBoot starts maintenance.

**Question:** Should BootCortexWithConfig optionally start maintenance for TUI long sessions?

## Q5 — Identity dimensions complete?

**Context:** Key uses workspace, provider, apiKey, model.

**Question:** Should engine (xai-oauth vs zai), worker LLM config, or `DisableSystemShards` participate? Changing engine without model/provider/key change might still hit stale Cortex.

## Q6 — Hard-fail embedding?

**Context:** Soft-fail today.

**Question:** For “holographic retrieval” product modes that require vectors, should boot fail closed when embedding health fails?

## Q7 — Where do crash dumps live?

**Context:** `debug_program_ERROR.mg` appeared under `internal/system/`.

**Question:** Should dumps always write under `.nerd/` workspace debug dir instead of package source tree?

## Q8 — session VS adapter permanence?

**Context:** Comments say fallback “for now.”

**Question:** Is there a blocked dependency that prevents full VS routing, or is this pure debt?
