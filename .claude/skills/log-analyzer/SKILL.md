---
name: log-analyzer
description: Analyze codeNERD system logs using Mangle logic programming. This skill should be used when debugging codeNERD execution, tracing cross-system interactions, identifying error patterns, analyzing performance bottlenecks, or correlating events across the 22 logging categories. Converts log files to Mangle facts for declarative querying. Now includes context harness log cross-referencing for infinite context system debugging.
license: Apache-2.0
version: 2.4.0
last_updated: 2025-12-29
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

## Loop & Anomaly Detection (v2.2.0)

As of December 2025, codeNERD log-analyzer includes sophisticated loop detection and root cause diagnosis. This addresses a critical debugging need: detecting patterns that appear as successful operations but indicate bugs.

### Quick Loop Detection

For fast analysis without Mangle compilation, use the `detect_loops.py` script:

```bash
# Analyze logs and output JSON
python3 scripts/detect_loops.py .nerd/logs/*.log --pretty

# With custom threshold (default: 5)
python3 scripts/detect_loops.py .nerd/logs/*.log --threshold 3

# Save to file
python3 scripts/detect_loops.py .nerd/logs/*.log -o anomalies.json
```

**Example Output:**
```json
{
  "analysis_timestamp": "2025-12-23T16:00:00",
  "anomalies": [{
    "type": "action_loop",
    "severity": "critical",
    "action": "/analyze_code",
    "count": 40,
    "root_cause": {
      "diagnosis": "missing_fact_update",
      "explanation": "Action completes with success=true but no kernel fact asserted",
      "suggested_fix": "Check VirtualStore.RouteAction() for missing fact assertion"
    }
  }],
  "summary": {
    "total_anomalies": 2,
    "critical": 1,
    "high": 1,
    "loops_detected": 1
  }
}
```

### logquery Loop Detection Builtins

The logquery tool includes 8 new builtins for loop analysis:

| Builtin | Description |
|---------|-------------|
| `:loops` | Find action loops (same action repeated >5 times) |
| `:stagnation` | Find routing stagnation (next_action not advancing) |
| `:identical-results` | Find suspicious patterns (same result_len repeatedly) |
| `:slot-starvation` | Find API scheduler slot starvation events |
| `:false-success` | Find success masking failure (success=true but looping) |
| `:anomalies` | Combined anomaly report with severity levels |
| `:diagnose` | Full diagnosis with loop count, duration, and root cause |
| `:root-cause` | Root cause analysis only |

```bash
# Interactive analysis
./logquery.exe facts.mg -i
logquery> :loops
logquery> :diagnose
logquery> :root-cause

# JSON output for programmatic use
./logquery.exe facts.mg --builtin diagnose --format json
```

### Detected Patterns

| Pattern | Severity | Evidence | Meaning |
|---------|----------|----------|---------|
| **Action Loop** | Critical | Same action executed >5 times | State not advancing |
| **Repeated Call ID** | Critical | Same call_id used >2 times | Execution not regenerating IDs |
| **Identical Results** | High | Same result_len returned repeatedly | Tool returning cached/dummy response |
| **Routing Stagnation** | High | Same predicate queried >10 times | Kernel rule stuck |
| **Slot Starvation** | High | Waiting count >3 | API slots exhausted |
| **Long Slot Wait** | High | Wait duration >10s | Severe slot contention |
| **False Success** | High | success=true but looping | Success masking failure |

### Root Cause Diagnosis

The system diagnoses 4 root causes:

| Cause | Diagnosis | Explanation |
|-------|-----------|-------------|
| `missing_fact_update` | Action completes but no kernel fact asserted | VirtualStore.RouteAction() not calling Assert() |
| `kernel_rule_stuck` | next_action predicate returns same result | Missing state transition conditions in Mangle policy |
| `tool_caching` | Tool returns identical result every time | Tool returning cached/dummy response |
| `slot_starvation_correlated` | Loop correlates with slot exhaustion | Loop is consuming all API slots |

### Structured Event Extraction

The enhanced `parse_log.py` now extracts structured events from log messages:

```mangle
# Tool execution with call_id tracking
tool_execution(Time, ToolName, Action, Target, CallId, Duration, ResultLen).

# Action routing events
action_routing(Time, Predicate, ArgCount).

# Action completion with success/output
action_completed(Time, Action, Success, OutputLen).

# API scheduler slot status
slot_status(Time, ShardId, Active, MaxSlots, Waiting).

# Slot acquisition timing
slot_acquired(Time, ShardId, WaitDuration).
```

### Example Debugging Workflow

```bash
# 1. Quick detection with detect_loops.py
python3 scripts/detect_loops.py .nerd/logs/2025-12-23*.log --pretty

# 2. If anomalies found, deeper analysis with logquery
python3 scripts/parse_log.py .nerd/logs/2025-12-23*.log --no-schema | \
  grep "^log_entry\|^tool_execution\|^action_completed" > /tmp/facts.mg
./scripts/logquery/logquery.exe /tmp/facts.mg -i

# 3. Interactive diagnosis
logquery> :diagnose
logquery> :root-cause

# 4. Export for reporting
./scripts/logquery/logquery.exe /tmp/facts.mg --builtin diagnose --format json > diagnosis.json
```

### Common Loop Scenarios

**Scenario 1: Action Not Advancing State**

Symptoms:
- Same action (e.g., `/analyze_code`) executed 40+ times
- Same `call_id` never regenerated
- `success=true` but kernel state unchanged

Root Cause: `missing_fact_update`
Fix: Check VirtualStore.RouteAction() for missing fact assertion after tool execution

**Scenario 2: Kernel Rule Stuck**

Symptoms:
- `next_action` predicate queried 10+ times
- Returns same action repeatedly
- Routing stagnation detected

Root Cause: `kernel_rule_stuck`
Fix: Check Mangle policy rules for missing state transition conditions

**Scenario 3: Tool Returning Dummy Response**

Symptoms:
- Identical `result_len` on every execution
- Tool appears successful but returns same data

Root Cause: `tool_caching`
Fix: Check if tool is returning cached/dummy response instead of executing

## Comprehensive Anomaly Detection (v2.3.0)

As of December 2025, codeNERD log-analyzer includes comprehensive anomaly detection covering message duplication, JIT spam, initialization spam, API/LLM issues, and automated health checks.

### Quick Health Check

```bash
# Comprehensive health check with one command
./logquery.exe --builtin health-check /tmp/facts.mg

# Example output:
# health_summary(/degraded, 2, "issues detected")
# health_issue(/duplicates, 5, "Duplicate log messages detected")
# health_issue(/jit_spam, 1, "JIT compilation spam detected")
```

### New Duplication Detection Builtins

| Builtin | Description |
|---------|-------------|
| `:duplicates` | Detect messages that appear 5+ times with severity levels |
| `:timestamp-dups` | Find multiple messages at exact same timestamp |
| `:jit-spam` | Detect JIT compilation of same prompt repeatedly |
| `:jit-events` | List all JIT compilation events |
| `:init-spam` | Detect system re-initialization patterns |
| `:init-events` | List all initialization events |

### New API/LLM Issue Builtins

| Builtin | Description |
|---------|-------------|
| `:db-locks` | Detect database lock events |
| `:rate-limits` | Detect rate limit events |
| `:timeouts` | Detect timeout events |
| `:empty-responses` | Detect empty LLM responses (length=0) |
| `:feedback-failures` | Detect FeedbackLoop failures |
| `:deadlines` | Detect context deadline exceeded events |
| `:health-check` | Comprehensive health check combining all detectors |

### Severity Levels

Results include severity classification:

| Severity | Threshold | Meaning |
|----------|-----------|---------|
| `/medium` | 5-10 occurrences | Minor concern, worth monitoring |
| `/high` | 10-20 occurrences | Significant issue needing attention |
| `/critical` | >20 occurrences | Serious problem requiring immediate action |

### Health Status

The `health-check` builtin returns overall health status:

| Status | Condition |
|--------|-----------|
| `/healthy` | No issues detected |
| `/degraded` | 1-3 issues detected |
| `/unhealthy` | >3 issues detected |

### Detecting JIT Spam

JIT spam occurs when the same prompt is recompiled repeatedly, indicating a logic bug:

```bash
./logquery.exe --builtin jit-spam /tmp/facts.mg

# Example output:
# jit_spam("51145 bytes", 37, /critical, 1031612, 1766557134099, 1766558165711)
# Meaning: A 51145-byte prompt was compiled 37 times (critical severity)
```

### Detecting Message Duplication

Finds any log message appearing multiple times:

```bash
./logquery.exe --builtin duplicates /tmp/facts.mg

# Example output:
# duplicate_message("JIT compiled prompt: 51145 bytes", 37, /critical, ...)
# duplicate_message("ProcessLLMResponseAllowPlain: empty response", 5, /medium, ...)
```

### Enhanced detect_loops.py

The `detect_loops.py` script now detects 12 additional anomaly patterns:

```python
# New patterns detected:
- message_duplication      # Same message appearing repeatedly
- timestamp_collision      # Multiple events at same timestamp
- exact_duplicate          # Identical messages at same time
- jit_spam                 # JIT compilation repetition
- shard_jit_spam           # Per-shard JIT analysis
- init_spam                # Repeated initialization
- db_lock_cascade          # Database lock events
- rate_limit_cascade       # Rate limit events
- llm_timeout_cascade      # LLM timeout events
- empty_llm_responses      # Empty LLM responses
- feedback_loop_failures   # FeedbackLoop errors
- context_deadline_cascade # Context deadline exceeded
```

### Example: Full Session Analysis

```bash
# Parse all today's logs
python3 scripts/parse_log.py .nerd/logs/2025-12-24*.log --no-schema | \
  grep "^log_entry" > /tmp/session.mg

# Run comprehensive health check
./logquery.exe --builtin health-check /tmp/session.mg

# Get detailed breakdown
./logquery.exe --builtin duplicates /tmp/session.mg --limit 20
./logquery.exe --builtin jit-spam /tmp/session.mg
./logquery.exe --builtin empty-responses /tmp/session.mg

# Quick Python analysis
python3 scripts/detect_loops.py .nerd/logs/2025-12-24*.log --pretty
```

### Interpreting Results

**JIT Spam (Critical):**
- Same prompt compiled 5+ times indicates state not advancing
- Check if `next_action` is stuck returning same action
- Verify kernel fact updates after action completion

**Empty Responses:**
- LLM returning 0-byte responses indicates API issues
- Check rate limits, timeouts, or model availability
- May need retry logic or fallback model

**Duplicate Messages:**
- Normal for debug-level messages (ignore)
- Critical for action/execution messages (investigate)
- Look for loops in business logic

**Timestamp Collisions:**
- Normal for goroutine logging
- Concerning if many identical messages at same time
- May indicate buffered logging or clock issues

## Context Harness Log Analysis (v2.4.0)

The log-analyzer skill now supports cross-referencing between system logs (`.nerd/logs/`) and context harness test sessions (`.nerd/context-tests/`).

### Context Harness Log Files

Each context test session generates 7 specialized log files:

| File | Purpose |
|------|---------|
| `prompts.log` | Full prompts sent to LLM, token counts, budget utilization |
| `jit-compilation.log` | JIT prompt compiler traces, atom selection, priority ordering |
| `spreading-activation.log` | Activation score calculations, dependency graph traversal |
| `compression.log` | Before/after compression comparisons, ratios per turn |
| `piggyback-protocol.log` | Surface vs. control packet parsing, kernel state changes |
| `context-feedback.log` | LLM context usefulness ratings, learned predicate scores |
| `summary.log` | Overall session statistics, checkpoint results |

### Quick Context Harness Analysis

```bash
# Parse context harness session
python3 scripts/parse_context_harness.py .nerd/context-tests/session-20251229-190425/

# Cross-reference with system logs from same time period
python3 scripts/parse_context_harness.py \
  .nerd/context-tests/session-20251229-190425/ \
  --cross-ref .nerd/logs/2025-12-29*.log \
  --output context_analysis.mg

# Use logquery for combined analysis
./scripts/logquery/logquery.exe context_analysis.mg --builtin context-issues
```

### Context Harness Mangle Facts

The parser generates these context-specific facts:

```mangle
# JIT compilation events
# jit_compilation(Time, AtomCount, TotalTokens, BudgetUsed)
Decl jit_compilation(Time.Type<int>, AtomCount.Type<int>, TotalTokens.Type<int>, BudgetUsed.Type<float>).

# Spreading activation events
# activation_score(Time, FactId, Score, Source)
Decl activation_score(Time.Type<int>, FactId.Type<string>, Score.Type<float>, Source.Type<string>).

# Compression events
# compression_event(Time, InputTokens, OutputTokens, Ratio)
Decl compression_event(Time.Type<int>, InputTokens.Type<int>, OutputTokens.Type<int>, Ratio.Type<float>).

# Checkpoint results
# checkpoint_result(Turn, Description, Passed, Precision, Recall)
Decl checkpoint_result(Turn.Type<int>, Description.Type<string>, Passed.Type<name>, Precision.Type<float>, Recall.Type<float>).

# Context feedback events
# context_feedback(Time, PredicateId, Rating, Impact)
Decl context_feedback(Time.Type<int>, PredicateId.Type<string>, Rating.Type<string>, Impact.Type<float>).

# Piggyback protocol events
# piggyback_event(Time, EventType, IntentVerb, ToolCount)
Decl piggyback_event(Time.Type<int>, EventType.Type<string>, IntentVerb.Type<string>, ToolCount.Type<int>).
```

### Cross-Reference Queries

Find correlations between context harness events and system logs:

```bash
# In logquery REPL
logquery> ?context_correlated(HarnessTime, HarnessCat, SystemTime, SystemCat)

# Find JIT compilations that preceded errors
logquery> ?jit_before_error(JitTime, AtomCount, ErrorTime, ErrorMsg)

# Find compression events correlated with API timeouts
logquery> ?compression_api_correlation(CompTime, Ratio, ApiTime, Duration)
```

### logquery Context Builtins

| Builtin | Description |
|---------|-------------|
| `:context-issues` | Combined context system health check |
| `:jit-hotspots` | Frequent JIT recompilation patterns |
| `:activation-anomalies` | Unusual activation score patterns |
| `:compression-failures` | Low compression ratio events |
| `:checkpoint-failures` | Failed checkpoint validations |
| `:feedback-drift` | Context feedback score degradation |

### Example: Full Context Debugging Session

```bash
# 1. Run context harness test
./nerd.exe test-context --scenario debugging-marathon

# 2. Find the latest test session
$session = Get-ChildItem .nerd/context-tests -Directory | Sort-Object LastWriteTime -Descending | Select-Object -First 1

# 3. Parse both context harness and system logs
python3 .claude/skills/log-analyzer/scripts/parse_context_harness.py $session.FullName \
  --cross-ref .nerd/logs/*.log \
  --no-schema > /tmp/context_facts.mg

python3 .claude/skills/log-analyzer/scripts/parse_log.py .nerd/logs/*.log \
  --no-schema >> /tmp/context_facts.mg

# 4. Analyze with logquery
./scripts/logquery/logquery.exe /tmp/context_facts.mg -i

logquery> :context-issues
logquery> :jit-hotspots
logquery> ?checkpoint_result(Turn, Desc, /false, P, R)
```

### Interpreting Context Harness Results

**Low Retrieval Precision (<10%):**
- Too many irrelevant facts selected by spreading activation
- Check `activation_score` facts for overly broad activation
- Review `activation-anomalies` builtin

**Low Retrieval Recall (<50%):**
- Important facts not being activated
- Check for missing dependency links in knowledge graph
- Review spreading activation decay parameters

**Checkpoint Failures:**
- Expected facts not retrievable after N turns
- Cross-reference with compression events (lossy compression?)
- Check for fact expiration in ephemeral predicates

**JIT Hotspots:**
- Same prompt compiled repeatedly
- Usually indicates kernel state not advancing
- Cross-reference with loop detection (`:loops`)

## See Also

- [mangle-programming skill](../mangle-programming/SKILL.md) - Full Mangle reference
- [LOG_ANALYSIS_PATTERNS](references/LOG_ANALYSIS_PATTERNS.md) - Extended pattern catalog
- [log-schema.mg](assets/log-schema.mg) - Full schema (for reference)
- [logquery/schema.mg](scripts/logquery/schema.mg) - Embedded schema (simplified)
- [detect_loops.py](scripts/detect_loops.py) - Quick loop detection script
- [parse_context_harness.py](scripts/parse_context_harness.py) - Context harness log parser
