# TDD Infinite Loop Stress Test

Stress test for the TDD repair loop with perpetually failing tests.

## Overview

Tests the TDD system with:

- Tests that always fail
- Maximum repair iterations
- Loop detection/breaking
- Resource cleanup on abort

**Expected Duration:** 20-30 minutes

## Quick Reference

### Test Commands

```bash
# Create a test that always fails
./nerd.exe run "create a test that checks if 1 equals 2"

# Trigger TDD repair loop
./nerd.exe test --tdd-repair

# Monitor iterations
./nerd.exe query "tdd_iteration"
```

### Expected Behavior

- TDD loop should hit max iterations (default 5)
- Should not infinite loop
- Should report failure after max attempts
- Memory should remain stable

---

See `campaign-marathon.md` for detailed workflow structure.

