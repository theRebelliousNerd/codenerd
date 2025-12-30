---
name: log-analyzer
description: Analyze codeNERD system logs using Mangle logic programming. This skill should be used when the user asks to query logs, debug execution, trace cross-system interactions, identify error patterns, analyze performance bottlenecks, or correlate events across logging categories using logquery or Mangle facts.
license: Apache-2.0
version: 2.1.0
last_updated: 2025-12-23
---

# Log Analyzer: Mangle-Powered codeNERD Debugging

This skill enables declarative log analysis using Google Mangle. Rather than grep-based searching, logs are converted to Mangle facts enabling powerful recursive queries, pattern detection, and cross-category correlation.

## Quick Start

### Option 1: logquery Tool (Recommended)

The `logquery` tool is a compiled Go binary with embedded Mangle engine and schemas.

```bash
# Build the tool (one-time)
cd .claude/skills/log-analyzer/scripts/logquery
go build -o logquery.exe .

# Parse logs and query in one step
python3 ../parse_log.py .nerd/logs/* --no-schema | grep "^log_entry" > /tmp/facts.mg
./logquery.exe /tmp/facts.mg --builtin errors

# Interactive REPL
./logquery.exe /tmp/facts.mg -i

# JSON output
./logquery.exe /tmp/facts.mg --builtin kernel-errors --format json
```

### Option 2: Python Scripts

```bash
# Parse all logs from current session
python3 scripts/parse_log.py .nerd/logs/*.log > session.mg

# Parse specific categories
python3 scripts/parse_log.py .nerd/logs/*kernel*.log .nerd/logs/*shards*.log > focus.mg

# Parse with time range filter
python3 scripts/parse_log.py --after "2025-12-08 10:00:00" .nerd/logs/*.log > window.mg
```

## logquery Tool Reference

### Built-in Analyses

```bash
$ logquery.exe --list-builtins

Available built-in analyses:

  api              All API events
  api-errors       API errors only
  autopoiesis      All autopoiesis events
  boot             All boot events
  categories       All active categories in the logs
  error-categories Categories that have errors
  errors           All error entries with timestamps and categories
  kernel           All kernel events
  kernel-errors    Kernel errors only
  perception       All perception events
  session          All session events
  shard-errors     Shard errors only
  shards           All shard events
  warnings         All warning entries
```

### Command-Line Options

```bash
logquery [options] <facts.mg>

Options:
  -builtin string    Run built-in analysis (see --list-builtins)
  -format string     Output format: text, json, table (default "text")
  -i                 Interactive REPL mode
  -limit int         Maximum results to display, 0=unlimited (default 100)
  -list-builtins     List available built-in analyses
  -query string      Mangle query predicate (e.g., error_entry)
  -schema-only       Print embedded schema and exit
  -stdin             Read facts from stdin
  -v                 Verbose output (show parse/eval stats)
```

### Interactive REPL

```text
$ logquery.exe facts.mg -i

logquery REPL - Mangle Log Analyzer
Commands:
  ?<predicate>   Query predicate (e.g., ?error_entry)
  :builtins      List built-in analyses
  :<builtin>     Run built-in (e.g., :errors)
  :predicates    List all available predicates
  :help          Show this help
  :quit          Exit

logquery> :categories
Results: 15

active_category(/kernel)
active_category(/shards)
active_category(/perception)
...

logquery> ?kernel_event
Results: 12345

kernel_event(1733680000123, /info, "Initializing Mangle engine")
...
```

## Log Fact Schema

The parser converts log entries to these Mangle facts:

```mangle
# Core log entry fact
# log_entry(Timestamp, Category, Level, Message, File, Line)
Decl log_entry(Time, Category, Level, Message, File, Line).

# Example generated facts:
log_entry(1733680000123, /kernel, /info, "Initializing Mangle engine", "kernel.go", 142).
log_entry(1733680000456, /shards, /debug, "CoderShard executing task", "coder.go", 89).
log_entry(1733680001789, /kernel, /error, "Failed to derive next_action", "kernel.go", 312).
```

### Derived Predicates

```mangle
# Filter by level
Decl error_entry(Time, Category, Message).
error_entry(T, C, M) :- log_entry(T, C, /error, M, _, _).

Decl warning_entry(Time, Category, Message).
warning_entry(T, C, M) :- log_entry(T, C, /warn, M, _, _).

# Category existence
Decl active_category(Category).
active_category(C) :- log_entry(_, C, _, _, _, _).

Decl error_category(Category).
error_category(C) :- error_entry(_, C, _).

# Category-specific predicates
Decl kernel_event(Time, Level, Message).
kernel_event(T, L, M) :- log_entry(T, /kernel, L, M, _, _).

Decl shard_event(Time, Level, Message).
shard_event(T, L, M) :- log_entry(T, /shards, L, M, _, _).

# ... similar for all 22 categories
```

## Common Debugging Patterns

### Pattern 1: Find All Errors

```bash
# Using logquery
logquery.exe facts.mg --builtin errors --limit 50

# REPL
logquery> :errors
```

### Pattern 2: Kernel-Specific Errors

```bash
logquery.exe facts.mg --builtin kernel-errors --format json > kernel_errors.json
```

### Pattern 3: Filter by Category

```bash
# In REPL
logquery> ?perception_event
logquery> ?autopoiesis_event
```

### Pattern 4: Direct Predicate Query

```bash
# Query any declared predicate
logquery.exe facts.mg --query warning_entry
```

## The 22 Log Categories

| Category | Description | Key Events |
|----------|-------------|------------|
| `/boot` | System startup | Initialization, config loading |
| `/session` | Chat session lifecycle | New session, turn processing |
| `/kernel` | Mangle engine ops | Fact assertion, rule derivation |
| `/perception` | NL -> atoms | Intent extraction, atom creation |
| `/articulation` | Atoms -> NL | Response generation, piggyback |
| `/routing` | Action dispatch | Tool selection, delegation |
| `/tools` | Tool execution | Invocations, results |
| `/virtual_store` | FFI layer | External API calls |
| `/shards` | Shard manager | Spawn, execute, destroy |
| `/coder` | CoderShard | Code generation, edits |
| `/tester` | TesterShard | Test execution |
| `/reviewer` | ReviewerShard | Code review |
| `/researcher` | ResearcherShard | Knowledge gathering |
| `/system_shards` | System shard ops | Policy, firewall, router |
| `/dream` | Dream state | Analysis, optimization |
| `/autopoiesis` | Self-improvement | Learning, pattern tracking |
| `/campaign` | Multi-phase goals | Planning, execution |
| `/context` | Context compression | Window management |
| `/world` | Filesystem projection | File topology, scanning |
| `/embedding` | Vector operations | Embedding generation |
| `/store` | Memory tiers | CRUD operations |
| `/api` | LLM API calls | Requests, responses |

## Enhanced Logging Coverage (v2.1.0)

As of December 2025, codeNERD includes comprehensive logging across all subsystems. This section documents the logging enhancements that provide better observability for debugging.

### Logging API Convenience Functions

All 22 log categories now have Warn/Error convenience functions in `internal/logging/logger.go`:

```go
// Example usage patterns
logging.BootDebug("Loading config from: %s", path)
logging.BootError("Failed to read config file %s: %v", path, err)
logging.PerceptionDebug("[Anthropic] CompleteWithSystem: model=%s", model)
logging.CampaignWarn("failed to save campaign after replan: %v", err)
logging.WorldWarn("ScanWorkspaceIncremental: failed to upsert world file %s: %v", path, err)
logging.ToolsDebug("RegisterTool: registering tool name=%s", name)
logging.StoreWarn("DocumentIngestor: failed to store vector for %s: %v", path, err)
logging.KernelWarn("failed to assert hypothetical fact: %v", err)
```

### Key Subsystem Logging

#### LLM Client Logging (`/perception`)

All 7 LLM client implementations now log:

- Entry: model, prompt lengths
- Completion: duration, response length
- Errors: API failures, timeouts, retries

```text
[Perception DEBUG] [Anthropic] CompleteWithSystem: model=claude-sonnet-4-5 system_len=2048 user_len=512
[Perception INFO]  [Anthropic] CompleteWithSystem: completed in 2.3s response_len=1024
[Perception ERROR] [OpenRouter] CompleteWithStreaming: max retries exceeded after 30s: connection timeout
```

#### Tool Registry Logging (`/tools`)

Tool execution now has full audit trail:

- Registration: tool name, command, shard affinity
- Execution: start/end, args, duration
- Errors: binary not found, execution failures

```text
[Tools DEBUG] RegisterTool: registering tool name=mytool command=./mytool affinity=coder
[Tools INFO]  ExecuteRegisteredTool: executing tool=mytool exec_count=5 args=[--verbose]
[Tools ERROR] ExecuteRegisteredTool: tool=mytool failed after 5.2s: exit code 1
```

#### Campaign Orchestration (`/campaign`)

Campaign execution events:

- Phase transitions, task execution
- Checkpoint results, replan triggers
- Save/load operations, context compression

```text
[Campaign INFO]  Executing phase: implementation (tasks=12)
[Campaign WARN]  failed to save campaign after compression: disk full
[Campaign DEBUG] DocumentIngestor.Ingest: starting campaign=abc123 files=5
```

#### World Model Scanning (`/world`)

Filesystem projection operations:

- Incremental scan progress
- File upsert/delete operations
- Parse errors, cache updates

```text
[World INFO]  Starting incremental workspace scan: /project
[World WARN]  ScanWorkspaceIncremental: failed to delete world file /old/file.go: not found
```

#### Configuration Loading (`/boot`)

Startup and config operations:

- Config file detection
- Provider selection
- Default fallbacks

```text
[Boot DEBUG] Loading config from: .nerd/config.json
[Boot INFO]  Config file not found, using defaults: .nerd/config.json
[Boot INFO]  Config loaded: provider=anthropic model=claude-sonnet-4-5
```

#### Safety System (`/coder`)

Safety check visibility:

- Impact analysis results
- Block reasons with targets
- Kernel query failures

```text
[Coder DEBUG] checkImpact: checking safety for target=/critical/file.go
[Coder INFO]  checkImpact: BLOCKED target=/etc/passwd reason=system_file
[Coder WARN]  checkImpact: failed to query coder_block_write: kernel timeout
```

### Swallowed Error Logging

Previously silent error ignores now log before continuing:

| Location | What's Logged |
|----------|---------------|
| `main.go` | Kernel assertions, world persistence |
| `verifier.go` | Verification storage, JSON marshal |
| `world/persist.go` | World file upserts, fact replacement |
| `world/incremental_scan.go` | Walk errors, DB operations |
| `campaign/orchestrator_*.go` | Save operations, kernel fact updates |
| `campaign/document_ingestor.go` | Store link/vector/atom operations |

### Debugging New Log Categories

```bash
# Find all LLM client operations
logquery.exe facts.mg --query perception_event --limit 100

# Track tool executions
grep "ExecuteRegisteredTool" .nerd/logs/tools.log

# Campaign save failures
logquery.exe facts.mg -i
logquery> ?warning_entry(T, /campaign, M)

# Config loading issues
grep "Config" .nerd/logs/boot.log
```

## Scripts Reference

### logquery (Go)

Purpose-built Mangle query engine with embedded schema.

```bash
# Location
.claude/skills/log-analyzer/scripts/logquery/

# Build
go build -o logquery.exe .

# Files
main.go       # Entry point, CLI, REPL
schema.mg     # Embedded Mangle schema (go:embed)
go.mod        # Module definition
```

### parse_log.py

Converts log files to Mangle facts.

```bash
# Usage
python3 scripts/parse_log.py [options] <log_files...>

# Options
--output FILE      Output file (default: stdout)
--after DATETIME   Only entries after this time
--before DATETIME  Only entries before this time
--category CAT     Filter to specific category
--level LEVEL      Minimum level (debug, info, warn, error)
--format FORMAT    Output format: mangle (default), json, csv
--no-schema        Omit schema declarations (for logquery)
--schema-only      Output only schema declarations
```

## Integration with codeNERD

### Live Debugging Session

```bash
# 1. Enable debug mode in config
# .nerd/config.json: "debug_mode": true

# 2. Run codeNERD session
./nerd chat

# 3. Parse and analyze logs
cd .claude/skills/log-analyzer/scripts
python3 parse_log.py .nerd/logs/* --no-schema | grep "^log_entry" > /tmp/session.mg
cd logquery
./logquery.exe /tmp/session.mg -i
```

### Quick Error Check

```bash
# One-liner to find all errors
python3 parse_log.py .nerd/logs/* --no-schema 2>/dev/null | \
  grep "^log_entry" | \
  ./logquery/logquery.exe --stdin --builtin errors --limit 20
```

## Troubleshooting

### No Facts Generated

1. Verify logs exist: `ls .nerd/logs/`
2. Check debug_mode is enabled in config
3. Verify log format matches expected pattern

### Query Returns Empty

1. Check fact file has content: `head session.mg`
2. Use `:predicates` in REPL to see all available predicates
3. Ensure predicate name is correct

### Fact Limit Exceeded

For very large log files (>500K entries), some predicates are disabled due to O(n^2) explosion. The tool handles up to 5M derived facts.

```bash
# Filter logs before parsing to reduce size
grep -E "(error|warn)" .nerd/logs/*.log > filtered.log
python3 parse_log.py filtered.log --no-schema | grep "^log_entry" > facts.mg
```

### Performance Optimization

```bash
# For fastest results, filter at grep level
grep "/kernel" .nerd/logs/* | python3 parse_log.py /dev/stdin --no-schema > kernel.mg

# Use --limit to cap output
./logquery.exe facts.mg --builtin errors --limit 100
```

## See Also

- [mangle-programming skill](../mangle-programming/SKILL.md) - Full Mangle reference
- [LOG_ANALYSIS_PATTERNS](references/LOG_ANALYSIS_PATTERNS.md) - Extended pattern catalog
- [log-schema.mg](assets/log-schema.mg) - Full schema (for reference)
- [logquery/schema.mg](scripts/logquery/schema.mg) - Embedded schema (simplified)
