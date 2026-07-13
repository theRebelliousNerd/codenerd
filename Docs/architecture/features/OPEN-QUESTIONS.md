# OPEN QUESTIONS — features

> Last verified against codebase: **2026-07-13**

## Q1 — Should PerShardFacts ever hard-lock to false?

**Context:** Comments/tests sometimes imply a hard short-circuit; implementation is normal `resolveBool` with FullyEnabled seed false.  
**Options:** (a) Keep soft seed-only OFF; (b) force `return false` until Track D ships; (c) enable FullyEnabled when ready.  
**Impact:** Experimentation via env vs accidental production enable.

## Q2 — Should resolved flags be asserted into Mangle?

**Context:** Logic executive cannot currently reason about `feature_diff_eval(true)`.  
**Options:** (a) Keep Go-only (current); (b) assert facts at boot for transparency/policy.  
**Tradeoff:** leaf purity vs logic visibility; asserting would be config/boot responsibility, not features importing mangle.

## Q3 — Env prefix unification timeline?

**Context:** Mix of `CODENERD_*` and `NERD_*`.  
**Question:** Dual-read period length? Deprecate which names first?  
**Risk:** Breaking CI scripts and operator muscle memory.

## Q4 — Is TaxonomyFast still a features concern?

**Context:** Tool ignores the accessor.  
**Options:** (a) Wire tool to features; (b) remove TaxonomyFast from FeaturesConfig; (c) leave dual paths.  
**Prefer:** (a) wiring audit before deletion.

## Q5 — Dynamic reload?

**Context:** Only SetActive on load.  
**Question:** Should config Save/reload hot-path re-install features without process restart?  
**Risk:** Mid-session DiffEval flip during evaluate.

## Q6 — Summary machine-parseable?

**Context:** Boot string is free-form; bools may print as pointers.  
**Question:** Structured log fields (`logging` key/value) vs keep single line?

## Q7 — How many flags is too many?

**Context:** Package charter says narrow marathon-era set.  
**Question:** Threshold before splitting categories or using a generic map?  
**Bias:** Prefer adding fields explicitly over map[string]bool to keep accessors typed and greppable.

## Q8 — Relationship to limits / engines config?

**Context:** Some “features” are really performance knobs (scan workers).  
**Question:** Keep under features for leaf access, or move to world-local config with features only for true bool gates?  
**Today:** Numerics stay here so world does not import full config.
