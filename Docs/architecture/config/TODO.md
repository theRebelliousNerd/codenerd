# TODO — config package backlog

> Last verified: 2026-07-13  
> Docs-only corpus rebuild; items below are **code/process** work, not done by this docs pass.

## P0

- [ ] Unify dual defaults between `DefaultConfig` / `GetCoreLimits` / `GetExecution` (shards concurrency, timeouts, context max tokens) **or** document forced “UserConfig only” for all boot paths and gate YAML.
- [ ] Audit every `LoadUserConfig` soft-ignore (`_`) and ensure malformed JSON is logged.

## P1

- [ ] Env override policy for UserConfig (explicit list: which keys override JSON, document in README).
- [ ] `ValidateUserConfig()` covering engine, provider, core limit floors, context percent sums.
- [ ] Deprecation plan for YAML `Config` (`cmd/nerd/main.go` migration).
- [ ] Tests for `GetEffectiveJITConfig` clamp math.

## P2

- [ ] Optional `config_version` field + migration hooks (coordinate with `internal/ux/migration.go`).
- [ ] Fold `UIConfig` into UserConfig if split-pane persistence is required.
- [ ] Explicit mid-session reload API that re-runs `features.SetActive`.
- [ ] Property tests: ValidProviders × Validate.

## Docs (done this rebuild)

- [x] Full architecture corpus under `Docs/architecture/config/` (2026-07-13)
- [x] Flagship IMPLEMENTED_SPEC with engines, limits, load paths
