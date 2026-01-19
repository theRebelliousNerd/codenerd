# Thunderdome Battle Stress Test

Stress test for adversarial tool testing arena.

## Overview

Tests the Thunderdome with:

- 100+ attack vectors
- Generated tools under attack
- Sandbox isolation
- Attack result aggregation

**Expected Duration:** 25-40 minutes

## Quick Reference

### Test Commands

```bash
# Generate a tool first
./nerd.exe tool generate "file processor"

# Run thunderdome with many attacks
./nerd.exe thunderdome --attacks 100

# Check attack results
./nerd.exe query "thunderdome_result"
```

### Expected Behavior

- All attacks should run in sandbox
- Failed attacks should be logged
- Tool should be hardened or rejected
- No sandbox escapes

---

See `tool-generation-nesting.md` for detailed workflow structure.

