# Prompt Evolution System Stress Test

Stress test for the System Prompt Learning (SPL) pipeline.

## Overview

Tests the Prompt Evolution System's handling of:

- Execution feedback collection and storage
- LLM-as-Judge evaluation with error categorization
- Strategy database updates and retrieval
- Automatic atom generation from failure patterns
- Evolved atom promotion to JIT corpus
- Meta-prompt generation for improvements

**Expected Duration:** 30-50 minutes total

### Key Files

- `internal/autopoiesis/prompt_evolution/evolver.go` - Main orchestrator
- `internal/autopoiesis/prompt_evolution/judge.go` - LLM-as-Judge
- `internal/autopoiesis/prompt_evolution/feedback_collector.go` - Execution recording
- `internal/autopoiesis/prompt_evolution/strategy_store.go` - Strategy database
- `internal/autopoiesis/prompt_evolution/atom_generator.go` - Atom creation
- `internal/autopoiesis/prompt_evolution/classifier.go` - Problem classification

### Storage Locations

```
.nerd/prompts/
├── evolution.db        # Execution records, verdicts
├── strategies.db       # Strategy database
└── evolved/
    ├── pending/        # Awaiting promotion
    ├── promoted/       # Promoted to corpus
    └── rejected/       # User rejected
```

---

## Conservative Test (10-15 min)

Test basic feedback collection and evaluation.

### Step 1: Verify Storage Initialized (wait 2 min)

```bash
./nerd.exe status
```

Check for prompt evolution initialization:

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "PromptEvolution|FeedbackCollector"
```

Verify databases exist:

```bash
Get-ChildItem .nerd/prompts/*.db
```

### Step 2: Generate Feedback Record (wait 5 min)

Execute a task that generates feedback:

```bash
./nerd.exe spawn coder "create a function to calculate factorial"
```

Check feedback was recorded:

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "ExecutionRecord|feedback recorded"
```

### Step 3: Verify Judge Evaluation (wait 4 min)

Check LLM-as-Judge evaluation:

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "JudgeVerdict|CORRECT|LOGIC_ERROR|SYNTAX_ERROR"
```

### Step 4: Check Strategy Database (wait 3 min)

Verify strategies are being tracked:

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "strategy|problem_type"
```

### Success Criteria

- [ ] Feedback collector initialized
- [ ] Execution records stored
- [ ] Judge produced verdict with explanation
- [ ] Error category assigned (if applicable)
- [ ] Strategy database updated

---

## Aggressive Test (12-18 min)

Push evolution system with multiple executions and failures.

### Step 1: Clear Evolution State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/prompts/evolved/pending/* -ErrorAction SilentlyContinue
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Generate Multiple Feedback Records (wait 8 min)

Run several tasks to generate feedback:

```bash
./nerd.exe spawn coder "write a sorting algorithm"
Start-Sleep 60
./nerd.exe spawn coder "implement a binary search tree"
Start-Sleep 60
./nerd.exe spawn coder "create a linked list with insert and delete"
Start-Sleep 60
```

### Step 3: Force Failure Scenarios (wait 5 min)

Create tasks likely to fail for evolution triggers:

```bash
# Ambiguous task (may trigger CONTEXT_MISS)
./nerd.exe spawn coder "fix the bug"

# Impossible task (may trigger INSTRUCTION_MISS)
./nerd.exe spawn coder "use the XYZ library we don't have"
```

### Step 4: Check Atom Generation (wait 3 min)

Verify atoms were generated from failures:

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "atom generated|new atom|evolved"
Get-ChildItem .nerd/prompts/evolved/pending/
```

### Step 5: Verify Error Category Distribution (wait 2 min)

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "LOGIC_ERROR|SYNTAX_ERROR|API_MISUSE|EDGE_CASE|CONTEXT_MISS|INSTRUCTION_MISS|HALLUCINATION"
```

### Success Criteria

- [ ] Multiple execution records created
- [ ] Failures properly categorized
- [ ] Atoms generated for failure patterns
- [ ] Strategy database contains multiple entries
- [ ] No duplicate atoms created

---

## Chaos Test (15-20 min)

Stress test with concurrent evolutions and edge cases.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Concurrent Execution Feedback (wait 7 min)

Generate feedback from multiple shards simultaneously:

```bash
Start-Job { ./nerd.exe spawn coder "task 1: hello world" }
Start-Job { ./nerd.exe spawn coder "task 2: fibonacci" }
Start-Job { ./nerd.exe spawn coder "task 3: prime checker" }
Start-Job { ./nerd.exe spawn coder "task 4: string reversal" }

Get-Job | Wait-Job -Timeout 420
Get-Job | Receive-Job -ErrorAction SilentlyContinue
Get-Job | Remove-Job
```

### Step 3: Database Contention (wait 4 min)

Check for database lock issues:

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "lock|contention|SQLITE_BUSY"
```

### Step 4: Rapid Evolution Cycles (wait 5 min)

Force rapid evolution attempts:

```bash
for ($i=1; $i -le 5; $i++) {
    ./nerd.exe spawn coder "quick task $i"
    Start-Sleep -Seconds 10
}
```

### Step 5: Strategy Store Overflow (wait 3 min)

Check strategy store handles many entries:

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "strategy count|cleanup|prune"
```

### Success Criteria

- [ ] Concurrent feedback recorded without corruption
- [ ] No database deadlocks
- [ ] Rapid cycles handled gracefully
- [ ] Strategy store maintained reasonable size
- [ ] System remained stable

---

## Hybrid Test (15-20 min)

Test integration with JIT compiler and Mangle.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Evolution with JIT Integration (wait 8 min)

Run task that triggers evolution and uses JIT:

```bash
./nerd.exe spawn coder "write a comprehensive error handling utility"
```

Check evolved atoms were considered by JIT:

```bash
Select-String -Path ".nerd/logs/*jit*.log" -Pattern "evolved atom|pending promotion"
```

### Step 3: Mangle Fact Assertion (wait 4 min)

Verify Mangle facts for evolution:

```bash
./nerd.exe query "prompt_evolved"
./nerd.exe query "execution_feedback"
```

### Step 4: Campaign Evolution Tracking (wait 6 min)

Run campaign and track evolution across phases:

```bash
./nerd.exe campaign start "build a validation library"
```

Monitor evolution per phase:

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "campaign_id|phase|evolution"
```

### Step 5: Verify Atom Promotion Pipeline (wait 3 min)

Check promotion workflow:

```bash
Get-ChildItem .nerd/prompts/evolved/pending/ -Recurse
Get-ChildItem .nerd/prompts/evolved/promoted/ -Recurse
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "promot|reject|pending"
```

### Success Criteria

- [ ] Evolved atoms available to JIT compiler
- [ ] Mangle facts reflect evolution state
- [ ] Campaign phases tracked for evolution
- [ ] Promotion pipeline functional
- [ ] No orphaned atoms

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Evolution-Specific Queries

```bash
# Count verdicts by category
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "verdict=" |
    ForEach-Object { $_.Line -match "verdict=(\w+)" | Out-Null; $matches[1] } |
    Group-Object | Format-Table

# Count atoms generated
Get-ChildItem .nerd/prompts/evolved/ -Recurse -File | Measure-Object

# Check for errors
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "error|panic|fail" |
    Where-Object { $_.Line -notmatch "LOGIC_ERROR|SYNTAX_ERROR" }
```

### Strategy Store Analysis

```bash
# Check strategy database size
Get-Item .nerd/prompts/strategies.db | Select-Object Length

# Query strategies (if sqlite3 available)
sqlite3 .nerd/prompts/strategies.db "SELECT problem_type, COUNT(*) FROM strategies GROUP BY problem_type"
```

### Success Metrics

| Metric | Conservative | Aggressive | Chaos | Hybrid |
|--------|--------------|------------|-------|--------|
| Panics | 0 | 0 | 0 | 0 |
| DB locks | 0 | 0 | 0 | 0 |
| Judge failures | 0 | <5% | <10% | <5% |
| Atom generation rate | N/A | >50% failures | >30% | >50% |
| Promotion success | N/A | 100% valid | 100% | 100% |

---

## Known Issues to Watch For

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| Judge timeout | Verdict missing | LLM call timeout | Increase timeout |
| Duplicate atoms | Same atom created twice | Race condition | Check deduplication |
| Strategy explosion | DB grows unbounded | No cleanup | Implement pruning |
| Category mismatch | Wrong error type | Classifier confusion | Improve prompts |
| Promotion stuck | Atoms stay pending | Missing trigger | Check promotion logic |

---

## Error Categories Reference

| Category | Description | Typical Fix |
|----------|-------------|-------------|
| `LOGIC_ERROR` | Wrong approach or algorithm | Strategy improvement |
| `SYNTAX_ERROR` | Code syntax issues | Syntax atom |
| `API_MISUSE` | Wrong API or library usage | API usage atom |
| `EDGE_CASE` | Missing edge case handling | Edge case atom |
| `CONTEXT_MISS` | Missed relevant codebase context | Context selection improvement |
| `INSTRUCTION_MISS` | Didn't follow instructions | Instruction following atom |
| `HALLUCINATION` | Made up information | Hallucination prevention atom |
| `CORRECT` | Task completed correctly | No evolution needed |

---

## Related Files

- [mangle-self-healing.md](../01-kernel-core/mangle-self-healing.md) - Self-healing patterns
- [tool-generation-nesting.md](../04-autopoiesis-ouroboros/tool-generation-nesting.md) - Autopoiesis
