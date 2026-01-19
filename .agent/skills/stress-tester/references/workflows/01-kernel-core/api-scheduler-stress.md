# API Scheduler Stress Test

Stress test for the APIScheduler cooperative shard scheduling system.

## Overview

The APIScheduler manages API call concurrency independently from shard slot limits:

- **SpawnQueue** limits concurrent shards (12 max)
- **APIScheduler** limits concurrent LLM API calls (5 max)

This separation allows many shards to be "in flight" while preventing API rate limit violations. Shards cooperatively yield their API slot after each LLM call, enabling efficient multiplexing.

### What This Test Stresses

- Slot acquisition/release under load
- Concurrent shard spawning with API limiting
- Context cancellation during slot waits
- Queue saturation scenarios
- Checkpoint save/load under concurrent access
- Metrics accuracy under load
- ScheduledLLMCall wrapper correctness

**Expected Duration:** 25-45 minutes total (all severity levels)

## Architecture Context

```text
SpawnQueue (100 items) --> LimitsEnforcer (12 shards) --> Shard spawns
                                                              |
                                                              v
                                               ScheduledLLMCall.Complete()
                                                              |
                                                              v
                                            APIScheduler.AcquireAPISlot() [5 slots max]
                                                              |
                                                              v
                                                    ZAIClient.Complete() [no semaphore]
                                                              |
                                                              v
                                            APIScheduler.ReleaseAPISlot()
```

### Key Files

- `internal/core/api_scheduler.go` - Scheduler implementation
- `internal/core/api_scheduler_test.go` - Unit tests
- `internal/core/shard_manager.go` - Integration point (lines 1309-1320, 1424-1425)
- `internal/perception/client.go` - ZAIClient with DisableSemaphore

## Prerequisites

```bash
# Build codeNERD
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd

# Run unit tests first
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go test ./internal/core/... -v -run "TestAPIScheduler"

# Clear previous logs
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue

# Verify clean state
./nerd.exe status
```

---

## Conservative Test (5-10 min)

Test normal scheduler operation within limits.

### Step 1: Verify Initial State (wait 30s)

```bash
./nerd.exe status
./nerd.exe agents
```

**Verify:** No active shards, scheduler idle.

### Step 2: Single Shard API Calls (wait 3 min)

Spawn a single shard and monitor slot acquisition:

```bash
./nerd.exe review internal/core/api_scheduler.go
```

While running, check logs for scheduler activity:

```bash
Select-String -Path ".nerd/logs/shards*.log" -Pattern "APIScheduler"
```

**Expected log entries:**
- `APIScheduler: initialized global instance (max_slots=5)`
- `APIScheduler: registered shard reviewer-xxx`
- `APIScheduler: shard reviewer-xxx acquired slot`
- `APIScheduler: shard reviewer-xxx released slot`

Wait 2 minutes for completion.

### Step 3: Verify Metrics (wait 1 min)

Check scheduler metrics after completion:

```bash
./nerd.exe query "shard_executed"
./nerd.exe status
```

Check log metrics:

```bash
Select-String -Path ".nerd/logs/shards*.log" -Pattern "api_calls=|total_wait="
```

### Step 4: Sequential Multi-Shard (wait 4 min)

Spawn multiple shards sequentially to verify slot reuse:

```bash
./nerd.exe review internal/core/kernel.go
```

Wait 90 seconds.

```bash
./nerd.exe review internal/core/limits.go
```

Wait 90 seconds.

### Expected Results

- All shards complete successfully
- Slot acquisition always immediate (5 slots available, 1 shard at a time)
- No "waiting for slot" warnings (no contention)
- Total API calls match actual LLM requests
- All slots released after each shard completes

### Success Criteria

- [ ] APIScheduler initialized on first shard spawn
- [ ] Each shard registered/unregistered properly
- [ ] Metrics show correct TotalAPICalls count
- [ ] No slots left active after test (ActiveSlots = 0)
- [ ] No errors in `.nerd/logs/shards*.log`

---

## Aggressive Test (10-15 min)

Push scheduler toward capacity with concurrent operations.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Concurrent Shards Within Limit (wait 5 min)

Spawn 5 shards simultaneously (matches API slot limit):

```bash
# Open 5 terminals and run simultaneously, or use background jobs:
Start-Job { ./nerd.exe review internal/core/api_scheduler.go }
Start-Job { ./nerd.exe review internal/core/shard_manager.go }
Start-Job { ./nerd.exe review internal/core/kernel.go }
Start-Job { ./nerd.exe review internal/perception/client.go }
Start-Job { ./nerd.exe review internal/core/limits.go }

# Monitor jobs
Get-Job | Format-Table

# Wait for completion
Get-Job | Wait-Job -Timeout 300
Get-Job | Receive-Job
```

During execution, monitor slot contention:

```bash
Select-String -Path ".nerd/logs/shards*.log" -Pattern "waiting for slot|acquired slot after"
```

### Step 3: Verify Slot Distribution (wait 2 min)

```bash
# Check all shards eventually completed
./nerd.exe query "shard_executed"

# Verify metrics
Select-String -Path ".nerd/logs/shards*.log" -Pattern "total_calls=|TotalAPICalls"
```

### Step 4: Exceed Slot Limit (wait 6 min)

Spawn 7 shards to force queuing (7 shards > 5 slots):

```bash
# Rapid spawn - some will wait for slots
for ($i=1; $i -le 7; $i++) {
    Start-Job -ScriptBlock { param($n) & ./nerd.exe review "internal/core/api_scheduler.go" } -ArgumentList $i
    Start-Sleep -Milliseconds 500
}

# Wait and monitor
Start-Sleep 30
Select-String -Path ".nerd/logs/shards*.log" -Pattern "waiting for slot"

# Wait for all
Get-Job | Wait-Job -Timeout 360
Get-Job | Remove-Job
```

### Expected Results

- First 5 shards acquire slots immediately
- Remaining shards queue and wait for slots
- "waiting for slot" messages appear in logs
- "acquired slot after Xms" shows wait duration
- All shards eventually complete
- No deadlocks or timeouts

### Success Criteria

- [ ] 5+ concurrent API calls achieved (logs show `active=5/5`)
- [ ] Waiting shards eventually acquired slots
- [ ] No ErrSlotTimeout errors
- [ ] All shards completed successfully
- [ ] Metrics accurate (TotalAPICalls = sum of all shard calls)

---

## Chaos Test (15-20 min)

Stress test with cancellations, rapid spawning, and edge cases.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Context Cancellation Under Load (wait 5 min)

Spawn shards and cancel them mid-execution:

```bash
# Start a long-running review
$job = Start-Job { ./nerd.exe review internal/core/shard_manager.go }

# Wait for it to acquire slot
Start-Sleep 10

# Check it's active
Select-String -Path ".nerd/logs/shards*.log" -Pattern "acquired slot"

# Cancel it (simulates Ctrl+C)
Stop-Job $job
Remove-Job $job

# Verify slot was released properly
Start-Sleep 5
Select-String -Path ".nerd/logs/shards*.log" -Pattern "released slot|cancelled"
```

Repeat 3 times to stress cancellation handling.

### Step 3: Rapid Spawn/Cancel Cycles (wait 5 min)

Rapid spawn and cancel to stress slot cleanup:

```bash
for ($i=1; $i -le 10; $i++) {
    $job = Start-Job { ./nerd.exe review internal/core/api_scheduler.go }
    Start-Sleep -Seconds 3
    Stop-Job $job -ErrorAction SilentlyContinue
    Remove-Job $job -ErrorAction SilentlyContinue
}

# Verify no orphaned slots
Start-Sleep 10
./nerd.exe status
```

### Step 4: Maximum Contention (wait 7 min)

Spawn 12 shards (max shard limit) all competing for 5 API slots:

```bash
# This creates maximum slot contention
for ($i=1; $i -le 12; $i++) {
    Start-Job -ScriptBlock {
        param($n)
        & ./nerd.exe review "internal/core/api_scheduler.go"
    } -ArgumentList $i
    Start-Sleep -Milliseconds 200
}

# Monitor contention
Start-Sleep 10
Select-String -Path ".nerd/logs/shards*.log" -Pattern "waiting=|active=5/5"

# Wait for all to complete or timeout
Get-Job | Wait-Job -Timeout 420
Get-Job | Receive-Job -ErrorAction SilentlyContinue
Get-Job | Remove-Job
```

### Step 5: Checkpoint Stress (wait 3 min)

If a shard uses checkpoints, verify concurrent access:

```bash
# Run multiple campaign commands that use checkpoints
./nerd.exe campaign start "test checkpoint handling"
Start-Sleep 5
./nerd.exe campaign status

# Verify no checkpoint corruption in logs
Select-String -Path ".nerd/logs/*.log" -Pattern "checkpoint|Checkpoint"
```

### Expected Results

- Cancelled shards release slots promptly
- No orphaned slots after cancellation
- Maximum contention handled without deadlock
- Wait queue cleaned up on cancellation
- Checkpoints survive concurrent access

### Success Criteria

- [ ] System did not crash or panic
- [ ] Cancelled shards released their slots
- [ ] Wait queue size returned to 0 after operations
- [ ] No slot leaks (ActiveSlots = 0 when idle)
- [ ] No "shard released slot it didn't hold" errors
- [ ] Recovery works after chaos

---

## Hybrid Test (20-30 min)

Test scheduler integration with campaigns and multi-phase operations.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Campaign With Concurrent Reviews (wait 15 min)

Start a campaign (spawns multiple shards internally) while also running manual reviews:

```bash
# Start campaign in background
$campaign = Start-Job { ./nerd.exe campaign start "build a simple REST API" }

# Wait for campaign to start spawning shards
Start-Sleep 30

# Add additional concurrent reviews
Start-Job { ./nerd.exe review internal/core/api_scheduler.go }
Start-Job { ./nerd.exe review internal/core/kernel.go }

# Monitor slot distribution
for ($i=1; $i -le 10; $i++) {
    Start-Sleep 30
    Write-Host "=== Check $i ==="
    Select-String -Path ".nerd/logs/shards*.log" -Pattern "active=[0-9]/5|waiting="
}

# Wait for completion
Get-Job | Wait-Job -Timeout 900
Get-Job | Receive-Job -ErrorAction SilentlyContinue
Get-Job | Remove-Job
```

### Step 3: Verify Fair Scheduling (wait 5 min)

Check that campaign shards and manual reviews both made progress:

```bash
./nerd.exe campaign status
./nerd.exe query "shard_executed"

# Check for starvation
Select-String -Path ".nerd/logs/shards*.log" -Pattern "total_wait=" | Sort-Object
```

### Step 4: TDD Loop Integration (wait 10 min)

TDD loops make multiple API calls - verify slot release between retries:

```bash
# Start a test task that may require TDD loop
./nerd.exe test internal/core/api_scheduler_test.go

# Monitor for retry with slot release
Select-String -Path ".nerd/logs/*.log" -Pattern "retry|released slot|acquired slot"
```

### Expected Results

- Campaign and manual reviews interleave fairly
- No single shard monopolizes slots indefinitely
- TDD loops release slots between retries
- Total throughput near optimal (5 concurrent API calls sustained)
- All operations eventually complete

### Success Criteria

- [ ] Campaign made progress (phases advanced)
- [ ] Manual reviews completed
- [ ] No shard starved for slots (all got turns)
- [ ] Average wait time reasonable (<30s typical)
- [ ] System remained responsive throughout

---

## Post-Test Analysis

### Log Analysis (via log-analyzer skill)

```bash
cd .claude/skills/log-analyzer/scripts
python parse_log.py ../../../.nerd/logs/* --no-schema > /tmp/api_sched_stress.mg
cd logquery
./logquery.exe /tmp/api_sched_stress.mg -i
```

In logquery REPL:

```mangle
# Find all slot acquisition events
slot_acquired(T, Shard, M) :-
    log_entry(T, /shards, _, M, _, _),
    fn:contains(M, "acquired slot").

# Find wait events
slot_wait(T, Shard, M) :-
    log_entry(T, /shards, _, M, _, _),
    fn:contains(M, "waiting for slot").

# Find release events
slot_released(T, Shard, M) :-
    log_entry(T, /shards, _, M, _, _),
    fn:contains(M, "released slot").

# Find errors
scheduler_error(T, M) :-
    log_entry(T, /shards, /error, M, _, _),
    fn:contains(M, "APIScheduler").

# Query them
?slot_acquired(T, S, M).
?slot_wait(T, S, M).
?slot_released(T, S, M).
?scheduler_error(T, M).
```

Or use the integrated analyzer:

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Custom Metrics Queries

```bash
# Count total slot acquisitions
Select-String -Path ".nerd/logs/shards*.log" -Pattern "acquired slot" | Measure-Object

# Count waits (indicates contention)
Select-String -Path ".nerd/logs/shards*.log" -Pattern "waiting for slot" | Measure-Object

# Check max concurrent reached
Select-String -Path ".nerd/logs/shards*.log" -Pattern "active=5/5"

# Find longest wait
Select-String -Path ".nerd/logs/shards*.log" -Pattern "acquired slot after" |
    ForEach-Object { $_.Line } | Sort-Object
```

### Success Metrics

| Metric | Conservative | Aggressive | Chaos | Hybrid |
|--------|--------------|------------|-------|--------|
| Panics | 0 | 0 | 0 | 0 |
| Slot leaks | 0 | 0 | 0 | 0 |
| Deadlocks | 0 | 0 | 0 | 0 |
| Max wait time | <1s | <30s | <60s | <30s |
| Completion rate | 100% | 100% | >80% | 100% |

---

## Known Issues to Watch For

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| Slot leak | ActiveSlots > 0 when idle | Missing ReleaseAPISlot | Check defer patterns |
| Double release | "released slot it didn't hold" | Mismatched acquire/release | Review slot lifecycle |
| Wait queue leak | WaitingShards > 0 after cancel | Cleanup not triggered | Check context cancellation |
| Deadlock | All shards blocked | Circular slot dependency | Review acquire order |
| Starvation | Some shards never progress | Unfair scheduling | Consider priority queue |
| Metric drift | TotalAPICalls != expected | Race in counter update | Check atomic operations |

---

## Recovery Procedures

### If Slots Are Leaked

```bash
# Restart nerd to reset scheduler
./nerd.exe /new-session

# Verify clean state
./nerd.exe status
```

### If Deadlocked

```bash
# Force terminate
Get-Process nerd | Stop-Process -Force

# Clear state
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue

# Restart fresh
./nerd.exe status
```

### If Metrics Are Wrong

```bash
# Check unit tests pass
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./internal/core/... -v -run "TestAPIScheduler"

# If tests fail, investigate api_scheduler.go
```

---

## Related Files

- [queue-saturation.md](queue-saturation.md) - SpawnQueue limits (different from API slots)
- [shard-explosion.md](../03-shards-campaigns/shard-explosion.md) - Shard lifecycle stress
- [concurrent-derivations.md](concurrent-derivations.md) - Kernel concurrency

## Test Coverage

This workflow covers:

- `api_scheduler.go`: AcquireAPISlot, ReleaseAPISlot, GetMetrics
- `api_scheduler.go`: RegisterShard, UnregisterShard
- `api_scheduler.go`: SaveCheckpoint, LoadCheckpoint, GetShardState
- `api_scheduler.go`: ScheduledLLMCall.Complete, CompleteWithRetry
- `shard_manager.go`: Integration at SpawnAsyncWithContext
