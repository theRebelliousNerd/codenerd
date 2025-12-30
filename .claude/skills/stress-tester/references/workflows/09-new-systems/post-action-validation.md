# Post-Action Validation System Stress Test

Stress test for the Post-Action Validation System that verifies every action succeeded after execution.

## Overview

Tests the validation system's handling of:

- File write validation (hash comparison, read-back)
- Syntax validation (Go, JSON, YAML, TOML, Mangle)
- Execution validation (output pattern scanning)
- CodeDOM validation (semantic integrity)
- Self-healing (retry, rollback, escalation)
- Mangle fact emission for policy reasoning

**Expected Duration:** 25-40 minutes total

### Key Files

- `internal/core/action_validator.go` - Core validator interface and registry
- `internal/core/validator_file.go` - File write/edit/delete validators
- `internal/core/validator_syntax.go` - Syntax validators (Go, JSON, YAML, TOML, Mangle)
- `internal/core/validator_exec.go` - Execution/build/test validators
- `internal/core/validator_codedom.go` - CodeDOM and line edit validators
- `internal/core/self_healing.go` - Self-healing strategies
- `internal/core/defaults/policy/validation.mg` - Validation policy rules

---

## Conservative Test (8-12 min)

Test basic validation for successful operations.

### Step 1: Verify Validator Registry (wait 2 min)

```bash
./nerd.exe status
```

Check for validator initialization:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "Validator registry initialized"
```

**Expected:** "Validator registry initialized with N validators" (where N >= 10)

### Step 2: File Write Validation (wait 3 min)

Create a simple file and verify validation:

```bash
./nerd.exe spawn coder "create a file test_validation.txt with content 'Hello World'"
```

Monitor validation:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "action_verified|ValidationMethodHash"
```

**Verify:**
- Hash validation occurred
- Confidence >= 0.95

### Step 3: Go Syntax Validation (wait 3 min)

Create a Go file:

```bash
./nerd.exe spawn coder "create a simple Go function in test_validation.go that returns 'hello'"
```

Check syntax validation:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "SyntaxValidator|go/parser|ValidationMethodSyntax"
```

**Verify:**
- Go file was syntax-checked
- No parse errors

### Step 4: JSON Validation (wait 2 min)

Create a JSON file:

```bash
./nerd.exe spawn coder "create a config.json file with some example settings"
```

Check:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "json.Unmarshal|ValidationMethodSyntax"
```

### Step 5: Verify Mangle Facts (wait 2 min)

Query for validation facts:

```bash
./nerd.exe query "action_verified"
./nerd.exe query "validation_method_used"
```

### Success Criteria

- [ ] Validator registry initialized with 10+ validators
- [ ] File writes validated with hash comparison (confidence >= 0.95)
- [ ] Go syntax validated without errors
- [ ] JSON syntax validated without errors
- [ ] Mangle facts emitted for successful validations

---

## Aggressive Test (10-15 min)

Test validation with edge cases and forced failures.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
Remove-Item test_validation* -ErrorAction SilentlyContinue
```

### Step 2: Large File Validation (wait 4 min)

Test hash validation on larger files:

```bash
./nerd.exe spawn coder "create a Go file with 50 functions for testing various scenarios"
```

Monitor performance:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "Validator|Duration|hash"
```

**Verify:** Validation completes in reasonable time

### Step 3: Concurrent Validations (wait 4 min)

Spawn multiple operations that trigger validation:

```powershell
Start-Job { ./nerd.exe spawn coder "create file1.go with a test function" }
Start-Job { ./nerd.exe spawn coder "create file2.go with another function" }
Start-Job { ./nerd.exe spawn coder "create data.json with sample data" }

Get-Job | Wait-Job -Timeout 240
Get-Job | Receive-Job -ErrorAction SilentlyContinue
Get-Job | Remove-Job
```

Check for race conditions:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "concurrent|race|lock"
```

### Step 4: Validation Priority Ordering (wait 3 min)

Verify validators run in priority order:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "validator.*priority|Priority"
```

Lower priority validators should run first.

### Step 5: Output Pattern Scanning (wait 3 min)

Run a command that produces output:

```bash
./nerd.exe spawn coder "run go build and show the output"
```

Check execution validation:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "ExecutionValidator|output_scan|failurePatterns"
```

### Success Criteria

- [ ] Large file validated without timeout
- [ ] Concurrent validations completed without race conditions
- [ ] Validators ran in priority order (lower first)
- [ ] Command output scanned for failure patterns
- [ ] No validation failures on valid operations

---

## Chaos Test (10-15 min)

Test self-healing and error recovery.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Syntax Error Recovery (wait 4 min)

Create a file with intentional syntax error, then fix:

```bash
# First, create a broken file manually
echo "func broken(" > broken_test.go

# Then ask to fix it
./nerd.exe spawn coder "fix the syntax error in broken_test.go"
```

Check self-healing:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "syntax validation failed|needs_self_healing|HealingRollback"
```

### Step 3: Retry Mechanism (wait 4 min)

Simulate transient failure (file busy):

```bash
# Create a file
./nerd.exe spawn coder "create retry_test.txt"

# Verify retry mechanism exists
Select-String -Path ".nerd/logs/*.log" -Pattern "HealingRetry|retryAction|attempt"
```

### Step 4: Max Retries Exceeded (wait 3 min)

Check escalation after max retries:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "validation_max_retries_reached|HealingEscalate|escalateToUser"
```

### Step 5: Validation Confidence Thresholds (wait 3 min)

Query threshold configuration:

```bash
./nerd.exe query "validation_threshold"
```

**Expected thresholds:**
- `/hash` -> 0.95
- `/syntax` -> 0.90
- `/existence` -> 0.70
- `/content_check` -> 0.85
- `/output_scan` -> 0.75
- `/codedom_refresh` -> 0.90

### Success Criteria

- [ ] Syntax errors detected and flagged
- [ ] Retry mechanism triggered on transient failures
- [ ] Escalation occurs after max retries (3 by default)
- [ ] Confidence thresholds properly configured
- [ ] Self-healing facts emitted to kernel

---

## Hybrid Test (10-15 min)

Test validation integration with other systems.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: CodeDOM Edit Validation (wait 5 min)

Test semantic code editing:

```bash
./nerd.exe spawn coder "add a new method to an existing struct in the codebase"
```

Monitor CodeDOM validation:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "CodeDOMValidator|codedom_refresh|verifyElementExists"
```

### Step 3: Validation with Transaction Manager (wait 4 min)

Check validation integration with transactions:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "ShadowValidation|Transaction|rollback"
```

### Step 4: Policy-Based Blocking (wait 3 min)

Query for action blocking due to validation:

```bash
./nerd.exe query "block_action"
./nerd.exe query "needs_self_healing"
```

### Step 5: Healing Outcomes (wait 3 min)

Query healing metrics:

```bash
./nerd.exe query "action_healed"
./nerd.exe query "action_resolved"
./nerd.exe query "unresolved_failure"
```

### Success Criteria

- [ ] CodeDOM edits validated for semantic integrity
- [ ] Shadow validation integrated with transaction manager
- [ ] Policy rules derived appropriate blocking decisions
- [ ] Healing outcomes tracked in Mangle facts
- [ ] No unresolved failures at end of test

---

## Unit Test Verification (5 min)

Run the validator unit tests:

```bash
cd c:/CodeProjects/codeNERD
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go test -tags=sqlite_vec ./internal/core/... -run "Validator" -v
```

**Expected:** All validator tests pass:
- `TestValidatorRegistry_Register`
- `TestValidatorRegistry_Validate`
- `TestValidatorRegistry_NoValidators`
- `TestValidatorRegistry_ShortCircuitOnFailure`
- `TestValidatorRegistry_ContextCancellation`

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Validation-Specific Queries

```bash
# Count validations by method
Select-String -Path ".nerd/logs/*.log" -Pattern "ValidationMethodHash" | Measure-Object
Select-String -Path ".nerd/logs/*.log" -Pattern "ValidationMethodSyntax" | Measure-Object
Select-String -Path ".nerd/logs/*.log" -Pattern "ValidationMethodExistence" | Measure-Object
Select-String -Path ".nerd/logs/*.log" -Pattern "ValidationMethodOutputScan" | Measure-Object
Select-String -Path ".nerd/logs/*.log" -Pattern "ValidationMethodCodeDOMRefresh" | Measure-Object

# Count healing attempts
Select-String -Path ".nerd/logs/*.log" -Pattern "HealingRetry" | Measure-Object
Select-String -Path ".nerd/logs/*.log" -Pattern "HealingRollback" | Measure-Object
Select-String -Path ".nerd/logs/*.log" -Pattern "HealingEscalate" | Measure-Object

# Find validation failures
Select-String -Path ".nerd/logs/*.log" -Pattern "action_validation_failed|Verified: false"
```

### Mangle Query Analysis

```bash
# Query validation facts
./nerd.exe query "action_verified"
./nerd.exe query "action_validation_failed"
./nerd.exe query "validation_attempt"
./nerd.exe query "healing_attempt"
./nerd.exe query "action_escalated"
```

### Success Metrics

| Metric | Conservative | Aggressive | Chaos | Hybrid |
|--------|--------------|------------|-------|--------|
| Panics | 0 | 0 | 0 | 0 |
| Validation failures on valid ops | 0 | 0 | 0 | 0 |
| Self-healing successes | N/A | >=1 | >=1 | N/A |
| Unresolved failures | 0 | 0 | <3 | 0 |
| Avg validation time | <100ms | <500ms | <1s | <500ms |

---

## Known Issues to Watch For

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| Hash mismatch | "content hash mismatch" | Race condition in write | Increase retry backoff |
| Syntax false positive | Valid code marked invalid | Parser error | Check parser imports |
| Slow validation | Validation > 5s | Large file | Add file size limit |
| Missing fact | Query returns empty | Assert failed | Check kernel connection |
| Infinite retry | Retries never stop | Max not reached | Check maxRetries config |

---

## Cleanup

```bash
Remove-Item test_validation* -ErrorAction SilentlyContinue
Remove-Item broken_test.go -ErrorAction SilentlyContinue
Remove-Item retry_test.txt -ErrorAction SilentlyContinue
Remove-Item file1.go, file2.go, data.json -ErrorAction SilentlyContinue
```

---

## Related Files

- [mangle-self-healing.md](../01-kernel-core/mangle-self-healing.md) - Mangle self-healing
- [shadow-mode-stress.md](../06-advanced-features/shadow-mode-stress.md) - Transaction shadow mode
- [full-ooda-loop-stress.md](../08-hybrid-integration/full-ooda-loop-stress.md) - Full OODA cycle
