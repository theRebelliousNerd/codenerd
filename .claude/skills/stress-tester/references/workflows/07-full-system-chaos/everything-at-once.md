# Everything At Once Stress Test

Maximum stress test running all subsystems simultaneously.

## Overview

The ultimate stress test that exercises:

- All shard types concurrently
- Campaign orchestration
- Tool generation (Ouroboros)
- Kernel derivation
- Memory management
- Queue management
- Context compression

**Expected Duration:** 60-120 minutes total

**WARNING:** This test will consume significant resources. Ensure adequate memory (8GB+ free) and disk space (5GB+ free).

---

## Conservative Test (15-20 min)

Light multi-system load.

### Step 1: Start Multiple Operations (wait 15 min)

In sequence (not parallel for conservative):

```bash
# 1. Start a small campaign
./nerd.exe campaign start "add a health check endpoint"

# Wait 2 minutes
Start-Sleep -Seconds 120

# 2. Generate a tool
./nerd.exe tool generate "a tool that checks HTTP endpoint health"

# Wait 2 minutes
Start-Sleep -Seconds 120

# 3. Spawn a reviewer
./nerd.exe spawn reviewer "review the health check implementation"

# Wait 2 minutes
Start-Sleep -Seconds 120

# 4. Query kernel
./nerd.exe query "shard_executed"
./nerd.exe query "campaign_phase"
```

### Step 2: Verify All Completed (wait 5 min)

```bash
./nerd.exe status
./nerd.exe campaign status
./nerd.exe tool list
./nerd.exe query "shard_executed"
```

### Success Criteria

- [ ] Campaign completed
- [ ] Tool generated
- [ ] Review completed
- [ ] All queries returned

---

## Aggressive Test (30-45 min)

Concurrent multi-system stress.

### Step 1: Launch All Systems (wait 30 min)

Start all operations as close together as possible:

```bash
# Start campaign (runs in background)
Start-Process -NoNewWindow -FilePath "./nerd.exe" -ArgumentList "campaign","start","implement user profile management with avatar upload"

Start-Sleep -Seconds 5

# Generate multiple tools
Start-Process -NoNewWindow -FilePath "./nerd.exe" -ArgumentList "tool","generate","a tool that resizes images"
Start-Process -NoNewWindow -FilePath "./nerd.exe" -ArgumentList "tool","generate","a tool that validates file types"

Start-Sleep -Seconds 5

# Spawn multiple shards
./nerd.exe spawn coder "create utility functions"
./nerd.exe spawn tester "write integration tests"
./nerd.exe spawn reviewer "review security aspects"
```

Wait 25 minutes.

### Step 2: Add Query Load (wait 5 min)

While above is running:

```bash
for ($i=1; $i -le 10; $i++) {
    ./nerd.exe query "shard_executed"
    Start-Sleep -Seconds 30
}
```

### Step 3: Monitor Resources (wait 5 min)

```bash
Get-Process nerd -ErrorAction SilentlyContinue | Select-Object WorkingSet64, CPU
./nerd.exe status
```

### Success Criteria

- [ ] Multiple systems ran concurrently
- [ ] No deadlocks
- [ ] Memory stayed within limits
- [ ] Most operations completed

---

## Chaos Test (45-60 min)

Maximum simultaneous load.

### Step 1: Saturate All Subsystems (wait 45 min)

```powershell
# Campaign
Start-Process -NoNewWindow -FilePath "./nerd.exe" -ArgumentList "campaign","start","build a complete blog platform with posts, comments, likes, follows, notifications, and admin panel"

# Wait for campaign to start
Start-Sleep -Seconds 10

# Tool generation storm
1..5 | ForEach-Object {
    Start-Process -NoNewWindow -FilePath "./nerd.exe" -ArgumentList "tool","generate","a tool that performs task number $_"
}

# Shard storm
1..10 | ForEach-Object {
    Start-Process -NoNewWindow -FilePath "./nerd.exe" -ArgumentList "spawn","coder","task $_"
}

# Query storm
1..20 | ForEach-Object {
    Start-Job -ScriptBlock {
        & ./nerd.exe query "shard_executed"
    }
}
```

Wait 40 minutes.

### Step 2: Monitor for Failures (wait 10 min)

```bash
# Check for errors
Select-String -Path ".nerd/logs/*.log" -Pattern "panic|fatal|OOM|deadlock"

# Check resource usage
Get-Process nerd -ErrorAction SilentlyContinue | Select-Object WorkingSet64, CPU

# Check system state
./nerd.exe status
```

### Step 3: Verify Recovery (wait 5 min)

```bash
./nerd.exe /new-session
./nerd.exe status
./nerd.exe run "simple test"
```

### Success Criteria

- [ ] System did not crash completely
- [ ] Some operations succeeded
- [ ] Queue handled overflow gracefully
- [ ] System recovered after load

---

## Hybrid Test (60-90 min)

Extended multi-system marathon.

### Step 1: Long-Running Combined Load (wait 60 min)

```bash
# Start a large campaign
./nerd.exe campaign start "build a complete project management application with tasks, projects, teams, deadlines, notifications, reports, and integrations"
```

Every 10 minutes while campaign runs:

```bash
# Add tool generation
./nerd.exe tool generate "a utility tool for the project"

# Add shard work
./nerd.exe spawn reviewer "review current implementation"

# Query kernel
./nerd.exe query "campaign_phase"
./nerd.exe query "shard_executed"
```

### Step 2: Dream and Shadow (wait 15 min)

```bash
./nerd.exe dream "what if we used a different architecture"
./nerd.exe shadow "refactor everything to use microservices"
```

### Step 3: Final Verification (wait 10 min)

```bash
./nerd.exe campaign status
./nerd.exe tool list
./nerd.exe status
./nerd.exe query "shard_executed" | Measure-Object -Line
```

### Success Criteria

- [ ] Campaign made significant progress
- [ ] Multiple tools generated
- [ ] Dream/shadow completed
- [ ] System remained functional throughout

---

## Post-Test Analysis

### Comprehensive Log Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose --output chaos_report.md
```

### Resource Analysis

```bash
# Check peak memory
Select-String -Path ".nerd/logs/*.log" -Pattern "memory|MB|GB"

# Check queue behavior
Select-String -Path ".nerd/logs/*.log" -Pattern "queue|backpressure"

# Check for any crashes
Select-String -Path ".nerd/logs/*.log" -Pattern "panic|fatal|crash|OOM"
```

### Database Integrity

```bash
# Check SQLite databases
Get-ChildItem .nerd/*.db | ForEach-Object {
    Write-Host "Checking $_"
    sqlite3 $_.FullName "PRAGMA integrity_check;"
}
```

### Success Criteria Summary

- [ ] No complete system crash
- [ ] No data corruption
- [ ] Memory stayed within limits
- [ ] Campaign made progress
- [ ] Tools were generated
- [ ] System recovered after test

### Known Issues

- `queue full` - Expected under extreme load
- `memory warning` - Expected, should trigger compression
- `timeout` - Expected for some operations
- `shard limit` - Expected, operations queued
- `gas limit` - Expected for complex derivations

### Recovery Commands

If system is in bad state:

```bash
# Force new session
./nerd.exe /new-session

# Clear caches
Remove-Item .nerd/cache/* -Recurse -Force

# Reinitialize
./nerd.exe init --force
```
