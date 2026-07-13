# 04 — Architectural Principles: config

> Last verified: 2026-07-13  
> These principles are **binding** for changes under `internal/config/`.

## P1 — Config is boss

If the user sets `provider`, only that provider’s credentials apply. Never silently fall back to another key. Fail loud with empty key.

**Evidence:** `UserConfig.GetActiveProvider`.

## P2 — Workspace root is go.mod-first

Project boundary is the Go module. Nested or home `.nerd` directories must not hijack state.

**Evidence:** `FindWorkspaceRoot`.

## P3 — Zero means default (except tracked booleans)

Numeric/string zeros are filled in `Get*` helpers. Booleans that must honor explicit `false` use `UnmarshalJSON` + `*Set` flags (`JITConfig`, `ReflectionConfig`) or pointer fields (`APISchedulerPolicy`).

## P4 — Engines are subprocess LLM APIs, not agents

Claude CLI / Codex CLI / xAI OAuth blocks document MaxTurns=1, sandbox read-only, shell tool off. codeNERD’s tools and tactile layer own side effects.

## P5 — Subscription engines are polite by default

Spacing + adaptive concurrency on for subscription engines; API engines stay aggressive unless configured.

**Evidence:** `subscriptionEngine`, `GetEffectiveAPISchedulerPolicy`.

## P6 — Concurrency is min(core, engine)

Engine-specific `MaxConcurrentCalls` can only **lower** the core ceiling, never raise it.

**Evidence:** `GetEffectiveMaxConcurrentAPICalls`.

## P7 — Do not invent embedding/model IDs at call sites

Embedding and image models resolve through helpers (`GetEmbeddingConfig`, `GetImageLLMConfig`) so one config.json change is authoritative.

## P8 — Feature flags cross the package boundary via SetActive

`LoadUserConfig` installs `features.FeaturesConfig` process-wide. Leaf packages must not import `internal/config` solely for flags.

## P9 — Shortest timeout wins; keep tiers aligned

`LLMTimeouts` documents that Go contexts can undercut HTTP clients. Presets keep HTTP, slot, and per-call aligned.

## P10 — Config does not decide actions

No derivation of `user_intent`, `next_action`, or `permitted`. Supply data; kernel and VirtualStore execute.

## P11 — Wiring audit before deleting “dead” config

YAML `Config` and fields that look unused may be referenced from `cmd/nerd/main.go`, campaigns, or tests. Grep reverse imports before removal.

## P12 — Prefer additive Get* over breaking JSON shape

New knobs should be optional fields with defaults so existing `.nerd/config.json` files keep working.
