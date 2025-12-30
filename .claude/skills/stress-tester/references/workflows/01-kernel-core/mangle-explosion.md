# Mangle Derivation Explosion Stress Test

Stress test for Mangle kernel derivation limits and gas consumption.

## Overview

This test pushes the Mangle Datalog engine to its limits with:

- Cyclic rules that cause derivation explosion
- Large fact sets (approaching 250k limit)
- Complex recursive queries
- Gas limit enforcement (100k derived facts)

**Expected Duration:** 20-40 minutes total

## Prerequisites

```bash
# Build codeNERD
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd

# Clear logs
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue

# Verify kernel boots
./nerd.exe status
```

---

## Conservative Test (5-10 min)

Test normal derivation within safe limits.

### Step 1: Verify Kernel State (wait 30s)

```bash
./nerd.exe query "system_boot"
./nerd.exe logic
```

### Step 2: Simple Queries (wait 2 min)

```bash
./nerd.exe query "user_intent"
./nerd.exe query "shard_executed"
./nerd.exe query "permitted"
```

### Step 3: Add Facts via Task (wait 5 min)

```bash
./nerd.exe run "analyze the main.go file and remember its structure"
```

Wait 3 minutes.

```bash
./nerd.exe query "file_topology"
```

### Expected Results

- All queries return within seconds
- No gas limit warnings
- Facts properly stored

### Success Criteria

- [ ] Queries completed quickly
- [ ] No kernel errors in logs
- [ ] Facts persisted correctly

---

## Aggressive Test (10-15 min)

Push derivation with complex queries.

### Step 1: Load Many Facts (wait 5 min)

```bash
./nerd.exe scan
./nerd.exe run "analyze all Go files in the internal directory"
```

Wait 4 minutes.

### Step 2: Complex Queries (wait 5 min)

```bash
# Query transitive relationships
./nerd.exe query "reachable"
./nerd.exe query "data_flow_sink"

# Query derived predicates
./nerd.exe why "next_action"
```

### Step 3: Check Kernel Health (wait 2 min)

```bash
./nerd.exe status
./nerd.exe logic | Select-Object -First 100
```

### Expected Results

- Some queries may be slow (seconds)
- No gas limit exceeded
- All queries complete

### Success Criteria

- [ ] All queries completed
- [ ] No timeout errors
- [ ] Kernel remained stable

---

## Chaos Test (15-20 min)

Attempt to trigger derivation explosion.

### Step 1: Load Cyclic Rules (wait 5 min)

```bash
# Check the cyclic rules asset
Get-Content .claude/skills/stress-tester/assets/cyclic_rules.mg | Select-Object -First 20

# Attempt to validate (should work but may warn)
./nerd.exe check-mangle .claude/skills/stress-tester/assets/cyclic_rules.mg
```

### Step 2: Trigger Explosive Derivation (wait 10 min)

Create a task that triggers many derivations:

```bash
./nerd.exe run "analyze all dependencies and create a full dependency graph showing all transitive relationships between all packages"
```

Wait 8 minutes. This should stress the derivation engine.

### Step 3: Monitor Gas Consumption (wait 3 min)

```bash
# Check for gas limit warnings
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "gas|limit|exceeded"

# Check derived fact count
./nerd.exe query "derived_fact" 2>$null | Measure-Object -Line
```

### Step 4: Verify Recovery (wait 2 min)

```bash
./nerd.exe status
./nerd.exe query "system_boot"
```

### Expected Results

- Gas limit warnings possible
- Some queries may timeout or be truncated
- System should NOT panic
- Kernel should recover

### Success Criteria

- [ ] No kernel panic
- [ ] Gas limit enforced (if hit)
- [ ] System remained stable
- [ ] Recovery successful

---

## Hybrid Test (20-30 min)

Combine kernel stress with shard execution.

### Step 1: Campaign with Heavy Analysis (wait 20 min)

```bash
./nerd.exe campaign start "analyze the entire codebase, build a complete call graph, identify all security issues, and create a comprehensive report"
```

Wait 15 minutes. This campaign will:

- Scan many files (world subsystem)
- Run many queries (kernel)
- Spawn multiple shards (queue)
- Store many facts (kernel)

### Step 2: Add Concurrent Queries (wait 5 min)

While campaign runs:

```bash
./nerd.exe query "file_topology"
./nerd.exe query "symbol_graph"
./nerd.exe why "permitted"
```

### Step 3: Verify System State (wait 5 min)

```bash
./nerd.exe campaign status
./nerd.exe status
./nerd.exe logic | Measure-Object -Line
```

### Expected Results

- Campaign makes progress
- Queries complete (may be slow)
- Fact count grows significantly
- System remains stable

### Success Criteria

- [ ] Campaign progressed
- [ ] No kernel crashes
- [ ] Queries completed (even if slow)
- [ ] System recovered

---

## Post-Test Analysis

### Log Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

Check specifically for:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "error|panic|gas|timeout"
```

### Success Criteria

- [ ] No kernel panics
- [ ] Gas limit warnings logged (Chaos test only)
- [ ] No infinite loops detected
- [ ] Derivation completed or safely terminated

### Known Issues to Watch For

- `CRITICAL: Kernel failed to boot` - Fatal, indicates corrupted state
- `gas limit exceeded` - Expected in Chaos test, derivation safely stopped
- `timeout` - Query took too long, may indicate explosion
- `undeclared predicate` - Schema issue, should not occur
