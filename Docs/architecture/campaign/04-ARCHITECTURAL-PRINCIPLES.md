# 04 — Architectural Principles: campaign

> Binding design principles for `internal/campaign`. Violations should be called out in review.  
> Last verified: **2026-07-13**

## P1 — Mangle schedules; Go enacts

The orchestrator **must not** invent a second eligibility engine that contradicts kernel facts. Go may filter practical constraints (backoff `NextRetryAt`, write-set timeouts, concurrency caps) but the source of “what is eligible” is `eligible_task` / related derived predicates.

## P2 — LLM proposes; validation is formal

`Decomposer` and `Replanner` may generate structure freely. Loading into the kernel and `validatePlan` / policy rules are the gate before executive trust. Never treat raw LLM JSON as the live plan without fact load.

## P3 — Event-before-ack durability

Campaign snapshot writes append journal events before and after atomic rename (`orchestrator_journal.go`). Do not “optimize” by writing JSON only in memory for multi-hour runs.

## P4 — Phase-scoped context, not infinite history

ContextPager reserves budgets and compresses completed phases. New features that inject unbounded transcript into every task violate long-horizon viability.

## P5 — Bounded parallelism with write-set exclusivity

Parallel tasks are first-class. Mutating tasks declare `WriteSet` and acquire leases. Prefer failing/scheduling delay over silent concurrent file races.

## P6 — Fail loud at checkpoints

Checkpoint failure asserts `replan_trigger` and keeps the phase open rather than marking success. Do not convert failed verification into soft warnings without replan/user visibility.

## P7 — Optional intelligence never corrupts required path

Intelligence, advisory, edge, tool-pregen may be nil. Core decompose/execute must still work. When present, they enrich; they must not leave half-asserted fact sets without cleanup.

## P8 — JIT-first for new campaign LLM text

New roles/prompts implement `PromptProvider` or prompt atoms — do not grow `prompts.go` without a migration plan. CLI adapter pattern (`CampaignJITProvider`) is the preferred production wire.

## P9 — TaskExecutor over direct ShardManager spawn

Execution path goes through `session.TaskExecutor` (`spawnTask`). ShardManager remains for monitoring. New task types should not reintroduce direct spawn sprawl.

## P10 — Assault is batched and deterministic at plan-time

Assault campaign structure is code-built, not LLM-decomposed, specifically to avoid combinatorial task explosion. Preserve batch files on disk as the source of work units.

## P11 — Risk gates are deterministic

Score and gate enablement use fixed thresholds and toggles. Do not replace with free-form LLM “looks safe” judgments for protected roots.

## P12 — General substrate only

Campaign types and task types remain general agent capabilities. Client-app-specific workflows belong outside this package (apps / showcase), not new `CampaignType` values for one product.

## P13 — No time/cost estimates in architecture roadmaps

Backlogs use dependency order and safety priority only (repo-wide rule).
