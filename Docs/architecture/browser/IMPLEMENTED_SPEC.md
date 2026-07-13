# browser — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/browser/` (complete internal coverage)
> **Implementation: `internal/browser/` — 3 non-test .go, 6 tests, 0 .mg**


## 1. Purpose

Browser automation / Rod session management and honeypot surfaces

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/browser/` | Primary implementation |
| `Docs/architecture/browser/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (3 src / 6 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/browser/session_manager.go` | 809 | source |
| `internal/browser/session_manager_dom.go` | 677 | source |
| `internal/browser/honeypot.go` | 412 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `DetectionResult` | `internal/browser/honeypot.go:17` |
| `Link` | `internal/browser/honeypot.go:27` |
| `HoneypotDetector` | `internal/browser/honeypot.go:36` |
| `Session` | `internal/browser/session_manager.go:25` |
| `Config` | `internal/browser/session_manager.go:73` |
| `EngineSink` | `internal/browser/session_manager.go:130` |
| `SessionManager` | `internal/browser/session_manager.go:144` |

### Functions (sampled)

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

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
