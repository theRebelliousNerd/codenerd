# Mangle File Watcher Stress Test

**Component:** MangleWatcher
**Location:** `internal/core/mangle_watcher.go`
**Purpose:** Real-time monitoring and validation of `.nerd/mangle/*.mg` files

## Overview

The MangleWatcher monitors the `.nerd/mangle/` directory for changes to Mangle schema and policy files, triggering automatic validation, repair, and kernel reloading on modifications. This workflow stress tests file system event handling, debouncing logic, concurrent modification handling, and validation pipeline integration.

**Key Capabilities Tested:**
- File creation/modification/deletion detection
- Debounced validation triggers (avoid event storms)
- Concurrent file modification handling
- Syntax/semantic validation pipeline
- Automatic repair trigger on invalid rules
- Stats tracking and metric updates
- Watcher recovery after crashes/errors

---

## Prerequisites

### System Requirements
- codeNERD built with file watcher support
- `.nerd/mangle/` directory exists
- MangleWatcher initialized and running
- File system write permissions

### Setup Commands

```bash
# Ensure workspace is initialized
cd C:/CodeProjects/codeNERD
./nerd.exe init

# Verify .nerd/mangle directory exists
ls .nerd/mangle/

# Start MangleWatcher (if not auto-started)
# The watcher typically starts automatically with the kernel
# For manual testing, trigger via:
./nerd.exe validate --watch
```

### Expected Initial State
- `.nerd/mangle/learned.mg` exists (learned facts)
- `.nerd/mangle/scan.mg` may exist (scan results)
- MangleWatcher goroutine active
- No pending validation errors

---

## Conservative Tests (5 minutes)

**Goal:** Verify basic file monitoring and validation flow under normal conditions.

### Test 1: New File Creation

**Scenario:** Create a new `.mg` file → validation triggered

```bash
# Create a valid new Mangle file
cat > .nerd/mangle/test_rules.mg << 'EOF'
# Test rules for stress testing
Decl test_fact(X.Type<string>).

test_fact("hello").
test_fact("world").
EOF

# Expected: Watcher detects creation, validates, loads into kernel
# Verify validation triggered
./nerd.exe query 'test_fact(X)?'
# Expected output: X = "hello", X = "world"
```

**Verification:**
- [ ] Watcher event logged (check logs for "detected change")
- [ ] Validation pass logged
- [ ] Facts queryable in kernel
- [ ] No error messages

### Test 2: File Modification

**Scenario:** Edit existing `.mg` file → re-validation triggered

```bash
# Modify the test file
cat >> .nerd/mangle/test_rules.mg << 'EOF'

test_fact("modified").
EOF

# Expected: Watcher detects modification, re-validates, reloads
./nerd.exe query 'test_fact("modified")?'
# Expected output: true
```

**Verification:**
- [ ] Modification event logged
- [ ] Re-validation triggered
- [ ] New fact loaded
- [ ] Previous facts still present

### Test 3: Valid Rule Addition

**Scenario:** Save a valid rule → passes validation seamlessly

```bash
# Add a rule with logic
cat >> .nerd/mangle/test_rules.mg << 'EOF'

Decl derived_fact(X.Type<string>).

derived_fact(X) :- test_fact(X), X = "hello".
EOF

# Expected: Validation passes, rule active
./nerd.exe query 'derived_fact(X)?'
# Expected output: X = "hello"
```

**Verification:**
- [ ] Validation pass
- [ ] Rule correctly derived
- [ ] No safety errors

### Test 4: Stats Update Verification

**Scenario:** Check that watcher stats are updated correctly

```bash
# Query watcher statistics
./nerd.exe stats --component mangle_watcher

# Expected metrics:
# - files_watched: 2+ (.nerd/mangle/*.mg)
# - events_processed: 3+ (from above tests)
# - validation_passes: 3+
# - validation_failures: 0
```

**Verification:**
- [ ] Event count incremented
- [ ] Validation stats accurate
- [ ] No leaked goroutines

---

## Aggressive Tests (10 minutes)

**Goal:** Test error handling, debouncing, concurrent modifications, and performance limits.

### Test 5: Invalid Rule → Repair Triggered

**Scenario:** Save invalid rule → automatic repair attempt

```bash
# Inject syntax error (lowercase variable)
cat > .nerd/mangle/broken.mg << 'EOF'
Decl broken_rule(X.Type<int>).

# SYNTAX ERROR: lowercase variable
broken_rule(x) :- x = 5.
EOF

# Expected: Validation fails, repair triggered
# Check logs for repair attempt
./nerd.exe logs --grep "repair"

# Manual fix after observing repair
cat > .nerd/mangle/broken.mg << 'EOF'
Decl broken_rule(X.Type<int>).

broken_rule(X) :- X = 5.
EOF
```

**Verification:**
- [ ] Validation failure logged
- [ ] Repair workflow triggered (check for "initiating repair")
- [ ] After fix: validation passes
- [ ] Error metrics incremented

### Test 6: Rapid Edit Debouncing

**Scenario:** 10 rapid saves in 5 seconds → debouncing prevents validation storm

```bash
# Script to rapidly modify file
for i in {1..10}; do
  echo "test_fact(\"edit_$i\")." >> .nerd/mangle/test_rules.mg
  sleep 0.5
done

# Expected: Watcher debounces events (e.g., 300ms window)
# Should trigger ~2-3 validations, not 10
./nerd.exe stats --component mangle_watcher | grep validation_count

# Check that not all 10 edits triggered separate validations
```

**Verification:**
- [ ] Fewer validations than edit events (debouncing active)
- [ ] Final state contains all 10 facts
- [ ] No event loss despite debouncing

### Test 7: Concurrent File Modifications

**Scenario:** Modify multiple `.mg` files simultaneously

```bash
# Terminal 1
cat >> .nerd/mangle/test_rules.mg << 'EOF'
test_fact("concurrent_1").
EOF

# Terminal 2 (run simultaneously)
cat >> .nerd/mangle/learned.mg << 'EOF'
Decl learned_fact(X.Type<string>).
learned_fact("concurrent_2").
EOF

# Expected: Watcher handles both events without race conditions
./nerd.exe query 'test_fact("concurrent_1")?, learned_fact("concurrent_2")?'
```

**Verification:**
- [ ] Both files validated
- [ ] No race condition errors
- [ ] Both facts loaded correctly
- [ ] Event queue processed in order

### Test 8: Large File Validation

**Scenario:** Create file with 100+ rules → performance test

```bash
# Generate large Mangle file
cat > .nerd/mangle/large.mg << 'EOF'
Decl large_fact(X.Type<int>).

EOF

# Add 100 facts
for i in {1..100}; do
  echo "large_fact($i)." >> .nerd/mangle/large.mg
done

# Expected: Validation completes within reasonable time (<5s)
time ./nerd.exe validate .nerd/mangle/large.mg
```

**Verification:**
- [ ] Validation completes <5s
- [ ] All 100 facts loaded
- [ ] Memory usage reasonable
- [ ] No timeout errors

### Test 9: File Deletion → Cleanup

**Scenario:** Delete `.mg` file → facts removed from kernel

```bash
# Delete test file
rm .nerd/mangle/test_rules.mg

# Expected: Watcher detects deletion, unloads facts
./nerd.exe query 'test_fact(X)?'
# Expected: No results (facts unloaded)
```

**Verification:**
- [ ] Deletion event logged
- [ ] Facts removed from kernel
- [ ] No stale data remains
- [ ] Stats updated (files_watched decremented)

---

## Chaos Tests (15 minutes)

**Goal:** Break the watcher with extreme scenarios and verify recovery.

### Test 10: Create/Delete/Modify Rapid Succession

**Scenario:** Chaotic file operations in rapid sequence

```bash
# Chaos script
for i in {1..20}; do
  # Create
  echo "Decl chaos_$i(X.Type<int>)." > .nerd/mangle/chaos_$i.mg
  sleep 0.1

  # Modify
  echo "chaos_$i(42)." >> .nerd/mangle/chaos_$i.mg
  sleep 0.1

  # Delete
  rm .nerd/mangle/chaos_$i.mg
  sleep 0.1
done

# Expected: Watcher survives, no crashes
./nerd.exe stats --component mangle_watcher
```

**Verification:**
- [ ] No watcher crashes
- [ ] Event queue processed
- [ ] No memory leaks
- [ ] Final state is clean (no orphaned facts)

### Test 11: Concurrent Modifications to Same File

**Scenario:** Two processes modify the same file simultaneously → race condition handling

```bash
# Terminal 1
while true; do
  echo "test_fact(\"writer_1_$$\")." >> .nerd/mangle/concurrent.mg
  sleep 0.1
done &
PID1=$!

# Terminal 2
while true; do
  echo "test_fact(\"writer_2_$$\")." >> .nerd/mangle/concurrent.mg
  sleep 0.1
done &
PID2=$!

# Let run for 10 seconds
sleep 10

# Stop both writers
kill $PID1 $PID2

# Expected: File is intact, watcher handled race gracefully
./nerd.exe validate .nerd/mangle/concurrent.mg
```

**Verification:**
- [ ] File not corrupted
- [ ] Watcher didn't crash
- [ ] Validation succeeds
- [ ] Possible duplicate facts (acceptable)

### Test 12: Corrupt File Mid-Write

**Scenario:** Simulate file corruption during write

```bash
# Start writing large file
cat > .nerd/mangle/corrupt.mg << 'EOF'
Decl corrupt_test(X.Type<string>).

EOF

for i in {1..1000}; do
  echo "corrupt_test(\"line_$i\")." >> .nerd/mangle/corrupt.mg
done &
WRITE_PID=$!

# Kill the write process mid-operation
sleep 0.5
kill -9 $WRITE_PID

# Expected: Watcher detects, validation may fail, but no crash
./nerd.exe validate .nerd/mangle/corrupt.mg
# Expected: Validation error (incomplete file)
```

**Verification:**
- [ ] Watcher didn't crash
- [ ] Validation error logged
- [ ] Repair may trigger
- [ ] System remains stable

### Test 13: Rename File During Validation

**Scenario:** Rename `.mg` file while validation is in progress

```bash
# Create large file for slow validation
cat > .nerd/mangle/rename_test.mg << 'EOF'
Decl rename_fact(X.Type<int>).

EOF

for i in {1..500}; do
  echo "rename_fact($i)." >> .nerd/mangle/rename_test.mg
done

# Trigger validation and immediately rename
./nerd.exe validate .nerd/mangle/rename_test.mg &
VALIDATE_PID=$!

sleep 0.2
mv .nerd/mangle/rename_test.mg .nerd/mangle/renamed.mg

# Wait for validation
wait $VALIDATE_PID

# Expected: Graceful handling (may fail validation, but no panic)
```

**Verification:**
- [ ] No panic/crash
- [ ] Error logged (file not found)
- [ ] Watcher detects new file (renamed.mg)
- [ ] System recovers

### Test 14: Directory Flood (1000 Files)

**Scenario:** Create 1000 `.mg` files → watcher resource limits

```bash
# Generate 1000 small Mangle files
mkdir -p .nerd/mangle/flood
for i in {1..1000}; do
  cat > .nerd/mangle/flood/file_$i.mg << EOF
Decl flood_$i(X.Type<int>).
flood_$i($i).
EOF
done

# Expected: Watcher may throttle, but processes all eventually
# Monitor resource usage
./nerd.exe stats --component mangle_watcher
```

**Verification:**
- [ ] Watcher doesn't crash
- [ ] Memory usage bounded (<500MB)
- [ ] Events eventually processed
- [ ] Possible throttling/backpressure logged

**Cleanup:**
```bash
rm -rf .nerd/mangle/flood
```

### Test 15: Watcher Recovery After Crash

**Scenario:** Simulate watcher crash → automatic restart and recovery

```bash
# This requires injecting a panic or killing the watcher goroutine
# If MangleWatcher has a recovery mechanism:

# Method 1: Send invalid signal to trigger panic (if implemented)
# (This is hypothetical; actual implementation may vary)

# Method 2: Manually stop and restart
# Stop watcher (if CLI command exists)
./nerd.exe watcher --stop

# Make changes while watcher is down
echo "Decl recovery_test(X.Type<string>)." > .nerd/mangle/recovery.mg

# Restart watcher
./nerd.exe watcher --start

# Expected: Watcher re-scans directory, catches up on missed changes
./nerd.exe query 'recovery_test(X)?'
```

**Verification:**
- [ ] Watcher restarts successfully
- [ ] Missed changes detected on restart
- [ ] Facts loaded correctly
- [ ] No state corruption

---

## Post-Test Verification

### System Health Check

```bash
# 1. Verify kernel is responsive
./nerd.exe query 'true?'

# 2. Check for goroutine leaks
./nerd.exe stats --goroutines

# 3. Validate all remaining .mg files
./nerd.exe validate .nerd/mangle/*.mg

# 4. Check watcher stats
./nerd.exe stats --component mangle_watcher
```

### Cleanup

```bash
# Remove test files
rm -f .nerd/mangle/test_rules.mg
rm -f .nerd/mangle/broken.mg
rm -f .nerd/mangle/large.mg
rm -f .nerd/mangle/chaos_*.mg
rm -f .nerd/mangle/concurrent.mg
rm -f .nerd/mangle/corrupt.mg
rm -f .nerd/mangle/renamed.mg
rm -f .nerd/mangle/recovery.mg
rm -rf .nerd/mangle/flood

# Restart watcher to clean state
./nerd.exe watcher --restart
```

---

## Success Criteria

### Conservative
- ✅ All file events detected (create, modify, delete)
- ✅ Validation triggered correctly
- ✅ Stats accurately updated
- ✅ No false positives/negatives

### Aggressive
- ✅ Invalid rules trigger repair
- ✅ Debouncing prevents event storms (<5 validations for 10 rapid edits)
- ✅ Concurrent modifications handled safely
- ✅ Large files validated in <5s
- ✅ Deletion cleanup works correctly

### Chaos
- ✅ No crashes under extreme load
- ✅ Graceful handling of file corruption
- ✅ Race condition safety (concurrent writes)
- ✅ Resource bounds maintained (<500MB for 1000 files)
- ✅ Recovery from crashes successful

---

## Known Edge Cases

### Platform Differences
- **Windows:** File locking may cause delayed deletions (handle "file in use" gracefully)
- **Linux/macOS:** inotify limits may throttle events for 1000+ files (check `fs.inotify.max_user_watches`)

### Debounce Tuning
- Default debounce window: **300ms**
- If tests fail due to event loss, check debounce configuration in `mangle_watcher.go`

### File System Latency
- Network drives or slow HDDs may cause validation delays
- Increase timeouts if running on non-SSD storage

---

## Integration with Log Analyzer

After completing tests, use the log-analyzer skill to query watcher events:

```bash
# Query all validation failures
./nerd.exe query 'log_entry(_, /validation_error, Msg, _)?'

# Count events by type
./nerd.exe query '
  log_entry(_, Type, _, _)
  |> do fn:group_by(Type), let Count = fn:count()
  |> select(Type, Count)?
'

# Find files that failed validation
./nerd.exe query 'validation_failed(File, Reason, _)?'
```

---

## Severity Ratings

| Test | Conservative | Aggressive | Chaos |
|------|--------------|------------|-------|
| **New file creation** | ✓ | | |
| **File modification** | ✓ | | |
| **Valid rule addition** | ✓ | | |
| **Stats verification** | ✓ | | |
| **Invalid rule repair** | | ✓ | |
| **Rapid edit debouncing** | | ✓ | |
| **Concurrent modifications** | | ✓ | |
| **Large file (100+ rules)** | | ✓ | |
| **File deletion cleanup** | | ✓ | |
| **Create/delete/modify rapid** | | | ✓ |
| **Same-file concurrent writes** | | | ✓ |
| **Corrupt file mid-write** | | | ✓ |
| **Rename during validation** | | | ✓ |
| **Directory flood (1000 files)** | | | ✓ |
| **Watcher crash recovery** | | | ✓ |

**Total Duration:** ~30 minutes (5 + 10 + 15)
