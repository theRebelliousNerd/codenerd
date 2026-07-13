# core — Open Questions

> Last verified: **2026-07-13**  
> Real design questions, not rhetorical. Resolutions should update IMPLEMENTED_SPEC.

## Q1 — Single orchestration authority

Should `internal/session.Executor` be the **only** supported multi-turn loop, with `core/shards.ShardManager` reduced to pure spawn registry?

**Why open:** Package README says ShardManager removed; code still has substantial manager/spawn/queue. Dual paths confuse wiring audits.

## Q2 — Diff-eval production default

Should differential evaluation default **off** until ApplyDelta caveats are closed, regardless of `internal/features` default?

**Why open:** `kernel_eval.go` comments document tension between Task #10 env default and features package default TRUE.

## Q3 — Cortex multi-domain as default

Is hierarchical `CortexKernel` the long-term default for interactive nerd, or a specialized mode for scale experiments?

**Why open:** Feature-flagged per-shard facts; most mental model still single RealKernel.

## Resolved decisions (2026-07-13)

- Permission authorization is exact `permitted(ActionType, Target,
  CanonicalPayload)`. `safe_action/1` is classification only.
- The default Cortex policy shard owns the complete pending-action permission
  envelope, and the router preserves the executive action ID in results.
- Destructive `RouteAction` and interactive preflight deny without a usable
  Dreamer; nil-kernel permission checks deny.

## Q5 — Direct Exec vs RouteAction

Is `VirtualStore.Exec` a permanent peer API for session tools, or a temporary convenience that should eventually funnel through RouteAction envelopes?

**Why open:** Defense-in-depth asymmetry (no Dreamer).

## Q6 — Policy program size strategy

At what point do we need profile-based partial policy loads (e.g., CLI `query` without campaign/coder modules)?

**Why open:** Boot latency and debug dump size grow with every module.

## Q7 — Dreamer completeness vs sandboxing

Should investment go into richer projected_fact models or into OS-level sandbox for exec?

**Why open:** Product threat model may prefer logic projections for speed; hostile shell payloads stress string rules.

## Q8 — Learned rule governance

What is the product UX for user-visible approval of HotLoadRule results (RuleCourt vs silent accept)?

**Why open:** RuleCourt exists; integration density unclear from core alone.

## Q9 — Ownership of safe_action list

Who owns adding `safe_action(/new_tool)` when a modular tool ships in `internal/tools` — tool author, policy owner, or generated from registry?

**Why open:** Drift risk between Go ActionType and Mangle safe_action.

## Q10 — Ephemeral predicate catalog

Is `IsEphemeral` list complete for all session-scoped predicates that must not boot-fire?

**Why open:** New predicates added across packages may miss the filter.

## Q11 — Dreamer cache identity and invalidation

Should cache identity include canonical payload plus a kernel/policy mutation epoch,
instead of action type and target with manual invalidation?

**Why open:** Bounded storage is proven; freshness across every mutation path is not.

## Q12 — Mid-evaluation cancellation

Should kernel evaluation itself accept context cancellation, rather than checking
only around the Dreamer evaluation boundary?

**Why open:** pre/post checks bound many cases, but a running Mangle evaluation is
not presently interrupted through the same context.
