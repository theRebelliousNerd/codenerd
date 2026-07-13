# OPEN QUESTIONS — config

> Last verified: 2026-07-13  
> Real open design questions (not rhetorical).

## Q1 — Single aggregate timeline

When can YAML `Config` be reduced to a pure adapter or deleted? How many production operators still depend on YAML load + env overrides in `main.go`?

## Q2 — Env vs file precedence for JSON path

Should UserConfig adopt the full `applyEnvOverrides` matrix, a subset, or forbid env for keys already in file (12-factor style)? Context7 already prefers env — should all secrets follow that?

## Q3 — Default concurrent shards 4 vs 12

Which number is operationally correct for the 2026 desktop profile (32t, 128GB)? Should defaults scale with hardware (like WorldConfig workers) or stay fixed?

## Q4 — TraceLLMIO default true

Is defaulting full LLM I/O tracing on in seed defaults acceptable for disk usage and privacy, or should defaults flip to false with opt-in?

## Q5 — Features reinstall on Save

Should `UserConfig.Save` re-call `features.SetActive`, or must every Save be followed by process restart for flag changes?

## Q6 — ClassificationModel defaults ownership

Per-provider fast-tier defaults for classification are described in comments; are they implemented only in perception, or should config expose a `GetClassificationModel()` resolver?

## Q7 — UIConfig orphan

Is `UIConfig` intentionally separate for future theming packages, or accidental leftover from a merge?

## Q8 — Execution timeout philosophy

30s default for tactile commands vs 10m for LLM-heavy execution: should these be **two named budgets** rather than one `DefaultTimeout` field?
