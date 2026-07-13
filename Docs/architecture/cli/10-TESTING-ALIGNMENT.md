# 10 — Testing Alignment: CLI

> Last verified: 2026-07-13  
> Inventory: **55** `*_test.go` files ≈ **16,755** lines under `cmd/nerd/`.

## 1. Current coverage shape

| Area | Evidence |
|------|----------|
| Root CLI commands | `cli_test.go` — init/scan/spawn/define-agent/direct actions/query smoke |
| UI widgets | `ui/*_test.go` — splitpane, diffview, keyboard, styles, debounce, render cache |
| Chat behaviors | multiple chat tests (activity pulse, glass box stream, delegation routing, etc.) |
| DOM | tests near dom commands if present |

## 2. Gaps (honest)

| Gap | Why it hurts | Suggested tests |
|-----|--------------|-----------------|
| `session_boot` failure injection | Boot is highest severity | Fake config/store errors → tea.Msg errors |
| `process.go` intent matrix | Central path | Table of slash vs NL inputs → expected handler/intent |
| Campaign assault end-to-end | Long horizon | Build tags / short synthetic campaign |
| Auth command integration | External CLIs | Mock runners; no network in unit tests |
| Cobra help completeness | Drift | Snapshot of `rootCmd.Commands()` names |

## 3. Commands

```powershell
go test ./cmd/nerd/...
go test -race ./cmd/nerd/chat/...
go test ./cmd/nerd/ui/...
```

## 4. Quality bar for new CLI code

- Table-driven unit tests for pure helpers.
- At least one test for each new Cobra command’s validation path.
- Chat handlers: prefer testing pure functions extracted from handlers over full tea programs when possible.
- Never delete failing tests to “go green”.
