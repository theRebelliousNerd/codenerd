# LLM Timeout Consolidation Stress Test

Stress test for the centralized 3-tier timeout configuration system.

## Overview

Tests the LLM Timeout Consolidation system's handling of:

- Timeout hierarchy (Tier 1 Per-Call → Tier 2 Operation → Tier 3 Campaign)
- Correct timeout propagation through call chain
- Context deadline inheritance
- Timeout overrides and profiles
- Race conditions in timeout chain

**Expected Duration:** 25-40 minutes total

### Key Insight

> In Go, the SHORTEST timeout in the chain wins. If you have a 10-minute HTTP client but wrap the call in a 90-second context, the context wins and the call fails after 90 seconds.

### Key Files

- `internal/config/llm_timeouts.go` - Centralized timeout configuration
- `internal/perception/client.go` - HTTP client timeout application
- `internal/core/shard_manager.go` - Shard execution timeout
- `internal/articulation/emitter.go` - Articulation timeout

### Timeout Tiers

| Tier | Purpose | Default |
|------|---------|---------|
| **Tier 1: Per-Call** | HTTP/API timeouts | 10 minutes |
| **Tier 2: Operation** | Multi-step operations | 5-20 minutes |
| **Tier 3: Campaign** | Long-running orchestration | 30 minutes |

### Key Timeouts

| Timeout | Default | Purpose |
|---------|---------|---------|
| `HTTPClientTimeout` | 10 min | Maximum HTTP operation time |
| `PerCallTimeout` | 10 min | Single LLM call context |
| `ShardExecutionTimeout` | 20 min | Shard spawn + research + LLM |
| `ArticulationTimeout` | 5 min | Transducer LLM calls |
| `CampaignPhaseTimeout` | 30 min | Full campaign phase |

---

## Conservative Test (8-12 min)

Test basic timeout configuration and propagation.

### Step 1: Verify Timeout Configuration (wait 2 min)

```bash
./nerd.exe status
```

Check timeout initialization:

```bash
Select-String -Path ".nerd/logs/*boot*.log" -Pattern "LLMTimeouts|timeout|HTTPClient"
```

### Step 2: Simple Operation Timeout (wait 4 min)

Execute task within timeout:

```bash
./nerd.exe spawn coder "write hello world"
```

Verify timeout context created:

```bash
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "timeout|context|deadline"
```

### Step 3: Check Tier Propagation (wait 3 min)

Verify child contexts inherit parent timeouts:

```bash
Select-String -Path ".nerd/logs/*api*.log" -Pattern "deadline|context"
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "ShardExecutionTimeout"
```

### Step 4: Verify No Conflicting Timeouts (wait 2 min)

Check for timeout mismatch warnings:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "timeout conflict|deadline before|shorter timeout"
```

### Success Criteria

- [ ] Default timeouts loaded from config
- [ ] Context deadlines set correctly
- [ ] Child contexts inherit parent timeouts
- [ ] No conflicting timeout warnings

---

## Aggressive Test (10-15 min)

Push timeout boundaries and verify correct behavior.

### Step 1: Clear Logs (wait 1 min)

```bash
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Long-Running Operation (wait 8 min)

Execute task that approaches timeout:

```bash
./nerd.exe spawn coder "analyze the entire internal/core package and generate comprehensive documentation"
```

Monitor timeout warnings:

```bash
# Watch for approaching deadline warnings
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "deadline approaching|remaining time"
```

### Step 3: Articulation Timeout Test (wait 4 min)

Test articulation-specific timeout:

```bash
./nerd.exe spawn coder "generate a very detailed explanation of the kernel architecture"
```

Check articulation timeout applied:

```bash
Select-String -Path ".nerd/logs/*articulation*.log" -Pattern "ArticulationTimeout|deadline"
```

### Step 4: Concurrent Operations with Different Timeouts (wait 4 min)

Run operations with different timeout requirements:

```bash
Start-Job { ./nerd.exe spawn coder "quick task" }
Start-Job { ./nerd.exe spawn researcher "deep research on Go patterns" }
Start-Job { ./nerd.exe perception "simple query" }

Get-Job | Wait-Job -Timeout 300
Get-Job | Receive-Job -ErrorAction SilentlyContinue
Get-Job | Remove-Job
```

### Success Criteria

- [ ] Long operations completed within timeout
- [ ] Articulation timeout applied correctly
- [ ] Different operations got appropriate timeouts
- [ ] No premature timeout failures

---

## Chaos Test (12-18 min)

Stress test with timeout edge cases.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Force Timeout Expiration (wait 6 min)

Create task that will exceed timeout (use aggressive timeouts):

```bash
# This should hit timeout if using AggressiveLLMTimeouts
./nerd.exe spawn coder "generate an extremely detailed 5000-line implementation with full documentation"
```

Check timeout handling:

```bash
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "context deadline exceeded|timeout|cancelled"
```

### Step 3: Nested Context Cancellation (wait 4 min)

Test cancellation propagation:

```bash
$job = Start-Job { ./nerd.exe campaign start "build a complex system" }
Start-Sleep 20
Stop-Job $job
Remove-Job $job
```

Verify child contexts cancelled:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "context cancel|propagat|child cancel"
```

### Step 4: Timeout Chain Validation (wait 4 min)

Verify timeout chain integrity:

```bash
# Campaign (30m) > Shard (20m) > API (10m)
./nerd.exe campaign start "simple test campaign"
```

Check timeout hierarchy in logs:

```bash
Select-String -Path ".nerd/logs/*campaign*.log" -Pattern "CampaignPhaseTimeout"
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "ShardExecutionTimeout"
Select-String -Path ".nerd/logs/*api*.log" -Pattern "PerCallTimeout|HTTPClientTimeout"
```

### Step 5: Retry with Timeout Reset (wait 3 min)

Test that retries get fresh timeouts:

```bash
./nerd.exe spawn coder "task that might need retry"
```

Check retry timeout handling:

```bash
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "retry|fresh timeout|reset deadline"
```

### Success Criteria

- [ ] Timeout expiration handled gracefully
- [ ] Cancellation propagated to child contexts
- [ ] Timeout hierarchy maintained (Campaign > Shard > API)
- [ ] Retries get fresh timeout windows

---

## Hybrid Test (12-15 min)

Test timeout integration with full system.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Campaign with Multiple Timeout Tiers (wait 8 min)

Run campaign exercising all timeout tiers:

```bash
./nerd.exe campaign start "create a REST API with tests and documentation"
```

Monitor timeout tier usage:

```bash
# Tier 3: Campaign
Select-String -Path ".nerd/logs/*campaign*.log" -Pattern "CampaignPhaseTimeout|OODALoopTimeout"

# Tier 2: Operations
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "ShardExecutionTimeout|ArticulationTimeout"

# Tier 1: API calls
Select-String -Path ".nerd/logs/*api*.log" -Pattern "HTTPClientTimeout|PerCallTimeout"
```

### Step 3: Verify Timeout Profiles (wait 3 min)

Check different timeout profiles applied:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "DefaultLLMTimeouts|FastLLMTimeouts|AggressiveLLMTimeouts"
```

### Step 4: Document Processing Timeout (wait 3 min)

Test document-specific timeout:

```bash
./nerd.exe refresh-docs
```

Check document timeout:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "DocumentProcessingTimeout"
```

### Success Criteria

- [ ] All three timeout tiers exercised
- [ ] Timeout profiles applied correctly
- [ ] Document processing timeout used
- [ ] Campaign completed within timeouts

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Timeout-Specific Queries

```bash
# Count timeout events
Select-String -Path ".nerd/logs/*.log" -Pattern "deadline exceeded" | Measure-Object

# Find timeout warnings
Select-String -Path ".nerd/logs/*.log" -Pattern "timeout|deadline" |
    Where-Object { $_.Line -match "warning|approaching" }

# Check timeout durations
Select-String -Path ".nerd/logs/*.log" -Pattern "timeout.*=.*min|timeout.*=.*sec"
```

### Timeout Chain Analysis

```mangle
# Log analysis queries
timeout_event(T, Tier, Duration, M) :-
    log_entry(T, Cat, _, M, _, _),
    fn:contains(M, "timeout"),
    extract_tier(M, Tier),
    extract_duration(M, Duration).

deadline_exceeded(T, M) :-
    log_entry(T, _, /error, M, _, _),
    fn:contains(M, "deadline exceeded").
```

### Success Metrics

| Metric | Conservative | Aggressive | Chaos | Hybrid |
|--------|--------------|------------|-------|--------|
| Panics | 0 | 0 | 0 | 0 |
| Premature timeouts | 0 | 0 | ≤2 | 0 |
| Timeout conflicts | 0 | 0 | 0 | 0 |
| Chain violations | 0 | 0 | 0 | 0 |
| Leaked contexts | 0 | 0 | 0 | 0 |

---

## Known Issues to Watch For

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| Premature timeout | Operation cut short | Shortest timeout wins | Align timeout chain |
| Timeout conflict | Warning in logs | Parent shorter than child | Fix hierarchy |
| Leaked context | Goroutine leak | Missing defer cancel | Add defer cancel() |
| No retry timeout | Retry uses old deadline | Fresh context not created | Reset deadline on retry |
| Chain broken | Child ignores parent | Context not passed | Pass ctx to children |

---

## Timeout Hierarchy Reference

```
Campaign Phase (30 min)
└── OODA Loop (30 min)
    ├── Shard Execution (20 min)
    │   ├── API Call (10 min)
    │   │   └── HTTP Client (10 min)
    │   └── Articulation (5 min)
    ├── Document Processing (20 min)
    └── Ouroboros (10 min)
```

---

## Related Files

- [api-scheduler-stress.md](../01-kernel-core/api-scheduler-stress.md) - API scheduling
- [llm-provider-system.md](llm-provider-system.md) - Provider configuration
- [campaign-marathon.md](../03-shards-campaigns/campaign-marathon.md) - Long campaigns
