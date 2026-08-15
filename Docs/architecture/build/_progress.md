# Progress — `Docs/architecture/build`

| Date | Change |
|------|--------|
| 2026-07-13 | **Full corpus rebuild** to cli-quality bar per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Replaced thin auto-inventory stubs with narrative docs grounded in `internal/build/env.go` (+ tests) and reverse deps in `internal/autopoiesis`. |
| 2026-07-13 | Prior thin corpus existed (tier-2 auto gen); superseded by this rewrite. |
| 2026-08-15 | **Backlog burn-down.** Closed P0 comment honesty, P1 adoption inventory (now `go_invocation_inventory_test.go`), all four P2 API-hygiene items (`GetBuildEnvForTest` specialization, `AppendGoFlags`, `BuildConfig` alias, `setEnvKey` normalization), all four P3 hardening items (GOCACHE warning, `SummarizeEnv`, toolchain integration tests, secret redaction), and the P4 doc refresh. Added `DetectionRootFor` / `GetBuildEnvForModule` for the monorepo detection-root split. Still open: threading real `*config.UserConfig` through autopoiesis and three session call sites. |

## Rebuild checklist

- [x] README.md with full doc map  
- [x] IMPLEMENTED_SPEC.md (flagship narrative)  
- [x] 00–12 series (alignment through failure modes)  
- [x] TODO.md, OPEN-QUESTIONS.md, _progress.md  
- [x] Real paths only; no pre-impl 0% claims  
- [x] Honest adoption gaps (autopoiesis-only, nil config)  
- [x] Docs only under `Docs/architecture/build/`  

## Package facts at rebuild

- Production: `internal/build/env.go` (~312 lines)  
- Tests: `env_test.go`, `env_gaps_test.go`  
- Mangle: none  
- Live importers: `tool_compiler.go`, `thunderdome.go` under `internal/autopoiesis`

## Package facts at 2026-08-15

- Production: `internal/build/env.go` (~611 lines)  
- Tests: `env_test.go`, `env_gaps_test.go`, `env_features_test.go`, `go_invocation_inventory_test.go` (~1500 lines)  
- Mangle: none  
- Live importers: `internal/autopoiesis` (tool_compiler.go, thunderdome.go),
  `internal/session` (build_verify.go, test_verify.go, coverage_profile.go, lsp_diagnostics.go),
  `internal/core` (virtual_store_actions.go) — enforced by test, not transcribed  
