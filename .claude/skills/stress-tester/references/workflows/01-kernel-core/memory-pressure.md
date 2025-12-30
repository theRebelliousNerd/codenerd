# Memory Pressure Stress Test

Stress test for memory limits and emergency compression.

## Overview

This test pushes memory usage to trigger:

- LimitsEnforcer memory checks
- Emergency context compression
- Fact store saturation
- Memory warnings and recovery

**Expected Duration:** 25-45 minutes total

## Prerequisites

```bash
# Build codeNERD
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd

# Clear logs
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue

# Check initial memory baseline
Get-Process nerd -ErrorAction SilentlyContinue | Select-Object WorkingSet64
```

---

## Conservative Test (5-10 min)

Normal memory usage patterns.

### Step 1: Baseline Memory (wait 1 min)

```bash
./nerd.exe status
```

Note initial memory usage.

### Step 2: Normal Operations (wait 5 min)

```bash
./nerd.exe run "explain what the main.go file does"
./nerd.exe run "summarize the README"
```

Wait 3 minutes.

### Step 3: Check Memory Growth (wait 2 min)

```bash
Get-Process nerd -ErrorAction SilentlyContinue | Select-Object WorkingSet64
./nerd.exe status
```

### Expected Results

- Memory usage stays under 500MB
- No compression triggered
- Normal operation

### Success Criteria

- [ ] Memory under 1GB
- [ ] No memory warnings
- [ ] Operations completed

---

## Aggressive Test (10-15 min)

Push memory with large contexts.

### Step 1: Large File Analysis (wait 5 min)

```bash
./nerd.exe run "read and analyze all Go files in internal/core and explain every function"
```

Wait 4 minutes.

### Step 2: Multi-Turn Conversation (wait 5 min)

Keep adding to history:

```bash
./nerd.exe run "now explain all the types and interfaces"
./nerd.exe run "list all the dependencies between files"
./nerd.exe run "create a summary document of everything"
```

Wait 3 minutes between each.

### Step 3: Check Memory State (wait 2 min)

```bash
Get-Process nerd -ErrorAction SilentlyContinue | Select-Object WorkingSet64
Select-String -Path ".nerd/logs/*.log" -Pattern "memory|compress"
```

### Expected Results

- Memory may reach 1-2GB
- Compression may trigger at 80% threshold
- Operations complete with possible delays

### Success Criteria

- [ ] Memory under config limit (12GB)
- [ ] Compression triggered if threshold hit
- [ ] No OOM errors

---

## Chaos Test (15-20 min)

Attempt to exhaust memory.

### Step 1: Generate Large Content (wait 10 min)

```bash
./nerd.exe run "generate a complete Go application with 50 files, each containing 10 functions with full documentation and tests"
```

Wait 8 minutes. This generates significant output.

### Step 2: Add More Pressure (wait 5 min)

```bash
./nerd.exe run "now review all the generated code and suggest improvements for each file"
./nerd.exe run "generate test coverage reports for everything"
```

### Step 3: Monitor Memory (wait 3 min)

```bash
# Check memory
Get-Process nerd -ErrorAction SilentlyContinue | Select-Object WorkingSet64

# Check for warnings
Select-String -Path ".nerd/logs/*.log" -Pattern "memory|limit|exceeded|compress|emergency"
```

### Step 4: Recovery (wait 2 min)

```bash
./nerd.exe /new-session
./nerd.exe status
```

### Expected Results

- Memory warnings should appear
- Emergency compression may trigger
- System should NOT OOM
- Recovery should work

### Success Criteria

- [ ] No OOM panic
- [ ] Memory warnings logged
- [ ] System recovered
- [ ] New session started successfully

---

## Hybrid Test (20-30 min)

Combine memory pressure with other stress.

### Step 1: Large Campaign (wait 20 min)

```bash
./nerd.exe campaign start "implement a complete microservices architecture with 5 services, each with its own database schema, API endpoints, tests, and documentation"
```

This creates:

- Many facts (kernel memory)
- Large context (LLM memory)
- Multiple shards (working memory)

Wait 15 minutes.

### Step 2: Parallel Operations (wait 5 min)

While campaign runs:

```bash
./nerd.exe spawn researcher "research best practices for microservices"
./nerd.exe tool generate "a tool that validates microservice contracts"
```

### Step 3: Final Check (wait 5 min)

```bash
./nerd.exe campaign status
Get-Process nerd -ErrorAction SilentlyContinue | Select-Object WorkingSet64
./nerd.exe status
```

### Expected Results

- Campaign progresses (may be slow)
- Memory stays manageable
- Compression helps maintain stability

### Success Criteria

- [ ] No crashes
- [ ] Campaign made progress
- [ ] Memory didn't exceed limits
- [ ] System recovered

---

## Post-Test Analysis

### Log Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

Specific memory checks:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "memory|compress|limit|OOM"
```

### Success Criteria

- [ ] No OOM panics
- [ ] Memory warnings logged appropriately
- [ ] Compression triggered when needed
- [ ] System remained stable

### Known Issues to Watch For

- `out of memory` - Critical, should not occur with limits
- `memory limit exceeded` - Warning, triggers compression
- `emergency compression` - Expected under extreme load
- `context truncated` - Normal behavior at limits
