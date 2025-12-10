# Intent Fuzzing Stress Test

Stress test for perception transducer with malformed and edge-case inputs.

## Overview

Tests the transducer's robustness against:

- Malformed natural language inputs
- Unicode edge cases
- Injection attempts
- Ambiguous intents
- Empty and extreme-length inputs

**Expected Duration:** 15-30 minutes total

---

## Conservative Test (5-10 min)

Clear, well-formed inputs.

### Step 1: Standard Intents (wait 3 min)

```bash
./nerd.exe perception "review the code"
./nerd.exe perception "fix the bug in main.go"
./nerd.exe perception "test all functions"
./nerd.exe perception "explain how the kernel works"
```

### Step 2: Verify Classification (wait 2 min)

Each should show:

- Extracted verb
- Target identification
- Confidence score
- Shard routing

### Success Criteria

- [ ] All intents classified
- [ ] Correct verb extraction
- [ ] High confidence scores

---

## Aggressive Test (5-10 min)

Edge cases and unusual inputs.

### Step 1: Generate Fuzzing Inputs (wait 1 min)

```bash
cd .claude/skills/stress-tester/scripts/fixtures
python malformed_inputs.py perception --count 20 --output perception_fuzz.json
```

### Step 2: Test Edge Cases (wait 5 min)

```bash
./nerd.exe perception ""
./nerd.exe perception "   "
./nerd.exe perception "maybe do something possibly"
./nerd.exe perception "don't review but also review"
./nerd.exe perception "review test fix refactor deploy"
```

### Step 3: Unicode Tests (wait 3 min)

```bash
./nerd.exe perception "review 这个代码"
./nerd.exe perception "🔥 fix the bug 🐛"
./nerd.exe perception "test مرحبا"
```

### Success Criteria

- [ ] No crashes on edge inputs
- [ ] Fallback handling works
- [ ] Unicode handled gracefully

---

## Chaos Test (10-15 min)

Injection and extreme inputs.

### Step 1: Injection Attempts (wait 5 min)

```bash
./nerd.exe perception "'; DROP TABLE users; --"
./nerd.exe perception "$(rm -rf /)"
./nerd.exe perception "<script>alert(1)</script>"
./nerd.exe perception "{{7*7}}"
./nerd.exe perception "../../../../etc/passwd"
```

### Step 2: Extreme Lengths (wait 5 min)

```bash
# Very long input
$longInput = "a" * 10000
./nerd.exe perception $longInput

# Many words
$manyWords = ("word " * 1000).Trim()
./nerd.exe perception $manyWords
```

### Step 3: Binary/Control Characters (wait 3 min)

```bash
./nerd.exe perception ([char]0 + "test")
./nerd.exe perception "test`0null`0bytes"
```

### Success Criteria

- [ ] No code execution
- [ ] No crashes
- [ ] Graceful error handling

---

## Hybrid Test (10-15 min)

Combine perception stress with execution.

### Step 1: Fuzzy Input → Execution (wait 10 min)

```bash
./nerd.exe run "maybe fix something if you want"
./nerd.exe run "review/test/fix all the things"
./nerd.exe run "do whatever seems appropriate for main.go"
```

### Step 2: Verify Behavior (wait 3 min)

```bash
./nerd.exe query "user_intent"
./nerd.exe query "shard_executed"
```

### Success Criteria

- [ ] System interpreted ambiguous input reasonably
- [ ] Some action was taken (or clarification requested)
- [ ] No unexpected behavior

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py

Select-String -Path ".nerd/logs/*perception*.log" -Pattern "error|panic|fail"
```

### Known Issues

- `parse failure` - Expected for malformed input
- `fallback verb` - Expected for ambiguous input
- `classification timeout` - LLM response too slow
