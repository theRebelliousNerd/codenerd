# Progress — types architecture corpus

| Date | Action |
|------|--------|
| 2026-07-13 | **Full corpus rebuild** per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Replaced thin auto-inventory stubs with code-grounded docs matching CLI depth bar. Source: `internal/types/` (5 non-test `.go`, 4 tests, 0 `.mg`). |
| 2026-07-13 | Flagship `IMPLEMENTED_SPEC.md` + numbered 00–12 set + README, TODO, OPEN-QUESTIONS. Old domain-model / cross-system stub names overwritten with redirects where they conflicted with the new map. |
| 2026-08-15 | **Backlog pass (code + docs).** Cleared P0–P2 and the P3 examples item. New code: `ctxkeys.go` (typed context keys, dual-writing the legacy string keys), `TransactorOf` + a panic message that names the concrete type, `KernelFact = Fact` (step 1 of the `KernelInterface` deprecation path), `typestest.MockKernel`. New tests: two repo-wide ratchets (`fact_conventions_guard_test.go`, `kernel_transactor_guard_test.go`), container `ToAtom` tables, the `Fact.String` float round-trip pin, context-key tests, godoc examples. Q1/Q2/Q4/Q5/Q6 answered; Q8 opened. The P0 sweep's findings against other packages are tabled at the end of `TODO.md`. |

## Verify intent

```powershell
go test ./internal/types/...
go test ./internal/types/ -run 'TestFactConventions|TestKernelTransactor' -v
```

## Scope discipline

- 2026-07-13 pass: **only** wrote under `Docs/architecture/types/`; no code changes.
- 2026-08-15 pass: wrote under `internal/types/` (incl. the new `typestest/` sibling) and
  `Docs/architecture/types/`. No `.mg` files touched; findings against other packages were
  **reported and baselined**, not edited.
