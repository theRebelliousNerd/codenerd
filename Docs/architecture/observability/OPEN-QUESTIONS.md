# OPEN QUESTIONS — observability

> Last verified against codebase: 2026-07-13  
> Package: `internal/observability/`

## Q1 — Should recovered chat panics dump the ring?

**Context:** Main’s defer only sees panics that escape `rootCmd.Execute` on that goroutine. Chat often recovers and continues.

**Options:**

1. Keep current (only hard process-fatal panics dump).  
2. Call `DumpFlightRecord` from chat recover (may spam dumps).  
3. Dump only when recover is classified as “severe” or when env `NERD_FLIGHTREC_DUMP_ON_RECOVER=1`.

**Status:** Unresolved product decision.

## Q2 — Is `/diag flightrec` still planned?

**Context:** `internal/features/features.go` comment references dump “on panic and on /diag flightrec”, but no handler exists.

**Options:** Implement slash + Cobra, or remove the comment.

**Status:** Wiring gap; intent unclear.

## Q3 — Should ring size be config-driven?

**Context:** Production hardcodes 64 MiB / 30 s in `main.go`. Constrained hosts may need smaller rings; long hang debug may need longer MinAge.

**Status:** Open; low urgency while default-on remains cheap enough.

## Q4 — Effective workspace for dumps

**Context:** Interactive `--workspace` chdir happens after main captures `ws`.

**Question:** Should dump always re-resolve workspace from Cobra flags / chat config?

**Status:** Known edge case; needs fix design.

## Q5 — Trace retention policy ownership

**Context:** No rotation. Campaigns and long-lived workspaces may accumulate traces.

**Question:** Own retention in this package, in `nerd init` cleanup, or leave to operators?

**Status:** Open.

## Q6 — Rename collision with prompt “Flight Recorder”?

**Context:** Prompt compiler uses “Flight Recorder” metaphor for manifests.

**Question:** Rename one side in user-facing docs only, or tolerate dual use?

**Status:** Documentation discipline for now; no code rename planned.

## Q7 — Mid-session metrics sampling?

**Context:** Only boot snapshot exists. Status commands might want a second sample.

**Question:** Add `LogRuntimeMetrics(reason string)` or keep one-shot forever?

**Status:** Open; only add with a concrete operator request.

## Q8 — Should observability ever import features for self-gating?

**Context:** Principle P8 keeps gating in main. Self-gating would allow other hosts to Start safely without forgetting the check — but breaks leaf purity if features grows deps.

**Status:** Prefer keep gating external unless a second binary appears.
