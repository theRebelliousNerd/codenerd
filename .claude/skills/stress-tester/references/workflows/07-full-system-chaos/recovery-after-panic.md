# Recovery After Panic Stress Test

Stress test for system recovery from panic states.

## Overview

Tests recovery with:

- Induced panic scenarios
- State restoration
- Partial operation recovery
- Data integrity verification

**Expected Duration:** 20-30 minutes

## Quick Reference

### Test Commands

```bash
# Trigger known panic scenarios
./nerd.exe check-mangle .claude/skills/stress-tester/assets/cyclic_rules.mg
# Force resource exhaustion
./nerd.exe run "process this extremely large input" < /dev/urandom | head -c 10M

# Verify recovery
./nerd.exe status
./nerd.exe query "all_facts"

# Check logs for panic
grep -i "panic" .nerd/logs/*.log
```

### Expected Behavior

- System should recover after panic
- State should be restorable
- No data corruption
- Graceful degradation

---

## Severity Levels

### Conservative
- Soft errors, clean recovery

### Aggressive
- Force panics, verify recovery

### Chaos
- Multiple panics in sequence

### Hybrid
- Panic during active operations

