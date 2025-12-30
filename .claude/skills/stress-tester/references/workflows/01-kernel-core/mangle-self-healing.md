# Mangle Self-Healing System Stress Test

Stress test for the Mangle Predicate Corpus and Self-Healing Repair System.

## Overview

This test validates the Mangle safety layer components:

- **PredicateCorpus** - Baked-in corpus with 799 predicates for validation
- **MangleRepairShard** - Type S system shard for rule interception and LLM-powered repair
- **PredicateSelector** - JIT-style context-aware predicate selection
- **FeedbackLoop integration** - Predicate selection during rule generation

**Expected Duration:** 15-30 minutes total

## Prerequisites

```bash
# Build codeNERD (ensure corpus is embedded)
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd

# Clear logs
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue

# Verify kernel boots with corpus
./nerd.exe status
```

**Expected:** Status shows "Mangle kernel initialized" - corpus loads at boot.

---

## Conservative Test (5-10 min)

Test that the corpus loads and validates correctly.

### Step 1: Verify Corpus Loading (wait 30s)

```bash
./nerd.exe status
```

Check logs for corpus loading:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "corpus|predicate"
```

**Expected:** Log shows "Predicate corpus loaded: X predicates"

### Step 2: Validate Known Good Rules (wait 2 min)

```bash
# Should pass - valid rule
./nerd.exe check-mangle .nerd/mangle/learned.mg
```

**Expected:** "OK" - existing learned rules are valid.

### Step 3: Test Schema Validation (wait 3 min)

Create a test file with a known-bad rule:

```bash
@"
# Test rule with undeclared predicate
bad_rule(X) :- fake_predicate_xyz(X).
"@ | Out-File -Encoding UTF8 /tmp/test_bad.mg

./nerd.exe check-mangle /tmp/test_bad.mg
```

**Expected:** Error about undeclared predicate `fake_predicate_xyz`.

### Success Criteria

- [ ] Corpus loaded at kernel boot
- [ ] Valid rules pass validation
- [ ] Invalid rules detected by schema check
- [ ] No kernel panics

---

## Aggressive Test (10-15 min)

Stress the validation pipeline with many rules.

### Step 1: Generate Bulk Test Rules (wait 5 min)

```bash
# Create many valid rules to parse
@"
# Bulk valid rules
test_rule_1(X) :- user_intent(X, _, _, _, _).
test_rule_2(X) :- file_topology(X, _, _, _, _).
test_rule_3(X) :- shard_executed(X, _, _, _).
test_rule_4(X) :- diagnostic(X, _, _, _, _).
test_rule_5(X) :- permitted(X).
test_rule_6(X) :- campaign(X, _, _, _, _).
test_rule_7(X) :- context_atom(X).
test_rule_8(X) :- next_action(X).
test_rule_9(X) :- active_strategy(X).
test_rule_10(X) :- shard_profile(X, _, _).
"@ | Out-File -Encoding UTF8 /tmp/test_bulk.mg

./nerd.exe check-mangle /tmp/test_bulk.mg
```

**Expected:** All valid rules pass.

### Step 2: Test Mixed Valid/Invalid Rules (wait 5 min)

```bash
@"
# Mixed rules - some valid, some invalid
valid_rule(X) :- user_intent(X, _, _, _, _).
hallucinated_1(X) :- server_health(X).
valid_rule_2(X) :- permitted(X).
hallucinated_2(X) :- is_running(X).
valid_rule_3(X) :- context_atom(X).
"@ | Out-File -Encoding UTF8 /tmp/test_mixed.mg

./nerd.exe check-mangle /tmp/test_mixed.mg
```

**Expected:** Errors for `server_health` and `is_running` (common LLM hallucinations).

### Step 3: Validate Against Full Corpus (wait 5 min)

Query the kernel with predicates that should exist:

```bash
./nerd.exe query "user_intent"
./nerd.exe query "file_topology"
./nerd.exe query "shard_executed"
./nerd.exe query "diagnostic"
./nerd.exe query "permitted"
```

**Expected:** All queries return (may be empty, but no schema errors).

### Success Criteria

- [ ] Bulk validation completed
- [ ] Invalid predicates detected in mixed rules
- [ ] No false positives (valid rules passing)
- [ ] Kernel remained stable

---

## Chaos Test (15-20 min)

Stress the repair pipeline with adversarial inputs.

### Step 1: Adversarial Rule Patterns (wait 5 min)

Test common LLM hallucination patterns:

```bash
@"
# Common AI agent mistakes
sql_style(X) :- SELECT X FROM table.
souffle_decl(x, y) :- .decl edge(x, y).
lowercase_vars(x, y) :- parent(x, y).
wrong_negation(X) :- NOT dangerous(X).
string_instead_of_atom(X) :- state(X, "active").
missing_period(X) :- source(X)
sql_aggregation(Sum) :- item(X), Sum = sum(X).
"@ | Out-File -Encoding UTF8 /tmp/test_adversarial.mg

./nerd.exe check-mangle /tmp/test_adversarial.mg
```

**Expected:** All rules flagged with specific error categories.

### Step 2: Simulate Rapid Rule Generation (wait 10 min)

Stress the FeedbackLoop with rapid validation requests:

```bash
# Generate 100 test rules
$rules = @()
for ($i = 1; $i -le 100; $i++) {
    $rules += "test_rule_$i(X) :- user_intent(X, _, _, _, _)."
}
$rules -join "`n" | Out-File -Encoding UTF8 /tmp/stress_rules.mg

./nerd.exe check-mangle /tmp/stress_rules.mg
```

Wait 2 minutes. Then:

```bash
# Check for any validation failures
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "validation|repair|error"
```

### Step 3: Test Concurrent Validation (wait 5 min)

Run multiple validation requests in parallel:

```powershell
# Start 5 parallel validation jobs
$jobs = @()
for ($i = 1; $i -le 5; $i++) {
    $jobs += Start-Job -ScriptBlock {
        cd C:\CodeProjects\codeNERD
        ./nerd.exe check-mangle .nerd/mangle/learned.mg
    }
}
$jobs | Wait-Job | Receive-Job
$jobs | Remove-Job
```

**Expected:** All validations complete without race conditions.

### Success Criteria

- [ ] Adversarial patterns detected
- [ ] No panics during rapid validation
- [ ] Concurrent validation stable
- [ ] All error categories properly classified

---

## Hybrid Test (20-30 min)

Test repair shard during actual rule generation.

### Step 1: Trigger Autopoiesis Learning (wait 15 min)

```bash
# Run a task that may trigger rule learning
./nerd.exe run "fix any issues in the test files and learn patterns from what you discover"
```

Wait 12 minutes. The executive shard may generate new rules.

### Step 2: Verify Learned Rules Are Valid (wait 5 min)

```bash
# Check that any new learned rules are valid
./nerd.exe check-mangle .nerd/mangle/learned.mg

# Check for repair activity in logs
Select-String -Path ".nerd/logs/*system*.log" -Pattern "MangleRepair|repair|validated"
```

### Step 3: Test Predicate Selection Context (wait 5 min)

Verify JIT-style selection is working:

```bash
# Query that should use context-aware predicate selection
./nerd.exe run "explain how the coder shard works"
```

Check logs for predicate selection:

```bash
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "JIT selected|predicates for domain"
```

**Expected:** Logs show context-aware predicate selection.

### Step 4: Verify No Schema Drift (wait 5 min)

```bash
# Final validation of all Mangle files
./nerd.exe check-mangle .nerd/mangle/*.mg

# Count predicates in use
./nerd.exe logic | Select-String -Pattern "Decl" | Measure-Object -Line
```

### Success Criteria

- [ ] Autopoiesis produced valid rules (or none)
- [ ] Repair shard intercepted any invalid rules
- [ ] JIT predicate selection active
- [ ] No schema drift detected

---

## Post-Test Analysis

### Log Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Specific Self-Healing Checks

```bash
# Check corpus loading
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "corpus loaded"

# Check repair activity
Select-String -Path ".nerd/logs/*system*.log" -Pattern "MangleRepair"

# Check validation errors
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "undeclared|undefined|invalid"

# Check predicate selection
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "JIT selected"
```

### Success Criteria

- [ ] PredicateCorpus loaded successfully at boot
- [ ] MangleRepairShard registered and active
- [ ] Invalid rules detected before persistence
- [ ] No new schema drift in learned.mg
- [ ] JIT predicate selection working
- [ ] No kernel panics

### Known Issues to Watch For

- `Predicate corpus not available` - Corpus database missing from embed
- `undeclared predicate in learned rule` - Autopoiesis generated invalid rule (repair should fix)
- `JIT selector failed` - Falling back to full predicate list
- `repair attempt exceeded` - Rule rejected after max retries (expected for truly bad rules)

---

## Stress Queries

Add these to analyze self-healing activity:

```mangle
# Corpus validation events
corpus_validation(Time, Rule, Status) :-
    log_entry(Time, _, _, Msg, _),
    fn:string:contains(Msg, "check-mangle"),
    fn:string:contains(Msg, Status).

# Repair shard activity
repair_event(Time, Action, Details) :-
    log_entry(Time, _, "system_shards", Msg, _),
    fn:string:contains(Msg, "MangleRepair"),
    fn:string:contains(Msg, Action).

# JIT predicate selection
jit_selection(Time, Count, Domain) :-
    log_entry(Time, _, "kernel", Msg, _),
    fn:string:contains(Msg, "JIT selected"),
    fn:string:contains(Msg, Count),
    fn:string:contains(Msg, Domain).
```
