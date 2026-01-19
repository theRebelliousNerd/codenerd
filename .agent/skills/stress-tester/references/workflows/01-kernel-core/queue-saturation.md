# Queue Saturation Stress Test

Stress test for SpawnQueue backpressure and shard concurrency limits.

## Overview

This test pushes the SpawnQueue to its limits by rapidly submitting spawn requests, testing:

- Queue capacity (max 100 requests)
- Backpressure at 70% utilization
- Priority handling under load
- Concurrent shard limits (max 4)
- Timeout and deadline handling

**Expected Duration:** 20-40 minutes total (all severity levels)

## Prerequisites

```bash
# Build codeNERD
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd

# Clear previous logs
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue

# Verify clean state
./nerd.exe status
```

---

## Conservative Test (5-10 min)

Test normal queue operation within limits.

### Step 1: Verify Initial State (wait 30s)

```bash
./nerd.exe status
./nerd.exe agents
```

Verify: No active shards, queue empty.

### Step 2: Sequential Spawning (wait 3 min)

Spawn shards one at a time, waiting for completion:

```bash
./nerd.exe spawn coder "write a simple hello world function"
```

Wait 60 seconds for completion.

```bash
./nerd.exe spawn tester "test the hello world function"
```

Wait 60 seconds for completion.

```bash
./nerd.exe spawn reviewer "review the hello world code"
```

Wait 60 seconds for completion.

### Step 3: Verify Queue Behavior (wait 1 min)

```bash
./nerd.exe query "shard_executed"
./nerd.exe status
```

### Expected Results

- All shards complete successfully
- No queue warnings in logs
- `shard_executed` facts present for each shard
- Queue never exceeded 1 item

### Success Criteria

- [ ] All 3 shards completed
- [ ] No errors in `.nerd/logs/shards*.log`
- [ ] No queue full warnings

---

## Aggressive Test (10-15 min)

Push queue to near-capacity.

### Step 1: Rapid Sequential Spawning (wait 5 min)

Spawn multiple shards in rapid succession (still respecting concurrency limit):

```bash
./nerd.exe spawn coder "create file handler.go with CRUD operations"
./nerd.exe spawn coder "create file service.go with business logic"
./nerd.exe spawn coder "create file model.go with data structures"
./nerd.exe spawn coder "create file utils.go with helper functions"
```

Wait 3 minutes for all to complete or queue.

### Step 2: Monitor Queue State (wait 2 min)

```bash
./nerd.exe status
./nerd.exe query "shard_executed"
```

Check logs for queue behavior:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "queue"
```

### Step 3: Add More Load (wait 5 min)

```bash
./nerd.exe spawn tester "test all the new files"
./nerd.exe spawn reviewer "review all the new code"
./nerd.exe spawn coder "add error handling to all files"
./nerd.exe spawn coder "add logging to all files"
```

Wait 4 minutes.

### Expected Results

- Some shards may queue (>4 concurrent)
- Backpressure warnings possible at 70% queue
- All shards eventually complete
- Priority order respected

### Success Criteria

- [ ] All shards eventually completed
- [ ] Queue warnings (if any) handled gracefully
- [ ] No ErrQueueFull errors
- [ ] No panics

---

## Chaos Test (15-20 min)

Attempt to overflow the queue.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
```

### Step 2: Mass Spawn Attempt (wait 10 min)

Attempt to spawn many shards rapidly. Note: This should hit queue limits.

```bash
# Spawn 20 coders in rapid succession
for ($i=1; $i -le 20; $i++) {
    Start-Process -NoNewWindow -FilePath "./nerd.exe" -ArgumentList "spawn","coder","task number $i"
    Start-Sleep -Milliseconds 100
}
```

Wait 8 minutes for queue processing.

### Step 3: Monitor Failures (wait 2 min)

```bash
# Check for queue full errors
Select-String -Path ".nerd/logs/*.log" -Pattern "queue full|ErrQueueFull"

# Check shard completion count
./nerd.exe query "shard_executed"
```

### Step 4: Recovery Test (wait 5 min)

After queue drains, verify system recovers:

```bash
./nerd.exe status
./nerd.exe spawn coder "simple recovery test"
```

Wait 2 minutes.

### Expected Results

- Many spawn requests should fail with ErrQueueFull
- System should NOT panic
- Recovery should work after queue drains
- Some shards should complete successfully

### Success Criteria

- [ ] System did not crash
- [ ] ErrQueueFull errors logged (expected)
- [ ] System recovered after queue drained
- [ ] No panics or deadlocks

---

## Hybrid Test (20-30 min)

Combine queue stress with other subsystems.

### Step 1: Start Campaign Under Queue Load (wait 15 min)

```bash
# Start a campaign (uses queue internally)
./nerd.exe campaign start "create a REST API with user authentication"
```

While campaign is running (in separate terminal):

```bash
# Add additional queue pressure
./nerd.exe spawn reviewer "review current progress"
./nerd.exe spawn tester "test current implementation"
```

Wait 12 minutes.

### Step 2: Add Tool Generation (wait 10 min)

```bash
# Tool generation also uses queue
./nerd.exe tool generate "a tool that validates JSON schemas"
./nerd.exe tool generate "a tool that formats Go code"
```

Wait 8 minutes.

### Step 3: Verify Integration (wait 5 min)

```bash
./nerd.exe campaign status
./nerd.exe tool list
./nerd.exe query "shard_executed"
```

### Expected Results

- Campaign phases may be delayed by queue pressure
- Tool generation may queue behind campaign tasks
- System should remain stable
- All operations eventually complete

### Success Criteria

- [ ] Campaign made progress (didn't stall)
- [ ] Tools generated (even if delayed)
- [ ] No deadlocks between campaign and standalone spawns
- [ ] System remained responsive

---

## Post-Test Analysis

### Log Analysis (via log-analyzer skill)

```bash
cd .claude/skills/log-analyzer/scripts
python parse_log.py ../../../.nerd/logs/* --no-schema | Select-String "^log_entry" > /tmp/queue_stress.mg
cd logquery
./logquery.exe /tmp/queue_stress.mg --builtin shard-errors
./logquery.exe /tmp/queue_stress.mg -i
# Run: ?queue_full
# Run: ?shard_spawned
# Run: ?shard_completed
```

Or use the integrated analyzer:

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Success Criteria

- [ ] No panics in logs
- [ ] Queue full errors only in Chaos test (expected)
- [ ] All Conservative/Aggressive shards completed
- [ ] System recovered after Chaos test

### Known Issues to Watch For

- `ErrQueueFull` - Expected in Chaos test, unexpected in Conservative/Aggressive
- `deadline exceeded` - Spawn request timed out waiting in queue
- `priority inversion` - Low-priority tasks blocked indefinitely
- Orphaned shards - Shards that started but never completed
