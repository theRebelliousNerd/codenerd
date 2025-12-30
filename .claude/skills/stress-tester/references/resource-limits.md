# Resource Limits Reference

All configurable limits in codeNERD with safe, aggressive, and chaos values for stress testing.

## Configuration File

**Location:** `.nerd/config.json`

## Core Limits

### Memory Limits

| Limit | Config Key | Default | Safe | Aggressive | Chaos |
|-------|-----------|---------|------|------------|-------|
| Total Memory | `core_limits.max_total_memory_mb` | 12288 | 8192 | 16384 | 32768 |
| Kernel Facts | `core_limits.max_facts_in_kernel` | 250000 | 100000 | 500000 | 1000000 |
| Derived Facts | `mangle.max_derived_facts_limit` | 100000 | 50000 | 200000 | 500000 |

**Memory Budget Allocation:**
- Core: 5%
- Atoms: 25%
- History: 20%
- Working: 50%

### Time Limits

| Limit | Config Key | Default | Safe | Aggressive | Chaos |
|-------|-----------|---------|------|------------|-------|
| Session Duration | `core_limits.max_session_duration_min` | 120 | 60 | 180 | 240 |
| Shard Timeout | `shard_profiles.*.timeout_seconds` | 300-900 | 300 | 1200 | 3600 |
| Action Timeout | `execution.action_timeout_seconds` | 300 | 180 | 600 | 1800 |

### Concurrency Limits

| Limit | Config Key | Default | Safe | Aggressive | Chaos |
|-------|-----------|---------|------|------------|-------|
| Concurrent Shards | `core_limits.max_concurrent_shards` | 4 | 2 | 8 | 16 |
| Queue Size | (hardcoded) | 100 | - | - | - |
| Queue Workers | (hardcoded) | 2 | - | - | - |
| Backpressure Threshold | (hardcoded) | 0.7 | - | - | - |

### API Limits

| Limit | Config Key | Default | Notes |
|-------|-----------|---------|-------|
| Max Retries | `shard_profiles.*.max_retries` | 2-3 | Per shard type |
| Rate Limit | (none) | - | NO RATE LIMITING! |
| Batch Size | `embedding.batch_size` | 100 | Embedding batches |

---

## Shard-Specific Limits

### CoderShard

| Limit | Config Key | Default | Notes |
|-------|-----------|---------|-------|
| Timeout | `shard_profiles.coder.timeout_seconds` | 600 | 10 minutes |
| Max Retries | `shard_profiles.coder.max_retries` | 3 | |
| Memory Limit | `shard_profiles.coder.memory_limit` | 2048 | 2GB |

### TesterShard

| Limit | Config Key | Default | Notes |
|-------|-----------|---------|-------|
| Timeout | `shard_profiles.tester.timeout_seconds` | 900 | 15 minutes |
| Max Retries | `shard_profiles.tester.max_retries` | 2 | |
| TDD Max Iterations | (hardcoded) | 5 | Red→Green cycles |

### ReviewerShard

| Limit | Config Key | Default | Notes |
|-------|-----------|---------|-------|
| Timeout | `shard_profiles.reviewer.timeout_seconds` | 600 | 10 minutes |
| Max Findings | (hardcoded) | 1000 | Before truncation |

### ResearcherShard

| Limit | Config Key | Default | Notes |
|-------|-----------|---------|-------|
| Timeout | `shard_profiles.researcher.timeout_seconds` | 900 | 15 minutes |
| Max Pages | (hardcoded) | 50 | Per research session |
| Connection Pool | (hardcoded) | 10 | Concurrent connections |

---

## Browser Integration Limits

| Limit | Config Key | Default | Notes |
|-------|-----------|---------|-------|
| Page Timeout | `integrations.browser.timeout` | 60s | Per page load |
| Max Sessions | (hardcoded) | 5 | Concurrent browsers |

---

## Campaign Limits

| Limit | Config Key | Default | Notes |
|-------|-----------|---------|-------|
| Max Phases | (hardcoded) | 50 | Per campaign |
| Max Tasks/Phase | (hardcoded) | 100 | |
| Checkpoint Interval | (hardcoded) | 5 min | Auto-save |

---

## Tool Generation Limits (Ouroboros)

| Limit | Config Key | Default | Notes |
|-------|-----------|---------|-------|
| Generation Timeout | `tool_generation.timeout_seconds` | 300 | 5 minutes |
| Max Compile Retries | `tool_generation.max_compile_retries` | 3 | |
| Max Tool Size | (hardcoded) | 10000 | Lines of code |

---

## Context/Compression Limits

| Limit | Config Key | Default | Notes |
|-------|-----------|---------|-------|
| Compression Threshold | `memory.compression_threshold` | 0.8 | 80% triggers compress |
| Emergency Threshold | (hardcoded) | 0.95 | 95% triggers emergency |
| Max Context Tokens | (hardcoded) | 128000 | Model dependent |

---

## Stress Test Config Modifications

### Conservative Test Config

No config changes - use defaults.

### Aggressive Test Config

Modify `.nerd/config.json`:
```json
{
  "core_limits": {
    "max_concurrent_shards": 8,
    "max_facts_in_kernel": 500000,
    "max_total_memory_mb": 16384,
    "max_session_duration_min": 180
  },
  "mangle": {
    "max_derived_facts_limit": 200000
  }
}
```

### Chaos Test Config

Modify `.nerd/config.json`:
```json
{
  "core_limits": {
    "max_concurrent_shards": 16,
    "max_facts_in_kernel": 1000000,
    "max_total_memory_mb": 32768,
    "max_session_duration_min": 240
  },
  "mangle": {
    "max_derived_facts_limit": 500000
  },
  "execution": {
    "action_timeout_seconds": 1800
  }
}
```

---

## System Resource Monitoring

### Memory Monitoring

```bash
# Windows PowerShell
while ($true) {
    Get-Process nerd -ErrorAction SilentlyContinue | Select-Object WorkingSet64
    Start-Sleep 5
}
```

### Process Monitoring

```bash
# Count nerd-related processes
Get-Process | Where-Object { $_.Name -match 'nerd|chrome|go' } | Measure-Object
```

### File Descriptor Monitoring

```bash
# Check open handles (Windows)
handle.exe -p nerd.exe
```

---

## Resource Exhaustion Scenarios

### Scenario: Memory Exhaustion

1. Load 250k facts into kernel
2. Trigger complex derivation
3. Monitor memory growth
4. Expect: Emergency compression at 95%

### Scenario: Queue Exhaustion

1. Submit 100 spawn requests rapidly
2. Monitor queue state
3. Expect: 101st request rejected with ErrQueueFull

### Scenario: Session Timeout

1. Keep session running for 120+ minutes
2. Trigger periodic queries to keep active
3. Expect: Session terminates at limit

### Scenario: Disk Exhaustion

1. Generate tools repeatedly
2. Keep all artifacts
3. Monitor disk usage
4. Expect: Eventually fails with disk full

---

## Recovery After Limit Hit

### After Memory Limit

```bash
# Force garbage collection (restart session)
nerd /new-session
```

### After Queue Full

```bash
# Wait for current shards to complete
nerd status
# Queue will drain
```

### After Session Timeout

```bash
# Session auto-terminates
# State saved to checkpoint
# Resume with new session
nerd campaign resume
```

### After Disk Full

```bash
# Clean up generated tools
rm -rf .nerd/tools/.compiled/*
rm -rf .nerd/tools/.traces/*
# Clean up campaign artifacts
rm -rf .nerd/campaigns/*
```
