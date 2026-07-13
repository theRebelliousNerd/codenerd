# Progress — types architecture corpus

| Date | Action |
|------|--------|
| 2026-07-13 | **Full corpus rebuild** per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Replaced thin auto-inventory stubs with code-grounded docs matching CLI depth bar. Source: `internal/types/` (5 non-test `.go`, 4 tests, 0 `.mg`). |
| 2026-07-13 | Flagship `IMPLEMENTED_SPEC.md` + numbered 00–12 set + README, TODO, OPEN-QUESTIONS. Old domain-model / cross-system stub names overwritten with redirects where they conflicted with the new map. |

## Verify intent

```powershell
go test ./internal/types/...
```

## Scope discipline

- **Only** wrote under `Docs/architecture/types/`
- No Go/Mangle/code changes
