# Mangle Startup Validation Stress Test

Stress test for boot-time validation of persisted learned.mg rules.

## Overview

This test validates the startup validation system that checks learned.mg on kernel boot:

- **Boot-time parsing** - Learned rules parsed and validated before kernel initialization
- **Invalid rule detection** - Syntax errors, undeclared predicates, safety violations
- **Self-healing markers** - Detection of previously self-healed rules (# SELF-HEALED:)
- **Graceful degradation** - System continues with invalid rules commented out
- **Performance** - Startup time with large learned.mg files

**Expected Duration:** 10-30 minutes total

## Prerequisites

```bash
# Build codeNERD
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd

# Clear logs
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue

# Backup current learned.mg
Copy-Item .nerd/mangle/learned.mg .nerd/mangle/learned.mg.backup -ErrorAction SilentlyContinue
```

---

## Conservative Test (5 min)

Test normal startup with valid learned.mg.

### Step 1: Boot with Valid learned.mg (wait 1 min)

```bash
./nerd.exe status
```

Check logs for validation success:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "learned.mg|validation|loaded"
```

**Expected:** Logs show "learned.mg validated successfully" or similar.

### Step 2: Boot with Empty learned.mg (wait 2 min)

```bash
# Clear learned.mg
@"

"@ | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

./nerd.exe status
```

**Expected:** Boot succeeds, no errors about missing rules.

### Step 3: Check Validation Stats (wait 2 min)

```bash
# Restore original
Copy-Item .nerd/mangle/learned.mg.backup .nerd/mangle/learned.mg -ErrorAction SilentlyContinue

./nerd.exe status
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "rules loaded|validated|skipped"
```

**Expected:** Stats show count of rules loaded.

### Success Criteria

- [ ] Valid learned.mg boots successfully
- [ ] Empty learned.mg boots successfully
- [ ] Validation stats reported
- [ ] No kernel panics

---

## Aggressive Test (10 min)

Test with invalid rules and self-healing.

### Step 1: Boot with Single Invalid Rule (wait 3 min)

Create learned.mg with one invalid rule:

```bash
@"
# Valid rule
permitted(/system_start).

# Invalid rule - undeclared predicate
next_action(/test) :- fake_predicate_does_not_exist(X).

# Another valid rule
next_action(/initialize) :- session_state(_,/booting,_).
"@ | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

./nerd.exe status
```

Check logs:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "invalid|undeclared|warning|commented"
```

**Expected:** Warning logged, invalid rule commented out automatically.

Verify file was modified:

```bash
Select-String -Path ".nerd/mangle/learned.mg" -Pattern "# DISABLED:|# INVALID:"
```

### Step 2: Boot with 50% Invalid Rules (wait 3 min)

```bash
@"
# Valid 1
permitted(/system_start).

# Invalid 1 - undeclared predicate
bad_rule(X) :- hallucinated_pred(X).

# Valid 2
next_action(/initialize) :- session_state(_,/booting,_).

# Invalid 2 - lowercase variable
wrong(x) :- parent(x).

# Valid 3
current_phase(/system_start).

# Invalid 3 - SQL style
weird(X) :- SELECT X FROM table.

# Valid 4
permitted(/initialize).

# Invalid 4 - missing period
broken(X) :- source(X)

# Valid 5
system_startup(/ready,/initialized).

# Invalid 5 - wrong negation syntax
neg(X) :- NOT dangerous(X).
"@ | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

./nerd.exe status
```

Check validation results:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "validation|commented|disabled"
Select-String -Path ".nerd/mangle/learned.mg" -Pattern "# DISABLED:|# INVALID:"
```

**Expected:** All 5 invalid rules commented, 5 valid rules loaded.

### Step 3: Boot with Self-Healed Rules (wait 2 min)

```bash
@"
# Valid rule
permitted(/system_start).

# Previously self-healed rule
# SELF-HEALED: rule uses undefined predicates: [fake_pred]
# next_action(/test) :- fake_pred(X).

# Another valid rule
current_phase(/system_start).
"@ | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

./nerd.exe status
```

Check logs:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "SELF-HEALED|detected|skipped"
```

**Expected:** Self-healed marker detected, rule skipped, no re-commenting.

### Step 4: Boot with Corrupted learned.mg (wait 2 min)

```bash
@"
# Valid start
permitted(/system_start).

# Corrupted section - incomplete rule
next_action(/test) :- user_

# More corruption - invalid syntax
@#$%^&*()

# Valid end
current_phase(/system_start).
"@ | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

./nerd.exe status
```

**Expected:** Parser errors logged, corrupted sections skipped, valid rules loaded.

### Success Criteria

- [ ] Single invalid rule detected and commented
- [ ] Multiple invalid rules handled correctly
- [ ] Self-healed markers detected
- [ ] Corrupted file handled gracefully
- [ ] System continues to boot in all cases

---

## Chaos Test (15 min)

Extreme validation scenarios.

### Step 1: Boot with ALL Rules Invalid (wait 3 min)

```bash
@"
# All invalid rules
hallucinated_1(X) :- fake_pred_1(X).
hallucinated_2(X) :- fake_pred_2(X).
hallucinated_3(X) :- fake_pred_3(X).
lowercase_vars(x, y) :- parent(x, y).
sql_style(X) :- SELECT X FROM table.
wrong_negation(X) :- NOT dangerous(X).
missing_period(X) :- source(X)
souffle_decl(x, y) :- .decl edge(x, y).
string_atom(X) :- state(X, "active").
bad_aggregation(Sum) :- item(X), Sum = sum(X).
"@ | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

./nerd.exe status
```

Check that system continues:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "commented|disabled|boot complete"
```

**Expected:** All rules commented, kernel boots with empty rule set.

### Step 2: Boot with Stratification Cycles (wait 3 min)

```bash
@"
# Valid rule
permitted(/system_start).

# Stratification violation - mutual recursion through negation
p(X) :- user_intent(X,_,_,_,_), !q(X).
q(X) :- user_intent(X,_,_,_,_), !p(X).

# Another valid rule
current_phase(/system_start).
"@ | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

./nerd.exe status
```

**Expected:** Stratification error detected, cycle rules commented or error logged.

### Step 3: Boot with Infinite Recursion Patterns (wait 3 min)

```bash
@"
# Valid rule
permitted(/system_start).

# Unbounded recursion
ancestor(X,Y) :- parent(X,Y).
ancestor(X,Z) :- ancestor(X,Y), ancestor(Y,Z).

# Counter without bound
count(N) :- count(M), N = fn:plus(M, 1).

# Another valid rule
current_phase(/system_start).
"@ | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

./nerd.exe status
```

Wait 2 minutes, then check for timeout handling:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "timeout|gas|recursion|derivation"
```

**Expected:** Gas limit or timeout prevents infinite derivation, system remains stable.

### Step 4: Boot with Duplicate Rules (wait 2 min)

```bash
@"
# Duplicate rules
permitted(/system_start).
permitted(/system_start).
permitted(/system_start).
next_action(/initialize) :- session_state(_,/booting,_).
next_action(/initialize) :- session_state(_,/booting,_).
current_phase(/system_start).
current_phase(/system_start).
"@ | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

./nerd.exe status
```

**Expected:** Duplicates detected, deduplication applied, no errors.

### Step 5: Boot with Large learned.mg (wait 3 min)

Generate 1000 rules:

```powershell
$rules = @()
for ($i = 1; $i -le 1000; $i++) {
    $rules += "# Autopoiesis-learned rule (added 2025-12-11)"
    $rules += "test_rule_$i(X) :- user_intent(X, _, _, _, _)."
    $rules += ""
}
$rules -join "`n" | Out-File -Encoding UTF8 .nerd/mangle/learned.mg

# Time the boot
Measure-Command { ./nerd.exe status }
```

Check logs:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "rules loaded|validation time"
```

**Expected:** Boot completes within 10 seconds, all rules validated.

### Step 6: Concurrent Boot Attempts (wait 1 min)

```powershell
# Restore a normal learned.mg
Copy-Item .nerd/mangle/learned.mg.backup .nerd/mangle/learned.mg -ErrorAction SilentlyContinue

# Start 3 parallel boot attempts
$jobs = @()
for ($i = 1; $i -le 3; $i++) {
    $jobs += Start-Job -ScriptBlock {
        cd C:\CodeProjects\codeNERD
        ./nerd.exe status
    }
}
$jobs | Wait-Job | Receive-Job
$jobs | Remove-Job
```

**Expected:** All boot attempts complete without file corruption or race conditions.

### Success Criteria

- [ ] All-invalid learned.mg handled gracefully
- [ ] Stratification cycles detected
- [ ] Infinite recursion prevented
- [ ] Duplicates handled
- [ ] Large file (1000 rules) boots quickly
- [ ] Concurrent boots don't corrupt state

---

## Post-Test Analysis

### Log Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Specific Validation Checks

```bash
# Check validation events
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "learned.mg|validation|invalid|commented"

# Check for panics
Select-String -Path ".nerd/logs/*.log" -Pattern "panic|fatal"

# Check startup times
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "boot|startup|initialized"

# Check self-healing activity
Select-String -Path ".nerd/mangle/learned.mg" -Pattern "# SELF-HEALED:|# DISABLED:|# INVALID:"
```

### Restore Original

```bash
# Restore backup
Copy-Item .nerd/mangle/learned.mg.backup .nerd/mangle/learned.mg -Force

# Verify restoration
./nerd.exe status
```

### Success Criteria

- [ ] No kernel panics during any boot scenario
- [ ] Invalid rules detected and commented
- [ ] Self-healing markers preserved
- [ ] System boots with empty, small, and large learned.mg
- [ ] Concurrent boots handled safely
- [ ] Startup time remains reasonable (<10s for 1000 rules)
- [ ] Original learned.mg restored successfully

### Known Issues to Watch For

- `parse error in learned.mg` - Expected for corrupted files, should be graceful
- `undeclared predicate in learned rule` - Should trigger commenting
- `stratification violation` - Should be detected and handled
- `gas limit exceeded during boot` - Infinite recursion detected
- `file corruption detected` - Should trigger recovery
- `concurrent modification` - File locking should prevent

---

## Test File Creation Commands

For easy test file generation:

### Create Valid Test File

```bash
@"
# All valid rules
permitted(/system_start).
next_action(/initialize) :- session_state(_,/booting,_).
current_phase(/system_start).
system_startup(/ready,/initialized).
"@ | Out-File -Encoding UTF8 .nerd/mangle/test_valid.mg
```

### Create Invalid Test File

```bash
@"
# Mix of valid and invalid
permitted(/system_start).
fake_rule(X) :- hallucinated_predicate(X).
next_action(/initialize) :- session_state(_,/booting,_).
lowercase(x) :- parent(x, y).
"@ | Out-File -Encoding UTF8 .nerd/mangle/test_invalid.mg
```

### Create Large Test File

```powershell
$rules = @()
for ($i = 1; $i -le 10000; $i++) {
    $rules += "large_rule_$i(X) :- user_intent(X, _, _, _, _)."
}
$rules -join "`n" | Out-File -Encoding UTF8 .nerd/mangle/test_large.mg
```

### Create Corrupted Test File

```bash
@"
# Valid start
permitted(/system_start).

# Corruption
next_action(/test) :- incomplete_rule_here
@#$%^&*()
{{{ invalid syntax }}}

# Valid end
current_phase(/system_start).
"@ | Out-File -Encoding UTF8 .nerd/mangle/test_corrupted.mg
```

---

## Verification Steps

After each boot test, verify:

1. **Boot completed:** `./nerd.exe status` returns successfully
2. **Logs clean:** No panics in `.nerd/logs/*.log`
3. **File integrity:** `learned.mg` is well-formed (valid Mangle or commented)
4. **Validation recorded:** Logs show validation activity
5. **System functional:** Can execute basic commands

---

## Stress Queries

Add these to analyze validation activity:

```mangle
# Validation events during boot
validation_event(Time, File, Status) :-
    log_entry(Time, _, _, Msg, _),
    fn:string:contains(Msg, "learned.mg"),
    fn:string:contains(Msg, "validation").

# Invalid rules detected
invalid_rule_detected(Time, Reason) :-
    log_entry(Time, _, "kernel", Msg, _),
    fn:string:contains(Msg, "invalid"),
    fn:string:contains(Msg, "rule").

# Self-healing markers
self_healed_rule(Time, Predicate) :-
    log_entry(Time, _, _, Msg, _),
    fn:string:contains(Msg, "SELF-HEALED"),
    fn:string:contains(Msg, Predicate).

# Boot performance
boot_time(Start, End, Duration) :-
    log_entry(Start, _, "kernel", "boot started", _),
    log_entry(End, _, "kernel", "boot complete", _),
    Duration = fn:minus(End, Start).
```
