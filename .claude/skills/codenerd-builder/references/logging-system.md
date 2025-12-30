# codeNERD Logging System

Config-driven categorized logging for debugging and diagnostics. Zero overhead in production mode.

## Architecture Overview

```text
┌─────────────────────────────────────────────────────────────────┐
│                    .nerd/config.json                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ "logging": {                                             │    │
│  │   "debug_mode": true,  ← Master toggle                   │    │
│  │   "level": "info",     ← Min level filter                │    │
│  │   "categories": {...}  ← Per-category toggles            │    │
│  │ }                                                        │    │
│  └─────────────────────────────────────────────────────────┘    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│              internal/logging/logger.go                         │
│  ┌───────────────────┐  ┌───────────────────┐                   │
│  │ Initialize(ws)    │  │ Get(category)     │                   │
│  │ - Load config     │  │ - Return logger   │                   │
│  │ - Create logs dir │  │ - No-op if off    │                   │
│  │ - Setup loggers   │  │                   │                   │
│  └───────────────────┘  └───────────────────┘                   │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ Convenience Functions: Kernel(), Shards(), Coder(), etc.  │  │
│  └───────────────────────────────────────────────────────────┘  │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    .nerd/logs/                                  │
│  ├── 2025-12-08_kernel.log                                      │
│  ├── 2025-12-08_shards.log                                      │
│  ├── 2025-12-08_perception.log                                  │
│  └── ...                                                        │
└─────────────────────────────────────────────────────────────────┘
```

## Configuration Reference

### Full Configuration Schema

```json
{
  "logging": {
    "level": "info",
    "format": "text",
    "file": "codenerd.log",
    "debug_mode": true,
    "categories": {
      "boot": true,
      "session": true,
      "kernel": true,
      "api": true,
      "perception": true,
      "articulation": true,
      "routing": true,
      "tools": true,
      "virtual_store": true,
      "shards": true,
      "coder": true,
      "tester": true,
      "reviewer": true,
      "researcher": true,
      "system_shards": true,
      "dream": true,
      "autopoiesis": true,
      "campaign": true,
      "context": true,
      "world": true,
      "embedding": true,
      "store": true
    }
  }
}
```

### Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `debug_mode` | bool | `false` | Master toggle. When `false`, ALL logging is disabled (production mode) |
| `level` | string | `"info"` | Minimum log level: `debug`, `info`, `warn`, `error` |
| `format` | string | `"text"` | Output format (currently only `text` supported) |
| `file` | string | `"codenerd.log"` | Legacy single-file option (unused when debug_mode enabled) |
| `categories` | map | `null` | Per-category toggles. Missing = enabled in debug mode |

### Production vs Debug Mode

| Mode | `debug_mode` | Behavior |
|------|--------------|----------|
| **Production** | `false` | Zero logging, zero overhead. No files created. |
| **Debug** | `true` | Full logging per category settings. Files in `.nerd/logs/`. |

## The 22 Log Categories

### Core System

| Category | Constant | Purpose | Key Events |
|----------|----------|---------|------------|
| `boot` | `CategoryBoot` | System startup | Config loading, initialization, feature flags |
| `session` | `CategorySession` | Session lifecycle | New session, turn processing, persistence |
| `kernel` | `CategoryKernel` | Mangle engine | Fact assertion, rule derivation, queries |
| `api` | `CategoryAPI` | LLM API calls | Requests, responses, token counts, latency |

### Transduction Layer

| Category | Constant | Purpose | Key Events |
|----------|----------|---------|------------|
| `perception` | `CategoryPerception` | NL → Atoms | Intent extraction, atom generation, validation |
| `articulation` | `CategoryArticulation` | Atoms → NL | Response generation, Piggyback protocol |

### Execution Layer

| Category | Constant | Purpose | Key Events |
|----------|----------|---------|------------|
| `routing` | `CategoryRouting` | Action dispatch | Tool selection, delegation decisions |
| `tools` | `CategoryTools` | Tool execution | Invocations, results, errors |
| `virtual_store` | `CategoryVirtualStore` | FFI layer | External API calls, fact loading |

### Shard System

| Category | Constant | Purpose | Key Events |
|----------|----------|---------|------------|
| `shards` | `CategoryShards` | Shard manager | Spawn, execute, destroy lifecycle |
| `coder` | `CategoryCoder` | CoderShard | Code generation, edits, Ouroboros |
| `tester` | `CategoryTester` | TesterShard | Test execution, coverage |
| `reviewer` | `CategoryReviewer` | ReviewerShard | Code review, security |
| `researcher` | `CategoryResearcher` | ResearcherShard | Knowledge gathering |
| `system_shards` | `CategorySystemShards` | System shards | Legislator, Constitution, Executive |

### Advanced Systems

| Category | Constant | Purpose | Key Events |
|----------|----------|---------|------------|
| `dream` | `CategoryDream` | Dream state | What-if simulations, Precog safety |
| `autopoiesis` | `CategoryAutopoiesis` | Self-improvement | Learning, Ouroboros loop |
| `campaign` | `CategoryCampaign` | Campaigns | Multi-phase orchestration |
| `context` | `CategoryContext` | Context mgmt | Compression, token budgets |
| `world` | `CategoryWorld` | World scanner | File topology, AST projection |
| `embedding` | `CategoryEmbedding` | Vectors | Embedding generation, similarity |
| `store` | `CategoryStore` | Memory tiers | CRUD across RAM/Vector/Graph/Cold |

## Go API Reference

### Initialization

```go
import "codenerd/internal/logging"

// Initialize at application startup
func main() {
    // workspacePath is the project root containing .nerd/
    if err := logging.Initialize(workspacePath); err != nil {
        log.Fatalf("Failed to initialize logging: %v", err)
    }
    defer logging.CloseAll()  // Flush and close all log files

    // ... application code ...
}
```

### Category-Specific Convenience Functions

Each category has convenience functions for Info and Debug levels:

```go
// Info level (logged when level <= info)
logging.Boot("System starting: version=%s", version)
logging.Session("New session: id=%s", sessionID)
logging.Kernel("Asserting fact: %s", fact.String())
logging.API("Request to %s: %d tokens", model, tokens)
logging.Perception("Extracted intent: %v", intent)
logging.Articulation("Generated response: %d chars", len(response))
logging.Routing("Routing to tool: %s", toolName)
logging.Tools("Executing tool: %s", toolName)
logging.VirtualStore("Loading facts for: %s", predicate)
logging.Shards("Spawning shard: %s type=%s", id, shardType)
logging.Coder("Generating code for: %s", target)
logging.Tester("Running tests: %s", testPath)
logging.Reviewer("Reviewing file: %s", filePath)
logging.Researcher("Fetching: %s", url)
logging.SystemShards("Legislator processing: %s", rule)
logging.Dream("Simulating: %s", action)
logging.Autopoiesis("Learning pattern: %s", pattern)
logging.Campaign("Phase transition: %s -> %s", from, to)
logging.Context("Compressing: %d -> %d tokens", before, after)
logging.World("Scanning: %s", directory)
logging.Embedding("Generating embedding: %d dims", dims)
logging.Store("Storing to %s: key=%s", tier, key)

// Debug level (logged when level <= debug)
logging.BootDebug("Detailed config: %+v", config)
logging.KernelDebug("Rule derivation: %v", derivation)
// ... etc for all categories
```

### Logger Instance API

For more control, get a logger instance:

```go
logger := logging.Get(logging.CategoryShards)

// Log levels
logger.Debug("Detailed info: %v", details)  // Only if level=debug
logger.Info("Normal info: %s", info)        // If level <= info
logger.Warn("Warning: %s", warning)         // If level <= warn
logger.Error("Error: %v", err)              // Always logged

// Check if logging is active
if logging.IsDebugMode() {
    // Expensive debug operation
}

if logging.IsCategoryEnabled(logging.CategoryKernel) {
    // Category-specific expensive operation
}
```

### Context Logging

Attach key-value context to log messages:

```go
logger := logging.Get(logging.CategoryShards)

ctx := logger.WithContext(map[string]interface{}{
    "shard_id":   shardID,
    "shard_type": shardType,
    "task":       taskName,
})

ctx.Info("Starting execution")
// Output: [INFO] Starting execution | ctx=map[shard_id:abc123 shard_type:coder task:generate]

ctx.Debug("Loaded %d facts", factCount)
ctx.Warn("Slow execution: %v", duration)
ctx.Error("Failed: %v", err)
```

### Performance Timing

Measure operation duration with automatic logging:

```go
// Basic timer
timer := logging.StartTimer(logging.CategoryKernel, "Query evaluation")
// ... operation ...
elapsed := timer.Stop()  // Logs: "[DEBUG] Query evaluation completed in 45ms"

// Log at Info level instead of Debug
timer := logging.StartTimer(logging.CategoryAPI, "LLM request")
elapsed := timer.StopWithInfo()  // Logs: "[INFO] LLM request completed in 2.3s"

// Warn if exceeds threshold
timer := logging.StartTimer(logging.CategoryShards, "Shard execution")
elapsed := timer.StopWithThreshold(5 * time.Second)
// If > 5s: "[WARN] Shard execution took 7.2s (threshold: 5s)"
// If <= 5s: "[DEBUG] Shard execution completed in 3.1s"
```

## Log File Format

### File Naming

```
.nerd/logs/{date}_{category}.log
```

Examples:
- `2025-12-08_kernel.log`
- `2025-12-08_shards.log`
- `2025-12-08_perception.log`

### Line Format

```
{date} {time}.{microseconds} [{LEVEL}] {message}
```

Examples:
```
2025/12/08 10:30:45.123456 [INFO] Asserting fact: user_intent(/id1, /query, /read, "foo", _)
2025/12/08 10:30:45.124000 [DEBUG] Derived 15 facts from rule next_action
2025/12/08 10:30:45.125000 [WARN] Query took 2.3s (threshold: 1s)
2025/12/08 10:30:45.126000 [ERROR] Failed to derive permitted(X): no matching rules
```

Context logging adds metadata:
```
2025/12/08 10:30:45.127000 [INFO] Starting execution | ctx=map[shard_id:abc task:generate]
```

## Integration with log-analyzer Skill

The logging system is designed for integration with the `log-analyzer` skill, which converts logs to Mangle facts for declarative debugging.

### Workflow

```bash
# 1. Enable debug mode
# Edit .nerd/config.json: "debug_mode": true

# 2. Run codeNERD session
./nerd chat

# 3. Parse logs to Mangle facts
python .claude/skills/log-analyzer/scripts/parse_log.py \
    .nerd/logs/*.log > session.mg

# 4. Run analysis queries
python .claude/skills/log-analyzer/scripts/analyze_logs.py \
    session.mg --builtin errors

# 5. Interactive debugging
python .claude/skills/log-analyzer/scripts/analyze_logs.py \
    session.mg --interactive
```

### Example Mangle Queries

```mangle
# Find all errors
?error_entry(Time, Category, Message).

# Error count by category
?error_count(Category, Count).

# Find root causes (first error in cascade)
?root_cause(Time, Category, Message).

# Error context (events before each error)
?error_context(ErrorTime, ErrorCat, PriorTime, PriorCat, PriorMsg).

# Cross-category event correlation
?correlated(Time1, Cat1, Time2, Cat2).

# Execution flow between categories
?flow_edge(FromCat, ToCat, Time).
?reachable(FromCat, ToCat).
```

### Log Fact Schema

The parser generates these Mangle facts:

```mangle
# Core fact structure
log_entry(Timestamp_ms, /category, /level, "message", "filename", line_number).

# Example:
log_entry(1733680045123, /kernel, /info, "Asserting fact: user_intent(...)", "2025-12-08_kernel.log", 42).
```

## Best Practices

### 1. Use Appropriate Log Levels

```go
// DEBUG: Detailed information for debugging
logging.KernelDebug("Rule body: %v", ruleBody)

// INFO: Normal operational messages
logging.Kernel("Query completed: %d results", len(results))

// WARN: Potentially problematic situations
logging.Get(logging.CategoryKernel).Warn("Query slow: %v", duration)

// ERROR: Errors that need attention
logging.Get(logging.CategoryKernel).Error("Query failed: %v", err)
```

### 2. Include Context in Messages

```go
// Good: includes context
logging.Shards("Spawning shard: id=%s type=%s task=%s", id, shardType, task)

// Bad: missing context
logging.Shards("Spawning shard")
```

### 3. Use Timers for Performance

```go
timer := logging.StartTimer(logging.CategoryAPI, "LLM request")
defer timer.StopWithThreshold(5 * time.Second)

// ... operation ...
```

### 4. Check Category Before Expensive Operations

```go
if logging.IsCategoryEnabled(logging.CategoryKernel) {
    // Only serialize if we're actually going to log
    logging.KernelDebug("Facts: %s", expensiveSerialize(facts))
}
```

### 5. Use Context Loggers for Related Operations

```go
ctx := logging.Get(logging.CategoryCoder).WithContext(map[string]interface{}{
    "task_id": taskID,
    "file":    targetFile,
})

ctx.Info("Starting code generation")
// ... multiple operations ...
ctx.Info("Code generation complete")
```

## Troubleshooting

### No Logs Created

1. Check `debug_mode` is `true` in `.nerd/config.json`
2. Verify the workspace path passed to `Initialize()`
3. Check category is enabled (default is enabled if not specified)

### Logs Missing Content

1. Verify log level allows the message (`debug` messages need `"level": "debug"`)
2. Check category is enabled in config
3. Ensure `CloseAll()` is called at shutdown to flush buffers

### Performance Issues

1. Set `debug_mode: false` for production
2. Disable verbose categories like `kernel` and `tools`
3. Raise log level to `warn` or `error`

## Implementation Details

### Source Files

| File | Purpose |
|------|---------|
| [internal/logging/logger.go](internal/logging/logger.go) | Main logger implementation |
| [internal/logging/logger_test.go](internal/logging/logger_test.go) | Test suite |
| [internal/config/config.go](internal/config/config.go) | LoggingConfig struct |

### Key Types

```go
type Category string  // Log category identifier

type Logger struct {
    category Category
    logger   *log.Logger
    file     *os.File
}

type ContextLogger struct {
    logger  *Logger
    context map[string]interface{}
}

type Timer struct {
    category Category
    op       string
    start    time.Time
}
```

### Thread Safety

- Logger creation is protected by `sync.RWMutex`
- Individual logger writes are thread-safe (Go's `log.Logger`)
- Config reads are protected by `sync.RWMutex`
