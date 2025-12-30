# Mangle Repair JIT System Stress Test

Stress test for the JIT-powered Mangle rule repair system (FeedbackLoop + PredicateSelector).

## Overview

This test validates the LLM-powered repair pipeline:

- **FeedbackLoop** - Validation and retry orchestration with budget tracking
- **PredicateSelector** - JIT context-aware predicate injection (~50-100 instead of 799)
- **SelectForRepair** - Error-type-specific predicate selection
- **ValidationBudget** - Per-rule and session-wide retry limits
- **LLM Integration** - Repair prompts with JIT-selected predicates

**Expected Duration:** 15-35 minutes total

## Prerequisites

```bash
# Build codeNERD (ensure corpus and repair shard are included)
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd

# Clear logs
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue

# Verify repair shard registered
./nerd.exe status
```

**Expected:** Status shows "MangleRepairShard registered" and "Predicate corpus loaded".

---

## Conservative Test (5-10 min)

Test basic repair flow with simple fixable errors.

### Step 1: Trigger Simple Syntax Error (wait 3 min)

Create a rule with a simple syntax error that the repair system can fix:

```bash
@"
# Test rule with missing period
bad_syntax(X) :- user_intent(X, _, _, _, _)
"@ | Out-File -Encoding UTF8 /tmp/test_repair.mg

./nerd.exe check-mangle /tmp/test_repair.mg
```

**Expected:** Error about missing period detected.

### Step 2: Verify Corpus Predicates Injected (wait 2 min)

Check logs to confirm repair shard received JIT-selected predicates:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "JIT selected|predicates for repair"
Select-String -Path ".nerd/logs/*system*.log" -Pattern "MangleRepair|repair attempt"
```

**Expected:** Logs show JIT selected ~50-100 predicates, not full 799.

### Step 3: Verify Repair Success (wait 2 min)

Create a rule with atom/string confusion (auto-repairable):

```bash
@"
# Atom should be /active not "active"
state_check(X) :- session(X), status(X, "active").
"@ | Out-File -Encoding UTF8 /tmp/test_atom_string.mg

./nerd.exe check-mangle /tmp/test_atom_string.mg
```

**Expected:** Sanitizer auto-fixes "active" to /active, rule validates.

### Success Criteria

- [ ] Simple errors detected by PreValidator
- [ ] JIT predicates selected (not full corpus)
- [ ] Auto-repair successful for fixable errors
- [ ] Repair logged with attempt count
- [ ] No panics or hangs

---

## Aggressive Test (10-15 min)

Stress repair with multiple error types and budget tracking.

### Step 1: Test 10 Different Error Types (wait 8 min)

Create rules with various error categories:

```bash
@"
# 1. Missing period
rule1(X) :- user_intent(X, _, _, _, _)

# 2. Lowercase variables (Prolog style)
rule2(x, y) :- parent(x, y).

# 3. String instead of atom
rule3(X) :- state(X, "pending").

# 4. Undeclared predicate
rule4(X) :- fake_predicate(X).

# 5. Unsafe negation
rule5(X) :- not dangerous(X).

# 6. SQL-style aggregation
rule6(Sum) :- item(X), Sum = sum(X).

# 7. Prolog negation syntax
rule7(X) :- candidate(X), \+ rejected(X).

# 8. Unsafe head variable
rule8(X) :- other_thing(Y).

# 9. Cartesian product (inefficient but valid)
rule9(X, Y) :- huge_table(X), other_table(Y), X = Y.

# 10. Missing Decl reference
rule10(X) :- unknown_domain_pred(X).
"@ | Out-File -Encoding UTF8 /tmp/test_multi_error.mg

./nerd.exe check-mangle /tmp/test_multi_error.mg
```

Wait 6 minutes for repair attempts.

Check logs:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "error category|classified as"
Select-String -Path ".nerd/logs/*system*.log" -Pattern "repair attempt \d+|attempt \d+ of"
```

**Expected:** Each error type classified correctly, appropriate predicates selected per error.

### Step 2: Verify PredicateSelector.SelectForRepair() (wait 3 min)

Check that different error types triggered different predicate selections:

```bash
# Look for domain-specific selections
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "selected.*for domain|repair context"
```

**Expected:**
- Shard errors → `shard_lifecycle` domain selected
- Campaign errors → `campaign` domain selected
- Tool errors → `tool` domain selected
- Unknown errors → `core`, `safety`, `routing` selected

### Step 3: Verify Budget Tracking (wait 2 min)

Check that attempts are counted:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "attempt \d+/\d+|budget.*remaining"
```

**Expected:** Budget tracked per rule, session budget decrements.

### Success Criteria

- [ ] All 10 error types detected
- [ ] Error classification accurate
- [ ] JIT predicates selected based on error type
- [ ] Budget tracking works correctly
- [ ] Complex rules attempted repair
- [ ] No infinite loops or hangs

---

## Chaos Test (15-20 min)

Push repair system to breaking point.

### Step 1: Exhaust Repair Budget (wait 8 min)

Create a rule that can't be repaired (but tries):

```bash
@"
# This rule is fundamentally broken - will exhaust budget
impossible_rule(X) :-
    does_not_exist_1(X),
    does_not_exist_2(X),
    does_not_exist_3(X),
    X = Y,  # Y unbounded
    not Z.  # Z unbounded
"@ | Out-File -Encoding UTF8 /tmp/test_exhaustion.mg

# Try to validate - will hit max retries (default: 3)
./nerd.exe check-mangle /tmp/test_exhaustion.mg
```

Wait 5 minutes.

Check budget exhaustion:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "budget exhausted|max retries exceeded"
```

**Expected:** After 3 attempts, budget exhausted for this rule.

### Step 2: Test Concurrent Repairs (wait 5 min)

Run 5 simultaneous repair attempts:

```powershell
$jobs = @()
for ($i = 1; $i -le 5; $i++) {
    $content = @"
# Concurrent test $i - missing period
concurrent_rule_$i(X) :- user_intent(X, _, _, _, _)
"@
    $content | Out-File -Encoding UTF8 "/tmp/concurrent_$i.mg"

    $jobs += Start-Job -ScriptBlock {
        param($file)
        cd C:\CodeProjects\codeNERD
        ./nerd.exe check-mangle $file
    } -ArgumentList "/tmp/concurrent_$i.mg"
}

$jobs | Wait-Job -Timeout 180 | Receive-Job
$jobs | Remove-Job
```

**Expected:** All repairs complete without race conditions or deadlocks.

### Step 3: Test Repair of Repair (wait 4 min)

Simulate LLM making a new error during repair:

```bash
@"
# First error: missing period
broken1(X) :- user_intent(X, _, _, _, _)
"@ | Out-File -Encoding UTF8 /tmp/test_iterative.mg

# Manually trigger repair, simulate LLM response with new error
# This would be done via internal testing - check logs for retry logic
./nerd.exe check-mangle /tmp/test_iterative.mg
```

Check logs for multiple repair cycles:

```bash
Select-String -Path ".nerd/logs/*system*.log" -Pattern "retry attempt|repair cycle"
```

**Expected:** System retries until max attempts or success.

### Step 4: Test Session Budget Exhaustion (wait 5 min)

Trigger 20+ repair attempts to exhaust session budget:

```powershell
# Session budget default: 20 attempts total
for ($i = 1; $i -le 25; $i++) {
    $content = @"
# Error $i - each unique rule hash
unique_error_$i(X) :- user_intent(X, _, _, _, _)
"@
    $content | Out-File -Encoding UTF8 "/tmp/session_$i.mg"
    ./nerd.exe check-mangle "/tmp/session_$i.mg" 2>&1 | Out-Null
}
```

Check for session exhaustion:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "session.*budget exhausted|session validation budget"
```

**Expected:** After ~20 attempts, session budget exhausted, no more repairs attempted.

### Step 5: Test Repair Timeout (wait 3 min)

Test per-attempt timeout (default: 60s):

```bash
# This test would require mocking LLM to delay response
# Check that timeout is enforced in logs
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "timeout|deadline exceeded"
```

**Expected:** Attempts timeout at 60s, move to next retry.

### Success Criteria

- [ ] Budget enforcement works (per-rule limit: 3)
- [ ] Session budget enforced (total limit: 20)
- [ ] Concurrent repairs don't deadlock
- [ ] Repair-of-repair cycles limited
- [ ] Timeout handling works
- [ ] System degrades gracefully under load
- [ ] No panics or memory leaks

---

## Post-Test Analysis

### Log Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose --category kernel,system
```

### Specific Repair JIT Checks

```bash
# Check JIT predicate selection
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "JIT selected \d+ predicates"

# Check SelectForRepair usage
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "SelectForRepair|repair context"

# Check budget tracking
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "budget|attempt \d+/\d+"

# Check error classification
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "classified as|error category"

# Check repair outcomes
Select-String -Path ".nerd/logs/*system*.log" -Pattern "repair succeeded|repair failed|budget exhausted"
```

### Verification Queries (Mangle)

Add these to `.nerd/mangle/scan.mg` for post-test analysis:

```mangle
# Repair attempt tracking
repair_attempt(Time, RuleHash, Attempt, MaxAttempts) :-
    log_entry(Time, _, "kernel", Msg, _),
    fn:string:contains(Msg, "attempt"),
    fn:string:contains(Msg, RuleHash).

# JIT selection events
jit_repair_selection(Time, PredicateCount, ErrorType) :-
    log_entry(Time, _, "kernel", Msg, _),
    fn:string:contains(Msg, "JIT selected"),
    fn:string:contains(Msg, "predicates for repair"),
    fn:string:contains(Msg, ErrorType).

# Budget exhaustion events
budget_exhausted(Time, Reason) :-
    log_entry(Time, _, "kernel", Msg, _),
    fn:string:contains(Msg, "budget exhausted"),
    fn:string:contains(Msg, Reason).

# Error classification distribution
error_category_count(Category, Count) :-
    Count = fn:count(Time),
    log_entry(Time, _, "kernel", Msg, _),
    fn:string:contains(Msg, "classified as"),
    fn:string:contains(Msg, Category).

# Repair success rate
repair_outcome(Time, Outcome) :-
    log_entry(Time, _, "system", Msg, _),
    fn:string:contains(Msg, "MangleRepair"),
    (fn:string:contains(Msg, "succeeded") -> Outcome = /success;
     fn:string:contains(Msg, "failed") -> Outcome = /failure;
     Outcome = /unknown).

# Average attempts per repair
avg_repair_attempts(Avg) :-
    Avg = fn:avg(Attempts),
    repair_attempt(_, _, Attempts, _).
```

Then query:

```bash
./nerd.exe query "repair_attempt"
./nerd.exe query "jit_repair_selection"
./nerd.exe query "budget_exhausted"
./nerd.exe query "error_category_count"
./nerd.exe query "repair_outcome"
```

---

## Pass/Fail Criteria

### Conservative Test

**PASS if:**
- Simple errors detected by PreValidator
- JIT selected <150 predicates (not all 799)
- Auto-repair succeeded for atom/string errors
- All operations completed in <10 min
- No panics or crashes

**FAIL if:**
- Full corpus injected (799 predicates)
- Auto-repair didn't fix known-fixable errors
- Timeouts or hangs
- Kernel panics

### Aggressive Test

**PASS if:**
- All 10 error types classified correctly
- Different error types triggered different predicate selections
- Budget tracking recorded all attempts
- Complex rules attempted repair (even if failed)
- Completed in <15 min
- No deadlocks

**FAIL if:**
- Error classification wrong (>20% misclassified)
- Same predicates selected for all error types
- Budget not tracked
- Concurrent operations failed
- Memory leaks detected

### Chaos Test

**PASS if:**
- Per-rule budget enforced (max 3 attempts)
- Session budget enforced (max 20 total)
- Concurrent repairs stable (no race conditions)
- System degrades gracefully when budget exhausted
- Timeouts handled correctly
- Completed in <20 min

**FAIL if:**
- Budget enforcement failed (unlimited retries)
- Race conditions in concurrent repairs
- Deadlocks or panics
- Memory exhaustion
- Infinite retry loops

---

## Known Issues to Watch For

| Issue | Symptom | Log Pattern |
|-------|---------|-------------|
| **Full corpus injection** | All 799 predicates in prompt | `selected 799 predicates` |
| **Budget not enforced** | Unlimited retries | `attempt 50/3` (exceeds max) |
| **Race condition** | Concurrent map access | `concurrent map` error |
| **Infinite loop** | Repair never completes | No `succeeded` or `failed` log after 5 min |
| **Wrong predicates** | Generic selection for specific errors | All repairs show same predicate list |
| **LLM timeout not handled** | Hang on slow LLM | No timeout log after 60s |
| **Session budget leak** | Budget doesn't accumulate | `session budget: 0/20` after 10 attempts |

---

## Repair System Architecture Reference

### FeedbackLoop Flow

```
User Input → PreValidator (regex checks)
           ↓
        Sanitizer (auto-fix known issues)
           ↓
        HotLoadRule (sandbox compile)
           ↓
        ValidateLearnedRule (schema check)
           ↓
        [if error] → ErrorClassifier → SelectForRepair → LLM with JIT predicates
           ↓
        [retry with feedback] → (back to PreValidator)
           ↓
        [success or budget exhausted]
```

### Budget Tracking

- **Per-rule limit:** 3 attempts (default `MaxRetries`)
- **Session limit:** 20 total attempts (default `SessionBudget`)
- **Timeouts:**
  - Per-attempt: 60s (default `PerAttemptTimeout`)
  - Total: 180s (default `TotalTimeout`)

### JIT Predicate Selection

SelectForRepair logic:
1. Always include: `core`, `safety`, `routing` domains
2. Add based on error type:
   - "shard" in error → add `shard_lifecycle`
   - "campaign" in error → add `campaign`
   - "tool" in error → add `tool`
3. Limit to ~60 predicates (vs 799 in full corpus)

---

## Integration Points Tested

1. **PredicateCorpus ↔ PredicateSelector** - Corpus provides predicates, selector filters
2. **FeedbackLoop ↔ PredicateSelector** - Loop uses `SelectForRepair()` for JIT injection
3. **FeedbackLoop ↔ ValidationBudget** - Budget tracked per attempt
4. **ErrorClassifier ↔ PredicateSelector** - Error types influence predicate selection
5. **MangleRepairShard ↔ FeedbackLoop** - Shard invokes loop for LLM repair
6. **Sanitizer ↔ PreValidator** - Quick fixes before heavy validation

---

## Performance Benchmarks

| Test | Expected Time | Max Acceptable |
|------|---------------|----------------|
| Simple syntax error repair | 3s | 10s |
| 10 error type classification | 5s | 15s |
| Single repair attempt (with LLM) | 5-10s | 30s |
| Budget exhaustion (3 attempts) | 15-30s | 60s |
| Concurrent 5 repairs | 10-20s | 45s |
| Session budget (20 attempts) | 60-120s | 180s |

Anything exceeding "Max Acceptable" indicates performance regression.
