# Tool Generation Nesting Stress Test

Stress test for Ouroboros self-generation loop.

## Overview

Tests the tool generation system's handling of:

- Recursive tool generation (tools that generate tools)
- Safety checker validation
- Compilation and execution
- Stagnation detection (Halting Oracle)

**Expected Duration:** 20-40 minutes total

---

## Conservative Test (5-10 min)

Simple tool generation.

### Step 1: Generate Simple Tool (wait 5 min)

```bash
./nerd.exe tool generate "a tool that counts lines in a file"
```

Wait 4 minutes.

### Step 2: Verify Tool (wait 2 min)

```bash
./nerd.exe tool list
./nerd.exe tool info line_counter  # or whatever name was generated
```

### Step 3: Test Tool (wait 2 min)

```bash
./nerd.exe tool run line_counter "main.go"
```

### Success Criteria

- [ ] Tool generated successfully
- [ ] Tool compiled
- [ ] Tool executed correctly

---

## Aggressive Test (10-15 min)

Complex tool generation.

### Step 1: Generate Complex Tool (wait 8 min)

```bash
./nerd.exe tool generate "a tool that analyzes Go code structure and reports function complexity metrics including cyclomatic complexity and lines of code"
```

Wait 6 minutes.

### Step 2: Verify Compilation (wait 3 min)

```bash
./nerd.exe tool list
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "compil|error|warn"
```

### Step 3: Test Execution (wait 3 min)

```bash
./nerd.exe tool run complexity_analyzer "internal/core/kernel.go"
```

### Success Criteria

- [ ] Complex tool generated
- [ ] Safety checks passed
- [ ] Tool executed without panic

---

## Chaos Test (15-20 min)

Recursive tool generation (tools generating tools).

### Step 1: Generate Meta-Tool (wait 10 min)

```bash
./nerd.exe tool generate "a tool that generates other tools based on a specification file"
```

This tests:

- Self-referential generation
- Stagnation detection
- Nesting limits

Wait 8 minutes.

### Step 2: Check for Infinite Loop Protection (wait 3 min)

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "stagnation|loop|halt|nest"
```

### Step 3: Attempt Forbidden Generation (wait 5 min)

```bash
# Try to generate tool with forbidden imports (should fail safety check)
./nerd.exe tool generate "a tool that uses unsafe pointer operations to directly manipulate memory"
```

### Step 4: Verify Safety (wait 2 min)

```bash
./nerd.exe tool list
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "forbidden|unsafe|blocked"
```

### Success Criteria

- [ ] No infinite generation loops
- [ ] Stagnation detected if applicable
- [ ] Forbidden imports blocked
- [ ] System remained stable

---

## Hybrid Test (15-20 min)

Tool generation during active operations.

### Step 1: Start Campaign (wait 5 min)

```bash
./nerd.exe campaign start "create a data validation library"
```

### Step 2: Generate Tools Concurrently (wait 10 min)

```bash
./nerd.exe tool generate "a tool that validates JSON against a schema"
./nerd.exe tool generate "a tool that validates email addresses"
./nerd.exe tool generate "a tool that validates phone numbers"
```

### Step 3: Verify Coexistence (wait 5 min)

```bash
./nerd.exe campaign status
./nerd.exe tool list
```

### Success Criteria

- [ ] Campaign continued during generation
- [ ] Multiple tools generated
- [ ] No resource conflicts

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py

# Check tool generation logs specifically
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "error|panic|fail|forbidden"
```

### Tool Registry Check

```bash
Get-ChildItem .nerd/tools/.compiled/
Get-Content .nerd/tools/.traces/reasoning_traces.json | Select-Object -First 50
```

### Known Issues

- `stagnation detected` - Tool generation not making progress (expected)
- `forbidden import` - Safety checker blocked unsafe code (expected)
- `compile error` - Generated code has syntax errors
- `execution panic` - Tool crashed during run
