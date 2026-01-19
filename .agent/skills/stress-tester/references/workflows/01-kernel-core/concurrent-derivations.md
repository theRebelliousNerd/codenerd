# Concurrent Derivations Stress Test

Stress test for concurrent kernel queries and thread safety.

## Overview

This test stresses the kernel with concurrent operations:

- Multiple shards querying kernel simultaneously
- Concurrent fact assertions and retractions
- Read/write lock contention
- Race condition detection

**Expected Duration:** 20-35 minutes total

---

## Conservative Test (5-10 min)

Sequential operations to establish baseline.

### Step 1: Sequential Queries (wait 3 min)

```bash
./nerd.exe query "user_intent"
./nerd.exe query "shard_executed"
./nerd.exe query "permitted"
./nerd.exe query "file_topology"
```

### Step 2: Sequential Assertions (wait 3 min)

```bash
./nerd.exe run "note that this is a test session"
./nerd.exe run "remember that we are testing concurrency"
```

### Success Criteria

- [ ] All queries completed
- [ ] No errors

---

## Aggressive Test (10-15 min)

Concurrent shard queries.

### Step 1: Spawn Multiple Shards (wait 8 min)

```bash
./nerd.exe spawn coder "create file1.go"
./nerd.exe spawn tester "test existing code"
./nerd.exe spawn reviewer "review main.go"
./nerd.exe spawn researcher "research Go patterns"
```

All 4 will query kernel concurrently.

Wait 6 minutes.

### Step 2: Verify Integrity (wait 3 min)

```bash
./nerd.exe query "shard_executed"
./nerd.exe status
```

### Success Criteria

- [ ] All shards completed
- [ ] No concurrent map access errors
- [ ] Facts consistent

---

## Chaos Test (10-15 min)

Maximum concurrent load.

### Step 1: Parallel Operations (wait 10 min)

In PowerShell:

```powershell
$jobs = @()
1..10 | ForEach-Object {
    $jobs += Start-Job -ScriptBlock {
        param($n)
        & ./nerd.exe run "task number $n with unique content"
    } -ArgumentList $_
}
$jobs | Wait-Job -Timeout 300
$jobs | Receive-Job
```

### Step 2: Check for Race Conditions (wait 3 min)

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "concurrent|race|deadlock|fatal"
```

### Success Criteria

- [ ] No race condition errors
- [ ] No deadlocks
- [ ] System stable

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py
```

### Known Issues

- `concurrent map` - Race condition in kernel
- `deadlock` - Lock contention
- `fatal error` - Critical race condition
