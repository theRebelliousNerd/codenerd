# TODO — features

> Last verified against codebase: **2026-07-13**  
> Docs-only backlog (this corpus does not implement code).

## P1 — Wiring

- [x] Wire `cmd/tools/verify_taxonomy` to `features.IsTaxonomyFastEnabled()` (and ensure SetActive/env path consistent with resolveBool, not only `== "1"`).
- [x] Align comments: remove “hard short-circuit” language for PerShardFacts where accessor is normal resolveBool; update `kernel_eval.go` DiffEval default claim; fix SystemShards field env comment.

## P2 — Operability

- [x] Improve `Summary()` to print resolved booleans (dereference `*bool` or log `Is*` snapshot) so Boot logs are human-readable.
- [x] Optional CLI: `nerd features` or status subsection listing resolved flags (env vs active vs default source).
- [ ] Optional chat slash `/features` mirroring Summary.

## P3 — Consistency

- [ ] Env prefix migration plan (`NERD_*` → `CODENERD_*` dual-read then deprecate).
- [ ] Document JSON schema snippet for `features` block in user-facing config docs (outside this package if preferred).

## P4 — Testing

- [x] Table-driven precedence matrix for all eight boolean accessors.
- [x] Summary format test once Summary is fixed.
- [x] Optional `-race` concurrent SetActive stress.

## P5 — Product / Track D

- [ ] When ShardFactRouter auto-wiring is production-ready, flip FullyEnabled PerShardFacts (or document continued opt-in) and expand integration tests.

## Done (do not re-open without evidence)

- Leaf package extraction for cycle break  
- env > active > default precedence  
- SetActive snapshot copy  
- LoadUserConfig install + Boot log  
- Conservative DefaultFeaturesConfig  
- FullyEnabled seed with PerShardFacts false  
- Config round-trip external tests  
