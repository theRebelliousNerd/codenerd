# internal/logging - Categorized File-Based Logging

This package provides config-driven categorized file-based logging for codeNERD. Logs are written to .nerd/logs/ with separate files per category.

**Related Packages:**
- [internal/config](../config/CLAUDE.md) - LoggingConfig controlling debug_mode
- [internal/shards/system](../shards/system/CLAUDE.md) - Log-analyzer skill consuming logs

## Architecture

The logging package provides:
- **Categorized Logging**: 22 categories for different subsystems
- **Config-Driven**: Controlled by debug_mode in .nerd/config.json
- **Structured JSON**: Optional format for Mangle parsing
- **Audit Trail**: Mangle-queryable event predicates

## File Index

| File | Description |
|------|-------------|
| `logger.go` | Core categorized logging with file output and config-driven control. Exports `Category` constants (22 categories: boot, session, kernel, api, perception, articulation, shards, etc.), `Logger`, `StructuredLogEntry` for JSON format, and category-specific helper functions. |
| `audit.go` | Audit logging outputting Mangle-queryable fact predicates. Exports `AuditEventType` constants (shard lifecycle, action routing, kernel ops, LLM calls, file ops, session events), `AuditEvent`, `AuditEntry`, and `ToFacts()` for conversion. |
| `logger_test.go` | Unit tests for logger initialization and category filtering. Tests config loading and log file creation. |

## Key Types

### Category
```go
const (
    CategoryBoot         Category = "boot"
    CategorySession      Category = "session"
    CategoryKernel       Category = "kernel"
    CategoryAPI          Category = "api"
    CategoryPerception   Category = "perception"
    CategoryArticulation Category = "articulation"
    CategoryRouting      Category = "routing"
    CategoryShards       Category = "shards"
    CategoryCoder        Category = "coder"
    CategoryTester       Category = "tester"
    CategoryReviewer     Category = "reviewer"
    CategoryResearcher   Category = "researcher"
    CategoryDream        Category = "dream"
    CategoryAutopoiesis  Category = "autopoiesis"
    CategoryCampaign     Category = "campaign"
    CategoryContext      Category = "context"
    CategoryWorld        Category = "world"
    CategoryEmbedding    Category = "embedding"
    CategoryStore        Category = "store"
    CategoryBrowser      Category = "browser"
    CategoryTactile      Category = "tactile"
    CategoryJIT          Category = "jit"
    CategoryBuild        Category = "build"
)
```

### AuditEventType (Maps to Mangle Predicates)
```go
const (
    AuditShardSpawn    AuditEventType = "shard_spawn"    // -> shard_lifecycle/5
    AuditActionRoute   AuditEventType = "action_route"   // -> action_routed/5
    AuditKernelAssert  AuditEventType = "kernel_assert"  // -> kernel_op/5
    AuditLLMRequest    AuditEventType = "llm_request"    // -> llm_call/6
    AuditFileRead      AuditEventType = "file_read"      // -> file_op/5
    AuditSessionStart  AuditEventType = "session_start"  // -> session_event/4
    AuditIntentParsed  AuditEventType = "intent_parsed"  // -> intent_parsed/5
)
```

## Usage

Logging is controlled by `.nerd/config.json`:
```json
{
  "logging": {
    "debug_mode": true,
    "categories": { "kernel": true, "shards": true },
    "level": "debug",
    "json_format": false
  }
}
```

## Dependencies

- Standard library only

## Testing

```bash
go test ./internal/logging/...
```

---

**Remember: Push to GitHub regularly!**
