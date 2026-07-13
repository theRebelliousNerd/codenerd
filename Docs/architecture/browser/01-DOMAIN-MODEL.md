# browser — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/browser/` (complete internal coverage)
> **Implementation: `internal/browser/` — 3 non-test .go, 6 tests, 0 .mg**


## Package

`internal/browser/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `DetectionResult` | `internal/browser/honeypot.go:17` |
| `Link` | `internal/browser/honeypot.go:27` |
| `HoneypotDetector` | `internal/browser/honeypot.go:36` |
| `Session` | `internal/browser/session_manager.go:25` |
| `Config` | `internal/browser/session_manager.go:73` |
| `EngineSink` | `internal/browser/session_manager.go:130` |
| `SessionManager` | `internal/browser/session_manager.go:144` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `NewHoneypotDetector` | `internal/browser/honeypot.go:41` |
| `AnalyzePage` | `internal/browser/honeypot.go:46` |
| `IsHoneypot` | `internal/browser/honeypot.go:253` |
| `GetSafeLinks` | `internal/browser/honeypot.go:314` |
| `GetAllLinksWithAnalysis` | `internal/browser/honeypot.go:370` |
| `Allow` | `internal/browser/session_manager.go:56` |
| `DefaultConfig` | `internal/browser/session_manager.go:88` |
| `IsHeadless` | `internal/browser/session_manager.go:101` |
| `GetViewportWidth` | `internal/browser/session_manager.go:106` |
| `GetViewportHeight` | `internal/browser/session_manager.go:114` |
| `NavigationTimeout` | `internal/browser/session_manager.go:122` |
| `AddFacts` | `internal/browser/session_manager.go:139` |
| `NewSessionManager` | `internal/browser/session_manager.go:154` |
| `NewSessionManagerWithSink` | `internal/browser/session_manager.go:167` |
| `Start` | `internal/browser/session_manager.go:176` |
| `ControlURL` | `internal/browser/session_manager.go:275` |
| `IsConnected` | `internal/browser/session_manager.go:282` |
| `Shutdown` | `internal/browser/session_manager.go:289` |
| `List` | `internal/browser/session_manager.go:319` |
| `CreateSession` | `internal/browser/session_manager.go:331` |
| `Attach` | `internal/browser/session_manager.go:396` |
| `Page` | `internal/browser/session_manager.go:434` |
| `UpdateMetadata` | `internal/browser/session_manager.go:445` |
| `GetSession` | `internal/browser/session_manager.go:456` |
| `ReifyReact` | `internal/browser/session_manager.go:467` |
| `ForkSession` | `internal/browser/session_manager.go:629` |
| `Navigate` | `internal/browser/session_manager.go:699` |
| `Click` | `internal/browser/session_manager.go:724` |
| `Type` | `internal/browser/session_manager.go:752` |
| `Screenshot` | `internal/browser/session_manager.go:780` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Browser automation / Rod session management and honeypot surfaces**
