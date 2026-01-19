# Long Running Session Stress Test

Stress test for extended session stability.

## Overview

Tests session management with:

- 2+ hour continuous operation
- Memory accumulation
- Context degradation
- State consistency

**Expected Duration:** 120+ minutes

## Quick Reference

### Test Commands

```bash
# Start long session with varied tasks
./nerd.exe run "implement feature A"
# wait 15 min
./nerd.exe run "add tests for feature A"
# wait 15 min
./nerd.exe run "review the implementation"
# wait 15 min
./nerd.exe run "refactor based on review"
# continue for 2+ hours...

# Monitor health
./nerd.exe query "session_duration"
./nerd.exe query "memory_usage"
./nerd.exe query "fact_count"
```

### Expected Behavior

- Session should remain stable
- Memory should not grow unbounded
- Context should stay coherent
- No gradual degradation

---

## Severity Levels

### Conservative
- 30 minute session

### Aggressive
- 2 hour session

### Chaos
- 4+ hour session with constant load

### Hybrid
- Long session with all subsystem usage

