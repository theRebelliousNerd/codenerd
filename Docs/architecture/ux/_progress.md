# ux — Progress

| Date | What |
|------|------|
| 2026-07-13 | Full architecture corpus **rebuild** per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md` (quality bar: cli-depth narrative, not auto-inventory stubs). |
| 2026-07-13 | Source reviewed: `internal/ux/{doc,preferences,migration,user_state}.go` + all four test files. |
| 2026-07-13 | Reverse deps traced: primary consumer `cmd/nerd/chat` (boot, onboarding, help, tips); lateral prefs writers in `internal/init`. |
| 2026-07-13 | Document set written: README, IMPLEMENTED_SPEC, 00–12 series (new naming), TODO, OPEN-QUESTIONS, this file. |
| 2026-07-13 | Earlier thin stubs (`01-DOMAIN-MODEL`, `02-CURRENT-STATE-UX`, …) may remain as historical names; **rebuild contract files above supersede** them. |

## Verify

```powershell
go test ./internal/ux/...
```

## Status line

`DONE ux` — see agent stdout for `files=N bytes=B`.
