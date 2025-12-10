# Shadow Mode Stress Test

Stress test for action simulation without execution.

## Overview

Tests Shadow Mode with:

- Complex action simulation
- Large derivation chains
- Rollback verification
- State isolation

**Expected Duration:** 15-25 minutes

## Quick Reference

### Test Commands

```bash
# Simulate dangerous operations
./nerd.exe shadow "delete all test files in the project"
./nerd.exe shadow "refactor 100 functions simultaneously"
./nerd.exe shadow "merge 5 conflicting branches"

# Verify no actual changes
git status
./nerd.exe query "shadow_action"
./nerd.exe query "shadow_rollback"
```

### Expected Behavior

- No actual file changes
- Derivations should complete
- State should roll back cleanly
- Shadow results should be accurate

---

## Severity Levels

### Conservative
- Simple shadow operations

### Aggressive
- Complex multi-step shadows

### Chaos
- Contradictory shadow requests

### Hybrid
- Shadow during active session

