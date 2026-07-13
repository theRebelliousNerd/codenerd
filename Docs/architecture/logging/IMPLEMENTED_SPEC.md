# logging — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/logging/` (complete internal coverage)
> **Implementation: `internal/logging/` — 4 non-test .go, 5 tests, 0 .mg**


## 1. Purpose

Categorized logging system for debug/diagnostics

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/logging/` | Primary implementation |
| `Docs/architecture/logging/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (4 src / 5 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/logging/logger.go` | 680 | source |
| `internal/logging/audit.go` | 617 | source |
| `internal/logging/logger_convenience.go` | 530 | source |
| `internal/logging/llm_io_logger.go` | 206 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `AuditEventType` | `internal/logging/audit.go:21` |
| `AuditEvent` | `internal/logging/audit.go:104` |
| `AuditLogger` | `internal/logging/audit.go:132` |
| `LLMMessage` | `internal/logging/llm_io_logger.go:38` |
| `Category` | `internal/logging/logger.go:18` |
| `StructuredLogEntry` | `internal/logging/logger.go:81` |
| `Logger` | `internal/logging/logger.go:93` |
| `ContextLogger` | `internal/logging/logger.go:424` |
| `RequestLogger` | `internal/logging/logger.go:479` |
| `Timer` | `internal/logging/logger.go:541` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `InitAudit` | `internal/logging/audit.go:139` |
| `CloseAudit` | `internal/logging/audit.go:168` |
| `Audit` | `internal/logging/audit.go:179` |
| `AuditWithSession` | `internal/logging/audit.go:187` |
| `AuditWithShard` | `internal/logging/audit.go:192` |
| `AuditWithContext` | `internal/logging/audit.go:197` |
| `Log` | `internal/logging/audit.go:210` |
| `ShardSpawn` | `internal/logging/audit.go:358` |
| `ShardExecute` | `internal/logging/audit.go:369` |
| `ShardComplete` | `internal/logging/audit.go:380` |
| `ActionRoute` | `internal/logging/audit.go:393` |
| `ActionComplete` | `internal/logging/audit.go:404` |
| `KernelAssert` | `internal/logging/audit.go:417` |
| `KernelQuery` | `internal/logging/audit.go:428` |
| `LLMCall` | `internal/logging/audit.go:440` |
| `FileOp` | `internal/logging/audit.go:453` |
| `IntentParsed` | `internal/logging/audit.go:465` |
| `SafetyCheck` | `internal/logging/audit.go:480` |
| `PerfMetric` | `internal/logging/audit.go:495` |
| `Error` | `internal/logging/audit.go:517` |
| `SessionStart` | `internal/logging/audit.go:536` |
| `SessionEnd` | `internal/logging/audit.go:546` |
| `TurnStart` | `internal/logging/audit.go:558` |
| `TurnEnd` | `internal/logging/audit.go:569` |
| `ToolExec` | `internal/logging/audit.go:581` |
| `CampaignEvent` | `internal/logging/audit.go:598` |
| `LearningEvent` | `internal/logging/audit.go:609` |
| `IsLLMIOTracingEnabled` | `internal/logging/llm_io_logger.go:77` |
| `LogLLMRequest` | `internal/logging/llm_io_logger.go:89` |
| `LogLLMResponse` | `internal/logging/llm_io_logger.go:156` |

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
