# logging — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/logging/` (complete internal coverage)
> **Implementation: `internal/logging/` — 4 non-test .go, 5 tests, 0 .mg**


## Package

`internal/logging/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

This package: **Categorized logging system for debug/diagnostics**
