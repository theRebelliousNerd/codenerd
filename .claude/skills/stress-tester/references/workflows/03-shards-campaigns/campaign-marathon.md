# Campaign Marathon Stress Test

Stress test for long-running multi-phase campaigns.

## Overview

Tests campaign orchestration under extended load:

- Multi-phase decomposition (50+ phases possible)
- Checkpoint/resume functionality
- Context paging across phases
- Shard coordination over time

**Expected Duration:** 30-120 minutes total

---

## Conservative Test (10-15 min)

Small campaign with few phases.

### Step 1: Start Small Campaign (wait 10 min)

```bash
./nerd.exe campaign start "add a logout button to the user interface"
```

This should decompose into 3-5 phases:

- Research existing UI
- Design button placement
- Implement button
- Add tests
- Document changes

Wait 8 minutes.

### Step 2: Monitor Progress (wait 3 min)

```bash
./nerd.exe campaign status
```

### Step 3: Verify Completion (wait 2 min)

```bash
./nerd.exe query "campaign_phase"
./nerd.exe query "shard_executed"
```

### Success Criteria

- [ ] Campaign decomposed correctly
- [ ] All phases executed
- [ ] Checkpoints created

---

## Aggressive Test (30-45 min)

Medium campaign with many phases.

### Step 1: Start Medium Campaign (wait 25 min)

```bash
./nerd.exe campaign start "implement a complete user authentication system with login, logout, password reset, email verification, and session management"
```

Should decompose into 10-20 phases.

Wait 20 minutes.

### Step 2: Pause and Resume (wait 10 min)

```bash
./nerd.exe campaign pause
```

Wait 2 minutes.

```bash
./nerd.exe campaign resume
```

Wait 5 minutes.

### Step 3: Verify State Persistence (wait 3 min)

```bash
./nerd.exe campaign status
Select-String -Path ".nerd/logs/*campaign*.log" -Pattern "checkpoint|resume"
```

### Success Criteria

- [ ] Campaign progressed through multiple phases
- [ ] Pause/resume worked
- [ ] Context maintained across resume

---

## Chaos Test (60-90 min)

Large campaign pushing limits.

### Step 1: Start Large Campaign (wait 60 min)

```bash
./nerd.exe campaign start "implement a complete e-commerce platform with product catalog, shopping cart, checkout flow, payment integration, order management, inventory tracking, user reviews, recommendation engine, admin dashboard, and analytics"
```

This should attempt to create 30-50 phases.

Let it run for 45-60 minutes.

### Step 2: Stress During Campaign (wait 15 min)

While campaign runs:

```bash
./nerd.exe spawn reviewer "review current progress"
./nerd.exe spawn tester "test implemented features"
./nerd.exe query "campaign_phase"
```

### Step 3: Force Interruption (wait 10 min)

```bash
# Ctrl+C the campaign or:
./nerd.exe campaign pause
```

Wait 5 minutes.

```bash
./nerd.exe campaign resume
```

### Step 4: Verify Recovery (wait 5 min)

```bash
./nerd.exe campaign status
./nerd.exe campaign list
```

### Success Criteria

- [ ] Campaign made significant progress
- [ ] Interruption handled gracefully
- [ ] Resume from checkpoint worked
- [ ] No orphaned shards

---

## Hybrid Test (45-60 min)

Campaign with concurrent tool generation.

### Step 1: Start Campaign (wait 30 min)

```bash
./nerd.exe campaign start "build a REST API framework with routing, middleware, authentication, validation, and documentation generation"
```

### Step 2: Generate Tools During Campaign (wait 15 min)

While campaign runs:

```bash
./nerd.exe tool generate "a tool that validates OpenAPI specifications"
./nerd.exe tool generate "a tool that generates API client code"
```

### Step 3: Use Tools in Campaign Context (wait 10 min)

```bash
./nerd.exe run "use the generated tools to validate and document the API"
```

### Success Criteria

- [ ] Campaign and tool generation coexisted
- [ ] Tools were available for use
- [ ] No resource conflicts

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py

Select-String -Path ".nerd/logs/*campaign*.log" -Pattern "error|fail|timeout|checkpoint"
```

### Campaign-Specific Checks

```bash
# Check phase completion
./nerd.exe query "campaign_phase"

# Check for incomplete phases
Select-String -Path ".nerd/logs/*campaign*.log" -Pattern "phase.*fail|abort"

# Check checkpoint integrity
Get-ChildItem .nerd/campaigns/*.db
```

### Known Issues

- `decomposition explosion` - Goal too large, too many phases
- `phase timeout` - Individual phase took too long
- `checkpoint corruption` - Resume from bad state
- `context overflow` - Too much accumulated context
