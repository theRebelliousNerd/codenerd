# 04 — Architectural principles

## Current principles worth preserving

1. **Config is boss for an explicit valid provider.**
   `internal/config/user_config.go#UserConfig.GetActiveProvider` never borrows a
   different provider's key.
2. **Workspace identity is `go.mod` first.**
   `internal/config/user_config.go#FindWorkspaceRoot` walks past nested `.nerd`.
3. **Effective values are resolved at the boundary.** Optional pointer/boolean
   presence is preserved where omission differs from false.
4. **CLI engines are LLM transports, not effect agents.** Read-only sandbox,
   disabled shell/tools and bounded turns must be enforced, not merely defaulted.
5. **Subscription concurrency may narrow, never broaden, the core ceiling.**
6. **Config supplies data, never `user_intent`, `next_action`, or `permitted/3`.**
7. **Dormant-looking fields require reverse wiring audit before deletion.** The
   YAML timeout path is live even though JSON is the broad aggregate.

## Uplift principles

1. **Present-invalid is not absent.** Absence may enter first-run/env policy;
   malformed or contradictory input fails closed.
2. **Persist the whole transaction or nothing.** Merge, validate, sync and atomic
   replace; active state changes only after durable success.
3. **Secrets are referenced or owner-protected.** Logs/receipts never contain raw
   config, keys, tokens, prompts or responses by default.
4. **One snapshot, explicit projections.** Every consumer proves which snapshot
   it accepted; no package reparses and invents precedence independently.
5. **Bounds compose with permission.** An allowlist does not authorize, and
   permission does not override an execution/resource bound.
6. **Defaults are schema behavior.** They are versioned, tested across consumers,
   and carry provenance; zero-value convenience cannot mask hostile input.
7. **Reload is a lifecycle transaction.** All consumers switch together or none
   do; teardown resets workspace-scoped globals.
8. **Compatibility is observable and expiring.** Deprecated fields have a
   migration, warning ID, removal decision and dry-run path.

**REJECTED.** Duplicating validation in perception, system, chat and campaign
would make runtime behavior depend on which constructor happened to run.
